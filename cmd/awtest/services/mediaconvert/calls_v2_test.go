package mediaconvert

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"regexp"
	"strings"
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
	query  url.Values
}

// fakeSvc speaks the MediaConvert restJson1 protocol. handle receives the
// SigV4 credential-scope region, the request path (without the API version
// prefix) and the query, and returns the HTTP status and JSON response.
type fakeSvc struct {
	handle func(region, path string, q url.Values) (int, interface{})

	mu   sync.Mutex
	reqs []recorded
}

func apiError(code, msg string) (int, interface{}) {
	return http.StatusForbidden, map[string]string{"__type": code, "message": msg}
}

func (f *fakeSvc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, r.Method, r.URL.Path, r.URL.Query()})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	var status int
	var out interface{}
	const prefix = "/2017-08-29/"
	if !strings.HasPrefix(r.URL.Path, prefix) || r.Method != http.MethodGet {
		status, out = apiError("UnknownOperationException", "bad request "+r.Method+" "+r.URL.Path)
	} else {
		status, out = f.handle(region, strings.TrimPrefix(r.URL.Path, prefix), r.URL.Query())
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(out)
}

func (f *fakeSvc) paths(path string) []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []recorded
	for _, r := range f.reqs {
		if r.path == "/2017-08-29/"+path {
			out = append(out, r)
		}
	}
	return out
}

func (f *fakeSvc) regionsOf(path string) []string {
	var out []string
	for _, r := range f.paths(path) {
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
	t.Setenv("AWS_ENDPOINT_URL_MEDIACONVERT", unreachable)

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
	svc := MediaConvertCalls[i]
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

// listHandler serves a two-page list for path: a populated resource carrying
// the full item and, on the second page (nextToken "p2"), a sparse resource.
func listHandler(path, key string, full, sparse map[string]interface{}) func(region, p string, q url.Values) (int, interface{}) {
	return func(region, p string, q url.Values) (int, interface{}) {
		if p != path {
			return apiError("UnknownOperationException", p)
		}
		if q.Get("maxResults") != "20" {
			return apiError("BadRequestException", "maxResults = "+q.Get("maxResults"))
		}
		if q.Get("nextToken") == "p2" {
			return http.StatusOK, map[string]interface{}{key: []map[string]interface{}{sparse}}
		}
		return http.StatusOK, map[string]interface{}{key: []map[string]interface{}{full}, "nextToken": "p2"}
	}
}

func TestListQueuesInvoke(t *testing.T) {
	f := &fakeSvc{handle: listHandler("queues", "queues",
		map[string]interface{}{
			"name": "Default", "arn": "arn:aws:mediaconvert:us-west-2:111111111111:queues/Default",
			"status": "ACTIVE", "type": "SYSTEM", "pricingPlan": "ON_DEMAND", "description": "Default queue",
			"submittedJobsCount": 5, "progressingJobsCount": 2, "createdAt": 1700000000,
		},
		map[string]interface{}{"name": "bare-queue"},
	)}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"Name": "Default", "Arn": "arn:aws:mediaconvert:us-west-2:111111111111:queues/Default",
			"Status": "ACTIVE", "Type": "SYSTEM", "PricingPlan": "ON_DEMAND", "Description": "Default queue",
			"SubmittedJobsCount": "5", "ProgressingJobsCount": "2", "CreatedAt": "2023-11-14T22:13:20Z",
			"Region": "us-west-2",
		},
		{
			"Name": "bare-queue", "Arn": "", "Status": "", "Type": "", "PricingPlan": "", "Description": "",
			"SubmittedJobsCount": "", "ProgressingJobsCount": "", "CreatedAt": "", "Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ServiceName != "MediaConvert" || results[0].MethodName != "mediaconvert:ListQueues" || results[0].ResourceType != "queue" || results[0].ResourceName != "Default" {
		t.Errorf("result 0 = %+v", results[0])
	}
	if got := len(f.paths("queues")); got != 2 {
		t.Errorf("ListQueues calls = %d, want 2 (NextToken must be followed)", got)
	}
}

func TestListJobsInvoke(t *testing.T) {
	f := &fakeSvc{handle: listHandler("jobs", "jobs",
		map[string]interface{}{
			"id": "1700000000000-abc123", "arn": "arn:aws:mediaconvert:us-west-2:111111111111:jobs/1700000000000-abc123",
			"status": "ERROR", "queue": "arn:queue", "role": "arn:role", "jobTemplate": "tmpl",
			"createdAt": 1700000000, "currentPhase": "UPLOADING", "errorMessage": "boom",
		},
		map[string]interface{}{"id": "bare-job"},
	)}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"Id": "1700000000000-abc123", "Arn": "arn:aws:mediaconvert:us-west-2:111111111111:jobs/1700000000000-abc123",
			"Status": "ERROR", "Queue": "arn:queue", "Role": "arn:role", "JobTemplate": "tmpl",
			"CreatedAt": "2023-11-14T22:13:20Z", "CurrentPhase": "UPLOADING", "ErrorMessage": "boom",
			"Region": "us-west-2",
		},
		{
			"Id": "bare-job", "Arn": "", "Status": "", "Queue": "", "Role": "", "JobTemplate": "",
			"CreatedAt": "", "CurrentPhase": "", "ErrorMessage": "", "Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ResourceType != "job" || results[0].ResourceName != "1700000000000-abc123" {
		t.Errorf("result 0 = %+v", results[0])
	}
	if got := len(f.paths("jobs")); got != 2 {
		t.Errorf("ListJobs calls = %d, want 2", got)
	}
}

func TestListPresetsInvoke(t *testing.T) {
	f := &fakeSvc{handle: listHandler("presets", "presets",
		map[string]interface{}{
			"name": "System-Generic_Hd_Mp4", "arn": "arn:preset", "description": "Generic HD",
			"type": "SYSTEM", "category": "VIDEO", "createdAt": 1700000000,
		},
		map[string]interface{}{"name": "bare-preset"},
	)}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"Name": "System-Generic_Hd_Mp4", "Arn": "arn:preset", "Description": "Generic HD", "Type": "SYSTEM",
			"Category": "VIDEO", "CreatedAt": "2023-11-14T22:13:20Z", "Region": "us-west-2",
		},
		{
			"Name": "bare-preset", "Arn": "", "Description": "", "Type": "", "Category": "", "CreatedAt": "",
			"Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ResourceType != "preset" || results[0].ResourceName != "System-Generic_Hd_Mp4" {
		t.Errorf("result 0 = %+v", results[0])
	}
	if got := len(f.paths("presets")); got != 2 {
		t.Errorf("ListPresets calls = %d, want 2", got)
	}
}

func TestInvokeErrorText(t *testing.T) {
	f := &fakeSvc{handle: func(region, p string, q url.Values) (int, interface{}) {
		return apiError("AccessDeniedException", "not authorized for "+p)
	}}
	for i, path := range []string{"queues", "jobs", "presets"} {
		ctx := newEnv(t, f, "us-west-2")
		results, err := run(ctx, i)
		if err == nil {
			t.Fatalf("%s: expected an error", MediaConvertCalls[i].Name)
		}
		want := "AccessDeniedException: not authorized for " + path
		if err.Error() != want {
			t.Errorf("%s: error = %q, want %q", MediaConvertCalls[i].Name, err.Error(), want)
		}
		if len(results) != 1 || results[0].Error == nil || results[0].Error.Error() != want {
			t.Errorf("%s: Process error result = %+v", MediaConvertCalls[i].Name, results)
		}
		if results[0].MethodName != MediaConvertCalls[i].Name {
			t.Errorf("MethodName = %q", results[0].MethodName)
		}
	}
}

// A failing region is skipped; the call only fails when nothing was found and
// at least one region failed.
func TestFailingRegionIsSkipped(t *testing.T) {
	f := &fakeSvc{handle: func(region, p string, q url.Values) (int, interface{}) {
		if region == "eu-west-1" {
			return apiError("AccessDeniedException", "denied in "+region)
		}
		return http.StatusOK, map[string]interface{}{"queues": []map[string]string{{"name": "q-" + region}}}
	}}
	ctx := newEnv(t, f, "us-east-1", "eu-west-1", "ap-south-1")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range results {
		names = append(names, r.ResourceName)
	}
	if want := []string{"q-us-east-1", "q-ap-south-1"}; !reflect.DeepEqual(names, want) {
		t.Errorf("queues = %v, want %v", names, want)
	}
}

func TestRegionScopeVisitsEachRegion(t *testing.T) {
	f := &fakeSvc{handle: func(region, p string, q url.Values) (int, interface{}) {
		switch p {
		case "queues":
			return http.StatusOK, map[string]interface{}{"queues": []map[string]string{{"name": "q-" + region}}}
		case "jobs":
			return http.StatusOK, map[string]interface{}{"jobs": []map[string]string{{"id": "j-" + region}}}
		case "presets":
			return http.StatusOK, map[string]interface{}{"presets": []map[string]string{{"name": "p-" + region}}}
		}
		return apiError("UnknownOperationException", p)
	}}
	scope := []string{"us-east-1", "eu-west-1", "ap-south-1"}
	for i, path := range []string{"queues", "jobs", "presets"} {
		ctx := newEnv(t, f, scope...)
		f.mu.Lock()
		f.reqs = nil
		f.mu.Unlock()
		results, err := run(ctx, i)
		if err != nil {
			t.Fatal(err)
		}
		if got := f.regionsOf(path); !reflect.DeepEqual(got, scope) {
			t.Errorf("%s SigV4 regions = %v, want %v", path, got, scope)
		}
		var regions []string
		for _, d := range detailsOf(results) {
			regions = append(regions, d["Region"].(string))
		}
		if !reflect.DeepEqual(regions, scope) {
			t.Errorf("%s result regions = %v, want %v", MediaConvertCalls[i].Name, regions, scope)
		}
	}
}
