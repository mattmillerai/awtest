package types

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
)

const DefaultModuleName = "AWTest"
const InvalidAccessKeyId = "InvalidAccessKeyId"
const InvalidClientTokenId = "InvalidClientTokenId"

var Regions = []string{
	"us-east-1",
	"us-east-2",
	"us-west-1",
	"us-west-2",
	// Add more regions as needed...
}

// ScanResult represents a single result from an AWS service enumeration.
// It captures service-specific information along with metadata about the scan.
type ScanResult struct {
	ServiceName  string                 // e.g., "S3", "EC2", "IAM"
	MethodName   string                 // e.g., "s3:ListBuckets", "ec2:DescribeInstances"
	ResourceType string                 // e.g., "bucket", "instance", "user"
	ResourceName string                 // e.g., "my-bucket", "i-1234567890abcdef0"
	Details      map[string]interface{} // Service-specific details (region, count, metadata)
	Error        error                  // nil if successful, error if failed
	Timestamp    time.Time              // When this result was collected
}

// HasError returns true if the scan result contains an error.
func (sr ScanResult) HasError() bool {
	return sr.Error != nil
}

type AWSService struct {
	Name       string
	Call       func(context.Context, *session.Session) (interface{}, error)
	Process    func(interface{}, error, bool) []ScanResult
	ModuleName string
}

var AwsErrorMessages = map[string]string{
	"UnauthorizedOperation": "You don't have permission to perform this operation.",
	"InvalidAccessKeyId":    "Invalid access key. Aborting scan.",
	"AccessDeniedException": "Access denied to this service.",
	"InvalidClientTokenId":  "The security token included in the request is invalid. Aborting scan.",
}

type InvalidKeyError struct {
	Message string
}

func (e *InvalidKeyError) Error() string {
	return e.Message
}
