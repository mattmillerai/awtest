package glue

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

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/glue/aws4_request`)

type recorded struct{ region, op string }

// fakeGlue speaks the Glue JSON-1.1 protocol for ListWorkflows and ListJobs.
type fakeGlue struct {
	workflows []string
	jobs      []string
	// deny maps operation -> AccessDeniedException message.
	deny map[string]string

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeGlue) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	target := r.Header.Get("X-Amz-Target")
	op := target[strings.LastIndex(target, ".")+1:]
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, op})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	if msg, ok := f.deny[op]; ok {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"__type": "AccessDeniedException", "message": msg})
		return
	}
	switch op {
	case "ListWorkflows":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"Workflows": f.workflows})
	case "ListJobs":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"JobNames": f.jobs})
	default:
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"__type": "UnknownOperationException", "message": op})
	}
}

func (f *fakeGlue) ops() []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recorded(nil), f.reqs...)
}

func newEnv(t *testing.T, f *fakeGlue) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_GLUE", unreachable)

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
	svc := GlueCalls[i]
	out, err := svc.Invoke(ctx)
	return svc.Process(out, err, false), err
}

func TestServicesAreV2Only(t *testing.T) {
	want := []string{"glue:ListWorkflows", "glue:ListJobs"}
	if len(GlueCalls) != len(want) {
		t.Fatalf("expected %d services, got %d", len(want), len(GlueCalls))
	}
	for i, svc := range GlueCalls {
		if svc.Name != want[i] {
			t.Errorf("service %d name = %q, want %q", i, svc.Name, want[i])
		}
		if svc.CallV2 == nil {
			t.Errorf("%s: want CallV2 set and Call nil", svc.Name)
		}
	}
}

func TestInvokeProcessDetails(t *testing.T) {
	cases := []struct {
		idx          int
		op, rtype    string
		workflowsJob []string
	}{
		{0, "ListWorkflows", "workflow", []string{"wf-one", ""}},
		{1, "ListJobs", "job", []string{"job-one", ""}},
	}
	for _, c := range cases {
		t.Run(c.op, func(t *testing.T) {
			f := &fakeGlue{workflows: c.workflowsJob, jobs: c.workflowsJob}
			results, err := run(newEnv(t, f), c.idx)
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != len(c.workflowsJob) {
				t.Fatalf("got %d results: %+v", len(results), results)
			}
			for j, name := range c.workflowsJob {
				r := results[j]
				if r.ServiceName != "Glue" || r.MethodName != "glue:"+c.op || r.ResourceType != c.rtype ||
					r.ResourceName != name || r.Error != nil || r.Timestamp.IsZero() {
					t.Errorf("unexpected result %+v", r)
				}
				if !reflect.DeepEqual(r.Details, map[string]interface{}{}) {
					t.Errorf("details = %#v, want empty map", r.Details)
				}
			}
			// Single non-looping call signed for the configured region.
			if got := f.ops(); !reflect.DeepEqual(got, []recorded{{"eu-central-1", c.op}}) {
				t.Errorf("requests = %v", got)
			}
		})
	}
}

func TestNothingFoundYieldsNoResults(t *testing.T) {
	for i := range GlueCalls {
		results, err := run(newEnv(t, &fakeGlue{}), i)
		if err != nil || len(results) != 0 {
			t.Errorf("%s: results = %+v, err = %v", GlueCalls[i].Name, results, err)
		}
	}
}

func TestErrorIsNormalized(t *testing.T) {
	for i, op := range []string{"ListWorkflows", "ListJobs"} {
		t.Run(op, func(t *testing.T) {
			f := &fakeGlue{deny: map[string]string{op: "not allowed"}}
			results, err := run(newEnv(t, f), i)
			if err == nil {
				t.Fatal("expected error")
			}
			if got, want := err.Error(), "AccessDeniedException: not allowed"; got != want {
				t.Errorf("error = %q, want %q", got, want)
			}
			if len(results) != 1 || results[0].Error == nil || results[0].MethodName != "glue:"+op ||
				results[0].ServiceName != "Glue" {
				t.Errorf("results = %+v", results)
			}
		})
	}
}
