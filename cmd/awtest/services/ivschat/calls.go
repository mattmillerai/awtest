package ivschat

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ivschat"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var IvsChatCalls = []types.AWSService{
	{
		Name: "ivsChat:ListRooms",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", ivschat.NewFromConfig)
			input := &ivschat.ListRoomsInput{}
			return svc.ListRooms(ctx, input)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "ivsChat:ListRooms", err)
				return []types.ScanResult{
					{
						ServiceName: "IVSChat",
						MethodName:  "ivsChat:ListRooms",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if roomsOutput, ok := output.(*ivschat.ListRoomsOutput); ok {
				for _, room := range roomsOutput.Rooms {
					name := aws.ToString(room.Name)
					results = append(results, types.ScanResult{
						ServiceName:  "IVSChat",
						MethodName:   "ivsChat:ListRooms",
						ResourceType: "room",
						ResourceName: name,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "ivsChat:ListRooms", fmt.Sprintf("Room: %s", name), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
