# Golden output baseline

`replay-sample.json` is the aws-sdk-go **v1** baseline: the full scan produced by
`awtest -replay builtin:sample -quiet -format json`, normalized by
`normalizeGolden` in `cmd/awtest/golden_test.go` (timestamps removed, keys
sorted, 2-space indent, result order preserved).

## How the migration uses it

Every v1 -> v2 service PR must keep `TestGoldenReplaySample` green. The check
is stricter than `awtest diff`: it also compares `details` and result order.
The only intentional v1/v2 difference, the dropped
`\n\tstatus code: N, request id: X` error suffix, is normalized away, so errors
compare as `<code>: <message>`.

## Known v1 quirks

`normalizeGolden` also strips the JSON-protocol v1 error struct dump
(`\n{\n  RespMetadata: {...}\n}`, including its request id), so those errors
compare as `<code>: <message>` too.

These golden entries have error text that is expected to change in the v2
migration. Review them per service; do not normalize them away:

1. CloudFront `cloudfront:ListDistributions`, Route53 `route53:ListHostedZones`
   and `route53:ListHealthChecks` have the empty error `": "` in v1 (REST-XML
   error body with no parsable code).
2. IVS `ivs:ListStreamKeys` has the v1 client-side validation error
   `"InvalidParameter: 1 validation error(s) found.\n- missing required field, ListStreamKeysInput.ChannelArn.\n"`
   (v2 validates differently).

## Regenerating

```
AWTEST_UPDATE_GOLDEN=1 go test ./cmd/awtest -run TestGoldenReplaySample
```

Regenerating requires a reviewed reason (e.g. new fixtures or a new service
in the sample). Never regenerate to make a migration PR pass.
