package config

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

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/config/aws4_request`)

const targetPrefix = "StarlingDoveService."

type recorderSpec struct {
	name, role string
	recording  *bool // nil: no status entry for this recorder
}

type ruleSpec struct{ name, state, owner string }

type recorded struct{ region, op string }

// fakeConfig speaks the AWS Config JSON-1.1 protocol for the three operations
// awtest uses. Data is keyed by the SigV4 credential-scope region.
type fakeConfig struct {
	recorders map[string][]recorderSpec
	rules     map[string][]ruleSpec
	// failOp maps "region/Operation" -> AccessDeniedException message.
	failOp map[string]string

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeConfig) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	op := strings.TrimPrefix(r.Header.Get("X-Amz-Target"), targetPrefix)
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, op})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	if msg, ok := f.failOp[region+"/"+op]; ok {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"__type": "AccessDeniedException", "message": msg})
		return
	}
	switch op {
	case "DescribeConfigurationRecorders":
		var list []map[string]string
		for _, rec := range f.recorders[region] {
			e := map[string]string{}
			if rec.name != "" {
				e["name"] = rec.name
			}
			if rec.role != "" {
				e["roleARN"] = rec.role
			}
			list = append(list, e)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ConfigurationRecorders": list})
	case "DescribeConfigurationRecorderStatus":
		var list []map[string]interface{}
		for _, rec := range f.recorders[region] {
			if rec.recording != nil {
				list = append(list, map[string]interface{}{"name": rec.name, "recording": *rec.recording})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ConfigurationRecordersStatus": list})
	case "DescribeConfigRules":
		var list []map[string]interface{}
		for _, rl := range f.rules[region] {
			e := map[string]interface{}{}
			if rl.name != "" {
				e["ConfigRuleName"] = rl.name
			}
			if rl.state != "" {
				e["ConfigRuleState"] = rl.state
			}
			if rl.owner != "" {
				e["Source"] = map[string]string{"Owner": rl.owner}
			}
			list = append(list, e)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ConfigRules": list})
	default:
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"__type": "UnknownOperationException", "message": op})
	}
}

func (f *fakeConfig) ops() []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recorded(nil), f.reqs...)
}

func newEnv(t *testing.T, f *fakeConfig) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_CONFIG_SERVICE", unreachable)

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
	svc := ConfigCalls[0]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

func boolp(b bool) *bool { return &b }

func TestServiceIsV2Only(t *testing.T) {
	if len(ConfigCalls) != 1 || ConfigCalls[0].Name != "config:DescribeConfigurationRecorders" {
		t.Fatalf("unexpected services: %+v", ConfigCalls)
	}
	if ConfigCalls[0].CallV2 == nil || ConfigCalls[0].Call != nil {
		t.Error("want CallV2 set and Call nil")
	}
}

func TestInvokeProcessDetails(t *testing.T) {
	f := &fakeConfig{
		recorders: map[string][]recorderSpec{"us-west-2": {
			{name: "default", role: "arn:aws:iam::123456789012:role/config", recording: boolp(true)},
			{name: "stopped", role: "arn:aws:iam::123456789012:role/other", recording: boolp(false)},
			{name: "sparse"}, // no role, no status entry
		}},
		rules: map[string][]ruleSpec{"us-west-2": {
			{name: "s3-versioning", state: "ACTIVE", owner: "AWS"},
			{name: "sparse-rule"},
		}},
	}
	results, err := run(scoped(newEnv(t, f), "us-west-2"))
	if err != nil {
		t.Fatal(err)
	}
	type exp struct {
		method, kind, name string
		details            map[string]interface{}
	}
	want := []exp{
		{"config:DescribeConfigurationRecorders", "configuration-recorder", "default",
			map[string]interface{}{"RoleARN": "arn:aws:iam::123456789012:role/config", "RecordingStatus": "Recording"}},
		{"config:DescribeConfigurationRecorders", "configuration-recorder", "stopped",
			map[string]interface{}{"RoleARN": "arn:aws:iam::123456789012:role/other", "RecordingStatus": "Stopped"}},
		{"config:DescribeConfigurationRecorders", "configuration-recorder", "sparse",
			map[string]interface{}{"RoleARN": "", "RecordingStatus": "Stopped"}},
		{"config:DescribeConfigRules", "config-rule", "s3-versioning",
			map[string]interface{}{"State": "ACTIVE", "Owner": "AWS"}},
		{"config:DescribeConfigRules", "config-rule", "sparse-rule",
			map[string]interface{}{"State": "", "Owner": ""}},
	}
	if len(results) != len(want) {
		t.Fatalf("got %d results: %+v", len(results), results)
	}
	for i, w := range want {
		r := results[i]
		if r.ServiceName != "Config" || r.MethodName != w.method || r.ResourceType != w.kind ||
			r.ResourceName != w.name || r.Error != nil || r.Timestamp.IsZero() {
			t.Errorf("result %d = %+v", i, r)
		}
		if !reflect.DeepEqual(r.Details, w.details) {
			t.Errorf("%s details = %#v, want %#v", w.name, r.Details, w.details)
		}
	}
}

func TestEveryRegionInScopeIsVisited(t *testing.T) {
	f := &fakeConfig{
		recorders: map[string][]recorderSpec{"us-east-1": {{name: "e"}}, "eu-west-1": {{name: "w"}}, "ap-south-1": {{name: "a"}}},
	}
	results, err := run(scoped(newEnv(t, f), "us-east-1", "eu-west-1", "ap-south-1"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range f.ops() {
		if r.op == "DescribeConfigurationRecorders" {
			got = append(got, r.region)
		}
	}
	if !reflect.DeepEqual(got, []string{"us-east-1", "eu-west-1", "ap-south-1"}) {
		t.Errorf("signed regions = %v", got)
	}
	// Each region runs all three operations, signed for that region.
	if n := len(f.ops()); n != 9 {
		t.Errorf("expected 9 requests, got %d: %v", n, f.ops())
	}
	var names []string
	for _, r := range results {
		names = append(names, r.ResourceName)
	}
	if !reflect.DeepEqual(names, []string{"e", "w", "a"}) {
		t.Errorf("resources = %v", names)
	}
}

func TestAnyFailureAbortsWithNormalizedError(t *testing.T) {
	f := &fakeConfig{
		recorders: map[string][]recorderSpec{"us-east-1": {{name: "e"}}, "eu-west-1": {{name: "w"}}},
		failOp:    map[string]string{"eu-west-1/DescribeConfigRules": "rules denied"},
	}
	results, err := run(scoped(newEnv(t, f), "us-east-1", "eu-west-1", "us-west-2"))
	if err == nil {
		t.Fatal("expected error: a failing call aborts the whole scan")
	}
	if got, want := err.Error(), "AccessDeniedException: rules denied"; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
	if len(results) != 1 || results[0].Error == nil || results[0].ServiceName != "Config" {
		t.Errorf("results = %+v", results)
	}
	// us-west-2 is never reached, and the first failure ends the region's calls.
	for _, r := range f.ops() {
		if r.region == "us-west-2" {
			t.Errorf("region after the failure was still queried: %v", f.ops())
		}
	}
}
