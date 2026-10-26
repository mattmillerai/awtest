package guardduty

import (
	"context"
	"encoding/json"
	"io"
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
	body   map[string]interface{}
}

// fakeSvc speaks the REST-JSON protocol GuardDuty uses. handle receives the
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
	rec := recorded{region, r.Method, r.URL.EscapedPath(), r.URL.Query(), body}
	f.mu.Lock()
	f.reqs = append(f.reqs, rec)
	f.mu.Unlock()

	status, out := f.handle(rec)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(out)
}

func (f *fakeSvc) matching(method, pathRe string) []recorded {
	re := regexp.MustCompile("^" + pathRe + "$")
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []recorded
	for _, r := range f.reqs {
		if r.method == method && re.MatchString(r.path) {
			out = append(out, r)
		}
	}
	return out
}

func (f *fakeSvc) regionsOf(method, pathRe string) []string {
	var out []string
	for _, r := range f.matching(method, pathRe) {
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
	t.Setenv("AWS_ENDPOINT_URL_GUARDDUTY", unreachable)

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
	svc := GuardDutyCalls[i]
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

func TestListDetectorsInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(r recorded) (int, interface{}) {
		switch {
		case r.method == "GET" && r.path == "/detector":
			if r.query.Get("nextToken") == "t2" {
				return http.StatusOK, map[string]interface{}{"detectorIds": []string{"bare"}}
			}
			return http.StatusOK, map[string]interface{}{"detectorIds": []string{"demo", "broken"}, "nextToken": "t2"}
		case r.method == "GET" && r.path == "/detector/demo":
			return http.StatusOK, map[string]interface{}{
				"status": "ENABLED", "serviceRole": "arn:aws:iam::111111111111:role/gd", "createdAt": "2023-11-14T22:13:20.000Z",
				"features": []map[string]string{
					{"name": "S3_DATA_EVENTS", "status": "ENABLED"},
					{"name": "EKS_AUDIT_LOGS", "status": "DISABLED"},
					{"name": "RDS_LOGIN_EVENTS", "status": "ENABLED"},
				},
			}
		case r.method == "GET" && r.path == "/detector/bare":
			return http.StatusOK, map[string]interface{}{}
		case r.method == "GET" && r.path == "/detector/broken":
			return apiError("BadRequestException", "boom")
		}
		return apiError("UnknownOperationException", r.method+" "+r.path)
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"DetectorId": "demo", "Status": "ENABLED", "ServiceRole": "arn:aws:iam::111111111111:role/gd",
			"CreatedAt": "2023-11-14T22:13:20.000Z", "Region": "us-west-2",
			"Features": []string{"S3_DATA_EVENTS", "RDS_LOGIN_EVENTS"},
		},
		{
			"DetectorId": "bare", "Status": "", "ServiceRole": "", "CreatedAt": "", "Region": "us-west-2",
			"Features": []string(nil),
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ServiceName != "GuardDuty" || results[0].MethodName != "guardduty:ListDetectors" || results[0].ResourceType != "detector" || results[0].ResourceName != "demo" {
		t.Errorf("result 0 = %+v", results[0])
	}
	// Pagination: both pages are listed; the failing GetDetector is swallowed.
	if got := len(f.matching("GET", "/detector")); got != 2 {
		t.Errorf("ListDetectors calls = %d, want 2", got)
	}
}

func TestGetFindingsInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(r recorded) (int, interface{}) {
		switch {
		case r.method == "GET" && r.path == "/detector":
			return http.StatusOK, map[string]interface{}{"detectorIds": []string{"d1", "d2", "d3"}}
		case r.method == "POST" && r.path == "/detector/d1/findings":
			return http.StatusOK, map[string]interface{}{"findingIds": []string{"f1", "f2"}}
		case r.method == "POST" && r.path == "/detector/d2/findings":
			return http.StatusOK, map[string]interface{}{"findingIds": []string{}}
		case r.method == "POST" && r.path == "/detector/d3/findings":
			return apiError("AccessDeniedException", "no")
		case r.method == "POST" && r.path == "/detector/d1/findings/get":
			return http.StatusOK, map[string]interface{}{"findings": []map[string]interface{}{
				{
					"id": "f1", "type": "Recon:EC2/PortProbeUnprotectedPort", "title": "Port probe", "severity": 8.9,
					"description": "A port was probed", "region": "eu-west-1",
				},
				{"type": "Policy:S3/Foo"},
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
			"FindingId": "f1", "Type": "Recon:EC2/PortProbeUnprotectedPort", "Title": "Port probe", "Severity": 8.9,
			"Description": "A port was probed", "Region": "eu-west-1", "DetectorId": "d1",
		},
		// A finding without a region falls back to the scanned region.
		{
			"FindingId": "", "Type": "Policy:S3/Foo", "Title": "", "Severity": float64(0),
			"Description": "", "Region": "us-west-2", "DetectorId": "d1",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ResourceType != "finding" || results[0].ResourceName != "Port probe" || results[1].ResourceName != "Policy:S3/Foo" {
		t.Errorf("results = %+v", results)
	}
	lf := f.matching("POST", "/detector/d1/findings")
	if len(lf) != 1 || lf[0].body["maxResults"] != float64(50) {
		t.Fatalf("ListFindings requests = %+v", lf)
	}
	if sc, _ := lf[0].body["sortCriteria"].(map[string]interface{}); sc["attributeName"] != "severity" || sc["orderBy"] != "DESC" {
		t.Errorf("sortCriteria = %v", lf[0].body["sortCriteria"])
	}
	gf := f.matching("POST", "/detector/d1/findings/get")
	if len(gf) != 1 || !reflect.DeepEqual(gf[0].body["findingIds"], []interface{}{"f1", "f2"}) {
		t.Errorf("GetFindings requests = %+v", gf)
	}
	// d2 has no findings, so GetFindings is never called for it.
	if got := len(f.matching("POST", "/detector/d2/findings/get")); got != 0 {
		t.Errorf("GetFindings for d2 called %d times", got)
	}
}

func TestListFiltersInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(r recorded) (int, interface{}) {
		switch {
		case r.method == "GET" && r.path == "/detector":
			return http.StatusOK, map[string]interface{}{"detectorIds": []string{"d1"}}
		case r.method == "GET" && r.path == "/detector/d1/filter":
			if r.query.Get("nextToken") == "f2" {
				return http.StatusOK, map[string]interface{}{"filterNames": []string{"bare", "broken"}}
			}
			return http.StatusOK, map[string]interface{}{"filterNames": []string{"demo"}, "nextToken": "f2"}
		case r.method == "GET" && r.path == "/detector/d1/filter/demo":
			return http.StatusOK, map[string]interface{}{"name": "demo", "action": "ARCHIVE", "description": "quiet it"}
		case r.method == "GET" && r.path == "/detector/d1/filter/bare":
			return http.StatusOK, map[string]interface{}{"name": "bare"}
		case r.method == "GET" && r.path == "/detector/d1/filter/broken":
			return apiError("BadRequestException", "boom")
		}
		return apiError("UnknownOperationException", r.method+" "+r.path)
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{"FilterName": "demo", "Action": "ARCHIVE", "Description": "quiet it", "DetectorId": "d1", "Region": "us-west-2"},
		{"FilterName": "bare", "Action": "", "Description": "", "DetectorId": "d1", "Region": "us-west-2"},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ResourceType != "filter" || results[0].ResourceName != "demo" {
		t.Errorf("result 0 = %+v", results[0])
	}
	if got := len(f.matching("GET", "/detector/d1/filter")); got != 2 {
		t.Errorf("ListFilters calls = %d, want 2", got)
	}
}

func TestInvokeErrorText(t *testing.T) {
	f := &fakeSvc{handle: func(r recorded) (int, interface{}) {
		return apiError("AccessDeniedException", "not authorized")
	}}
	for i := range GuardDutyCalls {
		ctx := newEnv(t, f, "us-west-2")
		results, err := run(ctx, i)
		if err == nil {
			t.Fatalf("%s: expected an error", GuardDutyCalls[i].Name)
		}
		want := "AccessDeniedException: not authorized"
		if err.Error() != want {
			t.Errorf("%s: error = %q, want %q", GuardDutyCalls[i].Name, err.Error(), want)
		}
		if len(results) != 1 || results[0].Error == nil || results[0].Error.Error() != want || results[0].MethodName != GuardDutyCalls[i].Name {
			t.Errorf("%s: Process error result = %+v", GuardDutyCalls[i].Name, results)
		}
	}
}

func TestRegionScopeVisitsEachRegion(t *testing.T) {
	f := &fakeSvc{handle: func(r recorded) (int, interface{}) {
		switch {
		case r.method == "GET" && r.path == "/detector":
			return http.StatusOK, map[string]interface{}{"detectorIds": []string{"d-" + r.region}}
		case r.method == "GET" && strings.HasPrefix(r.path, "/detector/d-") && !strings.Contains(r.path[len("/detector/"):], "/"):
			return http.StatusOK, map[string]interface{}{"status": "ENABLED"}
		}
		return apiError("UnknownOperationException", r.method+" "+r.path)
	}}
	scope := []string{"us-east-1", "eu-west-1", "ap-south-1"}
	ctx := newEnv(t, f, scope...)
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.regionsOf("GET", "/detector"); !reflect.DeepEqual(got, scope) {
		t.Errorf("ListDetectors SigV4 regions = %v, want %v", got, scope)
	}
	if got := f.regionsOf("GET", "/detector/d-.*"); !reflect.DeepEqual(got, scope) {
		t.Errorf("GetDetector SigV4 regions = %v, want %v", got, scope)
	}
	var regions []string
	for _, d := range detailsOf(results) {
		regions = append(regions, d["Region"].(string))
	}
	if !reflect.DeepEqual(regions, scope) {
		t.Errorf("result regions = %v, want %v", regions, scope)
	}
}
