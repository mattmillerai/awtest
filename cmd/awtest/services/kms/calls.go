package kms

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var KMSCalls = []types.AWSService{
	{
		Name: "kms:ListKeys",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allKeys []kmstypes.KeyListEntry
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, kms.NewFromConfig)
				output, err := svc.ListKeys(ctx, &kms.ListKeysInput{})
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

			if keys, ok := output.([]kmstypes.KeyListEntry); ok {
				for _, key := range keys {
					keyId := aws.ToString(key.KeyId)

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
