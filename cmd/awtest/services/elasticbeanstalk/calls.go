package elasticbeanstalk

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var ElasticBeanstalkCalls = []types.AWSService{
	{
		Name: "elasticbeanstalk:DescribeApplications",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", elasticbeanstalk.NewFromConfig)
			input := &elasticbeanstalk.DescribeApplicationsInput{}
			output, err := svc.DescribeApplications(ctx, input)
			if err != nil {
				return nil, err
			}
			return output.Applications, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "elasticbeanstalk:DescribeApplications", err)
				return []types.ScanResult{
					{
						ServiceName: "ElasticBeanstalk",
						MethodName:  "elasticbeanstalk:DescribeApplications",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if applications, ok := output.([]ebtypes.ApplicationDescription); ok {
				for _, app := range applications {
					name := aws.ToString(app.ApplicationName)
					utils.PrintResult(debug, "", "elasticbeanstalk:DescribeApplications", fmt.Sprintf("Found Application: %s", name), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "ElasticBeanstalk",
						MethodName:   "elasticbeanstalk:DescribeApplications",
						ResourceType: "application",
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
		Name: "elasticbeanstalk:DescribeEvents",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", elasticbeanstalk.NewFromConfig)
			input := &elasticbeanstalk.DescribeEventsInput{}
			output, err := svc.DescribeEvents(ctx, input)
			if err != nil {
				return nil, err
			}
			return output.Events, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "elasticbeanstalk:DescribeEvents", err)
				return []types.ScanResult{
					{
						ServiceName: "ElasticBeanstalk",
						MethodName:  "elasticbeanstalk:DescribeEvents",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if events, ok := output.([]ebtypes.EventDescription); ok {
				for _, event := range events {
					message := aws.ToString(event.Message)
					utils.PrintResult(debug, "", "elasticbeanstalk:DescribeEvents", fmt.Sprintf("Found Event: %s", message), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "ElasticBeanstalk",
						MethodName:   "elasticbeanstalk:DescribeEvents",
						ResourceType: "event",
						ResourceName: message,
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
