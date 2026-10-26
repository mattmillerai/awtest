package sts

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aws/smithy-go"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

const (
	stsNS = "https://sts.amazonaws.com/doc/2011-06-15/"
	iamNS = "https://iam.amazonaws.com/doc/2010-05-08/"
)

// fakeAWS answers the sts and iam query-protocol calls the service makes.
type fakeAWS struct {
	arn string
	// denyIAM makes both IAM follow-ups fail with AccessDenied.
	denyIAM bool
	// denySTS makes GetCallerIdentity fail with AccessDenied.
	denySTS bool
	// policiesFor is the only user the IAM calls return data for; any other
	// user gets empty lists, so a wrong user name shows up as missing results.
	policiesFor string

	mu      sync.Mutex
	actions []string
	users   []string
}

func (f *fakeAWS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	action, user := r.PostForm.Get("Action"), r.PostForm.Get("UserName")
	f.mu.Lock()
	f.actions = append(f.actions, action)
	if user != "" {
		f.users = append(f.users, user)
	}
	f.mu.Unlock()

	w.Header().Set("Content-Type", "text/xml")
	deny := func(msg string) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `<ErrorResponse xmlns="%s"><Error><Type>Sender</Type><Code>AccessDenied</Code><Message>%s</Message></Error><RequestId>req-denied-1</RequestId></ErrorResponse>`, iamNS, msg)
	}
	switch action {
	case "GetCallerIdentity":
		if f.denySTS {
			deny("User is not authorized to perform: sts:GetCallerIdentity")
			return
		}
		fmt.Fprintf(w, `<GetCallerIdentityResponse xmlns="%s"><GetCallerIdentityResult><Arn>%s</Arn><UserId>AIDAEXAMPLE</UserId><Account>123456789012</Account></GetCallerIdentityResult><ResponseMetadata><RequestId>r-1</RequestId></ResponseMetadata></GetCallerIdentityResponse>`, stsNS, f.arn)
	case "ListAttachedUserPolicies":
		if f.denyIAM {
			deny("not authorized to perform: iam:ListAttachedUserPolicies")
			return
		}
		body := ""
		if user == f.policiesFor {
			body = `<member><PolicyName>ReadOnly</PolicyName><PolicyArn>arn:aws:iam::aws:policy/ReadOnlyAccess</PolicyArn></member><member><PolicyName>Custom</PolicyName><PolicyArn>arn:aws:iam::123456789012:policy/Custom</PolicyArn></member>`
		}
		fmt.Fprintf(w, `<ListAttachedUserPoliciesResponse xmlns="%s"><ListAttachedUserPoliciesResult><AttachedPolicies>%s</AttachedPolicies><IsTruncated>false</IsTruncated></ListAttachedUserPoliciesResult><ResponseMetadata><RequestId>r-2</RequestId></ResponseMetadata></ListAttachedUserPoliciesResponse>`, iamNS, body)
	case "ListUserPolicies":
		if f.denyIAM {
			deny("not authorized to perform: iam:ListUserPolicies")
			return
		}
		body := ""
		if user == f.policiesFor {
			body = `<member>inline-one</member><member>inline-two</member>`
		}
		fmt.Fprintf(w, `<ListUserPoliciesResponse xmlns="%s"><ListUserPoliciesResult><PolicyNames>%s</PolicyNames><IsTruncated>false</IsTruncated></ListUserPoliciesResult><ResponseMetadata><RequestId>r-3</RequestId></ResponseMetadata></ListUserPoliciesResponse>`, iamNS, body)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

// newEnv starts the fake server and returns a context carrying a v2 config
// aimed at it. Ambient sts/iam endpoints point at an unreachable host, so a
// client that honoured them could never succeed.
func newEnv(t *testing.T, f *fakeAWS) context.Context {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	for _, k := range []string{"AWS_PROFILE", "AWS_DEFAULT_PROFILE", "AWS_SHARED_CREDENTIALS_FILE", "AWS_CONFIG_FILE", "AWS_REGION", "AWS_DEFAULT_REGION"} {
		t.Setenv(k, "")
	}
	const unreachable = "http://ambient-endpoint.invalid:9"
	t.Setenv("AWS_ENDPOINT_URL", unreachable)
	t.Setenv("AWS_ENDPOINT_URL_STS", unreachable)
	t.Setenv("AWS_ENDPOINT_URL_IAM", unreachable)

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

// run executes the service the way the scanner does: Invoke, then Process.
func run(t *testing.T, ctx context.Context) ([]types.ScanResult, error) {
	t.Helper()
	svc := STSCalls[0]
	out, err := svc.Invoke(ctx)
	return svc.Process(out, err, false), err
}

func TestServiceIsV2Only(t *testing.T) {
	if len(STSCalls) != 1 {
		t.Fatalf("expected 1 service, got %d", len(STSCalls))
	}
	svc := STSCalls[0]
	if svc.CallV2 == nil {
		t.Error("want CallV2 set")
	}
	if svc.Name != "sts:GetCallerIdentity" {
		t.Errorf("name = %q", svc.Name)
	}
}

func TestSuccessPath(t *testing.T) {
	f := &fakeAWS{arn: "arn:aws:iam::123456789012:user/alice", policiesFor: "alice"}
	results, err := run(t, newEnv(t, f))
	if err != nil {
		t.Fatal(err)
	}

	user := map[string]interface{}{"user": "alice"}
	want := []types.ScanResult{
		{ServiceName: "STS", MethodName: "sts:GetCallerIdentity", ResourceType: "identity", ResourceName: "AIDAEXAMPLE",
			Details: map[string]interface{}{"account": "123456789012", "arn": "arn:aws:iam::123456789012:user/alice"}},
		{ServiceName: "IAM", MethodName: "iam:ListAttachedUserPolicies", ResourceType: "policy", ResourceName: "arn:aws:iam::aws:policy/ReadOnlyAccess", Details: user},
		{ServiceName: "IAM", MethodName: "iam:ListAttachedUserPolicies", ResourceType: "policy", ResourceName: "arn:aws:iam::123456789012:policy/Custom", Details: user},
		{ServiceName: "IAM", MethodName: "iam:ListUserPolicies", ResourceType: "inline-policy", ResourceName: "inline-one", Details: user},
		{ServiceName: "IAM", MethodName: "iam:ListUserPolicies", ResourceType: "inline-policy", ResourceName: "inline-two", Details: user},
	}
	if len(results) != len(want) {
		t.Fatalf("got %d results, want %d: %+v", len(results), len(want), results)
	}
	for i, w := range want {
		g := results[i]
		if g.ServiceName != w.ServiceName || g.MethodName != w.MethodName || g.ResourceType != w.ResourceType ||
			g.ResourceName != w.ResourceName || fmt.Sprint(g.Details) != fmt.Sprint(w.Details) || g.Error != nil || g.Timestamp.IsZero() {
			t.Errorf("result %d = %+v, want %+v", i, g, w)
		}
	}
	// The requests reached the test server despite hostile ambient endpoints.
	if got := strings.Join(f.actions, ","); got != "GetCallerIdentity,ListAttachedUserPolicies,ListUserPolicies" {
		t.Errorf("actions = %s", got)
	}
}

func TestIAMFollowUpsAccessDenied(t *testing.T) {
	f := &fakeAWS{arn: "arn:aws:iam::123456789012:user/alice", denyIAM: true}
	results, err := run(t, newEnv(t, f))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ResourceType != "identity" || results[0].ResourceName != "AIDAEXAMPLE" || results[0].Error != nil {
		t.Fatalf("want only the identity result, got %+v", results)
	}
	if len(f.actions) != 3 {
		t.Errorf("both follow-ups must still be attempted, actions = %v", f.actions)
	}
}

func TestGetCallerIdentityAccessDeniedNormalized(t *testing.T) {
	f := &fakeAWS{denySTS: true}
	ctx := newEnv(t, f)
	svc := STSCalls[0]
	out, err := svc.Invoke(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	const want = "AccessDenied: User is not authorized to perform: sts:GetCallerIdentity"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	for _, noise := range []string{"status code", "request id", "req-denied-1", "operation error", "403"} {
		if strings.Contains(strings.ToLower(err.Error()), noise) {
			t.Errorf("error %q contains transport noise %q", err, noise)
		}
	}
	var ae smithy.APIError
	if !errors.As(err, &ae) || ae.ErrorCode() != "AccessDenied" {
		t.Errorf("errors.As(smithy.APIError) = %v, %v", ae, errors.As(err, &ae))
	}

	results := svc.Process(out, err, false)
	if len(results) != 1 || results[0].Error == nil || results[0].MethodName != "sts:GetCallerIdentity" || results[0].ServiceName != "STS" {
		t.Fatalf("results = %+v", results)
	}
	if results[0].ResourceType != "" || results[0].ResourceName != "" || results[0].Details != nil {
		t.Errorf("error result must carry only the error: %+v", results[0])
	}
	if len(f.actions) != 1 {
		t.Errorf("no IAM follow-up on identity failure, actions = %v", f.actions)
	}
}

func TestUserNameDerivation(t *testing.T) {
	for _, tc := range []struct{ arn, user string }{
		{"arn:aws:iam::123456789012:user/alice", "alice"},
		{"arn:aws:iam::123456789012:user/division/team/bob", "bob"},
		{"arn:aws:sts::123456789012:assumed-role/MyRole/session-1", "session-1"},
		{"arn:aws:iam::123456789012:role/path/MyRole", "MyRole"},
		{"arn:aws:iam::123456789012:root", "arn:aws:iam::123456789012:root"},
	} {
		t.Run(tc.arn, func(t *testing.T) {
			f := &fakeAWS{arn: tc.arn, policiesFor: tc.user}
			results, err := run(t, newEnv(t, f))
			if err != nil {
				t.Fatal(err)
			}
			if len(f.users) != 2 || f.users[0] != tc.user || f.users[1] != tc.user {
				t.Errorf("IAM calls used user names %v, want %q twice", f.users, tc.user)
			}
			if len(results) != 5 {
				t.Fatalf("got %d results, want 5 (identity + 4 policies): %+v", len(results), results)
			}
			for _, r := range results[1:] {
				if r.Details["user"] != tc.user {
					t.Errorf("details user = %v, want %q", r.Details["user"], tc.user)
				}
			}
		})
	}
}

func TestProcessNilAndMissingInputsDoNotPanic(t *testing.T) {
	p := STSCalls[0].Process
	var nilOut *struct{}
	for name, out := range map[string]interface{}{
		"nil":        nil,
		"wrong type": "x",
		"nil output": map[string]interface{}{"output": nil, "ctx": context.Background()},
		"empty map":  map[string]interface{}{},
		"no cfg":     map[string]interface{}{"output": nilOut},
	} {
		t.Run(name, func(t *testing.T) {
			if got := p(out, nil, false); len(got) != 0 {
				t.Errorf("results = %+v", got)
			}
		})
	}

	// No aws.Config in the output: identity is still reported, IAM is skipped.
	ctx := newEnv(t, &fakeAWS{arn: "arn:aws:iam::123456789012:user/alice", policiesFor: "alice"})
	cfg, _ := types.AWSConfigFrom(ctx)
	out, err := STSCalls[0].CallV2(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]interface{})
	delete(m, "cfg")
	if got := p(m, nil, false); len(got) != 1 || got[0].ResourceType != "identity" {
		t.Errorf("results without cfg = %+v", got)
	}
}
