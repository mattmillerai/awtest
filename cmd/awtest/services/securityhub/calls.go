package securityhub

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/securityhub"
)

type shStandard struct {
	StandardsSubscriptionArn string
	StandardsArn             string
	StandardsStatus          string
	Region                   string
}

type shFinding struct {
	Id               string
	Title            string
	SeverityLabel    string
	ComplianceStatus string
	ProductName      string
	ResourceType     string
	Region           string
	GeneratorId      string
}

type shProduct struct {
	ProductSubscriptionArn string
	Region                 string
}

// extractStandardName extracts a human-readable name from a standards ARN.
// e.g., "arn:aws:securityhub:::standards/aws-foundational-security-best-practices/v/1.0.0" -> "aws-foundational-security-best-practices"
// e.g., "arn:aws:securityhub:::ruleset/cis-aws-foundations-benchmark/v/1.2.0" -> "cis-aws-foundations-benchmark"
func extractStandardName(arn string) string {
	// ARN path format: ...standards/<name>/v/<version> or ...ruleset/<name>/v/<version>
	parts := strings.Split(arn, "/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return arn
}

var SecurityHubCalls = []types.AWSService{
	{
		Name: "securityhub:GetEnabledStandards",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allStandards []shStandard
			var lastErr error

			for _, region := range types.Regions {
				svc := securityhub.New(sess, &aws.Config{Region: aws.String(region)})

				input := &securityhub.GetEnabledStandardsInput{}
				for {
					output, err := svc.GetEnabledStandardsWithContext(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "securityhub:GetEnabledStandards", err)
						break
					}

					for _, sub := range output.StandardsSubscriptions {
						subscriptionArn := ""
						if sub.StandardsSubscriptionArn != nil {
							subscriptionArn = *sub.StandardsSubscriptionArn
						}

						standardsArn := ""
						if sub.StandardsArn != nil {
							standardsArn = *sub.StandardsArn
						}

						standardsStatus := ""
						if sub.StandardsStatus != nil {
							standardsStatus = *sub.StandardsStatus
						}

						allStandards = append(allStandards, shStandard{
							StandardsSubscriptionArn: subscriptionArn,
							StandardsArn:             standardsArn,
							StandardsStatus:          standardsStatus,
							Region:                   region,
						})
					}

					if output.NextToken == nil {
						break
					}
					input.NextToken = output.NextToken
				}
			}

			if len(allStandards) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allStandards, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "securityhub:GetEnabledStandards", err)
				return []types.ScanResult{
					{
						ServiceName: "SecurityHub",
						MethodName:  "securityhub:GetEnabledStandards",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			standards, ok := output.([]shStandard)
			if !ok {
				utils.HandleAWSError(debug, "securityhub:GetEnabledStandards", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, standard := range standards {
				standardName := extractStandardName(standard.StandardsArn)

				results = append(results, types.ScanResult{
					ServiceName:  "SecurityHub",
					MethodName:   "securityhub:GetEnabledStandards",
					ResourceType: "standard",
					ResourceName: standardName,
					Details: map[string]interface{}{
						"StandardsSubscriptionArn": standard.StandardsSubscriptionArn,
						"StandardsArn":             standard.StandardsArn,
						"StandardsStatus":          standard.StandardsStatus,
						"Region":                   standard.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "securityhub:GetEnabledStandards",
					fmt.Sprintf("Security Hub Standard: %s (Status: %s, Region: %s)", utils.ColorizeItem(standardName), standard.StandardsStatus, standard.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "securityhub:GetFindings",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allFindings []shFinding
			var lastErr error

			for _, region := range types.Regions {
				svc := securityhub.New(sess, &aws.Config{Region: aws.String(region)})

				input := &securityhub.GetFindingsInput{
					Filters: &securityhub.AwsSecurityFindingFilters{
						RecordState: []*securityhub.StringFilter{
							{
								Comparison: aws.String("EQUALS"),
								Value:      aws.String("ACTIVE"),
							},
						},
					},
					MaxResults: aws.Int64(100),
					SortCriteria: []*securityhub.SortCriterion{
						{
							Field:     aws.String("SeverityLabel"),
							SortOrder: aws.String("desc"),
						},
					},
				}

				output, err := svc.GetFindingsWithContext(ctx, input)
				if err != nil {
					lastErr = err
					utils.HandleAWSError(false, "securityhub:GetFindings", err)
					continue
				}

				for _, finding := range output.Findings {
					id := ""
					if finding.Id != nil {
						id = *finding.Id
					}

					title := ""
					if finding.Title != nil {
						title = *finding.Title
					}

					severityLabel := ""
					if finding.Severity != nil && finding.Severity.Label != nil {
						severityLabel = *finding.Severity.Label
					}

					complianceStatus := ""
					if finding.Compliance != nil && finding.Compliance.Status != nil {
						complianceStatus = *finding.Compliance.Status
					}

					productName := ""
					if finding.ProductName != nil {
						productName = *finding.ProductName
					}

					resourceType := ""
					if len(finding.Resources) > 0 && finding.Resources[0].Type != nil {
						resourceType = *finding.Resources[0].Type
					}

					generatorId := ""
					if finding.GeneratorId != nil {
						generatorId = *finding.GeneratorId
					}

					allFindings = append(allFindings, shFinding{
						Id:               id,
						Title:            title,
						SeverityLabel:    severityLabel,
						ComplianceStatus: complianceStatus,
						ProductName:      productName,
						ResourceType:     resourceType,
						Region:           region,
						GeneratorId:      generatorId,
					})
				}
			}

			if len(allFindings) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allFindings, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "securityhub:GetFindings", err)
				return []types.ScanResult{
					{
						ServiceName: "SecurityHub",
						MethodName:  "securityhub:GetFindings",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			findings, ok := output.([]shFinding)
			if !ok {
				utils.HandleAWSError(debug, "securityhub:GetFindings", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, finding := range findings {
				resourceName := finding.Title
				if resourceName == "" {
					resourceName = finding.GeneratorId
				}

				results = append(results, types.ScanResult{
					ServiceName:  "SecurityHub",
					MethodName:   "securityhub:GetFindings",
					ResourceType: "finding",
					ResourceName: resourceName,
					Details: map[string]interface{}{
						"Id":               finding.Id,
						"Title":            finding.Title,
						"SeverityLabel":    finding.SeverityLabel,
						"ComplianceStatus": finding.ComplianceStatus,
						"ProductName":      finding.ProductName,
						"ResourceType":     finding.ResourceType,
						"Region":           finding.Region,
						"GeneratorId":      finding.GeneratorId,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "securityhub:GetFindings",
					fmt.Sprintf("Security Hub Finding: %s (Severity: %s, Compliance: %s)", utils.ColorizeItem(resourceName), finding.SeverityLabel, finding.ComplianceStatus), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "securityhub:ListEnabledProductsForImport",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allProducts []shProduct
			var lastErr error

			for _, region := range types.Regions {
				svc := securityhub.New(sess, &aws.Config{Region: aws.String(region)})

				input := &securityhub.ListEnabledProductsForImportInput{}
				for {
					output, err := svc.ListEnabledProductsForImportWithContext(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "securityhub:ListEnabledProductsForImport", err)
						break
					}

					for _, productArn := range output.ProductSubscriptions {
						arn := ""
						if productArn != nil {
							arn = *productArn
						}

						allProducts = append(allProducts, shProduct{
							ProductSubscriptionArn: arn,
							Region:                 region,
						})
					}

					if output.NextToken == nil {
						break
					}
					input.NextToken = output.NextToken
				}
			}

			if len(allProducts) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allProducts, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "securityhub:ListEnabledProductsForImport", err)
				return []types.ScanResult{
					{
						ServiceName: "SecurityHub",
						MethodName:  "securityhub:ListEnabledProductsForImport",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			products, ok := output.([]shProduct)
			if !ok {
				utils.HandleAWSError(debug, "securityhub:ListEnabledProductsForImport", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, product := range products {
				results = append(results, types.ScanResult{
					ServiceName:  "SecurityHub",
					MethodName:   "securityhub:ListEnabledProductsForImport",
					ResourceType: "product",
					ResourceName: product.ProductSubscriptionArn,
					Details: map[string]interface{}{
						"ProductSubscriptionArn": product.ProductSubscriptionArn,
						"Region":                 product.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "securityhub:ListEnabledProductsForImport",
					fmt.Sprintf("Security Hub Product: %s (Region: %s)", utils.ColorizeItem(product.ProductSubscriptionArn), product.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
