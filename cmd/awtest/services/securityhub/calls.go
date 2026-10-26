package securityhub

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/securityhub"
	shtypes "github.com/aws/aws-sdk-go-v2/service/securityhub/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
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
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allStandards []shStandard
			var lastErr error

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, securityhub.NewFromConfig)

				input := &securityhub.GetEnabledStandardsInput{}
				for {
					output, err := svc.GetEnabledStandards(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "securityhub:GetEnabledStandards", err)
						break
					}

					for _, sub := range output.StandardsSubscriptions {
						allStandards = append(allStandards, shStandard{
							StandardsSubscriptionArn: aws.ToString(sub.StandardsSubscriptionArn),
							StandardsArn:             aws.ToString(sub.StandardsArn),
							StandardsStatus:          string(sub.StandardsStatus),
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
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allFindings []shFinding
			var lastErr error

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, securityhub.NewFromConfig)

				input := &securityhub.GetFindingsInput{
					Filters: &shtypes.AwsSecurityFindingFilters{
						RecordState: []shtypes.StringFilter{
							{
								Comparison: shtypes.StringFilterComparison("EQUALS"),
								Value:      aws.String("ACTIVE"),
							},
						},
					},
					MaxResults: aws.Int32(100),
					SortCriteria: []shtypes.SortCriterion{
						{
							Field:     aws.String("SeverityLabel"),
							SortOrder: shtypes.SortOrder("desc"),
						},
					},
				}

				output, err := svc.GetFindings(ctx, input)
				if err != nil {
					lastErr = err
					utils.HandleAWSError(false, "securityhub:GetFindings", err)
					continue
				}

				for _, finding := range output.Findings {
					severityLabel := ""
					if finding.Severity != nil {
						severityLabel = string(finding.Severity.Label)
					}

					complianceStatus := ""
					if finding.Compliance != nil {
						complianceStatus = string(finding.Compliance.Status)
					}

					resourceType := ""
					if len(finding.Resources) > 0 {
						resourceType = aws.ToString(finding.Resources[0].Type)
					}

					allFindings = append(allFindings, shFinding{
						Id:               aws.ToString(finding.Id),
						Title:            aws.ToString(finding.Title),
						SeverityLabel:    severityLabel,
						ComplianceStatus: complianceStatus,
						ProductName:      aws.ToString(finding.ProductName),
						ResourceType:     resourceType,
						Region:           region,
						GeneratorId:      aws.ToString(finding.GeneratorId),
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
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allProducts []shProduct
			var lastErr error

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, securityhub.NewFromConfig)

				input := &securityhub.ListEnabledProductsForImportInput{}
				for {
					output, err := svc.ListEnabledProductsForImport(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "securityhub:ListEnabledProductsForImport", err)
						break
					}

					for _, productArn := range output.ProductSubscriptions {
						allProducts = append(allProducts, shProduct{
							ProductSubscriptionArn: productArn,
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
