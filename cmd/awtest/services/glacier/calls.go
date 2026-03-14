package glacier

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/glacier"
	"time"
)

var GlacierCalls = []types.AWSService{
	{
		Name: "glacier:ListVaults",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := glacier.New(sess)
			output, err := svc.ListVaultsWithContext(ctx, &glacier.ListVaultsInput{})
			if err != nil {
				return nil, err
			}
			return output.VaultList, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "glacier:ListVaults", err)
				return []types.ScanResult{
					{
						ServiceName: "Glacier",
						MethodName:  "glacier:ListVaults",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if vaults, ok := output.([]*glacier.DescribeVaultOutput); ok {
				for _, vault := range vaults {
					results = append(results, types.ScanResult{
						ServiceName:  "Glacier",
						MethodName:   "glacier:ListVaults",
						ResourceType: "vault",
						ResourceName: *vault.VaultName,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "glacier:ListVaults", fmt.Sprintf("Glacier vault: %s", utils.ColorizeItem(*vault.VaultName)), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
