package awsv2

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func TestNewRegionalClient(t *testing.T) {
	amb := "https://" + ambientHost
	cases := []struct {
		name     string
		explicit string
		region   string
		wantHost string
		wantSign string // region expected in the SigV4 credential scope
	}{
		{"no endpoint, region override", "", "eu-west-1", "sts.eu-west-1.amazonaws.com", "eu-west-1"},
		{"no endpoint, empty region keeps config region", "", "", "sts." + testRegion + ".amazonaws.com", testRegion},
		{"explicit endpoint preserved, region override", "https://" + explicitHost, "ap-south-1", explicitHost, "ap-south-1"},
		{"explicit endpoint preserved, empty region", "https://" + explicitHost, "", explicitHost, testRegion},
	}
	for _, tc := range cases {
		for _, ambientKind := range []string{"AWS_ENDPOINT_URL_STS", "AWS_ENDPOINT_URL"} {
			t.Run(tc.name+"/"+ambientKind, func(t *testing.T) {
				isolateEnv(t)
				t.Setenv(ambientKind, amb)
				ft := &fakeTransport{}
				cfg, err := LoadConfig(context.Background(), Options{
					Region: testRegion, AccessKeyID: "AKIAFAKE", SecretAccessKey: "fake",
					Endpoint: tc.explicit, HTTPClient: ft.client(),
				})
				if err != nil {
					t.Fatal(err)
				}
				// The service gets the validated config; the client must keep its endpoint.
				client := NewRegionalClient(cfg, tc.region, sts.NewFromConfig)
				if _, err := client.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{}); err != nil {
					t.Fatal(err)
				}
				if len(ft.reqs) != 1 {
					t.Fatalf("requests = %d", len(ft.reqs))
				}
				r := ft.reqs[0]
				if r.URL.Host != tc.wantHost {
					t.Errorf("host = %q, want %q", r.URL.Host, tc.wantHost)
				}
				if auth := r.Header.Get("Authorization"); !strings.Contains(auth, "/"+tc.wantSign+"/sts/aws4_request") {
					t.Errorf("signing region: want %q in %q", tc.wantSign, auth)
				}
			})
		}
	}
}

// The ambient endpoint must be ignored even when the config did not come from
// LoadConfig (no isolation recorded in its config sources).
func TestNewRegionalClientIgnoresAmbientOnPlainConfig(t *testing.T) {
	isolateEnv(t)
	t.Setenv("AWS_ENDPOINT_URL_STS", "https://"+ambientHost)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAENV")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "s")
	ft := &fakeTransport{}
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(testRegion), config.WithHTTPClient(ft.client()))
	if err != nil {
		t.Fatal(err)
	}
	client := NewRegionalClient(cfg, "eu-central-1", sts.NewFromConfig)
	if _, err := client.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{}); err != nil {
		t.Fatal(err)
	}
	if len(ft.reqs) != 1 || ft.reqs[0].URL.Host != "sts.eu-central-1.amazonaws.com" {
		t.Fatalf("requests: %v", ft.reqs)
	}
}

// NewRegionalClient must not mutate the shared config.
func TestNewRegionalClientDoesNotMutateConfig(t *testing.T) {
	isolateEnv(t)
	cfg, err := LoadConfig(context.Background(), Options{Region: testRegion, AccessKeyID: "AKIAX", SecretAccessKey: "s", Endpoint: "https://" + explicitHost})
	if err != nil {
		t.Fatal(err)
	}
	_ = NewRegionalClient(cfg, "eu-west-1", sts.NewFromConfig)
	if cfg.Region != testRegion || cfg.BaseEndpoint == nil || *cfg.BaseEndpoint != "https://"+explicitHost {
		t.Fatalf("cfg mutated: region=%q endpoint=%v", cfg.Region, cfg.BaseEndpoint)
	}
}
