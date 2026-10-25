package transcribe

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

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/transcribe/aws4_request`)

type recorded struct{ region, op string }

// fakeTranscribe speaks the Transcribe JSON-1.1 protocol. An empty name yields
// an entry without a name field.
type fakeTranscribe struct {
	names []string
	// deny maps operation -> AccessDeniedException message.
	deny map[string]string

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeTranscribe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	switch op {
	case "ListTranscriptionJobs":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"TranscriptionJobSummaries": entries("TranscriptionJobName")})
	case "ListVocabularies":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"Vocabularies": entries("VocabularyName")})
	case "ListLanguageModels":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"Models": entries("ModelName")})
	default:
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"__type": "UnknownOperationException", "message": op})
	}
}

func (f *fakeTranscribe) ops() []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recorded(nil), f.reqs...)
}

func newEnv(t *testing.T, f *fakeTranscribe) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_TRANSCRIBE", unreachable)

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
	svc := TranscribeCalls[i]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

var ops = []struct{ op, rtype string }{
	{"ListTranscriptionJobs", "transcription-job"},
	{"ListVocabularies", "vocabulary"},
	{"ListLanguageModels", "language-model"},
}

func TestServicesAreV2Only(t *testing.T) {
	if len(TranscribeCalls) != len(ops) {
		t.Fatalf("expected %d services, got %d", len(ops), len(TranscribeCalls))
	}
	for i, svc := range TranscribeCalls {
		if svc.Name != "transcribe:"+ops[i].op {
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
			f := &fakeTranscribe{names: names}
			results, err := run(newEnv(t, f), i)
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != len(names) {
				t.Fatalf("got %d results: %+v", len(results), results)
			}
			for j, name := range names {
				r := results[j]
				if r.ServiceName != "Transcribe" || r.MethodName != "transcribe:"+c.op || r.ResourceType != c.rtype ||
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

func TestErrorIsNormalized(t *testing.T) {
	for i, c := range ops {
		t.Run(c.op, func(t *testing.T) {
			f := &fakeTranscribe{deny: map[string]string{c.op: "not allowed"}}
			results, err := run(newEnv(t, f), i)
			if err == nil {
				t.Fatal("expected error")
			}
			if got, want := err.Error(), "AccessDeniedException: not allowed"; got != want {
				t.Errorf("error = %q, want %q", got, want)
			}
			if len(results) != 1 || results[0].Error == nil || results[0].MethodName != "transcribe:"+c.op ||
				results[0].ServiceName != "Transcribe" {
				t.Errorf("results = %+v", results)
			}
		})
	}
}
