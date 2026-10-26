package ivs

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ivs"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var IvsCalls = []types.AWSService{
	{
		Name: "ivs:ListChannels",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", ivs.NewFromConfig)
			input := &ivs.ListChannelsInput{}
			return svc.ListChannels(ctx, input)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "ivs:ListChannels", err)
				return []types.ScanResult{
					{
						ServiceName: "IVS",
						MethodName:  "ivs:ListChannels",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if channels, ok := output.(*ivs.ListChannelsOutput); ok {
				for _, channel := range channels.Channels {
					arn := aws.ToString(channel.Arn)
					utils.PrintResult(debug, "", "ivs:ListChannels", fmt.Sprintf("Channel: %s", arn), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "IVS",
						MethodName:   "ivs:ListChannels",
						ResourceType: "channel",
						ResourceName: arn,
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
		Name: "ivs:ListStreams",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", ivs.NewFromConfig)
			input := &ivs.ListStreamsInput{}
			return svc.ListStreams(ctx, input)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "ivs:ListStreams", err)
				return []types.ScanResult{
					{
						ServiceName: "IVS",
						MethodName:  "ivs:ListStreams",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if streamsOutput, ok := output.(*ivs.ListStreamsOutput); ok {
				for _, stream := range streamsOutput.Streams {
					id := aws.ToString(stream.StreamId)
					utils.PrintResult(debug, "", "ivs:ListStreams", fmt.Sprintf("Stream: %s", id), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "IVS",
						MethodName:   "ivs:ListStreams",
						ResourceType: "stream",
						ResourceName: id,
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
		Name: "ivs:ListStreamKeys",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", ivs.NewFromConfig)
			input := &ivs.ListStreamKeysInput{}
			return svc.ListStreamKeys(ctx, input)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "ivs:ListStreamKeys", err)
				return []types.ScanResult{
					{
						ServiceName: "IVS",
						MethodName:  "ivs:ListStreamKeys",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if streamKeysOutput, ok := output.(*ivs.ListStreamKeysOutput); ok {
				for _, streamKey := range streamKeysOutput.StreamKeys {
					arn := aws.ToString(streamKey.Arn)
					utils.PrintResult(debug, "", "ivs:ListStreamKeys", fmt.Sprintf("StreamKey: %s", arn), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "IVS",
						MethodName:   "ivs:ListStreamKeys",
						ResourceType: "stream-key",
						ResourceName: arn,
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
