package kinesis

import (
	"context"
	"fmt"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
)

type kinesisStream struct {
	StreamName       string
	StreamARN        string
	StreamStatus     string
	StreamMode       string
	CreationTimestamp string
	Region           string
}

type kinesisShard struct {
	ShardId                string
	StreamName             string
	ParentShardId          string
	StartingHashKey        string
	EndingHashKey          string
	StartingSequenceNumber string
	EndingSequenceNumber   string
	Region                 string
}

type kinesisConsumer struct {
	ConsumerName     string
	ConsumerARN      string
	ConsumerStatus   string
	StreamName       string
	CreationTimestamp string
	Region           string
}

func extractStream(summary *kinesis.StreamSummary, region string) kinesisStream {
	name := ""
	if summary.StreamName != nil {
		name = *summary.StreamName
	}
	arn := ""
	if summary.StreamARN != nil {
		arn = *summary.StreamARN
	}
	status := ""
	if summary.StreamStatus != nil {
		status = *summary.StreamStatus
	}
	mode := ""
	if summary.StreamModeDetails != nil && summary.StreamModeDetails.StreamMode != nil {
		mode = *summary.StreamModeDetails.StreamMode
	}
	creationTimestamp := ""
	if summary.StreamCreationTimestamp != nil {
		creationTimestamp = summary.StreamCreationTimestamp.Format(time.RFC3339)
	}
	return kinesisStream{
		StreamName:       name,
		StreamARN:        arn,
		StreamStatus:     status,
		StreamMode:       mode,
		CreationTimestamp: creationTimestamp,
		Region:           region,
	}
}

func extractShard(shard *kinesis.Shard, streamName, region string) kinesisShard {
	shardId := ""
	if shard.ShardId != nil {
		shardId = *shard.ShardId
	}
	parentShardId := ""
	if shard.ParentShardId != nil {
		parentShardId = *shard.ParentShardId
	}
	startingHashKey := ""
	endingHashKey := ""
	if shard.HashKeyRange != nil {
		if shard.HashKeyRange.StartingHashKey != nil {
			startingHashKey = *shard.HashKeyRange.StartingHashKey
		}
		if shard.HashKeyRange.EndingHashKey != nil {
			endingHashKey = *shard.HashKeyRange.EndingHashKey
		}
	}
	startingSeqNum := ""
	endingSeqNum := ""
	if shard.SequenceNumberRange != nil {
		if shard.SequenceNumberRange.StartingSequenceNumber != nil {
			startingSeqNum = *shard.SequenceNumberRange.StartingSequenceNumber
		}
		if shard.SequenceNumberRange.EndingSequenceNumber != nil {
			endingSeqNum = *shard.SequenceNumberRange.EndingSequenceNumber
		}
	}
	return kinesisShard{
		ShardId:                shardId,
		StreamName:             streamName,
		ParentShardId:          parentShardId,
		StartingHashKey:        startingHashKey,
		EndingHashKey:          endingHashKey,
		StartingSequenceNumber: startingSeqNum,
		EndingSequenceNumber:   endingSeqNum,
		Region:                 region,
	}
}

func extractConsumer(consumer *kinesis.Consumer, streamName, region string) kinesisConsumer {
	name := ""
	if consumer.ConsumerName != nil {
		name = *consumer.ConsumerName
	}
	arn := ""
	if consumer.ConsumerARN != nil {
		arn = *consumer.ConsumerARN
	}
	status := ""
	if consumer.ConsumerStatus != nil {
		status = *consumer.ConsumerStatus
	}
	creationTimestamp := ""
	if consumer.ConsumerCreationTimestamp != nil {
		creationTimestamp = consumer.ConsumerCreationTimestamp.Format(time.RFC3339)
	}
	return kinesisConsumer{
		ConsumerName:     name,
		ConsumerARN:      arn,
		ConsumerStatus:   status,
		StreamName:       streamName,
		CreationTimestamp: creationTimestamp,
		Region:           region,
	}
}

var KinesisCalls = []types.AWSService{
	{
		Name: "kinesis:ListStreams",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allStreams []kinesisStream
			var lastErr error

			for _, region := range types.Regions {
				svc := kinesis.New(sess, &aws.Config{Region: aws.String(region)})
				var nextToken *string
				for {
					input := &kinesis.ListStreamsInput{}
					if nextToken != nil {
						input.NextToken = nextToken
					}
					output, err := svc.ListStreamsWithContext(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "kinesis:ListStreams", err)
						break
					}
					for _, summary := range output.StreamSummaries {
						if summary != nil {
							allStreams = append(allStreams, extractStream(summary, region))
						}
					}
					if output.NextToken == nil {
						break
					}
					nextToken = output.NextToken
				}
			}

			if len(allStreams) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allStreams, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "kinesis:ListStreams", err)
				return []types.ScanResult{
					{
						ServiceName: "Kinesis",
						MethodName:  "kinesis:ListStreams",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			streams, ok := output.([]kinesisStream)
			if !ok {
				utils.HandleAWSError(debug, "kinesis:ListStreams", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, s := range streams {
				results = append(results, types.ScanResult{
					ServiceName:  "Kinesis",
					MethodName:   "kinesis:ListStreams",
					ResourceType: "stream",
					ResourceName: s.StreamName,
					Details: map[string]interface{}{
						"StreamName":       s.StreamName,
						"StreamARN":        s.StreamARN,
						"StreamStatus":     s.StreamStatus,
						"StreamMode":       s.StreamMode,
						"CreationTimestamp": s.CreationTimestamp,
						"Region":           s.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "kinesis:ListStreams",
					fmt.Sprintf("Kinesis Stream: %s (Status: %s, Mode: %s, Region: %s)", utils.ColorizeItem(s.StreamName), s.StreamStatus, s.StreamMode, s.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "kinesis:ListShards",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allShards []kinesisShard
			var lastErr error

			for _, region := range types.Regions {
				svc := kinesis.New(sess, &aws.Config{Region: aws.String(region)})

				// Step 1: List all stream names
				var streamNames []string
				var streamToken *string
				for {
					input := &kinesis.ListStreamsInput{}
					if streamToken != nil {
						input.NextToken = streamToken
					}
					output, err := svc.ListStreamsWithContext(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "kinesis:ListShards", err)
						break
					}
					for _, summary := range output.StreamSummaries {
						if summary != nil && summary.StreamName != nil {
							streamNames = append(streamNames, *summary.StreamName)
						}
					}
					if output.NextToken == nil {
						break
					}
					streamToken = output.NextToken
				}

				// Step 2: For each stream, list shards
				for _, streamName := range streamNames {
					var shardToken *string
					first := true
					for {
						input := &kinesis.ListShardsInput{}
						if first {
							input.StreamName = aws.String(streamName)
							first = false
						} else {
							input.NextToken = shardToken
						}
						output, err := svc.ListShardsWithContext(ctx, input)
						if err != nil {
							lastErr = err
							utils.HandleAWSError(false, "kinesis:ListShards", err)
							break
						}
						for _, shard := range output.Shards {
							if shard != nil {
								allShards = append(allShards, extractShard(shard, streamName, region))
							}
						}
						if output.NextToken == nil {
							break
						}
						shardToken = output.NextToken
					}
				}
			}

			if len(allShards) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allShards, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "kinesis:ListShards", err)
				return []types.ScanResult{
					{
						ServiceName: "Kinesis",
						MethodName:  "kinesis:ListShards",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			shards, ok := output.([]kinesisShard)
			if !ok {
				utils.HandleAWSError(debug, "kinesis:ListShards", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, s := range shards {
				results = append(results, types.ScanResult{
					ServiceName:  "Kinesis",
					MethodName:   "kinesis:ListShards",
					ResourceType: "shard",
					ResourceName: s.ShardId,
					Details: map[string]interface{}{
						"ShardId":                s.ShardId,
						"StreamName":             s.StreamName,
						"ParentShardId":          s.ParentShardId,
						"StartingHashKey":        s.StartingHashKey,
						"EndingHashKey":          s.EndingHashKey,
						"StartingSequenceNumber": s.StartingSequenceNumber,
						"EndingSequenceNumber":   s.EndingSequenceNumber,
						"Region":                 s.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "kinesis:ListShards",
					fmt.Sprintf("Kinesis Shard: %s (Stream: %s, Region: %s)", utils.ColorizeItem(s.ShardId), s.StreamName, s.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "kinesis:ListStreamConsumers",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allConsumers []kinesisConsumer
			var lastErr error

			for _, region := range types.Regions {
				svc := kinesis.New(sess, &aws.Config{Region: aws.String(region)})

				// Step 1: List all stream names and ARNs
				var streams []struct{ Name, ARN string }
				var streamToken *string
				for {
					input := &kinesis.ListStreamsInput{}
					if streamToken != nil {
						input.NextToken = streamToken
					}
					output, err := svc.ListStreamsWithContext(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "kinesis:ListStreamConsumers", err)
						break
					}
					for _, summary := range output.StreamSummaries {
						if summary != nil && summary.StreamARN != nil {
							name := ""
							if summary.StreamName != nil {
								name = *summary.StreamName
							}
							streams = append(streams, struct{ Name, ARN string }{name, *summary.StreamARN})
						}
					}
					if output.NextToken == nil {
						break
					}
					streamToken = output.NextToken
				}

				// Step 2: For each stream, list consumers
				for _, stream := range streams {
					var consumerToken *string
					for {
						input := &kinesis.ListStreamConsumersInput{
							StreamARN: aws.String(stream.ARN),
						}
						if consumerToken != nil {
							input.NextToken = consumerToken
						}
						output, err := svc.ListStreamConsumersWithContext(ctx, input)
						if err != nil {
							lastErr = err
							utils.HandleAWSError(false, "kinesis:ListStreamConsumers", err)
							break
						}
						for _, consumer := range output.Consumers {
							if consumer != nil {
								allConsumers = append(allConsumers, extractConsumer(consumer, stream.Name, region))
							}
						}
						if output.NextToken == nil {
							break
						}
						consumerToken = output.NextToken
					}
				}
			}

			if len(allConsumers) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allConsumers, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "kinesis:ListStreamConsumers", err)
				return []types.ScanResult{
					{
						ServiceName: "Kinesis",
						MethodName:  "kinesis:ListStreamConsumers",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			consumers, ok := output.([]kinesisConsumer)
			if !ok {
				utils.HandleAWSError(debug, "kinesis:ListStreamConsumers", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, c := range consumers {
				results = append(results, types.ScanResult{
					ServiceName:  "Kinesis",
					MethodName:   "kinesis:ListStreamConsumers",
					ResourceType: "consumer",
					ResourceName: c.ConsumerName,
					Details: map[string]interface{}{
						"ConsumerName":     c.ConsumerName,
						"ConsumerARN":      c.ConsumerARN,
						"ConsumerStatus":   c.ConsumerStatus,
						"StreamName":       c.StreamName,
						"CreationTimestamp": c.CreationTimestamp,
						"Region":           c.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "kinesis:ListStreamConsumers",
					fmt.Sprintf("Kinesis Consumer: %s (Stream: %s, Status: %s, Region: %s)", utils.ColorizeItem(c.ConsumerName), c.StreamName, c.ConsumerStatus, c.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
