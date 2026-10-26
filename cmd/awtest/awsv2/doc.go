// Package awsv2 holds the aws-sdk-go-v2 scaffolding every service relies on:
// configuration, retry, endpoint and error-normalisation behaviour. main wires
// it in (see "Migration status" below).
//
// Rules for callers:
//
//   - Load configuration with LoadConfig, once, at startup before any worker
//     goroutines. It implements the three credential paths (static keys, named
//     profile, default chain) and guarantees that ambient endpoint settings
//     never survive in the returned aws.Config.
//   - Services must build clients only via NewClient or NewRegionalClient. v2
//     service clients resolve AWS_ENDPOINT_URL_<SERVICE> and shared-config
//     [services] endpoint_url themselves inside NewFromConfig, so only a
//     functional option applied afterwards (which these helpers do) can
//     guarantee that the explicit --endpoint (or no endpoint at all) is what
//     is used. NewRegionalClient is NewClient for a service that receives the
//     already-validated aws.Config: it keeps that config's explicit endpoint
//     and overrides only the region.
//   - Report errors with NormalizeError, or wrap them with Wrap to get an
//     *Error whose text drops transport noise such as request IDs, status
//     codes and operation prefixes while errors.Is / errors.As still reach the
//     original cause.
//
// # Retries
//
// LoadConfig installs a retryer making DefaultMaxAttempts (4, v1 parity)
// attempts. The SDK never retries the codes in AwtestThrottleCodes, because
// awtest's own scan backoff (cmd/awtest/backoff.go, classifyAWSError in
// cmd/awtest/safe_scan.go) retries those with 1s/2s/4s jittered delays and the
// two layers would otherwise multiply. AwtestThrottleCodes must stay in sync
// with classifyAWSError. Everything else the SDK considers retryable
// (including throttle codes awtest does not handle, such as
// ThrottlingException, 5xx responses and connection errors) is retried by the
// SDK. The same retryer governs the STS client built inside the credential
// chain.
//
// # Credential-chain endpoint isolation
//
// The STS client the SDK builds inside config.LoadDefaultConfig for
// role_arn+source_profile and web identity would normally honour ambient
// AWS_ENDPOINT_URL_STS, AWS_ENDPOINT_URL and shared-config endpoint_url /
// [services] settings. LoadConfig therefore sets
// AWS_IGNORE_CONFIGURED_ENDPOINT_URLS=true in the process environment for the
// duration of the load (serialised by a package mutex, previous value restored
// on every path); this makes the variable process-global while held. An
// explicit endpoint reaches that internal STS client through the loader's
// legacy endpoint resolver, because config.WithBaseEndpoint is resolved after
// the credential chain and so never reaches it; the resolver is cleared from
// the returned config. The result is that the chain talks either to the real
// AWS STS endpoint or only to the explicit endpoint.
//
// # Migration status
//
// Every service is on aws-sdk-go-v2 and aws-sdk-go v1 has been removed from
// the module (TestNoV1SDKImports keeps it out):
//
//   - types.AWSService has one implementation field, CallV2, which takes the
//     aws.Config. Callers never invoke it directly; they use
//     types.AWSService.Invoke(ctx), which takes the config from the context
//     (types.WithAWSConfig) and returns the error normalised with Wrap
//     ("<code>: <message>"). A service without a config in the context gets
//     an error, not a panic.
//   - main builds the one aws.Config with LoadConfig (region, the three
//     credential paths and the validated --endpoint-url; replay mode uses
//     replay.NewConfig, whose Options.Transport confines requests to the
//     replay server) and derives the scan context with types.WithAWSConfig, so
//     every scan path (worker pool, all-regions, safeScan, scanRegion) carries
//     it. discoverRegions (-all-regions) builds its ec2 client with
//     NewRegionalClient from the same config.
//   - Error handling (classifyAWSError, failureDetail, utils.HandleAWSError)
//     reads codes through utils.AWSErrorInfo, which accepts any v2 API error
//     (ErrorCode/ErrorMessage see through *Error). A test cross-checks that
//     classifyAWSError's throttle codes equal AwtestThrottleCodes.
//   - Some service modules are pinned below their newest release because
//     later ones need a newer core than the pinned one or speak a protocol
//     the replay fixtures cannot answer (cloudwatch v1.52.8, cloudwatchlogs
//     v1.89.3, ec2 v1.341.0); see go.mod and testdata/golden/README.md.
//
// # Guards
//
//   - TestNoV2ClientBypass (static, every non-test file) fails on any client
//     built without NewClient/NewRegionalClient, any config loading outside
//     this package and any endpoint or HTTP setting set directly.
//   - TestAllServicesHonorCustomEndpoint and TestAllServicesHonorRegionScope
//     (services package, dynamic) run every service in AllServices through
//     Invoke with a config built the guarded way and hostile ambient
//     AWS_ENDPOINT_URL* settings; they fail if any request leaves the custom
//     endpoint or is signed for a region outside the scope. Each service needs
//     a v2EnvSuffix/v2ServiceIDs entry (v2_env_suffix_test.go), and negative
//     controls prove the harness detects an unguarded client.
//   - The replay tests run every service through Invoke against the replay
//     server and fail on any request that reaches http.DefaultTransport.
//
// # Residual risk (out of scope)
//
// AWS_EC2_METADATA_SERVICE_ENDPOINT (IMDS) and credential_process are not AWS
// service endpoints and are neither isolated nor overridden by this package.
package awsv2
