package stepfunctions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

const (
	wantName    = "states:ListStateMachines"
	wantService = "Step Functions"
	deniedCode  = "AccessDeniedException"
	deniedMsg   = "User is not authorized to perform: states:ListStateMachines"
)

var credScope = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/([^/]+)/aws4_request`)

type request struct {
	target, region, signingName string
	token                       string
}

// fakeSFN speaks the JSON-1.0 protocol for states:ListStateMachines. Data is
// keyed by the SigV4 credential-scope region.
type fakeSFN struct {
	machines map[string][]string // region -> JSON objects of stateMachines
	// pageSize > 0 paginates with tokens "tok-<region>-<n>" (n = index of the
	// next item).
	pageSize int
	// deny maps region -> true when every call in the region fails;
	// denyPage maps region -> page index (0-based) from which calls fail.
	deny     map[string]bool
	denyPage map[string]int

	mu   sync.Mutex
	reqs []request
}

func (f *fakeSFN) fail(w http.ResponseWriter, msg string) {
	w.WriteHeader(http.StatusBadRequest)
	_, _ = io.WriteString(w, `{"__type":"`+deniedCode+`","message":"`+msg+`"}`)
}

func (f *fakeSFN) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	body := map[string]interface{}{}
	_ = json.Unmarshal(raw, &body)
	req := request{target: r.Header.Get("X-Amz-Target")}
	req.token, _ = body["nextToken"].(string)
	if m := credScope.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		req.region, req.signingName = m[1], m[2]
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, req)
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	items := f.machines[req.region]
	start, end := 0, len(items)
	if req.token != "" {
		start, _ = strconv.Atoi(strings.TrimPrefix(req.token, "tok-"+req.region+"-"))
	}
	page := 0
	if f.pageSize > 0 {
		page = start / f.pageSize
	}
	if f.deny[req.region] {
		f.fail(w, deniedMsg)
		return
	}
	if p, ok := f.denyPage[req.region]; ok && page >= p {
		f.fail(w, deniedMsg)
		return
	}
	next := ""
	if f.pageSize > 0 && start+f.pageSize < end {
		end = start + f.pageSize
		next = fmt.Sprintf(`,"nextToken":"tok-%s-%d"`, req.region, end)
	}
	_, _ = io.WriteString(w, `{"stateMachines":[`+strings.Join(items[start:end], ",")+`]`+next+`}`)
}

func (f *fakeSFN) requests() []request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]request(nil), f.reqs...)
}

func (f *fakeSFN) regions() []string {
	var out []string
	for _, r := range f.requests() {
		out = append(out, r.region)
	}
	return out
}

func newEnv(t *testing.T, f *fakeSFN) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_SFN", unreachable)

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
	svc := StepFunctionsCalls[0]
	out, err := svc.Invoke(ctx)
	return svc.Process(out, err, false), err
}

func names(results []types.ScanResult) []string {
	var out []string
	for _, r := range results {
		out = append(out, r.ResourceName)
	}
	return out
}

func sm(name string) string { return `{"name":"` + name + `"}` }

func TestServiceIsV2Only(t *testing.T) {
	if len(StepFunctionsCalls) != 1 {
		t.Fatalf("expected 1 service, got %d", len(StepFunctionsCalls))
	}
	svc := StepFunctionsCalls[0]
	if svc.Name != wantName {
		t.Errorf("name = %q, want %q", svc.Name, wantName)
	}
	if svc.CallV2 == nil {
		t.Error("want CallV2 set")
	}
}

func TestInvokePopulatedAndSparse(t *testing.T) {
	f := &fakeSFN{machines: map[string][]string{
		// 1705314600 = 2024-01-15 10:30:00 UTC
		"us-west-2": {
			`{"name":"my-workflow","stateMachineArn":"arn:aws:states:us-west-2:123456789012:stateMachine:my-workflow","type":"EXPRESS","creationDate":1705314600}`,
			`{"stateMachineArn":"arn:aws:states:us-west-2:123456789012:stateMachine:bare"}`,
		},
	}}
	ctx := scoped(newEnv(t, f), "us-west-2")
	results, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"my-workflow", ""}; !reflect.DeepEqual(names(results), want) {
		t.Fatalf("names = %q, want %q", names(results), want)
	}
	wantDetails := []map[string]interface{}{
		{"StateMachineArn": "arn:aws:states:us-west-2:123456789012:stateMachine:my-workflow", "Type": "EXPRESS", "CreationDate": "2024-01-15 10:30:00"},
		{"StateMachineArn": "arn:aws:states:us-west-2:123456789012:stateMachine:bare", "Type": "", "CreationDate": ""},
	}
	for i, r := range results {
		if r.ServiceName != wantService || r.MethodName != wantName || r.ResourceType != "state-machine" ||
			r.Error != nil || r.Timestamp.IsZero() {
			t.Errorf("result %d = %+v", i, r)
		}
		if !reflect.DeepEqual(r.Details, wantDetails[i]) {
			t.Errorf("result %d Details = %#v, want %#v", i, r.Details, wantDetails[i])
		}
	}

	reqs := f.requests()
	if len(reqs) != 1 {
		t.Fatalf("requests = %+v, want exactly 1", reqs)
	}
	if r := reqs[0]; r.region != "us-west-2" || r.signingName != "states" || r.target != "AWSStepFunctions.ListStateMachines" {
		t.Errorf("request = %+v, want us-west-2/states AWSStepFunctions.ListStateMachines", r)
	}
}

func TestMultiRegionLoop(t *testing.T) {
	f := &fakeSFN{machines: map[string][]string{
		"us-east-1": {sm("east1")},
		"us-west-2": {sm("west2")},
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
	f := &fakeSFN{machines: map[string][]string{
		"us-east-1": {sm("east")},
		"eu-west-1": {sm("eu")},
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
	ctx := scoped(newEnv(t, &fakeSFN{}), "us-west-2")
	if results, err := run(ctx); err != nil || len(results) != 0 {
		t.Errorf("results = %+v, err = %v", results, err)
	}
}

func TestPagination(t *testing.T) {
	f := &fakeSFN{
		machines: map[string][]string{
			"eu-west-1": {sm("eu-1"), sm("eu-2"), sm("eu-3")},
			"us-west-2": {sm("us-1"), sm("us-2")},
		},
		pageSize: 2,
	}
	ctx := scoped(newEnv(t, f), "eu-west-1", "us-west-2")
	results, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"eu-1", "eu-2", "eu-3", "us-1", "us-2"}; !reflect.DeepEqual(names(results), want) {
		t.Errorf("names = %v, want %v", names(results), want)
	}
	reqs := f.requests()
	if len(reqs) != 3 {
		t.Fatalf("want 3 requests (2 pages in eu-west-1, 1 in us-west-2), got %+v", reqs)
	}
	// Tokens embed the region, so a token leaking across regions would show.
	wantTok := []string{"", "tok-eu-west-1-2", ""}
	wantRegion := []string{"eu-west-1", "eu-west-1", "us-west-2"}
	for i, r := range reqs {
		if r.token != wantTok[i] || r.region != wantRegion[i] {
			t.Errorf("request %d: region=%s token=%q, want region=%s token=%q", i, r.region, r.token, wantRegion[i], wantTok[i])
		}
	}
}

func TestFailedRegionIsSwallowed(t *testing.T) {
	f := &fakeSFN{
		machines: map[string][]string{
			"us-east-1": {sm("east-1"), sm("east-2"), sm("east-3")},
			"eu-west-1": {sm("eu")},
			"us-west-2": {sm("west")},
		},
		pageSize: 2,
		deny:     map[string]bool{"eu-west-1": true},
		// us-east-1 fails on its second page: the first page is kept.
		denyPage: map[string]int{"us-east-1": 1},
	}
	ctx := scoped(newEnv(t, f), "us-east-1", "eu-west-1", "us-west-2")
	results, err := run(ctx)
	if err != nil {
		t.Fatalf("a failing region must be swallowed while another succeeds, got %v", err)
	}
	if want := []string{"east-1", "east-2", "west"}; !reflect.DeepEqual(names(results), want) {
		t.Errorf("names = %v, want %v", names(results), want)
	}
	if got := f.regions(); !reflect.DeepEqual(got, []string{"us-east-1", "us-east-1", "eu-west-1", "us-west-2"}) {
		t.Errorf("signed regions = %v, want a failure to end only its own region", got)
	}
}

func TestAllRegionsFailReturnsLastErrorNormalized(t *testing.T) {
	f := &fakeSFN{deny: map[string]bool{"us-east-1": true, "us-west-2": true}}
	ctx := scoped(newEnv(t, f), "us-east-1", "us-west-2")
	svc := StepFunctionsCalls[0]
	out, err := svc.Invoke(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	if want := deniedCode + ": " + deniedMsg; err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if code := awsv2.ErrorCode(err); code != deniedCode {
		t.Errorf("ErrorCode = %q", code)
	}
	for _, noise := range []string{"status code", "request id", "operation error", "400"} {
		if strings.Contains(strings.ToLower(err.Error()), noise) {
			t.Errorf("error %q contains transport noise %q", err, noise)
		}
	}
	if got := f.regions(); !reflect.DeepEqual(got, []string{"us-east-1", "us-west-2"}) {
		t.Errorf("signed regions = %v, want every region tried", got)
	}
	results := svc.Process(out, err, false)
	if len(results) != 1 || results[0].Error == nil || results[0].ServiceName != wantService ||
		results[0].MethodName != wantName || results[0].ResourceName != "" {
		t.Fatalf("results = %+v", results)
	}
}

func TestProcessIgnoresUnexpectedOutput(t *testing.T) {
	for _, out := range []interface{}{nil, "x", 3, []string{"nope"}} {
		if got := StepFunctionsCalls[0].Process(out, nil, false); len(got) != 0 {
			t.Errorf("unexpected output %v produced %+v", out, got)
		}
	}
}
