package batch

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/batch"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var BatchCalls = []types.AWSService{
	{
		Name: "batch:ListJobs",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", batch.NewFromConfig)
			output, err := svc.ListJobs(ctx, &batch.ListJobsInput{})
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

			jobs, ok := output.(*batch.ListJobsOutput)
			if !ok {
				utils.HandleAWSError(debug, "batch:ListJobs", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, job := range jobs.JobSummaryList {
				jobName := aws.ToString(job.JobName)
				results = append(results, types.ScanResult{
					ServiceName:  "Batch",
					MethodName:   "batch:ListJobs",
					ResourceType: "job",
					ResourceName: jobName,
					Details:      map[string]interface{}{},
					Timestamp:    time.Now(),
				})

				utils.PrintResult(debug, "", "batch:ListJobs", jobName, nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
