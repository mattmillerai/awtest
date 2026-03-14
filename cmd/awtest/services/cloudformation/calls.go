package cloudformation

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"time"
)

var CloudFormationCalls = []types.AWSService{
	{
		Name: "cloudformation:ListStacks",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := cloudformation.New(sess)
			input := &cloudformation.ListStacksInput{}
			return svc.ListStacksWithContext(ctx, input)
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

			if stacks, ok := output.(*cloudformation.ListStacksOutput); ok {
				for _, stack := range stacks.StackSummaries {
					results = append(results, types.ScanResult{
						ServiceName:  "CloudFormation",
						MethodName:   "cloudformation:ListStacks",
						ResourceType: "stack",
						ResourceName: *stack.StackName,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "cloudformation:ListStacks", fmt.Sprintf("Stack: %s", *stack.StackName), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
