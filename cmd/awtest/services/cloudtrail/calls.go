package cloudtrail

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var CloudTrailCalls = []types.AWSService{
	{
		Name: "cloudtrail:DescribeTrails",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", cloudtrail.NewFromConfig)
			input := &cloudtrail.DescribeTrailsInput{}
			return svc.DescribeTrails(ctx, input)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "cloudtrail:DescribeTrails", err)
				return []types.ScanResult{
					{
						ServiceName: "CloudTrail",
						MethodName:  "cloudtrail:DescribeTrails",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if trails, ok := output.(*cloudtrail.DescribeTrailsOutput); ok {
				for _, trail := range trails.TrailList {
					name := aws.ToString(trail.Name)
					utils.PrintResult(debug, "", "cloudtrail:DescribeTrails", fmt.Sprintf("Trail: %s", name), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "CloudTrail",
						MethodName:   "cloudtrail:DescribeTrails",
						ResourceType: "trail",
						ResourceName: name,
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
		Name: "cloudtrail:ListTrails",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", cloudtrail.NewFromConfig)
			input := &cloudtrail.ListTrailsInput{}
			return svc.ListTrails(ctx, input)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "cloudtrail:ListTrails", err)
				return []types.ScanResult{
					{
						ServiceName: "CloudTrail",
						MethodName:  "cloudtrail:ListTrails",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if trailsOutput, ok := output.(*cloudtrail.ListTrailsOutput); ok {
				for _, trail := range trailsOutput.Trails {
					name := aws.ToString(trail.Name)
					utils.PrintResult(debug, "", "cloudtrail:ListTrails", fmt.Sprintf("Trail: %s", name), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "CloudTrail",
						MethodName:   "cloudtrail:ListTrails",
						ResourceType: "trail",
						ResourceName: name,
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
