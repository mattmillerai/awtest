package opensearch

import (
	"context"
	"encoding/json"
	"fmt"
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

const (
	listPath     = "/2021-01-01/domain"
	describePath = "/2021-01-01/opensearch/domain-info"
)

type recorded struct {
	region string
	path   string
	body   map[string]interface{}
}

// fakeSvc speaks the OpenSearch restJson1 protocol. handle receives the SigV4
// credential-scope region, the request path and the decoded body and returns
// the HTTP status and JSON response.
type fakeSvc struct {
	handle func(region, path string, body map[string]interface{}) (int, interface{})

	mu   sync.Mutex
	reqs []recorded
}

func apiError(code, msg string) (int, interface{}) {
	return http.StatusForbidden, map[string]string{"__type": code, "message": msg}
}

func (f *fakeSvc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	raw, _ := io.ReadAll(r.Body)
	body := map[string]interface{}{}
	_ = json.Unmarshal(raw, &body)
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, r.URL.Path, body})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	var status int
	var out interface{}
	switch {
	case r.URL.Path == listPath && r.Method == http.MethodGet, r.URL.Path == describePath && r.Method == http.MethodPost:
		status, out = f.handle(region, r.URL.Path, body)
	default:
		status, out = apiError("UnknownOperationException", "bad request "+r.Method+" "+r.URL.Path)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(out)
}

func (f *fakeSvc) calls(path string) []recorded {
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

func (f *fakeSvc) regionsOf(path string) []string {
	var out []string
	for _, r := range f.calls(path) {
		out = append(out, r.region)
	}
	return out
}

func (f *fakeSvc) reset() {
	f.mu.Lock()
	f.reqs = nil
	f.mu.Unlock()
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
	t.Setenv("AWS_ENDPOINT_URL_OPENSEARCH", unreachable)

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
	svc := OpenSearchCalls[i]
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

func domainList(names ...string) map[string]interface{} {
	var l []map[string]string
	for _, n := range names {
		l = append(l, map[string]string{"DomainName": n})
	}
	return map[string]interface{}{"DomainNames": l}
}

func requestedNames(body map[string]interface{}) []string {
	var out []string
	if l, ok := body["DomainNames"].([]interface{}); ok {
		for _, n := range l {
			out = append(out, n.(string))
		}
	}
	return out
}

// statusHandler lists names and describes them from statuses.
func statusHandler(names []string, statuses map[string]map[string]interface{}) func(region, path string, body map[string]interface{}) (int, interface{}) {
	return func(region, path string, body map[string]interface{}) (int, interface{}) {
		if path == listPath {
			return http.StatusOK, domainList(names...)
		}
		var list []map[string]interface{}
		for _, n := range requestedNames(body) {
			list = append(list, statuses[n])
		}
		return http.StatusOK, map[string]interface{}{"DomainStatusList": list}
	}
}

func TestListDomainsInvoke(t *testing.T) {
	f := &fakeSvc{handle: statusHandler([]string{"d1", "d2", "d3"}, map[string]map[string]interface{}{
		"d1": {"DomainName": "d1", "ARN": "arn:d1", "Endpoint": "search-d1.us-west-2.es.amazonaws.com", "EngineVersion": "OpenSearch_2.11"},
		// No public endpoint: the VPC endpoint is used.
		"d2": {"DomainName": "d2", "ARN": "arn:d2", "Endpoints": map[string]string{"vpc": "vpc-d2.us-west-2.es.amazonaws.com"}, "EngineVersion": "OpenSearch_1.3"},
		"d3": {"DomainName": "d3"},
	})}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{"Name": "d1", "ARN": "arn:d1", "Endpoint": "search-d1.us-west-2.es.amazonaws.com", "EngineVersion": "OpenSearch_2.11", "Region": "us-west-2"},
		{"Name": "d2", "ARN": "arn:d2", "Endpoint": "vpc-d2.us-west-2.es.amazonaws.com", "EngineVersion": "OpenSearch_1.3", "Region": "us-west-2"},
		{"Name": "d3", "ARN": "", "Endpoint": "", "EngineVersion": "", "Region": "us-west-2"},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ServiceName != "OpenSearch" || results[0].MethodName != "opensearch:ListDomains" || results[0].ResourceType != "domain" || results[0].ResourceName != "d1" {
		t.Errorf("result 0 = %+v", results[0])
	}
}

func TestDescribeDomainAccessPoliciesInvoke(t *testing.T) {
	f := &fakeSvc{handle: statusHandler([]string{"d1", "d2", "d3"}, map[string]map[string]interface{}{
		"d1": {"DomainName": "d1", "AccessPolicies": `{"Version":"2012-10-17"}`},
		"d2": {"DomainName": "d2", "AccessPolicies": ""}, // empty policy is skipped
		"d3": {"DomainName": "d3"},                       // absent policy is skipped
	})}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{"DomainName": "d1", "AccessPolicy": `{"Version":"2012-10-17"}`, "Region": "us-west-2"},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].MethodName != "opensearch:DescribeDomainAccessPolicies" || results[0].ResourceType != "access-policy" || results[0].ResourceName != "d1" {
		t.Errorf("result 0 = %+v", results[0])
	}
}

func TestDescribeDomainEncryptionInvoke(t *testing.T) {
	f := &fakeSvc{handle: statusHandler([]string{"d1", "d2"}, map[string]map[string]interface{}{
		"d1": {
			"DomainName":                  "d1",
			"EncryptionAtRestOptions":     map[string]interface{}{"Enabled": true, "KmsKeyId": "kms-key"},
			"NodeToNodeEncryptionOptions": map[string]interface{}{"Enabled": true},
		},
		"d2": {"DomainName": "d2"},
	})}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{"DomainName": "d1", "EncryptionAtRestEnabled": true, "KmsKeyId": "kms-key", "NodeToNodeEncryptionEnabled": true, "Region": "us-west-2"},
		{"DomainName": "d2", "EncryptionAtRestEnabled": false, "KmsKeyId": "", "NodeToNodeEncryptionEnabled": false, "Region": "us-west-2"},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].MethodName != "opensearch:DescribeDomainEncryption" || results[0].ResourceType != "encryption-config" || results[0].ResourceName != "d1" {
		t.Errorf("result 0 = %+v", results[0])
	}
}

// Domain names are described in batches of describeDomainsBatchSize, in order,
// and names the API returned without a DomainName are not requested.
func TestDescribeDomainsBatches(t *testing.T) {
	var names []string
	statuses := map[string]map[string]interface{}{}
	for i := 1; i <= 7; i++ {
		n := fmt.Sprintf("d%d", i)
		names = append(names, n)
		statuses[n] = map[string]interface{}{"DomainName": n}
	}
	h := statusHandler(names, statuses)
	f := &fakeSvc{handle: func(region, path string, body map[string]interface{}) (int, interface{}) {
		if path == listPath {
			l := domainList(names...)
			l["DomainNames"] = append(l["DomainNames"].([]map[string]string), map[string]string{"EngineType": "OpenSearch"})
			return http.StatusOK, l
		}
		return h(region, path, body)
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 7 {
		t.Fatalf("results = %d, want 7", len(results))
	}
	calls := f.calls(describePath)
	if len(calls) != 2 {
		t.Fatalf("DescribeDomains calls = %d, want 2", len(calls))
	}
	if got := requestedNames(calls[0].body); !reflect.DeepEqual(got, names[:5]) {
		t.Errorf("batch 1 = %v", got)
	}
	if got := requestedNames(calls[1].body); !reflect.DeepEqual(got, names[5:]) {
		t.Errorf("batch 2 = %v", got)
	}
}

// A failing batch is skipped (partial recovery); the other batches still count.
func TestDescribeDomainsBatchErrorContinues(t *testing.T) {
	var names []string
	for i := 1; i <= 7; i++ {
		names = append(names, fmt.Sprintf("d%d", i))
	}
	f := &fakeSvc{handle: func(region, path string, body map[string]interface{}) (int, interface{}) {
		if path == listPath {
			return http.StatusOK, domainList(names...)
		}
		req := requestedNames(body)
		if req[0] == "d1" {
			return apiError("AccessDeniedException", "first batch denied")
		}
		var list []map[string]string
		for _, n := range req {
			list = append(list, map[string]string{"DomainName": n})
		}
		return http.StatusOK, map[string]interface{}{"DomainStatusList": list}
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range results {
		got = append(got, r.ResourceName)
	}
	if want := []string{"d6", "d7"}; !reflect.DeepEqual(got, want) {
		t.Errorf("domains = %v, want %v", got, want)
	}
	if n := len(f.calls(describePath)); n != 2 {
		t.Errorf("DescribeDomains calls = %d, want 2", n)
	}
}

// A ListDomainNames failure ends the whole region loop: later regions are not
// queried, and with nothing found the error is returned.
func TestListDomainNamesErrorStopsRegionLoop(t *testing.T) {
	f := &fakeSvc{handle: func(region, path string, body map[string]interface{}) (int, interface{}) {
		if path == listPath && region == "eu-west-1" {
			return apiError("AccessDeniedException", "denied in "+region)
		}
		if path == listPath {
			return http.StatusOK, domainList("d-" + region)
		}
		return http.StatusOK, map[string]interface{}{"DomainStatusList": []map[string]string{{"DomainName": "d-" + region}}}
	}}
	ctx := newEnv(t, f, "us-east-1", "eu-west-1", "ap-south-1")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ResourceName != "d-us-east-1" {
		t.Errorf("results = %+v, want only d-us-east-1", results)
	}
	if got, want := f.regionsOf(listPath), []string{"us-east-1", "eu-west-1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ListDomainNames regions = %v, want %v (ap-south-1 must not be visited)", got, want)
	}

	f2 := &fakeSvc{handle: func(region, path string, body map[string]interface{}) (int, interface{}) {
		return apiError("AccessDeniedException", "denied in "+region)
	}}
	for i := range OpenSearchCalls {
		ctx = newEnv(t, f2, "eu-west-1")
		results, err = run(ctx, i)
		if err == nil || err.Error() != "AccessDeniedException: denied in eu-west-1" {
			t.Errorf("%s: err = %v", OpenSearchCalls[i].Name, err)
		}
		if len(results) != 1 || results[0].Error == nil || results[0].MethodName != OpenSearchCalls[i].Name {
			t.Errorf("%s: results = %+v", OpenSearchCalls[i].Name, results)
		}
	}
}

func TestRegionScopeVisitsEachRegion(t *testing.T) {
	f := &fakeSvc{handle: func(region, path string, body map[string]interface{}) (int, interface{}) {
		if path == listPath {
			return http.StatusOK, domainList("d-" + region)
		}
		return http.StatusOK, map[string]interface{}{"DomainStatusList": []map[string]interface{}{{
			"DomainName": "d-" + region, "AccessPolicies": "{}",
		}}}
	}}
	scope := []string{"us-east-1", "eu-west-1", "ap-south-1"}
	for i := range OpenSearchCalls {
		ctx := newEnv(t, f, scope...)
		f.reset()
		results, err := run(ctx, i)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range []string{listPath, describePath} {
			if got := f.regionsOf(p); !reflect.DeepEqual(got, scope) {
				t.Errorf("%s %s SigV4 regions = %v, want %v", OpenSearchCalls[i].Name, p, got, scope)
			}
		}
		var regions []string
		for _, d := range detailsOf(results) {
			regions = append(regions, d["Region"].(string))
		}
		if !reflect.DeepEqual(regions, scope) {
			t.Errorf("%s result regions = %v, want %v", OpenSearchCalls[i].Name, regions, scope)
		}
	}
}

func TestInvokeErrorText(t *testing.T) {
	f := &fakeSvc{handle: func(region, path string, body map[string]interface{}) (int, interface{}) {
		return apiError("AccessDeniedException", "not authorized for "+path)
	}}
	for i := range OpenSearchCalls {
		ctx := newEnv(t, f, "us-west-2")
		results, err := run(ctx, i)
		if err == nil {
			t.Fatalf("%s: expected an error", OpenSearchCalls[i].Name)
		}
		want := "AccessDeniedException: not authorized for " + listPath
		if err.Error() != want || !strings.HasPrefix(err.Error(), "AccessDeniedException: ") {
			t.Errorf("%s: error = %q, want %q", OpenSearchCalls[i].Name, err.Error(), want)
		}
		if len(results) != 1 || results[0].Error == nil || results[0].Error.Error() != want {
			t.Errorf("%s: Process error result = %+v", OpenSearchCalls[i].Name, results)
		}
	}
}
