package directconnect

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

const targetPrefix = "OvertureService."

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
	t.Setenv("AWS_ENDPOINT_URL_DIRECT_CONNECT", unreachable)

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
	svc := DirectConnectCalls[i]
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

func TestDescribeConnectionsInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		if op != "DescribeConnections" {
			return apiError("UnknownOperationException", op)
		}
		return http.StatusOK, map[string]interface{}{"connections": []map[string]interface{}{
			{
				"connectionId": "dxcon-abc12345", "connectionName": "demo-conn", "connectionState": "available",
				"bandwidth": "1Gbps", "location": "EqDC2", "ownerAccount": "111111111111", "partnerName": "Equinix",
			},
			{"connectionName": "bare-conn"},
		}}
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"ConnectionId": "dxcon-abc12345", "ConnectionName": "demo-conn", "ConnectionState": "available",
			"Bandwidth": "1Gbps", "Location": "EqDC2", "OwnerAccount": "111111111111", "PartnerName": "Equinix",
			"Region": "us-west-2",
		},
		{
			"ConnectionId": "", "ConnectionName": "bare-conn", "ConnectionState": "",
			"Bandwidth": "", "Location": "", "OwnerAccount": "", "PartnerName": "",
			"Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	for i, name := range []string{"demo-conn", "bare-conn"} {
		r := results[i]
		if r.ServiceName != "DirectConnect" || r.MethodName != "directconnect:DescribeConnections" || r.ResourceType != "connection" || r.ResourceName != name {
			t.Errorf("result %d = %+v", i, r)
		}
	}
}

func TestDescribeVirtualInterfacesInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		if op != "DescribeVirtualInterfaces" {
			return apiError("UnknownOperationException", op)
		}
		return http.StatusOK, map[string]interface{}{"virtualInterfaces": []map[string]interface{}{
			{
				"virtualInterfaceId": "dxvif-1", "virtualInterfaceName": "demo-vif", "virtualInterfaceState": "available",
				"virtualInterfaceType": "private", "connectionId": "dxcon-1", "vlan": 101, "asn": 65001,
				"amazonAddress": "169.254.0.1/30", "customerAddress": "169.254.0.2/30",
			},
			{"virtualInterfaceName": "bare-vif"},
		}}
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"VirtualInterfaceId": "dxvif-1", "VirtualInterfaceName": "demo-vif", "VirtualInterfaceState": "available",
			"VirtualInterfaceType": "private", "ConnectionId": "dxcon-1", "Vlan": "101", "Asn": "65001",
			"AmazonAddress": "169.254.0.1/30", "CustomerAddress": "169.254.0.2/30", "Region": "us-west-2",
		},
		// v2 models Vlan and Asn as plain integers: absent reads "0" (v1 printed "").
		{
			"VirtualInterfaceId": "", "VirtualInterfaceName": "bare-vif", "VirtualInterfaceState": "",
			"VirtualInterfaceType": "", "ConnectionId": "", "Vlan": "0", "Asn": "0",
			"AmazonAddress": "", "CustomerAddress": "", "Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ResourceType != "virtual-interface" || results[0].ResourceName != "demo-vif" {
		t.Errorf("result 0 = %+v", results[0])
	}
}

func TestDescribeGatewaysPaginatesInFixedRegion(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		if op != "DescribeDirectConnectGateways" {
			return apiError("UnknownOperationException", op)
		}
		if body["nextToken"] == "t2" {
			return http.StatusOK, map[string]interface{}{"directConnectGateways": []map[string]interface{}{
				{"directConnectGatewayName": "bare-gw"},
			}}
		}
		return http.StatusOK, map[string]interface{}{"directConnectGateways": []map[string]interface{}{
			{
				"directConnectGatewayId": "dgw-1", "directConnectGatewayName": "demo-gw",
				"directConnectGatewayState": "available", "amazonSideAsn": 64512, "ownerAccount": "111111111111",
			},
		}, "nextToken": "t2"}
	}}
	// The scope is deliberately ignored: gateways use one fixed regional endpoint.
	ctx := newEnv(t, f, "eu-west-1", "ap-south-1")
	results, err := run(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"DirectConnectGatewayId": "dgw-1", "DirectConnectGatewayName": "demo-gw",
			"DirectConnectGatewayState": "available", "AmazonSideAsn": "64512", "OwnerAccount": "111111111111",
		},
		{
			"DirectConnectGatewayId": "", "DirectConnectGatewayName": "bare-gw",
			"DirectConnectGatewayState": "", "AmazonSideAsn": "", "OwnerAccount": "",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if got := f.regionsOf("DescribeDirectConnectGateways"); !reflect.DeepEqual(got, []string{types.Regions[0], types.Regions[0]}) {
		t.Errorf("SigV4 regions = %v, want two calls in %s", got, types.Regions[0])
	}
}

func TestInvokeErrorText(t *testing.T) {
	for i, op := range []string{"DescribeConnections", "DescribeVirtualInterfaces", "DescribeDirectConnectGateways"} {
		f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
			return apiError("AccessDeniedException", "not authorized for "+op)
		}}
		ctx := newEnv(t, f, "us-west-2")
		results, err := run(ctx, i)
		if err == nil {
			t.Fatalf("%s: expected an error", op)
		}
		want := "AccessDeniedException: not authorized for " + op
		if err.Error() != want {
			t.Errorf("%s: error = %q, want %q", op, err.Error(), want)
		}
		if len(results) != 1 || results[0].Error == nil || results[0].Error.Error() != want {
			t.Errorf("%s: Process error result = %+v", op, results)
		}
	}
}

func TestRegionScopeVisitsEachRegion(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		switch op {
		case "DescribeConnections":
			return http.StatusOK, map[string]interface{}{"connections": []map[string]string{{"connectionName": "c-" + region}}}
		case "DescribeVirtualInterfaces":
			return http.StatusOK, map[string]interface{}{"virtualInterfaces": []map[string]string{{"virtualInterfaceName": "v-" + region}}}
		}
		return apiError("UnknownOperationException", op)
	}}
	scope := []string{"us-east-1", "eu-west-1", "ap-south-1"}
	ctx := newEnv(t, f, scope...)
	for i, op := range []string{"DescribeConnections", "DescribeVirtualInterfaces"} {
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
			t.Errorf("%s result regions = %v, want %v", op, regions, scope)
		}
	}
}

// A region that fails is skipped; the call only fails when nothing was found.
func TestFailingRegionIsSkipped(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		if region == "eu-west-1" {
			return apiError("AccessDeniedException", "denied")
		}
		return http.StatusOK, map[string]interface{}{"connections": []map[string]string{{"connectionName": "c-" + region}}}
	}}
	ctx := newEnv(t, f, "us-east-1", "eu-west-1", "ap-south-1")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(results) != 2 || results[0].ResourceName != "c-us-east-1" || results[1].ResourceName != "c-ap-south-1" {
		t.Errorf("results = %+v", results)
	}
}
