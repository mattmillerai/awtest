package eventbridge

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"time"
)

var EventbridgeCalls = []types.AWSService{
	{
		Name: "eventbridge:ListEventBuses",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := eventbridge.New(sess)
			output, err := svc.ListEventBusesWithContext(ctx, &eventbridge.ListEventBusesInput{})
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

			if buses, ok := output.([]*eventbridge.EventBus); ok {
				for _, bus := range buses {
					results = append(results, types.ScanResult{
						ServiceName:  "EventBridge",
						MethodName:   "eventbridge:ListEventBuses",
						ResourceType: "event-bus",
						ResourceName: *bus.Name,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "eventbridge:ListEventBuses", fmt.Sprintf("EventBridge bus: %s", *bus.Name), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
