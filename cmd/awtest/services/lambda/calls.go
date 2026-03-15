package lambda

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"time"
)

type LambdaDetails struct {
	Function      *lambda.FunctionConfiguration
	Region        string
	Code          *lambda.GetFunctionOutput
	Configuration *lambda.FunctionConfiguration
}

var LambdaCalls = []types.AWSService{
	{
		Name: "lambda:ListFunctions",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allDetails []LambdaDetails

			originalConfig := sess.Config
			for _, region := range types.Regions {
				regionConfig := &aws.Config{
					Region:      aws.String(region),
					Credentials: originalConfig.Credentials,
				}
				regionSess, err := session.NewSession(regionConfig)
				if err != nil {
					return nil, err
				}
				svc := lambda.New(regionSess)
				output, err := svc.ListFunctionsWithContext(ctx, &lambda.ListFunctionsInput{})
				if err != nil {
					return nil, err
				}

				for _, function := range output.Functions {
					getFuncOutput, err := svc.GetFunctionWithContext(ctx, &lambda.GetFunctionInput{
						FunctionName: function.FunctionName,
					})
					if err != nil {
						return nil, err
					}

					configOutput, err := svc.GetFunctionConfigurationWithContext(ctx, &lambda.GetFunctionConfigurationInput{
						FunctionName: function.FunctionName,
					})
					if err != nil {
						return nil, err
					}

					details := LambdaDetails{
						Function:      function,
						Region:        region,
						Code:          getFuncOutput,
						Configuration: configOutput,
					}
					allDetails = append(allDetails, details)
				}
			}
			return allDetails, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "lambda:ListFunctions", err)
				return []types.ScanResult{
					{
						ServiceName: "Lambda",
						MethodName:  "lambda:ListFunctions",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if details, ok := output.([]LambdaDetails); ok {
				if len(details) == 0 {
					utils.PrintResult(debug, "", "lambda:ListFunctions", "No Lambda functions found.", nil)
				} else {
					for _, detail := range details {
						functionName := ""
						if detail.Function.FunctionName != nil {
							functionName = *detail.Function.FunctionName
						}

						codeLocation := ""
						if detail.Code.Code.Location != nil {
							codeLocation = *detail.Code.Code.Location
						}

						memorySize := ""
						timeout := ""
						runtime := ""
						handler := ""
						role := ""
						lastModified := ""

						if detail.Configuration.MemorySize != nil {
							memorySize = fmt.Sprintf("%d MB", *detail.Configuration.MemorySize)
						}
						if detail.Configuration.Timeout != nil {
							timeout = fmt.Sprintf("%d seconds", *detail.Configuration.Timeout)
						}
						if detail.Configuration.Runtime != nil {
							runtime = *detail.Configuration.Runtime
						}
						if detail.Configuration.Handler != nil {
							handler = *detail.Configuration.Handler
						}
						if detail.Configuration.Role != nil {
							role = *detail.Configuration.Role
						}
						if detail.Configuration.LastModified != nil {
							lastModified = *detail.Configuration.LastModified
						}

						// Build details map
						funcDetails := map[string]interface{}{
							"region":        detail.Region,
							"runtime":       runtime,
							"memory_size":   memorySize,
							"timeout":       timeout,
							"handler":       handler,
							"role":          role,
							"last_modified": lastModified,
							"code_location": codeLocation,
						}

						// Add description if present
						if detail.Configuration.Description != nil && *detail.Configuration.Description != "" {
							funcDetails["description"] = *detail.Configuration.Description
						}

						// Add environment variables if present
						if detail.Configuration.Environment != nil && len(detail.Configuration.Environment.Variables) > 0 {
							envVars := make(map[string]string)
							for key, value := range detail.Configuration.Environment.Variables {
								envVars[key] = aws.StringValue(value)
							}
							funcDetails["environment_variables"] = envVars
						}

						// Add KMS Key ARN if present
						if detail.Configuration.KMSKeyArn != nil && *detail.Configuration.KMSKeyArn != "" {
							funcDetails["kms_key_arn"] = *detail.Configuration.KMSKeyArn
						}

						// Add function result
						results = append(results, types.ScanResult{
							ServiceName:  "Lambda",
							MethodName:   "lambda:ListFunctions",
							ResourceType: "function",
							ResourceName: functionName,
							Details:      funcDetails,
							Timestamp:    time.Now(),
						})

						// Keep backward compatibility - print results
						utils.PrintResult(debug, "", "lambda:ListFunctions", fmt.Sprintf("Lambda function: %s", utils.ColorizeItem(functionName)), nil)
						utils.PrintResult(debug, "", "lambda:ListFunctions", fmt.Sprintf("Runtime: %s", runtime), nil)
						utils.PrintResult(debug, "", "lambda:ListFunctions", fmt.Sprintf("Memory Size: %s", memorySize), nil)
						utils.PrintResult(debug, "", "lambda:ListFunctions", fmt.Sprintf("Timeout: %s", timeout), nil)
						utils.PrintResult(debug, "", "lambda:ListFunctions", fmt.Sprintf("Handler: %s", handler), nil)
						utils.PrintResult(debug, "", "lambda:ListFunctions", fmt.Sprintf("Role: %s", role), nil)
						utils.PrintResult(debug, "", "lambda:ListFunctions", fmt.Sprintf("Last Modified: %s", lastModified), nil)

						// Print Description if it is not empty
						if detail.Configuration.Description != nil && *detail.Configuration.Description != "" {
							utils.PrintResult(debug, "", "lambda:ListFunctions", fmt.Sprintf("Description: %s", *detail.Configuration.Description), nil)
						}

						// Print environment variables if they exist
						if detail.Configuration.Environment != nil && len(detail.Configuration.Environment.Variables) > 0 {
							utils.PrintResult(debug, "", "lambda:ListFunctions", "Environment Variables:", nil)
							for key, value := range detail.Configuration.Environment.Variables {
								utils.PrintResult(debug, "", "lambda:ListFunctions", fmt.Sprintf("  %s: %s", key, aws.StringValue(value)), nil)
							}
						}

						// Print KMS Key ARN if it exists
						if detail.Configuration.KMSKeyArn != nil && *detail.Configuration.KMSKeyArn != "" {
							utils.PrintResult(debug, "", "lambda:ListFunctions", fmt.Sprintf("KMS Key ARN: %s", *detail.Configuration.KMSKeyArn), nil)
						}

						utils.PrintResult(debug, "", "lambda:ListFunctions", fmt.Sprintf("Code Location: %s", codeLocation), nil)
					}
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
