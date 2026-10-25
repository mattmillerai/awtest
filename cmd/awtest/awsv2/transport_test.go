package awsv2

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func TestLoadConfigTransportOption(t *testing.T) {
	isolateEnv(t)
	ft := &fakeTransport{}
	cfg, err := LoadConfig(context.Background(), Options{
		Region:          testRegion,
		AccessKeyID:     "AKIASTATICKEY",
		SecretAccessKey: "staticsecret",
		Endpoint:        "https://" + explicitHost,
		Transport:       ft,
		HTTPTimeout:     7 * time.Second,
		MaxAttempts:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	client := NewRegionalClient(cfg, testRegion, sts.NewFromConfig)
	if _, err := client.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{}); err != nil {
		t.Fatal(err)
	}
	if len(ft.reqs) != 1 || ft.reqs[0].URL.Host != explicitHost {
		t.Fatalf("requests = %v, want one to %s through the Transport", ft.reqs, explicitHost)
	}
}

func TestLoadConfigHTTPClientWinsOverTransport(t *testing.T) {
	isolateEnv(t)
	used, ignored := &fakeTransport{}, &fakeTransport{}
	cfg, err := LoadConfig(context.Background(), Options{
		Region: testRegion, AccessKeyID: "AKIASTATICKEY", SecretAccessKey: "s",
		HTTPClient: used.client(), Transport: ignored, MaxAttempts: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	client := NewRegionalClient(cfg, testRegion, sts.NewFromConfig)
	if _, err := client.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{}); err != nil {
		t.Fatal(err)
	}
	if len(used.reqs) != 1 || len(ignored.reqs) != 0 {
		t.Fatalf("HTTPClient requests=%d, Transport requests=%d", len(used.reqs), len(ignored.reqs))
	}
}
