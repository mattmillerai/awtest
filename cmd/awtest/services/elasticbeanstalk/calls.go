package elasticbeanstalk

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elasticbeanstalk"
	"time"
)

var ElasticBeanstalkCalls = []types.AWSService{
	{
		Name: "elasticbeanstalk:DescribeApplications",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := elasticbeanstalk.New(sess)
			input := &elasticbeanstalk.DescribeApplicationsInput{}
			output, err := svc.DescribeApplicationsWithContext(ctx, input)
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
			if applications, ok := output.([]*elasticbeanstalk.ApplicationDescription); ok {
				for _, app := range applications {
					utils.PrintResult(debug, "", "elasticbeanstalk:DescribeApplications", fmt.Sprintf("Found Application: %s", *app.ApplicationName), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "ElasticBeanstalk",
						MethodName:   "elasticbeanstalk:DescribeApplications",
						ResourceType: "application",
						ResourceName: *app.ApplicationName,
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
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := elasticbeanstalk.New(sess)
			input := &elasticbeanstalk.DescribeEventsInput{}
			output, err := svc.DescribeEventsWithContext(ctx, input)
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
			if events, ok := output.([]*elasticbeanstalk.EventDescription); ok {
				for _, event := range events {
					utils.PrintResult(debug, "", "elasticbeanstalk:DescribeEvents", fmt.Sprintf("Found Event: %s", *event.Message), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "ElasticBeanstalk",
						MethodName:   "elasticbeanstalk:DescribeEvents",
						ResourceType: "event",
						ResourceName: *event.Message,
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
