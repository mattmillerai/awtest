package glacier

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glacier"
	glaciertypes "github.com/aws/aws-sdk-go-v2/service/glacier/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var GlacierCalls = []types.AWSService{
	{
		Name: "glacier:ListVaults",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", glacier.NewFromConfig)
			output, err := svc.ListVaults(ctx, &glacier.ListVaultsInput{})
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

			vaults, ok := output.([]glaciertypes.DescribeVaultOutput)
			if !ok {
				utils.HandleAWSError(debug, "glacier:ListVaults", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, vault := range vaults {
				vaultName := aws.ToString(vault.VaultName)
				results = append(results, types.ScanResult{
					ServiceName:  "Glacier",
					MethodName:   "glacier:ListVaults",
					ResourceType: "vault",
					ResourceName: vaultName,
					Details:      map[string]interface{}{},
					Timestamp:    time.Now(),
				})

				utils.PrintResult(debug, "", "glacier:ListVaults", fmt.Sprintf("Glacier vault: %s", utils.ColorizeItem(vaultName)), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
