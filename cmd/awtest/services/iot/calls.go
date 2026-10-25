package iot

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iot"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var IoTCalls = []types.AWSService{
	{
		Name: "iot:ListThings",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", iot.NewFromConfig)
			input := &iot.ListThingsInput{}
			return svc.ListThings(ctx, input)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "iot:ListThings", err)
				return []types.ScanResult{
					{
						ServiceName: "IoT",
						MethodName:  "iot:ListThings",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if things, ok := output.(*iot.ListThingsOutput); ok {
				for _, thing := range things.Things {
					thingName := aws.ToString(thing.ThingName)
					utils.PrintResult(debug, "", "iot:ListThings", fmt.Sprintf("Found Thing: %s", thingName), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "IoT",
						MethodName:   "iot:ListThings",
						ResourceType: "thing",
						ResourceName: thingName,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "iot:ListPolicies",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", iot.NewFromConfig)
			input := &iot.ListPoliciesInput{}
			return svc.ListPolicies(ctx, input)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "iot:ListPolicies", err)
				return []types.ScanResult{
					{
						ServiceName: "IoT",
						MethodName:  "iot:ListPolicies",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if policies, ok := output.(*iot.ListPoliciesOutput); ok {
				for _, policy := range policies.Policies {
					policyName := aws.ToString(policy.PolicyName)
					utils.PrintResult(debug, "", "iot:ListPolicies", fmt.Sprintf("Found Policy: %s", policyName), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "IoT",
						MethodName:   "iot:ListPolicies",
						ResourceType: "policy",
						ResourceName: policyName,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "iot:ListCertificates",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", iot.NewFromConfig)
			input := &iot.ListCertificatesInput{}
			return svc.ListCertificates(ctx, input)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "iot:ListCertificates", err)
				return []types.ScanResult{
					{
						ServiceName: "IoT",
						MethodName:  "iot:ListCertificates",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if certificates, ok := output.(*iot.ListCertificatesOutput); ok {
				for _, cert := range certificates.Certificates {
					certArn := aws.ToString(cert.CertificateArn)
					utils.PrintResult(debug, "", "iot:ListCertificates", fmt.Sprintf("Found Certificate: %s", certArn), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "IoT",
						MethodName:   "iot:ListCertificates",
						ResourceType: "certificate",
						ResourceName: certArn,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
