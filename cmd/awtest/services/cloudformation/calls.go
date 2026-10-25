package cloudformation

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var CloudFormationCalls = []types.AWSService{
	{
		Name: "cloudformation:ListStacks",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", cloudformation.NewFromConfig)
			input := &cloudformation.ListStacksInput{}
			return svc.ListStacks(ctx, input)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "cloudformation:ListStacks", err)
				return []types.ScanResult{
					{
						ServiceName: "CloudFormation",
						MethodName:  "cloudformation:ListStacks",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			stacks, ok := output.(*cloudformation.ListStacksOutput)
			if !ok {
				utils.HandleAWSError(debug, "cloudformation:ListStacks", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, stack := range stacks.StackSummaries {
				stackName := aws.ToString(stack.StackName)
				results = append(results, types.ScanResult{
					ServiceName:  "CloudFormation",
					MethodName:   "cloudformation:ListStacks",
					ResourceType: "stack",
					ResourceName: stackName,
					Details:      map[string]interface{}{},
					Timestamp:    time.Now(),
				})

				utils.PrintResult(debug, "", "cloudformation:ListStacks", fmt.Sprintf("Stack: %s", stackName), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
