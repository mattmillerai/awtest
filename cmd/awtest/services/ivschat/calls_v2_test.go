package ivschat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

// fakeChat speaks the IVS Chat REST-JSON protocol and records "<method> <path>".
type fakeChat struct {
	status int
	header map[string]string
	body   string

	mu    sync.Mutex
	paths []string
}

func (f *fakeChat) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.paths = append(f.paths, r.Method+" "+r.URL.Path)
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	for k, v := range f.header {
		w.Header().Set(k, v)
	}
	w.WriteHeader(f.status)
	_, _ = w.Write([]byte(f.body))
}

func newCtx(t *testing.T, f *fakeChat) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_IVSCHAT", unreachable)

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

func run(t *testing.T, ctx context.Context) ([]types.ScanResult, error) {
	t.Helper()
	utils.Quiet = true
	t.Cleanup(func() { utils.Quiet = false })
	svc := IvsChatCalls[0]
	out, err := svc.Invoke(ctx)
	return svc.Process(out, err, false), err
}

func TestServicesAreV2Only(t *testing.T) {
	if len(IvsChatCalls) != 1 || IvsChatCalls[0].Name != "ivsChat:ListRooms" {
		t.Fatalf("unexpected services: %+v", IvsChatCalls)
	}
	if IvsChatCalls[0].CallV2 == nil {
		t.Error("ivsChat:ListRooms must implement CallV2 only")
	}
}

func TestListRoomsFunctional(t *testing.T) {
	f := &fakeChat{status: 200, body: `{"rooms":[` +
		`{"arn":"arn:aws:ivschat:us-west-2:123456789012:room/r1","name":"lobby"},` +
		`{"arn":"arn:aws:ivschat:us-west-2:123456789012:room/r2"}],"nextToken":"more"}`}
	results, err := run(t, newCtx(t, f))
	if err != nil {
		t.Fatal(err)
	}
	type row struct{ typ, name string }
	var got []row
	for _, r := range results {
		if r.ServiceName != "IVSChat" || r.MethodName != "ivsChat:ListRooms" {
			t.Errorf("unexpected result identity: %+v", r)
		}
		if !reflect.DeepEqual(r.Details, map[string]interface{}{}) {
			t.Errorf("details = %#v, want empty map", r.Details)
		}
		got = append(got, row{r.ResourceType, r.ResourceName})
	}
	// The second room is sparse (no name) and prints as an empty name.
	want := []row{{"room", "lobby"}, {"room", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("results = %v, want %v", got, want)
	}
	// nextToken is not followed (v1 ignored it).
	if !reflect.DeepEqual(f.paths, []string{"POST /ListRooms"}) {
		t.Errorf("requests = %v", f.paths)
	}
}

func TestErrorsAreNormalized(t *testing.T) {
	f := &fakeChat{
		status: 400,
		header: map[string]string{"X-Amzn-Errortype": "ThrottledByTest"},
		body:   `{"message":"not allowed"}`,
	}
	results, err := run(t, newCtx(t, f))
	if err == nil {
		t.Fatal("expected an error")
	}
	if got, want := err.Error(), "ThrottledByTest: not allowed"; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
	if len(results) != 1 || results[0].Error == nil || results[0].MethodName != "ivsChat:ListRooms" || results[0].ServiceName != "IVSChat" {
		t.Fatalf("results = %+v", results)
	}
	if got := results[0].Error.Error(); got != "ThrottledByTest: not allowed" {
		t.Errorf("result error = %q", got)
	}
}
