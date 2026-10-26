package cloudfront

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"sync"
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

const ns = "http://cloudfront.amazonaws.com/doc/2020-05-31/"

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/cloudfront/aws4_request`)

type recorded struct {
	region string
	path   string
}

// fakeCloudFront speaks the CloudFront REST-XML protocol.
type fakeCloudFront struct {
	status int
	body   string

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeCloudFront) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region: region, path: r.Method + " " + r.URL.Path})
	f.mu.Unlock()
	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(f.status)
	_, _ = w.Write([]byte(f.body))
}

func newCtx(t *testing.T, f *fakeCloudFront, regions ...string) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_CLOUDFRONT", unreachable)

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
	svc := CloudFrontCalls[0]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

const listBody = `<DistributionList xmlns="` + ns + `"><Marker></Marker><MaxItems>100</MaxItems><IsTruncated>false</IsTruncated><Quantity>3</Quantity><Items>` +
	`<DistributionSummary><Id>E1EXAMPLE0001</Id><Origins><Quantity>2</Quantity><Items>` +
	`<Origin><Id>web-origin-1</Id><DomainName>web.example.test</DomainName></Origin>` +
	`<Origin><Id>s3-origin-2</Id><DomainName>bucket.s3.amazonaws.com</DomainName></Origin>` +
	`</Items></Origins></DistributionSummary>` +
	`<DistributionSummary><Id>E1EXAMPLE0002</Id><Origins><Quantity>0</Quantity></Origins></DistributionSummary>` +
	`<DistributionSummary></DistributionSummary>` +
	`</Items></DistributionList>`

func TestServicesAreV2Only(t *testing.T) {
	if len(CloudFrontCalls) != 1 || CloudFrontCalls[0].Name != "cloudfront:ListDistributions" {
		t.Fatalf("unexpected services: %+v", CloudFrontCalls)
	}
	if CloudFrontCalls[0].CallV2 == nil || CloudFrontCalls[0].Call != nil {
		t.Error("cloudfront:ListDistributions must implement CallV2 only")
	}
}

func TestListDistributionsFunctional(t *testing.T) {
	f := &fakeCloudFront{status: 200, body: listBody}
	results, err := run(t, newCtx(t, f, "us-east-1"))
	if err != nil {
		t.Fatal(err)
	}
	type row struct{ method, typ, name string }
	var got []row
	for _, r := range results {
		if r.ServiceName != "CloudFront" {
			t.Errorf("unexpected service: %+v", r)
		}
		if !reflect.DeepEqual(r.Details, map[string]interface{}{}) {
			t.Errorf("details = %#v, want empty map", r.Details)
		}
		got = append(got, row{r.MethodName, r.ResourceType, r.ResourceName})
	}
	// Populated distribution with two origins, one without origins and a fully
	// sparse one (empty id, no origins).
	want := []row{
		{"cloudfront:ListDistributions", "distribution", "E1EXAMPLE0001"},
		{"cloudfront:ListOrigins", "origin", "web-origin-1"},
		{"cloudfront:ListOrigins", "origin", "s3-origin-2"},
		{"cloudfront:ListDistributions", "distribution", "E1EXAMPLE0002"},
		{"cloudfront:ListDistributions", "distribution", ""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("results = %v, want %v", got, want)
	}
	if len(f.reqs) != 1 || f.reqs[0].path != "GET /2020-05-31/distribution" {
		t.Errorf("requests = %+v", f.reqs)
	}
}

func TestListDistributionsEmptyList(t *testing.T) {
	// A response without a DistributionList must not panic.
	f := &fakeCloudFront{status: 200, body: `<DistributionList xmlns="` + ns + `"><IsTruncated>false</IsTruncated><Quantity>0</Quantity></DistributionList>`}
	results, err := run(t, newCtx(t, f, "us-east-1"))
	if err != nil || len(results) != 0 {
		t.Errorf("results = %+v, err = %v; want none", results, err)
	}
}

func TestListDistributionsVisitsEveryRegion(t *testing.T) {
	f := &fakeCloudFront{status: 200, body: listBody}
	regions := []string{"us-east-1", "eu-west-1", "ap-southeast-2"}
	results, err := run(t, newCtx(t, f, regions...))
	if err != nil {
		t.Fatal(err)
	}
	// v1 called the global endpoint once per region in scope and kept every
	// distribution, so the same distribution shows up once per region.
	if len(results) != 5*len(regions) {
		t.Errorf("got %d results, want %d", len(results), 5*len(regions))
	}
	if len(f.reqs) != len(regions) {
		t.Fatalf("got %d requests, want %d: %+v", len(f.reqs), len(regions), f.reqs)
	}
	// With an explicit endpoint each call is signed for its loop region.
	var signed []string
	for _, r := range f.reqs {
		signed = append(signed, r.region)
	}
	if !reflect.DeepEqual(signed, regions) {
		t.Errorf("SigV4 regions = %v, want %v", signed, regions)
	}
}

func TestErrorsAreNormalized(t *testing.T) {
	f := &fakeCloudFront{status: 403, body: `<ErrorResponse xmlns="` + ns + `"><Error><Type>Sender</Type><Code>AccessDenied</Code><Message>not allowed</Message></Error><RequestId>req-1</RequestId></ErrorResponse>`}
	results, err := run(t, newCtx(t, f, "us-east-1"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if got, want := err.Error(), "AccessDenied: not allowed"; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
	if len(results) != 1 || results[0].Error == nil || results[0].MethodName != "cloudfront:ListDistributions" {
		t.Fatalf("results = %+v", results)
	}
	if got := results[0].Error.Error(); got != "AccessDenied: not allowed" {
		t.Errorf("result error = %q", got)
	}
}

func TestErrorInLaterRegionAbortsScan(t *testing.T) {
	// v1 returned the first error and discarded earlier regions' results.
	var mu sync.Mutex
	n := 0
	srv := &fakeCloudFront{}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n++
		call := n
		mu.Unlock()
		if call == 1 {
			srv.status, srv.body = 200, listBody
		} else {
			srv.status, srv.body = 403, `<ErrorResponse xmlns="`+ns+`"><Error><Type>Sender</Type><Code>AccessDenied</Code><Message>no</Message></Error><RequestId>r</RequestId></ErrorResponse>`
		}
		srv.ServeHTTP(w, r)
	})
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)

	t.Setenv("HOME", t.TempDir())
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	cfg, err := awsv2.LoadConfig(context.Background(), awsv2.Options{
		Region: "us-west-2", AccessKeyID: "AKIAFAKEFAKEFAKE", SecretAccessKey: "fake",
		Endpoint: ts.URL, HTTPClient: ts.Client(), MaxAttempts: 1, Quiet: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, _ := types.WithRegionScope(types.WithAWSConfig(context.Background(), cfg), []string{"us-east-1", "eu-west-1"})
	results, err := run(t, ctx)
	if err == nil || err.Error() != "AccessDenied: no" {
		t.Fatalf("err = %v", err)
	}
	if len(results) != 1 || results[0].Error == nil {
		t.Errorf("results = %+v, want a single error result", results)
	}
}
