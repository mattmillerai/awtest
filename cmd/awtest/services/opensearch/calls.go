package opensearch

import (
	"context"
	"fmt"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/opensearchservice"
)

const describeDomainsBatchSize = 5

type osDomain struct {
	Name          string
	ARN           string
	Endpoint      string
	EngineVersion string
	Region        string
}

type osDomainPolicy struct {
	DomainName   string
	AccessPolicy string
	Region       string
}

type osDomainEncryption struct {
	DomainName                  string
	EncryptionAtRestEnabled     bool
	KmsKeyId                    string
	NodeToNodeEncryptionEnabled bool
	Region                      string
}

type regionDomains struct {
	Region           string
	DomainStatusList []*opensearchservice.DomainStatus
}

// describeDomainsByRegion lists all OpenSearch domains across regions and returns
// their full DomainStatus details. Each Call function processes the results differently.
func describeDomainsByRegion(ctx context.Context, sess *session.Session, methodName string) ([]regionDomains, error) {
	var allRegionDomains []regionDomains
	var lastErr error

	for _, region := range types.Regions {
		svc := opensearchservice.New(sess, &aws.Config{Region: aws.String(region)})

		listOutput, err := svc.ListDomainNamesWithContext(ctx, &opensearchservice.ListDomainNamesInput{})
		if err != nil {
			lastErr = err
			utils.HandleAWSError(false, methodName, err)
			break
		}

		if len(listOutput.DomainNames) == 0 {
			continue
		}

		var domainNames []*string
		for _, d := range listOutput.DomainNames {
			if d.DomainName != nil {
				domainNames = append(domainNames, d.DomainName)
			}
		}

		var regionStatuses []*opensearchservice.DomainStatus
		for i := 0; i < len(domainNames); i += describeDomainsBatchSize {
			end := i + describeDomainsBatchSize
			if end > len(domainNames) {
				end = len(domainNames)
			}
			batch := domainNames[i:end]

			descOutput, err := svc.DescribeDomainsWithContext(ctx, &opensearchservice.DescribeDomainsInput{
				DomainNames: batch,
			})
			if err != nil {
				lastErr = err
				utils.HandleAWSError(false, methodName, err)
				// Continue to next batch for partial recovery; a transient error
				// on one batch does not necessarily affect subsequent batches.
				continue
			}

			regionStatuses = append(regionStatuses, descOutput.DomainStatusList...)
		}

		if len(regionStatuses) > 0 {
			allRegionDomains = append(allRegionDomains, regionDomains{
				Region:           region,
				DomainStatusList: regionStatuses,
			})
		}
	}

	if len(allRegionDomains) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return allRegionDomains, nil
}

var OpenSearchCalls = []types.AWSService{
	{
		Name: "opensearch:ListDomains",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			regionResults, err := describeDomainsByRegion(ctx, sess, "opensearch:ListDomains")
			if err != nil {
				return nil, err
			}

			var allDomains []osDomain
			for _, rd := range regionResults {
				for _, ds := range rd.DomainStatusList {
					name := ""
					if ds.DomainName != nil {
						name = *ds.DomainName
					}

					arn := ""
					if ds.ARN != nil {
						arn = *ds.ARN
					}

					endpoint := ""
					if ds.Endpoint != nil {
						endpoint = *ds.Endpoint
					} else if ds.Endpoints != nil {
						if vpcEndpoint, ok := ds.Endpoints["vpc"]; ok && vpcEndpoint != nil {
							endpoint = *vpcEndpoint
						}
					}

					engineVersion := ""
					if ds.EngineVersion != nil {
						engineVersion = *ds.EngineVersion
					}

					allDomains = append(allDomains, osDomain{
						Name:          name,
						ARN:           arn,
						Endpoint:      endpoint,
						EngineVersion: engineVersion,
						Region:        rd.Region,
					})
				}
			}

			return allDomains, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "opensearch:ListDomains", err)
				return []types.ScanResult{
					{
						ServiceName: "OpenSearch",
						MethodName:  "opensearch:ListDomains",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			domains, ok := output.([]osDomain)
			if !ok {
				utils.HandleAWSError(debug, "opensearch:ListDomains", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, domain := range domains {
				results = append(results, types.ScanResult{
					ServiceName:  "OpenSearch",
					MethodName:   "opensearch:ListDomains",
					ResourceType: "domain",
					ResourceName: domain.Name,
					Details: map[string]interface{}{
						"Name":          domain.Name,
						"ARN":           domain.ARN,
						"Endpoint":      domain.Endpoint,
						"EngineVersion": domain.EngineVersion,
						"Region":        domain.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "opensearch:ListDomains",
					fmt.Sprintf("OpenSearch Domain: %s (Endpoint: %s, Engine: %s, Region: %s)", utils.ColorizeItem(domain.Name), domain.Endpoint, domain.EngineVersion, domain.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "opensearch:DescribeDomainAccessPolicies",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			regionResults, err := describeDomainsByRegion(ctx, sess, "opensearch:DescribeDomainAccessPolicies")
			if err != nil {
				return nil, err
			}

			var allPolicies []osDomainPolicy
			for _, rd := range regionResults {
				for _, ds := range rd.DomainStatusList {
					if ds.AccessPolicies == nil || *ds.AccessPolicies == "" {
						continue
					}

					domainName := ""
					if ds.DomainName != nil {
						domainName = *ds.DomainName
					}

					allPolicies = append(allPolicies, osDomainPolicy{
						DomainName:   domainName,
						AccessPolicy: *ds.AccessPolicies,
						Region:       rd.Region,
					})
				}
			}

			return allPolicies, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "opensearch:DescribeDomainAccessPolicies", err)
				return []types.ScanResult{
					{
						ServiceName: "OpenSearch",
						MethodName:  "opensearch:DescribeDomainAccessPolicies",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			policies, ok := output.([]osDomainPolicy)
			if !ok {
				utils.HandleAWSError(debug, "opensearch:DescribeDomainAccessPolicies", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, policy := range policies {
				results = append(results, types.ScanResult{
					ServiceName:  "OpenSearch",
					MethodName:   "opensearch:DescribeDomainAccessPolicies",
					ResourceType: "access-policy",
					ResourceName: policy.DomainName,
					Details: map[string]interface{}{
						"DomainName":   policy.DomainName,
						"AccessPolicy": policy.AccessPolicy,
						"Region":       policy.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "opensearch:DescribeDomainAccessPolicies",
					fmt.Sprintf("OpenSearch Access Policy: %s (Region: %s)", utils.ColorizeItem(policy.DomainName), policy.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "opensearch:DescribeDomainEncryption",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			regionResults, err := describeDomainsByRegion(ctx, sess, "opensearch:DescribeDomainEncryption")
			if err != nil {
				return nil, err
			}

			var allEncryption []osDomainEncryption
			for _, rd := range regionResults {
				for _, ds := range rd.DomainStatusList {
					domainName := ""
					if ds.DomainName != nil {
						domainName = *ds.DomainName
					}

					encAtRest := false
					kmsKeyId := ""
					if ds.EncryptionAtRestOptions != nil {
						if ds.EncryptionAtRestOptions.Enabled != nil {
							encAtRest = *ds.EncryptionAtRestOptions.Enabled
						}
						if ds.EncryptionAtRestOptions.KmsKeyId != nil {
							kmsKeyId = *ds.EncryptionAtRestOptions.KmsKeyId
						}
					}

					nodeToNode := false
					if ds.NodeToNodeEncryptionOptions != nil {
						if ds.NodeToNodeEncryptionOptions.Enabled != nil {
							nodeToNode = *ds.NodeToNodeEncryptionOptions.Enabled
						}
					}

					allEncryption = append(allEncryption, osDomainEncryption{
						DomainName:                  domainName,
						EncryptionAtRestEnabled:     encAtRest,
						KmsKeyId:                    kmsKeyId,
						NodeToNodeEncryptionEnabled: nodeToNode,
						Region:                      rd.Region,
					})
				}
			}

			return allEncryption, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "opensearch:DescribeDomainEncryption", err)
				return []types.ScanResult{
					{
						ServiceName: "OpenSearch",
						MethodName:  "opensearch:DescribeDomainEncryption",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			encryptions, ok := output.([]osDomainEncryption)
			if !ok {
				utils.HandleAWSError(debug, "opensearch:DescribeDomainEncryption", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, enc := range encryptions {
				results = append(results, types.ScanResult{
					ServiceName:  "OpenSearch",
					MethodName:   "opensearch:DescribeDomainEncryption",
					ResourceType: "encryption-config",
					ResourceName: enc.DomainName,
					Details: map[string]interface{}{
						"DomainName":                  enc.DomainName,
						"EncryptionAtRestEnabled":     enc.EncryptionAtRestEnabled,
						"KmsKeyId":                    enc.KmsKeyId,
						"NodeToNodeEncryptionEnabled": enc.NodeToNodeEncryptionEnabled,
						"Region":                      enc.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "opensearch:DescribeDomainEncryption",
					fmt.Sprintf("OpenSearch Encryption: %s (AtRest: %t, NodeToNode: %t, Region: %s)", utils.ColorizeItem(enc.DomainName), enc.EncryptionAtRestEnabled, enc.NodeToNodeEncryptionEnabled, enc.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
