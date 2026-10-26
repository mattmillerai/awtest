package kinesis

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

const targetPrefix = "Kinesis_20131202."

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
	t.Setenv("AWS_ENDPOINT_URL_KINESIS", unreachable)

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
	svc := KinesisCalls[i]
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

func TestListStreamsInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		if op != "ListStreams" {
			return apiError("UnknownOperationException", op)
		}
		if body["NextToken"] == "t2" {
			return http.StatusOK, map[string]interface{}{"StreamSummaries": []map[string]string{{"StreamName": "bare-stream"}}}
		}
		return http.StatusOK, map[string]interface{}{"StreamSummaries": []map[string]interface{}{{
			"StreamName": "demo-stream", "StreamARN": "arn:aws:kinesis:us-west-2:111111111111:stream/demo-stream",
			"StreamStatus": "ACTIVE", "StreamModeDetails": map[string]string{"StreamMode": "ON_DEMAND"},
			"StreamCreationTimestamp": 1700000000,
		}}, "NextToken": "t2"}
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"StreamName": "demo-stream", "StreamARN": "arn:aws:kinesis:us-west-2:111111111111:stream/demo-stream",
			"StreamStatus": "ACTIVE", "StreamMode": "ON_DEMAND", "CreationTimestamp": "2023-11-14T22:13:20Z",
			"Region": "us-west-2",
		},
		{
			"StreamName": "bare-stream", "StreamARN": "", "StreamStatus": "", "StreamMode": "",
			"CreationTimestamp": "", "Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ServiceName != "Kinesis" || results[0].MethodName != "kinesis:ListStreams" || results[0].ResourceType != "stream" || results[0].ResourceName != "demo-stream" {
		t.Errorf("result 0 = %+v", results[0])
	}
	if got := len(f.ops("ListStreams")); got != 2 {
		t.Errorf("ListStreams calls = %d, want 2", got)
	}
}

func TestListShardsInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		switch op {
		case "ListStreams":
			return http.StatusOK, map[string]interface{}{"StreamSummaries": []map[string]string{{"StreamName": "s1"}, {"StreamARN": "arn-without-name"}}}
		case "ListShards":
			if body["StreamName"] == "s1" {
				return http.StatusOK, map[string]interface{}{"Shards": []map[string]interface{}{{
					"ShardId": "shardId-0", "ParentShardId": "shardId-parent",
					"HashKeyRange":        map[string]string{"StartingHashKey": "0", "EndingHashKey": "99"},
					"SequenceNumberRange": map[string]string{"StartingSequenceNumber": "11", "EndingSequenceNumber": "22"},
				}}, "NextToken": "st2"}
			}
			if body["NextToken"] == "st2" {
				if _, has := body["StreamName"]; has {
					return apiError("InvalidArgumentException", "StreamName must not be sent with NextToken")
				}
				return http.StatusOK, map[string]interface{}{"Shards": []map[string]string{{"ShardId": "shardId-bare"}}}
			}
		}
		return apiError("UnknownOperationException", op)
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"ShardId": "shardId-0", "StreamName": "s1", "ParentShardId": "shardId-parent",
			"StartingHashKey": "0", "EndingHashKey": "99", "StartingSequenceNumber": "11",
			"EndingSequenceNumber": "22", "Region": "us-west-2",
		},
		{
			"ShardId": "shardId-bare", "StreamName": "s1", "ParentShardId": "",
			"StartingHashKey": "", "EndingHashKey": "", "StartingSequenceNumber": "",
			"EndingSequenceNumber": "", "Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ResourceType != "shard" || results[0].ResourceName != "shardId-0" {
		t.Errorf("result 0 = %+v", results[0])
	}
	// The stream without a name is never queried.
	if got := len(f.ops("ListShards")); got != 2 {
		t.Errorf("ListShards calls = %d, want 2", got)
	}
}

func TestListStreamConsumersInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		switch op {
		case "ListStreams":
			return http.StatusOK, map[string]interface{}{"StreamSummaries": []map[string]string{
				{"StreamName": "s1", "StreamARN": "arn:s1"}, {"StreamName": "no-arn"},
			}}
		case "ListStreamConsumers":
			if body["StreamARN"] != "arn:s1" {
				return apiError("ResourceNotFoundException", "unexpected stream")
			}
			if body["NextToken"] == "c2" {
				return http.StatusOK, map[string]interface{}{"Consumers": []map[string]string{{"ConsumerName": "bare-consumer"}}}
			}
			return http.StatusOK, map[string]interface{}{"Consumers": []map[string]interface{}{{
				"ConsumerName": "demo-consumer", "ConsumerARN": "arn:consumer", "ConsumerStatus": "ACTIVE",
				"ConsumerCreationTimestamp": 1700000000,
			}}, "NextToken": "c2"}
		}
		return apiError("UnknownOperationException", op)
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"ConsumerName": "demo-consumer", "ConsumerARN": "arn:consumer", "ConsumerStatus": "ACTIVE",
			"StreamName": "s1", "CreationTimestamp": "2023-11-14T22:13:20Z", "Region": "us-west-2",
		},
		{
			"ConsumerName": "bare-consumer", "ConsumerARN": "", "ConsumerStatus": "",
			"StreamName": "s1", "CreationTimestamp": "", "Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ResourceType != "consumer" || results[0].ResourceName != "demo-consumer" {
		t.Errorf("result 0 = %+v", results[0])
	}
	if got := len(f.ops("ListStreamConsumers")); got != 2 {
		t.Errorf("ListStreamConsumers calls = %d, want 2", got)
	}
}

func TestInvokeErrorText(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		return apiError("AccessDeniedException", "not authorized for "+op)
	}}
	for i := range KinesisCalls {
		ctx := newEnv(t, f, "us-west-2")
		results, err := run(ctx, i)
		if err == nil {
			t.Fatalf("%s: expected an error", KinesisCalls[i].Name)
		}
		want := "AccessDeniedException: not authorized for ListStreams"
		if err.Error() != want {
			t.Errorf("%s: error = %q, want %q", KinesisCalls[i].Name, err.Error(), want)
		}
		if len(results) != 1 || results[0].Error == nil || results[0].Error.Error() != want {
			t.Errorf("%s: Process error result = %+v", KinesisCalls[i].Name, results)
		}
		if results[0].MethodName != KinesisCalls[i].Name {
			t.Errorf("MethodName = %q", results[0].MethodName)
		}
	}
}

func TestRegionScopeVisitsEachRegion(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		switch op {
		case "ListStreams":
			return http.StatusOK, map[string]interface{}{"StreamSummaries": []map[string]string{{"StreamName": "s-" + region, "StreamARN": "arn:" + region}}}
		case "ListShards":
			return http.StatusOK, map[string]interface{}{"Shards": []map[string]string{{"ShardId": "sh-" + region}}}
		case "ListStreamConsumers":
			return http.StatusOK, map[string]interface{}{"Consumers": []map[string]string{{"ConsumerName": "c-" + region}}}
		}
		return apiError("UnknownOperationException", op)
	}}
	scope := []string{"us-east-1", "eu-west-1", "ap-south-1"}
	for _, tc := range []struct {
		idx int
		ops []string
	}{
		{0, []string{"ListStreams"}},
		{1, []string{"ListStreams", "ListShards"}},
		{2, []string{"ListStreams", "ListStreamConsumers"}},
	} {
		ctx := newEnv(t, f, scope...)
		f.mu.Lock()
		f.reqs = nil
		f.mu.Unlock()
		results, err := run(ctx, tc.idx)
		if err != nil {
			t.Fatal(err)
		}
		for _, op := range tc.ops {
			if got := f.regionsOf(op); !reflect.DeepEqual(got, scope) {
				t.Errorf("%s SigV4 regions = %v, want %v", op, got, scope)
			}
		}
		var regions []string
		for _, d := range detailsOf(results) {
			regions = append(regions, d["Region"].(string))
		}
		if !reflect.DeepEqual(regions, scope) {
			t.Errorf("%s result regions = %v, want %v", KinesisCalls[tc.idx].Name, regions, scope)
		}
	}
}
