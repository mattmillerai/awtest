package ivschat

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ivschat"
	"time"
)

var IvsChatCalls = []types.AWSService{
	{
		Name: "ivsChat:ListRooms",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := ivschat.New(sess)
			input := &ivschat.ListRoomsInput{}
			return svc.ListRoomsWithContext(ctx, input)
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
					results = append(results, types.ScanResult{
						ServiceName:  "IVSChat",
						MethodName:   "ivsChat:ListRooms",
						ResourceType: "room",
						ResourceName: *room.Name,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "ivsChat:ListRooms", fmt.Sprintf("Room: %s", *room.Name), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
