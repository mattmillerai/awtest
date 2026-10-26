package waf

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

// fakeWAF speaks the WAF Classic JSON-1.1 protocol and records the targets.
type fakeWAF struct {
	status int
	body   string

	mu      sync.Mutex
	targets []string
}

func (f *fakeWAF) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.targets = append(f.targets, r.Header.Get("X-Amz-Target"))
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.WriteHeader(f.status)
	_, _ = w.Write([]byte(f.body))
}

func newCtx(t *testing.T, f *fakeWAF) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_WAF", unreachable)

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

func run(t *testing.T, ctx context.Context) ([]types.ScanResult, error) {
	t.Helper()
	utils.Quiet = true
	t.Cleanup(func() { utils.Quiet = false })
	svc := WafCalls[0]
	out, err := svc.Invoke(ctx)
	return svc.Process(out, err, false), err
}

func TestServicesAreV2Only(t *testing.T) {
	if len(WafCalls) != 1 || WafCalls[0].Name != "waf:ListWebACLs" {
		t.Fatalf("unexpected services: %+v", WafCalls)
	}
	if WafCalls[0].CallV2 == nil {
		t.Error("waf:ListWebACLs must implement CallV2 only")
	}
}

func TestListWebACLsFunctional(t *testing.T) {
	f := &fakeWAF{status: 200, body: `{"WebACLs":[` +
		`{"Name":"edge-protection-acl","WebACLId":"id-1"},` +
		`{"WebACLId":"id-2"}],"NextMarker":"more"}`}
	results, err := run(t, newCtx(t, f))
	if err != nil {
		t.Fatal(err)
	}
	type row struct{ typ, name string }
	var got []row
	for _, r := range results {
		if r.ServiceName != "WAF" || r.MethodName != "waf:ListWebACLs" {
			t.Errorf("unexpected result identity: %+v", r)
		}
		if !reflect.DeepEqual(r.Details, map[string]interface{}{}) {
			t.Errorf("details = %#v, want empty map", r.Details)
		}
		got = append(got, row{r.ResourceType, r.ResourceName})
	}
	// The second ACL is sparse (no name) and prints as an empty name.
	want := []row{{"web-acl", "edge-protection-acl"}, {"web-acl", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("results = %v, want %v", got, want)
	}
	// NextMarker is not followed (v1 ignored it): exactly one request.
	if !reflect.DeepEqual(f.targets, []string{"AWSWAF_20150824.ListWebACLs"}) {
		t.Errorf("targets = %v, want one ListWebACLs", f.targets)
	}
}

func TestListWebACLsEmpty(t *testing.T) {
	results, err := run(t, newCtx(t, &fakeWAF{status: 200, body: `{}`}))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("results = %+v, want none", results)
	}
}

func TestErrorsAreNormalized(t *testing.T) {
	f := &fakeWAF{status: 400, body: `{"__type":"AccessDeniedException","message":"not allowed"}`}
	results, err := run(t, newCtx(t, f))
	if err == nil {
		t.Fatal("expected an error")
	}
	if got, want := err.Error(), "AccessDeniedException: not allowed"; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
	if len(results) != 1 || results[0].Error == nil || results[0].MethodName != "waf:ListWebACLs" || results[0].ServiceName != "WAF" {
		t.Fatalf("results = %+v", results)
	}
	if got := results[0].Error.Error(); got != "AccessDeniedException: not allowed" {
		t.Errorf("result error = %q", got)
	}
}
