package secretsmanager

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"time"
)

var SecretsManagerCalls = []types.AWSService{
	{
		Name: "secretsmanager:ListSecrets",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allSecrets []*secretsmanager.SecretListEntry
			for _, region := range types.Regions {
				sess.Config.Region = aws.String(region)
				svc := secretsmanager.New(sess)
				output, err := svc.ListSecretsWithContext(ctx, &secretsmanager.ListSecretsInput{})
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

			if secrets, ok := output.([]*secretsmanager.SecretListEntry); ok {
				for _, secret := range secrets {
					secretName := ""
					if secret.Name != nil {
						secretName = *secret.Name
					}

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
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
