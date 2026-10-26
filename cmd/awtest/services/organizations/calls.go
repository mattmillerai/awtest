package organizations

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	orgtypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

// globalRegion is the region the Organizations endpoint lives in; v1 pinned
// every client to it regardless of the configured region.
const globalRegion = "us-east-1"

type orgOU struct {
	OUId     string
	OUName   string
	OUArn    string
	ParentId string
}

var OrganizationsCalls = []types.AWSService{
	{
		Name: "organizations:ListAccounts",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, globalRegion, organizations.NewFromConfig)

			var allAccounts []orgtypes.Account
			input := &organizations.ListAccountsInput{}
			for {
				output, err := svc.ListAccounts(ctx, input)
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

			accounts, ok := output.([]orgtypes.Account)
			if !ok {
				utils.HandleAWSError(debug, "organizations:ListAccounts", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, account := range accounts {
				accountId := aws.ToString(account.Id)
				accountName := aws.ToString(account.Name)
				accountEmail := aws.ToString(account.Email)
				accountStatus := string(account.Status)
				accountArn := aws.ToString(account.Arn)

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
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, globalRegion, organizations.NewFromConfig)

			var allOUs []orgOU
			var queue []string

			// Step 1: Get roots
			rootsInput := &organizations.ListRootsInput{}
			for {
				rootsOutput, err := svc.ListRoots(ctx, rootsInput)
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
					ouOutput, err := svc.ListOrganizationalUnitsForParent(ctx, ouInput)
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
						allOUs = append(allOUs, orgOU{
							OUId:     ouId,
							OUName:   aws.ToString(ou.Name),
							OUArn:    aws.ToString(ou.Arn),
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
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, globalRegion, organizations.NewFromConfig)

			var allPolicies []orgtypes.PolicySummary
			input := &organizations.ListPoliciesInput{
				Filter: orgtypes.PolicyTypeServiceControlPolicy,
			}
			for {
				output, err := svc.ListPolicies(ctx, input)
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

			policies, ok := output.([]orgtypes.PolicySummary)
			if !ok {
				utils.HandleAWSError(debug, "organizations:ListPolicies", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, policy := range policies {
				policyId := aws.ToString(policy.Id)
				policyName := aws.ToString(policy.Name)
				policyArn := aws.ToString(policy.Arn)

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
