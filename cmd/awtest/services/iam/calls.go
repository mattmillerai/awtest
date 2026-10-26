package iam

import (
	"context"
	"fmt"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"time"
)

var IAMCalls = []types.AWSService{
	{
		Name: "iam:ListUsers",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", iam.NewFromConfig)
			output, err := svc.ListUsers(ctx, &iam.ListUsersInput{})
			return map[string]interface{}{
				"output": output,
				"cfg":    cfg,
				"ctx":    ctx,
			}, err
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "iam:ListUsers", err)
				return []types.ScanResult{
					{
						ServiceName: "IAM",
						MethodName:  "iam:ListUsers",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if outputMap, ok := output.(map[string]interface{}); ok {
				iamOutput, _ := outputMap["output"].(*iam.ListUsersOutput)
				cfg, _ := outputMap["cfg"].(aws.Config)
				ctx, _ := outputMap["ctx"].(context.Context)
				if ctx == nil {
					ctx = context.Background()
				}
				svc := awsv2.NewRegionalClient(cfg, "", iam.NewFromConfig)
				for _, user := range iamOutput.Users {
					userName := ""
					if user.UserName != nil {
						userName = *user.UserName
					}

					// Add user result
					results = append(results, types.ScanResult{
						ServiceName:  "IAM",
						MethodName:   "iam:ListUsers",
						ResourceType: "user",
						ResourceName: userName,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "iam:ListUsers", fmt.Sprintf("IAM user: %s", utils.ColorizeItem(userName)), nil)

					// list groups for user
					groupOutput, err := svc.ListGroupsForUser(ctx, &iam.ListGroupsForUserInput{
						UserName: user.UserName,
					})
					if err != nil {
						utils.HandleAWSError(debug, "iam:ListGroupsForUser", err)
						results = append(results, types.ScanResult{
							ServiceName:  "IAM",
							MethodName:   "iam:ListGroupsForUser",
							ResourceType: "user",
							ResourceName: userName,
							Error:        awsv2.Wrap(err),
							Timestamp:    time.Now(),
						})
					} else {
						for _, group := range groupOutput.Groups {
							groupName := ""
							if group.GroupName != nil {
								groupName = *group.GroupName
							}

							results = append(results, types.ScanResult{
								ServiceName:  "IAM",
								MethodName:   "iam:ListGroupsForUser",
								ResourceType: "group",
								ResourceName: groupName,
								Details:      map[string]interface{}{"user": userName},
								Timestamp:    time.Now(),
							})

							utils.PrintResult(debug, "", "iam:ListGroupsForUser", fmt.Sprintf("IAM User: %s | group: %s", utils.ColorizeItem(userName), groupName), nil)
						}
					}

					// list attached user policies
					attachedPolicyOutput, err := svc.ListAttachedUserPolicies(ctx, &iam.ListAttachedUserPoliciesInput{
						UserName: user.UserName,
					})
					if err != nil {
						utils.HandleAWSError(debug, "iam:ListAttachedUserPolicies", err)
						results = append(results, types.ScanResult{
							ServiceName:  "IAM",
							MethodName:   "iam:ListAttachedUserPolicies",
							ResourceType: "user",
							ResourceName: userName,
							Error:        awsv2.Wrap(err),
							Timestamp:    time.Now(),
						})
					} else {
						for _, policy := range attachedPolicyOutput.AttachedPolicies {
							policyName := ""
							if policy.PolicyName != nil {
								policyName = *policy.PolicyName
							}

							results = append(results, types.ScanResult{
								ServiceName:  "IAM",
								MethodName:   "iam:ListAttachedUserPolicies",
								ResourceType: "attached-policy",
								ResourceName: policyName,
								Details:      map[string]interface{}{"user": userName},
								Timestamp:    time.Now(),
							})

							utils.PrintResult(debug, "", "iam:ListAttachedUserPolicies", fmt.Sprintf("IAM user: %s | attached policy: %s", utils.ColorizeItem(userName), policyName), nil)
						}
					}

					// list user policies
					policyOutput, err := svc.ListUserPolicies(ctx, &iam.ListUserPoliciesInput{
						UserName: user.UserName,
					})
					if err != nil {
						utils.HandleAWSError(debug, "iam:ListUserPolicies", err)
						results = append(results, types.ScanResult{
							ServiceName:  "IAM",
							MethodName:   "iam:ListUserPolicies",
							ResourceType: "user",
							ResourceName: userName,
							Error:        awsv2.Wrap(err),
							Timestamp:    time.Now(),
						})
					} else {
						for _, pName := range policyOutput.PolicyNames {

							results = append(results, types.ScanResult{
								ServiceName:  "IAM",
								MethodName:   "iam:ListUserPolicies",
								ResourceType: "inline-policy",
								ResourceName: pName,
								Details:      map[string]interface{}{"user": userName},
								Timestamp:    time.Now(),
							})

							utils.PrintResult(debug, "", "iam:ListUserPolicies", fmt.Sprintf("IAM user: %s | inline policy: %s", utils.ColorizeItem(userName), pName), nil)
						}
					}

					// list access keys
					accessKeyOutput, err := svc.ListAccessKeys(ctx, &iam.ListAccessKeysInput{
						UserName: user.UserName,
					})
					if err != nil {
						utils.HandleAWSError(debug, "iam:ListAccessKeys", err)
						results = append(results, types.ScanResult{
							ServiceName:  "IAM",
							MethodName:   "iam:ListAccessKeys",
							ResourceType: "user",
							ResourceName: userName,
							Error:        awsv2.Wrap(err),
							Timestamp:    time.Now(),
						})
					} else {
						for _, accessKey := range accessKeyOutput.AccessKeyMetadata {
							accessKeyID := ""
							if accessKey.AccessKeyId != nil {
								accessKeyID = *accessKey.AccessKeyId
							}

							results = append(results, types.ScanResult{
								ServiceName:  "IAM",
								MethodName:   "iam:ListAccessKeys",
								ResourceType: "access-key",
								ResourceName: accessKeyID,
								Details:      map[string]interface{}{"user": userName},
								Timestamp:    time.Now(),
							})

							utils.PrintResult(debug, "", "iam:ListAccessKeys", fmt.Sprintf("IAM user: %s | access key: %s", utils.ColorizeItem(userName), accessKeyID), nil)
						}
					}
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
