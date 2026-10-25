package amplify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	amplifytypes "github.com/aws/aws-sdk-go-v2/service/amplify/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

const (
	wantName    = "amplify:ListApps"
	wantService = "Amplify"
	deniedCode  = "UnauthorizedException"
	deniedMsg   = "User is not authorized to perform: amplify:ListApps"
)

var credScope = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/([^/]+)/aws4_request`)

type request struct {
	method, path, region, signingName string
}

// fakeAmplify speaks the REST-JSON protocol for amplify:ListApps and
// amplify:ListBranches. Data is keyed by the SigV4 credential-scope region.
type fakeAmplify struct {
	apps     map[string]string // region -> JSON array of apps
	branches map[string]string // "region/appId" -> JSON array of branches
	// deny maps region -> true to fail ListApps; denyBranches maps
	// "region/appId" -> true to fail ListBranches.
	deny         map[string]bool
	denyBranches map[string]bool

	mu   sync.Mutex
	reqs []request
}

func (f *fakeAmplify) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, _ = io.Copy(io.Discard, r.Body)
	req := request{method: r.Method, path: r.URL.Path}
	if m := credScope.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		req.region, req.signingName = m[1], m[2]
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, req)
	f.mu.Unlock()

	denied := func() {
		w.Header().Set("X-Amzn-Errortype", deniedCode)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"message":"`+deniedMsg+`"}`)
	}
	w.Header().Set("Content-Type", "application/json")
	switch {
	case req.path == "/apps":
		if f.deny[req.region] {
			denied()
			return
		}
		list := f.apps[req.region]
		if list == "" {
			list = "[]"
		}
		// The continuation token must be ignored, as v1 ignored it.
		_, _ = io.WriteString(w, `{"apps":`+list+`,"nextToken":"more"}`)
	case strings.HasPrefix(req.path, "/apps/") && strings.HasSuffix(req.path, "/branches"):
		key := req.region + "/" + strings.TrimSuffix(strings.TrimPrefix(req.path, "/apps/"), "/branches")
		if f.denyBranches[key] {
			denied()
			return
		}
		list := f.branches[key]
		if list == "" {
			list = "[]"
		}
		_, _ = io.WriteString(w, `{"branches":`+list+`,"nextToken":"more"}`)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakeAmplify) requests() []request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]request(nil), f.reqs...)
}

func (f *fakeAmplify) regions(path string) []string {
	var out []string
	for _, r := range f.requests() {
		if r.path == path {
			out = append(out, r.region)
		}
	}
	return out
}

func newEnv(t *testing.T, f *fakeAmplify) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_AMPLIFY", unreachable)

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
	svc := AmplifyCalls[0]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

type named struct{ typ, name string }

func summary(results []types.ScanResult) []named {
	var out []named
	for _, r := range results {
		out = append(out, named{r.ResourceType, r.ResourceName})
	}
	return out
}

func TestServiceIsV2Only(t *testing.T) {
	if len(AmplifyCalls) != 1 {
		t.Fatalf("expected 1 service, got %d", len(AmplifyCalls))
	}
	svc := AmplifyCalls[0]
	if svc.Name != wantName {
		t.Errorf("name = %q, want %q", svc.Name, wantName)
	}
	if svc.CallV2 == nil || svc.Call != nil {
		t.Errorf("want CallV2 set and Call nil, got CallV2=%v Call=%v", svc.CallV2 != nil, svc.Call != nil)
	}
}

func TestInvokePopulatedAndSparse(t *testing.T) {
	f := &fakeAmplify{
		apps: map[string]string{
			"us-west-2": `[{"appId":"a1","name":"web"},{"appId":"a2"}]`,
		},
		branches: map[string]string{
			"us-west-2/a1": `[{"branchName":"main"},{}]`,
		},
	}
	ctx := scoped(newEnv(t, f), "us-west-2")
	results, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// A sparse app / branch (no name) maps to an empty ResourceName.
	want := []named{{"app", "web"}, {"branch", "main"}, {"branch", ""}, {"app", ""}}
	if got := summary(results); !reflect.DeepEqual(got, want) {
		t.Fatalf("results = %q, want %q", got, want)
	}
	for i, r := range results {
		if r.ServiceName != wantService || r.MethodName != wantName || r.Error != nil || r.Timestamp.IsZero() {
			t.Errorf("result %d = %+v", i, r)
		}
		if !reflect.DeepEqual(r.Details, map[string]interface{}{}) {
			t.Errorf("result %d Details = %#v, want empty map", i, r.Details)
		}
	}

	reqs := f.requests()
	wantReqs := []request{
		{"GET", "/apps", "us-west-2", "amplify"},
		{"GET", "/apps/a1/branches", "us-west-2", "amplify"},
		{"GET", "/apps/a2/branches", "us-west-2", "amplify"},
	}
	if !reflect.DeepEqual(reqs, wantReqs) {
		t.Errorf("requests = %+v, want %+v (v1 ignored the continuation tokens)", reqs, wantReqs)
	}
}

func TestMultiRegionLoop(t *testing.T) {
	f := &fakeAmplify{
		apps: map[string]string{
			"us-east-1": `[{"appId":"e1","name":"east1"}]`,
			"us-west-2": `[{"appId":"w1","name":"west2"}]`,
		},
		branches: map[string]string{
			"us-east-1/e1": `[{"branchName":"be"}]`,
			"us-west-2/w1": `[{"branchName":"bw"}]`,
		},
	}
	ctx := newEnv(t, f)
	regions := types.RegionsFor(ctx)
	if len(regions) < 3 {
		t.Fatalf("default regions = %v", regions)
	}
	results, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.regions("/apps"); !reflect.DeepEqual(got, regions) {
		t.Errorf("signed regions = %v, want %v (in order, once each)", got, regions)
	}
	// Follow-up calls are signed for the region of the app they belong to.
	if got := f.regions("/apps/e1/branches"); !reflect.DeepEqual(got, []string{"us-east-1"}) {
		t.Errorf("branches for e1 signed for %v", got)
	}
	if got := f.regions("/apps/w1/branches"); !reflect.DeepEqual(got, []string{"us-west-2"}) {
		t.Errorf("branches for w1 signed for %v", got)
	}
	var want []named
	for _, r := range regions {
		switch r {
		case "us-east-1":
			want = append(want, named{"app", "east1"}, named{"branch", "be"})
		case "us-west-2":
			want = append(want, named{"app", "west2"}, named{"branch", "bw"})
		}
	}
	if got := summary(results); !reflect.DeepEqual(got, want) {
		t.Errorf("results = %v, want %v", got, want)
	}
}

func TestRegionScope(t *testing.T) {
	f := &fakeAmplify{apps: map[string]string{
		"us-east-1": `[{"appId":"e","name":"east"}]`,
		"eu-west-1": `[{"appId":"u","name":"eu"}]`,
	}}
	ctx, scope := types.WithRegionScope(newEnv(t, f), []string{"eu-west-1"})
	results, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !scope.Used() {
		t.Error("service did not consult the region scope")
	}
	for _, r := range f.requests() {
		if r.region != "eu-west-1" {
			t.Errorf("request %s signed for %s, want only eu-west-1", r.path, r.region)
		}
	}
	if got, want := summary(results), []named{{"app", "eu"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("results = %v, want %v", got, want)
	}
}

func TestInvokeEmpty(t *testing.T) {
	ctx := scoped(newEnv(t, &fakeAmplify{}), "us-west-2")
	if results, err := run(ctx); err != nil || len(results) != 0 {
		t.Errorf("results = %+v, err = %v", results, err)
	}
}

func assertNormalized(t *testing.T, err error) {
	t.Helper()
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
}

func TestListAppsDeniedAbortsLoop(t *testing.T) {
	f := &fakeAmplify{
		apps: map[string]string{"us-east-1": `[{"appId":"e","name":"east"}]`, "eu-west-1": `[{"appId":"u","name":"eu"}]`},
		deny: map[string]bool{"us-west-2": true},
	}
	ctx := scoped(newEnv(t, f), "us-east-1", "us-west-2", "eu-west-1")
	svc := AmplifyCalls[0]
	out, err := svc.Invoke(ctx, nil)
	assertNormalized(t, err)
	// v1 returned on the first failing region: nothing from earlier regions, no later request.
	if got := f.regions("/apps"); !reflect.DeepEqual(got, []string{"us-east-1", "us-west-2"}) {
		t.Errorf("signed regions = %v, want loop to stop at us-west-2", got)
	}
	results := svc.Process(out, err, false)
	if len(results) != 1 || results[0].Error == nil || results[0].ServiceName != wantService ||
		results[0].MethodName != wantName || results[0].ResourceName != "" {
		t.Fatalf("results = %+v", results)
	}
}

func TestListBranchesErrorAbortsEverything(t *testing.T) {
	f := &fakeAmplify{
		apps: map[string]string{
			"us-east-1": `[{"appId":"a1","name":"one"},{"appId":"a2","name":"two"},{"appId":"a3","name":"three"}]`,
			"eu-west-1": `[{"appId":"u1","name":"eu"}]`,
		},
		denyBranches: map[string]bool{"us-east-1/a2": true},
	}
	ctx := scoped(newEnv(t, f), "us-east-1", "eu-west-1")
	svc := AmplifyCalls[0]
	out, err := svc.Invoke(ctx, nil)
	assertNormalized(t, err)
	if out != nil {
		t.Errorf("output = %#v, want nil: v1 discarded the partial result", out)
	}
	var paths []string
	for _, r := range f.requests() {
		paths = append(paths, r.path)
	}
	if want := []string{"/apps", "/apps/a1/branches", "/apps/a2/branches"}; !reflect.DeepEqual(paths, want) {
		t.Errorf("requests = %v, want %v (no later app or region after the failure)", paths, want)
	}
}

func TestProcessTypedOutput(t *testing.T) {
	process := AmplifyCalls[0].Process
	out := []AppWithBranches{
		{
			App:      amplifytypes.App{Name: aws.String("web")},
			Branches: []amplifytypes.Branch{{BranchName: aws.String("main")}, {}},
			Region:   "us-west-2",
		},
		{App: amplifytypes.App{}},
	}
	want := []named{{"app", "web"}, {"branch", "main"}, {"branch", ""}, {"app", ""}}
	if got := summary(process(out, nil, false)); !reflect.DeepEqual(got, want) {
		t.Errorf("results = %v, want %v", got, want)
	}
	if got := process([]AppWithBranches{}, nil, false); len(got) != 0 {
		t.Errorf("empty output produced %+v", got)
	}
	for _, o := range []interface{}{nil, "x", 3, []string{"nope"}} {
		if got := process(o, nil, false); len(got) != 0 {
			t.Errorf("unexpected output %v produced %+v", o, got)
		}
	}
}
