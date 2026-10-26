package elasticbeanstalk

import (
	"context"
	"fmt"
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

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/elasticbeanstalk/aws4_request`)

type recorded struct{ region, action string }

// fakeBeanstalk speaks the Elastic Beanstalk query/XML protocol for
// DescribeApplications and DescribeEvents. An empty string yields an entry
// without its name/message field.
type fakeBeanstalk struct {
	apps, events []string
	// deny maps action -> AccessDenied message.
	deny map[string]string

	mu   sync.Mutex
	reqs []recorded
}

func members(field string, vals []string) string {
	var sb strings.Builder
	for _, v := range vals {
		sb.WriteString("<member>")
		if v != "" {
			sb.WriteString("<" + field + ">" + v + "</" + field + ">")
		}
		sb.WriteString("</member>")
	}
	return sb.String()
}

func (f *fakeBeanstalk) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	_ = r.ParseForm()
	action := r.PostForm.Get("Action")
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, action})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "text/xml")
	const ns = ` xmlns="http://elasticbeanstalk.amazonaws.com/docs/2010-12-01/"`
	if msg, ok := f.deny[action]; ok {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `<ErrorResponse%s><Error><Type>Sender</Type><Code>AccessDenied</Code><Message>%s</Message></Error><RequestId>req-1</RequestId></ErrorResponse>`, ns, msg)
		return
	}
	switch action {
	case "DescribeApplications":
		fmt.Fprintf(w, `<DescribeApplicationsResponse%s><DescribeApplicationsResult><Applications>%s</Applications></DescribeApplicationsResult><ResponseMetadata><RequestId>req-1</RequestId></ResponseMetadata></DescribeApplicationsResponse>`, ns, members("ApplicationName", f.apps))
	case "DescribeEvents":
		fmt.Fprintf(w, `<DescribeEventsResponse%s><DescribeEventsResult><Events>%s</Events></DescribeEventsResult><ResponseMetadata><RequestId>req-1</RequestId></ResponseMetadata></DescribeEventsResponse>`, ns, members("Message", f.events))
	default:
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `<ErrorResponse%s><Error><Type>Sender</Type><Code>InvalidAction</Code><Message>%s</Message></Error><RequestId>req-1</RequestId></ErrorResponse>`, ns, action)
	}
}

func (f *fakeBeanstalk) ops() []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recorded(nil), f.reqs...)
}

func newEnv(t *testing.T, f *fakeBeanstalk) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_ELASTIC_BEANSTALK", unreachable)

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
	svc := ElasticBeanstalkCalls[i]
	out, err := svc.Invoke(ctx)
	return svc.Process(out, err, false), err
}

func TestServicesAreV2Only(t *testing.T) {
	want := []string{"elasticbeanstalk:DescribeApplications", "elasticbeanstalk:DescribeEvents"}
	if len(ElasticBeanstalkCalls) != len(want) {
		t.Fatalf("expected %d services, got %d", len(want), len(ElasticBeanstalkCalls))
	}
	for i, svc := range ElasticBeanstalkCalls {
		if svc.Name != want[i] {
			t.Errorf("service %d name = %q, want %q", i, svc.Name, want[i])
		}
		if svc.CallV2 == nil {
			t.Errorf("%s: want CallV2 set and Call nil", svc.Name)
		}
	}
}

func TestInvokeProcessDetails(t *testing.T) {
	cases := []struct {
		idx          int
		action, kind string
	}{
		{0, "DescribeApplications", "application"},
		{1, "DescribeEvents", "event"},
	}
	for _, c := range cases {
		t.Run(c.action, func(t *testing.T) {
			f := &fakeBeanstalk{apps: []string{"my-app", ""}, events: []string{"environment health changed", ""}}
			results, err := run(newEnv(t, f), c.idx)
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 2 {
				t.Fatalf("got %d results: %+v", len(results), results)
			}
			names := []string{"my-app", ""}
			if c.idx == 1 {
				names[0] = "environment health changed"
			}
			// The second entry is sparse (no name/message): v1 would have panicked.
			for j, name := range names {
				r := results[j]
				if r.ServiceName != "ElasticBeanstalk" || r.MethodName != "elasticbeanstalk:"+c.action ||
					r.ResourceType != c.kind || r.ResourceName != name || r.Error != nil || r.Timestamp.IsZero() {
					t.Errorf("unexpected result %+v", r)
				}
				if !reflect.DeepEqual(r.Details, map[string]interface{}{}) {
					t.Errorf("details = %#v, want empty map", r.Details)
				}
			}
			// Single, non-looping call signed for the configured region.
			if got := f.ops(); !reflect.DeepEqual(got, []recorded{{"eu-central-1", c.action}}) {
				t.Errorf("requests = %v", got)
			}
		})
	}
}

func TestErrorIsNormalized(t *testing.T) {
	for i, action := range []string{"DescribeApplications", "DescribeEvents"} {
		t.Run(action, func(t *testing.T) {
			f := &fakeBeanstalk{deny: map[string]string{action: "not allowed"}}
			results, err := run(newEnv(t, f), i)
			if err == nil {
				t.Fatal("expected error")
			}
			if got, want := err.Error(), "AccessDenied: not allowed"; got != want {
				t.Errorf("error = %q, want %q", got, want)
			}
			if len(results) != 1 || results[0].Error == nil || results[0].MethodName != "elasticbeanstalk:"+action {
				t.Errorf("results = %+v", results)
			}
		})
	}
}
