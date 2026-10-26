package elasticache

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

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/elasticache/aws4_request`)

type recorded struct {
	region string
	action string
	form   map[string]string
}

// ccSpec is a cache cluster returned by DescribeCacheClusters. An empty field
// is omitted from the XML.
type ccSpec struct {
	id, engine, version, nodeType, status, nodes, az string
}

// fakeElastiCache speaks the ElastiCache query (XML) protocol for
// DescribeCacheClusters. Data is keyed by the SigV4 region.
type fakeElastiCache struct {
	clusters map[string][]ccSpec
	// page > 0 paginates with Marker "m-<n>" (n = index of the next cluster).
	page int
	// deny maps region -> AccessDenied message.
	deny map[string]string

	mu   sync.Mutex
	reqs []recorded
}

func tag(name, v string) string {
	if v == "" {
		return ""
	}
	return "<" + name + ">" + v + "</" + name + ">"
}

func (f *fakeElastiCache) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	_ = r.ParseForm()
	form := map[string]string{}
	for k := range r.PostForm {
		form[k] = r.PostForm.Get(k)
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, form["Action"], form})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "text/xml")
	if form["Action"] != "DescribeCacheClusters" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `<ErrorResponse><Error><Type>Sender</Type><Code>InvalidAction</Code><Message>%s</Message></Error><RequestId>r</RequestId></ErrorResponse>`, form["Action"])
		return
	}
	if msg, ok := f.deny[region]; ok {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `<ErrorResponse><Error><Type>Sender</Type><Code>AccessDenied</Code><Message>%s</Message></Error><RequestId>r</RequestId></ErrorResponse>`, msg)
		return
	}
	list := f.clusters[region]
	start, end := 0, len(list)
	if m := form["Marker"]; m != "" {
		start, _ = strconv.Atoi(strings.TrimPrefix(m, "m-"))
	}
	marker := ""
	if f.page > 0 && start+f.page < end {
		end = start + f.page
		marker = fmt.Sprintf("m-%d", end)
	}
	var sb strings.Builder
	sb.WriteString(`<DescribeCacheClustersResponse xmlns="http://elasticache.amazonaws.com/doc/2015-02-02/"><DescribeCacheClustersResult>`)
	sb.WriteString(tag("Marker", marker))
	sb.WriteString("<CacheClusters>")
	for _, c := range list[start:end] {
		sb.WriteString("<CacheCluster>")
		sb.WriteString(tag("CacheClusterId", c.id) + tag("Engine", c.engine) + tag("EngineVersion", c.version) +
			tag("CacheNodeType", c.nodeType) + tag("CacheClusterStatus", c.status) + tag("NumCacheNodes", c.nodes) +
			tag("PreferredAvailabilityZone", c.az))
		sb.WriteString("</CacheCluster>")
	}
	sb.WriteString("</CacheClusters></DescribeCacheClustersResult><ResponseMetadata><RequestId>r</RequestId></ResponseMetadata></DescribeCacheClustersResponse>")
	_, _ = w.Write([]byte(sb.String()))
}

func (f *fakeElastiCache) reqsOf() []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recorded(nil), f.reqs...)
}

func (f *fakeElastiCache) regions() []string {
	var out []string
	for _, r := range f.reqsOf() {
		out = append(out, r.region)
	}
	return out
}

func newEnv(t *testing.T, f *fakeElastiCache) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_ELASTICACHE", unreachable)

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
	svc := ElastiCacheCalls[0]
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

func TestServiceIsV2Only(t *testing.T) {
	if len(ElastiCacheCalls) != 1 || ElastiCacheCalls[0].Name != "elasticache:DescribeCacheClusters" {
		t.Fatalf("unexpected services: %+v", ElastiCacheCalls)
	}
	if ElastiCacheCalls[0].CallV2 == nil {
		t.Error("want CallV2 set and Call nil")
	}
}

func TestDescribeCacheClustersThroughInvoke(t *testing.T) {
	f := &fakeElastiCache{
		clusters: map[string][]ccSpec{
			"us-east-1": {
				{"redis-1", "redis", "7.0.7", "cache.t3.micro", "available", "1", "us-east-1a"},
				{id: "bare"}, // sparse: only the id
			},
			"us-west-1": {{"mc-1", "memcached", "1.6.22", "cache.m5.large", "creating", "3", "us-west-1b"}},
		},
		page: 1, // exercises the Marker loop
	}
	ctx := newEnv(t, f)

	results, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := f.regions(), []string{"us-east-1", "us-east-1", "us-east-2", "us-west-1", "us-west-2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("regions = %v, want %v", got, want)
	}
	for i, r := range f.reqsOf() {
		if r.form["ShowCacheNodeInfo"] != "true" {
			t.Errorf("request %d ShowCacheNodeInfo = %q, want true", i, r.form["ShowCacheNodeInfo"])
		}
	}
	if got, want := names(results), []string{"redis-1", "bare", "mc-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("resources = %v, want %v", got, want)
	}
	for _, r := range results {
		if r.ServiceName != "ElastiCache" || r.MethodName != "elasticache:DescribeCacheClusters" ||
			r.ResourceType != "cluster" || r.Error != nil || r.Timestamp.IsZero() {
			t.Errorf("unexpected result %+v", r)
		}
	}
	wantFull := map[string]interface{}{
		"Engine":                    "redis",
		"EngineVersion":             "7.0.7",
		"CacheNodeType":             "cache.t3.micro",
		"CacheClusterStatus":        "available",
		"NumCacheNodes":             int64(1),
		"PreferredAvailabilityZone": "us-east-1a",
	}
	if !reflect.DeepEqual(results[0].Details, wantFull) {
		t.Errorf("populated details = %#v, want %#v", results[0].Details, wantFull)
	}
	wantSparse := map[string]interface{}{
		"Engine": "", "EngineVersion": "", "CacheNodeType": "", "CacheClusterStatus": "",
		"NumCacheNodes": int64(0), "PreferredAvailabilityZone": "",
	}
	if !reflect.DeepEqual(results[1].Details, wantSparse) {
		t.Errorf("sparse details = %#v, want %#v", results[1].Details, wantSparse)
	}
	if results[2].Details["NumCacheNodes"] != int64(3) {
		t.Errorf("mc-1 NumCacheNodes = %v", results[2].Details["NumCacheNodes"])
	}
}

func TestFailingRegionSkippedButAllFailingReturnsError(t *testing.T) {
	// One failing region is skipped and the others still scan.
	f := &fakeElastiCache{
		clusters: map[string][]ccSpec{"us-west-2": {{id: "kept"}}},
		deny:     map[string]string{"us-east-1": "denied east"},
	}
	results, err := run(newEnv(t, f))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := names(results), []string{"kept"}; !reflect.DeepEqual(got, want) {
		t.Errorf("resources = %v, want %v", got, want)
	}
	if got, want := f.regions(), []string{"us-east-1", "us-east-2", "us-west-1", "us-west-2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("regions = %v, want %v", got, want)
	}

	// Every region failing returns the last error, normalized.
	const msg = "User is not authorized to perform: elasticache:DescribeCacheClusters"
	f = &fakeElastiCache{deny: map[string]string{}}
	for _, r := range types.Regions {
		f.deny[r] = msg
	}
	results, err = run(newEnv(t, f))
	if err == nil {
		t.Fatal("expected error when every region fails")
	}
	if want := "AccessDenied: " + msg; err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if code := awsv2.ErrorCode(err); code != "AccessDenied" {
		t.Errorf("ErrorCode = %q", code)
	}
	for _, noise := range []string{"status code", "request id", "operation error", "403"} {
		if strings.Contains(strings.ToLower(err.Error()), noise) {
			t.Errorf("error %q contains transport noise %q", err, noise)
		}
	}
	if len(results) != 1 || results[0].Error == nil || results[0].ServiceName != "ElastiCache" {
		t.Errorf("error result = %+v", results)
	}
}
