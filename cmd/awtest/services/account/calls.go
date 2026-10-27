package account

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/account"
	accounttypes "github.com/aws/aws-sdk-go-v2/service/account/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

const serviceName = "Account"

// globalRegion is the region the Account endpoint is queried in; like
// Organizations, the calls are not looped over regions.
const globalRegion = "us-east-1"

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

var AccountCalls = []types.AWSService{
	{
		Name: "account:ListRegions",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, globalRegion, account.NewFromConfig)

			var all []accounttypes.Region
			p := account.NewListRegionsPaginator(svc, &account.ListRegionsInput{})
			for p.HasMorePages() {
				page, err := p.NextPage(ctx)
				if err != nil {
					return nil, err
				}
				all = append(all, page.Regions...)
			}
			return all, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			const method = "account:ListRegions"
			var results []types.ScanResult
			if err != nil {
				return errorResults(debug, method, err)
			}
			regions, ok := output.([]accounttypes.Region)
			if !ok {
				utils.HandleAWSError(debug, method, fmt.Errorf("unexpected output type %T", output))
				return results
			}
			for _, r := range regions {
				name := aws.ToString(r.RegionName)
				status := string(r.RegionOptStatus)
				results = append(results, types.ScanResult{
					ServiceName:  serviceName,
					MethodName:   method,
					ResourceType: "region",
					ResourceName: name,
					Details: map[string]interface{}{
						"RegionOptStatus": status,
					},
					Timestamp: time.Now(),
				})
				utils.PrintResult(debug, "", method, fmt.Sprintf("Account Region: %s (Status: %s)", utils.ColorizeItem(name), status), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "account:GetContactInformation",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, globalRegion, account.NewFromConfig)

			output, err := svc.GetContactInformation(ctx, &account.GetContactInformationInput{})
			if err != nil {
				return nil, err
			}
			if output.ContactInformation == nil {
				return accounttypes.ContactInformation{}, nil
			}
			return *output.ContactInformation, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			const method = "account:GetContactInformation"
			var results []types.ScanResult
			// Access denied is reported as a ScanResult error so the summary counts it.
			if err != nil {
				return errorResults(debug, method, err)
			}
			info, ok := output.(accounttypes.ContactInformation)
			if !ok {
				utils.HandleAWSError(debug, method, fmt.Errorf("unexpected output type %T", output))
				return results
			}
			name := aws.ToString(info.FullName)
			results = append(results, types.ScanResult{
				ServiceName:  serviceName,
				MethodName:   method,
				ResourceType: "contact-information",
				ResourceName: name,
				Details: map[string]interface{}{
					"CompanyName": aws.ToString(info.CompanyName),
					"CountryCode": aws.ToString(info.CountryCode),
				},
				Timestamp: time.Now(),
			})
			utils.PrintResult(debug, "", method, fmt.Sprintf("Account Contact: %s", utils.ColorizeItem(name)), nil)
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
