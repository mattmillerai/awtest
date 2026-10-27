package accessanalyzer

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/accessanalyzer"
	aatypes "github.com/aws/aws-sdk-go-v2/service/accessanalyzer/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

const serviceName = "Access Analyzer"

// maxFindingsPerAnalyzer caps accessanalyzer:ListFindingsV2 per analyzer.
const maxFindingsPerAnalyzer = 100

// finding is a finding summary tagged with the analyzer it belongs to.
type finding struct {
	Id          string
	Status      string
	Resource    string
	AnalyzerArn string
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

func listAnalyzers(ctx context.Context, svc *accessanalyzer.Client) ([]aatypes.AnalyzerSummary, error) {
	var analyzers []aatypes.AnalyzerSummary
	p := accessanalyzer.NewListAnalyzersPaginator(svc, &accessanalyzer.ListAnalyzersInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		analyzers = append(analyzers, page.Analyzers...)
	}
	return analyzers, nil
}

var AccessAnalyzerCalls = []types.AWSService{
	{
		Name: "accessanalyzer:ListAnalyzers",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var all []aatypes.AnalyzerSummary
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, accessanalyzer.NewFromConfig)
				analyzers, err := listAnalyzers(ctx, svc)
				if err != nil {
					return nil, err
				}
				all = append(all, analyzers...)
			}
			return all, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			const method = "accessanalyzer:ListAnalyzers"
			var results []types.ScanResult
			if err != nil {
				return errorResults(debug, method, err)
			}
			analyzers, ok := output.([]aatypes.AnalyzerSummary)
			if !ok {
				utils.HandleAWSError(debug, method, fmt.Errorf("unexpected output type %T", output))
				return results
			}
			for _, a := range analyzers {
				name := aws.ToString(a.Name)
				results = append(results, types.ScanResult{
					ServiceName:  serviceName,
					MethodName:   method,
					ResourceType: "analyzer",
					ResourceName: name,
					Details: map[string]interface{}{
						"Arn":    aws.ToString(a.Arn),
						"Type":   string(a.Type),
						"Status": string(a.Status),
					},
					Timestamp: time.Now(),
				})
				utils.PrintResult(debug, "", method, fmt.Sprintf("Found Access Analyzer: %s (Type: %s, Status: %s)", utils.ColorizeItem(name), string(a.Type), string(a.Status)), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "accessanalyzer:ListFindingsV2",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			all := []finding{}
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, accessanalyzer.NewFromConfig)
				analyzers, err := listAnalyzers(ctx, svc)
				if err != nil {
					return nil, err
				}
				for _, a := range analyzers {
					count := 0
					p := accessanalyzer.NewListFindingsV2Paginator(svc, &accessanalyzer.ListFindingsV2Input{
						AnalyzerArn: a.Arn,
						MaxResults:  aws.Int32(maxFindingsPerAnalyzer),
					})
					for count < maxFindingsPerAnalyzer && p.HasMorePages() {
						page, err := p.NextPage(ctx)
						if err != nil {
							return nil, err
						}
						for _, f := range page.Findings {
							if count >= maxFindingsPerAnalyzer {
								break
							}
							count++
							all = append(all, finding{
								Id:          aws.ToString(f.Id),
								Status:      string(f.Status),
								Resource:    aws.ToString(f.Resource),
								AnalyzerArn: aws.ToString(a.Arn),
							})
						}
					}
				}
			}
			return all, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			const method = "accessanalyzer:ListFindingsV2"
			var results []types.ScanResult
			if err != nil {
				return errorResults(debug, method, err)
			}
			findings, ok := output.([]finding)
			if !ok {
				utils.HandleAWSError(debug, method, fmt.Errorf("unexpected output type %T", output))
				return results
			}
			for _, f := range findings {
				results = append(results, types.ScanResult{
					ServiceName:  serviceName,
					MethodName:   method,
					ResourceType: "finding",
					ResourceName: f.Id,
					Details: map[string]interface{}{
						"Status":      f.Status,
						"Resource":    f.Resource,
						"AnalyzerArn": f.AnalyzerArn,
					},
					Timestamp: time.Now(),
				})
				utils.PrintResult(debug, "", method, fmt.Sprintf("Found Access Analyzer finding: %s (Status: %s, Resource: %s)", utils.ColorizeItem(f.Id), f.Status, f.Resource), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
