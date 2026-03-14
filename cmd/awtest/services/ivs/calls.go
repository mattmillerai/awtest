package ivs

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ivs"
	"time"
)

var IvsCalls = []types.AWSService{
	{
		Name: "ivs:ListChannels",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := ivs.New(sess)
			input := &ivs.ListChannelsInput{}
			return svc.ListChannelsWithContext(ctx, input)
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
					utils.PrintResult(debug, "", "ivs:ListChannels", fmt.Sprintf("Channel: %s", *channel.Arn), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "IVS",
						MethodName:   "ivs:ListChannels",
						ResourceType: "channel",
						ResourceName: *channel.Arn,
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
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := ivs.New(sess)
			input := &ivs.ListStreamsInput{}
			return svc.ListStreamsWithContext(ctx, input)
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
					utils.PrintResult(debug, "", "ivs:ListStreams", fmt.Sprintf("Stream: %s", *stream.StreamId), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "IVS",
						MethodName:   "ivs:ListStreams",
						ResourceType: "stream",
						ResourceName: *stream.StreamId,
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
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := ivs.New(sess)
			input := &ivs.ListStreamKeysInput{}
			return svc.ListStreamKeysWithContext(ctx, input)
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
					utils.PrintResult(debug, "", "ivs:ListStreamKeys", fmt.Sprintf("StreamKey: %s", *streamKey.Arn), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "IVS",
						MethodName:   "ivs:ListStreamKeys",
						ResourceType: "stream-key",
						ResourceName: *streamKey.Arn,
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
