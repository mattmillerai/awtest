package s3

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

func TestNormalizeBucketRegion(t *testing.T) {
	for in, want := range map[string]string{"": "us-east-1", "EU": "eu-west-1", "eu-west-1": "eu-west-1", "ap-south-1": "ap-south-1"} {
		if got := NormalizeBucketRegion(aws.String(in)); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
	if NormalizeBucketRegion(nil) != "us-east-1" {
		t.Error("nil should be us-east-1")
	}
}

func TestStatusFromError(t *testing.T) {
	for _, c := range []string{"AccessDenied", "AccessDeniedException", "Forbidden", "AllAccessDisabled"} {
		if st, ok := statusFromError(awserr.New(c, "x", nil), codeNoPolicy, "none"); !ok || st != "denied" {
			t.Errorf("%s -> %q,%v", c, st, ok)
		}
	}
	if st, ok := statusFromError(awserr.New(codeNoPolicy, "x", nil), codeNoPolicy, "none"); !ok || st != "none" {
		t.Errorf("none code: %q,%v", st, ok)
	}
	if _, ok := statusFromError(awserr.New("NoSuchBucket", "x", nil), codeNoPolicy, "none"); ok {
		t.Error("NoSuchBucket must be an error")
	}
	if _, ok := statusFromError(errors.New("plain"), "", ""); ok {
		t.Error("plain error must be an error")
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
	grants := []*s3.Grant{
		{Grantee: &s3.Grantee{Type: aws.String("Group"), URI: aws.String(allUsersURI)}, Permission: aws.String("READ")},
		{Grantee: &s3.Grantee{Type: aws.String("Group"), URI: aws.String(authUsersURI)}, Permission: aws.String("WRITE")},
		{Grantee: &s3.Grantee{Type: aws.String("CanonicalUser"), ID: aws.String("abc")}, Permission: aws.String("FULL_CONTROL")},
	}
	g, f := ACLFacts(grants)
	if len(g) != 3 || g[2]["grantee"] != "abc" || g[2]["grantee_type"] != "CanonicalUser" {
		t.Errorf("grants: %v", g)
	}
	want := []string{
		"grant to AllUsers (http://acs.amazonaws.com/groups/global/AllUsers): READ",
		"grant to AuthenticatedUsers (http://acs.amazonaws.com/groups/global/AuthenticatedUsers): WRITE",
	}
	if !reflect.DeepEqual(f, want) {
		t.Errorf("findings: %v", f)
	}
}

// ---- functional tests against a fake S3 ----

type fakeResp struct {
	code int
	body string
}

type fakeS3 struct {
	mu       sync.Mutex
	buckets  []string
	location map[string]string // bucket -> LocationConstraint ("" -> empty element)
	// responses keyed by "<bucket>|<op>" where op is location/policy/policyStatus/acl/publicAccessBlock
	resp    map[string]fakeResp
	regions map[string]string // "<bucket>|<op>" -> signing region
	reqs    int
}

const s3NS = `xmlns="http://s3.amazonaws.com/doc/2006-03-01/"`

func s3Err(code int, c string) fakeResp {
	return fakeResp{code, fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>%s</Code><Message>m</Message><RequestId>r</RequestId></Error>`, c)}
}

type stray struct {
	host     string
	delegate http.RoundTripper
	mu       sync.Mutex
	bad      []string
}

func (s *stray) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host == s.host {
		return s.delegate.RoundTrip(r)
	}
	s.mu.Lock()
	s.bad = append(s.bad, r.URL.Host)
	s.mu.Unlock()
	return nil, errors.New("blocked non-test host " + r.URL.Host)
}

func newFakeS3(t *testing.T, f *fakeS3) *session.Session {
	t.Helper()
	f.regions = map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.reqs++
		w.Header().Set("Content-Type", "application/xml")
		if r.URL.Path == "/" {
			var sb strings.Builder
			for _, b := range f.buckets {
				fmt.Fprintf(&sb, `<Bucket><Name>%s</Name><CreationDate>2024-01-01T00:00:00.000Z</CreationDate></Bucket>`, b)
			}
			fmt.Fprintf(w, `<ListAllMyBucketsResult %s><Owner><ID>o</ID></Owner><Buckets>%s</Buckets></ListAllMyBucketsResult>`, s3NS, sb.String())
			return
		}
		bucket := strings.Trim(r.URL.Path, "/")
		op := ""
		for _, q := range []string{"location", "policyStatus", "policy", "acl", "publicAccessBlock"} {
			if _, ok := r.URL.Query()[q]; ok {
				op = q
				break
			}
		}
		key := bucket + "|" + op
		if parts := strings.Split(r.Header.Get("Authorization"), "Credential="); len(parts) == 2 {
			scope := strings.Split(strings.SplitN(parts[1], ",", 2)[0], "/")
			if len(scope) == 5 {
				f.regions[key] = scope[2]
			}
		}
		if rp, ok := f.resp[key]; ok && op == "location" {
			w.WriteHeader(rp.code)
			_, _ = w.Write([]byte(rp.body))
			return
		}
		if op == "location" {
			fmt.Fprintf(w, `<LocationConstraint %s>%s</LocationConstraint>`, s3NS, f.location[bucket])
			return
		}
		if rp, ok := f.resp[key]; ok {
			w.WriteHeader(rp.code)
			_, _ = w.Write([]byte(rp.body))
			return
		}
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`<Error><Code>InternalError</Code><Message>unconfigured ` + key + `</Message></Error>`))
	}))
	t.Cleanup(srv.Close)

	g := &stray{host: strings.TrimPrefix(srv.URL, "http://"), delegate: http.DefaultTransport}
	orig := http.DefaultTransport
	http.DefaultTransport = g
	t.Cleanup(func() {
		http.DefaultTransport = orig
		if len(g.bad) > 0 {
			t.Errorf("requests bypassed fake server: %v", g.bad)
		}
	})

	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String("us-east-1"),
		Credentials:      credentials.NewStaticCredentials("AKIAFAKE", "fake", ""),
		Endpoint:         aws.String(srv.URL),
		S3ForcePathStyle: aws.Bool(true),
		MaxRetries:       aws.Int(0),
	})
	if err != nil {
		t.Fatal(err)
	}
	return sess
}

func runPolicy(t *testing.T, sess *session.Session) []types.ScanResult {
	t.Helper()
	utils.Quiet = true
	t.Cleanup(func() { utils.Quiet = false })
	var c types.AWSService
	n := 0
	for _, e := range S3Calls {
		if e.Name == "s3:GetBucketPolicy" {
			c = e
			n++
		}
	}
	if n != 1 {
		t.Fatalf("want one s3:GetBucketPolicy entry, got %d", n)
	}
	out, err := c.Call(context.Background(), sess)
	return c.Process(out, err, false)
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
	sess := newFakeS3(t, f)
	res := runPolicy(t, sess)

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

	// regions: EU -> eu-west-1; default -> us-east-1; all requests signed accordingly
	if r := pick(res, "eu-bucket", "s3:GetBucketPolicy"); r.Details["region"] != "eu-west-1" {
		t.Errorf("eu region: %v", r.Details["region"])
	}
	if r := pick(res, "open", "s3:GetBucketPolicy"); r.Details["region"] != "us-east-1" {
		t.Errorf("open region: %v", r.Details["region"])
	}
	for _, op := range []string{"policy", "policyStatus", "acl", "publicAccessBlock"} {
		if got := f.regions["eu-bucket|"+op]; got != "eu-west-1" {
			t.Errorf("eu-bucket %s signed for %q, want eu-west-1", op, got)
		}
		if got := f.regions["open|"+op]; got != "us-east-1" {
			t.Errorf("open %s signed for %q, want us-east-1", op, got)
		}
	}
	// 1 list + 3 x (location + 4)
	if f.reqs != 16 {
		t.Errorf("requests = %d, want 16", f.reqs)
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
	sess := newFakeS3(t, f)
	res := runPolicy(t, sess)
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
}

func TestGetBucketPolicyListBucketsError(t *testing.T) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String("us-east-1"), Credentials: credentials.NewStaticCredentials("a", "b", ""), MaxRetries: aws.Int(0)})
	if err != nil {
		t.Fatal(err)
	}
	// Process with an error never touches the network.
	_ = sess
	utils.Quiet = true
	defer func() { utils.Quiet = false }()
	for _, e := range S3Calls {
		if e.Name == "s3:GetBucketPolicy" {
			res := e.Process(nil, awserr.New("AccessDenied", "x", nil), false)
			if len(res) != 1 || res[0].Error == nil || res[0].MethodName != "s3:GetBucketPolicy" {
				t.Errorf("res: %+v", res)
			}
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
	sess := newFakeS3(t, f)
	res := runPolicy(t, sess)
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
