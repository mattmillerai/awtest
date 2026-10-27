package wafv2

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

const (
	serviceName = "WAFv2"
	// cloudFrontRegion is the only region that serves the CLOUDFRONT scope.
	cloudFrontRegion = "us-east-1"
)

// wafItem is a web ACL or IP set summary tagged with the scope it was listed in.
type wafItem struct {
	Name  string
	Id    string
	Arn   string
	Scope string
}

// scopedClient pairs a scope with the region its client must be built for.
type scopedClient struct {
	scope  wafv2types.Scope
	region string
}

// scopes lists REGIONAL for every scanned region plus CLOUDFRONT once, in us-east-1.
func scopes(ctx context.Context) []scopedClient {
	var out []scopedClient
	for _, region := range types.RegionsFor(ctx) {
		out = append(out, scopedClient{wafv2types.ScopeRegional, region})
	}
	return append(out, scopedClient{wafv2types.ScopeCloudfront, cloudFrontRegion})
}

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

func process(output interface{}, err error, debug bool, method, resourceType, label string) []types.ScanResult {
	var results []types.ScanResult
	if err != nil {
		return errorResults(debug, method, err)
	}
	items, ok := output.([]wafItem)
	if !ok {
		utils.HandleAWSError(debug, method, fmt.Errorf("unexpected output type %T", output))
		return results
	}
	for _, it := range items {
		results = append(results, types.ScanResult{
			ServiceName:  serviceName,
			MethodName:   method,
			ResourceType: resourceType,
			ResourceName: it.Name,
			Details: map[string]interface{}{
				"Id":    it.Id,
				"Arn":   it.Arn,
				"Scope": it.Scope,
			},
			Timestamp: time.Now(),
		})
		utils.PrintResult(debug, "", method, fmt.Sprintf("Found WAFv2 %s: %s (Scope: %s)", label, utils.ColorizeItem(it.Name), it.Scope), nil)
	}
	return results
}

var WAFv2Calls = []types.AWSService{
	{
		Name: "wafv2:ListWebACLs",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			items := []wafItem{}
			for _, sc := range scopes(ctx) {
				svc := awsv2.NewRegionalClient(cfg, sc.region, wafv2.NewFromConfig)
				input := &wafv2.ListWebACLsInput{Scope: sc.scope}
				for {
					output, err := svc.ListWebACLs(ctx, input)
					if err != nil {
						return nil, err
					}
					for _, acl := range output.WebACLs {
						items = append(items, wafItem{
							Name:  aws.ToString(acl.Name),
							Id:    aws.ToString(acl.Id),
							Arn:   aws.ToString(acl.ARN),
							Scope: string(sc.scope),
						})
					}
					if output.NextMarker == nil {
						break
					}
					input.NextMarker = output.NextMarker
				}
			}
			return items, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			return process(output, err, debug, "wafv2:ListWebACLs", "web-acl", "Web ACL")
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "wafv2:ListIPSets",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			items := []wafItem{}
			for _, sc := range scopes(ctx) {
				svc := awsv2.NewRegionalClient(cfg, sc.region, wafv2.NewFromConfig)
				input := &wafv2.ListIPSetsInput{Scope: sc.scope}
				for {
					output, err := svc.ListIPSets(ctx, input)
					if err != nil {
						return nil, err
					}
					for _, s := range output.IPSets {
						items = append(items, wafItem{
							Name:  aws.ToString(s.Name),
							Id:    aws.ToString(s.Id),
							Arn:   aws.ToString(s.ARN),
							Scope: string(sc.scope),
						})
					}
					if output.NextMarker == nil {
						break
					}
					input.NextMarker = output.NextMarker
				}
			}
			return items, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			return process(output, err, debug, "wafv2:ListIPSets", "ip-set", "IP Set")
		},
		ModuleName: types.DefaultModuleName,
	},
}
