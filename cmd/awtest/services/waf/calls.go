package waf

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/waf"
	"time"
)

var WafCalls = []types.AWSService{
	{
		Name: "waf:ListWebACLs",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := waf.New(sess)
			input := &waf.ListWebACLsInput{}
			return svc.ListWebACLsWithContext(ctx, input)
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
					results = append(results, types.ScanResult{
						ServiceName:  "WAF",
						MethodName:   "waf:ListWebACLs",
						ResourceType: "web-acl",
						ResourceName: *webAcl.Name,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "waf:ListWebACLs", fmt.Sprintf("WebACL: %s", *webAcl.Name), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
