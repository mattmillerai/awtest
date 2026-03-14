package ivsrealtime

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ivsrealtime"
	"time"
)

var IvsRealtimeCalls = []types.AWSService{
	{
		Name: "ivsRealtime:ListStages",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := ivsrealtime.New(sess)
			input := &ivsrealtime.ListStagesInput{}
			return svc.ListStagesWithContext(ctx, input)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "ivsRealtime:ListStages", err)
				return []types.ScanResult{
					{
						ServiceName: "IVSRealtime",
						MethodName:  "ivsRealtime:ListStages",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if stagesOutput, ok := output.(*ivsrealtime.ListStagesOutput); ok {
				for _, stage := range stagesOutput.Stages {
					results = append(results, types.ScanResult{
						ServiceName:  "IVSRealtime",
						MethodName:   "ivsRealtime:ListStages",
						ResourceType: "stage",
						ResourceName: *stage.Name,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "ivs-realtime:ListStages", fmt.Sprintf("Stage: %s", *stage.Name), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
