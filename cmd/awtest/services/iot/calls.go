package iot

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iot"
	"time"
)

var IoTCalls = []types.AWSService{
	{
		Name: "iot:ListThings",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := iot.New(sess)
			input := &iot.ListThingsInput{}
			return svc.ListThingsWithContext(ctx, input)
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
					utils.PrintResult(debug, "", "iot:ListThings", fmt.Sprintf("Found Thing: %s", *thing.ThingName), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "IoT",
						MethodName:   "iot:ListThings",
						ResourceType: "thing",
						ResourceName: *thing.ThingName,
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
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := iot.New(sess)
			input := &iot.ListPoliciesInput{}
			return svc.ListPoliciesWithContext(ctx, input)
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
					utils.PrintResult(debug, "", "iot:ListPolicies", fmt.Sprintf("Found Policy: %s", *policy.PolicyName), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "IoT",
						MethodName:   "iot:ListPolicies",
						ResourceType: "policy",
						ResourceName: *policy.PolicyName,
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
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := iot.New(sess)
			input := &iot.ListCertificatesInput{}
			return svc.ListCertificatesWithContext(ctx, input)
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
					utils.PrintResult(debug, "", "iot:ListCertificates", fmt.Sprintf("Found Certificate: %s", *cert.CertificateArn), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "IoT",
						MethodName:   "iot:ListCertificates",
						ResourceType: "certificate",
						ResourceName: *cert.CertificateArn,
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
