package waf

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/waf"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var WafCalls = []types.AWSService{
	{
		Name: "waf:ListWebACLs",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", waf.NewFromConfig)
			input := &waf.ListWebACLsInput{}
			return svc.ListWebACLs(ctx, input)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "waf:ListWebACLs", err)
				return []types.ScanResult{
					{
						ServiceName: "WAF",
						MethodName:  "waf:ListWebACLs",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if webAcls, ok := output.(*waf.ListWebACLsOutput); ok {
				for _, webAcl := range webAcls.WebACLs {
					name := aws.ToString(webAcl.Name)
					results = append(results, types.ScanResult{
						ServiceName:  "WAF",
						MethodName:   "waf:ListWebACLs",
						ResourceType: "web-acl",
						ResourceName: name,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "waf:ListWebACLs", fmt.Sprintf("WebACL: %s", name), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
