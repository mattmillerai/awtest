package ecs

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

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/ecs/aws4_request`)

const targetPrefix = "AmazonEC2ContainerServiceV20141113."

type recorded struct {
	region string
	op     string
	body   map[string]interface{}
}

// fakeECS speaks the ECS JSON-1.1 protocol for ListClusters and
// DescribeClusters. Data is keyed by the SigV4 credential-scope region.
type fakeECS struct {
	clusters map[string][]map[string]interface{} // region -> described clusters
	// deny maps region -> AccessDeniedException message for ListClusters.
	deny map[string]string
	// services is keyed "<region>|<cluster name>"; taskDefs by region.
	services     map[string][]string
	taskDefs     map[string][]string
	moreServices bool // ListServices always reports a further page
	taskDefPages int  // > 0: ListTaskDefinitions never ends

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeECS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	raw, _ := io.ReadAll(r.Body)
	body := map[string]interface{}{}
	_ = json.Unmarshal(raw, &body)
	op := strings.TrimPrefix(r.Header.Get("X-Amz-Target"), targetPrefix)
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, op, body})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	switch op {
	case "ListClusters":
		if msg, ok := f.deny[region]; ok {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"__type": "AccessDeniedException", "message": msg})
			return
		}
		var arns []string
		for _, c := range f.clusters[region] {
			arns = append(arns, "arn:aws:ecs:"+region+":123456789012:cluster/"+c["clusterName"].(string))
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"clusterArns": arns})
	case "DescribeClusters":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"clusters": f.clusters[region]})
	case "ListServices":
		cluster, _ := body["cluster"].(string)
		resp := map[string]interface{}{"serviceArns": f.services[region+"|"+arnName(cluster)]}
		if f.moreServices {
			resp["nextToken"] = "more"
		}
		_ = json.NewEncoder(w).Encode(resp)
	case "ListTaskDefinitions":
		tok, _ := body["nextToken"].(string)
		defs := f.taskDefs[region]
		resp := map[string]interface{}{"taskDefinitionArns": defs}
		if f.taskDefPages > 0 {
			// Endless pagination: every page claims another follows.
			resp["nextToken"] = tok + "x"
		} else if len(defs) > 1 && tok == "" {
			resp = map[string]interface{}{"taskDefinitionArns": defs[:1], "nextToken": "p2"}
		} else if tok == "p2" {
			resp = map[string]interface{}{"taskDefinitionArns": defs[1:]}
		}
		_ = json.NewEncoder(w).Encode(resp)
	default:
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"__type": "UnknownOperationException", "message": op})
	}
}

func (f *fakeECS) regionsOf(op string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.reqs {
		if r.op == op {
			out = append(out, r.region)
		}
	}
	return out
}

func newEnv(t *testing.T, f *fakeECS) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_ECS", unreachable)

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

func TestServiceIsV2Only(t *testing.T) {
	want := []string{"ecs:ListClusters", "ecs:ListServices", "ecs:ListTaskDefinitions"}
	if len(ECSCalls) != len(want) {
		t.Fatalf("unexpected services: %+v", ECSCalls)
	}
	for i, n := range want {
		if ECSCalls[i].Name != n || ECSCalls[i].CallV2 == nil {
			t.Errorf("service %d = %q, want %q with CallV2 set", i, ECSCalls[i].Name, n)
		}
	}
}

func TestListServicesAndTaskDefinitions(t *testing.T) {
	f := &fakeECS{
		clusters: map[string][]map[string]interface{}{
			"us-east-1": {{"clusterName": "web"}, {"clusterName": "empty"}},
			"us-west-1": {{"clusterName": "batch"}},
		},
		services: map[string][]string{
			"us-east-1|web":   {"arn:aws:ecs:us-east-1:1:service/web/api", "arn:aws:ecs:us-east-1:1:service/web/worker"},
			"us-west-1|batch": {"arn:aws:ecs:us-west-1:1:service/batch/etl"},
		},
		taskDefs: map[string][]string{
			"us-east-1": {"arn:aws:ecs:us-east-1:1:task-definition/api:3", "arn:aws:ecs:us-east-1:1:task-definition/api:4"},
		},
	}
	ctx := newEnv(t, f)
	regions := types.RegionsFor(ctx)

	svc := ECSCalls[1]
	out, err := svc.Invoke(ctx)
	res := svc.Process(out, err, false)
	var names []string
	for _, r := range res {
		if r.Error != nil || r.MethodName != "ecs:ListServices" || r.ResourceType != "service" {
			t.Errorf("unexpected result %+v", r)
		}
		names = append(names, r.Details["region"].(string)+":"+r.ResourceName)
	}
	if want := []string{"us-east-1:web/api", "us-east-1:web/worker", "us-west-1:batch/etl"}; !reflect.DeepEqual(names, want) {
		t.Errorf("services = %v, want %v", names, want)
	}
	if got := f.regionsOf("ListServices"); !reflect.DeepEqual(got, []string{"us-east-1", "us-east-1", "us-west-1"}) {
		t.Errorf("ListServices regions = %v", got)
	}

	svc = ECSCalls[2]
	out, err = svc.Invoke(ctx)
	res = svc.Process(out, err, false)
	if len(res) != 2 || res[0].ResourceName != "api:3" || res[1].ResourceName != "api:4" || res[0].ResourceType != "task-definition" ||
		res[0].Details["region"] != "us-east-1" {
		t.Errorf("task definitions = %+v", res)
	}
	if got := f.regionsOf("ListTaskDefinitions"); len(got) != len(regions)+1 {
		t.Errorf("ListTaskDefinitions calls = %v (want one per region plus the second us-east-1 page)", got)
	}
	if got := f.reqs[len(f.reqs)-1]; got.body["status"] != nil && got.body["status"] != "ACTIVE" {
		t.Errorf("status = %v", got.body["status"])
	}
}

func TestListServicesAndTaskDefinitionsCaps(t *testing.T) {
	f := &fakeECS{
		clusters:     map[string][]map[string]interface{}{"us-east-1": {{"clusterName": "web"}}},
		services:     map[string][]string{"us-east-1|web": {"arn:aws:ecs:us-east-1:1:service/web/api"}},
		taskDefs:     map[string][]string{"us-east-1": {"arn:aws:ecs:us-east-1:1:task-definition/a:1"}},
		moreServices: true,
		taskDefPages: 1,
	}
	ctx := newEnv(t, f)
	svc := ECSCalls[1]
	out, err := svc.Invoke(ctx)
	res := svc.Process(out, err, false)
	var notes int
	for _, r := range res {
		if r.ResourceType == "note" {
			notes++
		}
	}
	if len(res) != 2 || notes != 1 {
		t.Errorf("services results = %+v", res)
	}

	svc = ECSCalls[2]
	out, err = svc.Invoke(ctx)
	res = svc.Process(out, err, false)
	notes = 0
	for _, r := range res {
		if r.ResourceType == "note" {
			notes++
		}
	}
	if notes != len(types.RegionsFor(ctx)) {
		t.Errorf("task definition notes = %d, want one per region", notes)
	}
	// Each region read exactly maxTaskDefPages pages.
	if got := len(f.regionsOf("ListTaskDefinitions")); got != maxTaskDefPages*len(types.RegionsFor(ctx)) {
		t.Errorf("ListTaskDefinitions calls = %d", got)
	}
}

func TestListServicesDenied(t *testing.T) {
	f := &fakeECS{deny: map[string]string{"us-east-1": "nope"}}
	ctx := newEnv(t, f)
	for _, svc := range ECSCalls[1:2] {
		out, err := svc.Invoke(ctx)
		res := svc.Process(out, err, false)
		if len(res) != 1 || res[0].Error == nil || !strings.HasPrefix(res[0].Error.Error(), "AccessDeniedException") {
			t.Errorf("%s: results = %+v", svc.Name, res)
		}
	}
}

func TestListClustersThroughInvoke(t *testing.T) {
	f := &fakeECS{clusters: map[string][]map[string]interface{}{
		"us-east-1": {{
			"clusterName":         "full",
			"status":              "ACTIVE",
			"runningTasksCount":   3,
			"pendingTasksCount":   1,
			"activeServicesCount": 2,
		}},
		// Sparse: the API omitted everything but the name.
		"us-west-1": {{"clusterName": "sparse"}},
	}}
	ctx := newEnv(t, f)
	regions := types.RegionsFor(ctx)

	svc := ECSCalls[0]
	out, err := svc.Invoke(ctx)
	if err != nil {
		t.Fatal(err)
	}
	results := svc.Process(out, err, false)

	if got := f.regionsOf("ListClusters"); !reflect.DeepEqual(got, regions) {
		t.Errorf("ListClusters regions = %v, want %v (in order, once each)", got, regions)
	}
	// Only regions that listed a cluster get a DescribeClusters follow-up,
	// signed for that same region.
	if got, want := f.regionsOf("DescribeClusters"), []string{"us-east-1", "us-west-1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("DescribeClusters regions = %v, want %v", got, want)
	}

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2: %+v", len(results), results)
	}
	full, sparse := results[0], results[1]
	for _, r := range results {
		if r.ServiceName != "ECS" || r.MethodName != "ecs:ListClusters" || r.ResourceType != "cluster" ||
			r.Error != nil || r.Timestamp.IsZero() {
			t.Errorf("unexpected result %+v", r)
		}
	}
	wantFull := map[string]interface{}{
		"status":          "ACTIVE",
		"running_tasks":   int64(3),
		"pending_tasks":   int64(1),
		"active_services": int64(2),
	}
	if full.ResourceName != "full" || !reflect.DeepEqual(full.Details, wantFull) {
		t.Errorf("full = %q %#v, want details %#v", full.ResourceName, full.Details, wantFull)
	}
	wantSparse := map[string]interface{}{
		"status":          "",
		"running_tasks":   int64(0),
		"pending_tasks":   int64(0),
		"active_services": int64(0),
	}
	if sparse.ResourceName != "sparse" || !reflect.DeepEqual(sparse.Details, wantSparse) {
		t.Errorf("sparse = %q %#v, want details %#v", sparse.ResourceName, sparse.Details, wantSparse)
	}
}

func TestListClustersAccessDeniedNormalized(t *testing.T) {
	const msg = "User is not authorized to perform: ecs:ListClusters"
	f := &fakeECS{deny: map[string]string{"us-east-2": msg}}
	ctx := newEnv(t, f)
	svc := ECSCalls[0]
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
	for _, noise := range []string{"status code", "request id", "operation error", "400"} {
		if strings.Contains(strings.ToLower(err.Error()), noise) {
			t.Errorf("error %q contains transport noise %q", err, noise)
		}
	}
	// v1 aborted the whole call at the first failing region.
	got := f.regionsOf("ListClusters")
	if last := got[len(got)-1]; last != "us-east-2" {
		t.Errorf("regions visited = %v, want to stop at us-east-2", got)
	}
	results := svc.Process(nil, err, false)
	if len(results) != 1 || results[0].Error == nil || results[0].ServiceName != "ECS" {
		t.Errorf("error result = %+v", results)
	}
}
