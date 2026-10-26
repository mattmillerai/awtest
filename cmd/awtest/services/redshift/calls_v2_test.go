package redshift

import (
	"context"
	"fmt"
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

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/redshift/aws4_request`)

// clusterSpec is a cluster returned by the fake. Only non-empty fields are
// emitted, so a spec with just an id is a sparse cluster.
type clusterSpec struct {
	id, nodeType, status, user, db string
	addr, port                     string
	encrypted, nodes               string
}

type recorded struct {
	region string
	action string
	marker string
}

// fakeRedshift speaks the Redshift query/XML protocol for DescribeClusters.
// Data is keyed by the SigV4 credential-scope region.
type fakeRedshift struct {
	clusters map[string][]clusterSpec
	pageSize int               // > 0 paginates with Marker "m<next index>"
	deny     map[string]string // region -> AccessDenied message

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeRedshift) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	_ = r.ParseForm()
	action, marker := r.PostForm.Get("Action"), r.PostForm.Get("Marker")
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, action, marker})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "text/xml")
	if msg, ok := f.deny[region]; ok {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `<ErrorResponse xmlns="http://redshift.amazonaws.com/doc/2012-12-01/"><Error><Type>Sender</Type><Code>AccessDenied</Code><Message>%s</Message></Error><RequestId>req-1</RequestId></ErrorResponse>`, msg)
		return
	}
	if action != "DescribeClusters" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `<ErrorResponse><Error><Type>Sender</Type><Code>InvalidAction</Code><Message>%s</Message></Error><RequestId>req-1</RequestId></ErrorResponse>`, action)
		return
	}

	list := f.clusters[region]
	start, end := 0, len(list)
	if marker != "" {
		start, _ = strconv.Atoi(strings.TrimPrefix(marker, "m"))
	}
	next := ""
	if f.pageSize > 0 && start+f.pageSize < end {
		end = start + f.pageSize
		next = "m" + strconv.Itoa(end)
	}
	var sb strings.Builder
	sb.WriteString(`<DescribeClustersResponse xmlns="http://redshift.amazonaws.com/doc/2012-12-01/"><DescribeClustersResult>`)
	if next != "" {
		sb.WriteString("<Marker>" + next + "</Marker>")
	}
	sb.WriteString("<Clusters>")
	for _, c := range list[start:end] {
		sb.WriteString("<Cluster>")
		for _, kv := range [][2]string{
			{"ClusterIdentifier", c.id}, {"NodeType", c.nodeType}, {"ClusterStatus", c.status},
			{"MasterUsername", c.user}, {"DBName", c.db}, {"Encrypted", c.encrypted}, {"NumberOfNodes", c.nodes},
		} {
			if kv[1] != "" {
				sb.WriteString("<" + kv[0] + ">" + kv[1] + "</" + kv[0] + ">")
			}
		}
		if c.addr != "" || c.port != "" {
			sb.WriteString("<Endpoint>")
			if c.addr != "" {
				sb.WriteString("<Address>" + c.addr + "</Address>")
			}
			if c.port != "" {
				sb.WriteString("<Port>" + c.port + "</Port>")
			}
			sb.WriteString("</Endpoint>")
		}
		sb.WriteString("</Cluster>")
	}
	sb.WriteString(`</Clusters></DescribeClustersResult><ResponseMetadata><RequestId>req-1</RequestId></ResponseMetadata></DescribeClustersResponse>`)
	_, _ = w.Write([]byte(sb.String()))
}

func (f *fakeRedshift) regions() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.reqs {
		out = append(out, r.region)
	}
	return out
}

func newEnv(t *testing.T, f *fakeRedshift) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_REDSHIFT", unreachable)

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
	svc := RedshiftCalls[0]
	out, err := svc.Invoke(ctx)
	return svc.Process(out, err, false), err
}

func TestServiceIsV2Only(t *testing.T) {
	if len(RedshiftCalls) != 1 || RedshiftCalls[0].Name != "redshift:DescribeClusters" {
		t.Fatalf("unexpected services: %+v", RedshiftCalls)
	}
	if RedshiftCalls[0].CallV2 == nil {
		t.Error("want CallV2 set and Call nil")
	}
}

func TestInvokeProcessDetails(t *testing.T) {
	f := &fakeRedshift{clusters: map[string][]clusterSpec{"us-west-2": {
		{id: "full", nodeType: "dc2.large", status: "available", user: "admin", db: "mydb",
			addr: "full.abc.us-west-2.redshift.amazonaws.com", port: "5439", encrypted: "true", nodes: "2"},
		{id: "sparse"},
	}}}
	results, err := run(scoped(newEnv(t, f), "us-west-2"))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results: %+v", len(results), results)
	}
	want := []map[string]interface{}{
		{"NodeType": "dc2.large", "ClusterStatus": "available", "MasterUsername": "admin", "DBName": "mydb",
			"Endpoint": "full.abc.us-west-2.redshift.amazonaws.com:5439", "Encrypted": true, "NumberOfNodes": int64(2)},
		{"NodeType": "", "ClusterStatus": "", "MasterUsername": "", "DBName": "",
			"Endpoint": "", "Encrypted": false, "NumberOfNodes": int64(0)},
	}
	for i, name := range []string{"full", "sparse"} {
		r := results[i]
		if r.ServiceName != "Redshift" || r.MethodName != "redshift:DescribeClusters" || r.ResourceType != "cluster" ||
			r.ResourceName != name || r.Error != nil || r.Timestamp.IsZero() {
			t.Errorf("unexpected result %+v", r)
		}
		if !reflect.DeepEqual(r.Details, want[i]) {
			t.Errorf("%s details = %#v, want %#v", name, r.Details, want[i])
		}
	}
}

func TestEveryRegionInScopeIsVisited(t *testing.T) {
	f := &fakeRedshift{clusters: map[string][]clusterSpec{
		"us-east-1": {{id: "e1"}}, "eu-west-1": {{id: "w1"}}, "ap-south-1": {{id: "a1"}},
	}}
	ctx := scoped(newEnv(t, f), "us-east-1", "eu-west-1", "ap-south-1")
	results, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.regions(); !reflect.DeepEqual(got, []string{"us-east-1", "eu-west-1", "ap-south-1"}) {
		t.Errorf("signed regions = %v", got)
	}
	var names []string
	for _, r := range results {
		names = append(names, r.ResourceName)
	}
	if !reflect.DeepEqual(names, []string{"e1", "w1", "a1"}) {
		t.Errorf("resources = %v", names)
	}
}

func TestMarkerPagination(t *testing.T) {
	f := &fakeRedshift{
		pageSize: 2,
		clusters: map[string][]clusterSpec{"us-west-2": {{id: "c1"}, {id: "c2"}, {id: "c3"}, {id: "c4"}, {id: "c5"}}},
	}
	results, err := run(scoped(newEnv(t, f), "us-west-2"))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 5 || results[0].ResourceName != "c1" || results[4].ResourceName != "c5" {
		t.Errorf("results = %+v", results)
	}
	var markers []string
	for _, r := range f.reqs {
		markers = append(markers, r.marker)
	}
	if !reflect.DeepEqual(markers, []string{"", "m2", "m4"}) {
		t.Errorf("markers sent = %v", markers)
	}
}

func TestFailingRegionIsSkipped(t *testing.T) {
	f := &fakeRedshift{
		clusters: map[string][]clusterSpec{"us-east-1": {{id: "ok-1"}}, "us-west-2": {{id: "ok-2"}}},
		deny:     map[string]string{"eu-west-1": "denied in eu"},
	}
	results, err := run(scoped(newEnv(t, f), "us-east-1", "eu-west-1", "us-west-2"))
	if err != nil {
		t.Fatalf("partial failure must not be an error: %v", err)
	}
	if len(results) != 2 || results[0].ResourceName != "ok-1" || results[1].ResourceName != "ok-2" {
		t.Errorf("results = %+v", results)
	}
}

func TestAllRegionsFailReturnsLastNormalizedError(t *testing.T) {
	f := &fakeRedshift{deny: map[string]string{"us-east-1": "first denied", "us-west-2": "last denied"}}
	results, err := run(scoped(newEnv(t, f), "us-east-1", "us-west-2"))
	if err == nil {
		t.Fatal("expected error")
	}
	if got, want := err.Error(), "AccessDenied: last denied"; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
	if len(results) != 1 || results[0].Error == nil || results[0].ServiceName != "Redshift" {
		t.Errorf("results = %+v", results)
	}
}
