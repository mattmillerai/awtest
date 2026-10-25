package rekognition

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

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/rekognition/aws4_request`)

type recorded struct {
	region, op string
	body       map[string]interface{}
}

// fakeRekognition speaks the Rekognition JSON-1.1 protocol. An empty name or
// ARN yields an entry without that field.
type fakeRekognition struct {
	collections []string
	processors  [][2]string // name, status
	projects    [][2]string // arn, status
	// deny maps operation -> AccessDeniedException message.
	deny map[string]string

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeRekognition) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	target := r.Header.Get("X-Amz-Target")
	op := target[strings.LastIndex(target, ".")+1:]
	raw, _ := io.ReadAll(r.Body)
	body := map[string]interface{}{}
	_ = json.Unmarshal(raw, &body)
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, op, body})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	if msg, ok := f.deny[op]; ok {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"__type": "AccessDeniedException", "message": msg})
		return
	}
	pairs := func(l [][2]string, k0, k1 string) []map[string]string {
		var list []map[string]string
		for _, p := range l {
			e := map[string]string{}
			if p[0] != "" {
				e[k0] = p[0]
			}
			if p[1] != "" {
				e[k1] = p[1]
			}
			list = append(list, e)
		}
		return list
	}
	switch op {
	case "ListCollections":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"CollectionIds": f.collections})
	case "ListStreamProcessors":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"StreamProcessors": pairs(f.processors, "Name", "Status")})
	case "DescribeProjects":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ProjectDescriptions": pairs(f.projects, "ProjectArn", "Status")})
	default:
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"__type": "UnknownOperationException", "message": op})
	}
}

func (f *fakeRekognition) ops() []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recorded(nil), f.reqs...)
}

func newEnv(t *testing.T, f *fakeRekognition) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_REKOGNITION", unreachable)

	cfg, err := awsv2.LoadConfig(context.Background(), awsv2.Options{
		Region:          "eu-central-1",
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

func run(ctx context.Context, i int) ([]types.ScanResult, error) {
	svc := RekognitionCalls[i]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

var opNames = []string{"ListCollections", "ListStreamProcessors", "DescribeProjects"}

func TestServicesAreV2Only(t *testing.T) {
	if len(RekognitionCalls) != len(opNames) {
		t.Fatalf("expected %d services, got %d", len(opNames), len(RekognitionCalls))
	}
	for i, svc := range RekognitionCalls {
		if svc.Name != "rekognition:"+opNames[i] {
			t.Errorf("service %d name = %q", i, svc.Name)
		}
		if svc.CallV2 == nil || svc.Call != nil {
			t.Errorf("%s: want CallV2 set and Call nil", svc.Name)
		}
	}
}

func TestListCollections(t *testing.T) {
	// The second collection id is empty: v1 printed "" for a nil string.
	f := &fakeRekognition{collections: []string{"faces-prod", ""}}
	results, err := run(newEnv(t, f), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results: %+v", len(results), results)
	}
	for j, name := range []string{"faces-prod", ""} {
		r := results[j]
		if r.ServiceName != "Rekognition" || r.MethodName != "rekognition:ListCollections" || r.ResourceType != "collection" ||
			r.ResourceName != name || r.Error != nil || r.Timestamp.IsZero() {
			t.Errorf("unexpected result %+v", r)
		}
		if !reflect.DeepEqual(r.Details, map[string]interface{}{}) {
			t.Errorf("details = %#v, want empty map", r.Details)
		}
	}
	got := f.ops()
	if len(got) != 1 || got[0].region != "eu-central-1" || got[0].op != "ListCollections" {
		t.Fatalf("requests = %v", got)
	}
	// v1 requested MaxResults=4096 and did not paginate.
	if mr, _ := got[0].body["MaxResults"].(float64); mr != 4096 {
		t.Errorf("MaxResults = %v, want 4096", got[0].body["MaxResults"])
	}
	if _, ok := got[0].body["NextToken"]; ok {
		t.Errorf("unexpected NextToken in %v", got[0].body)
	}
}

func TestListStreamProcessors(t *testing.T) {
	f := &fakeRekognition{processors: [][2]string{{"proc-1", "RUNNING"}, {"", ""}}}
	results, err := run(newEnv(t, f), 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ name, status string }{{"proc-1", "RUNNING"}, {"", ""}}
	if len(results) != len(want) {
		t.Fatalf("got %d results: %+v", len(results), results)
	}
	for j, w := range want {
		r := results[j]
		if r.ServiceName != "Rekognition" || r.MethodName != "rekognition:ListStreamProcessors" ||
			r.ResourceType != "stream-processor" || r.ResourceName != w.name || r.Error != nil || r.Timestamp.IsZero() {
			t.Errorf("unexpected result %+v", r)
		}
		if !reflect.DeepEqual(r.Details, map[string]interface{}{"status": w.status}) {
			t.Errorf("details = %#v, want status %q", r.Details, w.status)
		}
	}
	if got := f.ops(); len(got) != 1 || got[0].region != "eu-central-1" || got[0].op != "ListStreamProcessors" {
		t.Errorf("requests = %v", got)
	}
}

func TestDescribeProjects(t *testing.T) {
	f := &fakeRekognition{projects: [][2]string{
		{"arn:aws:rekognition:eu-central-1:123456789012:project/my-proj/1700000000", "CREATED"},
		{"noslash", "CREATING"},
		{"", ""},
	}}
	results, err := run(newEnv(t, f), 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ name, status string }{{"my-proj", "CREATED"}, {"noslash", "CREATING"}, {"", ""}}
	if len(results) != len(want) {
		t.Fatalf("got %d results: %+v", len(results), results)
	}
	for j, w := range want {
		r := results[j]
		if r.ServiceName != "Rekognition" || r.MethodName != "rekognition:DescribeProjects" ||
			r.ResourceType != "project" || r.ResourceName != w.name || r.Error != nil || r.Timestamp.IsZero() {
			t.Errorf("unexpected result %+v", r)
		}
		if !reflect.DeepEqual(r.Details, map[string]interface{}{"status": w.status}) {
			t.Errorf("details = %#v, want status %q", r.Details, w.status)
		}
	}
	if got := f.ops(); len(got) != 1 || got[0].region != "eu-central-1" || got[0].op != "DescribeProjects" {
		t.Errorf("requests = %v", got)
	}
}

func TestNothingFoundYieldsNoResults(t *testing.T) {
	for i := range RekognitionCalls {
		results, err := run(newEnv(t, &fakeRekognition{}), i)
		if err != nil || len(results) != 0 {
			t.Errorf("%s: results = %+v, err = %v", RekognitionCalls[i].Name, results, err)
		}
	}
}

func TestErrorIsNormalized(t *testing.T) {
	for i, op := range opNames {
		t.Run(op, func(t *testing.T) {
			f := &fakeRekognition{deny: map[string]string{op: "not allowed"}}
			results, err := run(newEnv(t, f), i)
			if err == nil {
				t.Fatal("expected error")
			}
			if got, want := err.Error(), "AccessDeniedException: not allowed"; got != want {
				t.Errorf("error = %q, want %q", got, want)
			}
			if len(results) != 1 || results[0].Error == nil || results[0].MethodName != "rekognition:"+op ||
				results[0].ServiceName != "Rekognition" {
				t.Errorf("results = %+v", results)
			}
		})
	}
}
