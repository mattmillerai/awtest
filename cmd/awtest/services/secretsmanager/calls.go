package secretsmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var SecretsManagerCalls = []types.AWSService{
	{
		Name: "secretsmanager:ListSecrets",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allSecrets []smtypes.SecretListEntry
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, secretsmanager.NewFromConfig)
				output, err := svc.ListSecrets(ctx, &secretsmanager.ListSecretsInput{})
				if err != nil {
					return nil, err
				}
				allSecrets = append(allSecrets, output.SecretList...)
			}
			return allSecrets, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "secretsmanager:ListSecrets", err)
				return []types.ScanResult{
					{
						ServiceName: "SecretsManager",
						MethodName:  "secretsmanager:ListSecrets",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			secrets, ok := output.([]smtypes.SecretListEntry)
			if !ok {
				utils.HandleAWSError(debug, "secretsmanager:ListSecrets", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, secret := range secrets {
				secretName := aws.ToString(secret.Name)

				results = append(results, types.ScanResult{
					ServiceName:  "SecretsManager",
					MethodName:   "secretsmanager:ListSecrets",
					ResourceType: "secret",
					ResourceName: secretName,
					Details:      map[string]interface{}{},
					Timestamp:    time.Now(),
				})

				utils.PrintResult(debug, "", "secretsmanager:ListSecrets", fmt.Sprintf("Found secret: %s", secretName), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
