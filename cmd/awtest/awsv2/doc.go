// Package awsv2 holds the aws-sdk-go v2 scaffolding for the planned v1 -> v2
// migration.
//
// Nothing in cmd/awtest imports this package yet and no service call has been
// migrated. It exists so the configuration, retry, endpoint and
// error-normalisation behaviour can be compiled and tested in isolation before
// services move over.
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
// # Residual risk (out of scope)
//
// AWS_EC2_METADATA_SERVICE_ENDPOINT (IMDS) and credential_process are not AWS
// service endpoints and are neither isolated nor overridden by this package.
package awsv2
