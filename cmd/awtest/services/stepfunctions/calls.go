package stepfunctions

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
	"time"
)

var StepFunctionsCalls = []types.AWSService{
	{
		Name: "states:ListStateMachines",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allStateMachines []*sfn.StateMachineListItem
			var lastErr error
			anyRegionSucceeded := false
			for _, region := range types.Regions {
				regionSess := sess.Copy(&aws.Config{Region: aws.String(region)})
				svc := sfn.New(regionSess)
				input := &sfn.ListStateMachinesInput{}
				regionFailed := false
				for {
					output, err := svc.ListStateMachinesWithContext(ctx, input)
					if err != nil {
						lastErr = err
						regionFailed = true
						break
					}
					allStateMachines = append(allStateMachines, output.StateMachines...)
					if output.NextToken == nil {
						break
					}
					input.NextToken = output.NextToken
				}
				if !regionFailed {
					anyRegionSucceeded = true
				}
			}
			if !anyRegionSucceeded && lastErr != nil {
				return nil, lastErr
			}
			return allStateMachines, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "states:ListStateMachines", err)
				return []types.ScanResult{
					{
						ServiceName: "Step Functions",
						MethodName:  "states:ListStateMachines",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			stateMachines, ok := output.([]*sfn.StateMachineListItem)
			if !ok {
				utils.HandleAWSError(debug, "states:ListStateMachines", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			if len(stateMachines) == 0 {
				utils.PrintAccessGranted(debug, "states:ListStateMachines", "Step Functions state machines")
				return results
			}

			for _, sm := range stateMachines {
				name := ""
				if sm.Name != nil {
					name = *sm.Name
				}

				arn := ""
				if sm.StateMachineArn != nil {
					arn = *sm.StateMachineArn
				}

				smType := ""
				if sm.Type != nil {
					smType = *sm.Type
				}

				creationDate := ""
				if sm.CreationDate != nil {
					creationDate = sm.CreationDate.Format("2006-01-02 15:04:05")
				}

				results = append(results, types.ScanResult{
					ServiceName:  "Step Functions",
					MethodName:   "states:ListStateMachines",
					ResourceType: "state-machine",
					ResourceName: name,
					Details: map[string]interface{}{
						"StateMachineArn": arn,
						"Type":            smType,
						"CreationDate":    creationDate,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "states:ListStateMachines",
					fmt.Sprintf("Found Step Functions State Machine: %s (ARN: %s, Type: %s, Created: %s)",
						utils.ColorizeItem(name), arn, smType, creationDate), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
