package sqs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

func arnToQueueUrl(arn string) string {
	parts := strings.Split(arn, ":")
	if len(parts) != 6 {
		return "" // Return empty string or handle error appropriately
	}
	region := parts[3]
	accountId := parts[4]
	queueName := parts[5]

	return fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/%s", region, accountId, queueName)
}

var SQSCalls = []types.AWSService{
	{
		Name: "sqs:ListQueues",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allQueues []string
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, sqs.NewFromConfig)
				output, err := svc.ListQueues(ctx, &sqs.ListQueuesInput{})
				if err != nil {
					return nil, err
				}
				allQueues = append(allQueues, output.QueueUrls...)
			}
			return map[string]interface{}{
				"queues": allQueues,
				"ctx":    ctx,
				"cfg":    cfg,
			}, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "sqs:ListQueues", err)
				return []types.ScanResult{
					{
						ServiceName: "SQS",
						MethodName:  "sqs:ListQueues",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if outputMap, ok := output.(map[string]interface{}); ok {
				queues, _ := outputMap["queues"].([]string)
				cfg, _ := outputMap["cfg"].(aws.Config)
				ctx, _ := outputMap["ctx"].(context.Context)
				if ctx == nil {
					ctx = context.Background()
				}
				for _, queueUrl := range queues {
					queue := aws.String(queueUrl)

					// Add queue result
					results = append(results, types.ScanResult{
						ServiceName:  "SQS",
						MethodName:   "sqs:ListQueues",
						ResourceType: "queue",
						ResourceName: queueUrl,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "sqs:ListQueues", fmt.Sprintf("================================================================================"), nil)
					utils.PrintResult(debug, "", "sqs:ListQueues", fmt.Sprintf("Found SQS queue: %s", queueUrl), nil)

					svc := awsv2.NewRegionalClient(cfg, "us-east-1", sqs.NewFromConfig)

					// Receive messages from the queue
					receiveOutput, receiveErr := svc.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
						QueueUrl: queue,
					})

					if receiveErr != nil {
						utils.HandleAWSError(debug, "sqs:ReceiveMessage", awsv2.Wrap(receiveErr))
					} else {
						if len(receiveOutput.Messages) == 0 {
							utils.PrintResult(debug, "", "sqs:ReceiveMessage", "Access Granted: No messages in queue", nil)
						}

						for _, message := range receiveOutput.Messages {
							utils.PrintResult(debug, "", "sqs:ReceiveMessage", fmt.Sprintf("Received message: %s", aws.ToString(message.Body)), nil)
						}
					}

					attrOutput, attrErr := svc.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
						QueueUrl: queue,
						AttributeNames: []sqstypes.QueueAttributeName{
							sqstypes.QueueAttributeNameAll,
						},
					})

					if attrErr != nil {
						utils.HandleAWSError(debug, "sqs:GetQueueAttributes", awsv2.Wrap(attrErr))
					} else {
						// Format the attributes
						attributes := make(map[string]string)
						for key, value := range attrOutput.Attributes {
							attributes[key] = value
							utils.PrintResult(debug, "", "sqs:GetQueueAttributes", fmt.Sprintf("Queue attribute: %s = %s", key, value), nil)

							if key == "Policy" {
								// Get event source ARNs from the policy
								policy := value
								policyMap := make(map[string]interface{})
								err := utils.UnmarshalJSON([]byte(policy), &policyMap)
								if err != nil {
									utils.PrintResult(debug, "", "sqs:GetQueueAttributes", fmt.Sprintf("Error parsing Policy: %s", err), nil)
								} else {
									if policyMap["Statement"] != nil {
										statements := policyMap["Statement"].([]interface{})
										for _, statement := range statements {
											statementMap := statement.(map[string]interface{})
											if statementMap["Condition"] != nil {
												condition := statementMap["Condition"].(map[string]interface{})
												if condition["ArnEquals"] != nil {
													arns := condition["ArnEquals"].(map[string]interface{})
													for _, arn := range arns {
														utils.PrintResult(debug, "", "sqs:GetQueueAttributes", fmt.Sprintf("Found event source ARN: %s", arn), nil)

														// Parse the ARN and get the event source rule
														ruleName := strings.Split(arn.(string), "/")[1]

														// Attempt describe rule
														eventBridgeSvc := awsv2.NewRegionalClient(cfg, "us-west-2", eventbridge.NewFromConfig)
														eventBridgeOutput, eventBridgeErr := eventBridgeSvc.DescribeRule(ctx, &eventbridge.DescribeRuleInput{
															Name: aws.String(ruleName),
														})

														if eventBridgeErr != nil {
															utils.HandleAWSError(debug, "eventbridge:DescribeRule", awsv2.Wrap(eventBridgeErr))
														} else {
															utils.PrintResult(debug, "", "eventbridge:DescribeRule", fmt.Sprintf("Found event source rule: %s", aws.ToString(eventBridgeOutput.Name)), nil)
														}
													}
												}
											}
										}
									}
								}
							}

							if key == "RedrivePolicy" {
								// Get the dead letter queue
								redrivePolicy := value
								redrivePolicyMap := make(map[string]interface{})
								err := utils.UnmarshalJSON([]byte(redrivePolicy), &redrivePolicyMap)
								if err != nil {
									utils.PrintResult(debug, "", "sqs:GetQueueAttributes", fmt.Sprintf("Error parsing RedrivePolicy: %s", err), nil)
								} else {
									deadLetterQueue := redrivePolicyMap["deadLetterTargetArn"]
									if deadLetterQueue != nil {
										utils.PrintResult(debug, "", "sqs:GetQueueAttributes", fmt.Sprintf("Found Dead letter queue: %s", deadLetterQueue), nil)

										deadLetterQueueArn := deadLetterQueue.(string)
										deadLetterQueueUrl := arnToQueueUrl(deadLetterQueueArn)

										// Receive messages from the dead letter queue
										deadLetterSvc := awsv2.NewRegionalClient(cfg, "us-east-1", sqs.NewFromConfig)
										deadLetterAttrOutput, deadLetterAttrErr := deadLetterSvc.ListDeadLetterSourceQueues(ctx, &sqs.ListDeadLetterSourceQueuesInput{
											QueueUrl: aws.String(deadLetterQueueUrl),
										})

										if deadLetterAttrErr != nil {
											utils.HandleAWSError(debug, "sqs:ListDeadLetterSourceQueues", awsv2.Wrap(deadLetterAttrErr))
										} else {
											if len(deadLetterAttrOutput.QueueUrls) == 0 {
												utils.PrintResult(debug, "", "sqs:ListDeadLetterSourceQueues", "Access Granted: No dead letter source queues", nil)
											}
											for _, deadLetterQueue := range deadLetterAttrOutput.QueueUrls {
												utils.PrintResult(debug, "", "sqs:ListDeadLetterSourceQueues", fmt.Sprintf("Found dead letter source queue: %s", deadLetterQueue), nil)
											}
										}

										// Receive messages from the dead letter queue
										deadLetterReceiveOutput, deadLetterReceiveErr := deadLetterSvc.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
											QueueUrl: aws.String(deadLetterQueueUrl),
										})

										if deadLetterReceiveErr != nil {
											utils.HandleAWSError(debug, "sqs:ReceiveMessage", awsv2.Wrap(deadLetterReceiveErr))
										} else {
											if len(deadLetterReceiveOutput.Messages) == 0 {
												utils.PrintResult(debug, "", "sqs:ReceiveMessage", "Access Granted: No messages in dead letter queue", nil)
											}
											for _, message := range deadLetterReceiveOutput.Messages {
												utils.PrintResult(debug, "", "sqs:ReceiveMessage", fmt.Sprintf("Received message: %s", aws.ToString(message.Body)), nil)
											}
										}
									}
								}
							}
						}
					}
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
