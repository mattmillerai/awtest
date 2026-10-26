package ivs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ivs"
	ivstypes "github.com/aws/aws-sdk-go-v2/service/ivs/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

// fakeIVS speaks the IVS REST-JSON protocol and records "<method> <path>".
type fakeIVS struct {
	status int
	header map[string]string
	body   string

	mu    sync.Mutex
	paths []string
}

func (f *fakeIVS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func newCtx(t *testing.T, f *fakeIVS) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_IVS", unreachable)

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

func run(t *testing.T, ctx context.Context, i int) ([]types.ScanResult, error) {
	t.Helper()
	utils.Quiet = true
	t.Cleanup(func() { utils.Quiet = false })
	svc := IvsCalls[i]
	out, err := svc.Invoke(ctx)
	return svc.Process(out, err, false), err
}

type row struct{ typ, name string }

func rows(t *testing.T, results []types.ScanResult, method string) []row {
	t.Helper()
	var got []row
	for _, r := range results {
		if r.ServiceName != "IVS" || r.MethodName != method {
			t.Errorf("unexpected result identity: %+v", r)
		}
		if !reflect.DeepEqual(r.Details, map[string]interface{}{}) {
			t.Errorf("details = %#v, want empty map", r.Details)
		}
		got = append(got, row{r.ResourceType, r.ResourceName})
	}
	return got
}

func TestServicesAreV2Only(t *testing.T) {
	want := []string{"ivs:ListChannels", "ivs:ListStreams", "ivs:ListStreamKeys"}
	if len(IvsCalls) != len(want) {
		t.Fatalf("expected %d services, got %d", len(want), len(IvsCalls))
	}
	for i, s := range IvsCalls {
		if s.Name != want[i] {
			t.Errorf("service %d name = %q, want %q", i, s.Name, want[i])
		}
		if s.CallV2 == nil {
			t.Errorf("%s must implement CallV2 only", s.Name)
		}
	}
}

func TestListChannelsFunctional(t *testing.T) {
	f := &fakeIVS{status: 200, body: `{"channels":[` +
		`{"arn":"arn:aws:ivs:us-west-2:123456789012:channel/abc","name":"live"},` +
		`{"name":"no-arn"}],"nextToken":"more"}`}
	results, err := run(t, newCtx(t, f), 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []row{{"channel", "arn:aws:ivs:us-west-2:123456789012:channel/abc"}, {"channel", ""}}
	if got := rows(t, results, "ivs:ListChannels"); !reflect.DeepEqual(got, want) {
		t.Errorf("results = %v, want %v", got, want)
	}
	// nextToken is not followed (v1 ignored it).
	if !reflect.DeepEqual(f.paths, []string{"POST /ListChannels"}) {
		t.Errorf("requests = %v", f.paths)
	}
}

func TestListStreamsFunctional(t *testing.T) {
	f := &fakeIVS{status: 200, body: `{"streams":[` +
		`{"streamId":"st-1","channelArn":"arn:aws:ivs:us-west-2:123456789012:channel/abc"},` +
		`{"channelArn":"x"}],"nextToken":"more"}`}
	results, err := run(t, newCtx(t, f), 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []row{{"stream", "st-1"}, {"stream", ""}}
	if got := rows(t, results, "ivs:ListStreams"); !reflect.DeepEqual(got, want) {
		t.Errorf("results = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(f.paths, []string{"POST /ListStreams"}) {
		t.Errorf("requests = %v", f.paths)
	}
}

// ListStreamKeys requires a channel ARN; awtest sends none, so the SDK rejects
// the call client-side before any request (as v1 did).
func TestListStreamKeysClientSideValidation(t *testing.T) {
	f := &fakeIVS{status: 200, body: `{}`}
	results, err := run(t, newCtx(t, f), 2)
	if err == nil {
		t.Fatal("expected a validation error")
	}
	const want = "1 validation error(s) found.\n- missing required field, ListStreamKeysInput.ChannelArn.\n"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if len(results) != 1 || results[0].Error == nil || results[0].MethodName != "ivs:ListStreamKeys" {
		t.Fatalf("results = %+v", results)
	}
	if len(f.paths) != 0 {
		t.Errorf("unexpected requests: %v", f.paths)
	}
}

// Process of ListStreamKeys, driven directly because the call cannot succeed
// without a channel ARN: populated and sparse keys.
func TestListStreamKeysProcess(t *testing.T) {
	arn := "arn:aws:ivs:us-west-2:123456789012:stream-key/k1"
	out := &ivs.ListStreamKeysOutput{StreamKeys: []ivstypes.StreamKeySummary{{Arn: &arn}, {}}}
	utils.Quiet = true
	t.Cleanup(func() { utils.Quiet = false })
	results := IvsCalls[2].Process(out, nil, false)
	want := []row{{"stream-key", arn}, {"stream-key", ""}}
	if got := rows(t, results, "ivs:ListStreamKeys"); !reflect.DeepEqual(got, want) {
		t.Errorf("results = %v, want %v", got, want)
	}
}

// Real IVS puts the AccessDeniedException text in "exceptionMessage".
func TestAccessDeniedExceptionMessage(t *testing.T) {
	f := &fakeIVS{status: 403, header: map[string]string{"X-Amzn-Errortype": "AccessDeniedException"},
		body: `{"exceptionMessage":"User is not authorized to perform ivs:ListChannels"}`}
	_, err := IvsCalls[0].Invoke(newCtx(t, f))
	if err == nil {
		t.Fatal("expected error")
	}
	const want = "AccessDeniedException: User is not authorized to perform ivs:ListChannels"
	if got := awsv2.NormalizeError(err); got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}

func TestErrorsAreNormalized(t *testing.T) {
	for i, name := range []string{"ivs:ListChannels", "ivs:ListStreams"} {
		f := &fakeIVS{
			status: 400,
			header: map[string]string{"X-Amzn-Errortype": "ThrottledByTest"},
			body:   `{"message":"not allowed"}`,
		}
		results, err := run(t, newCtx(t, f), i)
		if err == nil {
			t.Fatalf("%s: expected an error", name)
		}
		if got, want := err.Error(), "ThrottledByTest: not allowed"; got != want {
			t.Errorf("%s: error = %q, want %q", name, got, want)
		}
		if len(results) != 1 || results[0].Error == nil || results[0].MethodName != name {
			t.Fatalf("%s: results = %+v", name, results)
		}
		if got := results[0].Error.Error(); got != "ThrottledByTest: not allowed" {
			t.Errorf("%s: result error = %q", name, got)
		}
	}
}
