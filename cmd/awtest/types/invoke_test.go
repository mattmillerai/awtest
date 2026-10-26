package types_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

func TestAWSConfigContext(t *testing.T) {
	if _, ok := types.AWSConfigFrom(context.Background()); ok {
		t.Fatal("empty context must not carry a config")
	}
	ctx := types.WithAWSConfig(context.Background(), aws.Config{Region: "eu-west-1"})
	cfg, ok := types.AWSConfigFrom(ctx)
	if !ok || cfg.Region != "eu-west-1" {
		t.Fatalf("got %+v, %v", cfg, ok)
	}
}

func TestInvokeUsesContextConfigAndWrapsErrors(t *testing.T) {
	var gotRegion string
	api := &smithy.OperationError{ServiceID: "STS", OperationName: "GetCallerIdentity",
		Err: &smithy.GenericAPIError{Code: "AccessDenied", Message: "nope"}}
	svc := types.AWSService{Name: "v2", CallV2: func(_ context.Context, cfg aws.Config) (interface{}, error) {
		gotRegion = cfg.Region
		return "out", api
	}}
	ctx := types.WithAWSConfig(context.Background(), aws.Config{Region: "ap-south-1"})
	out, err := svc.Invoke(ctx)
	if gotRegion != "ap-south-1" {
		t.Errorf("CallV2 got region %q, want the ctx config's", gotRegion)
	}
	if out != "out" {
		t.Errorf("out = %v", out)
	}
	if err == nil || err.Error() != "AccessDenied: nope" {
		t.Fatalf("err = %v, want normalised %q", err, "AccessDenied: nope")
	}
	var wrapped *awsv2.Error
	if !errors.As(err, &wrapped) || wrapped.Code != "AccessDenied" {
		t.Errorf("error is not an *awsv2.Error with the API code: %#v", err)
	}
	var op *smithy.OperationError
	if !errors.As(err, &op) {
		t.Error("errors.As must still reach the original cause")
	}
}

func TestInvokeNilErrorStaysNil(t *testing.T) {
	svc := types.AWSService{Name: "v2", CallV2: func(context.Context, aws.Config) (interface{}, error) { return 1, nil }}
	out, err := svc.Invoke(types.WithAWSConfig(context.Background(), aws.Config{}))
	if err != nil || out != 1 {
		t.Fatalf("out=%v err=%v", out, err)
	}
}

func TestInvokeMissingConfigIsAnErrorNotAPanic(t *testing.T) {
	called := false
	svc := types.AWSService{Name: "needs-cfg", CallV2: func(context.Context, aws.Config) (interface{}, error) {
		called = true
		return nil, nil
	}}
	_, err := svc.Invoke(context.Background())
	if err == nil || !strings.Contains(err.Error(), "needs-cfg") || !strings.Contains(err.Error(), "v2 config") {
		t.Fatalf("err = %v", err)
	}
	if called {
		t.Error("CallV2 must not run without a config")
	}
}

func TestInvokeMisconfiguredService(t *testing.T) {
	_, err := (types.AWSService{Name: "none"}).Invoke(types.WithAWSConfig(context.Background(), aws.Config{}))
	if err == nil || !strings.Contains(err.Error(), "none") {
		t.Errorf("service without CallV2 must return an error naming it, got %v", err)
	}
	if _, err := (types.AWSService{Name: "none"}).Invoke(context.Background()); err == nil {
		t.Error("service without CallV2 and without a config must return an error")
	}
}
