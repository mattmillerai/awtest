package codepipeline

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codepipeline"
	"time"
)

var CodePipelineCalls = []types.AWSService{
	{
		Name: "codepipeline:ListPipelines",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := codepipeline.New(sess)
			input := &codepipeline.ListPipelinesInput{}
			return svc.ListPipelinesWithContext(ctx, input)
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

			if pipelines, ok := output.(*codepipeline.ListPipelinesOutput); ok {
				for _, pipeline := range pipelines.Pipelines {
					results = append(results, types.ScanResult{
						ServiceName:  "CodePipeline",
						MethodName:   "codepipeline:ListPipelines",
						ResourceType: "pipeline",
						ResourceName: *pipeline.Name,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "codepipeline:ListPipelines", fmt.Sprintf("Pipeline: %s", *pipeline.Name), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
