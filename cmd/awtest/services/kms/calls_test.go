package kms

import (
	"context"
	"encoding/json"
	"fmt"
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

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/kms/aws4_request`)

// fakeKMS speaks the KMS JSON-1.1 protocol for ListKeys. Results differ per
// SigV4 credential-scope region.
type fakeKMS struct {
	// keys maps region -> key ids returned for that region.
	keys map[string][]string
	// deny maps region -> AccessDeniedException message.
	deny map[string]string

	mu      sync.Mutex
	regions []string
	targets []string
	bodies  []string
	markers []string
}

func (f *fakeKMS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization"))
	region := ""
	if m != nil {
		region = m[1]
	}
	body := make([]byte, 1<<16)
	n, _ := r.Body.Read(body)
	f.mu.Lock()
	f.regions = append(f.regions, region)
	f.targets = append(f.targets, r.Header.Get("X-Amz-Target"))
	f.bodies = append(f.bodies, string(body[:n]))
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	if msg, ok := f.deny[region]; ok {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"__type": "AccessDeniedException", "message": msg})
		return
	}
	type entry struct {
		KeyId  string
		KeyArn string
	}
	var entries []entry
	for _, id := range f.keys[region] {
		entries = append(entries, entry{id, "arn:aws:kms:" + region + ":123456789012:key/" + id})
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"Keys": entries, "Truncated": false})
}

func newEnv(t *testing.T, f *fakeKMS) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_KMS", unreachable)

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

func run(ctx context.Context) ([]types.ScanResult, error) {
	svc := KMSCalls[0]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

func TestServiceIsV2Only(t *testing.T) {
	if len(KMSCalls) != 1 {
		t.Fatalf("expected 1 service, got %d", len(KMSCalls))
	}
	svc := KMSCalls[0]
	if svc.CallV2 == nil || svc.Call != nil {
		t.Errorf("want CallV2 set and Call nil, got CallV2=%v Call=%v", svc.CallV2 != nil, svc.Call != nil)
	}
	if svc.Name != "kms:ListKeys" {
		t.Errorf("name = %q", svc.Name)
	}
}

func TestMultiRegionLoop(t *testing.T) {
	f := &fakeKMS{keys: map[string][]string{
		"us-east-1": {"key-east-1", "key-east-2"},
		"us-east-2": {"key-east2"},
		"us-west-2": {"key-west"},
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

	if !reflect.DeepEqual(f.regions, regions) {
		t.Errorf("request regions = %v, want %v (in order, once each)", f.regions, regions)
	}
	for _, tg := range f.targets {
		if tg != "TrentService.ListKeys" {
			t.Errorf("X-Amz-Target = %q", tg)
		}
	}

	var want []string
	for _, r := range regions {
		want = append(want, f.keys[r]...)
	}
	var got []string
	for _, r := range results {
		got = append(got, r.ResourceName)
		if r.ServiceName != "KMS" || r.MethodName != "kms:ListKeys" || r.ResourceType != "key" ||
			r.Error != nil || r.Timestamp.IsZero() || r.Details == nil || len(r.Details) != 0 {
			t.Errorf("unexpected result %+v", r)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("keys = %v, want %v", got, want)
	}
}

func TestRegionScope(t *testing.T) {
	f := &fakeKMS{keys: map[string][]string{
		"us-east-1": {"key-east"},
		"eu-west-1": {"key-eu"},
	}}
	ctx, scope := types.WithRegionScope(newEnv(t, f), []string{"eu-west-1"})
	results, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !scope.Used() {
		t.Error("service did not consult the region scope")
	}
	if !reflect.DeepEqual(f.regions, []string{"eu-west-1"}) {
		t.Errorf("request regions = %v, want only eu-west-1", f.regions)
	}
	if len(results) != 1 || results[0].ResourceName != "key-eu" {
		t.Errorf("results = %+v", results)
	}
}

func TestNoKeysYieldsNoResults(t *testing.T) {
	results, err := run(newEnv(t, &fakeKMS{}))
	if err != nil || len(results) != 0 {
		t.Errorf("results = %+v, err = %v", results, err)
	}
}

// v1 made exactly one ListKeys call per region and did not follow
// NextMarker; the migration keeps that.
func TestNoPagination(t *testing.T) {
	f := &fakeKMS{keys: map[string][]string{"us-west-2": {"k1"}}}
	ctx, _ := types.WithRegionScope(newEnv(t, f), []string{"us-west-2"})
	if _, err := run(ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.bodies) != 1 || strings.TrimSpace(f.bodies[0]) != "{}" {
		t.Errorf("want one empty-body request, got %q", f.bodies)
	}
}

func TestAccessDeniedNormalized(t *testing.T) {
	f := &fakeKMS{
		keys: map[string][]string{"us-east-1": {"k"}},
		deny: map[string]string{"us-west-1": "User is not authorized to perform: kms:ListKeys"},
	}
	ctx := newEnv(t, f)
	svc := KMSCalls[0]
	out, err := svc.Invoke(ctx, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	const want = "AccessDeniedException: User is not authorized to perform: kms:ListKeys"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if code := awsv2.ErrorCode(err); code != "AccessDeniedException" {
		t.Errorf("ErrorCode = %q", code)
	}
	for _, noise := range []string{"status code", "request id", "operation error", "400"} {
		if strings.Contains(strings.ToLower(err.Error()), noise) {
			t.Errorf("error %q contains transport noise %q", err, noise)
		}
	}
	// v1 aborted the whole call on the first failing region.
	regions := types.RegionsFor(ctx)
	idx := -1
	for i, r := range regions {
		if r == "us-west-1" {
			idx = i
		}
	}
	if len(f.regions) != idx+1 {
		t.Errorf("requests = %v, want loop to stop at us-west-1 (index %d)", f.regions, idx)
	}

	results := svc.Process(out, err, false)
	if len(results) != 1 || results[0].Error == nil || results[0].ServiceName != "KMS" ||
		results[0].MethodName != "kms:ListKeys" || results[0].ResourceName != "" {
		t.Fatalf("results = %+v", results)
	}
}

func TestAmbientEndpointIgnored(t *testing.T) {
	// newEnv sets AWS_ENDPOINT_URL_KMS to an unreachable host; success proves
	// the explicit endpoint won.
	f := &fakeKMS{keys: map[string][]string{"us-west-2": {"k1"}}}
	ctx, _ := types.WithRegionScope(newEnv(t, f), []string{"us-west-2"})
	results, err := run(ctx)
	if err != nil || len(results) != 1 {
		t.Fatalf("results = %+v, err = %v", results, fmt.Sprint(err))
	}
}

func TestProcessIgnoresUnexpectedOutput(t *testing.T) {
	p := KMSCalls[0].Process
	for _, out := range []interface{}{nil, "x", 3} {
		if got := p(out, nil, false); len(got) != 0 {
			t.Errorf("results = %+v", got)
		}
	}
}
