// Package awsv2 holds the aws-sdk-go v2 scaffolding for the planned v1 -> v2
// migration.
//
// In phase 0 this package is unused by main: nothing in cmd/awtest imports it
// and no service call has been migrated. It exists so the configuration,
// endpoint and error-normalisation behaviour can be compiled and tested in
// isolation before services move over.
//
// Rules for future callers:
//
//   - Load configuration with LoadConfig. It mirrors the three v1 credential
//     paths (static keys, named profile, default chain) and guarantees that
//     ambient endpoint settings never survive in the returned aws.Config.
//   - Services must build clients only via NewClient. v2 service clients
//     resolve AWS_ENDPOINT_URL_<SERVICE> and shared-config [services]
//     endpoint_url themselves inside NewFromConfig, so only a functional
//     option applied afterwards (which NewClient does) can guarantee that the
//     explicit --endpoint (or no endpoint at all) is what is used.
//   - Report errors with NormalizeError, which drops transport noise such as
//     request IDs, status codes and operation prefixes.
package awsv2
