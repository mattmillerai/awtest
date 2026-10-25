// Package awsv2 holds the aws-sdk-go v2 scaffolding for the planned v1 -> v2
// migration.
//
// main wires this package in (see "Migration status" below) and the pilot
// services, sts, kms and ecr, have been migrated. It holds the configuration, retry, endpoint and
// error-normalisation behaviour that migrated services rely on.
//
// Rules for future callers:
//
//   - Load configuration with LoadConfig, once, at startup before any worker
//     goroutines. It mirrors the three v1 credential paths (static keys, named
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
// aws-sdk-go v1 and v2 coexist while services are migrated one at a time:
//
//   - types.AWSService has two mutually exclusive implementations: Call (v1,
//     takes a *session.Session) and CallV2 (v2, takes an aws.Config). Exactly
//     one is set. Migrated to v2 (CallV2): sts
//     (sts:GetCallerIdentity, whose Process also makes the IAM
//     ListAttachedUserPolicies / ListUserPolicies follow-ups with the v2 config
//     carried in its output), kms (kms:ListKeys, a regional loop over
//     types.RegionsFor) and ecr (ecr:DescribeRepositories, ecr:ListImages and
//     ecr:GetRepositoryPolicy, regional loops with manual NextToken
//     pagination). Every other service is still v1.
//   - Callers never invoke either field directly; they use
//     types.AWSService.Invoke. For a CallV2 service it takes the aws.Config
//     from the context and returns the error normalised with Wrap
//     ("<code>: <message>"); for a v1 service it calls Call unchanged. A v2
//     service without a config in the context gets an error, not a panic.
//   - main builds the v2 aws.Config next to the v1 session with LoadConfig
//     (same region, credential paths and validated --endpoint-url; replay
//     mode uses replay.NewConfig, whose Options.Transport confines requests to
//     the replay server) and derives the scan context with
//     types.WithAWSConfig, so every scan path (worker pool, all-regions,
//     safeScan, scanRegion) carries it.
//   - Error handling shared by both SDKs (classifyAWSError, failureDetail,
//     utils.HandleAWSError) reads codes through utils.AWSErrorInfo, which
//     accepts a v1 awserr.Error or a v2 API error (ErrorCode/ErrorMessage see
//     through *Error). A test cross-checks that classifyAWSError's throttle
//     codes equal AwtestThrottleCodes.
//   - discoverRegions (-all-regions) still uses a v1 ec2 client and the v1
//     session; it has not been migrated.
//   - The dynamic guards in services and replay tests run every service
//     through Invoke with both a v1 session and a v2 config built the guarded
//     way, so a migrated service is covered automatically.
//
// # Residual risk (out of scope)
//
// AWS_EC2_METADATA_SERVICE_ENDPOINT (IMDS) and credential_process are not AWS
// service endpoints and are neither isolated nor overridden by this package.
package awsv2
