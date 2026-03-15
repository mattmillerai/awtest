package opensearch

import (
	"fmt"
	"testing"
)

func TestListDomainsProcess(t *testing.T) {
	process := OpenSearchCalls[0].Process

	tests := []struct {
		name              string
		output            interface{}
		err               error
		wantLen           int
		wantError         bool
		wantResourceName  string
		wantName          string
		wantARN           string
		wantEndpoint      string
		wantEngineVersion string
		wantRegion        string
	}{
		{
			name: "valid domains with full details",
			output: []osDomain{
				{
					Name:          "my-search-domain",
					ARN:           "arn:aws:es:us-east-1:111111111111:domain/my-search-domain",
					Endpoint:      "search-my-search-domain-abc123.us-east-1.es.amazonaws.com",
					EngineVersion: "OpenSearch_2.11",
					Region:        "us-east-1",
				},
				{
					Name:          "analytics-domain",
					ARN:           "arn:aws:es:us-west-2:111111111111:domain/analytics-domain",
					Endpoint:      "search-analytics-domain-xyz789.us-west-2.es.amazonaws.com",
					EngineVersion: "Elasticsearch_7.10",
					Region:        "us-west-2",
				},
			},
			wantLen:           2,
			wantResourceName:  "my-search-domain",
			wantName:          "my-search-domain",
			wantARN:           "arn:aws:es:us-east-1:111111111111:domain/my-search-domain",
			wantEndpoint:      "search-my-search-domain-abc123.us-east-1.es.amazonaws.com",
			wantEngineVersion: "OpenSearch_2.11",
			wantRegion:        "us-east-1",
		},
		{
			name:    "empty results",
			output:  []osDomain{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
		},
		{
			name: "nil-safe fields (empty strings)",
			output: []osDomain{
				{
					Name:          "",
					ARN:           "",
					Endpoint:      "",
					EngineVersion: "",
					Region:        "",
				},
			},
			wantLen:           1,
			wantResourceName:  "",
			wantName:          "",
			wantARN:           "",
			wantEndpoint:      "",
			wantEngineVersion: "",
			wantRegion:        "",
		},
		{
			name:    "type assertion failure",
			output:  "invalid type",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := process(tt.output, tt.err, false)

			if len(results) != tt.wantLen {
				t.Fatalf("expected %d results, got %d", tt.wantLen, len(results))
			}

			if tt.wantError {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "OpenSearch" {
					t.Errorf("expected ServiceName 'OpenSearch', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "opensearch:ListDomains" {
					t.Errorf("expected MethodName 'opensearch:ListDomains', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "OpenSearch" {
					t.Errorf("expected ServiceName 'OpenSearch', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "opensearch:ListDomains" {
					t.Errorf("expected MethodName 'opensearch:ListDomains', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "domain" {
					t.Errorf("expected ResourceType 'domain', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if name, ok := results[0].Details["Name"].(string); ok {
					if name != tt.wantName {
						t.Errorf("expected Name '%s', got '%s'", tt.wantName, name)
					}
				} else if tt.wantName != "" {
					t.Errorf("expected Name in Details, got none")
				}
				if arn, ok := results[0].Details["ARN"].(string); ok {
					if arn != tt.wantARN {
						t.Errorf("expected ARN '%s', got '%s'", tt.wantARN, arn)
					}
				} else if tt.wantARN != "" {
					t.Errorf("expected ARN in Details, got none")
				}
				if endpoint, ok := results[0].Details["Endpoint"].(string); ok {
					if endpoint != tt.wantEndpoint {
						t.Errorf("expected Endpoint '%s', got '%s'", tt.wantEndpoint, endpoint)
					}
				} else if tt.wantEndpoint != "" {
					t.Errorf("expected Endpoint in Details, got none")
				}
				if engineVersion, ok := results[0].Details["EngineVersion"].(string); ok {
					if engineVersion != tt.wantEngineVersion {
						t.Errorf("expected EngineVersion '%s', got '%s'", tt.wantEngineVersion, engineVersion)
					}
				} else if tt.wantEngineVersion != "" {
					t.Errorf("expected EngineVersion in Details, got none")
				}
				if region, ok := results[0].Details["Region"].(string); ok {
					if region != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, region)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}

func TestDescribeDomainAccessPoliciesProcess(t *testing.T) {
	process := OpenSearchCalls[1].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantDomainName   string
		wantAccessPolicy string
		wantRegion       string
	}{
		{
			name: "valid policies with JSON content",
			output: []osDomainPolicy{
				{
					DomainName:   "my-search-domain",
					AccessPolicy: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"*"},"Action":"es:*","Resource":"arn:aws:es:us-east-1:111111111111:domain/my-search-domain/*"}]}`,
					Region:       "us-east-1",
				},
				{
					DomainName:   "restricted-domain",
					AccessPolicy: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::111111111111:root"},"Action":"es:*","Resource":"arn:aws:es:us-west-2:111111111111:domain/restricted-domain/*"}]}`,
					Region:       "us-west-2",
				},
			},
			wantLen:          2,
			wantResourceName: "my-search-domain",
			wantDomainName:   "my-search-domain",
			wantAccessPolicy: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"*"},"Action":"es:*","Resource":"arn:aws:es:us-east-1:111111111111:domain/my-search-domain/*"}]}`,
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty results",
			output:  []osDomainPolicy{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
		},
		{
			name: "nil-safe fields (empty strings)",
			output: []osDomainPolicy{
				{
					DomainName:   "",
					AccessPolicy: "",
					Region:       "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantDomainName:   "",
			wantAccessPolicy: "",
			wantRegion:       "",
		},
		{
			name:    "type assertion failure",
			output:  "invalid type",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := process(tt.output, tt.err, false)

			if len(results) != tt.wantLen {
				t.Fatalf("expected %d results, got %d", tt.wantLen, len(results))
			}

			if tt.wantError {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "OpenSearch" {
					t.Errorf("expected ServiceName 'OpenSearch', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "opensearch:DescribeDomainAccessPolicies" {
					t.Errorf("expected MethodName 'opensearch:DescribeDomainAccessPolicies', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "OpenSearch" {
					t.Errorf("expected ServiceName 'OpenSearch', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "opensearch:DescribeDomainAccessPolicies" {
					t.Errorf("expected MethodName 'opensearch:DescribeDomainAccessPolicies', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "access-policy" {
					t.Errorf("expected ResourceType 'access-policy', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if domainName, ok := results[0].Details["DomainName"].(string); ok {
					if domainName != tt.wantDomainName {
						t.Errorf("expected DomainName '%s', got '%s'", tt.wantDomainName, domainName)
					}
				} else if tt.wantDomainName != "" {
					t.Errorf("expected DomainName in Details, got none")
				}
				if accessPolicy, ok := results[0].Details["AccessPolicy"].(string); ok {
					if accessPolicy != tt.wantAccessPolicy {
						t.Errorf("expected AccessPolicy '%s', got '%s'", tt.wantAccessPolicy, accessPolicy)
					}
				} else if tt.wantAccessPolicy != "" {
					t.Errorf("expected AccessPolicy in Details, got none")
				}
				if region, ok := results[0].Details["Region"].(string); ok {
					if region != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, region)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}

func TestDescribeDomainEncryptionProcess(t *testing.T) {
	process := OpenSearchCalls[2].Process

	tests := []struct {
		name                        string
		output                      interface{}
		err                         error
		wantLen                     int
		wantError                   bool
		wantResourceName            string
		wantDomainName              string
		wantEncryptionAtRestEnabled bool
		wantKmsKeyId                string
		wantNodeToNodeEnabled       bool
		wantRegion                  string
	}{
		{
			name: "valid encryption configs - both enabled",
			output: []osDomainEncryption{
				{
					DomainName:                  "encrypted-domain",
					EncryptionAtRestEnabled:     true,
					KmsKeyId:                    "arn:aws:kms:us-east-1:111111111111:key/12345678-1234-1234-1234-123456789012",
					NodeToNodeEncryptionEnabled: true,
					Region:                      "us-east-1",
				},
			},
			wantLen:                     1,
			wantResourceName:            "encrypted-domain",
			wantDomainName:              "encrypted-domain",
			wantEncryptionAtRestEnabled: true,
			wantKmsKeyId:                "arn:aws:kms:us-east-1:111111111111:key/12345678-1234-1234-1234-123456789012",
			wantNodeToNodeEnabled:       true,
			wantRegion:                  "us-east-1",
		},
		{
			name: "valid encryption configs - both disabled",
			output: []osDomainEncryption{
				{
					DomainName:                  "unencrypted-domain",
					EncryptionAtRestEnabled:     false,
					KmsKeyId:                    "",
					NodeToNodeEncryptionEnabled: false,
					Region:                      "us-west-2",
				},
			},
			wantLen:                     1,
			wantResourceName:            "unencrypted-domain",
			wantDomainName:              "unencrypted-domain",
			wantEncryptionAtRestEnabled: false,
			wantKmsKeyId:                "",
			wantNodeToNodeEnabled:       false,
			wantRegion:                  "us-west-2",
		},
		{
			name:    "empty results",
			output:  []osDomainEncryption{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
		},
		{
			name: "nil-safe fields (empty strings and false booleans)",
			output: []osDomainEncryption{
				{
					DomainName:                  "",
					EncryptionAtRestEnabled:     false,
					KmsKeyId:                    "",
					NodeToNodeEncryptionEnabled: false,
					Region:                      "",
				},
			},
			wantLen:                     1,
			wantResourceName:            "",
			wantDomainName:              "",
			wantEncryptionAtRestEnabled: false,
			wantKmsKeyId:                "",
			wantNodeToNodeEnabled:       false,
			wantRegion:                  "",
		},
		{
			name:    "type assertion failure",
			output:  "invalid type",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := process(tt.output, tt.err, false)

			if len(results) != tt.wantLen {
				t.Fatalf("expected %d results, got %d", tt.wantLen, len(results))
			}

			if tt.wantError {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "OpenSearch" {
					t.Errorf("expected ServiceName 'OpenSearch', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "opensearch:DescribeDomainEncryption" {
					t.Errorf("expected MethodName 'opensearch:DescribeDomainEncryption', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "OpenSearch" {
					t.Errorf("expected ServiceName 'OpenSearch', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "opensearch:DescribeDomainEncryption" {
					t.Errorf("expected MethodName 'opensearch:DescribeDomainEncryption', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "encryption-config" {
					t.Errorf("expected ResourceType 'encryption-config', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if domainName, ok := results[0].Details["DomainName"].(string); ok {
					if domainName != tt.wantDomainName {
						t.Errorf("expected DomainName '%s', got '%s'", tt.wantDomainName, domainName)
					}
				} else if tt.wantDomainName != "" {
					t.Errorf("expected DomainName in Details, got none")
				}
				if encAtRest, ok := results[0].Details["EncryptionAtRestEnabled"].(bool); ok {
					if encAtRest != tt.wantEncryptionAtRestEnabled {
						t.Errorf("expected EncryptionAtRestEnabled %t, got %t", tt.wantEncryptionAtRestEnabled, encAtRest)
					}
				}
				if kmsKeyId, ok := results[0].Details["KmsKeyId"].(string); ok {
					if kmsKeyId != tt.wantKmsKeyId {
						t.Errorf("expected KmsKeyId '%s', got '%s'", tt.wantKmsKeyId, kmsKeyId)
					}
				} else if tt.wantKmsKeyId != "" {
					t.Errorf("expected KmsKeyId in Details, got none")
				}
				if nodeToNode, ok := results[0].Details["NodeToNodeEncryptionEnabled"].(bool); ok {
					if nodeToNode != tt.wantNodeToNodeEnabled {
						t.Errorf("expected NodeToNodeEncryptionEnabled %t, got %t", tt.wantNodeToNodeEnabled, nodeToNode)
					}
				}
				if region, ok := results[0].Details["Region"].(string); ok {
					if region != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, region)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}
