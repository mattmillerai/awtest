package route53

import (
	"context"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"time"
)

var Route53Calls = []types.AWSService{
	{
		Name: "route53:ListHostedZones",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := route53.New(sess)
			output, err := svc.ListHostedZonesWithContext(ctx, &route53.ListHostedZonesInput{})

			if err != nil {
				return nil, err
			} else {
				return output, nil
			}
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "route53:ListHostedZones", err)
				return []types.ScanResult{
					{
						ServiceName: "Route53",
						MethodName:  "route53:ListHostedZones",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if zones, ok := output.(*route53.ListHostedZonesOutput); ok {
				for _, zone := range zones.HostedZones {
					zoneName := ""
					if zone.Name != nil {
						zoneName = *zone.Name
					}

					results = append(results, types.ScanResult{
						ServiceName:  "Route53",
						MethodName:   "route53:ListHostedZones",
						ResourceType: "hosted-zone",
						ResourceName: zoneName,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "route53:ListHostedZones", zoneName, nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "route53:ListHealthChecks",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := route53.New(sess)
			output, err := svc.ListHealthChecksWithContext(ctx, &route53.ListHealthChecksInput{})

			if err != nil {
				return nil, err
			} else {
				return output, nil
			}
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				return []types.ScanResult{
					{
						ServiceName: "Route53",
						MethodName:  "route53:ListHealthChecks",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if checks, ok := output.(*route53.ListHealthChecksOutput); ok {
				for _, check := range checks.HealthChecks {
					ipAddr := ""
					if check.HealthCheckConfig != nil && check.HealthCheckConfig.IPAddress != nil {
						ipAddr = *check.HealthCheckConfig.IPAddress
					}

					results = append(results, types.ScanResult{
						ServiceName:  "Route53",
						MethodName:   "route53:ListHealthChecks",
						ResourceType: "health-check",
						ResourceName: ipAddr,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "route53:ListHealthChecks", ipAddr, nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
