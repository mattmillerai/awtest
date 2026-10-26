package macie2

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"sync"
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/[^/]+/aws4_request`)

type recorded struct {
	region string
	method string
	path   string
	body   map[string]interface{}
}

// fakeSvc speaks the REST-JSON protocol Macie uses. handle receives the
// request (with the SigV4 credential-scope region) and returns the HTTP status
// and JSON response.
type fakeSvc struct {
	handle func(r recorded) (int, interface{})

	mu   sync.Mutex
	reqs []recorded
}

func apiError(code, msg string) (int, interface{}) {
	return http.StatusBadRequest, map[string]string{"__type": code, "message": msg}
}

func (f *fakeSvc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	raw, _ := io.ReadAll(r.Body)
	body := map[string]interface{}{}
	_ = json.Unmarshal(raw, &body)
	rec := recorded{region, r.Method, r.URL.EscapedPath(), body}
	f.mu.Lock()
	f.reqs = append(f.reqs, rec)
	f.mu.Unlock()

	status, out := f.handle(rec)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(out)
}

func (f *fakeSvc) matching(method, path string) []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []recorded
	for _, r := range f.reqs {
		if r.method == method && r.path == path {
			out = append(out, r)
		}
	}
	return out
}

func (f *fakeSvc) regionsOf(method, path string) []string {
	var out []string
	for _, r := range f.matching(method, path) {
		out = append(out, r.region)
	}
	return out
}

func newEnv(t *testing.T, f *fakeSvc, regions ...string) context.Context {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	t.Setenv("HOME", t.TempDir())
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	for _, k := range []string{"AWS_PROFILE", "AWS_DEFAULT_PROFILE", "AWS_SHARED_CREDENTIALS_FILE", "AWS_CONFIG_FILE", "AWS_REGION", "AWS_DEFAULT_REGION"} {
		t.Setenv(k, "")
	}
	// A client that honoured ambient endpoints could never reach the server.
	const unreachable = "http://ambient-endpoint.invalid:9"
	t.Setenv("AWS_ENDPOINT_URL", unreachable)
	t.Setenv("AWS_ENDPOINT_URL_MACIE2", unreachable)

	cfg, err := awsv2.LoadConfig(context.Background(), awsv2.Options{
		Region:          "us-west-2",
		AccessKeyID:     "AKIAFAKEFAKEFAKE",
		SecretAccessKey: "fake",
		Endpoint:        srv.URL,
		HTTPClient:      srv.Client(),
		MaxAttempts:     1,
		Quiet:           true,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := types.WithAWSConfig(context.Background(), cfg)
	if len(regions) > 0 {
		ctx, _ = types.WithRegionScope(ctx, regions)
	}
	return ctx
}

func run(ctx context.Context, i int) ([]types.ScanResult, error) {
	svc := Macie2Calls[i]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

func detailsOf(results []types.ScanResult) []map[string]interface{} {
	var out []map[string]interface{}
	for _, r := range results {
		out = append(out, r.Details)
	}
	return out
}

func TestListClassificationJobsInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(r recorded) (int, interface{}) {
		if r.method != "POST" || r.path != "/jobs/list" {
			return apiError("UnknownOperationException", r.method+" "+r.path)
		}
		if r.body["nextToken"] == "t2" {
			return http.StatusOK, map[string]interface{}{"items": []map[string]string{{"jobId": "job-bare"}}}
		}
		return http.StatusOK, map[string]interface{}{"items": []map[string]string{{
			"jobId": "job-1", "name": "demo-job", "jobType": "ONE_TIME", "jobStatus": "COMPLETE",
			"createdAt": "2023-11-14T22:13:20Z",
		}}, "nextToken": "t2"}
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"JobId": "job-1", "Name": "demo-job", "JobType": "ONE_TIME", "JobStatus": "COMPLETE",
			"CreatedAt": "2023-11-14T22:13:20Z", "Region": "us-west-2",
		},
		{"JobId": "job-bare", "Name": "", "JobType": "", "JobStatus": "", "CreatedAt": "", "Region": "us-west-2"},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ServiceName != "Macie" || results[0].MethodName != "macie2:ListClassificationJobs" || results[0].ResourceType != "classification-job" || results[0].ResourceName != "demo-job" {
		t.Errorf("result 0 = %+v", results[0])
	}
	// A job without a name is reported under its id.
	if results[1].ResourceName != "job-bare" {
		t.Errorf("result 1 ResourceName = %q", results[1].ResourceName)
	}
	reqs := f.matching("POST", "/jobs/list")
	if len(reqs) != 2 || reqs[0].body["maxResults"] != float64(25) {
		t.Errorf("ListClassificationJobs requests = %+v", reqs)
	}
}

func TestListFindingsInvoke(t *testing.T) {
	describeCalls := 0
	f := &fakeSvc{handle: func(r recorded) (int, interface{}) {
		switch {
		case r.method == "POST" && r.path == "/findings":
			if r.body["nextToken"] == "t2" {
				return http.StatusOK, map[string]interface{}{"findingIds": []string{"f3"}}
			}
			return http.StatusOK, map[string]interface{}{"findingIds": []string{"f1", "f2"}, "nextToken": "t2"}
		case r.method == "POST" && r.path == "/findings/describe":
			describeCalls++
			if describeCalls == 1 {
				// The first batch fails once; the single retry succeeds.
				return apiError("InternalServerException", "transient")
			}
			ids := r.body["findingIds"].([]interface{})
			if len(ids) == 1 {
				return http.StatusOK, map[string]interface{}{"findings": []map[string]string{{"id": "f3"}}}
			}
			return http.StatusOK, map[string]interface{}{"findings": []map[string]interface{}{
				{
					"id": "f1", "type": "SensitiveData:S3Object/Credentials", "title": "Creds found",
					"severity": map[string]interface{}{"description": "High", "score": 3}, "category": "CLASSIFICATION",
					"count": 4, "createdAt": "2023-11-14T22:13:20Z",
				},
				{"id": "f2"},
			}}
		}
		return apiError("UnknownOperationException", r.method+" "+r.path)
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"Id": "f1", "Type": "SensitiveData:S3Object/Credentials", "Title": "Creds found", "Severity": "High",
			"Category": "CLASSIFICATION", "Count": "4", "CreatedAt": "2023-11-14T22:13:20Z", "Region": "us-west-2",
		},
		{
			"Id": "f2", "Type": "", "Title": "", "Severity": "", "Category": "",
			"Count": "", "CreatedAt": "", "Region": "us-west-2",
		},
		{
			"Id": "f3", "Type": "", "Title": "", "Severity": "", "Category": "",
			"Count": "", "CreatedAt": "", "Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ResourceType != "finding" || results[0].ResourceName != "f1" {
		t.Errorf("result 0 = %+v", results[0])
	}
	// Page 1 is batch-fetched (failed once, retried), then page 2.
	if got := len(f.matching("POST", "/findings/describe")); got != 3 {
		t.Errorf("GetFindings calls = %d, want 3", got)
	}
	if got := len(f.matching("POST", "/findings")); got != 2 {
		t.Errorf("ListFindings calls = %d, want 2", got)
	}
}

// A GetFindings batch that fails twice aborts the region (no further pages) and
// is reported only when nothing was found at all.
func TestListFindingsBatchFailureAbortsRegion(t *testing.T) {
	f := &fakeSvc{handle: func(r recorded) (int, interface{}) {
		switch {
		case r.method == "POST" && r.path == "/findings":
			return http.StatusOK, map[string]interface{}{"findingIds": []string{"f1"}, "nextToken": "t2"}
		case r.method == "POST" && r.path == "/findings/describe":
			return apiError("AccessDeniedException", "no describe")
		}
		return apiError("UnknownOperationException", r.method+" "+r.path)
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 1)
	if err == nil {
		t.Fatal("expected an error")
	}
	want := "AccessDeniedException: no describe"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if len(results) != 1 || results[0].Error == nil || results[0].Error.Error() != want {
		t.Errorf("Process error result = %+v", results)
	}
	if got := len(f.matching("POST", "/findings/describe")); got != 2 {
		t.Errorf("GetFindings calls = %d, want 2 (one try, one retry)", got)
	}
	if got := len(f.matching("POST", "/findings")); got != 1 {
		t.Errorf("ListFindings calls = %d, want 1 (region aborted)", got)
	}
}

func TestDescribeBucketsInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(r recorded) (int, interface{}) {
		if r.method != "POST" || r.path != "/datasources/s3" {
			return apiError("UnknownOperationException", r.method+" "+r.path)
		}
		if r.body["nextToken"] == "t2" {
			return http.StatusOK, map[string]interface{}{"buckets": []map[string]string{{"bucketName": "bare-bucket"}}}
		}
		return http.StatusOK, map[string]interface{}{"buckets": []map[string]interface{}{{
			"bucketName": "demo-bucket", "accountId": "111111111111", "bucketArn": "arn:aws:s3:::demo-bucket",
			"objectCount": 12, "sizeInBytes": 3456, "classifiableObjectCount": 10, "sensitivityScore": 55,
		}}, "nextToken": "t2"}
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"BucketName": "demo-bucket", "AccountId": "111111111111", "BucketArn": "arn:aws:s3:::demo-bucket",
			"ObjectCount": "12", "SizeInBytes": "3456", "ClassifiableObjectCount": "10",
			"SensitivityScore": "55", "Region": "us-west-2",
		},
		{
			"BucketName": "bare-bucket", "AccountId": "", "BucketArn": "", "ObjectCount": "",
			"SizeInBytes": "", "ClassifiableObjectCount": "", "SensitivityScore": "", "Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ResourceType != "monitored-bucket" || results[0].ResourceName != "demo-bucket" {
		t.Errorf("result 0 = %+v", results[0])
	}
	reqs := f.matching("POST", "/datasources/s3")
	if len(reqs) != 2 || reqs[0].body["maxResults"] != float64(50) {
		t.Errorf("DescribeBuckets requests = %+v", reqs)
	}
}

func TestInvokeErrorText(t *testing.T) {
	f := &fakeSvc{handle: func(r recorded) (int, interface{}) {
		return apiError("AccessDeniedException", "not authorized")
	}}
	for i := range Macie2Calls {
		ctx := newEnv(t, f, "us-west-2")
		results, err := run(ctx, i)
		if err == nil {
			t.Fatalf("%s: expected an error", Macie2Calls[i].Name)
		}
		want := "AccessDeniedException: not authorized"
		if err.Error() != want {
			t.Errorf("%s: error = %q, want %q", Macie2Calls[i].Name, err.Error(), want)
		}
		if len(results) != 1 || results[0].Error == nil || results[0].Error.Error() != want || results[0].MethodName != Macie2Calls[i].Name {
			t.Errorf("%s: Process error result = %+v", Macie2Calls[i].Name, results)
		}
	}
}

func TestRegionScopeVisitsEachRegion(t *testing.T) {
	f := &fakeSvc{handle: func(r recorded) (int, interface{}) {
		switch {
		case r.method == "POST" && r.path == "/jobs/list":
			return http.StatusOK, map[string]interface{}{"items": []map[string]string{{"jobId": "j-" + r.region}}}
		case r.method == "POST" && r.path == "/findings":
			return http.StatusOK, map[string]interface{}{"findingIds": []string{"f-" + r.region}}
		case r.method == "POST" && r.path == "/findings/describe":
			return http.StatusOK, map[string]interface{}{"findings": []map[string]string{{"id": "f-" + r.region}}}
		case r.method == "POST" && r.path == "/datasources/s3":
			return http.StatusOK, map[string]interface{}{"buckets": []map[string]string{{"bucketName": "b-" + r.region}}}
		}
		return apiError("UnknownOperationException", r.method+" "+r.path)
	}}
	scope := []string{"us-east-1", "eu-west-1", "ap-south-1"}
	for _, tc := range []struct {
		idx   int
		paths []string
	}{
		{0, []string{"/jobs/list"}},
		{1, []string{"/findings", "/findings/describe"}},
		{2, []string{"/datasources/s3"}},
	} {
		ctx := newEnv(t, f, scope...)
		f.mu.Lock()
		f.reqs = nil
		f.mu.Unlock()
		results, err := run(ctx, tc.idx)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range tc.paths {
			if got := f.regionsOf("POST", p); !reflect.DeepEqual(got, scope) {
				t.Errorf("%s SigV4 regions = %v, want %v", p, got, scope)
			}
		}
		var regions []string
		for _, d := range detailsOf(results) {
			regions = append(regions, d["Region"].(string))
		}
		if !reflect.DeepEqual(regions, scope) {
			t.Errorf("%s result regions = %v, want %v", Macie2Calls[tc.idx].Name, regions, scope)
		}
	}
}
