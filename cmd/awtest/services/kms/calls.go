package kms

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	"time"
)

var KMSCalls = []types.AWSService{
	{
		Name: "kms:ListKeys",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allKeys []*kms.KeyListEntry
			for _, region := range types.Regions {
				sess.Config.Region = aws.String(region)
				svc := kms.New(sess)
				output, err := svc.ListKeysWithContext(ctx, &kms.ListKeysInput{})
				if err != nil {
					return nil, err
				}
				allKeys = append(allKeys, output.Keys...)
			}
			return allKeys, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "kms:ListKeys", err)
				return []types.ScanResult{
					{
						ServiceName: "KMS",
						MethodName:  "kms:ListKeys",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if keys, ok := output.([]*kms.KeyListEntry); ok {
				for _, key := range keys {
					keyId := ""
					if key.KeyId != nil {
						keyId = *key.KeyId
					}

					results = append(results, types.ScanResult{
						ServiceName:  "KMS",
						MethodName:   "kms:ListKeys",
						ResourceType: "key",
						ResourceName: keyId,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "kms:ListKeys", fmt.Sprintf("KMS key: %s", utils.ColorizeItem(keyId)), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
