package apigatewayv2

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

const serviceName = "API Gateway V2"

func errorResults(debug bool, method string, err error) []types.ScanResult {
	utils.HandleAWSError(debug, method, err)
	return []types.ScanResult{
		{
			ServiceName: serviceName,
			MethodName:  method,
			Error:       err,
			Timestamp:   time.Now(),
		},
	}
}

var APIGatewayV2Calls = []types.AWSService{
	{
		Name: "apigatewayv2:GetApis",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var all []apigwtypes.Api
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, apigatewayv2.NewFromConfig)
				input := &apigatewayv2.GetApisInput{}
				for {
					output, err := svc.GetApis(ctx, input)
					if err != nil {
						return nil, err
					}
					all = append(all, output.Items...)
					if output.NextToken == nil {
						break
					}
					input.NextToken = output.NextToken
				}
			}
			return all, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			const method = "apigatewayv2:GetApis"
			var results []types.ScanResult
			if err != nil {
				return errorResults(debug, method, err)
			}
			apis, ok := output.([]apigwtypes.Api)
			if !ok {
				utils.HandleAWSError(debug, method, fmt.Errorf("unexpected output type %T", output))
				return results
			}
			for _, api := range apis {
				name := aws.ToString(api.Name)
				results = append(results, types.ScanResult{
					ServiceName:  serviceName,
					MethodName:   method,
					ResourceType: "api",
					ResourceName: name,
					Details: map[string]interface{}{
						"ApiId":        aws.ToString(api.ApiId),
						"ProtocolType": string(api.ProtocolType),
					},
					Timestamp: time.Now(),
				})
				utils.PrintResult(debug, "", method, fmt.Sprintf("Found API Gateway V2 API: %s (Protocol: %s)", utils.ColorizeItem(name), string(api.ProtocolType)), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "apigatewayv2:GetDomainNames",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var all []apigwtypes.DomainName
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, apigatewayv2.NewFromConfig)
				input := &apigatewayv2.GetDomainNamesInput{}
				for {
					output, err := svc.GetDomainNames(ctx, input)
					if err != nil {
						return nil, err
					}
					all = append(all, output.Items...)
					if output.NextToken == nil {
						break
					}
					input.NextToken = output.NextToken
				}
			}
			return all, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			const method = "apigatewayv2:GetDomainNames"
			var results []types.ScanResult
			if err != nil {
				return errorResults(debug, method, err)
			}
			domains, ok := output.([]apigwtypes.DomainName)
			if !ok {
				utils.HandleAWSError(debug, method, fmt.Errorf("unexpected output type %T", output))
				return results
			}
			for _, d := range domains {
				name := aws.ToString(d.DomainName)
				results = append(results, types.ScanResult{
					ServiceName:  serviceName,
					MethodName:   method,
					ResourceType: "domain-name",
					ResourceName: name,
					Details:      map[string]interface{}{},
					Timestamp:    time.Now(),
				})
				utils.PrintResult(debug, "", method, fmt.Sprintf("Found API Gateway V2 Domain: %s", utils.ColorizeItem(name)), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
