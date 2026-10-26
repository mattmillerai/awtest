package cloudwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	service string // SigV4 service: monitoring or logs
	action  string // query Action or JSON target operation
	body    map[string]interface{}
}

// fakeAWS answers both CloudWatch (query protocol, XML) and CloudWatch Logs
// (JSON 1.1) requests.
type fakeAWS struct {
	// handler returns the HTTP status and body; for logs requests the body is
	// JSON, for monitoring requests XML.
	handler func(r recorded) (int, string)

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeAWS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rec := recorded{}
	if m := credScope.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		rec.region, rec.service = m[1], m[2]
	}
	raw, _ := io.ReadAll(r.Body)
	if rec.service == "logs" {
		rec.action = strings.TrimPrefix(r.Header.Get("X-Amz-Target"), "Logs_20140328.")
		_ = json.Unmarshal(raw, &rec.body)
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	} else {
		form, _ := url.ParseQuery(string(raw))
		rec.action = form.Get("Action")
		w.Header().Set("Content-Type", "text/xml")
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, rec)
	f.mu.Unlock()

	status, body := f.handler(rec)
	if status >= 400 && rec.service == "logs" {
		var e struct {
			Type string `json:"__type"`
		}
		_ = json.Unmarshal([]byte(body), &e)
		w.Header().Set("X-Amzn-ErrorType", e.Type)
	}
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func (f *fakeAWS) calls(action string) []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []recorded
	for _, r := range f.reqs {
		if r.action == action {
			out = append(out, r)
		}
	}
	return out
}

const monNS = "http://monitoring.amazonaws.com/doc/2010-08-01/"

func monResp(action, inner string) string {
	return fmt.Sprintf(`<%[1]sResponse xmlns="%[3]s"><%[1]sResult>%[2]s</%[1]sResult><ResponseMetadata><RequestId>r</RequestId></ResponseMetadata></%[1]sResponse>`, action, inner, monNS)
}

func monErr(code, msg string) string {
	return fmt.Sprintf(`<ErrorResponse xmlns="%s"><Error><Type>Sender</Type><Code>%s</Code><Message>%s</Message></Error><RequestId>r</RequestId></ErrorResponse>`, monNS, code, msg)
}

func logsErr(code, msg string) string {
	return fmt.Sprintf(`{"__type":%q,"message":%q}`, code, msg)
}

func newEnv(t *testing.T, f *fakeAWS, regions ...string) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_CLOUDWATCH", unreachable)
	t.Setenv("AWS_ENDPOINT_URL_CLOUDWATCH_LOGS", unreachable)

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

func run(t *testing.T, ctx context.Context, i int) ([]types.ScanResult, error) {
	t.Helper()
	utils.Quiet = true
	t.Cleanup(func() { utils.Quiet = false })
	svc := CloudwatchCalls[i]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

type row struct{ typ, name string }

func rows(t *testing.T, results []types.ScanResult, method string) []row {
	t.Helper()
	var out []row
	for _, r := range results {
		if r.MethodName != method {
			t.Errorf("method = %q, want %q", r.MethodName, method)
		}
		if !reflect.DeepEqual(r.Details, map[string]interface{}{}) {
			t.Errorf("details = %#v, want empty map", r.Details)
		}
		out = append(out, row{r.ResourceType, r.ResourceName})
	}
	return out
}

func TestServicesAreV2Only(t *testing.T) {
	want := []string{"cloudwatch:DescribeAlarms", "cloudwatchlogs:DescribeLogGroupsAndStreams", "cloudwatchlogs:ListMetrics"}
	if len(CloudwatchCalls) != len(want) {
		t.Fatalf("unexpected services: %+v", CloudwatchCalls)
	}
	for i, s := range CloudwatchCalls {
		if s.Name != want[i] {
			t.Errorf("service %d = %q, want %q", i, s.Name, want[i])
		}
		if s.CallV2 == nil || s.Call != nil {
			t.Errorf("%s must implement CallV2 only", s.Name)
		}
	}
}

func TestDescribeAlarmsFunctional(t *testing.T) {
	f := &fakeAWS{handler: func(r recorded) (int, string) {
		return 200, monResp("DescribeAlarms", `<MetricAlarms>`+
			`<member><AlarmName>high-cpu-web-fleet</AlarmName></member>`+
			`<member></member>`+
			`</MetricAlarms><NextToken>more</NextToken>`)
	}}
	results, err := run(t, newEnv(t, f, "us-east-1", "eu-west-1"), 0)
	if err != nil {
		t.Fatal(err)
	}
	got := rows(t, results, "cloudwatch:DescribeAlarms")
	// Two regions; the sparse alarm has an empty name; NextToken is ignored.
	want := []row{{"alarm", "high-cpu-web-fleet"}, {"alarm", ""}, {"alarm", "high-cpu-web-fleet"}, {"alarm", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("results = %v, want %v", got, want)
	}
	for _, r := range results {
		if r.ServiceName != "CloudWatch" {
			t.Errorf("service = %q", r.ServiceName)
		}
	}
	reqs := f.calls("DescribeAlarms")
	if len(reqs) != 2 || reqs[0].region != "us-east-1" || reqs[1].region != "eu-west-1" {
		t.Errorf("requests = %+v, want one per region in order", reqs)
	}
	for _, r := range reqs {
		if r.service != "monitoring" {
			t.Errorf("signed for service %q", r.service)
		}
	}
}

func TestDescribeAlarmsErrorAbortsRegionLoop(t *testing.T) {
	f := &fakeAWS{handler: func(r recorded) (int, string) {
		if r.region == "eu-west-1" {
			return 403, monErr("AccessDenied", "not allowed")
		}
		return 200, monResp("DescribeAlarms", `<MetricAlarms><member><AlarmName>a</AlarmName></member></MetricAlarms>`)
	}}
	results, err := run(t, newEnv(t, f, "us-east-1", "eu-west-1", "ap-southeast-2"), 0)
	if err == nil || err.Error() != "AccessDenied: not allowed" {
		t.Fatalf("err = %v, want AccessDenied: not allowed", err)
	}
	if len(results) != 1 || results[0].Error == nil || results[0].Error.Error() != "AccessDenied: not allowed" || results[0].MethodName != "cloudwatch:DescribeAlarms" {
		t.Errorf("results = %+v", results)
	}
	if n := len(f.calls("DescribeAlarms")); n != 2 {
		t.Errorf("%d requests, want 2 (the first error stops the loop)", n)
	}
}

func TestListMetricsFunctional(t *testing.T) {
	f := &fakeAWS{handler: func(r recorded) (int, string) {
		return 200, monResp("ListMetrics", `<Metrics>`+
			`<member><MetricName>CPUUtilization</MetricName><Namespace>AWS/EC2</Namespace></member>`+
			`<member><Namespace>AWS/EC2</Namespace></member>`+
			`</Metrics><NextToken>more</NextToken>`)
	}}
	results, err := run(t, newEnv(t, f, "us-east-1", "eu-west-1"), 2)
	if err != nil {
		t.Fatal(err)
	}
	got := rows(t, results, "cloudwatchlogs:ListMetrics")
	want := []row{{"metric", "CPUUtilization"}, {"metric", ""}, {"metric", "CPUUtilization"}, {"metric", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("results = %v, want %v", got, want)
	}
	for _, r := range results {
		if r.ServiceName != "CloudWatch" {
			t.Errorf("service = %q", r.ServiceName)
		}
	}
	reqs := f.calls("ListMetrics")
	if len(reqs) != 2 || reqs[0].region != "us-east-1" || reqs[1].region != "eu-west-1" {
		t.Errorf("requests = %+v, want one per region in order", reqs)
	}
}

func TestListMetricsError(t *testing.T) {
	f := &fakeAWS{handler: func(r recorded) (int, string) {
		return 400, monErr("InvalidParameterValue", "bad value")
	}}
	results, err := run(t, newEnv(t, f, "us-east-1"), 2)
	if err == nil || err.Error() != "InvalidParameterValue: bad value" {
		t.Fatalf("err = %v", err)
	}
	if len(results) != 1 || results[0].Error == nil || results[0].ServiceName != "CloudWatch" || results[0].Error.Error() != "InvalidParameterValue: bad value" {
		t.Errorf("results = %+v", results)
	}
}

// logsHandler serves a fixed two-page group listing for every region:
//
//	page 1: group-a (two stream pages), group-b (no streams); nextToken g2
//	page 2: group-c (streams: one unnamed)
func logsHandler(failStreamsFor string) func(recorded) (int, string) {
	return func(r recorded) (int, string) {
		switch r.action {
		case "DescribeLogGroups":
			if r.body["nextToken"] == "g2" {
				return 200, `{"logGroups":[{"logGroupName":"group-c"}]}`
			}
			return 200, `{"logGroups":[{"logGroupName":"group-a"},{"logGroupName":"group-b"}],"nextToken":"g2"}`
		case "DescribeLogStreams":
			group, _ := r.body["logGroupName"].(string)
			if group == failStreamsFor {
				return 400, logsErr("AccessDeniedException", "no streams for you")
			}
			switch group {
			case "group-a":
				if r.body["nextToken"] == "s2" {
					return 200, `{"logStreams":[{"logStreamName":"stream-2"}]}`
				}
				return 200, `{"logStreams":[{"logStreamName":"stream-1"}],"nextToken":"s2"}`
			case "group-c":
				return 200, `{"logStreams":[{}]}`
			}
			return 200, `{"logStreams":[]}`
		}
		return 400, logsErr("UnknownOperationException", r.action)
	}
}

func TestLogGroupsAndStreamsFunctional(t *testing.T) {
	f := &fakeAWS{handler: logsHandler("")}
	results, err := run(t, newEnv(t, f, "us-east-1"), 1)
	if err != nil {
		t.Fatal(err)
	}
	const method = "cloudwatchlogs:DescribeLogGroupsAndStreams"
	got := rows(t, results, method)
	want := []row{
		{"log-group", "group-a"}, {"log-stream", "stream-1"}, {"log-stream", "stream-2"},
		{"log-group", "group-b"},
		{"log-group", "group-c"}, {"log-stream", ""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("results = %v, want %v", got, want)
	}
	for _, r := range results {
		if r.ServiceName != "CloudWatchLogs" {
			t.Errorf("service = %q", r.ServiceName)
		}
	}
	// Both group pages and both stream pages of group-a were requested.
	if n := len(f.calls("DescribeLogGroups")); n != 2 {
		t.Errorf("%d DescribeLogGroups calls, want 2", n)
	}
	if n := len(f.calls("DescribeLogStreams")); n != 4 {
		t.Errorf("%d DescribeLogStreams calls, want 4 (a x2, b, c)", n)
	}
}

func TestLogGroupsNoneFound(t *testing.T) {
	f := &fakeAWS{handler: func(r recorded) (int, string) { return 200, `{"logGroups":[]}` }}
	results, err := run(t, newEnv(t, f, "us-east-1"), 1)
	if err != nil || len(results) != 0 {
		t.Errorf("results = %+v, err = %v; want none", results, err)
	}
}

func TestLogStreamErrorEndsPagingSilently(t *testing.T) {
	// v1 stopped paging log groups when listing a group's streams failed and
	// reported neither the error nor the failing group; groups collected so
	// far are kept and the next region is still visited.
	f := &fakeAWS{handler: logsHandler("group-b")}
	results, err := run(t, newEnv(t, f, "us-east-1", "eu-west-1"), 1)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	got := rows(t, results, "cloudwatchlogs:DescribeLogGroupsAndStreams")
	want := []row{
		{"log-group", "group-a"}, {"log-stream", "stream-1"}, {"log-stream", "stream-2"},
		{"log-group", "group-a"}, {"log-stream", "stream-1"}, {"log-stream", "stream-2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("results = %v, want %v", got, want)
	}
	// Page 2 of the groups is never requested after the failure.
	if n := len(f.calls("DescribeLogGroups")); n != 2 {
		t.Errorf("%d DescribeLogGroups calls, want 2 (one per region)", n)
	}
	regions := map[string]bool{}
	for _, r := range f.calls("DescribeLogGroups") {
		regions[r.region] = true
	}
	if !regions["us-east-1"] || !regions["eu-west-1"] {
		t.Errorf("regions visited = %v", regions)
	}
}

func TestLogGroupsError(t *testing.T) {
	f := &fakeAWS{handler: func(r recorded) (int, string) {
		return 400, logsErr("AccessDeniedException", "not allowed")
	}}
	results, err := run(t, newEnv(t, f, "us-east-1"), 1)
	if err == nil || err.Error() != "AccessDeniedException: not allowed" {
		t.Fatalf("err = %v", err)
	}
	if len(results) != 1 || results[0].Error == nil || results[0].ServiceName != "CloudWatchLogs" || results[0].Error.Error() != "AccessDeniedException: not allowed" {
		t.Errorf("results = %+v", results)
	}
}

func TestLogGroupsVisitEveryRegion(t *testing.T) {
	f := &fakeAWS{handler: func(r recorded) (int, string) { return 200, `{"logGroups":[]}` }}
	regions := []string{"us-east-1", "eu-west-1", "ap-southeast-2"}
	if _, err := run(t, newEnv(t, f, regions...), 1); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range f.calls("DescribeLogGroups") {
		if r.service != "logs" {
			t.Errorf("signed for service %q", r.service)
		}
		got = append(got, r.region)
	}
	if !reflect.DeepEqual(got, regions) {
		t.Errorf("SigV4 regions = %v, want %v", got, regions)
	}
}
