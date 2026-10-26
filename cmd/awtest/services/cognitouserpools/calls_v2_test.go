package cognitouserpools

import (
	"context"
	"encoding/json"
	"io"
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

const (
	wantName    = "cognito-idp:ListUserPools"
	wantService = "CognitoUserPools"
	deniedCode  = "NotAuthorizedException"
	deniedMsg   = "User is not authorized to perform: cognito-idp:ListUserPools"
)

var credScope = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/([^/]+)/aws4_request`)

type request struct {
	target, region, signingName string
	body                        map[string]interface{}
}

// fakeCognito speaks the JSON-1.1 protocol for cognito-idp:ListUserPools. Data
// is keyed by the SigV4 credential-scope region; a region in deny answers with
// an error, a region without data returns an empty list.
type fakeCognito struct {
	pools map[string]string // region -> JSON array of UserPools
	deny  map[string]bool

	mu   sync.Mutex
	reqs []request
}

func (f *fakeCognito) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	req := request{target: r.Header.Get("X-Amz-Target"), body: map[string]interface{}{}}
	_ = json.Unmarshal(raw, &req.body)
	if m := credScope.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		req.region, req.signingName = m[1], m[2]
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, req)
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	if f.deny[req.region] {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"__type":"`+deniedCode+`","message":"`+deniedMsg+`"}`)
		return
	}
	list := f.pools[req.region]
	if list == "" {
		list = "[]"
	}
	// The continuation token must be ignored, as v1 ignored it.
	_, _ = io.WriteString(w, `{"UserPools":`+list+`,"NextToken":"more"}`)
}

func (f *fakeCognito) requests() []request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]request(nil), f.reqs...)
}

func (f *fakeCognito) regions() []string {
	var out []string
	for _, r := range f.requests() {
		out = append(out, r.region)
	}
	return out
}

func newEnv(t *testing.T, f *fakeCognito) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_COGNITO_IDENTITY_PROVIDER", unreachable)

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
	svc := CognitoUserPoolsCalls[0]
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

func TestServiceIsV2Only(t *testing.T) {
	if len(CognitoUserPoolsCalls) != 1 {
		t.Fatalf("expected 1 service, got %d", len(CognitoUserPoolsCalls))
	}
	svc := CognitoUserPoolsCalls[0]
	if svc.Name != wantName {
		t.Errorf("name = %q, want %q", svc.Name, wantName)
	}
	if svc.CallV2 == nil {
		t.Error("want CallV2 set")
	}
}

func TestInvokePopulatedAndSparse(t *testing.T) {
	f := &fakeCognito{pools: map[string]string{
		// 1750000000 = 2025-06-15 15:06:40 UTC
		"us-west-2": `[{"Id":"us-west-2_abc","Name":"users","Status":"Enabled","CreationDate":1750000000},{"Id":"us-west-2_def"}]`,
	}}
	ctx := scoped(newEnv(t, f), "us-west-2")
	results, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"users", ""}; !reflect.DeepEqual(names(results), want) {
		t.Fatalf("names = %q, want %q", names(results), want)
	}
	wantDetails := []map[string]interface{}{
		{"Id": "us-west-2_abc", "Status": "Enabled", "CreationDate": "2025-06-15 15:06:40"},
		{"Id": "us-west-2_def", "Status": "", "CreationDate": ""},
	}
	for i, r := range results {
		if r.ServiceName != wantService || r.MethodName != wantName || r.ResourceType != "user-pool" ||
			r.Error != nil || r.Timestamp.IsZero() {
			t.Errorf("result %d = %+v", i, r)
		}
		if !reflect.DeepEqual(r.Details, wantDetails[i]) {
			t.Errorf("result %d Details = %#v, want %#v", i, r.Details, wantDetails[i])
		}
	}

	reqs := f.requests()
	if len(reqs) != 1 {
		t.Fatalf("requests = %+v, want exactly 1 (v1 ignored the continuation token)", reqs)
	}
	r := reqs[0]
	if r.region != "us-west-2" || r.signingName != "cognito-idp" || r.target != "AWSCognitoIdentityProviderService.ListUserPools" {
		t.Errorf("request = %+v, want us-west-2/cognito-idp AWSCognitoIdentityProviderService.ListUserPools", r)
	}
	if r.body["MaxResults"] != float64(60) {
		t.Errorf("MaxResults = %v, want 60", r.body["MaxResults"])
	}
}

func TestMultiRegionLoop(t *testing.T) {
	f := &fakeCognito{pools: map[string]string{
		"us-east-1": `[{"Name":"east1"}]`,
		"us-west-2": `[{"Name":"west2"}]`,
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
	f := &fakeCognito{pools: map[string]string{
		"us-east-1": `[{"Name":"east"}]`,
		"eu-west-1": `[{"Name":"eu"}]`,
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
	ctx := scoped(newEnv(t, &fakeCognito{}), "us-west-2")
	if results, err := run(ctx); err != nil || len(results) != 0 {
		t.Errorf("results = %+v, err = %v", results, err)
	}
}

func TestAccessDeniedNormalizedAbortsLoop(t *testing.T) {
	f := &fakeCognito{
		pools: map[string]string{"us-east-1": `[{"Name":"east"}]`, "eu-west-1": `[{"Name":"eu"}]`},
		deny:  map[string]bool{"us-west-2": true},
	}
	ctx := scoped(newEnv(t, f), "us-east-1", "us-west-2", "eu-west-1")
	svc := CognitoUserPoolsCalls[0]
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

func TestProcessIgnoresUnexpectedOutput(t *testing.T) {
	for _, out := range []interface{}{nil, "x", 3, []string{"nope"}} {
		if got := CognitoUserPoolsCalls[0].Process(out, nil, false); len(got) != 0 {
			t.Errorf("unexpected output %v produced %+v", out, got)
		}
	}
}
