package sns

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

type TopicWithAttributes struct {
	Topic      snstypes.Topic
	Attributes map[string]string
}

var SNSCalls = []types.AWSService{
	{
		Name: "sns:ListTopics",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allTopicsWithAttributes []TopicWithAttributes
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, sns.NewFromConfig)
				output, err := svc.ListTopics(ctx, &sns.ListTopicsInput{})
				if err != nil {
					return nil, err
				}

				for _, topic := range output.Topics {
					topicArn := aws.ToString(topic.TopicArn)
					arnParts := strings.Split(topicArn, ":")
					if len(arnParts) < 4 {
						return nil, fmt.Errorf("invalid ARN: %s", topicArn)
					}
					// The attributes are read from the region named in the ARN.
					attrSvc := awsv2.NewRegionalClient(cfg, arnParts[3], sns.NewFromConfig)

					attrOutput, attrErr := attrSvc.GetTopicAttributes(ctx, &sns.GetTopicAttributesInput{
						TopicArn: topic.TopicArn,
					})
					if attrErr != nil {
						return nil, attrErr
					}

					allTopicsWithAttributes = append(allTopicsWithAttributes, TopicWithAttributes{Topic: topic, Attributes: attrOutput.Attributes})
				}
			}
			return allTopicsWithAttributes, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "sns:ListTopics", err)
				return []types.ScanResult{
					{
						ServiceName: "SNS",
						MethodName:  "sns:ListTopics",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if topics, ok := output.([]TopicWithAttributes); ok {
				for _, topicWithAttr := range topics {
					topicArn := aws.ToString(topicWithAttr.Topic.TopicArn)

					// Build details from attributes
					topicDetails := make(map[string]interface{})
					for name, value := range topicWithAttr.Attributes {
						topicDetails[name] = value
					}

					// Add topic result
					results = append(results, types.ScanResult{
						ServiceName:  "SNS",
						MethodName:   "sns:ListTopics",
						ResourceType: "topic",
						ResourceName: topicArn,
						Details:      topicDetails,
						Timestamp:    time.Now(),
					})

					colorizedTopic := utils.ColorizeItem(topicArn)
					utils.PrintResult(debug, "", "sns:ListTopics", fmt.Sprintf("SNS Topic: %s", colorizedTopic), nil)

					// Print attributes
					for name, value := range topicWithAttr.Attributes {
						// Only display DisplayName and number of subscriptions
						if (name == "DisplayName" || name == "SubscriptionsConfirmed" || name == "SubscriptionsPending" || name == "SubscriptionsDeleted") && value != "" {
							utils.PrintResult(debug, "", "sns:GetTopicAttributes", fmt.Sprintf("SNS Topic: %s | %s = %s", colorizedTopic, name, value), nil)
						}
					}
					// print blank line
					fmt.Println()
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
