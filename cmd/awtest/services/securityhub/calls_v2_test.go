package securityhub

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/securityhub/aws4_request`)

type recorded struct {
	region string
	method string
	path   string
	query  string
	body   map[string]interface{}
}

// fakeHub speaks the Security Hub restJson1 protocol. handler returns the
// HTTP status and JSON body (or, for status >= 400, the error code and message).
type fakeHub struct {
	handler func(r recorded) (status int, body interface{}, errCode string)

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	raw, _ := io.ReadAll(r.Body)
	body := map[string]interface{}{}
	_ = json.Unmarshal(raw, &body)
	rec := recorded{region: region, method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: body}
	f.mu.Lock()
	f.reqs = append(f.reqs, rec)
	f.mu.Unlock()

	status, out, code := f.handler(rec)
	w.Header().Set("Content-Type", "application/json")
	if code != "" {
		w.Header().Set("X-Amzn-Errortype", code)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(out)
}

func (f *fakeHub) calls(path string) []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []recorded
	for _, r := range f.reqs {
		if r.path == path {
			out = append(out, r)
		}
	}
	return out
}

func newEnv(t *testing.T, f *fakeHub, regions ...string) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_SECURITYHUB", unreachable)

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

func run(t *testing.T, ctx context.Context, i int) ([]types.ScanResult, error) {
	t.Helper()
	utils.Quiet = true
	t.Cleanup(func() { utils.Quiet = false })
	svc := SecurityHubCalls[i]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

const (
	svcStandards = 0
	svcFindings  = 1
	svcProducts  = 2
)

func TestServicesAreV2Only(t *testing.T) {
	want := []string{"securityhub:GetEnabledStandards", "securityhub:GetFindings", "securityhub:ListEnabledProductsForImport"}
	if len(SecurityHubCalls) != len(want) {
		t.Fatalf("got %d services, want %d", len(SecurityHubCalls), len(want))
	}
	for i, svc := range SecurityHubCalls {
		if svc.Name != want[i] {
			t.Errorf("service %d = %q, want %q", i, svc.Name, want[i])
		}
		if svc.CallV2 == nil || svc.Call != nil {
			t.Errorf("%s must implement CallV2 only", svc.Name)
		}
	}
}

func TestGetEnabledStandardsFunctional(t *testing.T) {
	f := &fakeHub{handler: func(r recorded) (int, interface{}, string) {
		if r.path != "/standards/get" || r.method != "POST" {
			return 404, map[string]string{"message": "unexpected " + r.method + " " + r.path}, "UnknownOperationException"
		}
		if r.region == "us-east-1" {
			if r.body["NextToken"] == "t2" {
				return 200, map[string]interface{}{"StandardsSubscriptions": []map[string]string{{}}}, ""
			}
			return 200, map[string]interface{}{
				"NextToken": "t2",
				"StandardsSubscriptions": []map[string]string{{
					"StandardsSubscriptionArn": "arn:aws:securityhub:us-east-1:111111111111:subscription/cis-aws-foundations-benchmark/v/1.2.0",
					"StandardsArn":             "arn:aws:securityhub:::ruleset/cis-aws-foundations-benchmark/v/1.2.0",
					"StandardsStatus":          "READY",
				}},
			}, ""
		}
		return 200, map[string]interface{}{"StandardsSubscriptions": []map[string]string{}}, ""
	}}
	ctx := newEnv(t, f, "us-east-1", "eu-west-1")
	res, err := run(t, ctx, svcStandards)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Fatalf("got %d results, want 2: %+v", len(res), res)
	}
	full := res[0]
	if full.ServiceName != "SecurityHub" || full.MethodName != "securityhub:GetEnabledStandards" || full.ResourceType != "standard" || full.ResourceName != "cis-aws-foundations-benchmark" {
		t.Errorf("bad populated result: %+v", full)
	}
	wantDetails := map[string]interface{}{
		"StandardsSubscriptionArn": "arn:aws:securityhub:us-east-1:111111111111:subscription/cis-aws-foundations-benchmark/v/1.2.0",
		"StandardsArn":             "arn:aws:securityhub:::ruleset/cis-aws-foundations-benchmark/v/1.2.0",
		"StandardsStatus":          "READY",
		"Region":                   "us-east-1",
	}
	if !reflect.DeepEqual(full.Details, wantDetails) {
		t.Errorf("details = %#v, want %#v", full.Details, wantDetails)
	}
	// The second page holds a sparse subscription: nil strings read as "".
	sparse := res[1]
	wantSparse := map[string]interface{}{"StandardsSubscriptionArn": "", "StandardsArn": "", "StandardsStatus": "", "Region": "us-east-1"}
	if sparse.ResourceName != "" || !reflect.DeepEqual(sparse.Details, wantSparse) {
		t.Errorf("sparse result = %+v", sparse)
	}
	if got := len(f.calls("/standards/get")); got != 3 { // 2 pages in us-east-1 + 1 in eu-west-1
		t.Errorf("calls = %d, want 3 (NextToken followed)", got)
	}
}

func TestGetFindingsFunctional(t *testing.T) {
	f := &fakeHub{handler: func(r recorded) (int, interface{}, string) {
		if r.path != "/findings" || r.method != "POST" {
			return 404, map[string]string{"message": "unexpected " + r.path}, "UnknownOperationException"
		}
		return 200, map[string]interface{}{
			// GetFindings ignores NextToken in v1; it must keep doing so.
			"NextToken": "more",
			"Findings": []map[string]interface{}{
				{
					"Id":          "f-1",
					"Title":       "Public bucket",
					"GeneratorId": "gen-1",
					"ProductName": "Security Hub",
					"Severity":    map[string]string{"Label": "HIGH"},
					"Compliance":  map[string]string{"Status": "FAILED"},
					"Resources":   []map[string]string{{"Type": "AwsS3Bucket"}, {"Type": "Other"}},
				},
				{"Id": "f-2", "GeneratorId": "gen-2"},
				{},
			},
		}, ""
	}}
	ctx := newEnv(t, f, "ap-south-1")
	res, err := run(t, ctx, svcFindings)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(f.calls("/findings")); got != 1 {
		t.Fatalf("calls = %d, want exactly 1 (NextToken ignored)", got)
	}
	req := f.calls("/findings")[0]
	if req.body["MaxResults"] != float64(100) {
		t.Errorf("MaxResults = %v", req.body["MaxResults"])
	}
	wantFilters := map[string]interface{}{"RecordState": []interface{}{map[string]interface{}{"Comparison": "EQUALS", "Value": "ACTIVE"}}}
	if !reflect.DeepEqual(req.body["Filters"], wantFilters) {
		t.Errorf("Filters = %#v", req.body["Filters"])
	}
	wantSort := []interface{}{map[string]interface{}{"Field": "SeverityLabel", "SortOrder": "desc"}}
	if !reflect.DeepEqual(req.body["SortCriteria"], wantSort) {
		t.Errorf("SortCriteria = %#v", req.body["SortCriteria"])
	}
	if len(res) != 3 {
		t.Fatalf("got %d results: %+v", len(res), res)
	}
	wantFull := map[string]interface{}{
		"Id": "f-1", "Title": "Public bucket", "SeverityLabel": "HIGH", "ComplianceStatus": "FAILED",
		"ProductName": "Security Hub", "ResourceType": "AwsS3Bucket", "Region": "ap-south-1", "GeneratorId": "gen-1",
	}
	if res[0].ResourceName != "Public bucket" || res[0].ResourceType != "finding" || !reflect.DeepEqual(res[0].Details, wantFull) {
		t.Errorf("populated = %+v", res[0])
	}
	// No title: the generator id names the finding.
	wantNoTitle := map[string]interface{}{
		"Id": "f-2", "Title": "", "SeverityLabel": "", "ComplianceStatus": "",
		"ProductName": "", "ResourceType": "", "Region": "ap-south-1", "GeneratorId": "gen-2",
	}
	if res[1].ResourceName != "gen-2" || !reflect.DeepEqual(res[1].Details, wantNoTitle) {
		t.Errorf("sparse = %+v", res[1])
	}
	if res[2].ResourceName != "" || res[2].Details["Id"] != "" || res[2].Details["Region"] != "ap-south-1" {
		t.Errorf("empty finding = %+v", res[2])
	}
}

func TestListEnabledProductsFunctional(t *testing.T) {
	f := &fakeHub{handler: func(r recorded) (int, interface{}, string) {
		if r.path != "/productSubscriptions" || r.method != "GET" {
			return 404, map[string]string{"message": "unexpected " + r.path}, "UnknownOperationException"
		}
		if strings.Contains(r.query, "NextToken=p2") {
			return 200, map[string]interface{}{"ProductSubscriptions": []string{"arn:aws:securityhub:us-east-1:1:product-subscription/b"}}, ""
		}
		return 200, map[string]interface{}{
			"NextToken":            "p2",
			"ProductSubscriptions": []string{"arn:aws:securityhub:us-east-1:1:product-subscription/a"},
		}, ""
	}}
	ctx := newEnv(t, f, "us-east-1")
	res, err := run(t, ctx, svcProducts)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Fatalf("got %d results", len(res))
	}
	for i, suffix := range []string{"a", "b"} {
		arn := "arn:aws:securityhub:us-east-1:1:product-subscription/" + suffix
		r := res[i]
		if r.ResourceType != "product" || r.ResourceName != arn || r.MethodName != "securityhub:ListEnabledProductsForImport" ||
			!reflect.DeepEqual(r.Details, map[string]interface{}{"ProductSubscriptionArn": arn, "Region": "us-east-1"}) {
			t.Errorf("result %d = %+v", i, r)
		}
	}
	if got := len(f.calls("/productSubscriptions")); got != 2 {
		t.Errorf("calls = %d, want 2", got)
	}
}

func TestErrorsAreNormalisedAndRegionsSkipped(t *testing.T) {
	// us-east-1 is denied, eu-west-1 works: the denied region is skipped and
	// the scan succeeds. When every region fails the last error surfaces.
	f := &fakeHub{handler: func(r recorded) (int, interface{}, string) {
		if r.region == "us-east-1" {
			return 403, map[string]string{"message": "User is not authorized"}, "AccessDeniedException"
		}
		return 200, map[string]interface{}{"StandardsSubscriptions": []map[string]string{{"StandardsArn": "arn:aws:securityhub:::standards/x/v/1"}}}, ""
	}}
	res, err := run(t, newEnv(t, f, "us-east-1", "eu-west-1"), svcStandards)
	if err != nil || len(res) != 1 || res[0].ResourceName != "x" || res[0].Details["Region"] != "eu-west-1" {
		t.Fatalf("mixed regions: res=%+v err=%v", res, err)
	}

	all := &fakeHub{handler: func(r recorded) (int, interface{}, string) {
		return 403, map[string]string{"message": "User is not authorized"}, "AccessDeniedException"
	}}
	for i := range SecurityHubCalls {
		res, err := run(t, newEnv(t, all, "us-east-1", "eu-west-1"), i)
		if err == nil || err.Error() != "AccessDeniedException: User is not authorized" {
			t.Errorf("%s: err = %v, want normalised coded error", SecurityHubCalls[i].Name, err)
		}
		if len(res) != 1 || res[0].Error == nil || res[0].Error.Error() != "AccessDeniedException: User is not authorized" {
			t.Errorf("%s: results = %+v", SecurityHubCalls[i].Name, res)
		}
	}
}

func TestEveryScopeRegionIsVisitedWithItsSigningRegion(t *testing.T) {
	scope := []string{"us-east-1", "eu-west-1", "ap-southeast-2"}
	f := &fakeHub{handler: func(r recorded) (int, interface{}, string) {
		switch r.path {
		case "/standards/get":
			return 200, map[string]interface{}{"StandardsSubscriptions": []map[string]string{}}, ""
		case "/findings":
			return 200, map[string]interface{}{"Findings": []map[string]string{}}, ""
		}
		return 200, map[string]interface{}{"ProductSubscriptions": []string{}}, ""
	}}
	ctx := newEnv(t, f, scope...)
	for i := range SecurityHubCalls {
		if _, err := run(t, ctx, i); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"/standards/get", "/findings", "/productSubscriptions"} {
		var got []string
		for _, r := range f.calls(path) {
			got = append(got, r.region)
		}
		sort.Strings(got)
		want := append([]string(nil), scope...)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s visited %v, want %v", path, got, want)
		}
	}
}
