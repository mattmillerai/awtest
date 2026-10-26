package vpc

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/ec2/aws4_request`)

// Empty fields are omitted from the response (sparse resources).
type vpcSpec struct {
	id, cidr, state string
	isDefault       bool
}

type subnetSpec struct{ id, vpc, cidr, az string }

type sgSpec struct {
	id, name, vpc, desc string
	in, out             int
}

type regionData struct {
	vpcs    []vpcSpec
	subnets []subnetSpec
	sgs     []sgSpec
}

type recorded struct {
	region string
	action string
	form   map[string]string
}

// fakeVPC speaks the EC2 query protocol for DescribeVpcs, DescribeSubnets and
// DescribeSecurityGroups. Data is keyed by the SigV4 credential-scope region.
type fakeVPC struct {
	data map[string]regionData
	// pageSize > 0 paginates every operation with tokens "tok-<n>" (n = index
	// of the next item).
	pageSize int
	// fail maps "region/Action" -> request ordinal (0-based, per region and
	// action) of the call that fails with AccessDenied.
	fail map[string]int

	mu    sync.Mutex
	reqs  []recorded
	count map[string]int
}

func (f *fakeVPC) denied(w http.ResponseWriter, region, action string) {
	w.Header().Set("Content-Type", "text/xml;charset=UTF-8")
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprintf(w, `<Response><Errors><Error><Code>AccessDenied</Code><Message>%s</Message></Error></Errors><RequestID>req-1</RequestID></Response>`,
		html.EscapeString("not authorized for "+action+" in "+region))
}

// page returns the [start, end) window of n items and the next token.
func (f *fakeVPC) page(form map[string]string, n int) (int, int, string) {
	start := 0
	if tok := strings.TrimPrefix(form["NextToken"], "tok-"); form["NextToken"] != "" {
		start, _ = strconv.Atoi(tok)
	}
	end := n
	next := ""
	if f.pageSize > 0 && start+f.pageSize < n {
		end = start + f.pageSize
		next = "<nextToken>tok-" + strconv.Itoa(end) + "</nextToken>"
	}
	return start, end, next
}

func tag(name, v string) string {
	if v == "" {
		return ""
	}
	return "<" + name + ">" + html.EscapeString(v) + "</" + name + ">"
}

func items(n int) string {
	return strings.Repeat("<item><ipProtocol>tcp</ipProtocol></item>", n)
}

func (f *fakeVPC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	_ = r.ParseForm()
	form := map[string]string{}
	for k, v := range r.PostForm {
		form[k] = v[0]
	}
	action := form["Action"]
	key := region + "/" + action
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, action, form})
	if f.count == nil {
		f.count = map[string]int{}
	}
	ordinal := f.count[key]
	f.count[key]++
	f.mu.Unlock()

	if at, ok := f.fail[key]; ok && at == ordinal {
		f.denied(w, region, action)
		return
	}
	w.Header().Set("Content-Type", "text/xml;charset=UTF-8")
	const ns = ` xmlns="http://ec2.amazonaws.com/doc/2016-11-15/"`
	d := f.data[region]
	var b strings.Builder
	switch action {
	case "DescribeVpcs":
		s, e, next := f.page(form, len(d.vpcs))
		b.WriteString(`<DescribeVpcsResponse` + ns + `><requestId>r1</requestId><vpcSet>`)
		for _, v := range d.vpcs[s:e] {
			b.WriteString("<item>" + tag("vpcId", v.id) + tag("cidrBlock", v.cidr) + tag("state", v.state))
			if v.isDefault {
				b.WriteString("<isDefault>true</isDefault>")
			}
			b.WriteString("</item>")
		}
		b.WriteString("</vpcSet>" + next + "</DescribeVpcsResponse>")
	case "DescribeSubnets":
		s, e, next := f.page(form, len(d.subnets))
		b.WriteString(`<DescribeSubnetsResponse` + ns + `><requestId>r1</requestId><subnetSet>`)
		for _, v := range d.subnets[s:e] {
			b.WriteString("<item>" + tag("subnetId", v.id) + tag("vpcId", v.vpc) + tag("cidrBlock", v.cidr) + tag("availabilityZone", v.az) + "</item>")
		}
		b.WriteString("</subnetSet>" + next + "</DescribeSubnetsResponse>")
	case "DescribeSecurityGroups":
		s, e, next := f.page(form, len(d.sgs))
		b.WriteString(`<DescribeSecurityGroupsResponse` + ns + `><requestId>r1</requestId><securityGroupInfo>`)
		for _, v := range d.sgs[s:e] {
			b.WriteString("<item>" + tag("groupId", v.id) + tag("groupName", v.name) + tag("vpcId", v.vpc) + tag("groupDescription", v.desc))
			if v.in > 0 {
				b.WriteString("<ipPermissions>" + items(v.in) + "</ipPermissions>")
			}
			if v.out > 0 {
				b.WriteString("<ipPermissionsEgress>" + items(v.out) + "</ipPermissionsEgress>")
			}
			b.WriteString("</item>")
		}
		b.WriteString("</securityGroupInfo>" + next + "</DescribeSecurityGroupsResponse>")
	default:
		f.denied(w, region, action)
		return
	}
	_, _ = w.Write([]byte(b.String()))
}

func (f *fakeVPC) actions(action string) []recorded {
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

func (f *fakeVPC) regionsOf(action string) []string {
	var out []string
	for _, r := range f.actions(action) {
		out = append(out, r.region)
	}
	return out
}

func newEnv(t *testing.T, f *fakeVPC) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_EC2", unreachable)

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

func run(ctx context.Context) ([]types.ScanResult, error) {
	svc := VpcCalls[0]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

func resourceNames(results []types.ScanResult) []string {
	var out []string
	for _, r := range results {
		if r.Error == nil {
			out = append(out, r.ResourceType+":"+r.ResourceName)
		}
	}
	return out
}

func TestServicesAreV2Only(t *testing.T) {
	if len(VpcCalls) != 1 || VpcCalls[0].Name != "ec2:DescribeVpcs" {
		t.Fatalf("VpcCalls = %+v", VpcCalls)
	}
	if VpcCalls[0].CallV2 == nil || VpcCalls[0].Call != nil {
		t.Errorf("want CallV2 set and Call nil")
	}
}

func TestDetailsPopulatedAndSparse(t *testing.T) {
	f := &fakeVPC{data: map[string]regionData{"us-west-2": {
		vpcs: []vpcSpec{
			{id: "vpc-1", cidr: "10.0.0.0/16", state: "available", isDefault: true},
			{id: "vpc-2"},
		},
		subnets: []subnetSpec{
			{id: "subnet-1", vpc: "vpc-1", cidr: "10.0.1.0/24", az: "us-west-2a"},
			{id: "subnet-2"},
		},
		sgs: []sgSpec{
			{id: "sg-1", name: "web", vpc: "vpc-1", desc: "Web tier", in: 2, out: 1},
			{id: "sg-2"},
		},
	}}}
	results, err := run(scoped(newEnv(t, f), "us-west-2"))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 6 {
		t.Fatalf("results = %+v", results)
	}
	type want struct {
		method, rtype, name string
		details             map[string]interface{}
	}
	wants := []want{
		{"ec2:DescribeVpcs", "vpc", "vpc-1", map[string]interface{}{
			"VpcId": "vpc-1", "CidrBlock": "10.0.0.0/16", "State": "available", "IsDefault": true, "SubnetCount": 1, "SecurityGroupCount": 1}},
		{"ec2:DescribeVpcs", "vpc", "vpc-2", map[string]interface{}{
			"VpcId": "vpc-2", "CidrBlock": "", "State": "", "IsDefault": false, "SubnetCount": 0, "SecurityGroupCount": 0}},
		{"ec2:DescribeSubnets", "subnet", "subnet-1", map[string]interface{}{
			"VpcId": "vpc-1", "CidrBlock": "10.0.1.0/24", "AvailabilityZone": "us-west-2a"}},
		{"ec2:DescribeSubnets", "subnet", "subnet-2", map[string]interface{}{
			"VpcId": "", "CidrBlock": "", "AvailabilityZone": ""}},
		{"ec2:DescribeSecurityGroups", "security-group", "sg-1", map[string]interface{}{
			"GroupName": "web", "VpcId": "vpc-1", "Description": "Web tier", "InboundRules": 2, "OutboundRules": 1}},
		{"ec2:DescribeSecurityGroups", "security-group", "sg-2", map[string]interface{}{
			"GroupName": "", "VpcId": "", "Description": "", "InboundRules": 0, "OutboundRules": 0}},
	}
	for i, w := range wants {
		r := results[i]
		if r.ServiceName != "VPC" || r.MethodName != w.method || r.ResourceType != w.rtype || r.ResourceName != w.name {
			t.Errorf("result %d = %+v, want %+v", i, r, w)
		}
		if !reflect.DeepEqual(r.Details, w.details) {
			t.Errorf("result %d details = %#v\nwant %#v", i, r.Details, w.details)
		}
	}
}

func TestPaginationFollowsNextToken(t *testing.T) {
	d := regionData{}
	for i := 0; i < 5; i++ {
		d.vpcs = append(d.vpcs, vpcSpec{id: fmt.Sprintf("vpc-%d", i)})
		d.subnets = append(d.subnets, subnetSpec{id: fmt.Sprintf("subnet-%d", i)})
		d.sgs = append(d.sgs, sgSpec{id: fmt.Sprintf("sg-%d", i)})
	}
	f := &fakeVPC{data: map[string]regionData{"us-west-2": d}, pageSize: 2}
	results, err := run(scoped(newEnv(t, f), "us-west-2"))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 15 {
		t.Fatalf("got %d results: %v", len(results), resourceNames(results))
	}
	for _, a := range []string{"DescribeVpcs", "DescribeSubnets", "DescribeSecurityGroups"} {
		reqs := f.actions(a)
		if len(reqs) != 3 {
			t.Fatalf("%s calls = %d, want 3 pages", a, len(reqs))
		}
		for i, wantTok := range []string{"", "tok-2", "tok-4"} {
			if reqs[i].form["NextToken"] != wantTok {
				t.Errorf("%s page %d NextToken = %q, want %q", a, i, reqs[i].form["NextToken"], wantTok)
			}
		}
	}
}

func TestSubnetAndSecurityGroupErrorsArePartial(t *testing.T) {
	f := &fakeVPC{
		data: map[string]regionData{"us-west-2": {
			vpcs:    []vpcSpec{{id: "vpc-1"}},
			subnets: []subnetSpec{{id: "subnet-1"}},
			sgs:     []sgSpec{{id: "sg-1"}},
		}},
		fail: map[string]int{"us-west-2/DescribeSubnets": 0, "us-west-2/DescribeSecurityGroups": 0},
	}
	results, err := run(scoped(newEnv(t, f), "us-west-2"))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("results = %+v", results)
	}
	wantErrs := []string{
		"ec2:DescribeSubnets in us-west-2: AccessDenied: not authorized for DescribeSubnets in us-west-2",
		"ec2:DescribeSecurityGroups in us-west-2: AccessDenied: not authorized for DescribeSecurityGroups in us-west-2",
	}
	for i, w := range wantErrs {
		if results[i].Error == nil || results[i].Error.Error() != w || results[i].ServiceName != "VPC" || results[i].MethodName != "ec2:DescribeVpcs" {
			t.Errorf("partial %d = %+v, want error %q", i, results[i], w)
		}
	}
	if results[2].ResourceType != "vpc" || results[2].Details["SubnetCount"] != 0 {
		t.Errorf("vpc result = %+v", results[2])
	}
}

func TestPaginationErrorKeepsEarlierPages(t *testing.T) {
	// v1 broke out of the page loop on error but kept the pages already read.
	d := regionData{}
	for i := 0; i < 4; i++ {
		d.vpcs = append(d.vpcs, vpcSpec{id: fmt.Sprintf("vpc-%d", i)})
		d.subnets = append(d.subnets, subnetSpec{id: fmt.Sprintf("subnet-%d", i)})
	}
	f := &fakeVPC{
		data:     map[string]regionData{"us-west-2": d},
		pageSize: 2,
		fail:     map[string]int{"us-west-2/DescribeSubnets": 1},
	}
	results, err := run(scoped(newEnv(t, f), "us-west-2"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"vpc:vpc-0", "vpc:vpc-1", "vpc:vpc-2", "vpc:vpc-3", "subnet:subnet-0", "subnet:subnet-1"}
	if got := resourceNames(results); !reflect.DeepEqual(got, want) {
		t.Errorf("resources = %v, want %v", got, want)
	}
	if results[0].Error == nil || !strings.HasPrefix(results[0].Error.Error(), "ec2:DescribeSubnets in us-west-2: AccessDenied: ") {
		t.Errorf("first result = %+v, want the partial subnet error", results[0])
	}
	if n := len(f.actions("DescribeSubnets")); n != 2 {
		t.Errorf("DescribeSubnets calls = %d, want 2 (stop at the failing page)", n)
	}
}

func TestFailedVpcRegionIsSkipped(t *testing.T) {
	// A region whose DescribeVpcs fails is skipped entirely (no subnet/SG
	// calls); other regions still report.
	f := &fakeVPC{
		data: map[string]regionData{
			"us-east-1": {vpcs: []vpcSpec{{id: "vpc-east"}}, subnets: []subnetSpec{{id: "subnet-east"}}},
			"eu-west-1": {vpcs: []vpcSpec{{id: "vpc-eu"}}, subnets: []subnetSpec{{id: "subnet-eu"}}},
		},
		fail: map[string]int{"us-east-1/DescribeVpcs": 0},
	}
	results, err := run(scoped(newEnv(t, f), "us-east-1", "eu-west-1"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := resourceNames(results), []string{"vpc:vpc-eu", "subnet:subnet-eu"}; !reflect.DeepEqual(got, want) {
		t.Errorf("resources = %v, want %v", got, want)
	}
	if got := f.regionsOf("DescribeSubnets"); !reflect.DeepEqual(got, []string{"eu-west-1"}) {
		t.Errorf("DescribeSubnets regions = %v, want only eu-west-1", got)
	}
}

func TestAllRegionsFailingReturnsLastError(t *testing.T) {
	f := &fakeVPC{fail: map[string]int{"us-east-1/DescribeVpcs": 0, "eu-west-1/DescribeVpcs": 0}}
	ctx := scoped(newEnv(t, f), "us-east-1", "eu-west-1")
	svc := VpcCalls[0]
	out, err := svc.Invoke(ctx, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	const want = "AccessDenied: not authorized for DescribeVpcs in eu-west-1"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if code := awsv2.ErrorCode(err); code != "AccessDenied" {
		t.Errorf("ErrorCode = %q", code)
	}
	for _, noise := range []string{"status code", "request id", "operation error", "403"} {
		if strings.Contains(strings.ToLower(err.Error()), noise) {
			t.Errorf("error %q contains transport noise %q", err, noise)
		}
	}
	results := svc.Process(out, err, false)
	if len(results) != 1 || results[0].Error == nil || results[0].ServiceName != "VPC" ||
		results[0].MethodName != "ec2:DescribeVpcs" || results[0].ResourceName != "" {
		t.Fatalf("results = %+v", results)
	}
}

func TestAllScopedRegionsVisitedWithRightSigV4Region(t *testing.T) {
	f := &fakeVPC{data: map[string]regionData{
		"us-east-1": {vpcs: []vpcSpec{{id: "vpc-east"}}},
		"eu-west-1": {vpcs: []vpcSpec{{id: "vpc-eu"}}},
		"us-west-2": {vpcs: []vpcSpec{{id: "vpc-west"}}},
	}}
	ctx := newEnv(t, f)
	regions := types.RegionsFor(ctx)
	if len(regions) < 3 {
		t.Fatalf("default regions = %v", regions)
	}
	results, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range []string{"DescribeVpcs", "DescribeSubnets", "DescribeSecurityGroups"} {
		if got := f.regionsOf(a); !reflect.DeepEqual(got, regions) {
			t.Errorf("%s regions = %v, want %v", a, got, regions)
		}
	}
	var want []string
	for _, rg := range regions {
		for _, v := range f.data[rg].vpcs {
			want = append(want, "vpc:"+v.id)
		}
	}
	if got := resourceNames(results); !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestRegionScope(t *testing.T) {
	f := &fakeVPC{data: map[string]regionData{
		"us-east-1": {vpcs: []vpcSpec{{id: "vpc-east"}}},
		"eu-west-1": {vpcs: []vpcSpec{{id: "vpc-eu"}}},
	}}
	ctx, scope := types.WithRegionScope(newEnv(t, f), []string{"eu-west-1"})
	results, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !scope.Used() {
		t.Error("service did not consult the region scope")
	}
	for _, r := range f.reqs {
		if r.region != "eu-west-1" {
			t.Errorf("request %s signed for %s, want only eu-west-1", r.action, r.region)
		}
	}
	if got := resourceNames(results); !reflect.DeepEqual(got, []string{"vpc:vpc-eu"}) {
		t.Errorf("results = %v", got)
	}
}

func TestNoSharedStateMutation(t *testing.T) {
	f := &fakeVPC{data: map[string]regionData{
		"us-east-1": {vpcs: []vpcSpec{{id: "vpc-east"}}},
		"eu-west-1": {vpcs: []vpcSpec{{id: "vpc-eu"}}},
		"us-west-2": {vpcs: []vpcSpec{{id: "vpc-west"}}},
	}}
	base := newEnv(t, f)
	cfgBefore, ok := types.AWSConfigFrom(base)
	if !ok {
		t.Fatal("no aws.Config in context")
	}

	var wg sync.WaitGroup
	for _, region := range []string{"us-east-1", "eu-west-1", "us-west-2", "eu-west-1"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results, err := run(scoped(base, region))
			if err != nil || len(results) != 1 {
				t.Errorf("%s: results = %+v, err = %v", region, results, err)
			}
		}()
	}
	wg.Wait()

	cfgAfter, _ := types.AWSConfigFrom(base)
	if cfgAfter.Region != cfgBefore.Region || cfgBefore.Region != "us-west-2" {
		t.Errorf("config region changed: %q -> %q", cfgBefore.Region, cfgAfter.Region)
	}
	for _, r := range f.actions("DescribeVpcs") {
		if r.region == "" {
			t.Errorf("request without SigV4 region: %+v", r)
		}
	}
}

func TestNothingFoundPrintsNoResults(t *testing.T) {
	results, err := run(newEnv(t, &fakeVPC{}))
	if err != nil || len(results) != 0 {
		t.Errorf("results = %+v, err = %v", results, err)
	}
}

func TestAmbientEndpointIgnored(t *testing.T) {
	// newEnv sets AWS_ENDPOINT_URL_EC2 to an unreachable host; success proves
	// the explicit endpoint won.
	f := &fakeVPC{data: map[string]regionData{"us-west-2": {vpcs: []vpcSpec{{id: "vpc-1"}}}}}
	results, err := run(scoped(newEnv(t, f), "us-west-2"))
	if err != nil || len(results) != 1 {
		t.Errorf("results = %+v, err = %v", results, err)
	}
}

func TestProcessIgnoresUnexpectedOutput(t *testing.T) {
	for _, out := range []interface{}{nil, "x", 3, []string{"nope"}} {
		if got := VpcCalls[0].Process(out, nil, false); len(got) != 0 {
			t.Errorf("results = %+v", got)
		}
	}
}
