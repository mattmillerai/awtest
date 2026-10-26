package ivsrealtime

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ivsrealtime"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var IvsRealtimeCalls = []types.AWSService{
	{
		Name: "ivsRealtime:ListStages",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", ivsrealtime.NewFromConfig)
			input := &ivsrealtime.ListStagesInput{}
			return svc.ListStages(ctx, input)
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
					name := aws.ToString(stage.Name)
					results = append(results, types.ScanResult{
						ServiceName:  "IVSRealtime",
						MethodName:   "ivsRealtime:ListStages",
						ResourceType: "stage",
						ResourceName: name,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "ivs-realtime:ListStages", fmt.Sprintf("Stage: %s", name), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
