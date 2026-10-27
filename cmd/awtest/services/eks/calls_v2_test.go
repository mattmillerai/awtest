package eks

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

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/eks/aws4_request`)

type recorded struct {
	region string
	path   string
}

// fakeEKS speaks the EKS REST-JSON protocol for ListClusters (GET /clusters)
// and DescribeCluster (GET /clusters/{name}). Data is keyed by the SigV4
// region.
type fakeEKS struct {
	clusters map[string][]map[string]interface{} // region -> described clusters (by "name")
	// denyList maps region -> AccessDeniedException message for ListClusters.
	denyList map[string]string
	// denyDescribe lists "region/name" clusters whose DescribeCluster fails.
	denyDescribe map[string]bool
	// nodegroups is keyed "region/cluster"; moreGroups makes ListNodegroups
	// report a further page.
	nodegroups map[string][]string
	moreGroups bool

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeEKS) fail(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("X-Amzn-Errortype", code)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": msg})
}

func (f *fakeEKS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, r.URL.Path})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/clusters":
		if msg, ok := f.denyList[region]; ok {
			f.fail(w, http.StatusForbidden, "AccessDeniedException", msg)
			return
		}
		var names []string
		for _, c := range f.clusters[region] {
			names = append(names, c["name"].(string))
		}
		out := map[string]interface{}{"clusters": names}
		if len(names) > 0 {
			// v1 ignored the token; so must the migrated code.
			out["nextToken"] = "more"
		}
		_ = json.NewEncoder(w).Encode(out)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/clusters/") && strings.HasSuffix(r.URL.Path, "/node-groups"):
		cluster := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/clusters/"), "/node-groups")
		out := map[string]interface{}{"nodegroups": f.nodegroups[region+"/"+cluster]}
		if f.moreGroups {
			out["nextToken"] = "more"
		}
		_ = json.NewEncoder(w).Encode(out)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/clusters/"):
		name := strings.TrimPrefix(r.URL.Path, "/clusters/")
		if f.denyDescribe[region+"/"+name] {
			f.fail(w, http.StatusForbidden, "AccessDeniedException", "cannot describe "+name)
			return
		}
		for _, c := range f.clusters[region] {
			if c["name"] == name {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"cluster": c})
				return
			}
		}
		f.fail(w, http.StatusNotFound, "ResourceNotFoundException", "no cluster "+name)
	default:
		f.fail(w, http.StatusBadRequest, "BadRequestException", r.Method+" "+r.URL.Path)
	}
}

func (f *fakeEKS) regionsOf(path string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.reqs {
		if r.path == path {
			out = append(out, r.region)
		}
	}
	return out
}

func newEnv(t *testing.T, f *fakeEKS) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_EKS", unreachable)

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

func names(results []types.ScanResult) []string {
	var out []string
	for _, r := range results {
		out = append(out, r.ResourceName)
	}
	return out
}

func TestServiceIsV2Only(t *testing.T) {
	want := []string{"eks:ListClusters", "eks:ListNodegroups"}
	if len(EKSCalls) != len(want) {
		t.Fatalf("unexpected services: %+v", EKSCalls)
	}
	for i, n := range want {
		if EKSCalls[i].Name != n || EKSCalls[i].CallV2 == nil {
			t.Errorf("service %d = %q, want %q with CallV2 set", i, EKSCalls[i].Name, n)
		}
	}
}

func TestListNodegroups(t *testing.T) {
	f := &fakeEKS{
		clusters: map[string][]map[string]interface{}{
			"us-east-1": {{"name": "prod"}, {"name": "empty"}},
			"us-west-1": {{"name": "dev"}},
		},
		nodegroups: map[string][]string{
			"us-east-1/prod": {"ng-a", "ng-b"},
			"us-west-1/dev":  {"ng-c"},
		},
	}
	ctx := newEnv(t, f)
	svc := EKSCalls[1]
	out, err := svc.Invoke(ctx)
	res := svc.Process(out, err, false)
	var got []string
	for _, r := range res {
		if r.ResourceType == "note" {
			// The fake reports a further cluster page for every region it lists.
			continue
		}
		if r.Error != nil || r.MethodName != "eks:ListNodegroups" || r.ResourceType != "nodegroup" {
			t.Errorf("unexpected result %+v", r)
		}
		got = append(got, r.Details["region"].(string)+":"+r.ResourceName)
	}
	if want := []string{"us-east-1:prod/ng-a", "us-east-1:prod/ng-b", "us-west-1:dev/ng-c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("nodegroups = %v, want %v", got, want)
	}
	if g := f.regionsOf("/clusters/prod/node-groups"); !reflect.DeepEqual(g, []string{"us-east-1"}) {
		t.Errorf("prod node-groups regions = %v", g)
	}

	// A further page is noted, not followed.
	f.moreGroups = true
	f.reqs = nil
	out, err = svc.Invoke(ctx)
	res = svc.Process(out, err, false)
	notes := 0
	for _, r := range res {
		// Per-cluster scopes ("<region>/<cluster>") only.
		if scope, _ := r.Details["scope"].(string); r.ResourceType == "note" && strings.Contains(scope, "/") {
			notes++
		}
	}
	if notes != 3 || len(f.regionsOf("/clusters/prod/node-groups")) != 1 {
		t.Errorf("notes = %d, results = %+v", notes, res)
	}
}

func TestListNodegroupsDenied(t *testing.T) {
	f := &fakeEKS{denyList: map[string]string{"us-east-1": "nope"}}
	ctx := newEnv(t, f)
	svc := EKSCalls[1]
	out, err := svc.Invoke(ctx)
	res := svc.Process(out, err, false)
	if len(res) != 1 || res[0].Error == nil || !strings.HasPrefix(res[0].Error.Error(), "AccessDeniedException") {
		t.Errorf("results = %+v", res)
	}
}

func TestListClustersThroughInvoke(t *testing.T) {
	f := &fakeEKS{clusters: map[string][]map[string]interface{}{
		"us-east-1": {{
			"name":     "prod",
			"arn":      "arn:aws:eks:us-east-1:123456789012:cluster/prod",
			"status":   "ACTIVE",
			"version":  "1.29",
			"endpoint": "https://ABC.gr7.us-east-1.eks.amazonaws.com",
			"roleArn":  "arn:aws:iam::123456789012:role/eks",
			"resourcesVpcConfig": map[string]interface{}{
				"vpcId":            "vpc-123",
				"subnetIds":        []string{"subnet-1", "subnet-2"},
				"securityGroupIds": []string{"sg-1"},
			},
		}},
		// Sparse: only a name, no VPC config.
		"us-west-1": {{"name": "bare"}},
	}}
	ctx := newEnv(t, f)
	regions := types.RegionsFor(ctx)

	svc := EKSCalls[0]
	out, err := svc.Invoke(ctx)
	if err != nil {
		t.Fatal(err)
	}
	results := svc.Process(out, err, false)

	if got := f.regionsOf("/clusters"); !reflect.DeepEqual(got, regions) {
		t.Errorf("ListClusters regions = %v, want %v (in order, one request each, token ignored)", got, regions)
	}
	// Each DescribeCluster is signed for the region its cluster lives in.
	if got := f.regionsOf("/clusters/prod"); !reflect.DeepEqual(got, []string{"us-east-1"}) {
		t.Errorf("DescribeCluster prod regions = %v", got)
	}
	if got := f.regionsOf("/clusters/bare"); !reflect.DeepEqual(got, []string{"us-west-1"}) {
		t.Errorf("DescribeCluster bare regions = %v", got)
	}

	if got, want := names(results), []string{"prod", "bare"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("resources = %v, want %v", got, want)
	}
	for _, r := range results {
		if r.ServiceName != "EKS" || r.MethodName != "eks:ListClusters" || r.ResourceType != "cluster" ||
			r.Error != nil || r.Timestamp.IsZero() {
			t.Errorf("unexpected result %+v", r)
		}
	}
	wantFull := map[string]interface{}{
		"Arn":            "arn:aws:eks:us-east-1:123456789012:cluster/prod",
		"Status":         "ACTIVE",
		"Version":        "1.29",
		"Endpoint":       "https://ABC.gr7.us-east-1.eks.amazonaws.com",
		"RoleArn":        "arn:aws:iam::123456789012:role/eks",
		"VpcId":          "vpc-123",
		"Subnets":        2,
		"SecurityGroups": 1,
	}
	if !reflect.DeepEqual(results[0].Details, wantFull) {
		t.Errorf("populated details = %#v, want %#v", results[0].Details, wantFull)
	}
	wantSparse := map[string]interface{}{
		"Arn": "", "Status": "", "Version": "", "Endpoint": "", "RoleArn": "",
		"VpcId": "", "Subnets": 0, "SecurityGroups": 0,
	}
	if !reflect.DeepEqual(results[1].Details, wantSparse) {
		t.Errorf("sparse details = %#v, want %#v", results[1].Details, wantSparse)
	}
}

func TestFailedDescribeClusterIsSkipped(t *testing.T) {
	f := &fakeEKS{
		clusters: map[string][]map[string]interface{}{
			"us-east-1": {{"name": "denied"}, {"name": "ok"}},
		},
		denyDescribe: map[string]bool{"us-east-1/denied": true},
	}
	ctx := newEnv(t, f)
	svc := EKSCalls[0]
	out, err := svc.Invoke(ctx)
	if err != nil {
		t.Fatalf("per-cluster errors must be swallowed, got %v", err)
	}
	if got, want := names(svc.Process(out, err, false)), []string{"ok"}; !reflect.DeepEqual(got, want) {
		t.Errorf("resources = %v, want %v", got, want)
	}
}

func TestListClustersAccessDeniedNormalized(t *testing.T) {
	const msg = "User is not authorized to perform: eks:ListClusters"
	f := &fakeEKS{denyList: map[string]string{"us-east-2": msg}}
	ctx := newEnv(t, f)
	svc := EKSCalls[0]
	_, err := svc.Invoke(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	if want := "AccessDeniedException: " + msg; err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if code := awsv2.ErrorCode(err); code != "AccessDeniedException" {
		t.Errorf("ErrorCode = %q", code)
	}
	for _, noise := range []string{"status code", "request id", "operation error", "403"} {
		if strings.Contains(strings.ToLower(err.Error()), noise) {
			t.Errorf("error %q contains transport noise %q", err, noise)
		}
	}
	// v1 aborted the whole call at the first failing region.
	if got, want := f.regionsOf("/clusters"), []string{"us-east-1", "us-east-2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("regions visited = %v, want %v", got, want)
	}
	results := svc.Process(nil, err, false)
	if len(results) != 1 || results[0].Error == nil || results[0].ServiceName != "EKS" {
		t.Errorf("error result = %+v", results)
	}
}
