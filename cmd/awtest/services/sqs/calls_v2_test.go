package sqs

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
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var credScope = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/([^/]+)/aws4_request`)

type recorded struct {
	region  string
	service string // SigV4 signing name
	op      string
	body    map[string]interface{}
}

// fakeSQS speaks the SQS JSON-1.0 protocol (and the EventBridge JSON-1.1
// DescribeRule used by the follow-up) on one endpoint. Queues are keyed by the
// SigV4 credential-scope region.
type fakeSQS struct {
	queues map[string][]string // region -> queue URLs
	// attrs maps a queue URL to its attributes; a queue without an entry
	// answers GetQueueAttributes with no attributes.
	attrs map[string]map[string]string
	// denyOps maps an operation name to the error code it fails with.
	denyOps map[string]string

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeSQS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var region, service string
	if m := credScope.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region, service = m[1], m[2]
	}
	raw, _ := io.ReadAll(r.Body)
	body := map[string]interface{}{}
	_ = json.Unmarshal(raw, &body)
	target := r.Header.Get("X-Amz-Target")
	op := target[strings.LastIndex(target, ".")+1:]
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, service, op, body})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	if code, ok := f.denyOps[op]; ok {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"__type": code, "message": "denied " + op})
		return
	}
	url, _ := body["QueueUrl"].(string)
	switch op {
	case "ListQueues":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"QueueUrls": f.queues[region]})
	case "ReceiveMessage":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"Messages": []map[string]string{{"MessageId": "m1", "Body": "body of " + url}}})
	case "GetQueueAttributes":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"Attributes": f.attrs[url]})
	case "ListDeadLetterSourceQueues":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"queueUrls": []string{"https://sqs.us-east-1.amazonaws.com/123456789012/src"}})
	case "DescribeRule":
		_ = json.NewEncoder(w).Encode(map[string]string{"Name": body["Name"].(string)})
	default:
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"__type": "UnknownOperation", "message": op})
	}
}

func (f *fakeSQS) ops(op string) []recorded {
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

func newCtx(t *testing.T, f *fakeSQS) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_SQS", unreachable)
	t.Setenv("AWS_ENDPOINT_URL_EVENTBRIDGE", unreachable)

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

func scoped(ctx context.Context, regions ...string) context.Context {
	ctx, _ = types.WithRegionScope(ctx, regions)
	return ctx
}

func run(t *testing.T, ctx context.Context) ([]types.ScanResult, error) {
	t.Helper()
	utils.Quiet = true
	t.Cleanup(func() { utils.Quiet = false })
	svc := SQSCalls[0]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

func names(results []types.ScanResult) []string {
	var out []string
	for _, r := range results {
		out = append(out, r.ResourceName)
	}
	return out
}

const (
	qMain = "https://sqs.us-west-2.amazonaws.com/123456789012/orders-queue"
	qDLQ  = "https://sqs.us-west-2.amazonaws.com/123456789012/orders-dlq"
	qEast = "https://sqs.eu-west-1.amazonaws.com/123456789012/eu-queue"
)

func TestServicesAreV2Only(t *testing.T) {
	if len(SQSCalls) != 1 || SQSCalls[0].Name != "sqs:ListQueues" {
		t.Fatalf("unexpected services: %+v", SQSCalls)
	}
	if SQSCalls[0].CallV2 == nil || SQSCalls[0].Call != nil {
		t.Error("sqs:ListQueues must implement CallV2 only")
	}
}

func TestArnToQueueUrl(t *testing.T) {
	if got, want := arnToQueueUrl("arn:aws:sqs:us-west-2:123456789012:orders-dlq"), qDLQ; got != want {
		t.Errorf("arnToQueueUrl = %q, want %q", got, want)
	}
	if got := arnToQueueUrl("not-an-arn"); got != "" {
		t.Errorf("arnToQueueUrl(invalid) = %q, want empty", got)
	}
}

func TestListQueuesFunctional(t *testing.T) {
	f := &fakeSQS{
		queues: map[string][]string{"us-west-2": {qMain, qDLQ}},
		attrs: map[string]map[string]string{
			qMain: {
				"QueueArn":      "arn:aws:sqs:us-west-2:123456789012:orders-queue",
				"Policy":        `{"Statement":[{"Condition":{"ArnEquals":{"aws:SourceArn":"arn:aws:events:us-west-2:123456789012:rule/orders-rule"}}}]}`,
				"RedrivePolicy": `{"deadLetterTargetArn":"arn:aws:sqs:us-west-2:123456789012:orders-dlq","maxReceiveCount":5}`,
			},
		},
	}
	results, err := run(t, scoped(newCtx(t, f), "us-west-2"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := names(results), []string{qMain, qDLQ}; !reflect.DeepEqual(got, want) {
		t.Errorf("results = %v, want %v", got, want)
	}
	for _, r := range results {
		if r.ServiceName != "SQS" || r.MethodName != "sqs:ListQueues" || r.ResourceType != "queue" {
			t.Errorf("unexpected result: %+v", r)
		}
		if !reflect.DeepEqual(r.Details, map[string]interface{}{}) {
			t.Errorf("details = %#v, want empty map", r.Details)
		}
	}

	// The listing uses the scoped region; every follow-up is pinned to
	// us-east-1 (SQS) / us-west-2 (EventBridge) as in v1, whatever the queue's
	// own region.
	if got := f.ops("ListQueues"); len(got) != 1 || got[0].region != "us-west-2" || got[0].service != "sqs" {
		t.Errorf("ListQueues = %+v", got)
	}
	for _, op := range []string{"ReceiveMessage", "GetQueueAttributes"} {
		got := f.ops(op)
		var urls []string
		for _, r := range got {
			if r.region != "us-east-1" || r.service != "sqs" {
				t.Errorf("%s signed for %s/%s, want sqs in us-east-1", op, r.service, r.region)
			}
			if u, _ := r.body["QueueUrl"].(string); u != "" {
				urls = append(urls, u)
			}
		}
		// Both queues, plus the dead-letter queue for ReceiveMessage.
		want := []string{qMain, qDLQ}
		if op == "ReceiveMessage" {
			want = []string{qMain, qDLQ, qDLQ}
		}
		if op == "GetQueueAttributes" && len(got) > 0 {
			if names, _ := got[0].body["AttributeNames"].([]interface{}); !reflect.DeepEqual(names, []interface{}{"All"}) {
				t.Errorf("AttributeNames = %v, want [All]", names)
			}
		}
		if !reflect.DeepEqual(urls, want) {
			t.Errorf("%s queue urls = %v, want %v", op, urls, want)
		}
	}
	rules := f.ops("DescribeRule")
	if len(rules) != 1 || rules[0].region != "us-west-2" || rules[0].service != "events" || rules[0].body["Name"] != "orders-rule" {
		t.Errorf("DescribeRule = %+v, want one orders-rule call signed for events in us-west-2", rules)
	}
	dl := f.ops("ListDeadLetterSourceQueues")
	if len(dl) != 1 || dl[0].region != "us-east-1" || dl[0].body["QueueUrl"] != qDLQ {
		t.Errorf("ListDeadLetterSourceQueues = %+v", dl)
	}
}

func TestEveryScopedRegionIsVisited(t *testing.T) {
	f := &fakeSQS{queues: map[string][]string{
		"us-east-1": {"https://sqs.us-east-1.amazonaws.com/123456789012/e1"},
		"eu-west-1": {qEast},
		"us-west-2": {qMain},
	}}
	regions := []string{"us-east-1", "eu-west-1", "us-west-2"}
	results, err := run(t, scoped(newCtx(t, f), regions...))
	if err != nil {
		t.Fatal(err)
	}
	var visited []string
	for _, r := range f.ops("ListQueues") {
		visited = append(visited, r.region)
	}
	if !reflect.DeepEqual(visited, regions) {
		t.Errorf("ListQueues regions = %v, want %v", visited, regions)
	}
	want := []string{"https://sqs.us-east-1.amazonaws.com/123456789012/e1", qEast, qMain}
	if got := names(results); !reflect.DeepEqual(got, want) {
		t.Errorf("results = %v, want %v", got, want)
	}
}

func TestRegionScope(t *testing.T) {
	f := &fakeSQS{queues: map[string][]string{"us-east-1": {"x"}, "eu-west-1": {qEast}}}
	ctx, scope := types.WithRegionScope(newCtx(t, f), []string{"eu-west-1"})
	if _, err := run(t, ctx); err != nil {
		t.Fatal(err)
	}
	if !scope.Used() {
		t.Error("service did not consult the region scope")
	}
	for _, r := range f.ops("ListQueues") {
		if r.region != "eu-west-1" {
			t.Errorf("ListQueues signed for %s, want only eu-west-1", r.region)
		}
	}
}

// A failing region aborts the whole listing, as in v1.
func TestListQueuesErrorAbortsAndIsNormalized(t *testing.T) {
	f := &fakeSQS{denyOps: map[string]string{"ListQueues": "AccessDeniedException"}}
	results, err := run(t, scoped(newCtx(t, f), "us-east-1", "us-west-2"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if got, want := err.Error(), "AccessDeniedException: denied ListQueues"; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
	if len(results) != 1 || results[0].Error == nil || results[0].MethodName != "sqs:ListQueues" || results[0].ServiceName != "SQS" {
		t.Fatalf("results = %+v", results)
	}
	if got := results[0].Error.Error(); got != "AccessDeniedException: denied ListQueues" {
		t.Errorf("result error = %q", got)
	}
	if n := len(f.ops("ListQueues")); n != 1 {
		t.Errorf("ListQueues calls = %d, want 1 (no later region after the failure)", n)
	}
}

// Follow-up failures are reported (not returned) and never drop the queue.
func TestFollowUpErrorsAreSwallowed(t *testing.T) {
	f := &fakeSQS{
		queues:  map[string][]string{"us-west-2": {qMain}},
		denyOps: map[string]string{"ReceiveMessage": "AccessDeniedException", "GetQueueAttributes": "AccessDeniedException"},
	}
	results, err := run(t, scoped(newCtx(t, f), "us-west-2"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := names(results), []string{qMain}; !reflect.DeepEqual(got, want) {
		t.Errorf("results = %v, want %v", got, want)
	}
	if len(f.ops("DescribeRule")) != 0 || len(f.ops("ListDeadLetterSourceQueues")) != 0 {
		t.Error("no policy follow-ups expected when GetQueueAttributes fails")
	}
}

func TestNoQueuesYieldsNoResults(t *testing.T) {
	results, err := run(t, scoped(newCtx(t, &fakeSQS{}), "us-west-2"))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("results = %+v, want none", results)
	}
}
