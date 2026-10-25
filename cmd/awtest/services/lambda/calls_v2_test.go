package lambda

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/lambda/aws4_request`)

// fnSpec is a function returned by the fake; cfg holds the configuration
// document (keys as on the wire), so a spec with only a name is sparse.
type fnSpec struct {
	name     string
	cfg      map[string]interface{}
	location string
}

type recorded struct{ region, method, path string }

// fakeLambda speaks the Lambda REST-JSON protocol for ListFunctions,
// GetFunction and GetFunctionConfiguration. Data is keyed by the SigV4
// credential-scope region.
type fakeLambda struct {
	fns map[string][]fnSpec
	// denyList maps region -> message for an AccessDeniedException on ListFunctions.
	denyList map[string]string
	// denyGet maps "region/function" -> message for GetFunction.
	denyGet map[string]string

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeLambda) fail(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("X-Amzn-Errortype", code)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"Message": msg})
}

func (f *fakeLambda) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, r.Method, r.URL.Path})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	const base = "/2015-03-31/functions"
	rest := strings.TrimPrefix(r.URL.Path, base)
	if r.Method != http.MethodGet || rest == r.URL.Path {
		f.fail(w, http.StatusNotFound, "ResourceNotFoundException", "no route "+r.URL.Path)
		return
	}
	rest = strings.Trim(rest, "/")
	switch {
	case rest == "":
		if msg, ok := f.denyList[region]; ok {
			f.fail(w, http.StatusForbidden, "AccessDeniedException", msg)
			return
		}
		var list []map[string]interface{}
		for _, fn := range f.fns[region] {
			list = append(list, map[string]interface{}{"FunctionName": fn.name})
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"Functions": list})
	default:
		name, isConfig := strings.CutSuffix(rest, "/configuration")
		var spec *fnSpec
		for i := range f.fns[region] {
			if f.fns[region][i].name == name {
				spec = &f.fns[region][i]
			}
		}
		if spec == nil {
			f.fail(w, http.StatusNotFound, "ResourceNotFoundException", "Function not found: "+name)
			return
		}
		cfg := map[string]interface{}{"FunctionName": spec.name}
		for k, v := range spec.cfg {
			cfg[k] = v
		}
		if isConfig {
			_ = json.NewEncoder(w).Encode(cfg)
			return
		}
		if msg, ok := f.denyGet[region+"/"+name]; ok {
			f.fail(w, http.StatusForbidden, "AccessDeniedException", msg)
			return
		}
		out := map[string]interface{}{"Configuration": cfg}
		if spec.location != "" {
			out["Code"] = map[string]string{"Location": spec.location}
		}
		_ = json.NewEncoder(w).Encode(out)
	}
}

func (f *fakeLambda) ops() []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recorded(nil), f.reqs...)
}

func newEnv(t *testing.T, f *fakeLambda) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_LAMBDA", unreachable)

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
	return types.WithAWSConfig(context.Background(), cfg)
}

func scoped(ctx context.Context, regions ...string) context.Context {
	ctx, _ = types.WithRegionScope(ctx, regions)
	return ctx
}

func run(ctx context.Context) ([]types.ScanResult, error) {
	svc := LambdaCalls[0]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

func TestServiceIsV2Only(t *testing.T) {
	if len(LambdaCalls) != 1 || LambdaCalls[0].Name != "lambda:ListFunctions" {
		t.Fatalf("unexpected services: %+v", LambdaCalls)
	}
	if LambdaCalls[0].CallV2 == nil || LambdaCalls[0].Call != nil {
		t.Error("want CallV2 set and Call nil")
	}
}

func TestInvokeProcessDetails(t *testing.T) {
	f := &fakeLambda{fns: map[string][]fnSpec{"us-west-2": {
		{
			name:     "full",
			location: "https://code.example/full.zip",
			cfg: map[string]interface{}{
				"MemorySize": 256, "Timeout": 30, "Runtime": "python3.12", "Handler": "app.handler",
				"Role": "arn:aws:iam::123456789012:role/fn", "LastModified": "2024-01-02T03:04:05.000+0000",
				"Description": "does things", "KMSKeyArn": "arn:aws:kms:us-west-2:123456789012:key/abc",
				"Environment": map[string]interface{}{"Variables": map[string]string{"A": "1", "B": "two"}},
			},
		},
		{name: "sparse"},
	}}}
	results, err := run(scoped(newEnv(t, f), "us-west-2"))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results: %+v", len(results), results)
	}
	want := []map[string]interface{}{
		{
			"region": "us-west-2", "runtime": "python3.12", "memory_size": "256 MB", "timeout": "30 seconds",
			"handler": "app.handler", "role": "arn:aws:iam::123456789012:role/fn",
			"last_modified": "2024-01-02T03:04:05.000+0000", "code_location": "https://code.example/full.zip",
			"description": "does things", "kms_key_arn": "arn:aws:kms:us-west-2:123456789012:key/abc",
			"environment_variables": map[string]string{"A": "1", "B": "two"},
		},
		{
			"region": "us-west-2", "runtime": "", "memory_size": "", "timeout": "", "handler": "",
			"role": "", "last_modified": "", "code_location": "",
		},
	}
	for i, name := range []string{"full", "sparse"} {
		r := results[i]
		if r.ServiceName != "Lambda" || r.MethodName != "lambda:ListFunctions" || r.ResourceType != "function" ||
			r.ResourceName != name || r.Error != nil || r.Timestamp.IsZero() {
			t.Errorf("unexpected result %+v", r)
		}
		if !reflect.DeepEqual(r.Details, want[i]) {
			t.Errorf("%s details = %#v, want %#v", name, r.Details, want[i])
		}
	}
}

func TestEveryRegionInScopeIsVisited(t *testing.T) {
	f := &fakeLambda{fns: map[string][]fnSpec{
		"us-east-1": {{name: "e"}}, "eu-west-1": {{name: "w"}}, "ap-south-1": {{name: "a"}},
	}}
	results, err := run(scoped(newEnv(t, f), "us-east-1", "eu-west-1", "ap-south-1"))
	if err != nil {
		t.Fatal(err)
	}
	var lists []string
	for _, r := range f.ops() {
		if r.path == "/2015-03-31/functions" || r.path == "/2015-03-31/functions/" {
			lists = append(lists, r.region)
		}
	}
	if !reflect.DeepEqual(lists, []string{"us-east-1", "eu-west-1", "ap-south-1"}) {
		t.Errorf("ListFunctions regions = %v", lists)
	}
	// Per-function follow-ups are signed for the function's own region.
	for _, r := range f.ops() {
		if !strings.HasPrefix(r.path, "/2015-03-31/functions/") || r.path == "/2015-03-31/functions/" {
			continue // the ListFunctions request
		}
		fn := strings.Split(strings.TrimPrefix(r.path, "/2015-03-31/functions/"), "/")[0]
		if want := map[string]string{"e": "us-east-1", "w": "eu-west-1", "a": "ap-south-1"}[fn]; want != r.region {
			t.Errorf("%s signed for %s, want %s", r.path, r.region, want)
		}
	}
	if n := len(f.ops()); n != 9 {
		t.Errorf("expected 9 requests (list + get + config per region), got %d", n)
	}
	var names []string
	for _, r := range results {
		names = append(names, r.ResourceName)
		if r.Details["region"] == "" {
			t.Errorf("missing region in details of %s", r.ResourceName)
		}
	}
	if !reflect.DeepEqual(names, []string{"e", "w", "a"}) {
		t.Errorf("resources = %v", names)
	}
}

func TestListFailureAbortsWithNormalizedError(t *testing.T) {
	f := &fakeLambda{
		fns:      map[string][]fnSpec{"us-east-1": {{name: "e"}}},
		denyList: map[string]string{"eu-west-1": "list denied"},
	}
	results, err := run(scoped(newEnv(t, f), "us-east-1", "eu-west-1", "us-west-2"))
	if err == nil {
		t.Fatal("expected error")
	}
	if got, want := err.Error(), "AccessDeniedException: list denied"; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
	if len(results) != 1 || results[0].Error == nil || results[0].ServiceName != "Lambda" {
		t.Errorf("results = %+v", results)
	}
	for _, r := range f.ops() {
		if r.region == "us-west-2" {
			t.Errorf("region after the failure was still queried: %v", f.ops())
		}
	}
}

func TestGetFunctionFailureAbortsScan(t *testing.T) {
	f := &fakeLambda{
		fns:     map[string][]fnSpec{"us-west-2": {{name: "a"}, {name: "b"}}},
		denyGet: map[string]string{"us-west-2/b": "get denied"},
	}
	_, err := run(scoped(newEnv(t, f), "us-west-2"))
	if err == nil || err.Error() != "AccessDeniedException: get denied" {
		t.Errorf("error = %v", err)
	}
}
