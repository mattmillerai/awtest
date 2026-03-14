package services

import (
	"testing"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
)

// mockServices creates a set of test AWSService entries mimicking real service prefixes.
func mockServices() []types.AWSService {
	return []types.AWSService{
		{Name: "sts:GetCallerIdentity"},
		{Name: "s3:ListBuckets"},
		{Name: "ec2:DescribeInstances"},
		{Name: "ec2:DescribeVpcs"},
		{Name: "iam:ListUsers"},
		{Name: "cognitoidentity:ListIdentityPools"},
		{Name: "cognito-idp:ListUserPools"},
		{Name: "elasticache:DescribeCacheClusters"},
		{Name: "elasticbeanstalk:DescribeApplications"},
		{Name: "ivsChat:ListRooms"},
		{Name: "ivsRealtime:ListStages"},
	}
}

func TestFilterServices_NoFilter(t *testing.T) {
	all := mockServices()
	result := FilterServices(all, "", "")
	if len(result) != len(all) {
		t.Errorf("expected %d services, got %d", len(all), len(result))
	}
}

func TestFilterServices_EmptyStrings(t *testing.T) {
	all := mockServices()
	result := FilterServices(all, "  ", "  ")
	if len(result) != len(all) {
		t.Errorf("expected %d services, got %d", len(all), len(result))
	}
}

func TestFilterServices_IncludeExactMatch(t *testing.T) {
	all := mockServices()
	result := FilterServices(all, "s3", "")
	if len(result) != 1 {
		t.Fatalf("expected 1 service, got %d", len(result))
	}
	if extractServiceName(result[0].Name) != "s3" {
		t.Errorf("expected s3, got %s", result[0].Name)
	}
}

func TestFilterServices_IncludeCaseInsensitive(t *testing.T) {
	all := mockServices()
	tests := []struct {
		name    string
		include string
	}{
		{"uppercase", "S3"},
		{"lowercase", "s3"},
		{"mixed", "S3,IAM"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterServices(all, tt.include, "")
			if len(result) == 0 {
				t.Errorf("expected results for include=%q, got none", tt.include)
			}
		})
	}
}

func TestFilterServices_IncludePartialMatch(t *testing.T) {
	all := mockServices()
	result := FilterServices(all, "cognito", "")
	// Should match cognitoidentity and cognito-idp
	if len(result) != 2 {
		t.Errorf("expected 2 services matching 'cognito', got %d", len(result))
		for _, svc := range result {
			t.Logf("  matched: %s", svc.Name)
		}
	}
}

func TestFilterServices_IncludeMultipleEC2(t *testing.T) {
	all := mockServices()
	result := FilterServices(all, "ec2", "")
	// ec2:DescribeInstances and ec2:DescribeVpcs
	if len(result) != 2 {
		t.Errorf("expected 2 ec2 services, got %d", len(result))
	}
}

func TestFilterServices_ExcludeOnly(t *testing.T) {
	all := mockServices()
	result := FilterServices(all, "", "iam")
	for _, svc := range result {
		if extractServiceName(svc.Name) == "iam" {
			t.Errorf("expected iam to be excluded, but found %s", svc.Name)
		}
	}
	if len(result) != len(all)-1 {
		t.Errorf("expected %d services, got %d", len(all)-1, len(result))
	}
}

func TestFilterServices_IncludeExcludeCombination(t *testing.T) {
	all := mockServices()
	result := FilterServices(all, "s3,ec2,iam", "iam")
	// Include s3+ec2(x2)+iam = 4, then exclude iam = 3
	if len(result) != 3 {
		t.Errorf("expected 3 services (s3 + 2xec2), got %d", len(result))
		for _, svc := range result {
			t.Logf("  got: %s", svc.Name)
		}
	}
	for _, svc := range result {
		if extractServiceName(svc.Name) == "iam" {
			t.Errorf("iam should have been excluded")
		}
	}
}

func TestFilterServices_NoMatches(t *testing.T) {
	all := mockServices()
	result := FilterServices(all, "nonexistent", "")
	if len(result) != 0 {
		t.Errorf("expected 0 services, got %d", len(result))
	}
}

func TestFilterServices_ElasticPartialMatch(t *testing.T) {
	all := mockServices()
	result := FilterServices(all, "elastic", "")
	// Should match elasticache and elasticbeanstalk
	if len(result) != 2 {
		t.Errorf("expected 2 services matching 'elastic', got %d", len(result))
	}
}

func TestFilterServices_CamelCasePrefix(t *testing.T) {
	all := mockServices()
	result := FilterServices(all, "ivschat", "")
	if len(result) != 1 {
		t.Errorf("expected 1 service matching 'ivschat', got %d", len(result))
	}
}

func TestFilterServices_OverMatchingPrevented(t *testing.T) {
	all := mockServices()
	// "s33" should NOT match "s3" — filter is longer than prefix
	result := FilterServices(all, "s33", "")
	if len(result) != 0 {
		t.Errorf("expected 0 services for 's33' (should not over-match 's3'), got %d", len(result))
	}
	// "iamroles" should NOT match "iam"
	result = FilterServices(all, "iamroles", "")
	if len(result) != 0 {
		t.Errorf("expected 0 services for 'iamroles', got %d", len(result))
	}
}

func TestFilterServices_WhitespaceInCSV(t *testing.T) {
	all := mockServices()
	result := FilterServices(all, "s3 , ec2 , iam", "")
	// s3(1) + ec2(2) + iam(1) = 4
	if len(result) != 4 {
		t.Errorf("expected 4 services, got %d", len(result))
	}
}

func TestParseServiceList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"empty", "", 0},
		{"whitespace", "  ", 0},
		{"single", "s3", 1},
		{"multiple", "s3,ec2,iam", 3},
		{"whitespace_trimmed", " s3 , ec2 ", 2},
		{"duplicates", "s3,s3", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseServiceList(tt.input)
			if len(result) != tt.expected {
				t.Errorf("parseServiceList(%q) returned %d items, expected %d", tt.input, len(result), tt.expected)
			}
		})
	}
}

func TestExtractServiceName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"s3:ListBuckets", "s3"},
		{"ec2:DescribeInstances", "ec2"},
		{"ivsChat:ListRooms", "ivschat"},
		{"noColon", "nocolon"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractServiceName(tt.input)
			if result != tt.expected {
				t.Errorf("extractServiceName(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
