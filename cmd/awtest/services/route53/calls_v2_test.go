package route53

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

const ns = "https://route53.amazonaws.com/doc/2013-04-01/"

// fakeRoute53 speaks the Route 53 REST-XML protocol and records request paths.
type fakeRoute53 struct {
	status int
	body   string

	mu    sync.Mutex
	paths []string
}

func (f *fakeRoute53) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.paths = append(f.paths, r.Method+" "+r.URL.Path)
	f.mu.Unlock()
	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(f.status)
	_, _ = w.Write([]byte(f.body))
}

func newCtx(t *testing.T, f *fakeRoute53) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_ROUTE_53", unreachable)

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
	svc := Route53Calls[i]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

func TestServicesAreV2Only(t *testing.T) {
	if len(Route53Calls) != 2 || Route53Calls[0].Name != "route53:ListHostedZones" || Route53Calls[1].Name != "route53:ListHealthChecks" {
		t.Fatalf("unexpected services: %+v", Route53Calls)
	}
	for _, s := range Route53Calls {
		if s.CallV2 == nil || s.Call != nil {
			t.Errorf("%s must implement CallV2 only", s.Name)
		}
	}
}

func TestListHostedZonesFunctional(t *testing.T) {
	f := &fakeRoute53{status: 200, body: `<ListHostedZonesResponse xmlns="` + ns + `"><HostedZones>` +
		`<HostedZone><Id>/hostedzone/Z1</Id><Name>example-corp.test.</Name><CallerReference>c1</CallerReference></HostedZone>` +
		`<HostedZone><Id>/hostedzone/Z2</Id><CallerReference>c2</CallerReference></HostedZone>` +
		`</HostedZones><IsTruncated>true</IsTruncated><NextMarker>Z3</NextMarker><MaxItems>2</MaxItems></ListHostedZonesResponse>`}
	results, err := run(t, newCtx(t, f), 0)
	if err != nil {
		t.Fatal(err)
	}
	type row struct{ typ, name string }
	var got []row
	for _, r := range results {
		if r.ServiceName != "Route53" || r.MethodName != "route53:ListHostedZones" {
			t.Errorf("unexpected result identity: %+v", r)
		}
		if !reflect.DeepEqual(r.Details, map[string]interface{}{}) {
			t.Errorf("details = %#v, want empty map", r.Details)
		}
		got = append(got, row{r.ResourceType, r.ResourceName})
	}
	// The truncated page is not followed (v1 ignored NextMarker); the sparse
	// zone prints an empty name.
	want := []row{{"hosted-zone", "example-corp.test."}, {"hosted-zone", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("results = %v, want %v", got, want)
	}
	if len(f.paths) != 1 || f.paths[0] != "GET /2013-04-01/hostedzone" {
		t.Errorf("requests = %v, want a single GET /2013-04-01/hostedzone", f.paths)
	}
}

func TestListHealthChecksFunctional(t *testing.T) {
	f := &fakeRoute53{status: 200, body: `<ListHealthChecksResponse xmlns="` + ns + `"><HealthChecks>` +
		`<HealthCheck><Id>h1</Id><CallerReference>c1</CallerReference><HealthCheckConfig><IPAddress>192.0.2.10</IPAddress><Type>HTTP</Type></HealthCheckConfig><HealthCheckVersion>1</HealthCheckVersion></HealthCheck>` +
		`<HealthCheck><Id>h2</Id><CallerReference>c2</CallerReference><HealthCheckConfig><Type>HTTP</Type></HealthCheckConfig><HealthCheckVersion>1</HealthCheckVersion></HealthCheck>` +
		`<HealthCheck><Id>h3</Id><CallerReference>c3</CallerReference><HealthCheckVersion>1</HealthCheckVersion></HealthCheck>` +
		`</HealthChecks><Marker></Marker><IsTruncated>false</IsTruncated><MaxItems>100</MaxItems></ListHealthChecksResponse>`}
	results, err := run(t, newCtx(t, f), 1)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range results {
		if r.ResourceType != "health-check" || r.MethodName != "route53:ListHealthChecks" || r.ServiceName != "Route53" {
			t.Errorf("unexpected result: %+v", r)
		}
		if !reflect.DeepEqual(r.Details, map[string]interface{}{}) {
			t.Errorf("details = %#v, want empty map", r.Details)
		}
		names = append(names, r.ResourceName)
	}
	// Populated, config without address, and no config at all.
	if want := []string{"192.0.2.10", "", ""}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
	if len(f.paths) != 1 || f.paths[0] != "GET /2013-04-01/healthcheck" {
		t.Errorf("requests = %v", f.paths)
	}
}

func TestErrorsAreNormalized(t *testing.T) {
	for i, name := range []string{"route53:ListHostedZones", "route53:ListHealthChecks"} {
		f := &fakeRoute53{status: 403, body: `<ErrorResponse xmlns="` + ns + `"><Error><Type>Sender</Type><Code>AccessDenied</Code><Message>not allowed</Message></Error><RequestId>req-1</RequestId></ErrorResponse>`}
		results, err := run(t, newCtx(t, f), i)
		if err == nil {
			t.Fatalf("%s: expected an error", name)
		}
		if got, want := err.Error(), "AccessDenied: not allowed"; got != want {
			t.Errorf("%s: error = %q, want %q", name, got, want)
		}
		if len(results) != 1 || results[0].Error == nil || results[0].MethodName != name {
			t.Fatalf("%s: results = %+v", name, results)
		}
		if got := results[0].Error.Error(); got != "AccessDenied: not allowed" {
			t.Errorf("%s: result error = %q", name, got)
		}
	}
}
