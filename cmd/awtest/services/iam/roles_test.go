package iam

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

type fakeIAM struct {
	mu      sync.Mutex
	actions map[string]int
	forms   map[string]url.Values
	handler func(action string, form url.Values) (int, string)
}

func (f *fakeIAM) count(a string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.actions[a]
}

// newFakeIAM starts a fake IAM endpoint and returns a context carrying a v2
// config that points at it (and only at it: the endpoint is explicit and the
// transport is the fake server's client).
func newFakeIAM(t *testing.T, h func(action string, form url.Values) (int, string)) (*fakeIAM, context.Context) {
	t.Helper()
	fake := &fakeIAM{actions: map[string]int{}, forms: map[string]url.Values{}, handler: h}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		action := r.Form.Get("Action")
		fake.mu.Lock()
		fake.actions[action]++
		fake.forms[action] = r.Form
		fake.mu.Unlock()
		code, body := h(action, r.Form)
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	t.Setenv("HOME", t.TempDir())
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	for _, k := range []string{"AWS_PROFILE", "AWS_DEFAULT_PROFILE", "AWS_SHARED_CREDENTIALS_FILE", "AWS_CONFIG_FILE", "AWS_REGION", "AWS_DEFAULT_REGION"} {
		t.Setenv(k, "")
	}
	cfg, err := awsv2.LoadConfig(context.Background(), awsv2.Options{
		Region:          "us-east-1",
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
	return fake, types.WithAWSConfig(context.Background(), cfg)
}

func findCall(t *testing.T, name string) types.AWSService {
	t.Helper()
	n := 0
	var found types.AWSService
	for _, c := range IAMCalls {
		if c.Name == name {
			found = c
			n++
		}
	}
	if n != 1 {
		t.Fatalf("expected exactly one IAMCalls entry named %s, found %d", name, n)
	}
	return found
}

func run(t *testing.T, name string, ctx context.Context) []types.ScanResult {
	t.Helper()
	utils.Quiet = true
	t.Cleanup(func() { utils.Quiet = false })
	c := findCall(t, name)
	out, err := c.Invoke(ctx, nil)
	return c.Process(out, err, false)
}

func xmlResp(action, inner string) string {
	return fmt.Sprintf(`<%[1]sResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"><%[1]sResult>%[2]s</%[1]sResult><ResponseMetadata><RequestId>r</RequestId></ResponseMetadata></%[1]sResponse>`, action, inner)
}

const awsErrXML = `<ErrorResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"><Error><Type>Sender</Type><Code>%s</Code><Message>nope</Message></Error><RequestId>r</RequestId></ErrorResponse>`

func roleXML(name, trust string) string {
	return fmt.Sprintf(`<member><Path>/</Path><RoleName>%s</RoleName><RoleId>AROAX</RoleId><Arn>arn:aws:iam::111111111111:role/%[1]s</Arn><CreateDate>2024-01-02T03:04:05Z</CreateDate><MaxSessionDuration>3600</MaxSessionDuration><Description>d-%[1]s</Description><AssumeRolePolicyDocument>%s</AssumeRolePolicyDocument></member>`, name, trust)
}

func byMethod(rs []types.ScanResult, method string) []types.ScanResult {
	var out []types.ScanResult
	for _, r := range rs {
		if r.MethodName == method {
			out = append(out, r)
		}
	}
	return out
}

func TestListRolesFunctional(t *testing.T) {
	trust1 := url.QueryEscape(`{"Statement":[{"Effect":"Allow","Principal":{"Service":"ec2.amazonaws.com"}},{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::222222222222:root"}}]}`)
	fake, ctx := newFakeIAM(t, func(action string, form url.Values) (int, string) {
		switch action {
		case "ListRoles":
			return 200, xmlResp(action, `<Roles>`+roleXML("web", trust1)+roleXML("broken", "%zz")+`</Roles><IsTruncated>false</IsTruncated>`)
		case "ListAttachedRolePolicies":
			if form.Get("RoleName") == "web" {
				return 200, xmlResp(action, `<AttachedPolicies><member><PolicyName>P1</PolicyName><PolicyArn>arn:aws:iam::aws:policy/P1</PolicyArn></member></AttachedPolicies><IsTruncated>false</IsTruncated>`)
			}
			return 200, xmlResp(action, `<AttachedPolicies/><IsTruncated>false</IsTruncated>`)
		case "ListRolePolicies":
			if form.Get("RoleName") == "web" {
				return 200, xmlResp(action, `<PolicyNames><member>inline1</member></PolicyNames><IsTruncated>false</IsTruncated>`)
			}
			return 200, xmlResp(action, `<PolicyNames/><IsTruncated>false</IsTruncated>`)
		}
		return 400, fmt.Sprintf(awsErrXML, "InvalidAction")
	})

	res := run(t, "iam:ListRoles", ctx)
	roles := byMethod(res, "iam:ListRoles")
	if len(roles) != 2 {
		t.Fatalf("want 2 role results, got %d: %+v", len(roles), res)
	}
	web := roles[0]
	if web.ResourceType != "role" || web.ResourceName != "web" || web.Error != nil {
		t.Errorf("bad role result: %+v", web)
	}
	d := web.Details
	if d["arn"] != "arn:aws:iam::111111111111:role/web" || d["path"] != "/" || d["create_date"] != "2024-01-02T03:04:05Z" ||
		d["max_session_duration"] != int64(3600) || d["description"] != "d-web" {
		t.Errorf("bad details: %+v", d)
	}
	if got := fmt.Sprint(d["trusted_principals"]); got != "[Service:ec2.amazonaws.com AWS:arn:aws:iam::222222222222:root]" {
		t.Errorf("trusted_principals = %s", got)
	}
	if got := fmt.Sprint(d["trust_findings"]); got != "[cross-account principal: 222222222222 (role account is 111111111111)]" {
		t.Errorf("trust_findings = %s", got)
	}
	if _, ok := roles[1].Details["trust_policy_parse_error"]; !ok || roles[1].Error != nil {
		t.Errorf("broken role should carry trust_policy_parse_error and no Error: %+v", roles[1])
	}
	if att := byMethod(res, "iam:ListAttachedRolePolicies"); len(att) != 1 || att[0].ResourceName != "web/P1" || att[0].Details["role"] != "web" || att[0].Details["policy_arn"] != "arn:aws:iam::aws:policy/P1" {
		t.Errorf("attached follow-up: %+v", att)
	}
	if in := byMethod(res, "iam:ListRolePolicies"); len(in) != 1 || in[0].ResourceName != "web/inline1" {
		t.Errorf("inline follow-up: %+v", in)
	}
	for _, name := range []string{"GetRole", "GetAccountAuthorizationDetails"} {
		if fake.count(name) != 0 {
			t.Errorf("unexpected call %s", name)
		}
	}
}

func TestListRolesCapsAndFollowupBound(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < maxRoles+5; i++ {
		sb.WriteString(roleXML(fmt.Sprintf("r%04d", i), ""))
	}
	fake, ctx := newFakeIAM(t, func(action string, form url.Values) (int, string) {
		switch action {
		case "ListRoles":
			return 200, xmlResp(action, `<Roles>`+sb.String()+`</Roles><IsTruncated>false</IsTruncated>`)
		case "ListAttachedRolePolicies":
			return 200, xmlResp(action, `<AttachedPolicies/><IsTruncated>false</IsTruncated>`)
		case "ListRolePolicies":
			return 200, xmlResp(action, `<PolicyNames/><IsTruncated>false</IsTruncated>`)
		}
		return 400, fmt.Sprintf(awsErrXML, "InvalidAction")
	})
	res := run(t, "iam:ListRoles", ctx)
	roleCount, notes := 0, map[string]types.ScanResult{}
	for _, r := range res {
		if r.ResourceType == "role" {
			roleCount++
		}
		if r.ResourceType == "note" {
			notes[r.ResourceName] = r
		}
	}
	if roleCount != maxRoles {
		t.Errorf("roles = %d, want %d", roleCount, maxRoles)
	}
	if n, ok := notes["list-truncated"]; !ok || n.Details["truncated"] != true || n.Details["limit"] != maxRoles {
		t.Errorf("list-truncated note: %+v", notes)
	}
	n, ok := notes["followups-truncated"]
	if !ok || n.Details["truncated"] != true || n.Details["roles_total"] != maxRoles || n.Details["roles_followed_up"] != maxRoleFollowups {
		t.Errorf("followups-truncated note: %+v", n)
	}
	if c := fake.count("ListAttachedRolePolicies"); c != maxRoleFollowups {
		t.Errorf("ListAttachedRolePolicies calls = %d, want %d", c, maxRoleFollowups)
	}
	if c := fake.count("ListRolePolicies"); c != maxRoleFollowups {
		t.Errorf("ListRolePolicies calls = %d, want %d", c, maxRoleFollowups)
	}
}

func TestListRolesFollowupDeniedShortCircuit(t *testing.T) {
	fake, ctx := newFakeIAM(t, func(action string, form url.Values) (int, string) {
		switch action {
		case "ListRoles":
			return 200, xmlResp(action, `<Roles>`+roleXML("a", "")+roleXML("b", "")+roleXML("c", "")+`</Roles><IsTruncated>false</IsTruncated>`)
		case "ListAttachedRolePolicies":
			return 403, fmt.Sprintf(awsErrXML, "AccessDenied")
		case "ListRolePolicies":
			return 200, xmlResp(action, `<PolicyNames><member>p</member></PolicyNames><IsTruncated>false</IsTruncated>`)
		}
		return 400, fmt.Sprintf(awsErrXML, "InvalidAction")
	})
	res := run(t, "iam:ListRoles", ctx)
	if c := fake.count("ListAttachedRolePolicies"); c != 1 {
		t.Errorf("denied follow-up should stop after first call, got %d calls", c)
	}
	if c := fake.count("ListRolePolicies"); c != 3 {
		t.Errorf("allowed follow-up should still run for all roles, got %d", c)
	}
	errs := 0
	var note *types.ScanResult
	for i, r := range res {
		if r.MethodName == "iam:ListAttachedRolePolicies" {
			if r.Error != nil {
				errs++
			}
			if r.ResourceType == "note" {
				note = &res[i]
			}
		}
	}
	if errs != 1 {
		t.Errorf("want exactly 1 error result for denied step, got %d", errs)
	}
	if note == nil || note.Details["reason"] != "AccessDenied" || note.Details["skipped"] != 2 {
		t.Errorf("skip note: %+v", note)
	}
}

func TestListRolesError(t *testing.T) {
	_, ctx := newFakeIAM(t, func(string, url.Values) (int, string) {
		return 403, fmt.Sprintf(awsErrXML, "AccessDenied")
	})
	res := run(t, "iam:ListRoles", ctx)
	if len(res) != 1 || res[0].Error == nil || res[0].MethodName != "iam:ListRoles" {
		t.Errorf("want single error result, got %+v", res)
	}
}

func TestListGroupsFunctional(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < maxGroupFollowups+3; i++ {
		fmt.Fprintf(&sb, `<member><Path>/</Path><GroupName>g%03d</GroupName><GroupId>AGPA</GroupId><Arn>arn:aws:iam::111111111111:group/g%03d</Arn><CreateDate>2024-01-02T03:04:05Z</CreateDate></member>`, i, i)
	}
	fake, ctx := newFakeIAM(t, func(action string, form url.Values) (int, string) {
		switch action {
		case "ListGroups":
			return 200, xmlResp(action, `<Groups>`+sb.String()+`</Groups><IsTruncated>false</IsTruncated>`)
		case "ListAttachedGroupPolicies":
			if form.Get("GroupName") == "g000" {
				return 200, xmlResp(action, `<AttachedPolicies><member><PolicyName>GP</PolicyName><PolicyArn>arn:aws:iam::aws:policy/GP</PolicyArn></member></AttachedPolicies><IsTruncated>false</IsTruncated>`)
			}
			return 200, xmlResp(action, `<AttachedPolicies/><IsTruncated>false</IsTruncated>`)
		case "ListGroupPolicies":
			return 403, fmt.Sprintf(awsErrXML, "AccessDenied")
		}
		return 400, fmt.Sprintf(awsErrXML, "InvalidAction")
	})
	res := run(t, "iam:ListGroups", ctx)
	groups := byMethod(res, "iam:ListGroups")
	if len(groups) != maxGroupFollowups+3+1 { // + followups-truncated note
		t.Errorf("iam:ListGroups results = %d", len(groups))
	}
	if groups[0].ResourceType != "group" || groups[0].Details["arn"] != "arn:aws:iam::111111111111:group/g000" {
		t.Errorf("bad group: %+v", groups[0])
	}
	if c := fake.count("ListAttachedGroupPolicies"); c != maxGroupFollowups {
		t.Errorf("attached calls = %d", c)
	}
	if c := fake.count("ListGroupPolicies"); c != 1 {
		t.Errorf("denied inline calls = %d, want 1", c)
	}
	if att := byMethod(res, "iam:ListAttachedGroupPolicies"); len(att) != 1 || att[0].Details["group"] != "g000" {
		t.Errorf("attached results: %+v", att)
	}
	last := groups[len(groups)-1]
	if last.ResourceType != "note" || last.Details["groups_total"] != maxGroupFollowups+3 || last.Details["groups_followed_up"] != maxGroupFollowups {
		t.Errorf("truncation note: %+v", last)
	}
}

func TestListPoliciesFunctional(t *testing.T) {
	fake, ctx := newFakeIAM(t, func(action string, form url.Values) (int, string) {
		if action != "ListPolicies" {
			return 400, fmt.Sprintf(awsErrXML, "InvalidAction")
		}
		return 200, xmlResp(action, `<Policies><member><PolicyName>mine</PolicyName><PolicyId>ANPA</PolicyId><Arn>arn:aws:iam::111111111111:policy/mine</Arn><Path>/</Path><DefaultVersionId>v2</DefaultVersionId><AttachmentCount>3</AttachmentCount><IsAttachable>true</IsAttachable><CreateDate>2024-01-02T03:04:05Z</CreateDate><UpdateDate>2024-02-02T03:04:05Z</UpdateDate></member></Policies><IsTruncated>false</IsTruncated>`)
	})
	res := run(t, "iam:ListPolicies", ctx)
	form := fake.forms["ListPolicies"]
	if form.Get("Scope") != "Local" || form.Get("OnlyAttached") != "true" {
		t.Errorf("request form: %v", form)
	}
	if len(res) != 1 || res[0].ResourceType != "policy" || res[0].ResourceName != "mine" || res[0].Error != nil {
		t.Fatalf("results: %+v", res)
	}
	d := res[0].Details
	if d["attachment_count"] != int64(3) || d["default_version_id"] != "v2" || d["is_attachable"] != true ||
		d["create_date"] != "2024-01-02T03:04:05Z" || d["update_date"] != "2024-02-02T03:04:05Z" || d["arn"] != "arn:aws:iam::111111111111:policy/mine" {
		t.Errorf("details: %+v", d)
	}
}

func TestNewEntriesAreGlobalAndReadOnly(t *testing.T) {
	for _, n := range []string{"iam:ListRoles", "iam:ListGroups", "iam:ListPolicies"} {
		c := findCall(t, n)
		if c.CallV2 == nil || c.Process == nil || c.ModuleName == "" {
			t.Errorf("%s incomplete", n)
		}
	}
}
