package apigateway

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	agtypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

type ApiWithStages struct {
	Api                 agtypes.RestApi
	Stages              []agtypes.Stage
	Models              []agtypes.Model
	Resources           []agtypes.Resource // Add this line
	Region              string
	MethodParams        map[string]map[string]map[string]bool
	MethodIntegrations  map[string]map[string]*apigateway.GetIntegrationOutput
	AuthorizationScopes map[string]map[string][]string
}

var APIGatewayCalls = []types.AWSService{
	{
		Name: "apigateway:RestApis",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allApisWithStages []ApiWithStages

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, apigateway.NewFromConfig)
				apisOutput, err := svc.GetRestApis(ctx, &apigateway.GetRestApisInput{})
				if err != nil {
					return nil, err
				}
				for _, api := range apisOutput.Items {
					stagesOutput, err := svc.GetStages(ctx, &apigateway.GetStagesInput{RestApiId: api.Id})
					if err != nil {
						return nil, err
					}
					modelsOutput, err := svc.GetModels(ctx, &apigateway.GetModelsInput{RestApiId: api.Id})
					if err != nil {
						return nil, err
					}
					resourcesOutput, err := svc.GetResources(ctx, &apigateway.GetResourcesInput{RestApiId: api.Id})
					if err != nil {
						return nil, err
					}

					methodParams := make(map[string]map[string]map[string]bool)
					methodIntegrations := make(map[string]map[string]*apigateway.GetIntegrationOutput)
					for _, resource := range resourcesOutput.Items {
						if resource.ResourceMethods != nil {
							resourceID := aws.ToString(resource.Id)
							methodParams[resourceID] = make(map[string]map[string]bool)
							methodIntegrations[resourceID] = make(map[string]*apigateway.GetIntegrationOutput)

							for method := range resource.ResourceMethods {
								input := &apigateway.GetMethodInput{
									ResourceId: aws.String(resourceID),
									RestApiId:  api.Id,
									HttpMethod: aws.String(method),
								}

								methodOutput, err := svc.GetMethod(ctx, input)
								if err != nil {
									return nil, err
								}

								params := make(map[string]bool)
								for paramName, paramInfo := range methodOutput.RequestParameters {
									params[paramName] = paramInfo
								}

								methodParams[resourceID][method] = params

								// Get Integration
								integrationInput := &apigateway.GetIntegrationInput{
									ResourceId: aws.String(resourceID),
									RestApiId:  api.Id,
									HttpMethod: aws.String(method),
								}
								integrationOutput, err := svc.GetIntegration(ctx, integrationInput)

								if err != nil {
									// handle the error or just continue if integration is not a necessity
									continue
								}

								methodIntegrations[resourceID][method] = integrationOutput
							}
						}
					}

					apiWithStages := ApiWithStages{
						Api:          api,
						Stages:       stagesOutput.Item,
						Models:       modelsOutput.Items,
						Resources:    resourcesOutput.Items, // Add this line
						Region:       region,
						MethodParams: methodParams,
					}
					allApisWithStages = append(allApisWithStages, apiWithStages)
				}
			}
			return allApisWithStages, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "apigateway:RestApis", err)
				return []types.ScanResult{
					{
						ServiceName: "APIGateway",
						MethodName:  "apigateway:RestApis",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if apisWithStages, ok := output.([]ApiWithStages); ok {
				for _, apiWithStages := range apisWithStages {
					fmt.Println()
					apiName := aws.ToString(apiWithStages.Api.Name)
					restApiId := aws.ToString(apiWithStages.Api.Id)
					region := apiWithStages.Region
					apiUrl := fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com", restApiId, region)
					utils.PrintResult(debug, "", "apigateway:RestApis", fmt.Sprintf("Found API Gateway: %s", apiName), nil)
					utils.PrintResult(debug, "", "apigateway:RestApis", fmt.Sprintf("Base URL: %s", apiUrl), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "APIGateway",
						MethodName:   "apigateway:RestApis",
						ResourceType: "rest-api",
						ResourceName: apiName,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					// Add this loop to print resources
					if len(apiWithStages.Resources) > 0 {
						for _, resource := range apiWithStages.Resources {
							resourceID := aws.ToString(resource.Id)
							resourcePath := aws.ToString(resource.Path)

							//utils.PrintResult(debug, "", "apigateway:GetResources", fmt.Sprintf("Resource ID: %s", resourceID), nil)
							utils.PrintResult(debug, "", "apigateway:GetResources", fmt.Sprintf("Resource Path: %s", resourcePath), nil)

							results = append(results, types.ScanResult{
								ServiceName:  "APIGateway",
								MethodName:   "apigateway:GetResources",
								ResourceType: "resource",
								ResourceName: resourcePath,
								Details:      map[string]interface{}{},
								Timestamp:    time.Now(),
							})

							// Check if the ResourceMethods map is not nil
							for method, params := range apiWithStages.MethodParams[resourceID] {
								utils.PrintResult(debug, "", "apigateway:GetResources", fmt.Sprintf("Resource Method: %s", method), nil)

								for paramName, required := range params {
									utils.PrintResult(debug, "", "apigateway:GetResources", fmt.Sprintf("Request Parameter: %s, Required: %v", paramName, required), nil)
								}
							}
						}
					}

					if len(apiWithStages.MethodIntegrations) > 0 {
						for resourceID, methods := range apiWithStages.MethodIntegrations {
							for method, integration := range methods {
								utils.PrintResult(debug, "", "apigateway:GetMethodIntegration", fmt.Sprintf("Integration Type for Method %s of Resource %s: %s", method, resourceID, string(integration.Type)), nil)
							}
						}
					}

					if len(apiWithStages.Stages) == 0 {
						utils.PrintResult(debug, "", "apigateway:GetStages", fmt.Sprintf("No stages found for API: %s, but access is granted.", apiName), nil)
					} else {
						for _, stage := range apiWithStages.Stages {
							stageName := aws.ToString(stage.StageName)
							utils.PrintResult(debug, "", "apigateway:GetStages", fmt.Sprintf("Found Stage: %s (%s)", stageName, apiName), nil)

							results = append(results, types.ScanResult{
								ServiceName:  "APIGateway",
								MethodName:   "apigateway:GetStages",
								ResourceType: "stage",
								ResourceName: stageName,
								Details:      map[string]interface{}{},
								Timestamp:    time.Now(),
							})
						}
					}
					if len(apiWithStages.Models) > 0 {
						for _, model := range apiWithStages.Models {
							modelName := aws.ToString(model.Name)
							utils.PrintResult(debug, "", "apigateway:GetModels", fmt.Sprintf("Found Model: %s (%s)", modelName, apiName), nil)

							results = append(results, types.ScanResult{
								ServiceName:  "APIGateway",
								MethodName:   "apigateway:GetModels",
								ResourceType: "model",
								ResourceName: modelName,
								Details:      map[string]interface{}{},
								Timestamp:    time.Now(),
							})
						}
					}
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "apigateway:GetApiKeys",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allApiKeys []agtypes.ApiKey
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, apigateway.NewFromConfig)
				output, err := svc.GetApiKeys(ctx, &apigateway.GetApiKeysInput{})
				if err != nil {
					return nil, err
				}
				allApiKeys = append(allApiKeys, output.Items...)
			}
			return allApiKeys, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "apigateway:GetApiKeys", err)
				return []types.ScanResult{
					{
						ServiceName: "APIGateway",
						MethodName:  "apigateway:GetApiKeys",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if apiKeys, ok := output.([]agtypes.ApiKey); ok {
				if len(apiKeys) == 0 {
					utils.PrintAccessGranted(debug, "apigateway:GetApiKeys", "API keys")
				} else {
					for _, apiKey := range apiKeys {
						keyID := aws.ToString(apiKey.Id)
						utils.PrintResult(debug, "", "apigateway:GetApiKeys", fmt.Sprintf("Found API Key: %s", keyID), nil)

						results = append(results, types.ScanResult{
							ServiceName:  "APIGateway",
							MethodName:   "apigateway:GetApiKeys",
							ResourceType: "api-key",
							ResourceName: keyID,
							Details:      map[string]interface{}{},
							Timestamp:    time.Now(),
						})
					}
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "apigateway:GetDomainNames",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allDomainNames []agtypes.DomainName
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, apigateway.NewFromConfig)
				output, err := svc.GetDomainNames(ctx, &apigateway.GetDomainNamesInput{})
				if err != nil {
					return nil, err
				}
				allDomainNames = append(allDomainNames, output.Items...)
			}
			return allDomainNames, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "apigateway:GetDomainNames", err)
				return []types.ScanResult{
					{
						ServiceName: "APIGateway",
						MethodName:  "apigateway:GetDomainNames",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if domainNames, ok := output.([]agtypes.DomainName); ok {
				if len(domainNames) == 0 {
					utils.PrintAccessGranted(debug, "apigateway:GetDomainNames", "domain names")
				} else {
					for _, domainName := range domainNames {
						name := aws.ToString(domainName.DomainName)
						utils.PrintResult(debug, "", "apigateway:GetDomainNames", fmt.Sprintf("Found Domain Name: %s", name), nil)

						results = append(results, types.ScanResult{
							ServiceName:  "APIGateway",
							MethodName:   "apigateway:GetDomainNames",
							ResourceType: "domain-name",
							ResourceName: name,
							Details:      map[string]interface{}{},
							Timestamp:    time.Now(),
						})
					}
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
