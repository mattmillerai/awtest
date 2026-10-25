package types

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
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

// AWSService describes one service scan. During the aws-sdk-go v1 -> v2
// migration exactly one of Call (v1) and CallV2 (v2) is set; callers must run
// the service through Invoke rather than calling either field directly.
type AWSService struct {
	Name string
	// Call is the aws-sdk-go v1 implementation.
	Call func(context.Context, *session.Session) (interface{}, error)
	// CallV2 is the aws-sdk-go-v2 implementation. It receives the aws.Config
	// carried in ctx (see WithAWSConfig) and must build clients only with
	// awsv2.NewClient / awsv2.NewRegionalClient.
	CallV2     func(context.Context, aws.Config) (interface{}, error)
	Process    func(interface{}, error, bool) []ScanResult
	ModuleName string
}

type awsConfigKey struct{}

// WithAWSConfig returns a context carrying the aws-sdk-go-v2 config used by
// services that implement CallV2.
func WithAWSConfig(ctx context.Context, cfg aws.Config) context.Context {
	return context.WithValue(ctx, awsConfigKey{}, cfg)
}

// AWSConfigFrom returns the aws.Config stored by WithAWSConfig.
func AWSConfigFrom(ctx context.Context) (aws.Config, bool) {
	cfg, ok := ctx.Value(awsConfigKey{}).(aws.Config)
	return cfg, ok
}

// Invoke runs the service with whichever SDK it is implemented on. A v2
// service gets the aws.Config from ctx and its error is normalised with
// awsv2.Wrap ("<code>: <message>"; errors.As still reaches the cause); a v1
// service is called with sess exactly as before.
func (s AWSService) Invoke(ctx context.Context, sess *session.Session) (interface{}, error) {
	switch {
	case s.Call != nil && s.CallV2 != nil:
		return nil, fmt.Errorf("service %s sets both Call and CallV2", s.Name)
	case s.CallV2 != nil:
		cfg, ok := AWSConfigFrom(ctx)
		if !ok {
			return nil, errors.New("service " + s.Name + " needs an AWS SDK v2 config in the context (types.WithAWSConfig)")
		}
		out, err := s.CallV2(ctx, cfg)
		return out, awsv2.Wrap(err)
	case s.Call != nil:
		return s.Call(ctx, sess)
	default:
		return nil, fmt.Errorf("service %s has neither Call nor CallV2", s.Name)
	}
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
