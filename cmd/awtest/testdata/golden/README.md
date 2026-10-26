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

## Bulk 1 golden (batch, cloudformation, codepipeline, glacier, eventbridge)

`replay-bulk1.json` is the v1 baseline for the first bulk migration batch:
`batch:ListJobs`, `cloudformation:ListStacks`, `codepipeline:ListPipelines`,
`glacier:ListVaults` and `eventbridge:ListEventBuses`. It is produced from the
fictional fixtures in `bulk1/` (fixture dir `testdata/golden/bulk1`) with
`awtest -replay <abs>/testdata/golden/bulk1 -services batch,cloudformation,codepipeline,glacier,eventbridge -quiet -format json`.
Each service has one operation and the golden pins names-only results (empty
`details`, resource type and order). The functional tests in each service
package additionally cover sparse resources and error text.

Migration PRs for these services must keep `TestGoldenReplaySample/bulk1`
green without regenerating it.

## Bulk 2 golden (appsync, secretsmanager, ses, rds, cognitoidentity)

`replay-bulk2.json` is the v1 baseline for the second bulk migration batch:
`appsync:ListGraphqlApis`, `secretsmanager:ListSecrets`, `ses:ListIdentities`,
`rds:DescribeDBInstances` and `cognitoidentity:ListIdentityPools`. It is
produced from the fictional fixtures in `bulk2/` (fixture dir
`testdata/golden/bulk2`) with
`awtest -replay <abs>/testdata/golden/bulk2 -services appsync,secretsmanager,ses,rds,cognitoidentity -quiet -format json`.
The golden pins names-only results (rds also its `availability_zone` detail).
The functional tests in each service package additionally cover sparse
resources, error text and, for the regional loops, the visited regions.

Migration PRs for these services must keep `TestGoldenReplaySample/bulk2`
green without regenerating it.

## Bulk 3 golden (amplify, certificatemanager, stepfunctions, systemsmanager, cognitouserpools)

`replay-bulk3.json` is the v1 baseline for the third bulk migration batch:
`amplify:ListApps` (apps plus their branches), `acm:ListCertificates`,
`states:ListStateMachines`, `ssm:DescribeParameters` and
`cognito-idp:ListUserPools`. It is produced from the fictional fixtures in
`bulk3/` (fixture dir `testdata/golden/bulk3`) with
`awtest -replay <abs>/testdata/golden/bulk3 -services amplify,certificatemanager,stepfunctions,systemsmanager,cognitouserpools -quiet -format json`.
The golden pins resource names and types and, for stepfunctions,
systemsmanager and cognitouserpools, their populated and sparse `details`.
The functional tests in each service package additionally cover pagination,
swallowed per-region errors, error text and the visited regions.

Migration PRs for these services must keep `TestGoldenReplaySample/bulk3`
green without regenerating it.

## Bulk 4 golden (ecs, fargate, efs, eks, elasticache)

`replay-bulk4.json` is the v1 baseline for the fourth bulk migration batch:
`ecs:ListClusters`, `ecs:ListFargateTasks` (fargate; matched by the `ecs` filter), `efs:DescribeFileSystems`, `eks:ListClusters` and
`elasticache:DescribeCacheClusters`. It is produced from the fictional fixtures
in `bulk4/` (fixture dir `testdata/golden/bulk4`) with
`awtest -replay <abs>/testdata/golden/bulk4 -services ecs,efs,eks,elasticache -quiet -format json`.
The functional tests in each service package (including fargate, which shares
the ecs module) additionally cover pagination, swallowed per-region errors,
populated and sparse `details`, error text and the visited regions.

Migration PRs for these services must keep `TestGoldenReplaySample/bulk4`
green without regenerating it.

## Bulk 5 golden (redshift, config, lambda, cloudtrail, elasticbeanstalk)

`replay-bulk5.json` is the v1 baseline for the fifth bulk migration batch:
`redshift:DescribeClusters`, `config:DescribeConfigurationRecorders` (which also
yields the `DescribeConfigRules` results), `lambda:ListFunctions`,
`cloudtrail:DescribeTrails` / `cloudtrail:ListTrails` and
`elasticbeanstalk:DescribeApplications` / `elasticbeanstalk:DescribeEvents`. It
is produced from the fictional fixtures in `bulk5/` (fixture dir
`testdata/golden/bulk5`) with
`awtest -replay <abs>/testdata/golden/bulk5 -services redshift,config,lambda,cloudtrail,elasticbeanstalk -quiet -format json`.
The functional tests in each service package additionally cover Marker
pagination and swallowed per-region errors (redshift), abort-on-first-error
loops (config, lambda), populated and sparse `details`, error text and the
visited regions.

Migration PRs for these services must keep `TestGoldenReplaySample/bulk5`
green without regenerating it.

## Bulk 6 golden (glue, dynamodb, iot, rekognition, transcribe)

`replay-bulk6.json` is the v1 baseline for the sixth bulk migration batch:
`glue:ListWorkflows` / `glue:ListJobs`, `dynamodb:ListTables` /
`dynamodb:ListExports` / `dynamodb:ListBackups`, `iot:ListThings` /
`iot:ListPolicies` / `iot:ListCertificates`, the three rekognition calls and
`transcribe:ListTranscriptionJobs` / `ListVocabularies` / `ListLanguageModels`.
It is produced from the fictional fixtures in `bulk6/` (fixture dir
`testdata/golden/bulk6`) with
`awtest -replay <abs>/testdata/golden/bulk6 -services glue,dynamodb,iot,rekognition,transcribe -quiet -format json`.
The functional tests in each service package additionally cover populated and
sparse `details`, error text, the dynamodb region loop (all regions visited,
abort on the first error) and rekognition's `MaxResults=4096` request.

Migration PRs for these services must keep `TestGoldenReplaySample/bulk6`
green without regenerating it.

## Bulk 7 golden (athena, backup, codebuild, codecommit, codedeploy)

`replay-bulk7.json` is the v1 baseline for the seventh bulk migration batch:
the athena, backup, codebuild, codecommit and codedeploy list calls and their
batch-get follow-ups. It is produced from the fictional fixtures in `bulk7/`
(fixture dir `testdata/golden/bulk7`) with
`awtest -replay <abs>/testdata/golden/bulk7 -services athena,backup,codebuild,codecommit,codedeploy -quiet -format json`.
The functional tests in each service package additionally cover populated and
sparse `details`, error text, pagination, batching and the region loop.

Intentional v1 -> v2 difference (accepted as minimal and unavoidable with the typed v2 API): for the sparse
codedeploy application `bare-app`, `LinkedToGitHub` was `""` in v1 (the
`*bool` was absent) and is `"false"` after the migration, because the v2
`ApplicationInfo.LinkedToGitHub` is a plain `bool` and an absent field cannot
be told apart from `false` through the typed API. That single line of
`replay-bulk7.json` was edited by hand; nothing else in the golden changed.

Migration PRs for these services must keep `TestGoldenReplaySample/bulk7`
green without regenerating it.

## Bulk 8 golden (directconnect, emr, kinesis, guardduty, macie2)

`replay-bulk8.json` is the v1 baseline for the eighth bulk migration batch:
the directconnect, emr, kinesis, guardduty and macie2 list calls and their
describe/get follow-ups. It is produced from the fictional fixtures in `bulk8/`
(fixture dir `testdata/golden/bulk8`) with
`awtest -replay <abs>/testdata/golden/bulk8 -services directconnect,emr,kinesis,guardduty,macie2 -quiet -format json`.
The functional tests in each service package additionally cover populated and
sparse `details`, error text, pagination and the region loop.

Intentional v1 -> v2 difference (accepted as minimal and unavoidable with the
typed v2 API): for the sparse directconnect virtual interface `demo-bare-vif`,
`Vlan` and `Asn` were `""` in v1 (the `*int64` fields were absent) and are
`"0"` after the migration, because the v2 `VirtualInterface.Vlan` and `.Asn`
are plain `int32` values and an absent field cannot be told apart from zero
through the typed API. Those two lines of `replay-bulk8.json` were edited by
hand (`""` -> `"0"`); nothing else in the golden changed. (The gateway
`AmazonSideAsn` and the macie2 counts remain pointers in v2, so their absent
values still print `""`.)

Migration PRs for these services must keep `TestGoldenReplaySample/bulk8`
green without regenerating it.

## Bulk 9 golden (mediaconvert, neptune, opensearch, organizations, sagemaker)

`replay-bulk9.json` is the v1 baseline for the ninth bulk migration batch:
the mediaconvert, neptune, opensearch, organizations and sagemaker list and
describe calls. It is produced from the fictional fixtures in `bulk9/`
(fixture dir `testdata/golden/bulk9`) with
`awtest -replay <abs>/testdata/golden/bulk9 -services mediaconvert,neptune,opensearch,organizations,sagemaker -quiet -format json`.
The functional tests in each service package additionally cover populated and
sparse `details`, error text, pagination and the region loop.

There is no v1 -> v2 difference for this batch: every optional scalar the
services read (the mediaconvert queue job counts, the neptune ports and
booleans, the opensearch encryption flags) is still a pointer in the v2 types,
so absent values keep printing `""`, and `replay-bulk9.json` is unchanged.

Migration PRs for these services must keep `TestGoldenReplaySample/bulk9`
green without regenerating it.

## Bulk 10 golden (securityhub, apigateway, sns, iam)

`replay-bulk10.json` is the v1 baseline for the tenth bulk migration batch:
the securityhub, apigateway, sns and iam calls (iam: ListUsers, ListRoles,
ListGroups, ListPolicies and their follow-ups). It is produced from the
fictional fixtures in `bulk10/` (fixture dir `testdata/golden/bulk10`) with
`awtest -replay <abs>/testdata/golden/bulk10 -services securityhub,apigateway,sns,iam -quiet -format json`.
The functional tests in each service package additionally cover populated and
sparse `details`, error text, pagination and the region loop.

Intentional v1 -> v2 difference (accepted as minimal and unavoidable with the
typed v2 API): for the sparse policy `bare-policy` returned by
`iam:ListPolicies`, `is_attachable` was absent from `details` in v1 (the
`*bool` field was nil, so the key was not set) and is `false` after the
migration, because the v2 `Policy.IsAttachable` is a plain `bool` and an absent
field cannot be told apart from `false` through the typed API. That one line
(`"is_attachable": false,`) was added to `replay-bulk10.json` by hand; nothing
else in the golden changed. (`attachment_count` and the role
`max_session_duration` remain pointers in v2, so they stay absent when absent.)

Migration PRs for these services must keep `TestGoldenReplaySample/bulk10`
green without regenerating it.

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

## Region text normalization

`normalizeGolden` (applied to both the committed golden and the fresh output)
rewrites, inside each result's `error`, only the pattern
`no fixture for <svc>:<Action> (region <name>)` to `(region R)`; the rest of the
text is kept exact. v1 regional services leaked their region into the shared
session, so this region text changes as services migrate, even though nothing
else does. Region scoping is guarded separately by
`TestAllServicesHonorRegionScope`, so normalizing it here loses no coverage.
`TestNormalizeGolden` covers the normalizer.

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
later may shift the same text for services that run after them; this no
longer needs regeneration, because the region text is now normalized (see
"Region text normalization"). The committed goldens keep their current
region text unchanged.
