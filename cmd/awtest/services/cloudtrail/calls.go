package cloudtrail

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudtrail"
	"time"
)

var CloudTrailCalls = []types.AWSService{
	{
		Name: "cloudtrail:DescribeTrails",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := cloudtrail.New(sess)
			input := &cloudtrail.DescribeTrailsInput{}
			return svc.DescribeTrailsWithContext(ctx, input)
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
					utils.PrintResult(debug, "", "cloudtrail:DescribeTrails", fmt.Sprintf("Trail: %s", *trail.Name), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "CloudTrail",
						MethodName:   "cloudtrail:DescribeTrails",
						ResourceType: "trail",
						ResourceName: *trail.Name,
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
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := cloudtrail.New(sess)
			input := &cloudtrail.ListTrailsInput{}
			return svc.ListTrailsWithContext(ctx, input)
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
					utils.PrintResult(debug, "", "cloudtrail:ListTrails", fmt.Sprintf("Trail: %s", *trail.Name), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "CloudTrail",
						MethodName:   "cloudtrail:ListTrails",
						ResourceType: "trail",
						ResourceName: *trail.Name,
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
