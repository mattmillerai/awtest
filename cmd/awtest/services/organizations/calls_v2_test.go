package organizations

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

const targetPrefix = "AWSOrganizationsV20161128."

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

// newEnv builds a context whose config region is us-west-2 (and, when
// regions are given, whose region scope is those regions); Organizations must
// ignore both and always sign for us-east-1.
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
	t.Setenv("AWS_ENDPOINT_URL_ORGANIZATIONS", unreachable)

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
	svc := OrganizationsCalls[i]
	out, err := svc.Invoke(ctx)
	return svc.Process(out, err, false), err
}

func detailsOf(results []types.ScanResult) []map[string]interface{} {
	var out []map[string]interface{}
	for _, r := range results {
		out = append(out, r.Details)
	}
	return out
}

func TestListAccountsInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		if op != "ListAccounts" {
			return apiError("UnknownOperationException", op)
		}
		if body["NextToken"] == "p2" {
			return http.StatusOK, map[string]interface{}{"Accounts": []map[string]string{{"Name": "bare-account"}}}
		}
		return http.StatusOK, map[string]interface{}{"Accounts": []map[string]interface{}{{
			"Id": "123456789012", "Name": "Production", "Email": "prod@example.com", "Status": "ACTIVE",
			"Arn": "arn:aws:organizations::111111111111:account/o-abc123/123456789012", "JoinedTimestamp": 1700000000,
		}}, "NextToken": "p2"}
	}}
	ctx := newEnv(t, f)
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"AccountId": "123456789012", "Email": "prod@example.com", "Status": "ACTIVE",
			"Arn": "arn:aws:organizations::111111111111:account/o-abc123/123456789012", "JoinedTimestamp": "2023-11-14T22:13:20Z",
		},
		{"AccountId": "", "Email": "", "Status": "", "Arn": "", "JoinedTimestamp": ""},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ServiceName != "Organizations" || results[0].MethodName != "organizations:ListAccounts" || results[0].ResourceType != "account" || results[0].ResourceName != "Production" {
		t.Errorf("result 0 = %+v", results[0])
	}
	if results[1].ResourceName != "bare-account" {
		t.Errorf("result 1 = %+v", results[1])
	}
	reqs := f.ops("ListAccounts")
	if len(reqs) != 2 {
		t.Fatalf("ListAccounts calls = %d, want 2 (NextToken must be followed)", len(reqs))
	}
	for _, r := range reqs {
		if r.region != "us-east-1" {
			t.Errorf("SigV4 region = %q, want us-east-1", r.region)
		}
	}
}

func TestListOrganizationalUnitsInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		switch op {
		case "ListRoots":
			if body["NextToken"] == "r2" {
				return http.StatusOK, map[string]interface{}{"Roots": []map[string]string{{"Name": "root-without-id"}, {"Id": "r-2"}}}
			}
			return http.StatusOK, map[string]interface{}{"Roots": []map[string]string{{"Id": "r-1"}}, "NextToken": "r2"}
		case "ListOrganizationalUnitsForParent":
			switch body["ParentId"] {
			case "r-1":
				if body["NextToken"] == "o2" {
					// An OU without an Id is reported but never descended into.
					return http.StatusOK, map[string]interface{}{"OrganizationalUnits": []map[string]string{{"Name": "no-id-ou"}}}
				}
				return http.StatusOK, map[string]interface{}{"OrganizationalUnits": []map[string]string{
					{"Id": "ou-a", "Name": "Prod", "Arn": "arn:ou-a"},
				}, "NextToken": "o2"}
			case "r-2":
				// A failing parent is skipped and the walk continues.
				return apiError("AccessDeniedException", "denied for r-2")
			case "ou-a":
				return http.StatusOK, map[string]interface{}{"OrganizationalUnits": []map[string]string{{"Id": "ou-c", "Name": "Child", "Arn": "arn:ou-c"}}}
			case "ou-c":
				return http.StatusOK, map[string]interface{}{"OrganizationalUnits": []map[string]string{}}
			}
		}
		return apiError("UnknownOperationException", op)
	}}
	ctx := newEnv(t, f)
	results, err := run(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{"OUId": "ou-a", "Arn": "arn:ou-a", "ParentId": "r-1"},
		{"OUId": "", "Arn": "", "ParentId": "r-1"},
		{"OUId": "ou-c", "Arn": "arn:ou-c", "ParentId": "ou-a"},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ResourceType != "organizational-unit" || results[0].ResourceName != "Prod" || results[1].ResourceName != "no-id-ou" || results[2].ResourceName != "Child" {
		t.Errorf("results = %+v", results)
	}
	var parents []interface{}
	for _, r := range f.ops("ListOrganizationalUnitsForParent") {
		if r.region != "us-east-1" {
			t.Errorf("SigV4 region = %q, want us-east-1", r.region)
		}
		parents = append(parents, r.body["ParentId"])
	}
	if wantParents := []interface{}{"r-1", "r-1", "r-2", "ou-a", "ou-c"}; !reflect.DeepEqual(parents, wantParents) {
		t.Errorf("parents visited = %v, want %v", parents, wantParents)
	}
	if got := len(f.ops("ListRoots")); got != 2 {
		t.Errorf("ListRoots calls = %d, want 2", got)
	}
}

func TestListPoliciesInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		if op != "ListPolicies" {
			return apiError("UnknownOperationException", op)
		}
		if body["Filter"] != "SERVICE_CONTROL_POLICY" {
			return apiError("InvalidInputException", "unexpected filter")
		}
		if body["NextToken"] == "p2" {
			return http.StatusOK, map[string]interface{}{"Policies": []map[string]string{{"Name": "bare-policy"}}}
		}
		return http.StatusOK, map[string]interface{}{"Policies": []map[string]string{{
			"Id": "p-abc12345", "Name": "FullAWSAccess", "Arn": "arn:aws:organizations::aws:policy/service_control_policy/p-abc12345",
		}}, "NextToken": "p2"}
	}}
	ctx := newEnv(t, f)
	results, err := run(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{"PolicyId": "p-abc12345", "Arn": "arn:aws:organizations::aws:policy/service_control_policy/p-abc12345"},
		{"PolicyId": "", "Arn": ""},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ResourceType != "service-control-policy" || results[0].ResourceName != "FullAWSAccess" || results[1].ResourceName != "bare-policy" {
		t.Errorf("results = %+v", results)
	}
	reqs := f.ops("ListPolicies")
	if len(reqs) != 2 {
		t.Fatalf("ListPolicies calls = %d, want 2", len(reqs))
	}
	for _, r := range reqs {
		if r.region != "us-east-1" {
			t.Errorf("SigV4 region = %q, want us-east-1", r.region)
		}
	}
}

func TestInvokeErrorText(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		return apiError("AccessDeniedException", "not authorized for "+op)
	}}
	wantOps := []string{"ListAccounts", "ListRoots", "ListPolicies"}
	for i := range OrganizationsCalls {
		ctx := newEnv(t, f)
		results, err := run(ctx, i)
		if err == nil {
			t.Fatalf("%s: expected an error", OrganizationsCalls[i].Name)
		}
		want := "AccessDeniedException: not authorized for " + wantOps[i]
		if err.Error() != want {
			t.Errorf("%s: error = %q, want %q", OrganizationsCalls[i].Name, err.Error(), want)
		}
		if len(results) != 1 || results[0].Error == nil || results[0].Error.Error() != want {
			t.Errorf("%s: Process error result = %+v", OrganizationsCalls[i].Name, results)
		}
		if results[0].MethodName != OrganizationsCalls[i].Name {
			t.Errorf("MethodName = %q", results[0].MethodName)
		}
	}
}

// Organizations is a global service: whatever the configured region or the
// region scope, every request is signed for us-east-1 and sent exactly once.
func TestAlwaysSignsForUSEast1(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		return http.StatusOK, map[string]interface{}{"Accounts": []map[string]string{}, "Roots": []map[string]string{}, "Policies": []map[string]string{}}
	}}
	for i := range OrganizationsCalls {
		f.mu.Lock()
		f.reqs = nil
		f.mu.Unlock()
		ctx := newEnv(t, f, "eu-west-1", "ap-south-1")
		if _, err := run(ctx, i); err != nil {
			t.Fatal(err)
		}
		f.mu.Lock()
		reqs := append([]recorded(nil), f.reqs...)
		f.mu.Unlock()
		if len(reqs) != 1 || reqs[0].region != "us-east-1" {
			t.Errorf("%s: requests = %+v, want exactly one signed for us-east-1", OrganizationsCalls[i].Name, reqs)
		}
	}
}
