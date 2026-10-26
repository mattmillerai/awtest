package ec2

import (
	"context"
	"encoding/base64"
	"fmt"
	"html"
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

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/ec2/aws4_request`)

// inst is an instance served by the fake. Empty fields are omitted from the
// response (a sparse instance).
type inst struct {
	id, state, typ, ip, eip string
	tags                    [][2]string
	userData                string // plain text; the fake base64-encodes it
	userDataErr             bool   // DescribeInstanceAttribute fails
	addressErr              bool   // DescribeAddresses fails
	badBase64               bool   // DescribeInstanceAttribute returns undecodable data
}

type recorded struct {
	region string
	action string
	form   map[string]string
}

// fakeEC2 speaks the EC2 query protocol (form request, XML response) for the
// operations awtest uses. Data is keyed by the SigV4 credential-scope region.
type fakeEC2 struct {
	instances map[string][]inst
	// deny maps region -> message of an UnauthorizedOperation returned by
	// DescribeInstances.
	deny map[string]string
	// moreToken makes DescribeInstances return a nextToken (v1 ignored it).
	moreToken bool

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeEC2) fail(w http.ResponseWriter, code, msg string) {
	w.Header().Set("Content-Type", "text/xml;charset=UTF-8")
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprintf(w, `<Response><Errors><Error><Code>%s</Code><Message>%s</Message></Error></Errors><RequestID>req-1</RequestID></Response>`, code, html.EscapeString(msg))
}

func (f *fakeEC2) find(region, id string) *inst {
	for i := range f.instances[region] {
		if f.instances[region][i].id == id {
			return &f.instances[region][i]
		}
	}
	return nil
}

func (f *fakeEC2) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, action, form})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "text/xml;charset=UTF-8")
	const ns = ` xmlns="http://ec2.amazonaws.com/doc/2016-11-15/"`
	switch action {
	case "DescribeInstances":
		if msg, ok := f.deny[region]; ok {
			f.fail(w, "UnauthorizedOperation", msg)
			return
		}
		var b strings.Builder
		b.WriteString(`<DescribeInstancesResponse` + ns + `><requestId>r1</requestId><reservationSet>`)
		for _, in := range f.instances[region] {
			b.WriteString(`<item><instancesSet><item>`)
			if in.id != "" {
				fmt.Fprintf(&b, `<instanceId>%s</instanceId>`, in.id)
			}
			if in.state != "" {
				fmt.Fprintf(&b, `<instanceState><code>16</code><name>%s</name></instanceState>`, in.state)
			}
			if in.typ != "" {
				fmt.Fprintf(&b, `<instanceType>%s</instanceType>`, in.typ)
			}
			if in.ip != "" {
				fmt.Fprintf(&b, `<ipAddress>%s</ipAddress>`, in.ip)
			}
			if len(in.tags) > 0 {
				b.WriteString(`<tagSet>`)
				for _, t := range in.tags {
					fmt.Fprintf(&b, `<item><key>%s</key><value>%s</value></item>`, t[0], t[1])
				}
				b.WriteString(`</tagSet>`)
			}
			b.WriteString(`</item></instancesSet></item>`)
		}
		b.WriteString(`</reservationSet>`)
		if f.moreToken {
			b.WriteString(`<nextToken>more</nextToken>`)
		}
		b.WriteString(`</DescribeInstancesResponse>`)
		_, _ = w.Write([]byte(b.String()))
	case "DescribeInstanceAttribute":
		in := f.find(region, form["InstanceId"])
		if in == nil || in.userDataErr {
			f.fail(w, "InvalidInstanceID.NotFound", "The instance ID '"+form["InstanceId"]+"' does not exist")
			return
		}
		val := ""
		switch {
		case in.badBase64:
			val = "<value>!!!not-base64</value>"
		case in.userData != "":
			val = "<value>" + base64.StdEncoding.EncodeToString([]byte(in.userData)) + "</value>"
		}
		fmt.Fprintf(w, `<DescribeInstanceAttributeResponse`+ns+`><requestId>r1</requestId><instanceId>%s</instanceId><userData>%s</userData></DescribeInstanceAttributeResponse>`, in.id, val)
	case "DescribeAddresses":
		id := form["Filter.1.Value.1"]
		if form["Filter.1.Name"] != "instance-id" {
			f.fail(w, "InvalidParameterValue", "unexpected filter "+form["Filter.1.Name"])
			return
		}
		in := f.find(region, id)
		if in != nil && in.addressErr {
			f.fail(w, "UnauthorizedOperation", "no addresses for you")
			return
		}
		items := ""
		if in != nil && in.eip != "" {
			items = "<item><publicIp>" + in.eip + "</publicIp></item>"
		}
		fmt.Fprintf(w, `<DescribeAddressesResponse`+ns+`><requestId>r1</requestId><addressesSet>%s</addressesSet></DescribeAddressesResponse>`, items)
	default:
		f.fail(w, "InvalidAction", "unexpected action "+action)
	}
}

func (f *fakeEC2) actions(action string) []recorded {
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

func (f *fakeEC2) regionsOf(action string) []string {
	var out []string
	for _, r := range f.actions(action) {
		out = append(out, r.region)
	}
	return out
}

func newEnv(t *testing.T, f *fakeEC2) context.Context {
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
	svc := EC2Calls[0]
	out, err := svc.Invoke(ctx)
	return svc.Process(out, err, false), err
}

func TestServicesAreV2Only(t *testing.T) {
	if len(EC2Calls) != 1 || EC2Calls[0].Name != "ec2:DescribeInstances" {
		t.Fatalf("EC2Calls = %+v", EC2Calls)
	}
	if EC2Calls[0].CallV2 == nil {
		t.Errorf("want CallV2 set and Call nil")
	}
}

func TestDescribeInstancesDetails(t *testing.T) {
	f := &fakeEC2{instances: map[string][]inst{
		"us-west-2": {
			{id: "i-full", state: "running", typ: "t3.micro", ip: "203.0.113.10", eip: "198.51.100.7",
				userData: "#!/bin/sh\necho hi\n", tags: [][2]string{{"Name", "web"}, {"Env", "dev"}}},
			{id: "i-sparse"},
		},
	}}
	ctx := scoped(newEnv(t, f), "us-west-2")
	results, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %+v", results)
	}

	full := results[0]
	if full.ServiceName != "EC2" || full.MethodName != "ec2:DescribeInstances" || full.ResourceType != "instance" || full.ResourceName != "i-full" {
		t.Errorf("full = %+v", full)
	}
	wantFull := map[string]interface{}{
		"state":      "running",
		"type":       "t3.micro",
		"ip":         "203.0.113.10",
		"elastic_ip": "198.51.100.7",
		"user_data":  "#!/bin/sh\necho hi\n",
		"tags":       map[string]string{"Name": "web", "Env": "dev"},
	}
	if !reflect.DeepEqual(full.Details, wantFull) {
		t.Errorf("full details = %#v\nwant %#v", full.Details, wantFull)
	}

	sparse := results[1]
	if sparse.ResourceName != "i-sparse" {
		t.Errorf("sparse = %+v", sparse)
	}
	wantSparse := map[string]interface{}{
		"state": "", "type": "", "ip": "", "elastic_ip": "", "user_data": "",
	}
	if !reflect.DeepEqual(sparse.Details, wantSparse) {
		t.Errorf("sparse details = %#v\nwant %#v", sparse.Details, wantSparse)
	}
	if _, ok := sparse.Details["tags"]; ok {
		t.Error("sparse instance must not carry a tags key")
	}

	// The follow-up calls are keyed on the instance id.
	for _, r := range f.actions("DescribeAddresses") {
		if r.form["Filter.1.Name"] != "instance-id" || r.form["Filter.1.Value.1"] == "" {
			t.Errorf("DescribeAddresses form = %v", r.form)
		}
	}
	for _, r := range f.actions("DescribeInstanceAttribute") {
		if r.form["Attribute"] != "userData" {
			t.Errorf("DescribeInstanceAttribute form = %v", r.form)
		}
	}
}

func TestSparseResourceWithoutID(t *testing.T) {
	// An instance without an id is reported with an empty name (v1 would have
	// dereferenced nil); the follow-up calls simply fail and are swallowed.
	f := &fakeEC2{instances: map[string][]inst{"us-west-2": {{state: "pending"}}}}
	results, err := run(scoped(newEnv(t, f), "us-west-2"))
	if err != nil || len(results) != 1 {
		t.Fatalf("results = %+v, err = %v", results, err)
	}
	if results[0].ResourceName != "" || results[0].Details["state"] != "pending" {
		t.Errorf("result = %+v", results[0])
	}
}

func TestUserDataErrorTextNormalized(t *testing.T) {
	f := &fakeEC2{instances: map[string][]inst{"us-west-2": {
		{id: "i-1", userDataErr: true},
		{id: "i-2", badBase64: true},
	}}}
	results, err := run(scoped(newEnv(t, f), "us-west-2"))
	if err != nil || len(results) != 2 {
		t.Fatalf("results = %+v, err = %v", results, err)
	}
	const want = "Error retrieving user data: InvalidInstanceID.NotFound: The instance ID 'i-1' does not exist"
	if got := results[0].Details["user_data"]; got != want {
		t.Errorf("user_data = %q, want %q", got, want)
	}
	if got, _ := results[1].Details["user_data"].(string); !strings.HasPrefix(got, "Error retrieving user data: ") || strings.Contains(got, "status code") {
		t.Errorf("bad base64 user_data = %q", got)
	}
}

func TestAddressErrorIsSwallowed(t *testing.T) {
	f := &fakeEC2{instances: map[string][]inst{"us-west-2": {{id: "i-1", eip: "198.51.100.9", addressErr: true}}}}
	results, err := run(scoped(newEnv(t, f), "us-west-2"))
	if err != nil || len(results) != 1 {
		t.Fatalf("results = %+v, err = %v", results, err)
	}
	if results[0].Details["elastic_ip"] != "" {
		t.Errorf("elastic_ip = %v, want empty", results[0].Details["elastic_ip"])
	}
}

func TestNextTokenIgnored(t *testing.T) {
	// v1 made a single DescribeInstances call per region and ignored NextToken.
	f := &fakeEC2{moreToken: true, instances: map[string][]inst{"us-west-2": {{id: "i-1"}}, "eu-west-1": {{id: "i-2"}}}}
	results, err := run(scoped(newEnv(t, f), "us-west-2", "eu-west-1"))
	if err != nil || len(results) != 2 {
		t.Fatalf("results = %+v, err = %v", results, err)
	}
	if got := f.regionsOf("DescribeInstances"); !reflect.DeepEqual(got, []string{"us-west-2", "eu-west-1"}) {
		t.Errorf("DescribeInstances regions = %v, want one call per region", got)
	}
	for _, r := range f.actions("DescribeInstances") {
		if r.form["NextToken"] != "" {
			t.Errorf("unexpected NextToken in %v", r.form)
		}
	}
}

func TestAllScopedRegionsVisitedWithRightSigV4Region(t *testing.T) {
	f := &fakeEC2{instances: map[string][]inst{
		"us-east-1": {{id: "i-east"}},
		"eu-west-1": {{id: "i-eu"}},
		"us-west-2": {{id: "i-west"}},
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
	if got := f.regionsOf("DescribeInstances"); !reflect.DeepEqual(got, regions) {
		t.Errorf("DescribeInstances regions = %v, want %v", got, regions)
	}
	// Every follow-up call is signed for the region of the instance it targets.
	want := map[string]string{"i-east": "us-east-1", "i-eu": "eu-west-1", "i-west": "us-west-2"}
	for _, r := range f.actions("DescribeInstanceAttribute") {
		if want[r.form["InstanceId"]] != r.region {
			t.Errorf("%s attribute call signed for %s, want %s", r.form["InstanceId"], r.region, want[r.form["InstanceId"]])
		}
	}
	for _, r := range f.actions("DescribeAddresses") {
		if want[r.form["Filter.1.Value.1"]] != r.region {
			t.Errorf("%s address call signed for %s, want %s", r.form["Filter.1.Value.1"], r.region, want[r.form["Filter.1.Value.1"]])
		}
	}
	// Order follows the region loop.
	var names []string
	for _, res := range results {
		names = append(names, res.ResourceName)
	}
	var wantNames []string
	for _, rg := range regions {
		for _, in := range f.instances[rg] {
			wantNames = append(wantNames, in.id)
		}
	}
	if !reflect.DeepEqual(names, wantNames) {
		t.Errorf("order = %v, want %v", names, wantNames)
	}
}

func TestRegionScope(t *testing.T) {
	f := &fakeEC2{instances: map[string][]inst{"us-east-1": {{id: "i-east"}}, "eu-west-1": {{id: "i-eu"}}}}
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
	if len(results) != 1 || results[0].ResourceName != "i-eu" {
		t.Errorf("results = %+v", results)
	}
}

func TestNoSharedStateMutation(t *testing.T) {
	// v1 set the region on the shared session for every loop iteration. The v2
	// service must leave the caller's aws.Config untouched, and concurrent
	// scans with different scopes must not influence each other.
	f := &fakeEC2{instances: map[string][]inst{
		"us-east-1": {{id: "i-east"}}, "eu-west-1": {{id: "i-eu"}}, "us-west-2": {{id: "i-west"}},
	}}
	base := newEnv(t, f)
	cfgBefore, ok := types.AWSConfigFrom(base)
	if !ok {
		t.Fatal("no aws.Config in context")
	}
	before := cfgBefore.Region

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
	if cfgAfter.Region != before || before != "us-west-2" {
		t.Errorf("config region changed: %q -> %q", before, cfgAfter.Region)
	}
	// Each instance's follow-ups were signed for the region it lives in.
	home := map[string]string{"i-east": "us-east-1", "i-eu": "eu-west-1", "i-west": "us-west-2"}
	for _, r := range f.actions("DescribeInstanceAttribute") {
		if home[r.form["InstanceId"]] != r.region {
			t.Errorf("%s follow-up signed for %s, want %s", r.form["InstanceId"], r.region, home[r.form["InstanceId"]])
		}
	}
}

func TestDescribeInstancesErrorNormalized(t *testing.T) {
	const msg = "You are not authorized to perform this operation."
	f := &fakeEC2{
		instances: map[string][]inst{"us-east-1": {{id: "i-1"}}},
		deny:      map[string]string{"us-west-1": msg},
	}
	ctx := newEnv(t, f)
	svc := EC2Calls[0]
	out, err := svc.Invoke(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	if want := "UnauthorizedOperation: " + msg; err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if code := awsv2.ErrorCode(err); code != "UnauthorizedOperation" {
		t.Errorf("ErrorCode = %q", code)
	}
	for _, noise := range []string{"status code", "request id", "operation error", "403"} {
		if strings.Contains(strings.ToLower(err.Error()), noise) {
			t.Errorf("error %q contains transport noise %q", err, noise)
		}
	}
	// v1 aborted the whole call on the first failing region.
	idx := -1
	for j, r := range types.RegionsFor(ctx) {
		if r == "us-west-1" {
			idx = j
		}
	}
	if got := f.regionsOf("DescribeInstances"); len(got) != idx+1 {
		t.Errorf("DescribeInstances regions = %v, want loop to stop at us-west-1 (index %d)", got, idx)
	}
	results := svc.Process(out, err, false)
	if len(results) != 1 || results[0].Error == nil || results[0].ServiceName != "EC2" ||
		results[0].MethodName != "ec2:DescribeInstances" || results[0].ResourceName != "" {
		t.Fatalf("results = %+v", results)
	}
}

func TestNoInstancesYieldsNoResults(t *testing.T) {
	results, err := run(newEnv(t, &fakeEC2{}))
	if err != nil || len(results) != 0 {
		t.Errorf("results = %+v, err = %v", results, err)
	}
}

func TestAmbientEndpointIgnored(t *testing.T) {
	// newEnv sets AWS_ENDPOINT_URL_EC2 to an unreachable host; success proves
	// the explicit endpoint won.
	f := &fakeEC2{instances: map[string][]inst{"us-west-2": {{id: "i-1"}}}}
	results, err := run(scoped(newEnv(t, f), "us-west-2"))
	if err != nil || len(results) != 1 {
		t.Errorf("results = %+v, err = %v", results, err)
	}
}

func TestProcessIgnoresUnexpectedOutput(t *testing.T) {
	for _, out := range []interface{}{nil, "x", 3, []string{"nope"}} {
		if got := EC2Calls[0].Process(out, nil, false); len(got) != 0 {
			t.Errorf("results = %+v", got)
		}
	}
}
