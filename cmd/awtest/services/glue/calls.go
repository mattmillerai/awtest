package glue

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/glue"
	"time"
)

var GlueCalls = []types.AWSService{
	{
		Name: "glue:ListWorkflows",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := glue.New(sess)
			input := &glue.ListWorkflowsInput{}
			return svc.ListWorkflowsWithContext(ctx, input)
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
					utils.PrintResult(debug, "", "glue:ListWorkflows", fmt.Sprintf("Workflow: %s", *workflowName), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "Glue",
						MethodName:   "glue:ListWorkflows",
						ResourceType: "workflow",
						ResourceName: *workflowName,
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
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := glue.New(sess)
			input := &glue.ListJobsInput{}
			return svc.ListJobsWithContext(ctx, input)
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
					utils.PrintResult(debug, "", "glue:ListJobs", fmt.Sprintf("Job: %s", *job), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "Glue",
						MethodName:   "glue:ListJobs",
						ResourceType: "job",
						ResourceName: *job,
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
