package athena

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

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/[^/]+/aws4_request`)

const targetPrefix = "AmazonAthena."

type recorded struct {
	region string
	op     string
	body   map[string]interface{}
}

// fakeSvc speaks the JSON-1.1 protocol. handle receives the SigV4
// credential-scope region, the operation and the decoded request body and
// returns the HTTP status and JSON response.
type fakeSvc struct {
	handle func(region, op string, body map[string]interface{}) (int, interface{})

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
	target := r.Header.Get("X-Amz-Target")
	op := strings.TrimPrefix(target, targetPrefix)
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, op, body})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	var status int
	var out interface{}
	if !strings.HasPrefix(target, targetPrefix) || r.Method != http.MethodPost {
		status, out = apiError("UnknownOperationException", "bad request "+r.Method+" "+target)
	} else {
		status, out = f.handle(region, op, body)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(out)
}

func (f *fakeSvc) ops(op string) []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []recorded
	for _, r := range f.reqs {
		if r.op == op {
			out = append(out, r)
		}
	}
	return out
}

func (f *fakeSvc) regionsOf(op string) []string {
	var out []string
	for _, r := range f.ops(op) {
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
	t.Setenv("AWS_ENDPOINT_URL_ATHENA", unreachable)

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
	svc := AthenaCalls[i]
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

func TestServicesAreV2Only(t *testing.T) {
	for i, svc := range AthenaCalls {
		if svc.CallV2 == nil || svc.Call != nil {
			t.Errorf("service %d %s: want CallV2 set and Call nil", i, svc.Name)
		}
	}
}

func TestListWorkGroupsInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		return http.StatusOK, map[string]interface{}{"WorkGroups": []map[string]interface{}{
			{
				"Name":         "primary",
				"State":        "ENABLED",
				"Description":  "main group",
				"CreationTime": 1700000000,
				"EngineVersion": map[string]string{
					"SelectedEngineVersion":  "AUTO",
					"EffectiveEngineVersion": "Athena engine version 3",
				},
			},
			{"Name": "bare"},
		}}
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"Name": "primary", "State": "ENABLED", "Description": "main group",
			"CreationTime": "2023-11-14T22:13:20Z", "EngineVersion": "Athena engine version 3", "Region": "us-west-2",
		},
		{
			"Name": "bare", "State": "", "Description": "",
			"CreationTime": "", "EngineVersion": "", "Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	for i, name := range []string{"primary", "bare"} {
		r := results[i]
		if r.ServiceName != "Athena" || r.MethodName != "athena:ListWorkGroups" || r.ResourceType != "workgroup" || r.ResourceName != name {
			t.Errorf("result %d = %+v", i, r)
		}
	}
	reqs := f.ops("ListWorkGroups")
	if len(reqs) != 1 || reqs[0].region != "us-west-2" || reqs[0].body["MaxResults"] != float64(50) {
		t.Errorf("requests = %+v", reqs)
	}
}

func TestListWorkGroupsPaginates(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		if body["NextToken"] == "t2" {
			return http.StatusOK, map[string]interface{}{"WorkGroups": []map[string]string{{"Name": "second"}}}
		}
		return http.StatusOK, map[string]interface{}{"WorkGroups": []map[string]string{{"Name": "first"}}, "NextToken": "t2"}
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].ResourceName != "first" || results[1].ResourceName != "second" {
		t.Errorf("results = %+v", results)
	}
}

func TestInvokeErrorText(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		return apiError("AccessDeniedException", "not authorized for "+op)
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err == nil {
		t.Fatal("expected an error")
	}
	want := "AccessDeniedException: not authorized for ListWorkGroups"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if len(results) != 1 || results[0].Error == nil || results[0].Error.Error() != want {
		t.Errorf("Process error result = %+v", results)
	}
	if results[0].MethodName != "athena:ListWorkGroups" {
		t.Errorf("MethodName = %q", results[0].MethodName)
	}
}

func TestRegionScopeVisitsEachRegion(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		return http.StatusOK, map[string]interface{}{"WorkGroups": []map[string]string{{"Name": "wg-" + region}}}
	}}
	scope := []string{"us-east-1", "eu-west-1", "ap-south-1"}
	ctx := newEnv(t, f, scope...)
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.regionsOf("ListWorkGroups"); !reflect.DeepEqual(got, scope) {
		t.Errorf("SigV4 regions = %v, want %v", got, scope)
	}
	var regions []string
	for _, d := range detailsOf(results) {
		regions = append(regions, d["Region"].(string))
	}
	if !reflect.DeepEqual(regions, scope) {
		t.Errorf("result regions = %v, want %v", regions, scope)
	}
}
