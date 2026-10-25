package stepfunctions

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var StepFunctionsCalls = []types.AWSService{
	{
		Name: "states:ListStateMachines",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allStateMachines []sfntypes.StateMachineListItem
			var lastErr error
			anyRegionSucceeded := false
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, sfn.NewFromConfig)
				input := &sfn.ListStateMachinesInput{}
				regionFailed := false
				for {
					output, err := svc.ListStateMachines(ctx, input)
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

			stateMachines, ok := output.([]sfntypes.StateMachineListItem)
			if !ok {
				utils.HandleAWSError(debug, "states:ListStateMachines", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			if len(stateMachines) == 0 {
				utils.PrintAccessGranted(debug, "states:ListStateMachines", "Step Functions state machines")
				return results
			}

			for _, sm := range stateMachines {
				name := aws.ToString(sm.Name)
				arn := aws.ToString(sm.StateMachineArn)
				smType := string(sm.Type)

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
