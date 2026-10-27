package iam

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

const (
	// maxInstanceProfiles caps the profiles returned by iam:ListInstanceProfiles.
	maxInstanceProfiles = 1000
	// maxMFAUsers caps the users listed for iam:ListMFADevices.
	maxMFAUsers = 1000
	// maxMFAFollowups is the number of users that get a ListMFADevices call.
	maxMFAFollowups = 100
)

// Global like the other IAM entries: no region scope, one run per scan.
func init() {
	IAMCalls = append(IAMCalls, listInstanceProfilesCall, listOIDCProvidersCall, listSAMLProvidersCall,
		getAccountSummaryCall, getAccountPasswordPolicyCall, listMFADevicesCall)
}

var listInstanceProfilesCall = types.AWSService{
	Name: "iam:ListInstanceProfiles",
	CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
		svc := awsv2.NewRegionalClient(cfg, "", iam.NewFromConfig)
		res := &listResult{}
		var items []iamtypes.InstanceProfile
		var err error
		pager := iam.NewListInstanceProfilesPaginator(svc, &iam.ListInstanceProfilesInput{})
	pages:
		for pager.HasMorePages() {
			var page *iam.ListInstanceProfilesOutput
			page, err = pager.NextPage(ctx)
			if err != nil {
				break
			}
			for _, p := range page.InstanceProfiles {
				if len(items) >= maxInstanceProfiles {
					res.Truncated = true
					break pages
				}
				items = append(items, p)
			}
			if len(items) >= maxInstanceProfiles && pager.HasMorePages() {
				res.Truncated = true
				break
			}
		}
		res.Items = items
		return map[string]interface{}{"output": res, "cfg": cfg, "ctx": ctx}, err
	},
	Process: func(output interface{}, err error, debug bool) []types.ScanResult {
		if err != nil {
			utils.HandleAWSError(debug, "iam:ListInstanceProfiles", err)
			return errResult("iam:ListInstanceProfiles", err)
		}
		lr, _, _, _ := unpackList(output)
		if lr == nil {
			return nil
		}
		profiles, _ := lr.Items.([]iamtypes.InstanceProfile)
		var results []types.ScanResult
		for _, p := range profiles {
			name := str(p.InstanceProfileName)
			roles := []string{}
			for _, r := range p.Roles {
				roles = append(roles, str(r.RoleName))
			}
			results = append(results, types.ScanResult{
				ServiceName:  "IAM",
				MethodName:   "iam:ListInstanceProfiles",
				ResourceType: "instance-profile",
				ResourceName: name,
				Details: map[string]interface{}{
					"arn":         str(p.Arn),
					"path":        str(p.Path),
					"create_date": timeStr(p.CreateDate),
					"roles":       roles,
				},
				Timestamp: time.Now(),
			})
			utils.PrintResult(debug, "", "iam:ListInstanceProfiles", fmt.Sprintf("IAM instance profile: %s", utils.ColorizeItem(name)), nil)
		}
		if lr.Truncated {
			results = append(results, listTruncatedNote(debug, "iam:ListInstanceProfiles", "instance profile", maxInstanceProfiles))
		}
		return results
	},
	ModuleName: types.DefaultModuleName,
}

var listOIDCProvidersCall = types.AWSService{
	Name: "iam:ListOpenIDConnectProviders",
	CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
		svc := awsv2.NewRegionalClient(cfg, "", iam.NewFromConfig)
		return svc.ListOpenIDConnectProviders(ctx, &iam.ListOpenIDConnectProvidersInput{})
	},
	Process: func(output interface{}, err error, debug bool) []types.ScanResult {
		if err != nil {
			utils.HandleAWSError(debug, "iam:ListOpenIDConnectProviders", err)
			return errResult("iam:ListOpenIDConnectProviders", err)
		}
		out, _ := output.(*iam.ListOpenIDConnectProvidersOutput)
		if out == nil {
			return nil
		}
		var results []types.ScanResult
		for _, p := range out.OpenIDConnectProviderList {
			arn := str(p.Arn)
			results = append(results, types.ScanResult{
				ServiceName:  "IAM",
				MethodName:   "iam:ListOpenIDConnectProviders",
				ResourceType: "oidc-provider",
				ResourceName: arn,
				Details:      map[string]interface{}{},
				Timestamp:    time.Now(),
			})
			utils.PrintResult(debug, "", "iam:ListOpenIDConnectProviders", fmt.Sprintf("IAM OIDC provider: %s", utils.ColorizeItem(arn)), nil)
		}
		return results
	},
	ModuleName: types.DefaultModuleName,
}

var listSAMLProvidersCall = types.AWSService{
	Name: "iam:ListSAMLProviders",
	CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
		svc := awsv2.NewRegionalClient(cfg, "", iam.NewFromConfig)
		return svc.ListSAMLProviders(ctx, &iam.ListSAMLProvidersInput{})
	},
	Process: func(output interface{}, err error, debug bool) []types.ScanResult {
		if err != nil {
			utils.HandleAWSError(debug, "iam:ListSAMLProviders", err)
			return errResult("iam:ListSAMLProviders", err)
		}
		out, _ := output.(*iam.ListSAMLProvidersOutput)
		if out == nil {
			return nil
		}
		var results []types.ScanResult
		for _, p := range out.SAMLProviderList {
			arn := str(p.Arn)
			results = append(results, types.ScanResult{
				ServiceName:  "IAM",
				MethodName:   "iam:ListSAMLProviders",
				ResourceType: "saml-provider",
				ResourceName: arn,
				Details: map[string]interface{}{
					"create_date": timeStr(p.CreateDate),
					"valid_until": timeStr(p.ValidUntil),
				},
				Timestamp: time.Now(),
			})
			utils.PrintResult(debug, "", "iam:ListSAMLProviders", fmt.Sprintf("IAM SAML provider: %s", utils.ColorizeItem(arn)), nil)
		}
		return results
	},
	ModuleName: types.DefaultModuleName,
}

var getAccountSummaryCall = types.AWSService{
	Name: "iam:GetAccountSummary",
	CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
		svc := awsv2.NewRegionalClient(cfg, "", iam.NewFromConfig)
		return svc.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
	},
	Process: func(output interface{}, err error, debug bool) []types.ScanResult {
		if err != nil {
			utils.HandleAWSError(debug, "iam:GetAccountSummary", err)
			return errResult("iam:GetAccountSummary", err)
		}
		out, _ := output.(*iam.GetAccountSummaryOutput)
		if out == nil {
			return nil
		}
		summary := map[string]interface{}{}
		keys := make([]string, 0, len(out.SummaryMap))
		for k, v := range out.SummaryMap {
			summary[k] = int64(v)
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			utils.PrintResult(debug, "", "iam:GetAccountSummary", fmt.Sprintf("IAM account summary: %s = %d", k, out.SummaryMap[k]), nil)
		}
		return []types.ScanResult{{
			ServiceName:  "IAM",
			MethodName:   "iam:GetAccountSummary",
			ResourceType: "account-summary",
			ResourceName: "account",
			Details:      summary,
			Timestamp:    time.Now(),
		}}
	},
	ModuleName: types.DefaultModuleName,
}

var getAccountPasswordPolicyCall = types.AWSService{
	Name: "iam:GetAccountPasswordPolicy",
	CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
		svc := awsv2.NewRegionalClient(cfg, "", iam.NewFromConfig)
		return svc.GetAccountPasswordPolicy(ctx, &iam.GetAccountPasswordPolicyInput{})
	},
	Process: func(output interface{}, err error, debug bool) []types.ScanResult {
		const method = "iam:GetAccountPasswordPolicy"
		result := func(details map[string]interface{}) []types.ScanResult {
			utils.PrintResult(debug, "", method, fmt.Sprintf("IAM account password policy: %v", details["status"]), nil)
			return []types.ScanResult{{
				ServiceName:  "IAM",
				MethodName:   method,
				ResourceType: "password-policy",
				ResourceName: "account",
				Details:      details,
				Timestamp:    time.Now(),
			}}
		}
		if err != nil {
			err = awsv2.Wrap(err)
			switch {
			case awsv2.HasErrorCode(err, "NoSuchEntity"):
				return result(map[string]interface{}{"status": "none"})
			case isAccessDenied(err):
				return result(map[string]interface{}{"status": "denied"})
			}
			utils.HandleAWSError(debug, method, err)
			return errResult(method, err)
		}
		out, _ := output.(*iam.GetAccountPasswordPolicyOutput)
		if out == nil || out.PasswordPolicy == nil {
			return nil
		}
		p := out.PasswordPolicy
		details := map[string]interface{}{
			"status":                         "configured",
			"require_symbols":                p.RequireSymbols,
			"require_numbers":                p.RequireNumbers,
			"require_uppercase":              p.RequireUppercaseCharacters,
			"require_lowercase":              p.RequireLowercaseCharacters,
			"allow_users_to_change_password": p.AllowUsersToChangePassword,
			"expire_passwords":               p.ExpirePasswords,
		}
		if p.MinimumPasswordLength != nil {
			details["minimum_password_length"] = int64(*p.MinimumPasswordLength)
		}
		if p.MaxPasswordAge != nil {
			details["max_password_age"] = int64(*p.MaxPasswordAge)
		}
		if p.PasswordReusePrevention != nil {
			details["password_reuse_prevention"] = int64(*p.PasswordReusePrevention)
		}
		if p.HardExpiry != nil {
			details["hard_expiry"] = *p.HardExpiry
		}
		return result(details)
	},
	ModuleName: types.DefaultModuleName,
}

var listMFADevicesCall = types.AWSService{
	Name: "iam:ListMFADevices",
	CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
		svc := awsv2.NewRegionalClient(cfg, "", iam.NewFromConfig)
		res := &listResult{}
		var users []iamtypes.User
		var err error
		pager := iam.NewListUsersPaginator(svc, &iam.ListUsersInput{})
	pages:
		for pager.HasMorePages() {
			var page *iam.ListUsersOutput
			page, err = pager.NextPage(ctx)
			if err != nil {
				break
			}
			for _, u := range page.Users {
				if len(users) >= maxMFAUsers {
					res.Truncated = true
					break pages
				}
				users = append(users, u)
			}
			if len(users) >= maxMFAUsers && pager.HasMorePages() {
				res.Truncated = true
				break
			}
		}
		res.Items = users
		return map[string]interface{}{"output": res, "cfg": cfg, "ctx": ctx}, err
	},
	Process: func(output interface{}, err error, debug bool) []types.ScanResult {
		if err != nil {
			utils.HandleAWSError(debug, "iam:ListMFADevices", err)
			return errResult("iam:ListMFADevices", err)
		}
		lr, cfg, hasCfg, ctx := unpackList(output)
		if lr == nil || !hasCfg {
			return nil
		}
		users, _ := lr.Items.([]iamtypes.User)
		var names []string
		for _, u := range users {
			names = append(names, str(u.UserName))
		}
		svc := awsv2.NewRegionalClient(cfg, "", iam.NewFromConfig)
		steps := []followupStep{{
			Method: "iam:ListMFADevices",
			Label:  "MFA device",
			List: func(ctx context.Context, user string) ([]followupItem, error) {
				out, err := svc.ListMFADevices(ctx, &iam.ListMFADevicesInput{UserName: aws.String(user)})
				if err != nil {
					return nil, err
				}
				var items []followupItem
				for _, d := range out.MFADevices {
					items = append(items, followupItem{"mfa-device", str(d.SerialNumber),
						map[string]interface{}{"user": user, "enable_date": timeStr(d.EnableDate)}})
				}
				return items, nil
			},
		}}
		results := runFollowups(ctx, debug, "user", "iam:ListMFADevices", names, maxMFAFollowups, steps)
		if lr.Truncated {
			results = append(results, listTruncatedNote(debug, "iam:ListMFADevices", "user", maxMFAUsers))
		}
		return results
	},
	ModuleName: types.DefaultModuleName,
}
