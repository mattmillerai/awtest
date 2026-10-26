package apigateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"sort"
	"sync"
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/apigateway/aws4_request`)

type recorded struct {
	region string
	method string
	path   string
}

// fakeAPIGW speaks the API Gateway restJson1 protocol. handler returns the
// HTTP status, the JSON body and (for errors) the error code.
type fakeAPIGW struct {
	handler func(r recorded) (status int, body interface{}, errCode string)

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeAPIGW) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	rec := recorded{region: region, method: r.Method, path: r.URL.Path}
	f.mu.Lock()
	f.reqs = append(f.reqs, rec)
	f.mu.Unlock()

	status, out, code := f.handler(rec)
	w.Header().Set("Content-Type", "application/json")
	if code != "" {
		w.Header().Set("X-Amzn-Errortype", code)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(out)
}

func (f *fakeAPIGW) calls(path string) []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []recorded
	for _, r := range f.reqs {
		if r.path == path {
			out = append(out, r)
		}
	}
	return out
}

type items = map[string]interface{}

func list(v ...map[string]interface{}) items {
	if v == nil {
		v = []map[string]interface{}{}
	}
	return items{"item": v}
}

func newEnv(t *testing.T, f *fakeAPIGW, regions ...string) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_API_GATEWAY", unreachable)

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

func run(t *testing.T, ctx context.Context, i int) ([]types.ScanResult, error) {
	t.Helper()
	utils.Quiet = true
	t.Cleanup(func() { utils.Quiet = false })
	svc := APIGatewayCalls[i]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

const (
	svcRestApis = 0
	svcKeys     = 1
	svcDomains  = 2
)

func TestServicesAreV2Only(t *testing.T) {
	want := []string{"apigateway:RestApis", "apigateway:GetApiKeys", "apigateway:GetDomainNames"}
	if len(APIGatewayCalls) != len(want) {
		t.Fatalf("got %d services, want %d", len(APIGatewayCalls), len(want))
	}
	for i, svc := range APIGatewayCalls {
		if svc.Name != want[i] {
			t.Errorf("service %d = %q, want %q", i, svc.Name, want[i])
		}
		if svc.CallV2 == nil || svc.Call != nil {
			t.Errorf("%s must implement CallV2 only", svc.Name)
		}
	}
}

type row struct{ method, rtype, name string }

func rows(rs []types.ScanResult) []row {
	var out []row
	for _, r := range rs {
		out = append(out, row{r.MethodName, r.ResourceType, r.ResourceName})
	}
	return out
}

func TestRestApisFunctional(t *testing.T) {
	f := &fakeAPIGW{handler: func(r recorded) (int, interface{}, string) {
		switch r.path {
		case "/restapis":
			return 200, list(items{"id": "a1", "name": "demo"}), ""
		case "/restapis/a1/stages":
			return 200, list(items{"stageName": "prod"}, items{}), ""
		case "/restapis/a1/models":
			return 200, list(items{"name": "Pet"}, items{}), ""
		case "/restapis/a1/resources":
			return 200, list(
				items{"id": "root", "path": "/"},
				items{"id": "r1", "path": "/pets", "resourceMethods": items{"GET": items{}, "POST": items{}}},
			), ""
		case "/restapis/a1/resources/r1/methods/GET":
			return 200, items{"requestParameters": items{"method.request.querystring.q": true, "method.request.header.x": false}}, ""
		case "/restapis/a1/resources/r1/methods/POST":
			return 200, items{}, ""
		case "/restapis/a1/resources/r1/methods/GET/integration":
			return 200, items{"type": "MOCK"}, ""
		case "/restapis/a1/resources/r1/methods/POST/integration":
			// A missing integration is swallowed (v1 `continue`).
			return 404, items{"message": "no integration"}, "NotFoundException"
		}
		return 404, items{"message": "unexpected " + r.path}, "NotFoundException"
	}}
	ctx := newEnv(t, f, "eu-central-1")
	res, err := run(t, ctx, svcRestApis)
	if err != nil {
		t.Fatal(err)
	}
	want := []row{
		{"apigateway:RestApis", "rest-api", "demo"},
		{"apigateway:GetResources", "resource", "/"},
		{"apigateway:GetResources", "resource", "/pets"},
		{"apigateway:GetStages", "stage", "prod"},
		{"apigateway:GetStages", "stage", ""},
		{"apigateway:GetModels", "model", "Pet"},
		{"apigateway:GetModels", "model", ""},
	}
	// The stage and model without a name are the sparse resources: their
	// nil names read as "".
	if got := rows(res); !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %+v", got)
	}
	for _, r := range res {
		if r.ServiceName != "APIGateway" || r.Details == nil || len(r.Details) != 0 || r.Error != nil {
			t.Errorf("unexpected result shape: %+v", r)
		}
	}
	for _, p := range []string{"/restapis/a1/resources/r1/methods/GET", "/restapis/a1/resources/r1/methods/POST", "/restapis/a1/resources/r1/methods/GET/integration", "/restapis/a1/resources/r1/methods/POST/integration"} {
		if n := len(f.calls(p)); n != 1 {
			t.Errorf("%s calls = %d, want 1", p, n)
		}
	}
}

func TestRestApisErrorsAbort(t *testing.T) {
	f := &fakeAPIGW{handler: func(r recorded) (int, interface{}, string) {
		switch r.path {
		case "/restapis":
			return 200, list(items{"id": "a1", "name": "demo"}), ""
		case "/restapis/a1/stages":
			return 403, items{"message": "stages denied"}, "AccessDeniedException"
		}
		return 200, list(), ""
	}}
	res, err := run(t, newEnv(t, f, "us-east-1"), svcRestApis)
	const want = "AccessDeniedException: stages denied"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
	if len(res) != 1 || res[0].Error == nil || res[0].Error.Error() != want || res[0].MethodName != "apigateway:RestApis" {
		t.Errorf("results = %+v", res)
	}
}

func TestApiKeysAndDomainNamesFunctional(t *testing.T) {
	f := &fakeAPIGW{handler: func(r recorded) (int, interface{}, string) {
		switch r.path {
		case "/apikeys":
			return 200, list(items{"id": "k1"}, items{}), ""
		case "/domainnames":
			return 200, list(items{"domainName": "api.example.com"}, items{}), ""
		}
		return 404, items{"message": r.path}, "NotFoundException"
	}}
	ctx := newEnv(t, f, "us-east-1", "eu-west-1")
	keys, err := run(t, ctx, svcKeys)
	if err != nil {
		t.Fatal(err)
	}
	wantKeys := []row{
		{"apigateway:GetApiKeys", "api-key", "k1"}, {"apigateway:GetApiKeys", "api-key", ""},
		{"apigateway:GetApiKeys", "api-key", "k1"}, {"apigateway:GetApiKeys", "api-key", ""},
	}
	if got := rows(keys); !reflect.DeepEqual(got, wantKeys) {
		t.Errorf("keys = %+v", got)
	}
	doms, err := run(t, ctx, svcDomains)
	if err != nil {
		t.Fatal(err)
	}
	wantDoms := []row{
		{"apigateway:GetDomainNames", "domain-name", "api.example.com"}, {"apigateway:GetDomainNames", "domain-name", ""},
		{"apigateway:GetDomainNames", "domain-name", "api.example.com"}, {"apigateway:GetDomainNames", "domain-name", ""},
	}
	if got := rows(doms); !reflect.DeepEqual(got, wantDoms) {
		t.Errorf("domains = %+v", got)
	}
	for _, r := range append(keys, doms...) {
		if r.Details == nil || len(r.Details) != 0 {
			t.Errorf("details = %#v, want empty map", r.Details)
		}
	}

	// Empty lists produce no results (only the access-granted message).
	empty := &fakeAPIGW{handler: func(recorded) (int, interface{}, string) { return 200, list(), "" }}
	for _, i := range []int{svcKeys, svcDomains} {
		if res, err := run(t, newEnv(t, empty, "us-east-1"), i); err != nil || len(res) != 0 {
			t.Errorf("%s empty: res=%+v err=%v", APIGatewayCalls[i].Name, res, err)
		}
	}

	denied := &fakeAPIGW{handler: func(recorded) (int, interface{}, string) {
		return 403, items{"message": "nope"}, "AccessDeniedException"
	}}
	for _, i := range []int{svcRestApis, svcKeys, svcDomains} {
		res, err := run(t, newEnv(t, denied, "us-east-1"), i)
		if err == nil || err.Error() != "AccessDeniedException: nope" {
			t.Errorf("%s: err = %v", APIGatewayCalls[i].Name, err)
		}
		if len(res) != 1 || res[0].Error == nil || res[0].Error.Error() != "AccessDeniedException: nope" {
			t.Errorf("%s: results = %+v", APIGatewayCalls[i].Name, res)
		}
	}
}

func TestEveryScopeRegionIsVisitedWithItsSigningRegion(t *testing.T) {
	scope := []string{"us-east-1", "eu-west-1", "ap-southeast-2"}
	f := &fakeAPIGW{handler: func(recorded) (int, interface{}, string) { return 200, list(), "" }}
	ctx := newEnv(t, f, scope...)
	for i := range APIGatewayCalls {
		if _, err := run(t, ctx, i); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"/restapis", "/apikeys", "/domainnames"} {
		var got []string
		for _, r := range f.calls(path) {
			got = append(got, r.region)
		}
		sort.Strings(got)
		want := append([]string(nil), scope...)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s visited %v, want %v", path, got, want)
		}
	}
}
