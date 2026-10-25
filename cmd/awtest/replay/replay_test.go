package replay

import (
	"context"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
)

func mapFS(files ...string) fstest.MapFS {
	m := fstest.MapFS{}
	for _, f := range files {
		m[f] = &fstest.MapFile{Data: []byte("<x/>")}
	}
	return m
}

func TestLoadLayout(t *testing.T) {
	fsys := mapFS(
		"sts/GetCallerIdentity.xml",
		"ec2/us-east-1/DescribeInstances.xml",
		"s3/GetBucketPolicy/example-logs.404.xml",
		"s3/eu-west-1/GetBucketAcl/my.bucket.json",
		"dynamodb/ListTables.json",
		"README.md",
		"sts/notes.txt",
	)
	fsys["iam/ListUsers.XML"] = &fstest.MapFile{Data: []byte("<x/>")}
	fsys["sts/link.xml"] = &fstest.MapFile{Mode: fs.ModeSymlink | 0o777}
	f, err := Load(fsys)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		k      key
		status int
		ct     string
	}{
		{key{"sts", "", "GetCallerIdentity", ""}, 200, "text/xml"},
		{key{"ec2", "us-east-1", "DescribeInstances", ""}, 200, "text/xml"},
		{key{"s3", "", "GetBucketPolicy", "example-logs"}, 404, "text/xml"},
		{key{"s3", "eu-west-1", "GetBucketAcl", "my.bucket"}, 200, "application/x-amz-json-1.1"},
		{key{"dynamodb", "", "ListTables", ""}, 200, "application/x-amz-json-1.1"},
		{key{"iam", "", "ListUsers", ""}, 200, "text/xml"},
	}
	for _, c := range cases {
		fx, ok := f.m[c.k]
		if !ok {
			t.Errorf("missing %+v", c.k)
			continue
		}
		if fx.status != c.status || fx.contentType != c.ct {
			t.Errorf("%+v: status=%d ct=%s", c.k, fx.status, fx.contentType)
		}
	}
	if f.Len() != len(cases) {
		t.Errorf("loaded %d fixtures, want %d (ignored files / symlink must be skipped)", f.Len(), len(cases))
	}
}

func TestLoadRealSymlinkSkipped(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.xml")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "sts", "GetCallerIdentity.xml")); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	f, err := Load(os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}
	if f.Len() != 0 {
		t.Errorf("symlinked fixture was loaded")
	}
}

func TestLoadErrors(t *testing.T) {
	t.Run("duplicate extension", func(t *testing.T) {
		_, err := Load(mapFS("sts/GetCallerIdentity.xml", "sts/GetCallerIdentity.json"))
		if err == nil || !strings.Contains(err.Error(), "GetCallerIdentity.xml") || !strings.Contains(err.Error(), "GetCallerIdentity.json") {
			t.Errorf("err = %v, want duplicate naming both files", err)
		}
	})
	t.Run("duplicate status", func(t *testing.T) {
		_, err := Load(mapFS("s3/GetBucketPolicy/b.xml", "s3/GetBucketPolicy/b.404.xml"))
		if err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("bad layout", func(t *testing.T) {
		if _, err := Load(mapFS("a/b/c/d/e.xml")); err == nil {
			t.Error("want error")
		}
		if _, err := Load(mapFS("sts.xml")); err == nil {
			t.Error("want error for file without service dir")
		}
		if _, err := Load(mapFS("s3/NotARegion/GetBucketAcl/b.xml")); err == nil {
			t.Error("want error for non-region dir at depth 4")
		}
	})
	t.Run("too large", func(t *testing.T) {
		fsys := fstest.MapFS{"sts/Big.xml": &fstest.MapFile{Data: make([]byte, maxFixtureSize+1)}}
		if _, err := Load(fsys); err == nil {
			t.Error("want size error")
		}
	})
}

func TestRegionRegexp(t *testing.T) {
	for _, r := range []string{"us-east-1", "eu-west-2", "ap-southeast-4", "us-gov-west-1"} {
		if !regionRe.MatchString(r) {
			t.Errorf("%s should match", r)
		}
	}
	for _, r := range []string{"GetBucketAcl", "example-logs", "us-east", "iam"} {
		if regionRe.MatchString(r) {
			t.Errorf("%s should not match", r)
		}
	}
}

func TestLookupPriority(t *testing.T) {
	fsys := fstest.MapFS{}
	for name, body := range map[string]string{
		"svc/R.xml":                       "generic",
		"svc/R/res.xml":                   "resource",
		"svc/us-east-1/R.xml":             "region",
		"svc/us-east-1/R/res.xml":         "region+resource",
		"svc/Only/x.xml":                  "only-resource",
		"svc/us-west-2/Other.xml":         "west-only",
		"svc/us-west-2/Other/thing.xml":   "west-thing",
		"svc/us-west-2/Other/thing.5.xml": "", // not a status (2 digits): name is "thing.5"
	} {
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
	}
	f, err := Load(fsys)
	if err != nil {
		t.Fatal(err)
	}
	get := func(region, action, res string) string {
		fx, ok := f.lookup("svc", region, action, res)
		if !ok {
			return "<none>"
		}
		return string(fx.body)
	}
	for _, c := range []struct{ region, action, res, want string }{
		{"us-east-1", "R", "res", "region+resource"},
		{"us-east-1", "R", "other", "region"},
		{"us-west-2", "R", "res", "resource"},
		{"us-west-2", "R", "other", "generic"},
		{"us-west-2", "R", "", "generic"},
		{"", "R", "", "generic"},
		{"us-east-1", "Only", "x", "only-resource"},
		{"us-east-1", "Only", "y", "<none>"},
		{"us-east-1", "Other", "thing", "<none>"},
		{"us-west-2", "Other", "thing", "west-thing"},
		{"us-west-2", "Other", "zzz", "west-only"},
	} {
		if got := get(c.region, c.action, c.res); got != c.want {
			t.Errorf("lookup(%q,%q,%q) = %q, want %q", c.region, c.action, c.res, got, c.want)
		}
	}
}

func auth(service, region string) string {
	return "AWS4-HMAC-SHA256 Credential=AKIAREPLAYFAKE000000/20240101/" + region + "/" + service + "/aws4_request, SignedHeaders=host, Signature=abc"
}

func TestParseScope(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", auth("monitoring", "eu-west-1"))
	if s, reg := parseScope(r); s != "monitoring" || reg != "eu-west-1" {
		t.Errorf("got %q %q", s, reg)
	}
	r = httptest.NewRequest("GET", "/?X-Amz-Credential="+url.QueryEscape("AKID/20240101/us-west-2/s3/aws4_request"), nil)
	if s, reg := parseScope(r); s != "s3" || reg != "us-west-2" {
		t.Errorf("presigned: got %q %q", s, reg)
	}
	r = httptest.NewRequest("GET", "/", nil)
	if s, reg := parseScope(r); s != "unknown" || reg != "" {
		t.Errorf("unsigned: got %q %q", s, reg)
	}
}

func TestClassify(t *testing.T) {
	form := func(service, target string, body string) *http.Request {
		r := httptest.NewRequest("POST", target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Authorization", auth(service, "us-west-2"))
		return r
	}
	signed := func(method, target, service string) *http.Request {
		r := httptest.NewRequest(method, target, nil)
		r.Header.Set("Authorization", auth(service, "us-west-2"))
		return r
	}
	jsonReq := signed("POST", "/", "dynamodb")
	jsonReq.Header.Set("X-Amz-Target", "DynamoDB_20120810.DescribeTable")

	cases := []struct {
		name string
		r    *http.Request
		body string
		want operation
	}{
		{"json target", jsonReq, `{"TableName":"orders"}`, operation{"dynamodb", "us-west-2", "DescribeTable", "orders", protoJSON}},
		{"query body", form("iam", "/", ""), "Action=ListAttachedUserPolicies&UserName=alice&Version=2010-05-08", operation{"iam", "us-west-2", "ListAttachedUserPolicies", "alice", protoQuery}},
		{"query url", signed("GET", "/?Action=ListUsers&Version=2010-05-08", "iam"), "", operation{"iam", "us-west-2", "ListUsers", "", protoQuery}},
		{"ec2", form("ec2", "/", ""), "Action=DescribeInstances&Version=2016-11-15", operation{"ec2", "us-west-2", "DescribeInstances", "", protoEC2}},
		{"s3 list buckets", signed("GET", "/", "s3"), "", operation{"s3", "us-west-2", "ListBuckets", "", protoS3}},
		{"s3 location", signed("GET", "/b1?location", "s3"), "", operation{"s3", "us-west-2", "GetBucketLocation", "b1", protoS3}},
		{"s3 policy", signed("GET", "/b1?policy", "s3"), "", operation{"s3", "us-west-2", "GetBucketPolicy", "b1", protoS3}},
		{"s3 policyStatus", signed("GET", "/b1?policyStatus", "s3"), "", operation{"s3", "us-west-2", "GetBucketPolicyStatus", "b1", protoS3}},
		{"s3 acl", signed("GET", "/b1?acl", "s3"), "", operation{"s3", "us-west-2", "GetBucketAcl", "b1", protoS3}},
		{"s3 pab", signed("GET", "/b1?publicAccessBlock", "s3"), "", operation{"s3", "us-west-2", "GetPublicAccessBlock", "b1", protoS3}},
		{"s3 list v2", signed("GET", "/b1?list-type=2&max-keys=1", "s3"), "", operation{"s3", "us-west-2", "ListObjectsV2", "b1", protoS3}},
		{"s3 other query", signed("PUT", "/b1?versioning", "s3"), "", operation{"s3", "us-west-2", "PUT_versioning", "b1", protoS3}},
		{"s3 other path", signed("GET", "/b1/some/key.txt", "s3"), "", operation{"s3", "us-west-2", "GET_b1_some_key.txt", "b1", protoS3}},
		{"s3 object acl is not bucket acl", signed("GET", "/b1/k?acl", "s3"), "", operation{"s3", "us-west-2", "GET_acl", "b1", protoS3}},
		{"rest", signed("GET", "/2015-03-31/functions", "lambda"), "", operation{"lambda", "us-west-2", "GET_2015-03-31_functions", "", protoREST}},
	}
	for _, c := range cases {
		got := classify(c.r, []byte(c.body))
		if got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func startEmpty(t *testing.T) *Server {
	t.Helper()
	f, err := Load(fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	srv, err := Start(f)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	return srv
}

func TestUnknownCallErrorsParseWithSDK(t *testing.T) {
	srv := startEmpty(t)
	sess, err := NewSession(srv, "us-west-2")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &aws.Config{MaxRetries: aws.Int(0)}
	ctx := context.Background()

	checks := []struct {
		name, code, unmatched string
		call                  func() error
	}{
		{"sts (query)", "AccessDenied", "sts:GetCallerIdentity", func() error {
			_, err := sts.New(sess, cfg).GetCallerIdentityWithContext(ctx, &sts.GetCallerIdentityInput{})
			return err
		}},
		{"iam (query)", "AccessDenied", "iam:ListUsers", func() error {
			_, err := iam.New(sess, cfg).ListUsersWithContext(ctx, &iam.ListUsersInput{})
			return err
		}},
		{"ec2", "UnauthorizedOperation", "ec2:DescribeInstances", func() error {
			_, err := ec2.New(sess, cfg).DescribeInstancesWithContext(ctx, &ec2.DescribeInstancesInput{})
			return err
		}},
		{"s3", "AccessDenied", "s3:ListBuckets", func() error {
			_, err := s3.New(sess, cfg).ListBucketsWithContext(ctx, &s3.ListBucketsInput{})
			return err
		}},
		{"dynamodb (json)", "AccessDeniedException", "dynamodb:ListTables", func() error {
			_, err := dynamodb.New(sess, cfg).ListTablesWithContext(ctx, &dynamodb.ListTablesInput{})
			return err
		}},
		{"lambda (rest)", "AccessDeniedException", "lambda:GET_2015-03-31_functions", func() error {
			_, err := lambda.New(sess, cfg).ListFunctionsWithContext(ctx, &lambda.ListFunctionsInput{})
			return err
		}},
	}
	var wantUnmatched []string
	for _, c := range checks {
		err := c.call()
		ae, ok := err.(awserr.Error)
		if !ok || ae.Code() != c.code {
			t.Errorf("%s: err = %v, want code %s", c.name, err, c.code)
			continue
		}
		if !strings.Contains(ae.Message(), "no fixture for "+c.unmatched[:strings.Index(c.unmatched, ":")]) {
			t.Errorf("%s: message %q", c.name, ae.Message())
		}
		wantUnmatched = append(wantUnmatched, c.unmatched)
	}
	got := strings.Join(srv.Unmatched(), ",")
	for _, w := range wantUnmatched {
		if !strings.Contains(got, w) {
			t.Errorf("Unmatched() = %v, missing %s", srv.Unmatched(), w)
		}
	}
	if srv.Requests() != len(checks) {
		t.Errorf("Requests() = %d, want %d", srv.Requests(), len(checks))
	}
	if s := srv.Unmatched(); len(s) != len(checks) {
		t.Errorf("Unmatched() = %v", s)
	}
	// repeating a call does not add a duplicate entry
	_ = checks[0].call()
	if len(srv.Unmatched()) != len(checks) || srv.Requests() != len(checks)+1 {
		t.Errorf("after repeat: unmatched=%v requests=%d", srv.Unmatched(), srv.Requests())
	}
}

func TestServesFixtureWithStatus(t *testing.T) {
	fsys := fstest.MapFS{
		"s3/GetBucketPolicy/b.404.xml": &fstest.MapFile{Data: []byte(`<Error><Code>NoSuchBucketPolicy</Code><Message>none</Message></Error>`)},
	}
	f, err := Load(fsys)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := Start(f)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	sess, _ := NewSession(srv, "us-west-2")
	_, err = s3.New(sess, &aws.Config{MaxRetries: aws.Int(0)}).GetBucketPolicy(&s3.GetBucketPolicyInput{Bucket: aws.String("b")})
	if ae, ok := err.(awserr.Error); !ok || ae.Code() != "NoSuchBucketPolicy" {
		t.Errorf("err = %v", err)
	}
	if len(srv.Unmatched()) != 0 {
		t.Errorf("unexpected unmatched %v", srv.Unmatched())
	}
}

func TestBindsLoopbackOnly(t *testing.T) {
	srv := startEmpty(t)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatal(err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.Equal(net.IPv4(127, 0, 0, 1)) {
		t.Errorf("server host = %q, want 127.0.0.1", host)
	}
	// The listener must not accept connections on another local address.
	_, port, _ := net.SplitHostPort(u.Host)
	if c, err := net.Dial("tcp4", net.JoinHostPort("127.0.0.2", port)); err == nil {
		c.Close()
		t.Error("server reachable on 127.0.0.2; expected 127.0.0.1 only")
	}
}

func TestRestrictedTransportRefusesOtherHosts(t *testing.T) {
	var hits int64
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
	}))
	defer other.Close()
	srv := startEmpty(t)

	tr := RestrictedTransport(strings.TrimPrefix(srv.URL, "http://"))
	// proxy env must not matter
	t.Setenv("HTTP_PROXY", other.URL)
	t.Setenv("HTTPS_PROXY", other.URL)
	client := &http.Client{Transport: tr}

	if resp, err := client.Get(other.URL); err == nil {
		resp.Body.Close()
		t.Fatal("request to another server succeeded")
	}
	if n := atomic.LoadInt64(&hits); n != 0 {
		t.Errorf("other server saw %d request(s)", n)
	}
	resp, err := client.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("request to replay server failed: %v", err)
	}
	resp.Body.Close()
}

func TestFakeCredentialLengths(t *testing.T) {
	if len(FakeAccessKeyID) != 20 || !strings.HasPrefix(FakeAccessKeyID, "AKIA") {
		t.Errorf("access key %q has length %d", FakeAccessKeyID, len(FakeAccessKeyID))
	}
	if len(FakeSecretAccessKey) != 40 || !strings.Contains(FakeSecretAccessKey, "fake") {
		t.Errorf("secret has length %d", len(FakeSecretAccessKey))
	}
}

func TestSampleLoads(t *testing.T) {
	f, err := Load(SampleFS())
	if err != nil {
		t.Fatal(err)
	}
	if f.Len() < 20 {
		t.Errorf("sample has only %d fixtures", f.Len())
	}
}

func TestOpenFS(t *testing.T) {
	if _, err := OpenFS(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("want error for missing dir")
	}
	file := filepath.Join(t.TempDir(), "f.xml")
	_ = os.WriteFile(file, nil, 0o600)
	if _, err := OpenFS(file); err == nil {
		t.Error("want error for file")
	}
	if _, err := OpenFS(SampleName); err != nil {
		t.Error(err)
	}
}
