package sagemaker

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

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/[^/]+/aws4_request`)

const targetPrefix = "SageMaker."

type recorded struct {
	region string
	op     string
	body   map[string]interface{}
}

// fakeSvc speaks the JSON-1.1 protocol. handle receives the SigV4
// credential-scope region, the operation and the decoded request body and
// returns the HTTP status and JSON response.
type fakeSvc struct {
	handle func(region, op string, body map[string]interface{}) (int, interface{})

	mu   sync.Mutex
	reqs []recorded
}

func apiError(code, msg string) (int, interface{}) {
	return http.StatusBadRequest, map[string]string{"__type": code, "message": msg}
}

func (f *fakeSvc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	raw, _ := io.ReadAll(r.Body)
	body := map[string]interface{}{}
	_ = json.Unmarshal(raw, &body)
	target := r.Header.Get("X-Amz-Target")
	op := strings.TrimPrefix(target, targetPrefix)
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, op, body})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	var status int
	var out interface{}
	if !strings.HasPrefix(target, targetPrefix) || r.Method != http.MethodPost {
		status, out = apiError("UnknownOperationException", "bad request "+r.Method+" "+target)
	} else {
		status, out = f.handle(region, op, body)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(out)
}

func (f *fakeSvc) ops(op string) []recorded {
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

func (f *fakeSvc) regionsOf(op string) []string {
	var out []string
	for _, r := range f.ops(op) {
		out = append(out, r.region)
	}
	return out
}

func newEnv(t *testing.T, f *fakeSvc, regions ...string) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_SAGEMAKER", unreachable)

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
	ctx := types.WithAWSConfig(context.Background(), cfg)
	if len(regions) > 0 {
		ctx, _ = types.WithRegionScope(ctx, regions)
	}
	return ctx
}

func run(ctx context.Context, i int) ([]types.ScanResult, error) {
	svc := SageMakerCalls[i]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

func detailsOf(results []types.ScanResult) []map[string]interface{} {
	var out []map[string]interface{}
	for _, r := range results {
		out = append(out, r.Details)
	}
	return out
}

// pagedHandler serves a two-page list for op: full on the first page (with
// NextToken "p2") and sparse on the second.
func pagedHandler(op, key string, full, sparse map[string]interface{}) func(region, o string, body map[string]interface{}) (int, interface{}) {
	return func(region, o string, body map[string]interface{}) (int, interface{}) {
		if o != op {
			return apiError("UnknownOperationException", o)
		}
		if body["MaxResults"] != float64(100) {
			return apiError("ValidationException", "unexpected MaxResults")
		}
		if body["NextToken"] == "p2" {
			return http.StatusOK, map[string]interface{}{key: []map[string]interface{}{sparse}}
		}
		return http.StatusOK, map[string]interface{}{key: []map[string]interface{}{full}, "NextToken": "p2"}
	}
}

func TestListNotebookInstancesInvoke(t *testing.T) {
	f := &fakeSvc{handle: pagedHandler("ListNotebookInstances", "NotebookInstances",
		map[string]interface{}{
			"NotebookInstanceName": "nb", "NotebookInstanceArn": "arn:nb", "NotebookInstanceStatus": "InService",
			"InstanceType": "ml.t3.medium", "Url": "nb.notebook.us-west-2.sagemaker.aws",
			"DefaultCodeRepository": "repo", "LastModifiedTime": 1700000000,
		},
		map[string]interface{}{"NotebookInstanceName": "bare-nb"},
	)}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"Name": "nb", "Arn": "arn:nb", "Status": "InService", "InstanceType": "ml.t3.medium",
			"URL": "nb.notebook.us-west-2.sagemaker.aws", "DefaultCodeRepository": "repo",
			"LastModifiedTime": "2023-11-14T22:13:20Z", "Region": "us-west-2",
		},
		{
			"Name": "bare-nb", "Arn": "", "Status": "", "InstanceType": "", "URL": "",
			"DefaultCodeRepository": "", "LastModifiedTime": "", "Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ServiceName != "SageMaker" || results[0].MethodName != "sagemaker:ListNotebookInstances" || results[0].ResourceType != "notebook-instance" || results[0].ResourceName != "nb" {
		t.Errorf("result 0 = %+v", results[0])
	}
	if got := len(f.ops("ListNotebookInstances")); got != 2 {
		t.Errorf("ListNotebookInstances calls = %d, want 2 (NextToken must be followed)", got)
	}
}

func TestListEndpointsInvoke(t *testing.T) {
	f := &fakeSvc{handle: pagedHandler("ListEndpoints", "Endpoints",
		map[string]interface{}{
			"EndpointName": "ep", "EndpointArn": "arn:ep", "EndpointStatus": "InService",
			"CreationTime": 1700000000, "LastModifiedTime": 1700003600,
		},
		map[string]interface{}{"EndpointName": "bare-ep"},
	)}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"Name": "ep", "Arn": "arn:ep", "Status": "InService", "CreationTime": "2023-11-14T22:13:20Z",
			"LastModifiedTime": "2023-11-14T23:13:20Z", "Region": "us-west-2",
		},
		{
			"Name": "bare-ep", "Arn": "", "Status": "", "CreationTime": "", "LastModifiedTime": "",
			"Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ResourceType != "endpoint" || results[0].ResourceName != "ep" {
		t.Errorf("result 0 = %+v", results[0])
	}
	if got := len(f.ops("ListEndpoints")); got != 2 {
		t.Errorf("ListEndpoints calls = %d, want 2", got)
	}
}

func TestListModelsInvoke(t *testing.T) {
	f := &fakeSvc{handle: pagedHandler("ListModels", "Models",
		map[string]interface{}{"ModelName": "m", "ModelArn": "arn:m", "CreationTime": 1700000000},
		map[string]interface{}{"ModelName": "bare-m"},
	)}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{"Name": "m", "Arn": "arn:m", "CreationTime": "2023-11-14T22:13:20Z", "Region": "us-west-2"},
		{"Name": "bare-m", "Arn": "", "CreationTime": "", "Region": "us-west-2"},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ResourceType != "model" || results[0].ResourceName != "m" {
		t.Errorf("result 0 = %+v", results[0])
	}
	if got := len(f.ops("ListModels")); got != 2 {
		t.Errorf("ListModels calls = %d, want 2", got)
	}
}

func TestListTrainingJobsInvoke(t *testing.T) {
	f := &fakeSvc{handle: pagedHandler("ListTrainingJobs", "TrainingJobSummaries",
		map[string]interface{}{
			"TrainingJobName": "tj", "TrainingJobArn": "arn:tj", "TrainingJobStatus": "Completed",
			"CreationTime": 1700000000, "LastModifiedTime": 1700003600,
		},
		map[string]interface{}{"TrainingJobName": "bare-tj"},
	)}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"Name": "tj", "Arn": "arn:tj", "Status": "Completed", "CreationTime": "2023-11-14T22:13:20Z",
			"LastModifiedTime": "2023-11-14T23:13:20Z", "Region": "us-west-2",
		},
		{
			"Name": "bare-tj", "Arn": "", "Status": "", "CreationTime": "", "LastModifiedTime": "",
			"Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ResourceType != "training-job" || results[0].ResourceName != "tj" {
		t.Errorf("result 0 = %+v", results[0])
	}
	if got := len(f.ops("ListTrainingJobs")); got != 2 {
		t.Errorf("ListTrainingJobs calls = %d, want 2", got)
	}
}

func TestInvokeErrorText(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		return apiError("AccessDeniedException", "not authorized for "+op)
	}}
	wantOps := []string{"ListNotebookInstances", "ListEndpoints", "ListModels", "ListTrainingJobs"}
	for i := range SageMakerCalls {
		ctx := newEnv(t, f, "us-west-2")
		results, err := run(ctx, i)
		if err == nil {
			t.Fatalf("%s: expected an error", SageMakerCalls[i].Name)
		}
		want := "AccessDeniedException: not authorized for " + wantOps[i]
		if err.Error() != want {
			t.Errorf("%s: error = %q, want %q", SageMakerCalls[i].Name, err.Error(), want)
		}
		if len(results) != 1 || results[0].Error == nil || results[0].Error.Error() != want {
			t.Errorf("%s: Process error result = %+v", SageMakerCalls[i].Name, results)
		}
		if results[0].MethodName != SageMakerCalls[i].Name {
			t.Errorf("MethodName = %q", results[0].MethodName)
		}
	}
}

// A failing region is skipped; the call only fails when nothing was found and
// at least one region failed.
func TestFailingRegionIsSkipped(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		if region == "eu-west-1" {
			return apiError("AccessDeniedException", "denied in "+region)
		}
		return http.StatusOK, map[string]interface{}{"Models": []map[string]string{{"ModelName": "m-" + region}}}
	}}
	ctx := newEnv(t, f, "us-east-1", "eu-west-1", "ap-south-1")
	results, err := run(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range results {
		names = append(names, r.ResourceName)
	}
	if want := []string{"m-us-east-1", "m-ap-south-1"}; !reflect.DeepEqual(names, want) {
		t.Errorf("models = %v, want %v", names, want)
	}
}

func TestRegionScopeVisitsEachRegion(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		switch op {
		case "ListNotebookInstances":
			return http.StatusOK, map[string]interface{}{"NotebookInstances": []map[string]string{{"NotebookInstanceName": "nb-" + region}}}
		case "ListEndpoints":
			return http.StatusOK, map[string]interface{}{"Endpoints": []map[string]string{{"EndpointName": "ep-" + region}}}
		case "ListModels":
			return http.StatusOK, map[string]interface{}{"Models": []map[string]string{{"ModelName": "m-" + region}}}
		case "ListTrainingJobs":
			return http.StatusOK, map[string]interface{}{"TrainingJobSummaries": []map[string]string{{"TrainingJobName": "tj-" + region}}}
		}
		return apiError("UnknownOperationException", op)
	}}
	scope := []string{"us-east-1", "eu-west-1", "ap-south-1"}
	for i, op := range []string{"ListNotebookInstances", "ListEndpoints", "ListModels", "ListTrainingJobs"} {
		ctx := newEnv(t, f, scope...)
		f.mu.Lock()
		f.reqs = nil
		f.mu.Unlock()
		results, err := run(ctx, i)
		if err != nil {
			t.Fatal(err)
		}
		if got := f.regionsOf(op); !reflect.DeepEqual(got, scope) {
			t.Errorf("%s SigV4 regions = %v, want %v", op, got, scope)
		}
		var regions []string
		for _, d := range detailsOf(results) {
			regions = append(regions, d["Region"].(string))
		}
		if !reflect.DeepEqual(regions, scope) {
			t.Errorf("%s result regions = %v, want %v", SageMakerCalls[i].Name, regions, scope)
		}
	}
}
