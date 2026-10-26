package codepipeline

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

const (
	wantName     = "codepipeline:ListPipelines"
	wantService  = "CodePipeline"
	wantResource = "pipeline"
	deniedCode   = "AccessDeniedException"
	deniedMsg    = "User is not authorized to perform: codepipeline:ListPipelines"
)

var credScope = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/([^/]+)/aws4_request`)

type request struct {
	method, target, region, signingName string
}

// fakeCodePipeline speaks the JSON-1.1 protocol for codepipeline:ListPipelines.
// mode selects the response: "list" (a populated and a sparse pipeline plus a
// continuation token, which the v1 code never followed), "empty" or "denied".
type fakeCodePipeline struct {
	mode string

	mu   sync.Mutex
	reqs []request
}

func (f *fakeCodePipeline) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, _ = io.Copy(io.Discard, r.Body)
	req := request{method: r.Method, target: r.Header.Get("X-Amz-Target")}
	if m := credScope.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		req.region, req.signingName = m[1], m[2]
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, req)
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	switch f.mode {
	case "denied":
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"__type":"`+deniedCode+`","message":"`+deniedMsg+`"}`)
	case "empty":
		_, _ = io.WriteString(w, `{"pipelines":[]}`)
	default:
		_, _ = io.WriteString(w, `{"pipelines":[{"name":"deploy-prod","version":3},{"version":1}],"nextToken":"more"}`)
	}
}

func (f *fakeCodePipeline) requests() []request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]request(nil), f.reqs...)
}

func newEnv(t *testing.T, mode string) (context.Context, *fakeCodePipeline) {
	t.Helper()
	f := &fakeCodePipeline{mode: mode}
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
	t.Setenv("AWS_ENDPOINT_URL_CODEPIPELINE", unreachable)

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
	return types.WithAWSConfig(context.Background(), cfg), f
}

func run(ctx context.Context) ([]types.ScanResult, error) {
	svc := CodePipelineCalls[0]
	out, err := svc.Invoke(ctx)
	return svc.Process(out, err, false), err
}

func TestServiceIsV2Only(t *testing.T) {
	if len(CodePipelineCalls) != 1 {
		t.Fatalf("expected 1 service, got %d", len(CodePipelineCalls))
	}
	svc := CodePipelineCalls[0]
	if svc.Name != wantName {
		t.Errorf("name = %q, want %q", svc.Name, wantName)
	}
	if svc.CallV2 == nil {
		t.Error("want CallV2 set")
	}
}

func TestInvokePopulatedAndSparse(t *testing.T) {
	ctx, f := newEnv(t, "list")
	results, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// A sparse pipeline (no name) maps to an empty ResourceName, as v1 printed "".
	if len(results) != 2 {
		t.Fatalf("results = %+v, want 2", results)
	}
	for i, want := range []string{"deploy-prod", ""} {
		r := results[i]
		if r.ServiceName != wantService || r.MethodName != wantName || r.ResourceType != wantResource ||
			r.ResourceName != want || r.Error != nil || r.Timestamp.IsZero() {
			t.Errorf("result %d = %+v, want name %q", i, r, want)
		}
		if !reflect.DeepEqual(r.Details, map[string]interface{}{}) {
			t.Errorf("result %d Details = %#v, want empty map", i, r.Details)
		}
	}

	// One request: the continuation token in the response is not followed.
	reqs := f.requests()
	if len(reqs) != 1 {
		t.Fatalf("requests = %+v, want exactly 1 (v1 ignored the continuation token)", reqs)
	}
	r := reqs[0]
	if r.region != "us-west-2" || r.signingName != "codepipeline" {
		t.Errorf("signed for region %q service %q, want us-west-2/codepipeline", r.region, r.signingName)
	}
	if r.method != "POST" || r.target != "CodePipeline_20150709.ListPipelines" {
		t.Errorf("request = %s target %q, want POST CodePipeline_20150709.ListPipelines", r.method, r.target)
	}
}

func TestInvokeEmpty(t *testing.T) {
	ctx, _ := newEnv(t, "empty")
	if results, err := run(ctx); err != nil || len(results) != 0 {
		t.Errorf("results = %+v, err = %v", results, err)
	}
}

func TestAccessDeniedNormalized(t *testing.T) {
	ctx, _ := newEnv(t, "denied")
	svc := CodePipelineCalls[0]
	out, err := svc.Invoke(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	if want := deniedCode + ": " + deniedMsg; err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if code := awsv2.ErrorCode(err); code != deniedCode {
		t.Errorf("ErrorCode = %q", code)
	}
	for _, noise := range []string{"status code", "request id", "operation error", "400"} {
		if strings.Contains(strings.ToLower(err.Error()), noise) {
			t.Errorf("error %q contains transport noise %q", err, noise)
		}
	}

	results := svc.Process(out, err, false)
	if len(results) != 1 || results[0].Error == nil || results[0].ServiceName != wantService ||
		results[0].MethodName != wantName || results[0].ResourceName != "" {
		t.Fatalf("results = %+v", results)
	}
}

func TestProcessTypedOutput(t *testing.T) {
	process := CodePipelineCalls[0].Process
	tests := []struct {
		name   string
		output interface{}
		want   []string
	}{
		{"populated and sparse", &codepipeline.ListPipelinesOutput{Pipelines: []cptypes.PipelineSummary{
			{Name: aws.String("deploy-prod"), Version: aws.Int32(3)},
			{Version: aws.Int32(1)},
		}}, []string{"deploy-prod", ""}},
		{"empty", &codepipeline.ListPipelinesOutput{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, r := range process(tt.output, nil, false) {
				if r.ServiceName != wantService || r.MethodName != wantName || r.ResourceType != wantResource || r.Error != nil {
					t.Errorf("unexpected result %+v", r)
				}
				got = append(got, r.ResourceName)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("names = %q, want %q", got, tt.want)
			}
		})
	}

	errIn := fmt.Errorf("AccessDeniedException: nope")
	results := process(nil, errIn, false)
	if len(results) != 1 || results[0].Error != errIn || results[0].ServiceName != wantService || results[0].MethodName != wantName {
		t.Errorf("error results = %+v", results)
	}

	for _, out := range []interface{}{nil, "x", 3, []string{"nope"}} {
		if got := process(out, nil, false); len(got) != 0 {
			t.Errorf("unexpected output %v produced %+v", out, got)
		}
	}
}
