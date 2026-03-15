package types

import (
	"errors"
	"testing"
	"time"
)

func TestScanResult_HasError_WithError(t *testing.T) {
	result := ScanResult{
		ServiceName: "S3",
		MethodName:  "s3:ListBuckets",
		Error:       errors.New("access denied"),
		Timestamp:   time.Now(),
	}
	if !result.HasError() {
		t.Error("HasError() should return true when Error is set")
	}
}

func TestScanResult_HasError_WithoutError(t *testing.T) {
	result := ScanResult{
		ServiceName:  "S3",
		MethodName:   "s3:ListBuckets",
		ResourceType: "bucket",
		ResourceName: "my-bucket",
		Timestamp:    time.Now(),
	}
	if result.HasError() {
		t.Error("HasError() should return false when Error is nil")
	}
}

func TestScanResult_Creation(t *testing.T) {
	now := time.Now()
	details := map[string]interface{}{"region": "us-east-1", "count": 5}

	result := ScanResult{
		ServiceName:  "DynamoDB",
		MethodName:   "dynamodb:ListTables",
		ResourceType: "table",
		ResourceName: "users-table",
		Details:      details,
		Timestamp:    now,
	}

	if result.ServiceName != "DynamoDB" {
		t.Errorf("ServiceName = %s, want DynamoDB", result.ServiceName)
	}
	if result.MethodName != "dynamodb:ListTables" {
		t.Errorf("MethodName = %s, want dynamodb:ListTables", result.MethodName)
	}
	if result.ResourceType != "table" {
		t.Errorf("ResourceType = %s, want table", result.ResourceType)
	}
	if result.ResourceName != "users-table" {
		t.Errorf("ResourceName = %s, want users-table", result.ResourceName)
	}
	if result.Details["region"] != "us-east-1" {
		t.Error("Details region mismatch")
	}
	if result.Details["count"] != 5 {
		t.Error("Details count mismatch")
	}
	if !result.Timestamp.Equal(now) {
		t.Error("Timestamp mismatch")
	}
	if result.Error != nil {
		t.Error("Error should be nil")
	}
}

func TestScanResult_ZeroValue(t *testing.T) {
	var result ScanResult
	if result.HasError() {
		t.Error("zero-value ScanResult should not have an error")
	}
	if result.ServiceName != "" {
		t.Error("zero-value ServiceName should be empty")
	}
	if result.Details != nil {
		t.Error("zero-value Details should be nil")
	}
}

func TestScanResult_NilDetails(t *testing.T) {
	result := ScanResult{
		ServiceName: "EC2",
		MethodName:  "ec2:DescribeInstances",
		Timestamp:   time.Now(),
	}
	if result.Details != nil {
		t.Error("Details should be nil when not set")
	}
}

func TestInvalidKeyError(t *testing.T) {
	err := &InvalidKeyError{Message: "invalid key"}
	if err.Error() != "invalid key" {
		t.Errorf("Error() = %s, want 'invalid key'", err.Error())
	}

	// Verify it satisfies the error interface
	var _ error = err
}

func TestAwsErrorMessages(t *testing.T) {
	expectedKeys := []string{
		"UnauthorizedOperation",
		"InvalidAccessKeyId",
		"AccessDeniedException",
		"InvalidClientTokenId",
	}
	for _, key := range expectedKeys {
		if _, ok := AwsErrorMessages[key]; !ok {
			t.Errorf("AwsErrorMessages missing key: %s", key)
		}
	}
}

func TestConstants(t *testing.T) {
	if DefaultModuleName != "AWTest" {
		t.Errorf("DefaultModuleName = %s, want AWTest", DefaultModuleName)
	}
	if InvalidAccessKeyId != "InvalidAccessKeyId" {
		t.Errorf("InvalidAccessKeyId = %s, want InvalidAccessKeyId", InvalidAccessKeyId)
	}
	if InvalidClientTokenId != "InvalidClientTokenId" {
		t.Errorf("InvalidClientTokenId = %s, want InvalidClientTokenId", InvalidClientTokenId)
	}
}

func TestRegions(t *testing.T) {
	if len(Regions) == 0 {
		t.Error("Regions should not be empty")
	}
	// Check that us-east-1 is present
	found := false
	for _, r := range Regions {
		if r == "us-east-1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Regions should contain us-east-1")
	}
}
