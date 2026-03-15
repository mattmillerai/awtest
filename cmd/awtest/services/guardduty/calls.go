package guardduty

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/guardduty"
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
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allDetectors []guarddutyDetector
			var lastErr error

			for _, region := range types.Regions {
				svc := guardduty.New(sess, &aws.Config{Region: aws.String(region)})

				input := &guardduty.ListDetectorsInput{}
				for {
					output, err := svc.ListDetectorsWithContext(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "guardduty:ListDetectors", err)
						break
					}

					for _, detectorId := range output.DetectorIds {
						if detectorId == nil {
							continue
						}

						detectorOutput, err := svc.GetDetectorWithContext(ctx, &guardduty.GetDetectorInput{
							DetectorId: detectorId,
						})
						if err != nil {
							utils.HandleAWSError(false, "guardduty:ListDetectors", err)
							continue
						}

						status := ""
						if detectorOutput.Status != nil {
							status = *detectorOutput.Status
						}

						serviceRole := ""
						if detectorOutput.ServiceRole != nil {
							serviceRole = *detectorOutput.ServiceRole
						}

						createdAt := ""
						if detectorOutput.CreatedAt != nil {
							createdAt = *detectorOutput.CreatedAt
						}

						var enabledFeatures []string
						if detectorOutput.Features != nil {
							for _, feature := range detectorOutput.Features {
								if feature.Name != nil && feature.Status != nil && *feature.Status == "ENABLED" {
									enabledFeatures = append(enabledFeatures, *feature.Name)
								}
							}
						}

						allDetectors = append(allDetectors, guarddutyDetector{
							DetectorId:  *detectorId,
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
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allFindings []guarddutyFinding
			var lastErr error

			for _, region := range types.Regions {
				svc := guardduty.New(sess, &aws.Config{Region: aws.String(region)})

				// Discover detectors in this region
				listDetectorsInput := &guardduty.ListDetectorsInput{}
				for {
					detectorsOutput, err := svc.ListDetectorsWithContext(ctx, listDetectorsInput)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "guardduty:GetFindings", err)
						break
					}

					for _, detectorId := range detectorsOutput.DetectorIds {
						if detectorId == nil {
							continue
						}

						// List findings for this detector (first 50, sorted by severity DESC)
						listFindingsInput := &guardduty.ListFindingsInput{
							DetectorId: detectorId,
							MaxResults: aws.Int64(50),
							SortCriteria: &guardduty.SortCriteria{
								AttributeName: aws.String("severity"),
								OrderBy:       aws.String("DESC"),
							},
						}

						findingsOutput, err := svc.ListFindingsWithContext(ctx, listFindingsInput)
						if err != nil {
							utils.HandleAWSError(false, "guardduty:GetFindings", err)
							continue
						}

						if len(findingsOutput.FindingIds) == 0 {
							continue
						}

						// Hydrate findings via GetFindings
						getFindingsOutput, err := svc.GetFindingsWithContext(ctx, &guardduty.GetFindingsInput{
							DetectorId: detectorId,
							FindingIds: findingsOutput.FindingIds,
						})
						if err != nil {
							utils.HandleAWSError(false, "guardduty:GetFindings", err)
							continue
						}

						for _, finding := range getFindingsOutput.Findings {
							findingId := ""
							if finding.Id != nil {
								findingId = *finding.Id
							}

							findingType := ""
							if finding.Type != nil {
								findingType = *finding.Type
							}

							title := ""
							if finding.Title != nil {
								title = *finding.Title
							}

							severity := float64(0)
							if finding.Severity != nil {
								severity = *finding.Severity
							}

							description := ""
							if finding.Description != nil {
								description = *finding.Description
							}

							findingRegion := region
							if finding.Region != nil {
								findingRegion = *finding.Region
							}

							allFindings = append(allFindings, guarddutyFinding{
								FindingId:   findingId,
								Type:        findingType,
								Title:       title,
								Severity:    severity,
								Description: description,
								Region:      findingRegion,
								DetectorId:  *detectorId,
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
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allFilters []guarddutyFilter
			var lastErr error

			for _, region := range types.Regions {
				svc := guardduty.New(sess, &aws.Config{Region: aws.String(region)})

				// Discover detectors in this region
				listDetectorsInput := &guardduty.ListDetectorsInput{}
				for {
					detectorsOutput, err := svc.ListDetectorsWithContext(ctx, listDetectorsInput)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "guardduty:ListFilters", err)
						break
					}

					for _, detectorId := range detectorsOutput.DetectorIds {
						if detectorId == nil {
							continue
						}

						// List filters for this detector (paginated)
						listFiltersInput := &guardduty.ListFiltersInput{
							DetectorId: detectorId,
						}
						for {
							filtersOutput, err := svc.ListFiltersWithContext(ctx, listFiltersInput)
							if err != nil {
								utils.HandleAWSError(false, "guardduty:ListFilters", err)
								break
							}

							for _, filterName := range filtersOutput.FilterNames {
								if filterName == nil {
									continue
								}

								// Hydrate filter details
								filterOutput, err := svc.GetFilterWithContext(ctx, &guardduty.GetFilterInput{
									DetectorId: detectorId,
									FilterName: filterName,
								})
								if err != nil {
									utils.HandleAWSError(false, "guardduty:ListFilters", err)
									continue
								}

								action := ""
								if filterOutput.Action != nil {
									action = *filterOutput.Action
								}

								description := ""
								if filterOutput.Description != nil {
									description = *filterOutput.Description
								}

								allFilters = append(allFilters, guarddutyFilter{
									FilterName:  *filterName,
									Action:      action,
									Description: description,
									DetectorId:  *detectorId,
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
