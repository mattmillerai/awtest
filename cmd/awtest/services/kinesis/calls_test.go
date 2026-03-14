package kinesis

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/kinesis"
)

func TestListStreamsProcess(t *testing.T) {
	process := KinesisCalls[0].Process

	tests := []struct {
		name              string
		output            interface{}
		err               error
		wantLen           int
		wantError         bool
		wantResourceName  string
		wantStreamName    string
		wantStreamARN     string
		wantStreamStatus  string
		wantStreamMode    string
		wantCreated       string
		wantRegion        string
	}{
		{
			name: "valid streams with full details",
			output: []kinesisStream{
				{
					StreamName:       "my-data-stream",
					StreamARN:        "arn:aws:kinesis:us-east-1:123456789012:stream/my-data-stream",
					StreamStatus:     "ACTIVE",
					StreamMode:       "ON_DEMAND",
					CreationTimestamp: "2026-01-15T10:00:00Z",
					Region:           "us-east-1",
				},
				{
					StreamName:       "event-stream",
					StreamARN:        "arn:aws:kinesis:us-west-2:123456789012:stream/event-stream",
					StreamStatus:     "CREATING",
					StreamMode:       "PROVISIONED",
					CreationTimestamp: "2026-02-20T14:00:00Z",
					Region:           "us-west-2",
				},
			},
			wantLen:          2,
			wantResourceName: "my-data-stream",
			wantStreamName:   "my-data-stream",
			wantStreamARN:    "arn:aws:kinesis:us-east-1:123456789012:stream/my-data-stream",
			wantStreamStatus: "ACTIVE",
			wantStreamMode:   "ON_DEMAND",
			wantCreated:      "2026-01-15T10:00:00Z",
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty results",
			output:  []kinesisStream{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
		},
		{
			name: "nil-safe fields (empty strings)",
			output: []kinesisStream{
				{
					StreamName:       "",
					StreamARN:        "",
					StreamStatus:     "",
					StreamMode:       "",
					CreationTimestamp: "",
					Region:           "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantStreamName:   "",
			wantStreamARN:    "",
			wantStreamStatus: "",
			wantStreamMode:   "",
			wantCreated:      "",
			wantRegion:       "",
		},
		{
			name:    "type assertion failure",
			output:  "invalid type",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := process(tt.output, tt.err, false)

			if len(results) != tt.wantLen {
				t.Fatalf("expected %d results, got %d", tt.wantLen, len(results))
			}

			if tt.wantError {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "Kinesis" {
					t.Errorf("expected ServiceName 'Kinesis', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "kinesis:ListStreams" {
					t.Errorf("expected MethodName 'kinesis:ListStreams', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "Kinesis" {
					t.Errorf("expected ServiceName 'Kinesis', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "kinesis:ListStreams" {
					t.Errorf("expected MethodName 'kinesis:ListStreams', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "stream" {
					t.Errorf("expected ResourceType 'stream', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if v, ok := results[0].Details["StreamName"].(string); ok {
					if v != tt.wantStreamName {
						t.Errorf("expected StreamName '%s', got '%s'", tt.wantStreamName, v)
					}
				} else if tt.wantStreamName != "" {
					t.Errorf("expected StreamName in Details, got none")
				}
				if v, ok := results[0].Details["StreamARN"].(string); ok {
					if v != tt.wantStreamARN {
						t.Errorf("expected StreamARN '%s', got '%s'", tt.wantStreamARN, v)
					}
				} else if tt.wantStreamARN != "" {
					t.Errorf("expected StreamARN in Details, got none")
				}
				if v, ok := results[0].Details["StreamStatus"].(string); ok {
					if v != tt.wantStreamStatus {
						t.Errorf("expected StreamStatus '%s', got '%s'", tt.wantStreamStatus, v)
					}
				} else if tt.wantStreamStatus != "" {
					t.Errorf("expected StreamStatus in Details, got none")
				}
				if v, ok := results[0].Details["StreamMode"].(string); ok {
					if v != tt.wantStreamMode {
						t.Errorf("expected StreamMode '%s', got '%s'", tt.wantStreamMode, v)
					}
				} else if tt.wantStreamMode != "" {
					t.Errorf("expected StreamMode in Details, got none")
				}
				if v, ok := results[0].Details["CreationTimestamp"].(string); ok {
					if v != tt.wantCreated {
						t.Errorf("expected CreationTimestamp '%s', got '%s'", tt.wantCreated, v)
					}
				} else if tt.wantCreated != "" {
					t.Errorf("expected CreationTimestamp in Details, got none")
				}
				if v, ok := results[0].Details["Region"].(string); ok {
					if v != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, v)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}

func TestListShardsProcess(t *testing.T) {
	process := KinesisCalls[1].Process

	tests := []struct {
		name              string
		output            interface{}
		err               error
		wantLen           int
		wantError         bool
		wantResourceName  string
		wantShardId       string
		wantStreamName    string
		wantParentShardId string
		wantStartHash     string
		wantEndHash       string
		wantStartSeq      string
		wantEndSeq        string
		wantRegion        string
	}{
		{
			name: "valid shards with full details",
			output: []kinesisShard{
				{
					ShardId:                "shardId-000000000000",
					StreamName:             "my-data-stream",
					ParentShardId:          "",
					StartingHashKey:        "0",
					EndingHashKey:          "170141183460469231731687303715884105727",
					StartingSequenceNumber: "49590338271490256608559692538361571095921575989136588802",
					EndingSequenceNumber:   "",
					Region:                 "us-east-1",
				},
				{
					ShardId:                "shardId-000000000001",
					StreamName:             "my-data-stream",
					ParentShardId:          "shardId-000000000000",
					StartingHashKey:        "170141183460469231731687303715884105728",
					EndingHashKey:          "340282366920938463463374607431768211455",
					StartingSequenceNumber: "49590338271512557353758223161503106818843136405372436482",
					EndingSequenceNumber:   "49590338271534858098956753784644642541764696821608284162",
					Region:                 "us-east-1",
				},
			},
			wantLen:           2,
			wantResourceName:  "shardId-000000000000",
			wantShardId:       "shardId-000000000000",
			wantStreamName:    "my-data-stream",
			wantParentShardId: "",
			wantStartHash:     "0",
			wantEndHash:       "170141183460469231731687303715884105727",
			wantStartSeq:      "49590338271490256608559692538361571095921575989136588802",
			wantEndSeq:        "",
			wantRegion:        "us-east-1",
		},
		{
			name:    "empty results",
			output:  []kinesisShard{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
		},
		{
			name: "nil-safe fields (empty strings)",
			output: []kinesisShard{
				{
					ShardId:                "",
					StreamName:             "",
					ParentShardId:          "",
					StartingHashKey:        "",
					EndingHashKey:          "",
					StartingSequenceNumber: "",
					EndingSequenceNumber:   "",
					Region:                 "",
				},
			},
			wantLen:           1,
			wantResourceName:  "",
			wantShardId:       "",
			wantStreamName:    "",
			wantParentShardId: "",
			wantStartHash:     "",
			wantEndHash:       "",
			wantStartSeq:      "",
			wantEndSeq:        "",
			wantRegion:        "",
		},
		{
			name:    "type assertion failure",
			output:  "invalid type",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := process(tt.output, tt.err, false)

			if len(results) != tt.wantLen {
				t.Fatalf("expected %d results, got %d", tt.wantLen, len(results))
			}

			if tt.wantError {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "Kinesis" {
					t.Errorf("expected ServiceName 'Kinesis', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "kinesis:ListShards" {
					t.Errorf("expected MethodName 'kinesis:ListShards', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "Kinesis" {
					t.Errorf("expected ServiceName 'Kinesis', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "kinesis:ListShards" {
					t.Errorf("expected MethodName 'kinesis:ListShards', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "shard" {
					t.Errorf("expected ResourceType 'shard', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if v, ok := results[0].Details["ShardId"].(string); ok {
					if v != tt.wantShardId {
						t.Errorf("expected ShardId '%s', got '%s'", tt.wantShardId, v)
					}
				} else if tt.wantShardId != "" {
					t.Errorf("expected ShardId in Details, got none")
				}
				if v, ok := results[0].Details["StreamName"].(string); ok {
					if v != tt.wantStreamName {
						t.Errorf("expected StreamName '%s', got '%s'", tt.wantStreamName, v)
					}
				} else if tt.wantStreamName != "" {
					t.Errorf("expected StreamName in Details, got none")
				}
				if v, ok := results[0].Details["ParentShardId"].(string); ok {
					if v != tt.wantParentShardId {
						t.Errorf("expected ParentShardId '%s', got '%s'", tt.wantParentShardId, v)
					}
				} else if tt.wantParentShardId != "" {
					t.Errorf("expected ParentShardId in Details, got none")
				}
				if v, ok := results[0].Details["StartingHashKey"].(string); ok {
					if v != tt.wantStartHash {
						t.Errorf("expected StartingHashKey '%s', got '%s'", tt.wantStartHash, v)
					}
				} else if tt.wantStartHash != "" {
					t.Errorf("expected StartingHashKey in Details, got none")
				}
				if v, ok := results[0].Details["EndingHashKey"].(string); ok {
					if v != tt.wantEndHash {
						t.Errorf("expected EndingHashKey '%s', got '%s'", tt.wantEndHash, v)
					}
				} else if tt.wantEndHash != "" {
					t.Errorf("expected EndingHashKey in Details, got none")
				}
				if v, ok := results[0].Details["StartingSequenceNumber"].(string); ok {
					if v != tt.wantStartSeq {
						t.Errorf("expected StartingSequenceNumber '%s', got '%s'", tt.wantStartSeq, v)
					}
				} else if tt.wantStartSeq != "" {
					t.Errorf("expected StartingSequenceNumber in Details, got none")
				}
				if v, ok := results[0].Details["EndingSequenceNumber"].(string); ok {
					if v != tt.wantEndSeq {
						t.Errorf("expected EndingSequenceNumber '%s', got '%s'", tt.wantEndSeq, v)
					}
				} else if tt.wantEndSeq != "" {
					t.Errorf("expected EndingSequenceNumber in Details, got none")
				}
				if v, ok := results[0].Details["Region"].(string); ok {
					if v != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, v)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}

func TestListStreamConsumersProcess(t *testing.T) {
	process := KinesisCalls[2].Process

	tests := []struct {
		name              string
		output            interface{}
		err               error
		wantLen           int
		wantError         bool
		wantResourceName  string
		wantConsumerName  string
		wantConsumerARN   string
		wantConsumerStatus string
		wantStreamName    string
		wantCreated       string
		wantRegion        string
	}{
		{
			name: "valid consumers with full details",
			output: []kinesisConsumer{
				{
					ConsumerName:     "my-consumer-app",
					ConsumerARN:      "arn:aws:kinesis:us-east-1:123456789012:stream/my-data-stream/consumer/my-consumer-app:1234567890",
					ConsumerStatus:   "ACTIVE",
					StreamName:       "my-data-stream",
					CreationTimestamp: "2026-01-15T10:00:00Z",
					Region:           "us-east-1",
				},
				{
					ConsumerName:     "analytics-consumer",
					ConsumerARN:      "arn:aws:kinesis:us-west-2:123456789012:stream/event-stream/consumer/analytics-consumer:9876543210",
					ConsumerStatus:   "CREATING",
					StreamName:       "event-stream",
					CreationTimestamp: "2026-02-20T14:00:00Z",
					Region:           "us-west-2",
				},
			},
			wantLen:            2,
			wantResourceName:   "my-consumer-app",
			wantConsumerName:   "my-consumer-app",
			wantConsumerARN:    "arn:aws:kinesis:us-east-1:123456789012:stream/my-data-stream/consumer/my-consumer-app:1234567890",
			wantConsumerStatus: "ACTIVE",
			wantStreamName:     "my-data-stream",
			wantCreated:        "2026-01-15T10:00:00Z",
			wantRegion:         "us-east-1",
		},
		{
			name:    "empty results",
			output:  []kinesisConsumer{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
		},
		{
			name: "nil-safe fields (empty strings)",
			output: []kinesisConsumer{
				{
					ConsumerName:     "",
					ConsumerARN:      "",
					ConsumerStatus:   "",
					StreamName:       "",
					CreationTimestamp: "",
					Region:           "",
				},
			},
			wantLen:            1,
			wantResourceName:   "",
			wantConsumerName:   "",
			wantConsumerARN:    "",
			wantConsumerStatus: "",
			wantStreamName:     "",
			wantCreated:        "",
			wantRegion:         "",
		},
		{
			name:    "type assertion failure",
			output:  "invalid type",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := process(tt.output, tt.err, false)

			if len(results) != tt.wantLen {
				t.Fatalf("expected %d results, got %d", tt.wantLen, len(results))
			}

			if tt.wantError {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "Kinesis" {
					t.Errorf("expected ServiceName 'Kinesis', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "kinesis:ListStreamConsumers" {
					t.Errorf("expected MethodName 'kinesis:ListStreamConsumers', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "Kinesis" {
					t.Errorf("expected ServiceName 'Kinesis', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "kinesis:ListStreamConsumers" {
					t.Errorf("expected MethodName 'kinesis:ListStreamConsumers', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "consumer" {
					t.Errorf("expected ResourceType 'consumer', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if v, ok := results[0].Details["ConsumerName"].(string); ok {
					if v != tt.wantConsumerName {
						t.Errorf("expected ConsumerName '%s', got '%s'", tt.wantConsumerName, v)
					}
				} else if tt.wantConsumerName != "" {
					t.Errorf("expected ConsumerName in Details, got none")
				}
				if v, ok := results[0].Details["ConsumerARN"].(string); ok {
					if v != tt.wantConsumerARN {
						t.Errorf("expected ConsumerARN '%s', got '%s'", tt.wantConsumerARN, v)
					}
				} else if tt.wantConsumerARN != "" {
					t.Errorf("expected ConsumerARN in Details, got none")
				}
				if v, ok := results[0].Details["ConsumerStatus"].(string); ok {
					if v != tt.wantConsumerStatus {
						t.Errorf("expected ConsumerStatus '%s', got '%s'", tt.wantConsumerStatus, v)
					}
				} else if tt.wantConsumerStatus != "" {
					t.Errorf("expected ConsumerStatus in Details, got none")
				}
				if v, ok := results[0].Details["StreamName"].(string); ok {
					if v != tt.wantStreamName {
						t.Errorf("expected StreamName '%s', got '%s'", tt.wantStreamName, v)
					}
				} else if tt.wantStreamName != "" {
					t.Errorf("expected StreamName in Details, got none")
				}
				if v, ok := results[0].Details["CreationTimestamp"].(string); ok {
					if v != tt.wantCreated {
						t.Errorf("expected CreationTimestamp '%s', got '%s'", tt.wantCreated, v)
					}
				} else if tt.wantCreated != "" {
					t.Errorf("expected CreationTimestamp in Details, got none")
				}
				if v, ok := results[0].Details["Region"].(string); ok {
					if v != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, v)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}

func TestExtractStream(t *testing.T) {
	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		input      *kinesis.StreamSummary
		region     string
		wantName   string
		wantARN    string
		wantStatus string
		wantMode   string
		wantCreated string
		wantRegion string
	}{
		{
			name: "all fields populated",
			input: &kinesis.StreamSummary{
				StreamName:   aws.String("my-stream"),
				StreamARN:    aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/my-stream"),
				StreamStatus: aws.String("ACTIVE"),
				StreamModeDetails: &kinesis.StreamModeDetails{
					StreamMode: aws.String("ON_DEMAND"),
				},
				StreamCreationTimestamp: &ts,
			},
			region:      "us-east-1",
			wantName:    "my-stream",
			wantARN:     "arn:aws:kinesis:us-east-1:123456789012:stream/my-stream",
			wantStatus:  "ACTIVE",
			wantMode:    "ON_DEMAND",
			wantCreated: "2026-01-15T10:00:00Z",
			wantRegion:  "us-east-1",
		},
		{
			name:        "all fields nil",
			input:       &kinesis.StreamSummary{},
			region:      "eu-west-1",
			wantName:    "",
			wantARN:     "",
			wantStatus:  "",
			wantMode:    "",
			wantCreated: "",
			wantRegion:  "eu-west-1",
		},
		{
			name: "stream mode details present but stream mode nil",
			input: &kinesis.StreamSummary{
				StreamName:        aws.String("partial-stream"),
				StreamModeDetails: &kinesis.StreamModeDetails{},
			},
			region:      "us-west-2",
			wantName:    "partial-stream",
			wantARN:     "",
			wantStatus:  "",
			wantMode:    "",
			wantCreated: "",
			wantRegion:  "us-west-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractStream(tt.input, tt.region)
			if result.StreamName != tt.wantName {
				t.Errorf("StreamName: got %q, want %q", result.StreamName, tt.wantName)
			}
			if result.StreamARN != tt.wantARN {
				t.Errorf("StreamARN: got %q, want %q", result.StreamARN, tt.wantARN)
			}
			if result.StreamStatus != tt.wantStatus {
				t.Errorf("StreamStatus: got %q, want %q", result.StreamStatus, tt.wantStatus)
			}
			if result.StreamMode != tt.wantMode {
				t.Errorf("StreamMode: got %q, want %q", result.StreamMode, tt.wantMode)
			}
			if result.CreationTimestamp != tt.wantCreated {
				t.Errorf("CreationTimestamp: got %q, want %q", result.CreationTimestamp, tt.wantCreated)
			}
			if result.Region != tt.wantRegion {
				t.Errorf("Region: got %q, want %q", result.Region, tt.wantRegion)
			}
		})
	}
}

func TestExtractShard(t *testing.T) {
	tests := []struct {
		name           string
		input          *kinesis.Shard
		streamName     string
		region         string
		wantShardId    string
		wantParent     string
		wantStartHash  string
		wantEndHash    string
		wantStartSeq   string
		wantEndSeq     string
		wantRegion     string
	}{
		{
			name: "all fields populated",
			input: &kinesis.Shard{
				ShardId:       aws.String("shardId-000000000000"),
				ParentShardId: aws.String("shardId-parent"),
				HashKeyRange: &kinesis.HashKeyRange{
					StartingHashKey: aws.String("0"),
					EndingHashKey:   aws.String("340282366920938463463374607431768211455"),
				},
				SequenceNumberRange: &kinesis.SequenceNumberRange{
					StartingSequenceNumber: aws.String("49590338271490256608559692538361571095921575989136588802"),
					EndingSequenceNumber:   aws.String("49590338271534858098956753784644642541764696821608284162"),
				},
			},
			streamName:    "my-stream",
			region:        "us-east-1",
			wantShardId:   "shardId-000000000000",
			wantParent:    "shardId-parent",
			wantStartHash: "0",
			wantEndHash:   "340282366920938463463374607431768211455",
			wantStartSeq:  "49590338271490256608559692538361571095921575989136588802",
			wantEndSeq:    "49590338271534858098956753784644642541764696821608284162",
			wantRegion:    "us-east-1",
		},
		{
			name:          "all fields nil",
			input:         &kinesis.Shard{},
			streamName:    "empty-stream",
			region:        "eu-west-1",
			wantShardId:   "",
			wantParent:    "",
			wantStartHash: "",
			wantEndHash:   "",
			wantStartSeq:  "",
			wantEndSeq:    "",
			wantRegion:    "eu-west-1",
		},
		{
			name: "hash key range present but keys nil",
			input: &kinesis.Shard{
				ShardId:      aws.String("shardId-000000000001"),
				HashKeyRange: &kinesis.HashKeyRange{},
				SequenceNumberRange: &kinesis.SequenceNumberRange{
					StartingSequenceNumber: aws.String("12345"),
				},
			},
			streamName:    "partial-stream",
			region:        "us-west-2",
			wantShardId:   "shardId-000000000001",
			wantParent:    "",
			wantStartHash: "",
			wantEndHash:   "",
			wantStartSeq:  "12345",
			wantEndSeq:    "",
			wantRegion:    "us-west-2",
		},
		{
			name: "sequence number range present but numbers nil",
			input: &kinesis.Shard{
				ShardId: aws.String("shardId-000000000002"),
				HashKeyRange: &kinesis.HashKeyRange{
					StartingHashKey: aws.String("100"),
				},
				SequenceNumberRange: &kinesis.SequenceNumberRange{},
			},
			streamName:    "another-stream",
			region:        "ap-southeast-1",
			wantShardId:   "shardId-000000000002",
			wantParent:    "",
			wantStartHash: "100",
			wantEndHash:   "",
			wantStartSeq:  "",
			wantEndSeq:    "",
			wantRegion:    "ap-southeast-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractShard(tt.input, tt.streamName, tt.region)
			if result.ShardId != tt.wantShardId {
				t.Errorf("ShardId: got %q, want %q", result.ShardId, tt.wantShardId)
			}
			if result.StreamName != tt.streamName {
				t.Errorf("StreamName: got %q, want %q", result.StreamName, tt.streamName)
			}
			if result.ParentShardId != tt.wantParent {
				t.Errorf("ParentShardId: got %q, want %q", result.ParentShardId, tt.wantParent)
			}
			if result.StartingHashKey != tt.wantStartHash {
				t.Errorf("StartingHashKey: got %q, want %q", result.StartingHashKey, tt.wantStartHash)
			}
			if result.EndingHashKey != tt.wantEndHash {
				t.Errorf("EndingHashKey: got %q, want %q", result.EndingHashKey, tt.wantEndHash)
			}
			if result.StartingSequenceNumber != tt.wantStartSeq {
				t.Errorf("StartingSequenceNumber: got %q, want %q", result.StartingSequenceNumber, tt.wantStartSeq)
			}
			if result.EndingSequenceNumber != tt.wantEndSeq {
				t.Errorf("EndingSequenceNumber: got %q, want %q", result.EndingSequenceNumber, tt.wantEndSeq)
			}
			if result.Region != tt.wantRegion {
				t.Errorf("Region: got %q, want %q", result.Region, tt.wantRegion)
			}
		})
	}
}

func TestExtractConsumer(t *testing.T) {
	ts := time.Date(2026, 2, 20, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		input       *kinesis.Consumer
		streamName  string
		region      string
		wantName    string
		wantARN     string
		wantStatus  string
		wantCreated string
		wantRegion  string
	}{
		{
			name: "all fields populated",
			input: &kinesis.Consumer{
				ConsumerName:              aws.String("my-consumer"),
				ConsumerARN:               aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/my-stream/consumer/my-consumer:1234567890"),
				ConsumerStatus:            aws.String("ACTIVE"),
				ConsumerCreationTimestamp: &ts,
			},
			streamName:  "my-stream",
			region:      "us-east-1",
			wantName:    "my-consumer",
			wantARN:     "arn:aws:kinesis:us-east-1:123456789012:stream/my-stream/consumer/my-consumer:1234567890",
			wantStatus:  "ACTIVE",
			wantCreated: "2026-02-20T14:00:00Z",
			wantRegion:  "us-east-1",
		},
		{
			name:        "all fields nil",
			input:       &kinesis.Consumer{},
			streamName:  "empty-stream",
			region:      "eu-west-1",
			wantName:    "",
			wantARN:     "",
			wantStatus:  "",
			wantCreated: "",
			wantRegion:  "eu-west-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractConsumer(tt.input, tt.streamName, tt.region)
			if result.ConsumerName != tt.wantName {
				t.Errorf("ConsumerName: got %q, want %q", result.ConsumerName, tt.wantName)
			}
			if result.ConsumerARN != tt.wantARN {
				t.Errorf("ConsumerARN: got %q, want %q", result.ConsumerARN, tt.wantARN)
			}
			if result.ConsumerStatus != tt.wantStatus {
				t.Errorf("ConsumerStatus: got %q, want %q", result.ConsumerStatus, tt.wantStatus)
			}
			if result.StreamName != tt.streamName {
				t.Errorf("StreamName: got %q, want %q", result.StreamName, tt.streamName)
			}
			if result.CreationTimestamp != tt.wantCreated {
				t.Errorf("CreationTimestamp: got %q, want %q", result.CreationTimestamp, tt.wantCreated)
			}
			if result.Region != tt.wantRegion {
				t.Errorf("Region: got %q, want %q", result.Region, tt.wantRegion)
			}
		})
	}
}
