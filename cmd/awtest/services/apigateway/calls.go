package apigateway

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/apigateway"
	"time"
)

type ApiWithStages struct {
	Api                 *apigateway.RestApi
	Stages              []*apigateway.Stage
	Models              []*apigateway.Model
	Resources           []*apigateway.Resource // Add this line
	Region              string
	MethodParams        map[string]map[string]map[string]bool
	MethodIntegrations  map[string]map[string]*apigateway.Integration
	AuthorizationScopes map[string]map[string][]string
}

var APIGatewayCalls = []types.AWSService{
	{
		Name: "apigateway:RestApis",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allApisWithStages []ApiWithStages

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
				svc := apigateway.New(regionSess)
				apisOutput, err := svc.GetRestApisWithContext(ctx, &apigateway.GetRestApisInput{})
				if err != nil {
					return nil, err
				}
				for _, api := range apisOutput.Items {
					stagesOutput, err := svc.GetStagesWithContext(ctx, &apigateway.GetStagesInput{RestApiId: api.Id})
					if err != nil {
						return nil, err
					}
					modelsOutput, err := svc.GetModelsWithContext(ctx, &apigateway.GetModelsInput{RestApiId: api.Id})
					if err != nil {
						return nil, err
					}
					resourcesOutput, err := svc.GetResourcesWithContext(ctx, &apigateway.GetResourcesInput{RestApiId: api.Id})
					if err != nil {
						return nil, err
					}

					methodParams := make(map[string]map[string]map[string]bool)
					methodIntegrations := make(map[string]map[string]*apigateway.Integration)
					for _, resource := range resourcesOutput.Items {
						if resource.ResourceMethods != nil {
							resourceID := *resource.Id
							methodParams[resourceID] = make(map[string]map[string]bool)
							methodIntegrations[resourceID] = make(map[string]*apigateway.Integration)

							for method, _ := range resource.ResourceMethods {
								input := &apigateway.GetMethodInput{
									ResourceId: aws.String(resourceID),
									RestApiId:  api.Id,
									HttpMethod: aws.String(method),
								}

								methodOutput, err := svc.GetMethodWithContext(ctx, input)
								if err != nil {
									return nil, err
								}

								params := make(map[string]bool)
								for paramName, paramInfo := range methodOutput.RequestParameters {
									params[paramName] = aws.BoolValue(paramInfo)
								}

								methodParams[resourceID][method] = params

								// Get Integration
								integrationInput := &apigateway.GetIntegrationInput{
									ResourceId: aws.String(resourceID),
									RestApiId:  api.Id,
									HttpMethod: aws.String(method),
								}
								integrationOutput, err := svc.GetIntegrationWithContext(ctx, integrationInput)

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
					apiName := *apiWithStages.Api.Name
					restApiId := *apiWithStages.Api.Id
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
							resourceID := *resource.Id
							resourcePath := *resource.Path

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
								utils.PrintResult(debug, "", "apigateway:GetMethodIntegration", fmt.Sprintf("Integration Type for Method %s of Resource %s: %s", method, resourceID, *integration.Type), nil)
							}
						}
					}

					if len(apiWithStages.Stages) == 0 {
						utils.PrintResult(debug, "", "apigateway:GetStages", fmt.Sprintf("No stages found for API: %s, but access is granted.", apiName), nil)
					} else {
						for _, stage := range apiWithStages.Stages {
							utils.PrintResult(debug, "", "apigateway:GetStages", fmt.Sprintf("Found Stage: %s (%s)", *stage.StageName, apiName), nil)

							results = append(results, types.ScanResult{
								ServiceName:  "APIGateway",
								MethodName:   "apigateway:GetStages",
								ResourceType: "stage",
								ResourceName: *stage.StageName,
								Details:      map[string]interface{}{},
								Timestamp:    time.Now(),
							})
						}
					}
					if len(apiWithStages.Models) > 0 {
						for _, model := range apiWithStages.Models {
							utils.PrintResult(debug, "", "apigateway:GetModels", fmt.Sprintf("Found Model: %s (%s)", *model.Name, apiName), nil)

							results = append(results, types.ScanResult{
								ServiceName:  "APIGateway",
								MethodName:   "apigateway:GetModels",
								ResourceType: "model",
								ResourceName: *model.Name,
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
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allApiKeys []*apigateway.ApiKey
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
				svc := apigateway.New(regionSess)
				output, err := svc.GetApiKeysWithContext(ctx, &apigateway.GetApiKeysInput{})
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
			if apiKeys, ok := output.([]*apigateway.ApiKey); ok {
				if len(apiKeys) == 0 {
					utils.PrintAccessGranted(debug, "apigateway:GetApiKeys", "API keys")
				} else {
					for _, apiKey := range apiKeys {
						utils.PrintResult(debug, "", "apigateway:GetApiKeys", fmt.Sprintf("Found API Key: %s", *apiKey.Id), nil)

						results = append(results, types.ScanResult{
							ServiceName:  "APIGateway",
							MethodName:   "apigateway:GetApiKeys",
							ResourceType: "api-key",
							ResourceName: *apiKey.Id,
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
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allDomainNames []*apigateway.DomainName
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
				svc := apigateway.New(regionSess)
				output, err := svc.GetDomainNamesWithContext(ctx, &apigateway.GetDomainNamesInput{})
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
			if domainNames, ok := output.([]*apigateway.DomainName); ok {
				if len(domainNames) == 0 {
					utils.PrintAccessGranted(debug, "apigateway:GetDomainNames", "domain names")
				} else {
					for _, domainName := range domainNames {
						utils.PrintResult(debug, "", "apigateway:GetDomainNames", fmt.Sprintf("Found Domain Name: %s", *domainName.DomainName), nil)

						results = append(results, types.ScanResult{
							ServiceName:  "APIGateway",
							MethodName:   "apigateway:GetDomainNames",
							ResourceType: "domain-name",
							ResourceName: *domainName.DomainName,
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
