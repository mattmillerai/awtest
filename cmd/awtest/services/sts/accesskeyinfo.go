package sts

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

// STS is global: this entry runs once per scan and does not consult the region scope.
func init() {
	STSCalls = append(STSCalls, getAccessKeyInfoCall)
}

// getAccessKeyInfoCall resolves the account that owns the scan's own access key.
var getAccessKeyInfoCall = types.AWSService{
	Name: "sts:GetAccessKeyInfo",
	CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
		if cfg.Credentials == nil {
			return nil, nil
		}
		creds, err := cfg.Credentials.Retrieve(ctx)
		if err != nil || creds.AccessKeyID == "" {
			// No retrievable access key: nothing to look up, not a failure.
			return nil, nil
		}
		svc := awsv2.NewRegionalClient(cfg, "", sts.NewFromConfig)
		return svc.GetAccessKeyInfo(ctx, &sts.GetAccessKeyInfoInput{AccessKeyId: aws.String(creds.AccessKeyID)})
	},
	Process: func(output interface{}, err error, debug bool) []types.ScanResult {
		if err != nil {
			err = awsv2.Wrap(err)
			utils.HandleAWSError(debug, "sts:GetAccessKeyInfo", err)
			return []types.ScanResult{{ServiceName: "STS", MethodName: "sts:GetAccessKeyInfo", Error: err, Timestamp: time.Now()}}
		}
		out, _ := output.(*sts.GetAccessKeyInfoOutput)
		if out == nil {
			return nil
		}
		account := aws.ToString(out.Account)
		utils.PrintResult(debug, "", "sts:GetAccessKeyInfo", fmt.Sprintf("Access key account: %s", utils.ColorizeItem(account)), nil)
		return []types.ScanResult{{
			ServiceName:  "STS",
			MethodName:   "sts:GetAccessKeyInfo",
			ResourceType: "access-key",
			ResourceName: account,
			Details:      map[string]interface{}{"account": account},
			Timestamp:    time.Now(),
		}}
	},
	ModuleName: types.DefaultModuleName,
}
