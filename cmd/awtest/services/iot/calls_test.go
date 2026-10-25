package iot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"sync"
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

// IoT control-plane requests are signed with the "execute-api" signing name.
var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/[^/]+/aws4_request`)

type recorded struct{ region, method, path string }

// fakeIoT speaks the IoT REST-JSON protocol for ListThings, ListPolicies and
// ListCertificates. An empty name yields an entry without the name field.
type fakeIoT struct {
	names []string
	// deny maps request path -> AccessDeniedException message.
	deny map[string]string

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeIoT) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, r.Method, r.URL.Path})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if msg, ok := f.deny[r.URL.Path]; ok {
		w.Header().Set("X-Amzn-Errortype", "AccessDeniedException")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": msg})
		return
	}
	entries := func(field string) []map[string]string {
		var list []map[string]string
		for _, n := range f.names {
			e := map[string]string{}
			if n != "" {
				e[field] = n
			}
			list = append(list, e)
		}
		return list
	}
	switch r.URL.Path {
	case "/things":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"things": entries("thingName")})
	case "/policies":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"policies": entries("policyName")})
	case "/certificates":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"certificates": entries("certificateArn")})
	default:
		w.Header().Set("X-Amzn-Errortype", "ResourceNotFoundException")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": r.URL.Path})
	}
}

func (f *fakeIoT) ops() []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recorded(nil), f.reqs...)
}

func newEnv(t *testing.T, f *fakeIoT) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_IOT", unreachable)

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
	svc := IoTCalls[i]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

var ops = []struct{ op, path, rtype string }{
	{"ListThings", "/things", "thing"},
	{"ListPolicies", "/policies", "policy"},
	{"ListCertificates", "/certificates", "certificate"},
}

func TestServicesAreV2Only(t *testing.T) {
	if len(IoTCalls) != len(ops) {
		t.Fatalf("expected %d services, got %d", len(ops), len(IoTCalls))
	}
	for i, svc := range IoTCalls {
		if svc.Name != "iot:"+ops[i].op {
			t.Errorf("service %d name = %q", i, svc.Name)
		}
		if svc.CallV2 == nil || svc.Call != nil {
			t.Errorf("%s: want CallV2 set and Call nil", svc.Name)
		}
	}
}

func TestInvokeProcessDetails(t *testing.T) {
	for i, c := range ops {
		t.Run(c.op, func(t *testing.T) {
			// The second entry has no name: v1 would have panicked, v2 reports "".
			names := []string{"populated-name", ""}
			f := &fakeIoT{names: names}
			results, err := run(newEnv(t, f), i)
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != len(names) {
				t.Fatalf("got %d results: %+v", len(results), results)
			}
			for j, name := range names {
				r := results[j]
				if r.ServiceName != "IoT" || r.MethodName != "iot:"+c.op || r.ResourceType != c.rtype ||
					r.ResourceName != name || r.Error != nil || r.Timestamp.IsZero() {
					t.Errorf("unexpected result %+v", r)
				}
				if !reflect.DeepEqual(r.Details, map[string]interface{}{}) {
					t.Errorf("details = %#v, want empty map", r.Details)
				}
			}
			// Single non-looping call signed for the configured region.
			if got := f.ops(); !reflect.DeepEqual(got, []recorded{{"eu-central-1", "GET", c.path}}) {
				t.Errorf("requests = %v", got)
			}
		})
	}
}

func TestErrorIsNormalized(t *testing.T) {
	for i, c := range ops {
		t.Run(c.op, func(t *testing.T) {
			f := &fakeIoT{deny: map[string]string{c.path: "not allowed"}}
			results, err := run(newEnv(t, f), i)
			if err == nil {
				t.Fatal("expected error")
			}
			if got, want := err.Error(), "AccessDeniedException: not allowed"; got != want {
				t.Errorf("error = %q, want %q", got, want)
			}
			if len(results) != 1 || results[0].Error == nil || results[0].MethodName != "iot:"+c.op ||
				results[0].ServiceName != "IoT" {
				t.Errorf("results = %+v", results)
			}
		})
	}
}
