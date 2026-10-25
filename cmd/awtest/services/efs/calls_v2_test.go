package efs

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

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/elasticfilesystem/aws4_request`)

type recorded struct {
	region string
	method string
	path   string
	query  string
}

// fakeEFS speaks the EFS REST-JSON protocol for DescribeFileSystems
// (GET /2015-02-01/file-systems). Data is keyed by the SigV4 region.
type fakeEFS struct {
	systems map[string][]map[string]interface{}
	// deny maps region -> AccessDenied message.
	deny map[string]string

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeEFS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, r.Method, r.URL.Path, r.URL.RawQuery})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet || r.URL.Path != "/2015-02-01/file-systems" {
		w.Header().Set("X-Amzn-Errortype", "BadRequest")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": r.Method + " " + r.URL.Path})
		return
	}
	if msg, ok := f.deny[region]; ok {
		w.Header().Set("X-Amzn-Errortype", "AccessDenied")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": msg})
		return
	}
	out := map[string]interface{}{"FileSystems": f.systems[region]}
	if len(f.systems[region]) > 0 {
		// v1 ignored the marker; so must the migrated code.
		out["NextMarker"] = "more"
	}
	_ = json.NewEncoder(w).Encode(out)
}

func (f *fakeEFS) regions() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.reqs {
		out = append(out, r.region)
	}
	return out
}

func newEnv(t *testing.T, f *fakeEFS) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_EFS", unreachable)

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
	if len(EfsCalls) != 1 || EfsCalls[0].Name != "efs:DescribeFileSystems" {
		t.Fatalf("unexpected services: %+v", EfsCalls)
	}
	if EfsCalls[0].CallV2 == nil || EfsCalls[0].Call != nil {
		t.Error("want CallV2 set and Call nil")
	}
}

func TestDescribeFileSystemsThroughInvoke(t *testing.T) {
	f := &fakeEFS{systems: map[string][]map[string]interface{}{
		"us-east-1": {{
			"FileSystemId":         "fs-full",
			"Name":                 "data",
			"LifeCycleState":       "available",
			"SizeInBytes":          map[string]interface{}{"Value": 1024000},
			"NumberOfMountTargets": 2,
			"Encrypted":            true,
		}},
		// Sparse: the API omitted everything but the id.
		"us-west-1": {{"FileSystemId": "fs-sparse"}},
	}}
	ctx := newEnv(t, f)
	regions := types.RegionsFor(ctx)

	svc := EfsCalls[0]
	out, err := svc.Invoke(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	results := svc.Process(out, err, false)

	if got := f.regions(); !reflect.DeepEqual(got, regions) {
		t.Errorf("regions = %v, want %v (in order, one request each, marker ignored)", got, regions)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2: %+v", len(results), results)
	}
	for _, r := range results {
		if r.ServiceName != "EFS" || r.MethodName != "efs:DescribeFileSystems" || r.ResourceType != "file-system" ||
			r.Error != nil || r.Timestamp.IsZero() {
			t.Errorf("unexpected result %+v", r)
		}
	}
	wantFull := map[string]interface{}{
		"Name":                 "data",
		"LifeCycleState":       "available",
		"SizeInBytes":          int64(1024000),
		"NumberOfMountTargets": int64(2),
		"Encrypted":            true,
	}
	if results[0].ResourceName != "fs-full" || !reflect.DeepEqual(results[0].Details, wantFull) {
		t.Errorf("full = %q %#v, want %#v", results[0].ResourceName, results[0].Details, wantFull)
	}
	wantSparse := map[string]interface{}{
		"Name":                 "",
		"LifeCycleState":       "",
		"SizeInBytes":          int64(0),
		"NumberOfMountTargets": int64(0),
		"Encrypted":            false,
	}
	if results[1].ResourceName != "fs-sparse" || !reflect.DeepEqual(results[1].Details, wantSparse) {
		t.Errorf("sparse = %q %#v, want %#v", results[1].ResourceName, results[1].Details, wantSparse)
	}
}

func TestDescribeFileSystemsAccessDeniedNormalized(t *testing.T) {
	const msg = "User is not authorized to perform: elasticfilesystem:DescribeFileSystems"
	f := &fakeEFS{deny: map[string]string{"us-east-2": msg}}
	ctx := newEnv(t, f)
	svc := EfsCalls[0]
	_, err := svc.Invoke(ctx, nil)
	if err == nil {
		t.Fatal("expected error")
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
	// v1 aborted the whole call at the first failing region.
	if got, want := f.regions(), []string{"us-east-1", "us-east-2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("regions visited = %v, want %v", got, want)
	}
	results := svc.Process(nil, err, false)
	if len(results) != 1 || results[0].Error == nil || results[0].ServiceName != "EFS" {
		t.Errorf("error result = %+v", results)
	}
}
