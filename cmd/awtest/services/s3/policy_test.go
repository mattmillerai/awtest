package s3

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

func TestNormalizeBucketRegion(t *testing.T) {
	for in, want := range map[string]string{"": "us-east-1", "EU": "eu-west-1", "eu-west-1": "eu-west-1", "ap-south-1": "ap-south-1"} {
		in := in
		if got := NormalizeBucketRegion(&in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
	if NormalizeBucketRegion(nil) != "us-east-1" {
		t.Error("nil should be us-east-1")
	}
}

func apiErr(code string) error { return &smithy.GenericAPIError{Code: code, Message: "x"} }

func TestStatusFromError(t *testing.T) {
	for _, c := range []string{"AccessDenied", "AccessDeniedException", "Forbidden", "AllAccessDisabled"} {
		if st, ok := statusFromError(apiErr(c), codeNoPolicy, "none"); !ok || st != "denied" {
			t.Errorf("%s -> %q,%v", c, st, ok)
		}
	}
	if st, ok := statusFromError(apiErr(codeNoPolicy), codeNoPolicy, "none"); !ok || st != "none" {
		t.Errorf("none code: %q,%v", st, ok)
	}
	if _, ok := statusFromError(apiErr("NoSuchBucket"), codeNoPolicy, "none"); ok {
		t.Error("NoSuchBucket must be an error")
	}
	if _, ok := statusFromError(errors.New("plain"), "", ""); ok {
		t.Error("plain error must be an error")
	}
}

// errCode must see through the wrappers a real v2 call returns.
func TestErrCodeSeesThroughWrappers(t *testing.T) {
	base := apiErr("NoSuchBucketPolicy")
	for name, err := range map[string]error{
		"api":       base,
		"operation": &smithy.OperationError{ServiceID: "S3", OperationName: "GetBucketPolicy", Err: base},
		"wrapped":   awsv2.Wrap(&smithy.OperationError{ServiceID: "S3", OperationName: "GetBucketPolicy", Err: base}),
		"fmt":       fmt.Errorf("ctx: %w", base),
	} {
		if got := errCode(err); got != "NoSuchBucketPolicy" {
			t.Errorf("%s: errCode = %q", name, got)
		}
	}
	if errCode(nil) != "" || errCode(errors.New("plain")) != "" {
		t.Error("non-API errors must give an empty code")
	}
}

func TestPolicyFacts(t *testing.T) {
	tests := []struct {
		name     string
		doc      string
		count    int
		findings []string
		err      bool
	}{
		{"wildcard", `{"Statement":[{"Effect":"Allow","Principal":"*","Action":"s3:GetObject"}]}`, 1, []string{"Allow statement with wildcard principal (*)"}, false},
		{"aws wildcard with condition", `{"Statement":[{"Effect":"Allow","Principal":{"AWS":"*"},"Condition":{"IpAddress":{"aws:SourceIp":"1.2.3.4"}}}]}`, 1, []string{"Allow statement with wildcard principal (*) (statement has a Condition)"}, false},
		{"deny wildcard ignored", `{"Statement":{"Effect":"Deny","Principal":"*"}}`, 1, []string{}, false},
		{"specific principal", `{"Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::1:root"}},{"Effect":"Allow","Principal":{"Service":"x"}}]}`, 2, []string{}, false},
		{"malformed", `{`, 0, []string{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, f, err := PolicyFacts(tc.doc)
			if (err != nil) != tc.err || c != tc.count || !reflect.DeepEqual(f, tc.findings) {
				t.Errorf("got %d %v %v", c, f, err)
			}
		})
	}
}

func TestACLFacts(t *testing.T) {
	grants := []s3types.Grant{
		{Grantee: &s3types.Grantee{Type: s3types.TypeGroup, URI: aws.String(allUsersURI)}, Permission: s3types.PermissionRead},
		{Grantee: &s3types.Grantee{Type: s3types.TypeGroup, URI: aws.String(authUsersURI)}, Permission: s3types.PermissionWrite},
		{Grantee: &s3types.Grantee{Type: s3types.TypeCanonicalUser, ID: aws.String("abc")}, Permission: s3types.PermissionFullControl},
		{Permission: s3types.PermissionRead}, // no grantee: skipped
	}
	g, f := ACLFacts(grants)
	if len(g) != 3 || g[2]["grantee"] != "abc" || g[2]["grantee_type"] != "CanonicalUser" || g[2]["permission"] != "FULL_CONTROL" {
		t.Errorf("grants: %v", g)
	}
	if g[0]["grantee_type"] != "Group" || g[0]["permission"] != "READ" || g[0]["grantee"] != allUsersURI {
		t.Errorf("grants[0]: %v", g[0])
	}
	want := []string{
		"grant to AllUsers (http://acs.amazonaws.com/groups/global/AllUsers): READ",
		"grant to AuthenticatedUsers (http://acs.amazonaws.com/groups/global/AuthenticatedUsers): WRITE",
	}
	if !reflect.DeepEqual(f, want) {
		t.Errorf("findings: %v", f)
	}
}

func TestServicesAreV2Only(t *testing.T) {
	names := map[string]bool{}
	for _, e := range S3Calls {
		names[e.Name] = true
		if e.CallV2 == nil || e.Call != nil {
			t.Errorf("%s: CallV2 set = %v, Call set = %v; want v2 only", e.Name, e.CallV2 != nil, e.Call != nil)
		}
	}
	for _, n := range []string{"s3:ListBuckets", "s3:GetBucketPolicy"} {
		if !names[n] {
			t.Errorf("%s not registered", n)
		}
	}
}

// ---- functional tests against a fake S3 ----

type fakeResp struct {
	code int
	body string
}

// rec is one request seen by the fake.
type rec struct {
	host, method, path, op, region, token string
}

type fakeS3 struct {
	mu       sync.Mutex
	buckets  []string
	location map[string]string // bucket -> LocationConstraint ("" -> empty element)
	listErr  *fakeResp         // ListBuckets failure
	// responses keyed by "<bucket>|<op>" where op is
	// location/policy/policyStatus/acl/publicAccessBlock/objects; the key
	// "<bucket>|objects2" fails the second ListObjectsV2 page.
	resp     map[string]fakeResp
	objects  map[string]int // bucket -> number of objects
	pageSize int            // ListObjectsV2 page size (default 1000)
	reqs     []rec

	host string // expected Host header, set by newEnv
}

const s3NS = `xmlns="http://s3.amazonaws.com/doc/2006-03-01/"`

func s3Err(code int, c string) fakeResp {
	return fakeResp{code, fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>%s</Code><Message>m</Message><RequestId>r</RequestId></Error>`, c)}
}

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/s3/aws4_request`)

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rc := rec{host: r.Host, method: r.Method, path: r.URL.Path}
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		rc.region = m[1]
	}
	q := r.URL.Query()
	w.Header().Set("Content-Type", "application/xml")
	if r.URL.Path == "/" {
		rc.op = "listBuckets"
		f.reqs = append(f.reqs, rc)
		if f.listErr != nil {
			w.WriteHeader(f.listErr.code)
			_, _ = w.Write([]byte(f.listErr.body))
			return
		}
		var sb strings.Builder
		for _, b := range f.buckets {
			fmt.Fprintf(&sb, `<Bucket><Name>%s</Name><CreationDate>2024-01-01T00:00:00.000Z</CreationDate></Bucket>`, b)
		}
		fmt.Fprintf(w, `<ListAllMyBucketsResult %s><Owner><ID>o</ID></Owner><Buckets>%s</Buckets></ListAllMyBucketsResult>`, s3NS, sb.String())
		return
	}
	bucket := strings.Trim(r.URL.Path, "/")
	for _, k := range []string{"location", "policyStatus", "policy", "acl", "publicAccessBlock"} {
		if _, ok := q[k]; ok {
			rc.op = k
			break
		}
	}
	if q.Get("list-type") == "2" {
		rc.op = "objects"
		rc.token = q.Get("continuation-token")
	}
	f.reqs = append(f.reqs, rc)
	key := bucket + "|" + rc.op

	switch rc.op {
	case "location":
		if rp, ok := f.resp[key]; ok {
			w.WriteHeader(rp.code)
			_, _ = w.Write([]byte(rp.body))
			return
		}
		fmt.Fprintf(w, `<LocationConstraint %s>%s</LocationConstraint>`, s3NS, f.location[bucket])
		return
	case "objects":
		if rc.token != "" {
			key = bucket + "|objects2"
		}
		if rp, ok := f.resp[key]; ok {
			w.WriteHeader(rp.code)
			_, _ = w.Write([]byte(rp.body))
			return
		}
		size := f.pageSize
		if size == 0 {
			size = 1000
		}
		total := f.objects[bucket]
		start := 0
		if rc.token != "" {
			start, _ = strconv.Atoi(strings.TrimPrefix(rc.token, "t"))
		}
		end := start + size
		if end > total {
			end = total
		}
		var sb strings.Builder
		for i := start; i < end; i++ {
			fmt.Fprintf(&sb, `<Contents><Key>k%d</Key></Contents>`, i)
		}
		next := ""
		if end < total {
			next = fmt.Sprintf(`<NextContinuationToken>t%d</NextContinuationToken>`, end)
		}
		fmt.Fprintf(w, `<ListBucketResult %s><Name>%s</Name><KeyCount>%d</KeyCount><IsTruncated>%t</IsTruncated>%s%s</ListBucketResult>`,
			s3NS, bucket, end-start, end < total, next, sb.String())
		return
	}
	if rp, ok := f.resp[key]; ok {
		w.WriteHeader(rp.code)
		_, _ = w.Write([]byte(rp.body))
		return
	}
	w.WriteHeader(500)
	_, _ = w.Write([]byte(`<Error><Code>InternalError</Code><Message>unconfigured ` + key + `</Message></Error>`))
}

func (f *fakeS3) requests(op string) []rec {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []rec
	for _, r := range f.reqs {
		if r.op == op {
			out = append(out, r)
		}
	}
	return out
}

// regionsFor returns the SigV4 scope regions of the requests for op on bucket.
func (f *fakeS3) regionsFor(bucket, op string) []string {
	var out []string
	for _, r := range f.requests(op) {
		if strings.Trim(r.path, "/") == bucket {
			out = append(out, r.region)
		}
	}
	return out
}

// badHosts returns the Host headers that were not the configured path-style
// endpoint host (virtual-hosted addressing would put the bucket in front).
func (f *fakeS3) badHosts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.reqs {
		if r.host != f.host {
			out = append(out, r.host)
		}
	}
	return out
}

// newEnv serves f over httptest and returns a context carrying a v2 config
// built with awsv2.LoadConfig. The endpoint host is a name that only resolves
// because the transport dials the test server for every address, so
// virtual-hosted addressing (<bucket>.<host>) would show up in the Host
// header. The configured region is us-west-2: no bucket lives there, so a
// client that does not take the bucket's region signs with the wrong one.
func newEnv(t *testing.T, f *fakeS3) context.Context {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	f.host = "s3.fake.test:" + port
	tr := &http.Transport{DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
	}}
	t.Cleanup(tr.CloseIdleConnections)

	t.Setenv("HOME", t.TempDir())
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	for _, k := range []string{"AWS_PROFILE", "AWS_DEFAULT_PROFILE", "AWS_SHARED_CREDENTIALS_FILE", "AWS_CONFIG_FILE", "AWS_REGION", "AWS_DEFAULT_REGION"} {
		t.Setenv(k, "")
	}
	// A client that honoured ambient endpoints could never reach the server.
	const unreachable = "http://ambient-endpoint.invalid:9"
	t.Setenv("AWS_ENDPOINT_URL", unreachable)
	t.Setenv("AWS_ENDPOINT_URL_S3", unreachable)

	cfg, err := awsv2.LoadConfig(context.Background(), awsv2.Options{
		Region:          "us-west-2",
		AccessKeyID:     "AKIAFAKEFAKEFAKE",
		SecretAccessKey: "fake",
		Endpoint:        "http://" + f.host,
		Transport:       tr,
		MaxAttempts:     1,
		Quiet:           true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return types.WithAWSConfig(context.Background(), cfg)
}

func findCall(t *testing.T, name string) types.AWSService {
	t.Helper()
	var c types.AWSService
	n := 0
	for _, e := range S3Calls {
		if e.Name == name {
			c = e
			n++
		}
	}
	if n != 1 {
		t.Fatalf("want one %s entry, got %d", name, n)
	}
	return c
}

func run(t *testing.T, name string, ctx context.Context) []types.ScanResult {
	t.Helper()
	utils.Quiet = true
	t.Cleanup(func() { utils.Quiet = false })
	c := findCall(t, name)
	out, err := c.Invoke(ctx, nil)
	return c.Process(out, err, false)
}

func runPolicy(t *testing.T, ctx context.Context) []types.ScanResult {
	return run(t, "s3:GetBucketPolicy", ctx)
}

func pick(rs []types.ScanResult, bucket, method string) *types.ScanResult {
	for i, r := range rs {
		if r.ResourceName == bucket && r.MethodName == method {
			return &rs[i]
		}
	}
	return nil
}

func TestGetBucketPolicyFunctional(t *testing.T) {
	policy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"s3:GetObject","Resource":"*","Condition":{"StringEquals":{"a":"b"}}}]}`
	f := &fakeS3{
		buckets:  []string{"open", "locked", "eu-bucket"},
		location: map[string]string{"open": "", "locked": "", "eu-bucket": "EU"},
		resp: map[string]fakeResp{
			"open|policy":                 {200, policy},
			"open|policyStatus":           {200, `<PolicyStatus ` + s3NS + `><IsPublic>TRUE</IsPublic></PolicyStatus>`},
			"open|acl":                    {200, `<AccessControlPolicy ` + s3NS + `><Owner><ID>oid</ID><DisplayName>me</DisplayName></Owner><AccessControlList><Grant><Grantee xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Group"><URI>` + allUsersURI + `</URI></Grantee><Permission>READ</Permission></Grant></AccessControlList></AccessControlPolicy>`},
			"open|publicAccessBlock":      {200, `<PublicAccessBlockConfiguration ` + s3NS + `><BlockPublicAcls>true</BlockPublicAcls><IgnorePublicAcls>false</IgnorePublicAcls><BlockPublicPolicy>true</BlockPublicPolicy><RestrictPublicBuckets>false</RestrictPublicBuckets></PublicAccessBlockConfiguration>`},
			"locked|policy":               s3Err(404, "NoSuchBucketPolicy"),
			"locked|policyStatus":         s3Err(404, "NoSuchBucketPolicy"),
			"locked|acl":                  s3Err(403, "AccessDenied"),
			"locked|publicAccessBlock":    s3Err(404, "NoSuchPublicAccessBlockConfiguration"),
			"eu-bucket|policy":            s3Err(403, "AccessDenied"),
			"eu-bucket|policyStatus":      {200, `<PolicyStatus ` + s3NS + `><IsPublic>FALSE</IsPublic></PolicyStatus>`},
			"eu-bucket|acl":               s3Err(301, "PermanentRedirect"),
			"eu-bucket|publicAccessBlock": s3Err(403, "AccessDenied"),
		},
	}
	ctx := newEnv(t, f)
	res := runPolicy(t, ctx)

	if len(res) != 12 {
		t.Fatalf("want 12 results (3 buckets x 4 calls), got %d", len(res))
	}
	for _, r := range res {
		if r.ResourceType != "bucket" || r.ServiceName != "S3" {
			t.Errorf("bad result %+v", r)
		}
		if _, ok := r.Details["region"]; !ok {
			t.Errorf("missing region: %+v", r)
		}
		if _, ok := r.Details["status"]; !ok {
			t.Errorf("missing status: %+v", r)
		}
	}

	p := pick(res, "open", "s3:GetBucketPolicy")
	if p.Error != nil || p.Details["status"] != "present" || p.Details["policy"] != policy || p.Details["statement_count"] != 1 ||
		!reflect.DeepEqual(p.Details["findings"], []string{"Allow statement with wildcard principal (*) (statement has a Condition)"}) {
		t.Errorf("open policy: %+v", p)
	}
	if r := pick(res, "open", "s3:GetBucketPolicyStatus"); r.Details["status"] != "public" || r.Error != nil {
		t.Errorf("open status: %+v", r)
	}
	acl := pick(res, "open", "s3:GetBucketAcl")
	if acl.Details["status"] != "ok" || acl.Details["owner_id"] != "oid" || acl.Details["owner_display_name"] != "me" ||
		!reflect.DeepEqual(acl.Details["findings"], []string{"grant to AllUsers (" + allUsersURI + "): READ"}) {
		t.Errorf("open acl: %+v", acl)
	}
	if grants, _ := acl.Details["grants"].([]map[string]interface{}); len(grants) != 1 || grants[0]["grantee_type"] != "Group" || grants[0]["permission"] != "READ" {
		t.Errorf("grants: %+v", acl.Details["grants"])
	}
	pab := pick(res, "open", "s3:GetPublicAccessBlock")
	if pab.Details["status"] != "configured" || pab.Details["block_public_acls"] != true || pab.Details["ignore_public_acls"] != false ||
		pab.Details["block_public_policy"] != true || pab.Details["restrict_public_buckets"] != false {
		t.Errorf("open pab: %+v", pab)
	}

	// none / denied are statuses, not errors
	for _, c := range []struct{ b, m, st string }{
		{"locked", "s3:GetBucketPolicy", "none"},
		{"locked", "s3:GetBucketPolicyStatus", "no_policy"},
		{"locked", "s3:GetBucketAcl", "denied"},
		{"locked", "s3:GetPublicAccessBlock", "none"},
		{"eu-bucket", "s3:GetBucketPolicy", "denied"},
		{"eu-bucket", "s3:GetBucketPolicyStatus", "not_public"},
		{"eu-bucket", "s3:GetPublicAccessBlock", "denied"},
	} {
		r := pick(res, c.b, c.m)
		if r == nil || r.Error != nil || r.Details["status"] != c.st {
			t.Errorf("%s %s: want status %s without error, got %+v", c.b, c.m, c.st, r)
		}
	}
	// other errors are errors
	if r := pick(res, "eu-bucket", "s3:GetBucketAcl"); r == nil || r.Error == nil {
		t.Errorf("PermanentRedirect should be an error result: %+v", r)
	}

	// results keep the v1 ordering: per bucket policy, status, acl, pab
	var order []string
	for _, r := range res {
		order = append(order, r.ResourceName+"/"+r.MethodName)
	}
	wantOrder := []string{
		"open/s3:GetBucketPolicy", "open/s3:GetBucketPolicyStatus", "open/s3:GetBucketAcl", "open/s3:GetPublicAccessBlock",
		"locked/s3:GetBucketPolicy", "locked/s3:GetBucketPolicyStatus", "locked/s3:GetBucketAcl", "locked/s3:GetPublicAccessBlock",
		"eu-bucket/s3:GetBucketPolicy", "eu-bucket/s3:GetBucketPolicyStatus", "eu-bucket/s3:GetBucketAcl", "eu-bucket/s3:GetPublicAccessBlock",
	}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Errorf("order: %v", order)
	}

	// regions: EU -> eu-west-1; default -> us-east-1
	if r := pick(res, "eu-bucket", "s3:GetBucketPolicy"); r.Details["region"] != "eu-west-1" {
		t.Errorf("eu region: %v", r.Details["region"])
	}
	if r := pick(res, "open", "s3:GetBucketPolicy"); r.Details["region"] != "us-east-1" {
		t.Errorf("open region: %v", r.Details["region"])
	}
	// 1 list + 3 x (location + 4)
	if n := len(f.reqs); n != 16 {
		t.Errorf("requests = %d, want 16", n)
	}
	if bad := f.badHosts(); len(bad) > 0 {
		t.Errorf("requests not sent to the path-style endpoint host: %v", bad)
	}
}

// The per-bucket checks must be signed for the bucket's own region, the
// location lookup and bucket listing with the configured region.
func TestGetBucketPolicySignsForBucketRegion(t *testing.T) {
	f := &fakeS3{
		buckets:  []string{"eu", "legacy-eu", "east", "ap"},
		location: map[string]string{"eu": "eu-west-1", "legacy-eu": "EU", "east": "", "ap": "ap-south-1"},
		resp:     map[string]fakeResp{},
	}
	for _, b := range f.buckets {
		for _, op := range []string{"policy", "policyStatus", "acl", "publicAccessBlock"} {
			f.resp[b+"|"+op] = s3Err(403, "AccessDenied")
		}
	}
	ctx := newEnv(t, f)
	res := runPolicy(t, ctx)
	if len(res) != 16 {
		t.Fatalf("results: %d", len(res))
	}
	want := map[string]string{"eu": "eu-west-1", "legacy-eu": "eu-west-1", "east": "us-east-1", "ap": "ap-south-1"}
	for b, region := range want {
		if got := pick(res, b, "s3:GetBucketPolicy").Details["region"]; got != region {
			t.Errorf("%s region detail = %v, want %s", b, got, region)
		}
		for _, op := range []string{"policy", "policyStatus", "acl", "publicAccessBlock"} {
			if got := f.regionsFor(b, op); !reflect.DeepEqual(got, []string{region}) {
				t.Errorf("%s %s signed for %v, want [%s]", b, op, got, region)
			}
		}
		if got := f.regionsFor(b, "location"); !reflect.DeepEqual(got, []string{"us-west-2"}) {
			t.Errorf("%s location signed for %v, want the configured region us-west-2", b, got)
		}
	}
	if got := f.requests("listBuckets"); len(got) != 1 || got[0].region != "us-west-2" {
		t.Errorf("listBuckets: %+v", got)
	}
}

func TestGetBucketPolicyLocationFailure(t *testing.T) {
	f := &fakeS3{buckets: []string{"b"}, location: map[string]string{}, resp: map[string]fakeResp{
		"b|location":          s3Err(403, "AccessDenied"),
		"b|policy":            s3Err(404, "NoSuchBucketPolicy"),
		"b|policyStatus":      s3Err(404, "NoSuchBucketPolicy"),
		"b|acl":               s3Err(404, "NoSuchBucket"),
		"b|publicAccessBlock": s3Err(404, "NoSuchPublicAccessBlockConfiguration"),
	}}
	ctx := newEnv(t, f)
	res := runPolicy(t, ctx)
	if len(res) != 4 {
		t.Fatalf("results: %d", len(res))
	}
	if r := pick(res, "b", "s3:GetBucketAcl"); r.Error == nil || r.Details["status"] != "error" {
		t.Errorf("NoSuchBucket should be error: %+v", r)
	}
	for _, r := range res {
		if r.Details["region"] != "unknown" {
			t.Errorf("failed location lookup should give region unknown: %+v", r)
		}
	}
	// with no region known the checks use the default client (configured region)
	for _, op := range []string{"policy", "policyStatus", "acl", "publicAccessBlock"} {
		if got := f.regionsFor("b", op); !reflect.DeepEqual(got, []string{"us-west-2"}) {
			t.Errorf("%s signed for %v, want [us-west-2]", op, got)
		}
	}
}

func TestGetBucketPolicyListBucketsError(t *testing.T) {
	f := &fakeS3{listErr: &fakeResp{403, s3Err(403, "AccessDenied").body}}
	ctx := newEnv(t, f)
	res := runPolicy(t, ctx)
	if len(res) != 1 || res[0].Error == nil || res[0].MethodName != "s3:GetBucketPolicy" || res[0].ResourceName != "" {
		t.Fatalf("res: %+v", res)
	}
	// normalised: no request id, status code or operation prefix
	if got := res[0].Error.Error(); got != "AccessDenied: m" {
		t.Errorf("error text = %q, want %q", got, "AccessDenied: m")
	}
	if code := awsv2.ErrorCode(res[0].Error); code != "AccessDenied" {
		t.Errorf("error code = %q", code)
	}
}

// Process with an error never touches the network.
func TestProcessWithError(t *testing.T) {
	utils.Quiet = true
	defer func() { utils.Quiet = false }()
	for _, e := range S3Calls {
		res := e.Process(nil, apiErr("AccessDenied"), false)
		if len(res) != 1 || res[0].Error == nil || res[0].MethodName != e.Name {
			t.Errorf("%s: res: %+v", e.Name, res)
		}
	}
}

func TestGetBucketPolicyBucketCap(t *testing.T) {
	f := &fakeS3{location: map[string]string{}, resp: map[string]fakeResp{}}
	for i := 0; i < maxBuckets+2; i++ {
		b := fmt.Sprintf("b%04d", i)
		f.buckets = append(f.buckets, b)
		f.resp[b+"|policy"] = s3Err(404, "NoSuchBucketPolicy")
		f.resp[b+"|policyStatus"] = s3Err(404, "NoSuchBucketPolicy")
		f.resp[b+"|acl"] = s3Err(403, "AccessDenied")
		f.resp[b+"|publicAccessBlock"] = s3Err(403, "AccessDenied")
	}
	ctx := newEnv(t, f)
	res := runPolicy(t, ctx)
	var note *types.ScanResult
	buckets := map[string]bool{}
	for i, r := range res {
		if r.ResourceType == "note" {
			note = &res[i]
		} else {
			buckets[r.ResourceName] = true
		}
	}
	if len(buckets) != maxBuckets {
		t.Errorf("buckets examined = %d, want %d", len(buckets), maxBuckets)
	}
	if note == nil || note.Details["truncated"] != true || note.Details["buckets_total"] != maxBuckets+2 || note.Details["buckets_examined"] != maxBuckets {
		t.Errorf("note: %+v", note)
	}
}

// ---- s3:ListBuckets ----

func TestListBucketsFunctional(t *testing.T) {
	f := &fakeS3{
		buckets:  []string{"east", "eu", "nolocation", "forbidden", "paged", "big", "small"},
		location: map[string]string{"east": "", "eu": "eu-west-1", "nolocation": "", "forbidden": "ap-south-1", "paged": "us-east-2", "big": "eu-west-1", "small": ""},
		resp: map[string]fakeResp{
			"nolocation|location": s3Err(403, "AccessDenied"),
			"forbidden|objects":   s3Err(403, "AccessDenied"),
		},
		objects:  map[string]int{"east": 3, "eu": 0, "paged": 1500, "big": 15000, "small": 9999},
		pageSize: 1000,
	}
	ctx := newEnv(t, f)
	res := run(t, "s3:ListBuckets", ctx)

	// bucket result for every bucket, with an empty detail map
	for _, b := range f.buckets {
		r := pick(res, b, "s3:ListBuckets")
		if r == nil || r.ResourceType != "bucket" || r.ServiceName != "S3" || r.Error != nil || len(r.Details) != 0 {
			t.Errorf("%s bucket result: %+v", b, r)
		}
	}

	// object counts and regions
	for _, c := range []struct {
		b      string
		count  int
		region string
	}{
		{"east", 3, "us-east-1"}, // empty LocationConstraint -> us-east-1
		{"eu", 0, "eu-west-1"},
		{"paged", 1500, "us-east-2"},
		{"big", 10000, "eu-west-1"},
		{"small", 9999, "us-east-1"},
	} {
		r := pick(res, c.b, "s3:ListObjects")
		if r == nil || r.Error != nil || r.Details["object_count"] != c.count || r.Details["region"] != c.region {
			t.Errorf("%s ListObjects: %+v, want %d objects in %s", c.b, r, c.count, c.region)
		}
		for _, got := range f.regionsFor(c.b, "objects") {
			if got != c.region {
				t.Errorf("%s ListObjectsV2 signed for %q, want %s", c.b, got, c.region)
			}
		}
		if got := f.regionsFor(c.b, "location"); !reflect.DeepEqual(got, []string{"us-west-2"}) {
			t.Errorf("%s GetBucketLocation signed for %v, want the configured region", c.b, got)
		}
	}

	// failed location lookup: bucket result only, no ListObjects attempt
	if r := pick(res, "nolocation", "s3:ListObjects"); r != nil {
		t.Errorf("location failure should not produce a ListObjects result: %+v", r)
	}
	if got := f.regionsFor("nolocation", "objects"); len(got) != 0 {
		t.Errorf("ListObjectsV2 attempted without a location: %v", got)
	}

	// ListObjects error is reported with a normalised error and no details
	r := pick(res, "forbidden", "s3:ListObjects")
	if r == nil || r.Error == nil || r.Details != nil || r.ResourceType != "bucket" {
		t.Fatalf("forbidden ListObjects: %+v", r)
	}
	if got := r.Error.Error(); got != "AccessDenied: m" {
		t.Errorf("error text = %q, want %q", got, "AccessDenied: m")
	}
	if got := awsv2.ErrorCode(r.Error); got != "AccessDenied" {
		t.Errorf("error code = %q", got)
	}

	// results are in bucket order: bucket result, then its ListObjects result
	var order []string
	for _, r := range res {
		order = append(order, r.ResourceName+"/"+r.MethodName)
	}
	wantOrder := []string{
		"east/s3:ListBuckets", "east/s3:ListObjects",
		"eu/s3:ListBuckets", "eu/s3:ListObjects",
		"nolocation/s3:ListBuckets",
		"forbidden/s3:ListBuckets", "forbidden/s3:ListObjects",
		"paged/s3:ListBuckets", "paged/s3:ListObjects",
		"big/s3:ListBuckets", "big/s3:ListObjects",
		"small/s3:ListBuckets", "small/s3:ListObjects",
	}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Errorf("order: %v", order)
	}
	if n := len(f.requests("listBuckets")); n != 1 {
		t.Errorf("ListBuckets calls = %d, want 1", n)
	}
	if bad := f.badHosts(); len(bad) > 0 {
		t.Errorf("requests not sent to the path-style endpoint host: %v", bad)
	}
}

func tokens(rs []rec) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.token)
	}
	return out
}

func bucketRequests(f *fakeS3, bucket, op string) []rec {
	var out []rec
	for _, r := range f.requests(op) {
		if strings.Trim(r.path, "/") == bucket {
			out = append(out, r)
		}
	}
	return out
}

func TestListBucketsPagination(t *testing.T) {
	f := &fakeS3{
		buckets:  []string{"paged"},
		location: map[string]string{"paged": "eu-west-1"},
		objects:  map[string]int{"paged": 1500},
		pageSize: 1000,
	}
	ctx := newEnv(t, f)
	res := run(t, "s3:ListBuckets", ctx)
	r := pick(res, "paged", "s3:ListObjects")
	if r == nil || r.Error != nil || r.Details["object_count"] != 1500 {
		t.Fatalf("result: %+v", r)
	}
	// two pages, the second carrying the continuation token of the first
	if got := tokens(bucketRequests(f, "paged", "objects")); !reflect.DeepEqual(got, []string{"", "t1000"}) {
		t.Errorf("continuation tokens = %q, want [\"\" \"t1000\"]", got)
	}
}

func TestListBucketsStopsAtTenThousand(t *testing.T) {
	f := &fakeS3{
		buckets:  []string{"big", "exact"},
		location: map[string]string{"big": "us-east-1", "exact": "us-east-1"},
		objects:  map[string]int{"big": 25000, "exact": 10000},
		pageSize: 5000,
	}
	ctx := newEnv(t, f)
	res := run(t, "s3:ListBuckets", ctx)
	for _, b := range []string{"big", "exact"} {
		r := pick(res, b, "s3:ListObjects")
		if r == nil || r.Error != nil || r.Details["object_count"] != 10000 {
			t.Errorf("%s: %+v", b, r)
		}
		// 2 pages of 5000 reach 10000: no third page is requested
		if got := tokens(bucketRequests(f, b, "objects")); !reflect.DeepEqual(got, []string{"", "t5000"}) {
			t.Errorf("%s: continuation tokens = %q, want [\"\" \"t5000\"]", b, got)
		}
	}
}

func TestListBucketsSecondPageError(t *testing.T) {
	f := &fakeS3{
		buckets:  []string{"b"},
		location: map[string]string{"b": ""},
		objects:  map[string]int{"b": 2500},
		pageSize: 1000,
		resp:     map[string]fakeResp{"b|objects2": s3Err(500, "InternalError")},
	}
	ctx := newEnv(t, f)
	res := run(t, "s3:ListBuckets", ctx)
	r := pick(res, "b", "s3:ListObjects")
	if r == nil || r.Error == nil || r.Details != nil {
		t.Fatalf("a failing later page must give an error result without details: %+v", r)
	}
	if got := r.Error.Error(); got != "InternalError: m" {
		t.Errorf("error text = %q", got)
	}
}

func TestListBucketsListError(t *testing.T) {
	f := &fakeS3{listErr: &fakeResp{403, s3Err(403, "AccessDenied").body}}
	ctx := newEnv(t, f)
	res := run(t, "s3:ListBuckets", ctx)
	if len(res) != 1 || res[0].Error == nil || res[0].MethodName != "s3:ListBuckets" {
		t.Fatalf("res: %+v", res)
	}
	if got := res[0].Error.Error(); got != "AccessDenied: m" {
		t.Errorf("error text = %q, want %q", got, "AccessDenied: m")
	}
}

// With AWS_ENDPOINT_URL and AWS_ENDPOINT_URL_S3 pointing at an unreachable
// host (newEnv sets both), both services must still use the configured
// endpoint, path-style.
func TestAmbientS3EndpointIgnoredAndPathStyle(t *testing.T) {
	f := &fakeS3{
		buckets:  []string{"my-bucket"},
		location: map[string]string{"my-bucket": "eu-west-1"},
		objects:  map[string]int{"my-bucket": 2},
		resp: map[string]fakeResp{
			"my-bucket|policy":            s3Err(404, "NoSuchBucketPolicy"),
			"my-bucket|policyStatus":      s3Err(404, "NoSuchBucketPolicy"),
			"my-bucket|acl":               s3Err(403, "AccessDenied"),
			"my-bucket|publicAccessBlock": s3Err(404, "NoSuchPublicAccessBlockConfiguration"),
		},
	}
	ctx := newEnv(t, f)
	if r := pick(run(t, "s3:ListBuckets", ctx), "my-bucket", "s3:ListObjects"); r == nil || r.Error != nil {
		t.Errorf("ListBuckets: %+v", r)
	}
	if r := pick(runPolicy(t, ctx), "my-bucket", "s3:GetBucketPolicy"); r == nil || r.Error != nil || r.Details["status"] != "none" {
		t.Errorf("GetBucketPolicy: %+v", r)
	}
	if len(f.reqs) == 0 {
		t.Fatal("no request reached the fake server")
	}
	if bad := f.badHosts(); len(bad) > 0 {
		t.Errorf("virtual-hosted or ambient endpoint used: %v", bad)
	}
	for _, r := range f.reqs {
		if r.op != "listBuckets" && !strings.HasPrefix(r.path, "/my-bucket") {
			t.Errorf("bucket request not path-style: %+v", r)
		}
	}
}
