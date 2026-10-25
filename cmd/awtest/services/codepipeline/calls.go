package codepipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var CodePipelineCalls = []types.AWSService{
	{
		Name: "codepipeline:ListPipelines",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", codepipeline.NewFromConfig)
			input := &codepipeline.ListPipelinesInput{}
			return svc.ListPipelines(ctx, input)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "codepipeline:ListPipelines", err)
				return []types.ScanResult{
					{
						ServiceName: "CodePipeline",
						MethodName:  "codepipeline:ListPipelines",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			pipelines, ok := output.(*codepipeline.ListPipelinesOutput)
			if !ok {
				utils.HandleAWSError(debug, "codepipeline:ListPipelines", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, pipeline := range pipelines.Pipelines {
				pipelineName := aws.ToString(pipeline.Name)
				results = append(results, types.ScanResult{
					ServiceName:  "CodePipeline",
					MethodName:   "codepipeline:ListPipelines",
					ResourceType: "pipeline",
					ResourceName: pipelineName,
					Details:      map[string]interface{}{},
					Timestamp:    time.Now(),
				})

				utils.PrintResult(debug, "", "codepipeline:ListPipelines", fmt.Sprintf("Pipeline: %s", pipelineName), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
