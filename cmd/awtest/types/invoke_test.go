package types_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws/session"
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

func TestInvokeV1CallsCallUnchanged(t *testing.T) {
	sess := &session.Session{}
	want := errors.New("plain v1 error")
	var gotSess *session.Session
	svc := types.AWSService{Name: "v1", Call: func(_ context.Context, s *session.Session) (interface{}, error) {
		gotSess = s
		return "out", want
	}}
	// A v2 config in the context must not matter for a v1 service.
	ctx := types.WithAWSConfig(context.Background(), aws.Config{})
	out, err := svc.Invoke(ctx, sess)
	if out != "out" || err != want || gotSess != sess {
		t.Fatalf("out=%v err=%v sess same=%v", out, err, gotSess == sess)
	}
	var wrapped *awsv2.Error
	if errors.As(err, &wrapped) {
		t.Fatal("v1 errors must not be wrapped")
	}
}

func TestInvokeV2UsesContextConfigAndWrapsErrors(t *testing.T) {
	var gotRegion string
	api := &smithy.OperationError{ServiceID: "STS", OperationName: "GetCallerIdentity",
		Err: &smithy.GenericAPIError{Code: "AccessDenied", Message: "nope"}}
	svc := types.AWSService{Name: "v2", CallV2: func(_ context.Context, cfg aws.Config) (interface{}, error) {
		gotRegion = cfg.Region
		return "out", api
	}}
	ctx := types.WithAWSConfig(context.Background(), aws.Config{Region: "ap-south-1"})
	out, err := svc.Invoke(ctx, nil)
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

func TestInvokeV2NilErrorStaysNil(t *testing.T) {
	svc := types.AWSService{Name: "v2", CallV2: func(context.Context, aws.Config) (interface{}, error) { return 1, nil }}
	out, err := svc.Invoke(types.WithAWSConfig(context.Background(), aws.Config{}), nil)
	if err != nil || out != 1 {
		t.Fatalf("out=%v err=%v", out, err)
	}
}

func TestInvokeV2MissingConfigIsAnErrorNotAPanic(t *testing.T) {
	called := false
	svc := types.AWSService{Name: "needs-cfg", CallV2: func(context.Context, aws.Config) (interface{}, error) {
		called = true
		return nil, nil
	}}
	_, err := svc.Invoke(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "needs-cfg") || !strings.Contains(err.Error(), "v2 config") {
		t.Fatalf("err = %v", err)
	}
	if called {
		t.Error("CallV2 must not run without a config")
	}
}

func TestInvokeMisconfiguredService(t *testing.T) {
	if _, err := (types.AWSService{Name: "none"}).Invoke(context.Background(), nil); err == nil {
		t.Error("service with neither Call nor CallV2 must return an error")
	}
	both := types.AWSService{
		Name:   "both",
		Call:   func(context.Context, *session.Session) (interface{}, error) { return nil, nil },
		CallV2: func(context.Context, aws.Config) (interface{}, error) { return nil, nil },
	}
	if _, err := both.Invoke(types.WithAWSConfig(context.Background(), aws.Config{}), nil); err == nil {
		t.Error("service with both Call and CallV2 must return an error")
	}
}
