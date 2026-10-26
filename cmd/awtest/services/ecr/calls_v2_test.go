package ecr

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

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/ecr/aws4_request`)

const targetPrefix = "AmazonEC2ContainerRegistry_V20150921."

// repoSpec is a repository returned by DescribeRepositories. An empty name
// yields a repository entry without a repositoryName.
type repoSpec struct{ name string }

type recorded struct {
	region string
	op     string
	body   map[string]interface{}
}

// fakeECR speaks the ECR JSON-1.1 protocol for the three operations awtest
// uses. Data is keyed by the SigV4 credential-scope region.
type fakeECR struct {
	repos map[string][]repoSpec // region -> repositories
	// repoPageSize > 0 paginates DescribeRepositories with tokens
	// "tok-<region>-<n>" (n = index of the next repository).
	repoPageSize int
	images       map[string][]string // "region/repo" -> image tags
	imagePage    int                 // > 0 paginates ListImages
	// imageErr maps "region/repo" -> page index (0-based) whose ListImages call fails.
	imageErr map[string]int
	policies map[string]string // "region/repo" -> policy text
	// policyErr maps "region/repo" -> error code returned by GetRepositoryPolicy.
	policyErr map[string]string
	// deny maps region -> AccessDeniedException message for DescribeRepositories.
	deny map[string]string

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeECR) fail(w http.ResponseWriter, code, msg string) {
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"__type": code, "message": msg})
}

func (f *fakeECR) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	case "DescribeRepositories":
		if msg, ok := f.deny[region]; ok {
			f.fail(w, "AccessDeniedException", msg)
			return
		}
		repos := f.repos[region]
		start, end := 0, len(repos)
		if tok := str("nextToken"); tok != "" {
			start, _ = strconv.Atoi(strings.TrimPrefix(tok, "tok-"+region+"-"))
		}
		out := map[string]interface{}{}
		if f.repoPageSize > 0 && start+f.repoPageSize < end {
			end = start + f.repoPageSize
			out["nextToken"] = fmt.Sprintf("tok-%s-%d", region, end)
		}
		var list []map[string]string
		for _, rp := range repos[start:end] {
			e := map[string]string{}
			if rp.name != "" {
				e["repositoryName"] = rp.name
				e["repositoryArn"] = "arn:aws:ecr:" + region + ":123456789012:repository/" + rp.name
				e["repositoryUri"] = "123456789012.dkr.ecr." + region + ".amazonaws.com/" + rp.name
			}
			list = append(list, e)
		}
		out["repositories"] = list
		_ = json.NewEncoder(w).Encode(out)
	case "ListImages":
		key := region + "/" + str("repositoryName")
		start := 0
		if tok := str("nextToken"); tok != "" {
			start, _ = strconv.Atoi(strings.TrimPrefix(tok, "img-"))
		}
		page := 0
		if f.imagePage > 0 {
			page = start / f.imagePage
		}
		if p, ok := f.imageErr[key]; ok && p == page {
			f.fail(w, "AccessDeniedException", "not authorized to list images")
			return
		}
		tags := f.images[key]
		end := len(tags)
		out := map[string]interface{}{}
		if f.imagePage > 0 && start+f.imagePage < end {
			end = start + f.imagePage
			out["nextToken"] = fmt.Sprintf("img-%d", end)
		}
		var ids []map[string]string
		for _, tag := range tags[start:end] {
			ids = append(ids, map[string]string{"imageTag": tag, "imageDigest": "sha256:" + tag})
		}
		out["imageIds"] = ids
		_ = json.NewEncoder(w).Encode(out)
	case "GetRepositoryPolicy":
		key := region + "/" + str("repositoryName")
		if code, ok := f.policyErr[key]; ok {
			f.fail(w, code, "policy error for "+key)
			return
		}
		text, ok := f.policies[key]
		if !ok {
			f.fail(w, "RepositoryPolicyNotFoundException", "no policy")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"repositoryName": str("repositoryName"), "policyText": text})
	default:
		f.fail(w, "UnknownOperationException", op)
	}
}

func (f *fakeECR) ops(op string) []recorded {
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

func (f *fakeECR) regionsOf(op string) []string {
	var out []string
	for _, r := range f.ops(op) {
		out = append(out, r.region)
	}
	return out
}

func newEnv(t *testing.T, f *fakeECR) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_ECR", unreachable)

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

func run(ctx context.Context, i int) ([]types.ScanResult, error) {
	svc := ECRCalls[i]
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

const (
	svcDescribe = 0
	svcImages   = 1
	svcPolicy   = 2
)

func TestServicesAreV2Only(t *testing.T) {
	want := []string{"ecr:DescribeRepositories", "ecr:ListImages", "ecr:GetRepositoryPolicy"}
	if len(ECRCalls) != len(want) {
		t.Fatalf("expected %d services, got %d", len(want), len(ECRCalls))
	}
	for i, svc := range ECRCalls {
		if svc.Name != want[i] {
			t.Errorf("service %d name = %q, want %q", i, svc.Name, want[i])
		}
		if svc.CallV2 == nil {
			t.Errorf("%s: want CallV2 set", svc.Name)
		}
	}
}

func TestMultiRegionLoop(t *testing.T) {
	f := &fakeECR{
		repos: map[string][]repoSpec{
			"us-east-1": {{"east1-a"}, {"east1-b"}},
			"us-east-2": {{"east2-a"}},
			"us-west-2": {{"west-a"}},
		},
		images: map[string][]string{
			"us-east-1/east1-a": {"v1", "v2"},
			"us-east-1/east1-b": {"b1"},
			"us-east-2/east2-a": {"e2"},
			"us-west-2/west-a":  {"w1"},
		},
		policies: map[string]string{
			"us-east-1/east1-b": `{"east":1}`,
			"us-west-2/west-a":  `{"west":1}`,
		},
	}
	ctx := newEnv(t, f)
	regions := types.RegionsFor(ctx)
	if len(regions) < 3 {
		t.Fatalf("default regions = %v", regions)
	}

	cases := []struct {
		idx          int
		method, kind string
		want         []string
	}{
		{svcDescribe, "ecr:DescribeRepositories", "repository", []string{"east1-a", "east1-b", "east2-a", "west-a"}},
		{svcImages, "ecr:ListImages", "image", []string{"v1", "v2", "b1", "e2", "w1"}},
		{svcPolicy, "ecr:GetRepositoryPolicy", "repository-policy", []string{"east1-b", "west-a"}},
	}
	for _, c := range cases {
		t.Run(c.method, func(t *testing.T) {
			f.mu.Lock()
			f.reqs = nil
			f.mu.Unlock()
			results, err := run(ctx, c.idx)
			if err != nil {
				t.Fatal(err)
			}
			if got := f.regionsOf("DescribeRepositories"); !reflect.DeepEqual(got, regions) {
				t.Errorf("DescribeRepositories regions = %v, want %v (in order, once each)", got, regions)
			}
			if got := names(results); !reflect.DeepEqual(got, c.want) {
				t.Errorf("resources = %v, want %v", got, c.want)
			}
			for _, r := range results {
				if r.ServiceName != "ECR" || r.MethodName != c.method || r.ResourceType != c.kind ||
					r.Error != nil || r.Timestamp.IsZero() {
					t.Errorf("unexpected result %+v", r)
				}
			}
		})
	}

	// Follow-up calls are signed for the region of the repository they belong to.
	for _, op := range []string{"ListImages", "GetRepositoryPolicy"} {
		for _, r := range f.ops(op) {
			repo, _ := r.body["repositoryName"].(string)
			if !strings.HasPrefix(repo, map[string]string{"us-east-1": "east1", "us-east-2": "east2", "us-west-2": "west"}[r.region]) {
				t.Errorf("%s for %q signed for region %s", op, repo, r.region)
			}
		}
	}
}

func TestPolicyOutputCarriesText(t *testing.T) {
	f := &fakeECR{
		repos:    map[string][]repoSpec{"us-west-2": {{"a"}}},
		policies: map[string]string{"us-west-2/a": `{"Version":"2012-10-17"}`},
	}
	ctx := scoped(newEnv(t, f), "us-west-2")
	out, err := ECRCalls[svcPolicy].Invoke(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []ecrPolicy{{RepoName: "a", PolicyText: `{"Version":"2012-10-17"}`}}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("output = %#v, want %#v", out, want)
	}
	if ops := f.ops("GetRepositoryPolicy"); len(ops) != 1 || ops[0].body["repositoryName"] != "a" {
		t.Errorf("GetRepositoryPolicy requests = %+v", ops)
	}
}

func TestRegionScope(t *testing.T) {
	for i := range ECRCalls {
		t.Run(ECRCalls[i].Name, func(t *testing.T) {
			f := &fakeECR{
				repos: map[string][]repoSpec{
					"us-east-1": {{"east"}},
					"eu-west-1": {{"eu"}},
				},
				images:   map[string][]string{"eu-west-1/eu": {"t"}},
				policies: map[string]string{"eu-west-1/eu": "{}"},
			}
			ctx, scope := types.WithRegionScope(newEnv(t, f), []string{"eu-west-1"})
			results, err := run(ctx, i)
			if err != nil {
				t.Fatal(err)
			}
			if !scope.Used() {
				t.Error("service did not consult the region scope")
			}
			for _, r := range f.reqs {
				if r.region != "eu-west-1" {
					t.Errorf("request %s signed for %s, want only eu-west-1", r.op, r.region)
				}
			}
			if len(results) != 1 {
				t.Errorf("results = %+v", results)
			}
		})
	}
}

func TestNoResourcesYieldsNoResults(t *testing.T) {
	ctx := newEnv(t, &fakeECR{})
	for i := range ECRCalls {
		if results, err := run(ctx, i); err != nil || len(results) != 0 {
			t.Errorf("%s: results = %+v, err = %v", ECRCalls[i].Name, results, err)
		}
	}
}

func TestDescribeRepositoriesPagination(t *testing.T) {
	f := &fakeECR{
		repos: map[string][]repoSpec{
			"eu-west-1": {{"eu-1"}, {"eu-2"}, {"eu-3"}},
			"us-west-2": {{"us-1"}, {"us-2"}},
		},
		repoPageSize: 2,
	}
	ctx := scoped(newEnv(t, f), "eu-west-1", "us-west-2")
	results, err := run(ctx, svcDescribe)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"eu-1", "eu-2", "eu-3", "us-1", "us-2"}; !reflect.DeepEqual(names(results), want) {
		t.Errorf("repositories = %v, want %v", names(results), want)
	}
	reqs := f.ops("DescribeRepositories")
	if len(reqs) != 3 {
		t.Fatalf("want 3 requests (2 pages in eu-west-1, 1 in us-west-2), got %d: %+v", len(reqs), reqs)
	}
	// Tokens embed the region, so a token leaking across regions would show.
	wantTok := []interface{}{nil, "tok-eu-west-1-2", nil}
	wantRegion := []string{"eu-west-1", "eu-west-1", "us-west-2"}
	for i, r := range reqs {
		if r.body["nextToken"] != wantTok[i] || r.region != wantRegion[i] {
			t.Errorf("request %d: region=%s nextToken=%v, want region=%s nextToken=%v", i, r.region, r.body["nextToken"], wantRegion[i], wantTok[i])
		}
	}
}

func TestListImagesPaginationAndSwallowedErrors(t *testing.T) {
	f := &fakeECR{
		repos: map[string][]repoSpec{
			"us-west-2": {{"paged"}, {"broken"}, {"named"}, {""}, {"flaky"}, {"last"}},
		},
		images: map[string][]string{
			"us-west-2/paged":  {"p1", "p2", "p3"},
			"us-west-2/broken": {"never"},
			"us-west-2/named":  {"n1"},
			"us-west-2/flaky":  {"f1", "f2", "f3"},
			"us-west-2/last":   {"l1"},
		},
		imagePage: 2,
		// broken fails on its first page; flaky fails on its second page.
		imageErr: map[string]int{"us-west-2/broken": 0, "us-west-2/flaky": 1},
	}
	ctx := scoped(newEnv(t, f), "us-west-2")
	results, err := run(ctx, svcImages)
	if err != nil {
		t.Fatalf("a failing ListImages must be swallowed, got %v", err)
	}
	// flaky keeps the images of its first page; the failed page ends its loop.
	want := []string{"p1", "p2", "p3", "n1", "f1", "f2", "l1"}
	if !reflect.DeepEqual(names(results), want) {
		t.Errorf("images = %v, want %v", names(results), want)
	}
	for _, r := range results {
		if r.Error != nil {
			t.Errorf("unexpected error result %+v", r)
		}
	}

	var seq []string
	for _, r := range f.ops("ListImages") {
		tok, _ := r.body["nextToken"].(string)
		seq = append(seq, fmt.Sprintf("%v/%s", r.body["repositoryName"], tok))
	}
	wantSeq := []string{
		"paged/", "paged/img-2",
		"broken/",
		"named/",
		"flaky/", "flaky/img-2",
		"last/",
	}
	if !reflect.DeepEqual(seq, wantSeq) {
		t.Errorf("ListImages requests = %v, want %v (a repository without a name is skipped)", seq, wantSeq)
	}
}

func TestListImagesTagAndDigest(t *testing.T) {
	f := &fakeECR{
		repos:  map[string][]repoSpec{"us-west-2": {{"a"}}},
		images: map[string][]string{"us-west-2/a": {"latest"}},
	}
	ctx := scoped(newEnv(t, f), "us-west-2")
	out, err := ECRCalls[svcImages].Invoke(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []ecrImage{{RepoName: "a", ImageTag: "latest", ImageDigest: "sha256:latest"}}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("output = %#v, want %#v", out, want)
	}
}

func TestGetRepositoryPolicySkipsErrors(t *testing.T) {
	f := &fakeECR{
		repos: map[string][]repoSpec{
			"us-west-2": {{"first"}, {"no-policy"}, {"denied"}, {""}, {"throttled"}, {"last"}},
		},
		policies: map[string]string{
			"us-west-2/first": `{"p":"first"}`,
			"us-west-2/last":  `{"p":"last"}`,
		},
		// no-policy has none configured: the fake answers RepositoryPolicyNotFoundException.
		policyErr: map[string]string{
			"us-west-2/denied":    "AccessDeniedException",
			"us-west-2/throttled": "ServerException",
		},
	}
	ctx := scoped(newEnv(t, f), "us-west-2")
	results, err := run(ctx, svcPolicy)
	if err != nil {
		t.Fatalf("per-repository errors must be skipped, got %v", err)
	}
	if want := []string{"first", "last"}; !reflect.DeepEqual(names(results), want) {
		t.Errorf("policies = %v, want %v", names(results), want)
	}
	var asked []interface{}
	for _, r := range f.ops("GetRepositoryPolicy") {
		asked = append(asked, r.body["repositoryName"])
	}
	// The named repository gets no request; every named one is tried once.
	if want := []interface{}{"first", "no-policy", "denied", "throttled", "last"}; !reflect.DeepEqual(asked, want) {
		t.Errorf("GetRepositoryPolicy repositories = %v, want %v", asked, want)
	}
}

func TestDescribeRepositoriesAccessDeniedNormalized(t *testing.T) {
	const msg = "User is not authorized to perform: ecr:DescribeRepositories"
	for i := range ECRCalls {
		t.Run(ECRCalls[i].Name, func(t *testing.T) {
			f := &fakeECR{
				repos:    map[string][]repoSpec{"us-east-1": {{"r"}}},
				images:   map[string][]string{"us-east-1/r": {"t"}},
				policies: map[string]string{"us-east-1/r": "{}"},
				deny:     map[string]string{"us-west-1": msg},
			}
			ctx := newEnv(t, f)
			svc := ECRCalls[i]
			out, err := svc.Invoke(ctx)
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
			// v1 aborted the whole call on the first failing region.
			idx := -1
			for j, r := range types.RegionsFor(ctx) {
				if r == "us-west-1" {
					idx = j
				}
			}
			if got := f.regionsOf("DescribeRepositories"); len(got) != idx+1 {
				t.Errorf("DescribeRepositories regions = %v, want loop to stop at us-west-1 (index %d)", got, idx)
			}

			results := svc.Process(out, err, false)
			if len(results) != 1 || results[0].Error == nil || results[0].ServiceName != "ECR" ||
				results[0].MethodName != svc.Name || results[0].ResourceName != "" {
				t.Fatalf("results = %+v", results)
			}
		})
	}
}

func TestAmbientEndpointIgnored(t *testing.T) {
	// newEnv sets AWS_ENDPOINT_URL_ECR to an unreachable host; success proves
	// the explicit endpoint won.
	f := &fakeECR{
		repos:    map[string][]repoSpec{"us-west-2": {{"r"}}},
		images:   map[string][]string{"us-west-2/r": {"t"}},
		policies: map[string]string{"us-west-2/r": "{}"},
	}
	ctx := scoped(newEnv(t, f), "us-west-2")
	for i := range ECRCalls {
		results, err := run(ctx, i)
		if err != nil || len(results) != 1 {
			t.Errorf("%s: results = %+v, err = %v", ECRCalls[i].Name, results, err)
		}
	}
}

func TestProcessIgnoresUnexpectedOutput(t *testing.T) {
	for _, svc := range ECRCalls {
		for _, out := range []interface{}{nil, "x", 3, []string{"nope"}} {
			if got := svc.Process(out, nil, false); len(got) != 0 {
				t.Errorf("%s: results = %+v", svc.Name, got)
			}
		}
	}
}
