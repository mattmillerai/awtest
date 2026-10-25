package sts

import (
	"context"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var STSCalls = []types.AWSService{
	{
		Name: "sts:GetCallerIdentity",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", sts.NewFromConfig)
			output, err := svc.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
			return map[string]interface{}{
				"output": output,
				"ctx":    ctx,
				"cfg":    cfg,
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
				if stsOutput == nil {
					return results
				}
				ctx, _ := outputMap["ctx"].(context.Context)
				if ctx == nil {
					ctx = context.Background()
				}
				cfg, hasCfg := outputMap["cfg"].(aws.Config)
				userId := aws.ToString(stsOutput.UserId)
				account := aws.ToString(stsOutput.Account)
				arn := aws.ToString(stsOutput.Arn)

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

				if !hasCfg {
					return results
				}

				// List attached user policies by calling the IAM service using the Policy Simulator
				svc := awsv2.NewRegionalClient(cfg, "", iam.NewFromConfig)
				attachedPolicyOutput, err := svc.ListAttachedUserPolicies(ctx, &iam.ListAttachedUserPoliciesInput{
					UserName: &userName,
				})

				if err != nil {
					utils.HandleAWSError(debug, "iam:ListAttachedUserPolicies", awsv2.Wrap(err))
				} else {
					for _, policy := range attachedPolicyOutput.AttachedPolicies {
						policyArn := aws.ToString(policy.PolicyArn)

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
				policyOutput, err := svc.ListUserPolicies(ctx, &iam.ListUserPoliciesInput{
					UserName: &userName,
				})

				if err != nil {
					utils.HandleAWSError(debug, "iam:ListUserPolicies", awsv2.Wrap(err))
				} else {
					for _, policyName := range policyOutput.PolicyNames {
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
