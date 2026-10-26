package cloudwatch

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	logstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

type LogGroupWithStreams struct {
	LogGroup   logstypes.LogGroup
	LogStreams []logstypes.LogStream
}

var CloudwatchCalls = []types.AWSService{
	{
		Name: "cloudwatch:DescribeAlarms",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allAlarms []cwtypes.MetricAlarm
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, cloudwatch.NewFromConfig)
				output, err := svc.DescribeAlarms(ctx, &cloudwatch.DescribeAlarmsInput{})
				if err != nil {
					return nil, err
				}
				allAlarms = append(allAlarms, output.MetricAlarms...)
			}
			return allAlarms, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "cloudwatch:DescribeAlarms", err)
				return []types.ScanResult{
					{
						ServiceName: "CloudWatch",
						MethodName:  "cloudwatch:DescribeAlarms",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if alarms, ok := output.([]cwtypes.MetricAlarm); ok {
				for _, alarm := range alarms {
					alarmName := aws.ToString(alarm.AlarmName)
					utils.PrintResult(debug, "", "cloudwatch:DescribeAlarms", fmt.Sprintf("CloudWatch alarm: %s", utils.ColorizeItem(alarmName)), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "CloudWatch",
						MethodName:   "cloudwatch:DescribeAlarms",
						ResourceType: "alarm",
						ResourceName: alarmName,
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
		Name: "cloudwatchlogs:DescribeLogGroupsAndStreams",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allLogGroupsWithStreams []*LogGroupWithStreams

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, cloudwatchlogs.NewFromConfig)

				// Describe Log Groups
				groupPages := cloudwatchlogs.NewDescribeLogGroupsPaginator(svc, &cloudwatchlogs.DescribeLogGroupsInput{})
			pages:
				for groupPages.HasMorePages() {
					output, err := groupPages.NextPage(ctx)
					if err != nil {
						return nil, err
					}
					for _, logGroup := range output.LogGroups {
						// Describe Log Streams for each Log Group
						streamInput := &cloudwatchlogs.DescribeLogStreamsInput{
							LogGroupName: logGroup.LogGroupName,
						}
						var logStreams []logstypes.LogStream
						streamPages := cloudwatchlogs.NewDescribeLogStreamsPaginator(svc, streamInput)
						for streamPages.HasMorePages() {
							streamOutput, err := streamPages.NextPage(ctx)
							if err != nil {
								// v1 stopped paging log groups here without
								// reporting the error.
								break pages
							}
							logStreams = append(logStreams, streamOutput.LogStreams...)
						}

						allLogGroupsWithStreams = append(allLogGroupsWithStreams, &LogGroupWithStreams{
							LogGroup:   logGroup,
							LogStreams: logStreams,
						})
					}
				}
			}

			return allLogGroupsWithStreams, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "cloudwatchlogs:DescribeLogGroupsAndStreams", err)
				return []types.ScanResult{
					{
						ServiceName: "CloudWatchLogs",
						MethodName:  "cloudwatchlogs:DescribeLogGroupsAndStreams",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if logGroupsWithStreams, ok := output.([]*LogGroupWithStreams); ok {
				if len(logGroupsWithStreams) == 0 {
					utils.PrintResult(debug, "", "cloudwatchlogs:DescribeLogGroupsAndStreams", "No log groups found.", nil)
				} else {
					for _, lgws := range logGroupsWithStreams {
						logGroupName := aws.ToString(lgws.LogGroup.LogGroupName)
						// Print the Log Group
						utils.PrintResult(debug, "", "cloudwatchlogs:DescribeLogGroupsAndStreams", fmt.Sprintf("Found Log Group: %s", logGroupName), nil)

						results = append(results, types.ScanResult{
							ServiceName:  "CloudWatchLogs",
							MethodName:   "cloudwatchlogs:DescribeLogGroupsAndStreams",
							ResourceType: "log-group",
							ResourceName: logGroupName,
							Details:      map[string]interface{}{},
							Timestamp:    time.Now(),
						})

						// Check if there are any Log Streams
						if len(lgws.LogStreams) > 0 {
							utils.PrintResult(debug, "", "cloudwatchlogs:DescribeLogGroupsAndStreams", fmt.Sprintf("  Log Streams for %s:", logGroupName), nil)
							for _, logStream := range lgws.LogStreams {
								logStreamName := aws.ToString(logStream.LogStreamName)
								utils.PrintResult(debug, "", "cloudwatchlogs:DescribeLogGroupsAndStreams", fmt.Sprintf("    - Log Stream: %s", logStreamName), nil)

								results = append(results, types.ScanResult{
									ServiceName:  "CloudWatchLogs",
									MethodName:   "cloudwatchlogs:DescribeLogGroupsAndStreams",
									ResourceType: "log-stream",
									ResourceName: logStreamName,
									Details:      map[string]interface{}{},
									Timestamp:    time.Now(),
								})
							}
						} else {
							utils.PrintResult(debug, "", "cloudwatchlogs:DescribeLogGroupsAndStreams", "  No log streams found or access denied.", nil)
						}
					}
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "cloudwatchlogs:ListMetrics",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allMetrics []cwtypes.Metric
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, cloudwatch.NewFromConfig)
				input := &cloudwatch.ListMetricsInput{}
				output, err := svc.ListMetrics(ctx, input)
				if err != nil {
					return nil, err
				}
				allMetrics = append(allMetrics, output.Metrics...)
			}
			return allMetrics, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "cloudwatchlogs:ListMetrics", err)
				return []types.ScanResult{
					{
						ServiceName: "CloudWatch",
						MethodName:  "cloudwatchlogs:ListMetrics",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if metrics, ok := output.([]cwtypes.Metric); ok {
				for _, metric := range metrics {
					metricName := aws.ToString(metric.MetricName)
					utils.PrintResult(debug, "", "cloudwatchlogs:ListMetrics", fmt.Sprintf("Found Metric: %s", metricName), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "CloudWatch",
						MethodName:   "cloudwatchlogs:ListMetrics",
						ResourceType: "metric",
						ResourceName: metricName,
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
