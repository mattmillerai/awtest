package certificatemanager

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/acm"
	"time"
)

var CertificateManagerCalls = []types.AWSService{
	{
		Name: "acm:ListCertificates",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allCertificates []*acm.CertificateSummary
			for _, region := range types.Regions {
				sess.Config.Region = aws.String(region)
				svc := acm.New(sess)
				output, err := svc.ListCertificatesWithContext(ctx, &acm.ListCertificatesInput{})
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

			if certs, ok := output.([]*acm.CertificateSummary); ok {
				for _, cert := range certs {
					domainName := ""
					if cert.DomainName != nil {
						domainName = *cert.DomainName
					}

					certArn := ""
					if cert.CertificateArn != nil {
						certArn = *cert.CertificateArn
					}

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
