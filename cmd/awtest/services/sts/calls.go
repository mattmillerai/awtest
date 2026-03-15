package sts

import (
	"context"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sts"
	"strings"
	"time"
)

var STSCalls = []types.AWSService{
	{
		Name: "sts:GetCallerIdentity",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := sts.New(sess)
			output, err := svc.GetCallerIdentityWithContext(ctx, &sts.GetCallerIdentityInput{})
			return map[string]interface{}{
				"output": output,
				"ctx":    ctx,
			}, err
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "sts:GetCallerIdentity", err)
				return []types.ScanResult{
					{
						ServiceName: "STS",
						MethodName:  "sts:GetCallerIdentity",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if outputMap, ok := output.(map[string]interface{}); ok {
				stsOutput, _ := outputMap["output"].(*sts.GetCallerIdentityOutput)
				ctx, _ := outputMap["ctx"].(context.Context)
				if ctx == nil {
					ctx = context.Background()
				}
				userId := ""
				account := ""
				arn := ""
				if stsOutput.UserId != nil {
					userId = *stsOutput.UserId
				}
				if stsOutput.Account != nil {
					account = *stsOutput.Account
				}
				if stsOutput.Arn != nil {
					arn = *stsOutput.Arn
				}

				// Add identity result
				results = append(results, types.ScanResult{
					ServiceName:  "STS",
					MethodName:   "sts:GetCallerIdentity",
					ResourceType: "identity",
					ResourceName: userId,
					Details: map[string]interface{}{
						"account": account,
						"arn":     arn,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "user-id", userId, nil)
				utils.PrintResult(debug, "", "account-number", account, nil)
				utils.PrintResult(debug, "", "iam-arn", arn, nil)

				// Parse the ARN inline to get user name
				arnParts := strings.Split(arn, "/")
				userName := arnParts[len(arnParts)-1]
				utils.PrintResult(debug, "", "iam-user", userName, nil)

				// List attached user policies by calling the IAM service using the Policy Simulator
				sess := session.Must(session.NewSession())
				svc := iam.New(sess)
				attachedPolicyOutput, err := svc.ListAttachedUserPoliciesWithContext(ctx, &iam.ListAttachedUserPoliciesInput{
					UserName: &userName,
				})

				if err != nil {
					utils.HandleAWSError(debug, "iam:ListAttachedUserPolicies", err)
				} else {
					for _, policy := range attachedPolicyOutput.AttachedPolicies {
						policyArn := ""
						if policy.PolicyArn != nil {
							policyArn = *policy.PolicyArn
						}

						results = append(results, types.ScanResult{
							ServiceName:  "IAM",
							MethodName:   "iam:ListAttachedUserPolicies",
							ResourceType: "policy",
							ResourceName: policyArn,
							Details:      map[string]interface{}{"user": userName},
							Timestamp:    time.Now(),
						})

						utils.PrintResult(debug, "", "iam:ListAttachedUserPolicies", policyArn, nil)
					}
				}

				// List user policies by calling the IAM service
				policyOutput, err := svc.ListUserPoliciesWithContext(ctx, &iam.ListUserPoliciesInput{
					UserName: &userName,
				})

				if err != nil {
					utils.HandleAWSError(debug, "iam:ListUserPolicies", err)
				} else {
					for _, policy := range policyOutput.PolicyNames {
						policyName := ""
						if policy != nil {
							policyName = *policy
						}

						results = append(results, types.ScanResult{
							ServiceName:  "IAM",
							MethodName:   "iam:ListUserPolicies",
							ResourceType: "inline-policy",
							ResourceName: policyName,
							Details:      map[string]interface{}{"user": userName},
							Timestamp:    time.Now(),
						})

						utils.PrintResult(debug, "", "iam:ListUserPolicies", policyName, nil)
					}
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
