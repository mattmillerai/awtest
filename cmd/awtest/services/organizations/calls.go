package organizations

import (
	"context"
	"fmt"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/organizations"
)

var globalRegionConfig = &aws.Config{Region: aws.String("us-east-1")}

type orgOU struct {
	OUId     string
	OUName   string
	OUArn    string
	ParentId string
}

var OrganizationsCalls = []types.AWSService{
	{
		Name: "organizations:ListAccounts",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := organizations.New(sess, globalRegionConfig)

			var allAccounts []*organizations.Account
			input := &organizations.ListAccountsInput{}
			for {
				output, err := svc.ListAccountsWithContext(ctx, input)
				if err != nil {
					return nil, err
				}
				allAccounts = append(allAccounts, output.Accounts...)
				if output.NextToken == nil {
					break
				}
				input.NextToken = output.NextToken
			}
			return allAccounts, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "organizations:ListAccounts", err)
				return []types.ScanResult{
					{
						ServiceName: "Organizations",
						MethodName:  "organizations:ListAccounts",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			accounts, ok := output.([]*organizations.Account)
			if !ok {
				utils.HandleAWSError(debug, "organizations:ListAccounts", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, account := range accounts {
				accountId := ""
				if account.Id != nil {
					accountId = *account.Id
				}

				accountName := ""
				if account.Name != nil {
					accountName = *account.Name
				}

				accountEmail := ""
				if account.Email != nil {
					accountEmail = *account.Email
				}

				accountStatus := ""
				if account.Status != nil {
					accountStatus = *account.Status
				}

				accountArn := ""
				if account.Arn != nil {
					accountArn = *account.Arn
				}

				joinedTimestamp := ""
				if account.JoinedTimestamp != nil {
					joinedTimestamp = account.JoinedTimestamp.Format(time.RFC3339)
				}

				results = append(results, types.ScanResult{
					ServiceName:  "Organizations",
					MethodName:   "organizations:ListAccounts",
					ResourceType: "account",
					ResourceName: accountName,
					Details: map[string]interface{}{
						"AccountId":       accountId,
						"Email":           accountEmail,
						"Status":          accountStatus,
						"Arn":             accountArn,
						"JoinedTimestamp": joinedTimestamp,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "organizations:ListAccounts",
					fmt.Sprintf("Organizations Account: %s (ID: %s, Status: %s)", utils.ColorizeItem(accountName), accountId, accountStatus), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "organizations:ListOrganizationalUnits",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := organizations.New(sess, globalRegionConfig)

			var allOUs []orgOU
			var queue []string

			// Step 1: Get roots
			rootsInput := &organizations.ListRootsInput{}
			for {
				rootsOutput, err := svc.ListRootsWithContext(ctx, rootsInput)
				if err != nil {
					return nil, err
				}
				for _, root := range rootsOutput.Roots {
					if root.Id != nil {
						queue = append(queue, *root.Id)
					}
				}
				if rootsOutput.NextToken == nil {
					break
				}
				rootsInput.NextToken = rootsOutput.NextToken
			}

			// Step 2: BFS — process queue
			for len(queue) > 0 {
				parentId := queue[0]
				queue = queue[1:]

				ouInput := &organizations.ListOrganizationalUnitsForParentInput{
					ParentId: aws.String(parentId),
				}
				for {
					ouOutput, err := svc.ListOrganizationalUnitsForParentWithContext(ctx, ouInput)
					if err != nil {
						utils.HandleAWSError(false, "organizations:ListOrganizationalUnits", err)
						break
					}
					for _, ou := range ouOutput.OrganizationalUnits {
						ouId := ""
						if ou.Id != nil {
							ouId = *ou.Id
							queue = append(queue, ouId)
						}
						ouName := ""
						if ou.Name != nil {
							ouName = *ou.Name
						}
						ouArn := ""
						if ou.Arn != nil {
							ouArn = *ou.Arn
						}
						allOUs = append(allOUs, orgOU{
							OUId:     ouId,
							OUName:   ouName,
							OUArn:    ouArn,
							ParentId: parentId,
						})
					}
					if ouOutput.NextToken == nil {
						break
					}
					ouInput.NextToken = ouOutput.NextToken
				}
			}
			return allOUs, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "organizations:ListOrganizationalUnits", err)
				return []types.ScanResult{
					{
						ServiceName: "Organizations",
						MethodName:  "organizations:ListOrganizationalUnits",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			ous, ok := output.([]orgOU)
			if !ok {
				utils.HandleAWSError(debug, "organizations:ListOrganizationalUnits", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, ou := range ous {
				results = append(results, types.ScanResult{
					ServiceName:  "Organizations",
					MethodName:   "organizations:ListOrganizationalUnits",
					ResourceType: "organizational-unit",
					ResourceName: ou.OUName,
					Details: map[string]interface{}{
						"OUId":     ou.OUId,
						"Arn":      ou.OUArn,
						"ParentId": ou.ParentId,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "organizations:ListOrganizationalUnits",
					fmt.Sprintf("Organizations OU: %s (ID: %s, Parent: %s)", utils.ColorizeItem(ou.OUName), ou.OUId, ou.ParentId), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "organizations:ListPolicies",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := organizations.New(sess, globalRegionConfig)

			var allPolicies []*organizations.PolicySummary
			input := &organizations.ListPoliciesInput{
				Filter: aws.String("SERVICE_CONTROL_POLICY"),
			}
			for {
				output, err := svc.ListPoliciesWithContext(ctx, input)
				if err != nil {
					return nil, err
				}
				allPolicies = append(allPolicies, output.Policies...)
				if output.NextToken == nil {
					break
				}
				input.NextToken = output.NextToken
			}
			return allPolicies, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "organizations:ListPolicies", err)
				return []types.ScanResult{
					{
						ServiceName: "Organizations",
						MethodName:  "organizations:ListPolicies",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			policies, ok := output.([]*organizations.PolicySummary)
			if !ok {
				utils.HandleAWSError(debug, "organizations:ListPolicies", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, policy := range policies {
				policyId := ""
				if policy.Id != nil {
					policyId = *policy.Id
				}

				policyName := ""
				if policy.Name != nil {
					policyName = *policy.Name
				}

				policyArn := ""
				if policy.Arn != nil {
					policyArn = *policy.Arn
				}

				results = append(results, types.ScanResult{
					ServiceName:  "Organizations",
					MethodName:   "organizations:ListPolicies",
					ResourceType: "service-control-policy",
					ResourceName: policyName,
					Details: map[string]interface{}{
						"PolicyId": policyId,
						"Arn":      policyArn,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "organizations:ListPolicies",
					fmt.Sprintf("Organizations SCP: %s (ID: %s)", utils.ColorizeItem(policyName), policyId), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
