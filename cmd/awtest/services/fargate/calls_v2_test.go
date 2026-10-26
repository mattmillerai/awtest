package fargate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strconv"
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

// fakeECS speaks the ECS JSON-1.1 protocol for the three operations the
// fargate service uses. Data is keyed by the SigV4 credential-scope region.
type fakeECS struct {
	clusters map[string][]string // region -> cluster names, in order
	// clusterPage > 0 paginates ListClusters with tokens "c-<n>".
	clusterPage int
	// tasks maps "region/cluster" -> task ids.
	tasks map[string][]string
	// sparse lists task ids that DescribeTasks returns with only a taskArn.
	sparse map[string]bool
	// denyList maps region -> ListClusters AccessDeniedException message.
	denyList map[string]string
	// denyTasks maps "region/cluster" -> code returned by ListTasks.
	denyTasks map[string]string

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeECS) fail(w http.ResponseWriter, code, msg string) {
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"__type": code, "message": msg})
}

func clusterName(arn string) string { return arn[strings.LastIndex(arn, "/")+1:] }

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
	str := func(k string) string { s, _ := body[k].(string); return s }
	switch op {
	case "ListClusters":
		if msg, ok := f.denyList[region]; ok {
			f.fail(w, "AccessDeniedException", msg)
			return
		}
		names := f.clusters[region]
		start, end := 0, len(names)
		if tok := str("nextToken"); tok != "" {
			start, _ = strconv.Atoi(strings.TrimPrefix(tok, "c-"))
		}
		out := map[string]interface{}{}
		if f.clusterPage > 0 && start+f.clusterPage < end {
			end = start + f.clusterPage
			out["nextToken"] = fmt.Sprintf("c-%d", end)
		}
		var arns []string
		for _, n := range names[start:end] {
			arns = append(arns, "arn:aws:ecs:"+region+":123456789012:cluster/"+n)
		}
		out["clusterArns"] = arns
		_ = json.NewEncoder(w).Encode(out)
	case "ListTasks":
		key := region + "/" + clusterName(str("cluster"))
		if code, ok := f.denyTasks[key]; ok {
			f.fail(w, code, "tasks error for "+key)
			return
		}
		var arns []string
		for _, id := range f.tasks[key] {
			arns = append(arns, "arn:aws:ecs:"+region+":123456789012:task/"+clusterName(str("cluster"))+"/"+id)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"taskArns": arns})
	case "DescribeTasks":
		cluster := clusterName(str("cluster"))
		var tasks []map[string]interface{}
		list, _ := body["tasks"].([]interface{})
		for _, a := range list {
			arn := a.(string)
			t := map[string]interface{}{"taskArn": arn}
			if !f.sparse[arn[strings.LastIndex(arn, "/")+1:]] {
				t["clusterArn"] = "arn:aws:ecs:" + region + ":123456789012:cluster/" + cluster
				t["lastStatus"] = "RUNNING"
				t["desiredStatus"] = "RUNNING"
				t["taskDefinitionArn"] = "arn:aws:ecs:" + region + ":123456789012:task-definition/web:7"
				t["cpu"] = "256"
				t["memory"] = "512"
				t["launchType"] = "FARGATE"
			}
			tasks = append(tasks, t)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"tasks": tasks})
	default:
		f.fail(w, "UnknownOperationException", op)
	}
}

func (f *fakeECS) ops(op string) []recorded {
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

func (f *fakeECS) regionsOf(op string) []string {
	var out []string
	for _, r := range f.ops(op) {
		out = append(out, r.region)
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

func run(ctx context.Context) ([]types.ScanResult, error) {
	svc := FargateCalls[0]
	out, err := svc.Invoke(ctx)
	return svc.Process(out, err, false), err
}

func names(results []types.ScanResult) []string {
	var out []string
	for _, r := range results {
		out = append(out, r.ResourceName)
	}
	return out
}

func taskARN(region, cluster, id string) string {
	return "arn:aws:ecs:" + region + ":123456789012:task/" + cluster + "/" + id
}

func TestServiceIsV2Only(t *testing.T) {
	if len(FargateCalls) != 1 || FargateCalls[0].Name != "ecs:ListFargateTasks" {
		t.Fatalf("unexpected services: %+v", FargateCalls)
	}
	if FargateCalls[0].CallV2 == nil {
		t.Error("want CallV2 set and Call nil")
	}
}

func TestListFargateTasksThroughInvoke(t *testing.T) {
	f := &fakeECS{
		clusters:    map[string][]string{"us-east-1": {"c1", "c2"}, "us-west-1": {"w1"}},
		clusterPage: 1, // exercises the manual NextToken loop
		tasks: map[string][]string{
			"us-east-1/c1": {"a", "sparse"},
			"us-east-1/c2": {"b"},
			"us-west-1/w1": {"z"},
		},
		sparse: map[string]bool{"sparse": true},
	}
	ctx := newEnv(t, f)
	regions := types.RegionsFor(ctx)

	results, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.regionsOf("ListClusters"); !reflect.DeepEqual(got, []string{
		"us-east-1", "us-east-1", "us-east-2", "us-west-1", "us-west-2",
	}) {
		t.Errorf("ListClusters regions = %v (us-east-1 paged x2; regions %v)", got, regions)
	}
	want := []string{
		taskARN("us-east-1", "c1", "a"), taskARN("us-east-1", "c1", "sparse"),
		taskARN("us-east-1", "c2", "b"), taskARN("us-west-1", "w1", "z"),
	}
	if got := names(results); !reflect.DeepEqual(got, want) {
		t.Fatalf("resources = %v, want %v", got, want)
	}
	// Every task listing asks for FARGATE tasks and is signed for its region.
	for _, r := range f.ops("ListTasks") {
		if r.body["launchType"] != "FARGATE" {
			t.Errorf("ListTasks body = %v, want launchType FARGATE", r.body)
		}
	}
	if got := f.regionsOf("ListTasks"); !reflect.DeepEqual(got, []string{"us-east-1", "us-east-1", "us-west-1"}) {
		t.Errorf("ListTasks regions = %v", got)
	}

	for _, r := range results {
		if r.ServiceName != "Fargate" || r.MethodName != "ecs:ListFargateTasks" || r.ResourceType != "task" ||
			r.Error != nil || r.Timestamp.IsZero() {
			t.Errorf("unexpected result %+v", r)
		}
	}
	wantFull := map[string]interface{}{
		"ClusterArn":        "arn:aws:ecs:us-east-1:123456789012:cluster/c1",
		"LastStatus":        "RUNNING",
		"DesiredStatus":     "RUNNING",
		"TaskDefinitionArn": "arn:aws:ecs:us-east-1:123456789012:task-definition/web:7",
		"Cpu":               "256",
		"Memory":            "512",
		"LaunchType":        "FARGATE",
	}
	if !reflect.DeepEqual(results[0].Details, wantFull) {
		t.Errorf("populated details = %#v, want %#v", results[0].Details, wantFull)
	}
	wantSparse := map[string]interface{}{
		"ClusterArn": "", "LastStatus": "", "DesiredStatus": "", "TaskDefinitionArn": "",
		"Cpu": "", "Memory": "", "LaunchType": "",
	}
	if !reflect.DeepEqual(results[1].Details, wantSparse) {
		t.Errorf("sparse details = %#v, want %#v", results[1].Details, wantSparse)
	}
}

func TestDescribeTasksBatchesOf100(t *testing.T) {
	var ids []string
	for i := 0; i < 101; i++ {
		ids = append(ids, fmt.Sprintf("t%03d", i))
	}
	f := &fakeECS{
		clusters: map[string][]string{"us-east-1": {"big"}},
		tasks:    map[string][]string{"us-east-1/big": ids},
	}
	ctx := newEnv(t, f)
	results, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 101 {
		t.Fatalf("got %d results, want 101", len(results))
	}
	var sizes []int
	for _, r := range f.ops("DescribeTasks") {
		sizes = append(sizes, len(r.body["tasks"].([]interface{})))
	}
	if !reflect.DeepEqual(sizes, []int{100, 1}) {
		t.Errorf("DescribeTasks batch sizes = %v, want [100 1]", sizes)
	}
}

func TestFailedListTasksIsSkipped(t *testing.T) {
	f := &fakeECS{
		clusters:  map[string][]string{"us-east-1": {"bad", "good"}},
		tasks:     map[string][]string{"us-east-1/good": {"ok"}},
		denyTasks: map[string]string{"us-east-1/bad": "AccessDeniedException"},
	}
	results, err := run(newEnv(t, f))
	if err != nil {
		t.Fatalf("per-cluster errors must be swallowed, got %v", err)
	}
	if want := []string{taskARN("us-east-1", "good", "ok")}; !reflect.DeepEqual(names(results), want) {
		t.Errorf("resources = %v, want %v", names(results), want)
	}
}

func TestFailingRegionSkippedButAllFailingReturnsError(t *testing.T) {
	// One failing region is skipped and the others still scan.
	f := &fakeECS{
		clusters: map[string][]string{"us-west-2": {"c"}},
		tasks:    map[string][]string{"us-west-2/c": {"t"}},
		denyList: map[string]string{"us-east-1": "denied east"},
	}
	results, err := run(newEnv(t, f))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{taskARN("us-west-2", "c", "t")}; !reflect.DeepEqual(names(results), want) {
		t.Errorf("resources = %v, want %v", names(results), want)
	}

	// Every region failing returns the last error, normalized.
	const msg = "User is not authorized to perform: ecs:ListClusters"
	f = &fakeECS{denyList: map[string]string{}}
	for _, r := range types.Regions {
		f.denyList[r] = msg
	}
	results, err = run(newEnv(t, f))
	if err == nil {
		t.Fatal("expected error when every region fails")
	}
	if want := "AccessDeniedException: " + msg; err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	for _, noise := range []string{"status code", "request id", "operation error", "400"} {
		if strings.Contains(strings.ToLower(err.Error()), noise) {
			t.Errorf("error %q contains transport noise %q", err, noise)
		}
	}
	if len(results) != 1 || results[0].Error == nil || results[0].ServiceName != "Fargate" {
		t.Errorf("error result = %+v", results)
	}
}
