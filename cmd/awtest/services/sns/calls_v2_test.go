package sns

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/sns/aws4_request`)

type recorded struct {
	region string
	action string
	form   url.Values
}

// fakeSNS speaks the SNS query protocol (form request, XML response).
type fakeSNS struct {
	// handler returns the HTTP status and the XML body of the response.
	handler func(r recorded) (int, string)

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeSNS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	_ = r.ParseForm()
	rec := recorded{region: region, action: r.Form.Get("Action"), form: r.Form}
	f.mu.Lock()
	f.reqs = append(f.reqs, rec)
	f.mu.Unlock()

	status, body := f.handler(rec)
	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func (f *fakeSNS) calls(action string) []recorded {
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

const ns = "http://sns.amazonaws.com/doc/2010-03-31/"

func resp(action, inner string) string {
	return fmt.Sprintf(`<%[1]sResponse xmlns="%[3]s"><%[1]sResult>%[2]s</%[1]sResult><ResponseMetadata><RequestId>r</RequestId></ResponseMetadata></%[1]sResponse>`, action, inner, ns)
}

func errResp(code, msg string) string {
	return fmt.Sprintf(`<ErrorResponse xmlns="%s"><Error><Type>Sender</Type><Code>%s</Code><Message>%s</Message></Error><RequestId>r</RequestId></ErrorResponse>`, ns, code, msg)
}

func attrs(kv map[string]string) string {
	var keys []string
	for k := range kv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString("<Attributes>")
	for _, k := range keys {
		fmt.Fprintf(&sb, "<entry><key>%s</key><value>%s</value></entry>", k, kv[k])
	}
	sb.WriteString("</Attributes>")
	return sb.String()
}

func newEnv(t *testing.T, f *fakeSNS, regions ...string) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_SNS", unreachable)

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

func run(t *testing.T, ctx context.Context) ([]types.ScanResult, error) {
	t.Helper()
	utils.Quiet = true
	t.Cleanup(func() { utils.Quiet = false })
	svc := SNSCalls[0]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

func TestServicesAreV2Only(t *testing.T) {
	if len(SNSCalls) != 1 || SNSCalls[0].Name != "sns:ListTopics" {
		t.Fatalf("unexpected services: %+v", SNSCalls)
	}
	if SNSCalls[0].CallV2 == nil || SNSCalls[0].Call != nil {
		t.Error("sns:ListTopics must implement CallV2 only")
	}
}

func TestListTopicsFunctional(t *testing.T) {
	const (
		arnA = "arn:aws:sns:us-east-1:111111111111:alpha"
		arnB = "arn:aws:sns:eu-west-1:111111111111:beta"
	)
	f := &fakeSNS{handler: func(r recorded) (int, string) {
		switch r.action {
		case "ListTopics":
			// NextToken is ignored by v1 and must stay ignored.
			return 200, resp("ListTopics", `<Topics><member><TopicArn>`+arnA+`</TopicArn></member><member><TopicArn>`+arnB+`</TopicArn></member></Topics><NextToken>more</NextToken>`)
		case "GetTopicAttributes":
			if r.form.Get("TopicArn") == arnA {
				return 200, resp("GetTopicAttributes", attrs(map[string]string{
					"DisplayName": "Alpha", "SubscriptionsConfirmed": "3", "SubscriptionsPending": "", "TopicArn": arnA,
				}))
			}
			return 200, resp("GetTopicAttributes", `<Attributes/>`)
		}
		return 400, errResp("InvalidAction", r.action)
	}}
	ctx := newEnv(t, f, "us-east-1")
	res, err := run(t, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(f.calls("ListTopics")); got != 1 {
		t.Errorf("ListTopics calls = %d, want 1 (NextToken ignored)", got)
	}
	if len(res) != 2 {
		t.Fatalf("got %d results: %+v", len(res), res)
	}
	a := res[0]
	wantA := map[string]interface{}{"DisplayName": "Alpha", "SubscriptionsConfirmed": "3", "SubscriptionsPending": "", "TopicArn": arnA}
	if a.ServiceName != "SNS" || a.MethodName != "sns:ListTopics" || a.ResourceType != "topic" || a.ResourceName != arnA || !reflect.DeepEqual(a.Details, wantA) {
		t.Errorf("populated = %+v", a)
	}
	// A topic without attributes still yields an (empty) details map.
	b := res[1]
	if b.ResourceName != arnB || b.Details == nil || len(b.Details) != 0 {
		t.Errorf("sparse = %+v", b)
	}
	// Attributes are read in the region named by each topic's ARN.
	attr := f.calls("GetTopicAttributes")
	if len(attr) != 2 || attr[0].region != "us-east-1" || attr[1].region != "eu-west-1" {
		t.Errorf("GetTopicAttributes regions = %+v", attr)
	}
}

func TestListTopicsErrors(t *testing.T) {
	denied := &fakeSNS{handler: func(r recorded) (int, string) {
		return 403, errResp("AuthorizationError", "not authorized to list")
	}}
	res, err := run(t, newEnv(t, denied, "us-east-1"))
	const want = "AuthorizationError: not authorized to list"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
	if len(res) != 1 || res[0].Error == nil || res[0].Error.Error() != want || res[0].MethodName != "sns:ListTopics" {
		t.Errorf("results = %+v", res)
	}

	// A failing GetTopicAttributes aborts the scan (v1 behaviour).
	attrDenied := &fakeSNS{handler: func(r recorded) (int, string) {
		if r.action == "ListTopics" {
			return 200, resp("ListTopics", `<Topics><member><TopicArn>arn:aws:sns:us-east-1:1:t</TopicArn></member></Topics>`)
		}
		return 403, errResp("AuthorizationError", "no attrs")
	}}
	if _, err := run(t, newEnv(t, attrDenied, "us-east-1")); err == nil || err.Error() != "AuthorizationError: no attrs" {
		t.Errorf("attribute error = %v", err)
	}

	// A malformed ARN aborts the scan before any attribute call.
	bad := &fakeSNS{handler: func(r recorded) (int, string) {
		return 200, resp("ListTopics", `<Topics><member><TopicArn>bogus</TopicArn></member></Topics>`)
	}}
	if _, err := run(t, newEnv(t, bad, "us-east-1")); err == nil || err.Error() != "invalid ARN: bogus" {
		t.Errorf("invalid ARN error = %v", err)
	}
	if n := len(bad.calls("GetTopicAttributes")); n != 0 {
		t.Errorf("GetTopicAttributes calls = %d, want 0", n)
	}
}

func TestEveryScopeRegionIsVisitedWithItsSigningRegion(t *testing.T) {
	scope := []string{"us-east-1", "eu-west-1", "ap-southeast-2"}
	f := &fakeSNS{handler: func(r recorded) (int, string) {
		return 200, resp("ListTopics", `<Topics/>`)
	}}
	if _, err := run(t, newEnv(t, f, scope...)); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range f.calls("ListTopics") {
		got = append(got, r.region)
	}
	sort.Strings(got)
	want := append([]string(nil), scope...)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("visited %v, want %v", got, want)
	}
}
