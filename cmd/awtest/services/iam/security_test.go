package iam

import (
	"fmt"
	"net/url"
	"testing"
)

func TestDepthSimpleLists(t *testing.T) {
	_, ctx := newFakeIAM(t, func(action string, form url.Values) (int, string) {
		switch action {
		case "ListInstanceProfiles":
			return 200, xmlResp(action, `<InstanceProfiles><member><Path>/</Path><InstanceProfileName>web-profile</InstanceProfileName><InstanceProfileId>AIPA</InstanceProfileId><Arn>arn:aws:iam::111111111111:instance-profile/web-profile</Arn><CreateDate>2024-01-02T03:04:05Z</CreateDate><Roles><member><Path>/</Path><RoleName>web</RoleName><RoleId>AROA</RoleId><Arn>arn:aws:iam::111111111111:role/web</Arn><CreateDate>2024-01-02T03:04:05Z</CreateDate></member></Roles></member></InstanceProfiles><IsTruncated>false</IsTruncated>`)
		case "ListOpenIDConnectProviders":
			return 200, xmlResp(action, `<OpenIDConnectProviderList><member><Arn>arn:aws:iam::111111111111:oidc-provider/example.com</Arn></member></OpenIDConnectProviderList>`)
		case "ListSAMLProviders":
			return 200, xmlResp(action, `<SAMLProviderList><member><Arn>arn:aws:iam::111111111111:saml-provider/corp</Arn><ValidUntil>2030-01-01T00:00:00Z</ValidUntil><CreateDate>2024-01-02T03:04:05Z</CreateDate></member></SAMLProviderList>`)
		case "GetAccountSummary":
			return 200, xmlResp(action, `<SummaryMap><entry><key>Users</key><value>4</value></entry><entry><key>AccountMFAEnabled</key><value>0</value></entry></SummaryMap>`)
		}
		return 400, fmt.Sprintf(awsErrXML, "InvalidAction")
	})

	ip := run(t, "iam:ListInstanceProfiles", ctx)
	if len(ip) != 1 || ip[0].ResourceName != "web-profile" || ip[0].ResourceType != "instance-profile" {
		t.Fatalf("instance profiles = %+v", ip)
	}
	if roles, _ := ip[0].Details["roles"].([]string); len(roles) != 1 || roles[0] != "web" {
		t.Errorf("roles = %v", ip[0].Details["roles"])
	}
	if r := run(t, "iam:ListOpenIDConnectProviders", ctx); len(r) != 1 || r[0].ResourceName != "arn:aws:iam::111111111111:oidc-provider/example.com" {
		t.Errorf("oidc = %+v", r)
	}
	if r := run(t, "iam:ListSAMLProviders", ctx); len(r) != 1 || r[0].ResourceType != "saml-provider" || r[0].Details["valid_until"] != "2030-01-01T00:00:00Z" {
		t.Errorf("saml = %+v", r)
	}
	sum := run(t, "iam:GetAccountSummary", ctx)
	if len(sum) != 1 || sum[0].Details["Users"] != int64(4) || sum[0].Details["AccountMFAEnabled"] != int64(0) {
		t.Errorf("summary = %+v", sum)
	}
}

func TestPasswordPolicySoftResults(t *testing.T) {
	for _, tc := range []struct{ code, status string }{{"NoSuchEntity", "none"}, {"AccessDenied", "denied"}} {
		_, ctx := newFakeIAM(t, func(string, url.Values) (int, string) {
			return 404, fmt.Sprintf(awsErrXML, tc.code)
		})
		res := run(t, "iam:GetAccountPasswordPolicy", ctx)
		if len(res) != 1 || res[0].Error != nil || res[0].Details["status"] != tc.status {
			t.Errorf("%s: results = %+v", tc.code, res)
		}
	}
	_, ctx := newFakeIAM(t, func(string, url.Values) (int, string) {
		return 500, fmt.Sprintf(awsErrXML, "ServiceFailure")
	})
	if res := run(t, "iam:GetAccountPasswordPolicy", ctx); len(res) != 1 || res[0].Error == nil {
		t.Errorf("hard failure must stay an error: %+v", res)
	}

	_, ctx = newFakeIAM(t, func(action string, _ url.Values) (int, string) {
		return 200, xmlResp(action, `<PasswordPolicy><MinimumPasswordLength>14</MinimumPasswordLength><RequireSymbols>true</RequireSymbols><MaxPasswordAge>90</MaxPasswordAge><ExpirePasswords>true</ExpirePasswords></PasswordPolicy>`)
	})
	res := run(t, "iam:GetAccountPasswordPolicy", ctx)
	if len(res) != 1 || res[0].Details["status"] != "configured" || res[0].Details["minimum_password_length"] != int64(14) ||
		res[0].Details["require_symbols"] != true || res[0].Details["max_password_age"] != int64(90) {
		t.Errorf("configured = %+v", res)
	}
	if _, ok := res[0].Details["password_reuse_prevention"]; ok {
		t.Errorf("absent field must stay absent: %+v", res[0].Details)
	}
}

func TestListMFADevices(t *testing.T) {
	fake, ctx := newFakeIAM(t, func(action string, form url.Values) (int, string) {
		switch action {
		case "ListUsers":
			return 200, xmlResp(action, `<Users>`+userXML("alice")+userXML("bob")+userXML("carol")+`</Users><IsTruncated>false</IsTruncated>`)
		case "ListMFADevices":
			if form.Get("UserName") == "alice" {
				return 200, xmlResp(action, `<MFADevices><member><UserName>alice</UserName><SerialNumber>arn:aws:iam::111111111111:mfa/alice</SerialNumber><EnableDate>2024-01-02T03:04:05Z</EnableDate></member></MFADevices><IsTruncated>false</IsTruncated>`)
			}
			return 403, fmt.Sprintf(awsErrXML, "AccessDenied")
		}
		return 400, fmt.Sprintf(awsErrXML, "InvalidAction")
	})
	res := run(t, "iam:ListMFADevices", ctx)
	var devices, denied, notes int
	for _, r := range res {
		switch {
		case r.Error != nil:
			denied++
		case r.ResourceType == "mfa-device":
			devices++
			if r.ResourceName != "alice/arn:aws:iam::111111111111:mfa/alice" || r.Details["user"] != "alice" {
				t.Errorf("device = %+v", r)
			}
		case r.ResourceName == "followups-skipped":
			notes++
			if r.Details["skipped"] != 1 {
				t.Errorf("skipped = %v", r.Details["skipped"])
			}
		}
	}
	// bob is denied, which short-circuits carol.
	if devices != 1 || denied != 1 || notes != 1 || fake.count("ListMFADevices") != 2 {
		t.Errorf("devices=%d denied=%d notes=%d calls=%d: %+v", devices, denied, notes, fake.count("ListMFADevices"), res)
	}
}

func TestListMFADevicesListUsersError(t *testing.T) {
	_, ctx := newFakeIAM(t, func(string, url.Values) (int, string) {
		return 403, fmt.Sprintf(awsErrXML, "AccessDenied")
	})
	res := run(t, "iam:ListMFADevices", ctx)
	if len(res) != 1 || res[0].MethodName != "iam:ListMFADevices" || res[0].Error == nil {
		t.Errorf("results = %+v", res)
	}
}
