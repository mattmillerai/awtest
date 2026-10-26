package emr

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

const targetPrefix = "ElasticMapReduce."

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
	t.Setenv("AWS_ENDPOINT_URL_EMR", unreachable)

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
	svc := EMRCalls[i]
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

func TestListClustersInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		switch op {
		case "ListClusters":
			if body["Marker"] == "m2" {
				return http.StatusOK, map[string]interface{}{"Clusters": []map[string]string{{"Id": "j-BARE", "Name": "ignored"}}}
			}
			return http.StatusOK, map[string]interface{}{
				"Clusters": []map[string]string{{"Id": "j-DEMO"}, {"Name": "no-id-skipped"}, {"Id": "j-FAIL"}},
				"Marker":   "m2",
			}
		case "DescribeCluster":
			switch body["ClusterId"] {
			case "j-DEMO":
				return http.StatusOK, map[string]interface{}{"Cluster": map[string]interface{}{
					"Id": "j-DEMO", "Name": "demo-cluster", "ReleaseLabel": "emr-6.10.0",
					"Status": map[string]string{"State": "RUNNING"},
				}}
			case "j-BARE":
				return http.StatusOK, map[string]interface{}{"Cluster": map[string]interface{}{"Id": "j-BARE"}}
			}
			return apiError("InvalidRequestException", "gone")
		}
		return apiError("UnknownOperationException", op)
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{"ClusterId": "j-DEMO", "Name": "demo-cluster", "State": "RUNNING", "ReleaseLabel": "emr-6.10.0", "Region": "us-west-2"},
		{"ClusterId": "j-BARE", "Name": "", "State": "", "ReleaseLabel": "", "Region": "us-west-2"},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ServiceName != "EMR" || results[0].MethodName != "emr:ListClusters" || results[0].ResourceType != "cluster" || results[0].ResourceName != "demo-cluster" {
		t.Errorf("result 0 = %+v", results[0])
	}
	if got := len(f.ops("ListClusters")); got != 2 {
		t.Errorf("ListClusters calls = %d, want 2", got)
	}
	// The cluster without an Id is skipped; the failing DescribeCluster is swallowed.
	if got := len(f.ops("DescribeCluster")); got != 3 {
		t.Errorf("DescribeCluster calls = %d, want 3", got)
	}
}

func TestListInstanceGroupsInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		switch op {
		case "ListClusters":
			return http.StatusOK, map[string]interface{}{"Clusters": []map[string]string{{"Id": "j-1", "Name": "demo-cluster"}, {"Id": "j-2"}}}
		case "ListInstanceGroups":
			if body["ClusterId"] == "j-1" {
				if body["Marker"] == "m2" {
					return http.StatusOK, map[string]interface{}{"InstanceGroups": []map[string]string{{"Id": "ig-bare"}}}
				}
				return http.StatusOK, map[string]interface{}{"InstanceGroups": []map[string]interface{}{{
					"Id": "ig-1", "Name": "Master", "InstanceGroupType": "MASTER", "InstanceType": "m5.xlarge",
					"RequestedInstanceCount": 1, "RunningInstanceCount": 1, "Market": "ON_DEMAND",
					"Status": map[string]string{"State": "RUNNING"},
				}}, "Marker": "m2"}
			}
			return apiError("InvalidRequestException", "cluster j-2 failed")
		}
		return apiError("UnknownOperationException", op)
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"Id": "ig-1", "Name": "Master", "ClusterId": "j-1", "ClusterName": "demo-cluster",
			"InstanceGroupType": "MASTER", "InstanceType": "m5.xlarge", "RequestedInstanceCount": "1",
			"RunningInstanceCount": "1", "Market": "ON_DEMAND", "State": "RUNNING", "Region": "us-west-2",
		},
		{
			"Id": "ig-bare", "Name": "", "ClusterId": "j-1", "ClusterName": "demo-cluster",
			"InstanceGroupType": "", "InstanceType": "", "RequestedInstanceCount": "",
			"RunningInstanceCount": "", "Market": "", "State": "", "Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ResourceType != "instance-group" || results[0].ResourceName != "Master" {
		t.Errorf("result 0 = %+v", results[0])
	}
}

func TestListSecurityConfigurationsInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		if op != "ListSecurityConfigurations" {
			return apiError("UnknownOperationException", op)
		}
		return http.StatusOK, map[string]interface{}{"SecurityConfigurations": []map[string]interface{}{
			{"Name": "demo-config", "CreationDateTime": 1700000000},
			{"Name": "bare-config"},
		}}
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{"Name": "demo-config", "CreationDateTime": "2023-11-14T22:13:20Z", "Region": "us-west-2"},
		{"Name": "bare-config", "CreationDateTime": "", "Region": "us-west-2"},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ResourceType != "security-configuration" || results[0].ResourceName != "demo-config" {
		t.Errorf("result 0 = %+v", results[0])
	}
}

func TestInvokeErrorText(t *testing.T) {
	for i, op := range []string{"ListClusters", "ListClusters", "ListSecurityConfigurations"} {
		f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
			return apiError("AccessDeniedException", "not authorized for "+op)
		}}
		ctx := newEnv(t, f, "us-west-2")
		results, err := run(ctx, i)
		if err == nil {
			t.Fatalf("%s: expected an error", op)
		}
		want := "AccessDeniedException: not authorized for " + op
		if err.Error() != want {
			t.Errorf("%s: error = %q, want %q", op, err.Error(), want)
		}
		if len(results) != 1 || results[0].Error == nil || results[0].Error.Error() != want {
			t.Errorf("%s: Process error result = %+v", op, results)
		}
	}
}

func TestRegionScopeVisitsEachRegion(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		switch op {
		case "ListClusters":
			return http.StatusOK, map[string]interface{}{"Clusters": []map[string]string{{"Id": "j-" + region, "Name": "n-" + region}}}
		case "DescribeCluster":
			return http.StatusOK, map[string]interface{}{"Cluster": map[string]string{"Id": body["ClusterId"].(string), "Name": "c-" + region}}
		case "ListInstanceGroups":
			return http.StatusOK, map[string]interface{}{"InstanceGroups": []map[string]string{{"Id": "ig-" + region}}}
		case "ListSecurityConfigurations":
			return http.StatusOK, map[string]interface{}{"SecurityConfigurations": []map[string]string{{"Name": "s-" + region}}}
		}
		return apiError("UnknownOperationException", op)
	}}
	scope := []string{"us-east-1", "eu-west-1", "ap-south-1"}
	for _, tc := range []struct {
		idx int
		ops []string
	}{
		{0, []string{"ListClusters", "DescribeCluster"}},
		{1, []string{"ListClusters", "ListInstanceGroups"}},
		{2, []string{"ListSecurityConfigurations"}},
	} {
		ctx := newEnv(t, f, scope...)
		f.mu.Lock()
		f.reqs = nil
		f.mu.Unlock()
		results, err := run(ctx, tc.idx)
		if err != nil {
			t.Fatal(err)
		}
		for _, op := range tc.ops {
			if got := f.regionsOf(op); !reflect.DeepEqual(got, scope) {
				t.Errorf("%s SigV4 regions = %v, want %v", op, got, scope)
			}
		}
		var regions []string
		for _, d := range detailsOf(results) {
			regions = append(regions, d["Region"].(string))
		}
		if !reflect.DeepEqual(regions, scope) {
			t.Errorf("%s result regions = %v, want %v", EMRCalls[tc.idx].Name, regions, scope)
		}
	}
}
