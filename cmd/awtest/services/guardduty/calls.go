package guardduty

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/guardduty"
	guarddutytypes "github.com/aws/aws-sdk-go-v2/service/guardduty/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

type guarddutyDetector struct {
	DetectorId  string
	Status      string
	ServiceRole string
	CreatedAt   string
	Region      string
	Features    []string
}

type guarddutyFinding struct {
	FindingId   string
	Type        string
	Title       string
	Severity    float64
	Description string
	Region      string
	DetectorId  string
}

type guarddutyFilter struct {
	FilterName  string
	Action      string
	Description string
	DetectorId  string
	Region      string
}

var GuardDutyCalls = []types.AWSService{
	{
		Name: "guardduty:ListDetectors",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allDetectors []guarddutyDetector
			var lastErr error

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, guardduty.NewFromConfig)

				input := &guardduty.ListDetectorsInput{}
				for {
					output, err := svc.ListDetectors(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "guardduty:ListDetectors", err)
						break
					}

					for _, detectorId := range output.DetectorIds {
						detectorOutput, err := svc.GetDetector(ctx, &guardduty.GetDetectorInput{
							DetectorId: aws.String(detectorId),
						})
						if err != nil {
							utils.HandleAWSError(false, "guardduty:ListDetectors", err)
							continue
						}

						status := string(detectorOutput.Status)
						serviceRole := aws.ToString(detectorOutput.ServiceRole)
						createdAt := aws.ToString(detectorOutput.CreatedAt)

						var enabledFeatures []string
						for _, feature := range detectorOutput.Features {
							if feature.Name != "" && feature.Status == guarddutytypes.FeatureStatusEnabled {
								enabledFeatures = append(enabledFeatures, string(feature.Name))
							}
						}

						allDetectors = append(allDetectors, guarddutyDetector{
							DetectorId:  detectorId,
							Status:      status,
							ServiceRole: serviceRole,
							CreatedAt:   createdAt,
							Region:      region,
							Features:    enabledFeatures,
						})
					}

					if output.NextToken == nil {
						break
					}
					input.NextToken = output.NextToken
				}
			}

			if len(allDetectors) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allDetectors, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "guardduty:ListDetectors", err)
				return []types.ScanResult{
					{
						ServiceName: "GuardDuty",
						MethodName:  "guardduty:ListDetectors",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			detectors, ok := output.([]guarddutyDetector)
			if !ok {
				utils.HandleAWSError(debug, "guardduty:ListDetectors", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, detector := range detectors {
				featuresDisplay := strings.Join(detector.Features, ", ")

				results = append(results, types.ScanResult{
					ServiceName:  "GuardDuty",
					MethodName:   "guardduty:ListDetectors",
					ResourceType: "detector",
					ResourceName: detector.DetectorId,
					Details: map[string]interface{}{
						"DetectorId":  detector.DetectorId,
						"Status":      detector.Status,
						"ServiceRole": detector.ServiceRole,
						"CreatedAt":   detector.CreatedAt,
						"Region":      detector.Region,
						"Features":    detector.Features,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "guardduty:ListDetectors",
					fmt.Sprintf("GuardDuty Detector: %s (Status: %s, Region: %s, Features: %s)", utils.ColorizeItem(detector.DetectorId), detector.Status, detector.Region, featuresDisplay), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "guardduty:GetFindings",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allFindings []guarddutyFinding
			var lastErr error

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, guardduty.NewFromConfig)

				// Discover detectors in this region
				listDetectorsInput := &guardduty.ListDetectorsInput{}
				for {
					detectorsOutput, err := svc.ListDetectors(ctx, listDetectorsInput)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "guardduty:GetFindings", err)
						break
					}

					for _, detectorId := range detectorsOutput.DetectorIds {
						// List findings for this detector (first 50, sorted by severity DESC)
						listFindingsInput := &guardduty.ListFindingsInput{
							DetectorId: aws.String(detectorId),
							MaxResults: aws.Int32(50),
							SortCriteria: &guarddutytypes.SortCriteria{
								AttributeName: aws.String("severity"),
								OrderBy:       guarddutytypes.OrderByDesc,
							},
						}

						findingsOutput, err := svc.ListFindings(ctx, listFindingsInput)
						if err != nil {
							utils.HandleAWSError(false, "guardduty:GetFindings", err)
							continue
						}

						if len(findingsOutput.FindingIds) == 0 {
							continue
						}

						// Hydrate findings via GetFindings
						getFindingsOutput, err := svc.GetFindings(ctx, &guardduty.GetFindingsInput{
							DetectorId: aws.String(detectorId),
							FindingIds: findingsOutput.FindingIds,
						})
						if err != nil {
							utils.HandleAWSError(false, "guardduty:GetFindings", err)
							continue
						}

						for _, finding := range getFindingsOutput.Findings {
							findingRegion := region
							if finding.Region != nil {
								findingRegion = *finding.Region
							}

							allFindings = append(allFindings, guarddutyFinding{
								FindingId:   aws.ToString(finding.Id),
								Type:        aws.ToString(finding.Type),
								Title:       aws.ToString(finding.Title),
								Severity:    aws.ToFloat64(finding.Severity),
								Description: aws.ToString(finding.Description),
								Region:      findingRegion,
								DetectorId:  detectorId,
							})
						}
					}

					if detectorsOutput.NextToken == nil {
						break
					}
					listDetectorsInput.NextToken = detectorsOutput.NextToken
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
				utils.HandleAWSError(debug, "guardduty:GetFindings", err)
				return []types.ScanResult{
					{
						ServiceName: "GuardDuty",
						MethodName:  "guardduty:GetFindings",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			findings, ok := output.([]guarddutyFinding)
			if !ok {
				utils.HandleAWSError(debug, "guardduty:GetFindings", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, finding := range findings {
				resourceName := finding.Title
				if resourceName == "" {
					resourceName = finding.Type
				}

				results = append(results, types.ScanResult{
					ServiceName:  "GuardDuty",
					MethodName:   "guardduty:GetFindings",
					ResourceType: "finding",
					ResourceName: resourceName,
					Details: map[string]interface{}{
						"FindingId":   finding.FindingId,
						"Type":        finding.Type,
						"Title":       finding.Title,
						"Severity":    finding.Severity,
						"Description": finding.Description,
						"Region":      finding.Region,
						"DetectorId":  finding.DetectorId,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "guardduty:GetFindings",
					fmt.Sprintf("GuardDuty Finding: %s (Severity: %.1f, Type: %s)", utils.ColorizeItem(resourceName), finding.Severity, finding.Type), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "guardduty:ListFilters",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allFilters []guarddutyFilter
			var lastErr error

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, guardduty.NewFromConfig)

				// Discover detectors in this region
				listDetectorsInput := &guardduty.ListDetectorsInput{}
				for {
					detectorsOutput, err := svc.ListDetectors(ctx, listDetectorsInput)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "guardduty:ListFilters", err)
						break
					}

					for _, detectorId := range detectorsOutput.DetectorIds {
						// List filters for this detector (paginated)
						listFiltersInput := &guardduty.ListFiltersInput{
							DetectorId: aws.String(detectorId),
						}
						for {
							filtersOutput, err := svc.ListFilters(ctx, listFiltersInput)
							if err != nil {
								utils.HandleAWSError(false, "guardduty:ListFilters", err)
								break
							}

							for _, filterName := range filtersOutput.FilterNames {
								// Hydrate filter details
								filterOutput, err := svc.GetFilter(ctx, &guardduty.GetFilterInput{
									DetectorId: aws.String(detectorId),
									FilterName: aws.String(filterName),
								})
								if err != nil {
									utils.HandleAWSError(false, "guardduty:ListFilters", err)
									continue
								}

								allFilters = append(allFilters, guarddutyFilter{
									FilterName:  filterName,
									Action:      string(filterOutput.Action),
									Description: aws.ToString(filterOutput.Description),
									DetectorId:  detectorId,
									Region:      region,
								})
							}

							if filtersOutput.NextToken == nil {
								break
							}
							listFiltersInput.NextToken = filtersOutput.NextToken
						}
					}

					if detectorsOutput.NextToken == nil {
						break
					}
					listDetectorsInput.NextToken = detectorsOutput.NextToken
				}
			}

			if len(allFilters) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allFilters, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "guardduty:ListFilters", err)
				return []types.ScanResult{
					{
						ServiceName: "GuardDuty",
						MethodName:  "guardduty:ListFilters",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			filters, ok := output.([]guarddutyFilter)
			if !ok {
				utils.HandleAWSError(debug, "guardduty:ListFilters", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, filter := range filters {
				results = append(results, types.ScanResult{
					ServiceName:  "GuardDuty",
					MethodName:   "guardduty:ListFilters",
					ResourceType: "filter",
					ResourceName: filter.FilterName,
					Details: map[string]interface{}{
						"FilterName":  filter.FilterName,
						"Action":      filter.Action,
						"Description": filter.Description,
						"DetectorId":  filter.DetectorId,
						"Region":      filter.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "guardduty:ListFilters",
					fmt.Sprintf("GuardDuty Filter: %s (Action: %s, Detector: %s)", utils.ColorizeItem(filter.FilterName), filter.Action, filter.DetectorId), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
