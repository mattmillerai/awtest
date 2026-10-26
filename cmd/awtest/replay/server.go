package replay

import (
	"context"
	"embed"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

// SampleName is the -replay value selecting the embedded sample fixtures.
const SampleName = "builtin:sample"

// Obviously fake credentials used for every replay session.
const (
	FakeAccessKeyID     = "AKIAREPLAYFAKE000000"
	FakeSecretAccessKey = "replay-fake-secret-not-a-real-key-000000"
)

const maxBodySize = 1 << 20

//go:embed all:sample
var sampleFS embed.FS

// SampleFS returns the embedded sample fixtures.
func SampleFS() fs.FS {
	sub, err := fs.Sub(sampleFS, "sample")
	if err != nil {
		panic(err) // the directory is embedded at build time
	}
	return sub
}

// OpenFS resolves a -replay value to a fixture file system: "builtin:sample"
// or an existing directory.
func OpenFS(spec string) (fs.FS, error) {
	if spec == SampleName {
		return SampleFS(), nil
	}
	st, err := os.Stat(spec)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", spec)
	}
	return os.DirFS(spec), nil
}

// Server is a loopback HTTP server answering AWS API calls from fixtures.
type Server struct {
	URL string

	fx   *Fixtures
	srv  *http.Server
	reqs int64
	seq  int64

	mu        sync.Mutex
	unmatched map[string]struct{}
}

// Start listens on 127.0.0.1 (random port) and serves f until Close.
func Start(f *Fixtures) (*Server, error) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &Server{fx: f, unmatched: map[string]struct{}{}}
	s.URL = "http://" + ln.Addr().String()
	s.srv = &http.Server{Handler: http.HandlerFunc(s.handle), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = s.srv.Serve(ln) }()
	return s, nil
}

// Close stops the server.
func (s *Server) Close() { _ = s.srv.Close() }

// Requests returns the number of requests received.
func (s *Server) Requests() int { return int(atomic.LoadInt64(&s.reqs)) }

// Unmatched returns the sorted, de-duplicated "service:Action" names that had
// no fixture.
func (s *Server) Unmatched() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.unmatched))
	for k := range s.unmatched {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// RestrictedTransport returns a transport that can only dial addr
// ("127.0.0.1:port") and never uses a proxy.
func RestrictedTransport(addr string) *http.Transport {
	d := &net.Dialer{Timeout: 10 * time.Second}
	return &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if address != addr {
				return nil, fmt.Errorf("awtest replay: refusing to connect to %s (only %s is allowed)", address, addr)
			}
			return d.DialContext(ctx, network, address)
		},
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     30 * time.Second,
	}
}

// NewSession builds a session with fake static credentials whose HTTP client
// can only reach srv, then points every client at srv. Shared config files,
// profiles and credential environment variables are not consulted.
func NewSession(srv *Server, region string) (*session.Session, error) {
	addr := strings.TrimPrefix(srv.URL, "http://")
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigDisable,
		SharedConfigFiles: []string{},
		Config: aws.Config{
			Region:      aws.String(region),
			Credentials: credentials.NewStaticCredentials(FakeAccessKeyID, FakeSecretAccessKey, ""),
			HTTPClient:  &http.Client{Transport: RestrictedTransport(addr), Timeout: 60 * time.Second},
		},
	})
	if err != nil {
		return nil, err
	}
	utils.ApplyEndpoint(sess, srv.URL)
	return sess, nil
}

type protocol int

const (
	protoQuery protocol = iota
	protoJSON
	protoS3
	protoEC2
	protoREST
)

type operation struct {
	service, region, action, resource string
	proto                             protocol
}

var credRe = regexp.MustCompile(`Credential=[^/,\s]+/[^/]+/([^/]+)/([^/]+)/aws4_request`)

// parseScope extracts the signing service name and region from a SigV4
// Authorization header (or presigned X-Amz-Credential parameter).
func parseScope(r *http.Request) (service, region string) {
	if m := credRe.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		return m[2], m[1]
	}
	if c := r.URL.Query().Get("X-Amz-Credential"); c != "" {
		if p := strings.Split(c, "/"); len(p) == 5 {
			return p[3], p[2]
		}
	}
	return "unknown", ""
}

// resourceKeys are request parameters that name the resource acted upon, in
// priority order.
var resourceKeys = []string{"UserName", "RoleName", "GroupName", "PolicyName", "InstanceId", "TableName", "FunctionName", "Name"}

func formResource(get func(string) string) string {
	for _, k := range resourceKeys {
		if v := get(k); v != "" {
			return v
		}
	}
	return ""
}

// classify names the operation of a request; body is the (already size
// limited) request body.
func classify(r *http.Request, body []byte) operation {
	service, region := parseScope(r)
	op := operation{service: service, region: region}

	if t := r.Header.Get("X-Amz-Target"); t != "" {
		op.proto = protoJSON
		op.action = t[strings.LastIndex(t, ".")+1:]
		var m map[string]interface{}
		if json.Unmarshal(body, &m) == nil {
			op.resource = formResource(func(k string) string { s, _ := m[k].(string); return s })
		}
		return op
	}
	if service == "s3" {
		op.proto = protoS3
		op.action, op.resource = s3Operation(r)
		return op
	}
	vals := url.Values{}
	if r.Method == http.MethodPost {
		if v, err := url.ParseQuery(string(body)); err == nil {
			vals = v
		}
	}
	if vals.Get("Action") == "" {
		vals = r.URL.Query()
	}
	if action := vals.Get("Action"); action != "" {
		op.proto = protoQuery
		if service == "ec2" {
			op.proto = protoEC2
		}
		op.action = action
		op.resource = formResource(vals.Get)
		return op
	}
	op.proto = protoREST
	op.action = r.Method + "_" + strings.ReplaceAll(strings.Trim(r.URL.Path, "/"), "/", "_")
	return op
}

// s3Operation names a path-style S3 request.
func s3Operation(r *http.Request) (action, resource string) {
	p := strings.TrimPrefix(r.URL.Path, "/")
	bucket, key, _ := strings.Cut(p, "/")
	q := r.URL.Query()
	if r.Method == http.MethodGet && p == "" {
		return "ListBuckets", ""
	}
	if r.Method == http.MethodGet && key == "" {
		for _, sub := range []struct{ param, action string }{
			{"location", "GetBucketLocation"},
			{"policyStatus", "GetBucketPolicyStatus"},
			{"policy", "GetBucketPolicy"},
			{"acl", "GetBucketAcl"},
			{"publicAccessBlock", "GetPublicAccessBlock"},
		} {
			if _, ok := q[sub.param]; ok {
				return sub.action, bucket
			}
		}
		if q.Get("list-type") == "2" {
			return "ListObjectsV2", bucket
		}
	}
	name := strings.ReplaceAll(strings.Trim(r.URL.Path, "/"), "/", "_")
	if rq := r.URL.RawQuery; rq != "" {
		name, _, _ = strings.Cut(strings.SplitN(rq, "&", 2)[0], "=")
	}
	return r.Method + "_" + name, bucket
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.reqs, 1)
	body, _ := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
	_ = r.Body.Close()
	op := classify(r, body)

	w.Header().Set("x-amzn-RequestId", fmt.Sprintf("replay-%08d", atomic.AddInt64(&s.seq, 1)))
	if fx, ok := s.fx.lookup(op.service, op.region, op.action, op.resource); ok {
		w.Header().Set("Content-Type", fx.contentType)
		w.WriteHeader(fx.status)
		_, _ = w.Write(fx.body)
		return
	}

	s.mu.Lock()
	s.unmatched[op.service+":"+op.action] = struct{}{}
	s.mu.Unlock()
	writeNoFixture(w, op)
}

// writeNoFixture answers with an access-denied style error in the wire
// format of the request's protocol so SDK clients parse it normally.
func writeNoFixture(w http.ResponseWriter, op operation) {
	msg := fmt.Sprintf("awtest replay: no fixture for %s:%s (region %s); expected %s/%s.xml or .json",
		op.service, op.action, op.region, op.service, op.action)
	var esc strings.Builder
	_ = xml.EscapeText(&esc, []byte(msg))
	x := esc.String()
	reqID := w.Header().Get("x-amzn-RequestId")
	jsonBody, _ := json.Marshal(map[string]string{"__type": "AccessDeniedException", "message": msg})

	status, ct, body := http.StatusForbidden, "text/xml", ""
	switch op.proto {
	case protoJSON:
		status, ct, body = http.StatusBadRequest, "application/x-amz-json-1.1", string(jsonBody)
	case protoS3:
		body = `<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>` + x + `</Message><RequestId>` + reqID + `</RequestId></Error>`
	case protoEC2:
		body = `<?xml version="1.0" encoding="UTF-8"?><Response><Errors><Error><Code>UnauthorizedOperation</Code><Message>` + x + `</Message></Error></Errors><RequestID>` + reqID + `</RequestID></Response>`
	case protoQuery:
		body = `<?xml version="1.0" encoding="UTF-8"?><ErrorResponse><Error><Type>Sender</Type><Code>AccessDenied</Code><Message>` + x + `</Message></Error><RequestId>` + reqID + `</RequestId></ErrorResponse>`
	default:
		// REST-JSON: IVS and IVS Realtime put the AccessDeniedException text in
		// "exceptionMessage", so carry it there as well as in "message".
		restBody, _ := json.Marshal(map[string]string{"__type": "AccessDeniedException", "message": msg, "exceptionMessage": msg})
		ct, body = "application/json", string(restBody)
		w.Header().Set("x-amzn-ErrorType", "AccessDeniedException")
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}
