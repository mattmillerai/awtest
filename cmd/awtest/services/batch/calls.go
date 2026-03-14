package batch

import (
	"context"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/batch"
	"time"
)

var BatchCalls = []types.AWSService{
	{
		Name: "batch:ListJobs",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := batch.New(sess)
			output, err := svc.ListJobsWithContext(ctx, &batch.ListJobsInput{})
			if err != nil {
				return nil, err
			} else {
				return output, nil
			}
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "batch:ListJobs", err)
				return []types.ScanResult{
					{
						ServiceName: "Batch",
						MethodName:  "batch:ListJobs",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if jobs, ok := output.(*batch.ListJobsOutput); ok {
				for _, job := range jobs.JobSummaryList {
					results = append(results, types.ScanResult{
						ServiceName:  "Batch",
						MethodName:   "batch:ListJobs",
						ResourceType: "job",
						ResourceName: *job.JobName,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "batch:ListJobs", *job.JobName, nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
