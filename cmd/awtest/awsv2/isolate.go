package awsv2

import (
	"context"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

const ignoreEndpointsEnv = "AWS_IGNORE_CONFIGURED_ENDPOINT_URLS"

// loadEnvMu serialises LoadConfig's temporary environment change.
var loadEnvMu sync.Mutex

// loadDefaultConfigIsolated runs config.LoadDefaultConfig with
// AWS_IGNORE_CONFIGURED_ENDPOINT_URLS=true, restoring the previous state of
// the variable afterwards (also on error paths).
//
// This is needed because the STS client that the SDK builds inside the loader
// (assume-role, web identity) resolves ambient service endpoints itself in
// NewFromConfig; the SDK only honours the ignore switch via the environment
// or shared config, so the process environment is the one place to set it.
func loadDefaultConfigIsolated(ctx context.Context, loadOpts []func(*config.LoadOptions) error) (aws.Config, error) {
	loadEnvMu.Lock()
	defer loadEnvMu.Unlock()

	prev, had := os.LookupEnv(ignoreEndpointsEnv)
	if err := os.Setenv(ignoreEndpointsEnv, "true"); err != nil {
		return aws.Config{}, err
	}
	defer func() {
		if had {
			os.Setenv(ignoreEndpointsEnv, prev)
		} else {
			os.Unsetenv(ignoreEndpointsEnv)
		}
	}()
	return config.LoadDefaultConfig(ctx, loadOpts...)
}

// endpointLoadOpts returns loader options that make the credential chain's
// internal STS client use endpoint (and only it).
//
// config.WithBaseEndpoint cannot do this: the SDK resolves cfg.BaseEndpoint
// after the credential chain (resolveBaseEndpoint runs after
// resolveCredentials), so the internal STS client never sees it. The legacy
// endpoint resolver, in contrast, is resolved before credentials and is
// honoured by NewFromConfig. LoadConfig clears it from the returned config
// (see clearLegacyResolvers), so it only ever affects the chain.
func endpointLoadOpts(endpoint string) []func(*config.LoadOptions) error {
	if endpoint == "" {
		return nil
	}
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, _ ...any) (aws.Endpoint, error) {
		return aws.Endpoint{URL: endpoint}, nil
	})
	//nolint:staticcheck // deprecated, but the only hook resolved before credentials
	return []func(*config.LoadOptions) error{config.WithEndpointResolverWithOptions(resolver)}
}

// clearLegacyResolvers drops the deprecated global endpoint resolvers so the
// returned config carries only BaseEndpoint.
func clearLegacyResolvers(cfg *aws.Config) {
	cfg.EndpointResolver = nil            //nolint:staticcheck
	cfg.EndpointResolverWithOptions = nil //nolint:staticcheck
}
