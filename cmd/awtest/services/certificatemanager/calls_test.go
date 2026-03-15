package certificatemanager

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/acm"
	"testing"
)

func TestProcess_ValidCertificates(t *testing.T) {
	process := CertificateManagerCalls[0].Process

	certs := []*acm.CertificateSummary{
		{
			CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/abc-123"),
			DomainName:     aws.String("example.com"),
		},
		{
			CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/def-456"),
			DomainName:     aws.String("test.example.com"),
		},
	}

	results := process(certs, nil, false)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].ServiceName != "CertificateManager" {
		t.Errorf("expected ServiceName 'CertificateManager', got '%s'", results[0].ServiceName)
	}
	if results[0].MethodName != "acm:ListCertificates" {
		t.Errorf("expected MethodName 'acm:ListCertificates', got '%s'", results[0].MethodName)
	}
	if results[0].ResourceType != "certificate" {
		t.Errorf("expected ResourceType 'certificate', got '%s'", results[0].ResourceType)
	}
	if results[0].ResourceName != "example.com" {
		t.Errorf("expected ResourceName 'example.com', got '%s'", results[0].ResourceName)
	}
	if results[1].ResourceName != "test.example.com" {
		t.Errorf("expected ResourceName 'test.example.com', got '%s'", results[1].ResourceName)
	}
}

func TestProcess_EmptyResults(t *testing.T) {
	process := CertificateManagerCalls[0].Process

	certs := []*acm.CertificateSummary{}
	results := process(certs, nil, false)

	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestProcess_AccessDenied(t *testing.T) {
	process := CertificateManagerCalls[0].Process

	testErr := fmt.Errorf("AccessDeniedException: User is not authorized")
	results := process(nil, testErr, false)

	if len(results) != 1 {
		t.Fatalf("expected 1 error result, got %d", len(results))
	}
	if results[0].Error == nil {
		t.Error("expected error in result, got nil")
	}
	if results[0].ServiceName != "CertificateManager" {
		t.Errorf("expected ServiceName 'CertificateManager', got '%s'", results[0].ServiceName)
	}
}

func TestProcess_NilFields(t *testing.T) {
	process := CertificateManagerCalls[0].Process

	certs := []*acm.CertificateSummary{
		{
			CertificateArn: nil,
			DomainName:     nil,
		},
	}

	results := process(certs, nil, false)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ResourceName != "" {
		t.Errorf("expected empty ResourceName for nil DomainName, got '%s'", results[0].ResourceName)
	}
}
