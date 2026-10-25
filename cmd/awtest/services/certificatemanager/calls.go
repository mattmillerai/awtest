package certificatemanager

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var CertificateManagerCalls = []types.AWSService{
	{
		Name: "acm:ListCertificates",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allCertificates []acmtypes.CertificateSummary
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, acm.NewFromConfig)
				output, err := svc.ListCertificates(ctx, &acm.ListCertificatesInput{})
				if err != nil {
					return nil, err
				}
				allCertificates = append(allCertificates, output.CertificateSummaryList...)
			}
			return allCertificates, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "acm:ListCertificates", err)
				return []types.ScanResult{
					{
						ServiceName: "CertificateManager",
						MethodName:  "acm:ListCertificates",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if certs, ok := output.([]acmtypes.CertificateSummary); ok {
				for _, cert := range certs {
					domainName := aws.ToString(cert.DomainName)
					certArn := aws.ToString(cert.CertificateArn)

					results = append(results, types.ScanResult{
						ServiceName:  "CertificateManager",
						MethodName:   "acm:ListCertificates",
						ResourceType: "certificate",
						ResourceName: domainName,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "acm:ListCertificates",
						fmt.Sprintf("Certificate: %s (ARN: %s)", utils.ColorizeItem(domainName), certArn), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
