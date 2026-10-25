package appsync

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	appsynctypes "github.com/aws/aws-sdk-go-v2/service/appsync/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

const (
	wantName     = "appsync:ListGraphqlApis"
	wantService  = "AppSync"
	wantResource = "graphql-api"
	deniedCode   = "UnauthorizedException"
	deniedMsg    = "User is not authorized to perform: appsync:ListGraphqlApis"
)

var credScope = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/([^/]+)/aws4_request`)

type request struct {
	method, path, region, signingName string
}

// fakeAppSync speaks the REST-JSON protocol for appsync:ListGraphqlApis. Data
// is keyed by the SigV4 credential-scope region; a region in deny answers with
// an error, a region without data returns an empty list.
type fakeAppSync struct {
	apis map[string]string // region -> JSON array of graphqlApis
	deny map[string]bool

	mu   sync.Mutex
	reqs []request
}

func (f *fakeAppSync) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, _ = io.Copy(io.Discard, r.Body)
	req := request{method: r.Method, path: r.URL.Path}
	if m := credScope.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		req.region, req.signingName = m[1], m[2]
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, req)
	f.mu.Unlock()

	if f.deny[req.region] {
		w.Header().Set("X-Amzn-Errortype", deniedCode)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"message":"`+deniedMsg+`"}`)
		return
	}
	list := f.apis[req.region]
	if list == "" {
		list = "[]"
	}
	w.Header().Set("Content-Type", "application/json")
	// The continuation token must be ignored, as v1 ignored it.
	_, _ = io.WriteString(w, `{"graphqlApis":`+list+`,"nextToken":"more"}`)
}

func (f *fakeAppSync) requests() []request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]request(nil), f.reqs...)
}

func (f *fakeAppSync) regions() []string {
	var out []string
	for _, r := range f.requests() {
		out = append(out, r.region)
	}
	return out
}

func newEnv(t *testing.T, f *fakeAppSync) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_APPSYNC", unreachable)

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
	svc := AppSyncCalls[0]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

func names(results []types.ScanResult) []string {
	var out []string
	for _, r := range results {
		out = append(out, r.ResourceName)
	}
	return out
}

func TestServiceIsV2Only(t *testing.T) {
	if len(AppSyncCalls) != 1 {
		t.Fatalf("expected 1 service, got %d", len(AppSyncCalls))
	}
	svc := AppSyncCalls[0]
	if svc.Name != wantName {
		t.Errorf("name = %q, want %q", svc.Name, wantName)
	}
	if svc.CallV2 == nil || svc.Call != nil {
		t.Errorf("want CallV2 set and Call nil, got CallV2=%v Call=%v", svc.CallV2 != nil, svc.Call != nil)
	}
}

func TestInvokePopulatedAndSparse(t *testing.T) {
	f := &fakeAppSync{apis: map[string]string{
		"us-west-2": `[{"name":"orders-api","apiId":"a1"},{"apiId":"a2"}]`,
	}}
	ctx := scoped(newEnv(t, f), "us-west-2")
	results, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// A sparse API (no name) maps to an empty ResourceName.
	if want := []string{"orders-api", ""}; !reflect.DeepEqual(names(results), want) {
		t.Fatalf("names = %q, want %q", names(results), want)
	}
	for i, r := range results {
		if r.ServiceName != wantService || r.MethodName != wantName || r.ResourceType != wantResource ||
			r.Error != nil || r.Timestamp.IsZero() {
			t.Errorf("result %d = %+v", i, r)
		}
		if !reflect.DeepEqual(r.Details, map[string]interface{}{}) {
			t.Errorf("result %d Details = %#v, want empty map", i, r.Details)
		}
	}

	reqs := f.requests()
	if len(reqs) != 1 {
		t.Fatalf("requests = %+v, want exactly 1 (v1 ignored the continuation token)", reqs)
	}
	r := reqs[0]
	if r.region != "us-west-2" || r.signingName != "appsync" {
		t.Errorf("signed for region %q service %q, want us-west-2/appsync", r.region, r.signingName)
	}
	if r.method != "GET" || r.path != "/v1/apis" {
		t.Errorf("request = %s %s, want GET /v1/apis", r.method, r.path)
	}
}

func TestMultiRegionLoop(t *testing.T) {
	f := &fakeAppSync{apis: map[string]string{
		"us-east-1": `[{"name":"east1"}]`,
		"us-west-2": `[{"name":"west2"}]`,
	}}
	ctx := newEnv(t, f)
	regions := types.RegionsFor(ctx)
	if len(regions) < 3 {
		t.Fatalf("default regions = %v", regions)
	}
	results, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.regions(); !reflect.DeepEqual(got, regions) {
		t.Errorf("signed regions = %v, want %v (in order, once each)", got, regions)
	}
	// Results follow region order.
	var want []string
	for _, r := range regions {
		switch r {
		case "us-east-1":
			want = append(want, "east1")
		case "us-west-2":
			want = append(want, "west2")
		}
	}
	if !reflect.DeepEqual(names(results), want) {
		t.Errorf("names = %v, want %v", names(results), want)
	}
}

func TestRegionScope(t *testing.T) {
	f := &fakeAppSync{apis: map[string]string{
		"us-east-1": `[{"name":"east"}]`,
		"eu-west-1": `[{"name":"eu"}]`,
	}}
	ctx, scope := types.WithRegionScope(newEnv(t, f), []string{"eu-west-1"})
	results, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !scope.Used() {
		t.Error("service did not consult the region scope")
	}
	if got := f.regions(); !reflect.DeepEqual(got, []string{"eu-west-1"}) {
		t.Errorf("signed regions = %v, want only eu-west-1", got)
	}
	if want := []string{"eu"}; !reflect.DeepEqual(names(results), want) {
		t.Errorf("names = %v, want %v", names(results), want)
	}
}

func TestInvokeEmpty(t *testing.T) {
	ctx := scoped(newEnv(t, &fakeAppSync{}), "us-west-2")
	if results, err := run(ctx); err != nil || len(results) != 0 {
		t.Errorf("results = %+v, err = %v", results, err)
	}
}

func TestAccessDeniedNormalizedAbortsLoop(t *testing.T) {
	f := &fakeAppSync{
		apis: map[string]string{"us-east-1": `[{"name":"east"}]`, "eu-west-1": `[{"name":"eu"}]`},
		deny: map[string]bool{"us-west-2": true},
	}
	ctx := scoped(newEnv(t, f), "us-east-1", "us-west-2", "eu-west-1")
	svc := AppSyncCalls[0]
	out, err := svc.Invoke(ctx, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if want := deniedCode + ": " + deniedMsg; err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if code := awsv2.ErrorCode(err); code != deniedCode {
		t.Errorf("ErrorCode = %q", code)
	}
	for _, noise := range []string{"status code", "request id", "operation error", "401"} {
		if strings.Contains(strings.ToLower(err.Error()), noise) {
			t.Errorf("error %q contains transport noise %q", err, noise)
		}
	}
	// v1 returned on the first failing region: nothing from earlier regions, no later request.
	if got := f.regions(); !reflect.DeepEqual(got, []string{"us-east-1", "us-west-2"}) {
		t.Errorf("signed regions = %v, want loop to stop at us-west-2", got)
	}
	results := svc.Process(out, err, false)
	if len(results) != 1 || results[0].Error == nil || results[0].ServiceName != wantService ||
		results[0].MethodName != wantName || results[0].ResourceName != "" {
		t.Fatalf("results = %+v", results)
	}
}

func TestProcessTypedOutput(t *testing.T) {
	process := AppSyncCalls[0].Process
	tests := []struct {
		name   string
		output interface{}
		want   []string
	}{
		{"populated and sparse", []appsynctypes.GraphqlApi{
			{Name: aws.String("orders-api"), ApiId: aws.String("a1")},
			{ApiId: aws.String("a2")},
		}, []string{"orders-api", ""}},
		{"empty", []appsynctypes.GraphqlApi{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, r := range process(tt.output, nil, false) {
				if r.ServiceName != wantService || r.MethodName != wantName || r.ResourceType != wantResource ||
					r.Error != nil || !reflect.DeepEqual(r.Details, map[string]interface{}{}) {
					t.Errorf("unexpected result %+v", r)
				}
				got = append(got, r.ResourceName)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("names = %q, want %q", got, tt.want)
			}
		})
	}

	errIn := fmt.Errorf("AccessDeniedException: nope")
	results := process(nil, errIn, false)
	if len(results) != 1 || results[0].Error != errIn || results[0].ServiceName != wantService || results[0].MethodName != wantName {
		t.Errorf("error results = %+v", results)
	}

	for _, out := range []interface{}{nil, "x", 3, []string{"nope"}} {
		if got := process(out, nil, false); len(got) != 0 {
			t.Errorf("unexpected output %v produced %+v", out, got)
		}
	}
}
