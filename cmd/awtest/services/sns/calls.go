package sns

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"strings"
	"time"
)

type TopicWithAttributes struct {
	Topic      *sns.Topic
	Attributes map[string]*string
}

var SNSCalls = []types.AWSService{
	{
		Name: "sns:ListTopics",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allTopicsWithAttributes []TopicWithAttributes
			originalConfig := sess.Config
			for _, region := range types.Regions {
				regionConfig := &aws.Config{
					Region:      aws.String(region),
					Credentials: originalConfig.Credentials,
				}
				regionSess, err := session.NewSession(regionConfig)
				if err != nil {
					return nil, err
				}
				svc := sns.New(regionSess)
				output, err := svc.ListTopicsWithContext(ctx, &sns.ListTopicsInput{})
				if err != nil {
					return nil, err
				}

				for _, topic := range output.Topics {
					arnParts := strings.Split(*topic.TopicArn, ":")
					if len(arnParts) < 4 {
						return nil, fmt.Errorf("invalid ARN: %s", *topic.TopicArn)
					}
					region := arnParts[3]
					regionConfigForAttr := &aws.Config{
						Region:      aws.String(region),
						Credentials: originalConfig.Credentials,
					}
					attrSess, err := session.NewSession(regionConfigForAttr)
					if err != nil {
						return nil, err
					}
					svc = sns.New(attrSess)

					attrOutput, attrErr := svc.GetTopicAttributesWithContext(ctx, &sns.GetTopicAttributesInput{
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
					topicArn := ""
					if topicWithAttr.Topic.TopicArn != nil {
						topicArn = *topicWithAttr.Topic.TopicArn
					}

					// Build details from attributes
					topicDetails := make(map[string]interface{})
					for name, value := range topicWithAttr.Attributes {
						if value != nil {
							topicDetails[name] = *value
						}
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
						if (name == "DisplayName" || name == "SubscriptionsConfirmed" || name == "SubscriptionsPending" || name == "SubscriptionsDeleted") && value != nil && *value != "" {
							utils.PrintResult(debug, "", "sns:GetTopicAttributes", fmt.Sprintf("SNS Topic: %s | %s = %s", colorizedTopic, name, *value), nil)
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
