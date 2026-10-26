package awsv2

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// Every client built from a config carrying an explicit endpoint inherits it
// (any service, any region), and S3 is switched to path-style. These are the
// behaviours the removed v1 utils.ApplyEndpoint tests proved.
func TestNewRegionalClientInheritsEndpoint(t *testing.T) {
	const ep = "http://localhost:4566"
	cfg := aws.Config{Region: "us-west-2"}
	if err := ApplyEndpoint(&cfg, ep); err != nil {
		t.Fatal(err)
	}
	if got := aws.ToString(NewRegionalClient(cfg, "us-west-2", s3.NewFromConfig).Options().BaseEndpoint); got != ep {
		t.Errorf("s3 endpoint = %q", got)
	}
	sq := NewRegionalClient(cfg, "eu-west-1", sqs.NewFromConfig).Options()
	if aws.ToString(sq.BaseEndpoint) != ep || sq.Region != "eu-west-1" {
		t.Errorf("sqs endpoint = %q region = %q", aws.ToString(sq.BaseEndpoint), sq.Region)
	}
	ec := NewRegionalClient(cfg, "ap-south-1", ec2.NewFromConfig).Options()
	if aws.ToString(ec.BaseEndpoint) != ep || ec.Region != "ap-south-1" {
		t.Errorf("ec2 endpoint = %q region = %q", aws.ToString(ec.BaseEndpoint), ec.Region)
	}
}

func TestNewRegionalClientS3PathStyle(t *testing.T) {
	isolateEnv(t)
	ft := &fakeTransport{}
	cfg, err := LoadConfig(context.Background(), Options{
		Region: "us-west-2", AccessKeyID: "AKIAFAKE", SecretAccessKey: "fake",
		Endpoint: "http://localhost:4566", HTTPClient: ft.client(), MaxAttempts: 1, Quiet: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	client := NewRegionalClient(cfg, "us-west-2", s3.NewFromConfig)
	if !client.Options().UsePathStyle {
		t.Error("UsePathStyle not set for an explicit endpoint")
	}
	_, _ = client.GetObject(context.Background(), &s3.GetObjectInput{Bucket: aws.String("my-bucket"), Key: aws.String("key.txt")})
	if len(ft.reqs) != 1 {
		t.Fatalf("requests = %d", len(ft.reqs))
	}
	u := ft.reqs[0].URL
	if u.Host != "localhost:4566" {
		t.Errorf("host = %q, want localhost:4566", u.Host)
	}
	if !strings.HasPrefix(u.Path, "/my-bucket/") {
		t.Errorf("path = %q, want path-style with bucket", u.Path)
	}
}

func TestNewRegionalClientNoEndpointIsNotPathStyle(t *testing.T) {
	if NewRegionalClient(aws.Config{Region: "us-west-2"}, "us-west-2", s3.NewFromConfig).Options().UsePathStyle {
		t.Error("UsePathStyle must stay off without an explicit endpoint")
	}
}
