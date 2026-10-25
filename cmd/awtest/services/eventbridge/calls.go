package eventbridge

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var EventbridgeCalls = []types.AWSService{
	{
		Name: "eventbridge:ListEventBuses",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", eventbridge.NewFromConfig)
			output, err := svc.ListEventBuses(ctx, &eventbridge.ListEventBusesInput{})
			if err != nil {
				return nil, err
			}
			return output.EventBuses, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "eventbridge:ListEventBuses", err)
				return []types.ScanResult{
					{
						ServiceName: "EventBridge",
						MethodName:  "eventbridge:ListEventBuses",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			buses, ok := output.([]ebtypes.EventBus)
			if !ok {
				utils.HandleAWSError(debug, "eventbridge:ListEventBuses", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, bus := range buses {
				busName := aws.ToString(bus.Name)
				results = append(results, types.ScanResult{
					ServiceName:  "EventBridge",
					MethodName:   "eventbridge:ListEventBuses",
					ResourceType: "event-bus",
					ResourceName: busName,
					Details:      map[string]interface{}{},
					Timestamp:    time.Now(),
				})

				utils.PrintResult(debug, "", "eventbridge:ListEventBuses", fmt.Sprintf("EventBridge bus: %s", busName), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
