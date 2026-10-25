package services

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

// Test-only aws-sdk-go-v2 services. There are no migrated services yet, so
// these stand in for one to prove that the dynamic guards (endpoint and region
// scope) work for CallV2 services and detect a deliberately unguarded one.

// syntheticV2Service is a well-behaved v2 service: one sts:GetCallerIdentity
// per region of the scan scope, clients built only with NewRegionalClient.
func syntheticV2Service() types.AWSService {
	return types.AWSService{
		Name:    "sts:SyntheticV2",
		CallV2:  syntheticV2Call(awsv2NewRegionalSTS),
		Process: func(interface{}, error, bool) []types.ScanResult { return nil },
	}
}

// unguardedSyntheticV2Service builds its client with a plain sts.NewFromConfig,
// which lets AWS_ENDPOINT_URL_STS override the configured endpoint.
func unguardedSyntheticV2Service() types.AWSService {
	return types.AWSService{
		Name:    "sts:UnguardedSyntheticV2",
		CallV2:  syntheticV2Call(func(cfg aws.Config, region string) *sts.Client { cfg.Region = region; return sts.NewFromConfig(cfg) }),
		Process: func(interface{}, error, bool) []types.ScanResult { return nil },
	}
}

func awsv2NewRegionalSTS(cfg aws.Config, region string) *sts.Client {
	return awsv2.NewRegionalClient(cfg, region, sts.NewFromConfig)
}

func syntheticV2Call(newClient func(aws.Config, string) *sts.Client) func(context.Context, aws.Config) (interface{}, error) {
	return func(ctx context.Context, cfg aws.Config) (interface{}, error) {
		var lastErr error
		for _, region := range types.RegionsFor(ctx) {
			client := newClient(cfg, region)
			if _, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}); err != nil {
				lastErr = err
			}
		}
		return nil, lastErr
	}
}

// guardTarget is a service to run through the dynamic guards. The v1 session
// and the v2 config are both prepared for every target; Invoke picks the one
// the service implements, so a service that migrates is covered automatically.
type guardTarget struct {
	svc types.AWSService
	// envSuffixes are the <SVC> parts of AWS_ENDPOINT_URL_<SVC> to point at an
	// unreachable host for this service (v2 clients resolve it themselves).
	// Empty derives it from the service name ("sts:GetCallerIdentity" -> STS).
	envSuffixes []string
}

func (g guardTarget) ambientSuffixes() []string {
	if len(g.envSuffixes) > 0 {
		return g.envSuffixes
	}
	name := g.svc.Name
	if i := strings.IndexByte(name, ':'); i >= 0 {
		name = name[:i]
	}
	return []string{strings.ToUpper(strings.NewReplacer("-", "_", " ", "_").Replace(name))}
}

// guardTargets returns every registered service plus the guarded synthetic v2
// service.
func guardTargets() []guardTarget {
	var out []guardTarget
	for _, svc := range AllServices() {
		tg := guardTarget{svc: svc}
		if svc.Name == "sts:GetCallerIdentity" {
			// Process makes IAM follow-up calls with the same config.
			tg.envSuffixes = []string{"STS", "IAM"}
		}
		out = append(out, tg)
	}
	return append(out, guardTarget{svc: syntheticV2Service(), envSuffixes: []string{"STS"}})
}

// setAmbientEndpoints points AWS_ENDPOINT_URL and AWS_ENDPOINT_URL_<SVC> of
// every v2 target at an unreachable host, as a hostile or careless
// environment would.
func setAmbientEndpoints(t *testing.T, targets []guardTarget) {
	t.Helper()
	const unreachable = "http://ambient-endpoint.invalid:9"
	t.Setenv("AWS_ENDPOINT_URL", unreachable)
	for _, tg := range targets {
		if tg.svc.CallV2 == nil {
			continue
		}
		for _, s := range tg.ambientSuffixes() {
			t.Setenv("AWS_ENDPOINT_URL_"+s, unreachable)
		}
	}
}

// guardedV2Config builds the v2 config the guarded way: awsv2.LoadConfig with
// the guard's transport injected as the HTTP client (v2's default HTTP client
// does not use http.DefaultTransport), fake static credentials and no retries.
//
// Call it after setAmbientEndpoints. A config from LoadConfig already records
// "ignore configured endpoints" in its ConfigSources, which would make even a
// plain sts.NewFromConfig immune to ambient AWS_ENDPOINT_URL_<SVC>. Services
// must not depend on that, so the guard swaps in the ConfigSources of the
// SDK's own loader (which honours the ambient variables): only
// awsv2.NewClient/NewRegionalClient then keep a client on the endpoint.
func guardedV2Config(t *testing.T, rt http.RoundTripper, endpoint string) aws.Config {
	t.Helper()
	hostile, err := config.LoadDefaultConfig(context.Background(), config.WithRegion("us-west-2"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := awsv2.LoadConfig(context.Background(), awsv2.Options{
		Region:          "us-west-2",
		AccessKeyID:     "AKIAFAKEFAKEFAKE",
		SecretAccessKey: "fake",
		Endpoint:        endpoint,
		HTTPClient:      &http.Client{Transport: rt},
		MaxAttempts:     1,
		Quiet:           true,
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigSources = hostile.ConfigSources
	return cfg
}
