package neptune

import (
	"context"
	"fmt"
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
)

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/[^/]+/aws4_request`)

type recorded struct {
	region string
	action string
	form   url.Values
}

// fakeSvc speaks the Neptune (RDS query, XML) protocol. handle receives the
// SigV4 credential-scope region, the action and the form, and returns the
// HTTP status and the XML body.
type fakeSvc struct {
	handle func(region, action string, form url.Values) (int, string)

	mu   sync.Mutex
	reqs []recorded
}

func apiError(code, msg string) (int, string) {
	return http.StatusForbidden, fmt.Sprintf(`<ErrorResponse xmlns="http://rds.amazonaws.com/doc/2014-10-31/"><Error><Type>Sender</Type><Code>%s</Code><Message>%s</Message></Error><RequestId>req-1</RequestId></ErrorResponse>`, code, msg)
}

// result wraps inner in the query-protocol response envelope for action.
func result(action, inner string) (int, string) {
	return http.StatusOK, fmt.Sprintf(`<%[1]sResponse xmlns="http://rds.amazonaws.com/doc/2014-10-31/"><%[1]sResult>%[2]s</%[1]sResult><ResponseMetadata><RequestId>req-1</RequestId></ResponseMetadata></%[1]sResponse>`, action, inner)
}

func (f *fakeSvc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	_ = r.ParseForm()
	action := r.PostForm.Get("Action")
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, action, r.PostForm})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "text/xml")
	var status int
	var out string
	if r.Method != http.MethodPost || r.PostForm.Get("Version") != "2014-10-31" {
		status, out = apiError("InvalidAction", "bad request "+r.Method)
	} else {
		status, out = f.handle(region, action, r.PostForm)
	}
	w.WriteHeader(status)
	_, _ = w.Write([]byte(out))
}

func (f *fakeSvc) ops(action string) []recorded {
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

func (f *fakeSvc) regionsOf(action string) []string {
	var out []string
	for _, r := range f.ops(action) {
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
	t.Setenv("AWS_ENDPOINT_URL_NEPTUNE", unreachable)

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
	svc := NeptuneCalls[i]
	out, err := svc.Invoke(ctx)
	return svc.Process(out, err, false), err
}

func detailsOf(results []types.ScanResult) []map[string]interface{} {
	var out []map[string]interface{}
	for _, r := range results {
		out = append(out, r.Details)
	}
	return out
}

// pagedHandler serves a two-page list for action: full on the first page
// (with Marker "m2") and sparse on the second.
func pagedHandler(action, list, item, full, sparse string) func(region, a string, form url.Values) (int, string) {
	return func(region, a string, form url.Values) (int, string) {
		if a != action {
			return apiError("InvalidAction", a)
		}
		if form.Get("Marker") == "m2" {
			return result(action, fmt.Sprintf("<%[1]s><%[2]s>%[3]s</%[2]s></%[1]s>", list, item, sparse))
		}
		return result(action, fmt.Sprintf("<Marker>m2</Marker><%[1]s><%[2]s>%[3]s</%[2]s></%[1]s>", list, item, full))
	}
}

func TestDescribeDBClustersInvoke(t *testing.T) {
	f := &fakeSvc{handle: pagedHandler("DescribeDBClusters", "DBClusters", "DBCluster",
		`<DBClusterIdentifier>c1</DBClusterIdentifier><DBClusterArn>arn:c1</DBClusterArn><Status>available</Status>
<Engine>neptune</Engine><EngineVersion>1.3.0.0</EngineVersion><Endpoint>c1.cluster.neptune.amazonaws.com</Endpoint>
<ReaderEndpoint>c1.cluster-ro.neptune.amazonaws.com</ReaderEndpoint><Port>8182</Port><MultiAZ>true</MultiAZ>
<StorageEncrypted>true</StorageEncrypted><KmsKeyId>kms-key</KmsKeyId><DeletionProtection>false</DeletionProtection>
<IAMDatabaseAuthenticationEnabled>true</IAMDatabaseAuthenticationEnabled>
<DBClusterParameterGroup>default.neptune1.3</DBClusterParameterGroup><ClusterCreateTime>2023-11-14T22:13:20.000Z</ClusterCreateTime>`,
		`<DBClusterIdentifier>bare-cluster</DBClusterIdentifier>`,
	)}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"DBClusterIdentifier": "c1", "DBClusterArn": "arn:c1", "Status": "available", "Engine": "neptune",
			"EngineVersion": "1.3.0.0", "Endpoint": "c1.cluster.neptune.amazonaws.com",
			"ReaderEndpoint": "c1.cluster-ro.neptune.amazonaws.com", "Port": "8182", "MultiAZ": "true",
			"StorageEncrypted": "true", "KmsKeyId": "kms-key", "DeletionProtection": "false",
			"IAMDatabaseAuthenticationEnabled": "true", "DBClusterParameterGroup": "default.neptune1.3",
			"ClusterCreateTime": "2023-11-14T22:13:20Z", "Region": "us-west-2",
		},
		{
			"DBClusterIdentifier": "bare-cluster", "DBClusterArn": "", "Status": "", "Engine": "", "EngineVersion": "",
			"Endpoint": "", "ReaderEndpoint": "", "Port": "", "MultiAZ": "", "StorageEncrypted": "", "KmsKeyId": "",
			"DeletionProtection": "", "IAMDatabaseAuthenticationEnabled": "", "DBClusterParameterGroup": "",
			"ClusterCreateTime": "", "Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ServiceName != "Neptune" || results[0].MethodName != "neptune:DescribeDBClusters" || results[0].ResourceType != "db-cluster" || results[0].ResourceName != "c1" {
		t.Errorf("result 0 = %+v", results[0])
	}
	if got := len(f.ops("DescribeDBClusters")); got != 2 {
		t.Errorf("DescribeDBClusters calls = %d, want 2 (Marker must be followed)", got)
	}
}

func TestDescribeDBInstancesInvoke(t *testing.T) {
	f := &fakeSvc{handle: pagedHandler("DescribeDBInstances", "DBInstances", "DBInstance",
		`<DBInstanceIdentifier>i1</DBInstanceIdentifier><DBInstanceArn>arn:i1</DBInstanceArn><DBInstanceClass>db.r5.large</DBInstanceClass>
<Engine>neptune</Engine><EngineVersion>1.3.0.0</EngineVersion><DBInstanceStatus>available</DBInstanceStatus>
<Endpoint><Address>i1.neptune.amazonaws.com</Address><Port>8182</Port></Endpoint><DBClusterIdentifier>c1</DBClusterIdentifier>
<AvailabilityZone>us-west-2a</AvailabilityZone><PubliclyAccessible>false</PubliclyAccessible>
<StorageEncrypted>true</StorageEncrypted><AutoMinorVersionUpgrade>true</AutoMinorVersionUpgrade>`,
		// An endpoint without address and port.
		`<DBInstanceIdentifier>bare-instance</DBInstanceIdentifier><Endpoint></Endpoint>`,
	)}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"DBInstanceIdentifier": "i1", "DBInstanceArn": "arn:i1", "DBInstanceClass": "db.r5.large", "Engine": "neptune",
			"EngineVersion": "1.3.0.0", "DBInstanceStatus": "available", "EndpointAddress": "i1.neptune.amazonaws.com",
			"EndpointPort": "8182", "DBClusterIdentifier": "c1", "AvailabilityZone": "us-west-2a",
			"PubliclyAccessible": "false", "StorageEncrypted": "true", "AutoMinorVersionUpgrade": "true",
			"Region": "us-west-2",
		},
		{
			"DBInstanceIdentifier": "bare-instance", "DBInstanceArn": "", "DBInstanceClass": "", "Engine": "",
			"EngineVersion": "", "DBInstanceStatus": "", "EndpointAddress": "", "EndpointPort": "",
			"DBClusterIdentifier": "", "AvailabilityZone": "", "PubliclyAccessible": "", "StorageEncrypted": "",
			"AutoMinorVersionUpgrade": "", "Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ResourceType != "db-instance" || results[0].ResourceName != "i1" {
		t.Errorf("result 0 = %+v", results[0])
	}
	if got := len(f.ops("DescribeDBInstances")); got != 2 {
		t.Errorf("DescribeDBInstances calls = %d, want 2", got)
	}
}

func TestDescribeDBClusterParameterGroupsInvoke(t *testing.T) {
	f := &fakeSvc{handle: pagedHandler("DescribeDBClusterParameterGroups", "DBClusterParameterGroups", "DBClusterParameterGroup",
		`<DBClusterParameterGroupName>pg1</DBClusterParameterGroupName><DBClusterParameterGroupArn>arn:pg1</DBClusterParameterGroupArn>
<Description>my group</Description><DBParameterGroupFamily>neptune1.3</DBParameterGroupFamily>`,
		`<DBClusterParameterGroupName>bare-pg</DBClusterParameterGroupName>`,
	)}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"DBClusterParameterGroupName": "pg1", "DBClusterParameterGroupArn": "arn:pg1",
			"Description": "my group", "DBParameterGroupFamily": "neptune1.3", "Region": "us-west-2",
		},
		{
			"DBClusterParameterGroupName": "bare-pg", "DBClusterParameterGroupArn": "", "Description": "",
			"DBParameterGroupFamily": "", "Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	if results[0].ResourceType != "db-cluster-parameter-group" || results[0].ResourceName != "pg1" {
		t.Errorf("result 0 = %+v", results[0])
	}
	if got := len(f.ops("DescribeDBClusterParameterGroups")); got != 2 {
		t.Errorf("DescribeDBClusterParameterGroups calls = %d, want 2", got)
	}
}

func TestInvokeErrorText(t *testing.T) {
	f := &fakeSvc{handle: func(region, action string, form url.Values) (int, string) {
		return apiError("AccessDenied", "not authorized for "+action)
	}}
	actions := []string{"DescribeDBClusters", "DescribeDBInstances", "DescribeDBClusterParameterGroups"}
	for i := range NeptuneCalls {
		ctx := newEnv(t, f, "us-west-2")
		results, err := run(ctx, i)
		if err == nil {
			t.Fatalf("%s: expected an error", NeptuneCalls[i].Name)
		}
		want := "AccessDenied: not authorized for " + actions[i]
		if err.Error() != want {
			t.Errorf("%s: error = %q, want %q", NeptuneCalls[i].Name, err.Error(), want)
		}
		if len(results) != 1 || results[0].Error == nil || results[0].Error.Error() != want {
			t.Errorf("%s: Process error result = %+v", NeptuneCalls[i].Name, results)
		}
		if results[0].MethodName != NeptuneCalls[i].Name {
			t.Errorf("MethodName = %q", results[0].MethodName)
		}
	}
}

// A failing region is skipped; the call only fails when nothing was found and
// at least one region failed.
func TestFailingRegionIsSkipped(t *testing.T) {
	f := &fakeSvc{handle: func(region, action string, form url.Values) (int, string) {
		if region == "eu-west-1" {
			return apiError("AccessDenied", "denied in "+region)
		}
		return result(action, `<DBClusters><DBCluster><DBClusterIdentifier>c-`+region+`</DBClusterIdentifier></DBCluster></DBClusters>`)
	}}
	ctx := newEnv(t, f, "us-east-1", "eu-west-1", "ap-south-1")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range results {
		names = append(names, r.ResourceName)
	}
	if want := []string{"c-us-east-1", "c-ap-south-1"}; !reflect.DeepEqual(names, want) {
		t.Errorf("clusters = %v, want %v", names, want)
	}
}

func TestRegionScopeVisitsEachRegion(t *testing.T) {
	f := &fakeSvc{handle: func(region, action string, form url.Values) (int, string) {
		switch action {
		case "DescribeDBClusters":
			return result(action, `<DBClusters><DBCluster><DBClusterIdentifier>c-`+region+`</DBClusterIdentifier></DBCluster></DBClusters>`)
		case "DescribeDBInstances":
			return result(action, `<DBInstances><DBInstance><DBInstanceIdentifier>i-`+region+`</DBInstanceIdentifier></DBInstance></DBInstances>`)
		case "DescribeDBClusterParameterGroups":
			return result(action, `<DBClusterParameterGroups><DBClusterParameterGroup><DBClusterParameterGroupName>pg-`+region+`</DBClusterParameterGroupName></DBClusterParameterGroup></DBClusterParameterGroups>`)
		}
		return apiError("InvalidAction", action)
	}}
	scope := []string{"us-east-1", "eu-west-1", "ap-south-1"}
	for i, action := range []string{"DescribeDBClusters", "DescribeDBInstances", "DescribeDBClusterParameterGroups"} {
		ctx := newEnv(t, f, scope...)
		f.mu.Lock()
		f.reqs = nil
		f.mu.Unlock()
		results, err := run(ctx, i)
		if err != nil {
			t.Fatal(err)
		}
		if got := f.regionsOf(action); !reflect.DeepEqual(got, scope) {
			t.Errorf("%s SigV4 regions = %v, want %v", action, got, scope)
		}
		var regions []string
		for _, d := range detailsOf(results) {
			regions = append(regions, d["Region"].(string))
		}
		if !reflect.DeepEqual(regions, scope) {
			t.Errorf("%s result regions = %v, want %v", NeptuneCalls[i].Name, regions, scope)
		}
		if !strings.HasPrefix(results[0].ResourceName, []string{"c-", "i-", "pg-"}[i]) {
			t.Errorf("%s: result 0 = %+v", NeptuneCalls[i].Name, results[0])
		}
	}
}
