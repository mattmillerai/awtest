package iam

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

// IAM is a global service: these entries intentionally do not consult the
// region scope (types.RegionsFor) and run exactly once per scan.
func init() {
	IAMCalls = append(IAMCalls, listRolesCall, listGroupsCall, listPoliciesCall)
}

func unpackList(output interface{}) (*listResult, *session.Session, context.Context) {
	m, _ := output.(map[string]interface{})
	lr, _ := m["output"].(*listResult)
	sess, _ := m["sess"].(*session.Session)
	ctx, _ := m["ctx"].(context.Context)
	if ctx == nil {
		ctx = context.Background()
	}
	return lr, sess, ctx
}

func timeStr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

var listRolesCall = types.AWSService{
	Name: "iam:ListRoles",
	Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
		svc := iam.New(sess)
		res := &listResult{}
		var roles []*iam.Role
		err := svc.ListRolesPagesWithContext(ctx, &iam.ListRolesInput{}, func(page *iam.ListRolesOutput, last bool) bool {
			for _, r := range page.Roles {
				if len(roles) >= maxRoles {
					res.Truncated = true
					return false
				}
				roles = append(roles, r)
			}
			if len(roles) >= maxRoles && !last {
				res.Truncated = true
				return false
			}
			return true
		})
		res.Items = roles
		return map[string]interface{}{"output": res, "sess": sess, "ctx": ctx}, err
	},
	Process: func(output interface{}, err error, debug bool) []types.ScanResult {
		if err != nil {
			utils.HandleAWSError(debug, "iam:ListRoles", err)
			return errResult("iam:ListRoles", err)
		}
		lr, sess, ctx := unpackList(output)
		if lr == nil {
			return nil
		}
		roles, _ := lr.Items.([]*iam.Role)
		var results []types.ScanResult
		var names []string

		for _, role := range roles {
			name := str(role.RoleName)
			names = append(names, name)
			details := map[string]interface{}{
				"arn":         str(role.Arn),
				"path":        str(role.Path),
				"create_date": timeStr(role.CreateDate),
			}
			if role.MaxSessionDuration != nil {
				details["max_session_duration"] = *role.MaxSessionDuration
			}
			if role.Description != nil && *role.Description != "" {
				details["description"] = *role.Description
			}
			// RoleLastUsed is not returned by ListRoles (only by GetRole /
			// GetAccountAuthorizationDetails), and we deliberately do not
			// call GetRole per role, so last-used data is not collected.
			sum, perr := SummarizeTrustPolicy(str(role.AssumeRolePolicyDocument), str(role.Arn))
			details["trusted_principals"] = sum.Principals
			details["trust_findings"] = sum.Findings
			if perr != nil {
				details["trust_policy_parse_error"] = perr.Error()
			}
			results = append(results, types.ScanResult{
				ServiceName:  "IAM",
				MethodName:   "iam:ListRoles",
				ResourceType: "role",
				ResourceName: name,
				Details:      details,
				Timestamp:    time.Now(),
			})
			utils.PrintResult(debug, "", "iam:ListRoles", fmt.Sprintf("IAM role: %s", utils.ColorizeItem(name)), nil)
		}
		if lr.Truncated {
			results = append(results, listTruncatedNote(debug, "iam:ListRoles", "role", maxRoles))
		}
		if sess == nil {
			return results
		}

		svc := iam.New(sess)
		steps := []followupStep{
			{
				Method: "iam:ListAttachedRolePolicies",
				Label:  "attached policy",
				List: func(ctx context.Context, role string) ([]followupItem, error) {
					out, err := svc.ListAttachedRolePoliciesWithContext(ctx, &iam.ListAttachedRolePoliciesInput{RoleName: aws.String(role)})
					if err != nil {
						return nil, err
					}
					var items []followupItem
					for _, p := range out.AttachedPolicies {
						items = append(items, followupItem{"attached-policy", str(p.PolicyName),
							map[string]interface{}{"role": role, "policy_arn": str(p.PolicyArn)}})
					}
					return items, nil
				},
			},
			{
				Method: "iam:ListRolePolicies",
				Label:  "inline policy",
				List: func(ctx context.Context, role string) ([]followupItem, error) {
					out, err := svc.ListRolePoliciesWithContext(ctx, &iam.ListRolePoliciesInput{RoleName: aws.String(role)})
					if err != nil {
						return nil, err
					}
					var items []followupItem
					for _, p := range out.PolicyNames {
						items = append(items, followupItem{"inline-policy", str(p),
							map[string]interface{}{"role": role}})
					}
					return items, nil
				},
			},
		}
		return append(results, runFollowups(ctx, debug, "role", "iam:ListRoles", names, maxRoleFollowups, steps)...)
	},
	ModuleName: types.DefaultModuleName,
}

var listGroupsCall = types.AWSService{
	Name: "iam:ListGroups",
	Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
		svc := iam.New(sess)
		res := &listResult{}
		var groups []*iam.Group
		err := svc.ListGroupsPagesWithContext(ctx, &iam.ListGroupsInput{}, func(page *iam.ListGroupsOutput, last bool) bool {
			for _, g := range page.Groups {
				if len(groups) >= maxGroups {
					res.Truncated = true
					return false
				}
				groups = append(groups, g)
			}
			if len(groups) >= maxGroups && !last {
				res.Truncated = true
				return false
			}
			return true
		})
		res.Items = groups
		return map[string]interface{}{"output": res, "sess": sess, "ctx": ctx}, err
	},
	Process: func(output interface{}, err error, debug bool) []types.ScanResult {
		if err != nil {
			utils.HandleAWSError(debug, "iam:ListGroups", err)
			return errResult("iam:ListGroups", err)
		}
		lr, sess, ctx := unpackList(output)
		if lr == nil {
			return nil
		}
		groups, _ := lr.Items.([]*iam.Group)
		var results []types.ScanResult
		var names []string

		for _, g := range groups {
			name := str(g.GroupName)
			names = append(names, name)
			results = append(results, types.ScanResult{
				ServiceName:  "IAM",
				MethodName:   "iam:ListGroups",
				ResourceType: "group",
				ResourceName: name,
				Details: map[string]interface{}{
					"arn":         str(g.Arn),
					"path":        str(g.Path),
					"create_date": timeStr(g.CreateDate),
				},
				Timestamp: time.Now(),
			})
			utils.PrintResult(debug, "", "iam:ListGroups", fmt.Sprintf("IAM group: %s", utils.ColorizeItem(name)), nil)
		}
		if lr.Truncated {
			results = append(results, listTruncatedNote(debug, "iam:ListGroups", "group", maxGroups))
		}
		if sess == nil {
			return results
		}

		svc := iam.New(sess)
		steps := []followupStep{
			{
				Method: "iam:ListAttachedGroupPolicies",
				Label:  "attached policy",
				List: func(ctx context.Context, group string) ([]followupItem, error) {
					out, err := svc.ListAttachedGroupPoliciesWithContext(ctx, &iam.ListAttachedGroupPoliciesInput{GroupName: aws.String(group)})
					if err != nil {
						return nil, err
					}
					var items []followupItem
					for _, p := range out.AttachedPolicies {
						items = append(items, followupItem{"attached-policy", str(p.PolicyName),
							map[string]interface{}{"group": group, "policy_arn": str(p.PolicyArn)}})
					}
					return items, nil
				},
			},
			{
				Method: "iam:ListGroupPolicies",
				Label:  "inline policy",
				List: func(ctx context.Context, group string) ([]followupItem, error) {
					out, err := svc.ListGroupPoliciesWithContext(ctx, &iam.ListGroupPoliciesInput{GroupName: aws.String(group)})
					if err != nil {
						return nil, err
					}
					var items []followupItem
					for _, p := range out.PolicyNames {
						items = append(items, followupItem{"inline-policy", str(p),
							map[string]interface{}{"group": group}})
					}
					return items, nil
				},
			},
		}
		return append(results, runFollowups(ctx, debug, "group", "iam:ListGroups", names, maxGroupFollowups, steps)...)
	},
	ModuleName: types.DefaultModuleName,
}

// listPoliciesCall lists customer managed policies that are attached to at
// least one entity. AWS-managed policies are excluded on purpose: there are
// hundreds of them and they are not account-specific. Policy documents are
// not fetched (no GetPolicyVersion).
var listPoliciesCall = types.AWSService{
	Name: "iam:ListPolicies",
	Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
		svc := iam.New(sess)
		res := &listResult{}
		var policies []*iam.Policy
		input := &iam.ListPoliciesInput{
			Scope:        aws.String(iam.PolicyScopeTypeLocal),
			OnlyAttached: aws.Bool(true),
		}
		err := svc.ListPoliciesPagesWithContext(ctx, input, func(page *iam.ListPoliciesOutput, last bool) bool {
			for _, p := range page.Policies {
				if len(policies) >= maxPolicies {
					res.Truncated = true
					return false
				}
				policies = append(policies, p)
			}
			if len(policies) >= maxPolicies && !last {
				res.Truncated = true
				return false
			}
			return true
		})
		res.Items = policies
		return map[string]interface{}{"output": res, "sess": sess, "ctx": ctx}, err
	},
	Process: func(output interface{}, err error, debug bool) []types.ScanResult {
		if err != nil {
			utils.HandleAWSError(debug, "iam:ListPolicies", err)
			return errResult("iam:ListPolicies", err)
		}
		lr, _, _ := unpackList(output)
		if lr == nil {
			return nil
		}
		policies, _ := lr.Items.([]*iam.Policy)
		var results []types.ScanResult
		for _, p := range policies {
			name := str(p.PolicyName)
			details := map[string]interface{}{
				"arn":                str(p.Arn),
				"path":               str(p.Path),
				"default_version_id": str(p.DefaultVersionId),
				"create_date":        timeStr(p.CreateDate),
				"update_date":        timeStr(p.UpdateDate),
			}
			if p.AttachmentCount != nil {
				details["attachment_count"] = *p.AttachmentCount
			}
			if p.IsAttachable != nil {
				details["is_attachable"] = *p.IsAttachable
			}
			results = append(results, types.ScanResult{
				ServiceName:  "IAM",
				MethodName:   "iam:ListPolicies",
				ResourceType: "policy",
				ResourceName: name,
				Details:      details,
				Timestamp:    time.Now(),
			})
			utils.PrintResult(debug, "", "iam:ListPolicies", fmt.Sprintf("IAM policy: %s", utils.ColorizeItem(name)), nil)
		}
		if lr.Truncated {
			results = append(results, listTruncatedNote(debug, "iam:ListPolicies", "policy", maxPolicies))
		}
		return results
	},
	ModuleName: types.DefaultModuleName,
}
