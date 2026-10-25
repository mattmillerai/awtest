package glue

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var GlueCalls = []types.AWSService{
	{
		Name: "glue:ListWorkflows",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", glue.NewFromConfig)
			input := &glue.ListWorkflowsInput{}
			return svc.ListWorkflows(ctx, input)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "glue:ListWorkflows", err)
				return []types.ScanResult{
					{
						ServiceName: "Glue",
						MethodName:  "glue:ListWorkflows",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if workflows, ok := output.(*glue.ListWorkflowsOutput); ok {
				for _, workflowName := range workflows.Workflows {
					utils.PrintResult(debug, "", "glue:ListWorkflows", fmt.Sprintf("Workflow: %s", workflowName), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "Glue",
						MethodName:   "glue:ListWorkflows",
						ResourceType: "workflow",
						ResourceName: workflowName,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "glue:ListJobs",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", glue.NewFromConfig)
			input := &glue.ListJobsInput{}
			return svc.ListJobs(ctx, input)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "glue:ListJobs", err)
				return []types.ScanResult{
					{
						ServiceName: "Glue",
						MethodName:  "glue:ListJobs",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if jobs, ok := output.(*glue.ListJobsOutput); ok {
				for _, job := range jobs.JobNames {
					utils.PrintResult(debug, "", "glue:ListJobs", fmt.Sprintf("Job: %s", job), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "Glue",
						MethodName:   "glue:ListJobs",
						ResourceType: "job",
						ResourceName: job,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
