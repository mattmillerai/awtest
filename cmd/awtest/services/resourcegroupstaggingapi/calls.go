package resourcegroupstaggingapi

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	tagtypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

const serviceName = "Resource Groups Tagging API"

// maxResources caps tag:GetResources across all regions.
const maxResources = 500

// tagKey is a tag key together with the region it was seen in.
type tagKey struct {
	Key    string
	Region string
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

var ResourceGroupsTaggingAPICalls = []types.AWSService{
	{
		Name: "tag:GetResources",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var all []tagtypes.ResourceTagMapping
		regions:
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, resourcegroupstaggingapi.NewFromConfig)
				p := resourcegroupstaggingapi.NewGetResourcesPaginator(svc, &resourcegroupstaggingapi.GetResourcesInput{})
				for p.HasMorePages() {
					page, err := p.NextPage(ctx)
					if err != nil {
						return nil, err
					}
					all = append(all, page.ResourceTagMappingList...)
					if len(all) >= maxResources {
						all = all[:maxResources]
						break regions
					}
				}
			}
			return all, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			const method = "tag:GetResources"
			var results []types.ScanResult
			if err != nil {
				return errorResults(debug, method, err)
			}
			mappings, ok := output.([]tagtypes.ResourceTagMapping)
			if !ok {
				utils.HandleAWSError(debug, method, fmt.Errorf("unexpected output type %T", output))
				return results
			}
			for _, m := range mappings {
				arn := aws.ToString(m.ResourceARN)
				results = append(results, types.ScanResult{
					ServiceName:  serviceName,
					MethodName:   method,
					ResourceType: "tagged-resource",
					ResourceName: arn,
					Details: map[string]interface{}{
						"TagCount": len(m.Tags),
					},
					Timestamp: time.Now(),
				})
				utils.PrintResult(debug, "", method, fmt.Sprintf("Found tagged resource: %s (Tags: %d)", utils.ColorizeItem(arn), len(m.Tags)), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "tag:GetTagKeys",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var all []tagKey
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, resourcegroupstaggingapi.NewFromConfig)
				p := resourcegroupstaggingapi.NewGetTagKeysPaginator(svc, &resourcegroupstaggingapi.GetTagKeysInput{})
				for p.HasMorePages() {
					page, err := p.NextPage(ctx)
					if err != nil {
						return nil, err
					}
					for _, k := range page.TagKeys {
						all = append(all, tagKey{Key: k, Region: region})
					}
				}
			}
			return all, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			const method = "tag:GetTagKeys"
			var results []types.ScanResult
			if err != nil {
				return errorResults(debug, method, err)
			}
			keys, ok := output.([]tagKey)
			if !ok {
				utils.HandleAWSError(debug, method, fmt.Errorf("unexpected output type %T", output))
				return results
			}
			for _, k := range keys {
				results = append(results, types.ScanResult{
					ServiceName:  serviceName,
					MethodName:   method,
					ResourceType: "tag-key",
					ResourceName: k.Key,
					Details: map[string]interface{}{
						"Region": k.Region,
					},
					Timestamp: time.Now(),
				})
				utils.PrintResult(debug, "", method, fmt.Sprintf("Found tag key: %s (Region: %s)", utils.ColorizeItem(k.Key), k.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
