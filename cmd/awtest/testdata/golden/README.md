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

## Pilot golden (ecr, kms)

`replay-pilot.json` is a second v1 baseline with **real, non-error data** for
the pilot services `ecr` (`DescribeRepositories`, `ListImages`,
`GetRepositoryPolicy`) and `kms` (`ListKeys`). It is produced from the
fictional fixtures in `pilot/` with
`awtest -replay <abs>/testdata/golden/pilot -services ecr,kms -exclude-services secretsmanager -quiet -format json`
(`-services` matches substrings and `ecr` is inside `secretsmanager`, hence the
exclude). Data is returned for `us-west-2` only; other regions get empty
generic fallbacks. Fixtures are generic (no per-resource files), so both
repositories return the same images, and the single policy fixture is returned
for each repository.

It uses its own fixture dir because `builtin:sample` must stay byte-stable:
adding ecr/kms data there would change `replay-sample.json`.

Migration PRs for ecr and kms must keep this golden green
(`TestGoldenReplaySample/pilot`) without regenerating it.

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

Add `/sample` or `/pilot` to the `-run` pattern to regenerate just one golden.

Regenerating requires a reviewed reason (e.g. new fixtures or a new service
in the sample). Never regenerate to make a migration PR pass.

## Intentional baseline change: leaked session region (ecr migration)

v1 regional-loop services set `sess.Config.Region` on the **shared** session.
When the v1 `ecr` service aborted in its first region (us-east-1) under
`builtin:sample`, later v1 services that build their client from the session
without setting a region inherited us-east-1. The migrated v2 `ecr` (like
`kms` before it) never touches shared state, so those services now use the
session's own region (us-west-2). Only the region text inside 13 replay
"no fixture for ... (region X)" errors changed (us-east-1 -> us-west-2);
resource names, details and order are identical. `replay-sample.json` was
regenerated for exactly these lines. Further regional v1 services migrated
later may shift the same text for services that run after them.
