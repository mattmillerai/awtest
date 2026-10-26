package iam

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

func TestServicesAreV2Only(t *testing.T) {
	want := []string{"iam:ListUsers", "iam:ListRoles", "iam:ListGroups", "iam:ListPolicies"}
	if len(IAMCalls) != len(want) {
		t.Fatalf("got %d services, want %d", len(IAMCalls), len(want))
	}
	for i, svc := range IAMCalls {
		if svc.Name != want[i] {
			t.Errorf("service %d = %q, want %q", i, svc.Name, want[i])
		}
		if svc.CallV2 == nil || svc.Call != nil {
			t.Errorf("%s must implement CallV2 only", svc.Name)
		}
	}
}

func userXML(name string) string {
	return fmt.Sprintf(`<member><Path>/</Path><UserName>%s</UserName><UserId>AIDAX</UserId><Arn>arn:aws:iam::111111111111:user/%[1]s</Arn><CreateDate>2024-01-02T03:04:05Z</CreateDate></member>`, name)
}

func TestListUsersFunctional(t *testing.T) {
	fake, ctx := newFakeIAM(t, func(action string, form url.Values) (int, string) {
		user := form.Get("UserName")
		switch action {
		case "ListUsers":
			// IsTruncated is ignored: ListUsers is a single call (v1 behaviour).
			return 200, xmlResp(action, `<Users>`+userXML("alice")+userXML("bob")+`</Users><IsTruncated>true</IsTruncated><Marker>m</Marker>`)
		case "ListGroupsForUser":
			if user == "alice" {
				return 200, xmlResp(action, `<Groups><member><Path>/</Path><GroupName>admins</GroupName><GroupId>AGPA</GroupId><Arn>arn:aws:iam::111111111111:group/admins</Arn><CreateDate>2024-01-02T03:04:05Z</CreateDate></member><member><Path>/</Path></member></Groups><IsTruncated>false</IsTruncated>`)
			}
			return 200, xmlResp(action, `<Groups/><IsTruncated>false</IsTruncated>`)
		case "ListAttachedUserPolicies":
			if user == "alice" {
				return 200, xmlResp(action, `<AttachedPolicies><member><PolicyName>ReadOnly</PolicyName><PolicyArn>arn:aws:iam::aws:policy/ReadOnly</PolicyArn></member><member/></AttachedPolicies><IsTruncated>false</IsTruncated>`)
			}
			return 403, fmt.Sprintf(awsErrXML, "AccessDenied")
		case "ListUserPolicies":
			if user == "alice" {
				return 200, xmlResp(action, `<PolicyNames><member>inline1</member></PolicyNames><IsTruncated>false</IsTruncated>`)
			}
			return 200, xmlResp(action, `<PolicyNames/><IsTruncated>false</IsTruncated>`)
		case "ListAccessKeys":
			if user == "alice" {
				return 200, xmlResp(action, `<AccessKeyMetadata><member><UserName>alice</UserName><AccessKeyId>AKIAALICE</AccessKeyId><Status>Active</Status></member><member><UserName>alice</UserName></member></AccessKeyMetadata><IsTruncated>false</IsTruncated>`)
			}
			return 200, xmlResp(action, `<AccessKeyMetadata/><IsTruncated>false</IsTruncated>`)
		}
		return 400, fmt.Sprintf(awsErrXML, "InvalidAction")
	})

	res := run(t, "iam:ListUsers", ctx)
	if n := fake.count("ListUsers"); n != 1 {
		t.Errorf("ListUsers calls = %d, want 1", n)
	}
	users := byMethod(res, "iam:ListUsers")
	if len(users) != 2 || users[0].ResourceName != "alice" || users[1].ResourceName != "bob" ||
		users[0].ResourceType != "user" || users[0].ServiceName != "IAM" || !reflect.DeepEqual(users[0].Details, map[string]interface{}{}) {
		t.Fatalf("users = %+v", users)
	}

	type want struct{ rtype, name string }
	collect := func(method string) (got []want) {
		for _, r := range byMethod(res, method) {
			if r.Error == nil {
				got = append(got, want{r.ResourceType, r.ResourceName})
				if !reflect.DeepEqual(r.Details, map[string]interface{}{"user": "alice"}) {
					t.Errorf("%s details = %#v", method, r.Details)
				}
			}
		}
		return got
	}
	// The second entry of each list is sparse: a nil name reads as "".
	if got := collect("iam:ListGroupsForUser"); !reflect.DeepEqual(got, []want{{"group", "admins"}, {"group", ""}}) {
		t.Errorf("groups = %+v", got)
	}
	if got := collect("iam:ListAttachedUserPolicies"); !reflect.DeepEqual(got, []want{{"attached-policy", "ReadOnly"}, {"attached-policy", ""}}) {
		t.Errorf("attached = %+v", got)
	}
	if got := collect("iam:ListUserPolicies"); !reflect.DeepEqual(got, []want{{"inline-policy", "inline1"}}) {
		t.Errorf("inline = %+v", got)
	}
	if got := collect("iam:ListAccessKeys"); !reflect.DeepEqual(got, []want{{"access-key", "AKIAALICE"}, {"access-key", ""}}) {
		t.Errorf("keys = %+v", got)
	}

	// bob's denied follow-up is reported as a coded, normalised error and does
	// not stop the remaining follow-ups.
	var denied []types.ScanResult
	for _, r := range byMethod(res, "iam:ListAttachedUserPolicies") {
		if r.Error != nil {
			denied = append(denied, r)
		}
	}
	if len(denied) != 1 || denied[0].ResourceName != "bob" || denied[0].ResourceType != "user" || denied[0].Error.Error() != "AccessDenied: nope" {
		t.Errorf("denied = %+v", denied)
	}
	if fake.count("ListUserPolicies") != 2 || fake.count("ListAccessKeys") != 2 {
		t.Errorf("follow-ups must run for both users: %v", fake.actions)
	}
}

func TestListUsersError(t *testing.T) {
	_, ctx := newFakeIAM(t, func(string, url.Values) (int, string) {
		return 403, fmt.Sprintf(awsErrXML, "AccessDenied")
	})
	res := run(t, "iam:ListUsers", ctx)
	if len(res) != 1 || res[0].MethodName != "iam:ListUsers" || res[0].Error == nil || res[0].Error.Error() != "AccessDenied: nope" {
		t.Errorf("results = %+v", res)
	}
}

func rolePage(names []string, marker string) string {
	var sb strings.Builder
	for _, n := range names {
		sb.WriteString(roleXML(n, ""))
	}
	if marker != "" {
		return `<Roles>` + sb.String() + `</Roles><IsTruncated>true</IsTruncated><Marker>` + marker + `</Marker>`
	}
	return `<Roles>` + sb.String() + `</Roles><IsTruncated>false</IsTruncated>`
}

func emptyFollowups(action string) (int, string) {
	switch action {
	case "ListAttachedRolePolicies":
		return 200, xmlResp(action, `<AttachedPolicies/><IsTruncated>false</IsTruncated>`)
	case "ListRolePolicies":
		return 200, xmlResp(action, `<PolicyNames/><IsTruncated>false</IsTruncated>`)
	}
	return 400, fmt.Sprintf(awsErrXML, "InvalidAction")
}

func TestListRolesFollowsMarkerAcrossPages(t *testing.T) {
	fake, ctx := newFakeIAM(t, func(action string, form url.Values) (int, string) {
		if action != "ListRoles" {
			return emptyFollowups(action)
		}
		if form.Get("Marker") == "m2" {
			return 200, xmlResp(action, rolePage([]string{"c"}, ""))
		}
		return 200, xmlResp(action, rolePage([]string{"a", "b"}, "m2"))
	})
	res := run(t, "iam:ListRoles", ctx)
	var names []string
	for _, r := range byMethod(res, "iam:ListRoles") {
		names = append(names, r.ResourceName)
	}
	if !reflect.DeepEqual(names, []string{"a", "b", "c"}) {
		t.Errorf("roles = %v, want all pages", names)
	}
	if n := fake.count("ListRoles"); n != 2 {
		t.Errorf("ListRoles calls = %d, want 2", n)
	}
}

func TestListRolesCapStopsPaging(t *testing.T) {
	var page []string
	for i := 0; i < maxRoles; i++ {
		page = append(page, fmt.Sprintf("r%04d", i))
	}
	fake, ctx := newFakeIAM(t, func(action string, form url.Values) (int, string) {
		if action != "ListRoles" {
			return emptyFollowups(action)
		}
		// A full page that claims another page follows: the cap must stop the
		// listing and be noted without requesting that page.
		return 200, xmlResp(action, rolePage(page, "more"))
	})
	res := run(t, "iam:ListRoles", ctx)
	if n := fake.count("ListRoles"); n != 1 {
		t.Errorf("ListRoles calls = %d, want 1", n)
	}
	var note *types.ScanResult
	for i, r := range res {
		if r.ResourceName == "list-truncated" {
			note = &res[i]
		}
	}
	if note == nil || note.Details["truncated"] != true || note.Details["limit"] != maxRoles {
		t.Errorf("truncation note = %+v", note)
	}
}

func TestListPoliciesAndGroupsFollowMarker(t *testing.T) {
	fake, ctx := newFakeIAM(t, func(action string, form url.Values) (int, string) {
		switch action {
		case "ListPolicies":
			if form.Get("Marker") == "p2" {
				return 200, xmlResp(action, `<Policies><member><PolicyName>two</PolicyName></member></Policies><IsTruncated>false</IsTruncated>`)
			}
			return 200, xmlResp(action, `<Policies><member><PolicyName>one</PolicyName></member></Policies><IsTruncated>true</IsTruncated><Marker>p2</Marker>`)
		case "ListGroups":
			if form.Get("Marker") == "g2" {
				return 200, xmlResp(action, `<Groups><member><GroupName>g-two</GroupName></member></Groups><IsTruncated>false</IsTruncated>`)
			}
			return 200, xmlResp(action, `<Groups><member><GroupName>g-one</GroupName></member></Groups><IsTruncated>true</IsTruncated><Marker>g2</Marker>`)
		case "ListAttachedGroupPolicies":
			return 200, xmlResp(action, `<AttachedPolicies/><IsTruncated>false</IsTruncated>`)
		case "ListGroupPolicies":
			return 200, xmlResp(action, `<PolicyNames/><IsTruncated>false</IsTruncated>`)
		}
		return 400, fmt.Sprintf(awsErrXML, "InvalidAction")
	})
	pol := run(t, "iam:ListPolicies", ctx)
	if len(pol) != 2 || pol[0].ResourceName != "one" || pol[1].ResourceName != "two" || fake.count("ListPolicies") != 2 {
		t.Errorf("policies = %+v", pol)
	}
	// A policy without optional fields: attachment_count stays absent (a
	// nullable value) while is_attachable (a plain bool in v2) is always set.
	if _, ok := pol[0].Details["attachment_count"]; ok {
		t.Errorf("attachment_count must be absent: %+v", pol[0].Details)
	}
	if v, ok := pol[0].Details["is_attachable"]; !ok || v != false {
		t.Errorf("is_attachable = %v (present %v), want false", v, ok)
	}
	grp := run(t, "iam:ListGroups", ctx)
	var names []string
	for _, r := range byMethod(grp, "iam:ListGroups") {
		names = append(names, r.ResourceName)
	}
	if !reflect.DeepEqual(names, []string{"g-one", "g-two"}) || fake.count("ListGroups") != 2 {
		t.Errorf("groups = %v", names)
	}
}
