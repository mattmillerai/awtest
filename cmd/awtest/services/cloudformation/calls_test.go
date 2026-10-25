package cloudformation

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
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

const (
	wantName     = "cloudformation:ListStacks"
	wantService  = "CloudFormation"
	wantResource = "stack"
	deniedCode   = "AccessDenied"
	deniedMsg    = "User is not authorized to perform: cloudformation:ListStacks"
)

var credScope = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/([^/]+)/aws4_request`)

type request struct {
	method, path, body, region, signingName string
}

// fakeCloudFormation speaks the query (XML) protocol for
// cloudformation:ListStacks. mode selects the response: "list" (a populated
// and a sparse stack plus a continuation token, which the v1 code never
// followed), "empty" or "denied".
type fakeCloudFormation struct {
	mode string

	mu   sync.Mutex
	reqs []request
}

func (f *fakeCloudFormation) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	req := request{method: r.Method, path: r.URL.Path, body: string(raw)}
	if m := credScope.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		req.region, req.signingName = m[1], m[2]
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, req)
	f.mu.Unlock()

	w.Header().Set("Content-Type", "text/xml")
	const head = `<ListStacksResponse xmlns="http://cloudformation.amazonaws.com/doc/2010-05-15/"><ListStacksResult>`
	const tail = `</ListStacksResult><ResponseMetadata><RequestId>req-1</RequestId></ResponseMetadata></ListStacksResponse>`
	switch f.mode {
	case "denied":
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `<ErrorResponse xmlns="http://cloudformation.amazonaws.com/doc/2010-05-15/"><Error><Type>Sender</Type><Code>`+deniedCode+`</Code><Message>`+deniedMsg+`</Message></Error><RequestId>req-1</RequestId></ErrorResponse>`)
	case "empty":
		_, _ = io.WriteString(w, head+`<StackSummaries/>`+tail)
	default:
		_, _ = io.WriteString(w, head+`<StackSummaries>`+
			`<member><StackName>network-stack</StackName><StackId>id-1</StackId><StackStatus>CREATE_COMPLETE</StackStatus><CreationTime>2020-01-01T00:00:00Z</CreationTime></member>`+
			`<member><StackStatus>CREATE_COMPLETE</StackStatus><CreationTime>2020-01-01T00:00:00Z</CreationTime></member>`+
			`</StackSummaries><NextToken>more</NextToken>`+tail)
	}
}

func (f *fakeCloudFormation) requests() []request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]request(nil), f.reqs...)
}

func newEnv(t *testing.T, mode string) (context.Context, *fakeCloudFormation) {
	t.Helper()
	f := &fakeCloudFormation{mode: mode}
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
	t.Setenv("AWS_ENDPOINT_URL_CLOUDFORMATION", unreachable)

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
	svc := CloudFormationCalls[0]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

func TestServiceIsV2Only(t *testing.T) {
	if len(CloudFormationCalls) != 1 {
		t.Fatalf("expected 1 service, got %d", len(CloudFormationCalls))
	}
	svc := CloudFormationCalls[0]
	if svc.Name != wantName {
		t.Errorf("name = %q, want %q", svc.Name, wantName)
	}
	if svc.CallV2 == nil || svc.Call != nil {
		t.Errorf("want CallV2 set and Call nil, got CallV2=%v Call=%v", svc.CallV2 != nil, svc.Call != nil)
	}
}

func TestInvokePopulatedAndSparse(t *testing.T) {
	ctx, f := newEnv(t, "list")
	results, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// A sparse stack (no name) maps to an empty ResourceName, as v1 printed "".
	if len(results) != 2 {
		t.Fatalf("results = %+v, want 2", results)
	}
	for i, want := range []string{"network-stack", ""} {
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
	if r.region != "us-west-2" || r.signingName != "cloudformation" {
		t.Errorf("signed for region %q service %q, want us-west-2/cloudformation", r.region, r.signingName)
	}
	if r.method != "POST" || r.path != "/" || !strings.Contains(r.body, "Action=ListStacks") {
		t.Errorf("request = %s %s %q, want POST / with Action=ListStacks", r.method, r.path, r.body)
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
	svc := CloudFormationCalls[0]
	out, err := svc.Invoke(ctx, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if want := deniedCode + ": " + deniedMsg; err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if code := awsv2.ErrorCode(err); code != deniedCode {
		t.Errorf("ErrorCode = %q", code)
	}
	for _, noise := range []string{"status code", "request id", "operation error", "403"} {
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
	process := CloudFormationCalls[0].Process
	tests := []struct {
		name   string
		output interface{}
		want   []string
	}{
		{"populated and sparse", &cloudformation.ListStacksOutput{StackSummaries: []cftypes.StackSummary{
			{StackName: aws.String("network-stack"), StackId: aws.String("id-1")},
			{StackId: aws.String("id-2")},
		}}, []string{"network-stack", ""}},
		{"empty", &cloudformation.ListStacksOutput{}, nil},
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

	errIn := fmt.Errorf("AccessDenied: nope")
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
