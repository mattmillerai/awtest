<p align="center">
  <img src="https://readme-typing-svg.demolab.com?font=Fira+Code&weight=700&size=28&pause=1000&color=FF9900&center=true&vCenter=true&width=500&lines=AWTest" alt="AWTest" />
</p>

<p align="center">
<pre align="center">
     ___  _      ________        __
    /   || | /| / /_  __/__ ___ / /_
   / /| || |/ |/ / / / / -_|_-&lt;/ __/
  /_/ |_||__/|__/ /_/  \__/___/\__/
</pre>
</p>

<p align="center">
  <strong>Fast AWS Credential Permission Enumeration for Pentesters & Red Teamers</strong>
</p>

<p align="center">
  <a href="https://github.com/mattmillerai/awtest/releases/latest"><img src="https://img.shields.io/github/v/release/mattmillerai/awtest?color=ff9900&style=flat-square" alt="Latest Release"></a>
  <a href="https://github.com/mattmillerai/awtest/actions/workflows/test.yml"><img src="https://img.shields.io/github/actions/workflow/status/mattmillerai/awtest/test.yml?label=tests&style=flat-square" alt="Tests"></a>
  <a href="https://github.com/mattmillerai/awtest/blob/main/LICENSE"><img src="https://img.shields.io/github/license/mattmillerai/awtest?style=flat-square" alt="License"></a>
  <a href="https://goreportcard.com/report/github.com/mattmillerai/awtest"><img src="https://goreportcard.com/badge/github.com/mattmillerai/awtest" alt="Go Report Card"></a>
  <img src="https://img.shields.io/badge/go-1.26+-00ADD8?style=flat-square&logo=go" alt="Go Version">
</p>

---

## What It Does

AWTest quickly enumerates the permissions of AWS credentials by performing read-only list/describe operations across **69 AWS services** with **143 API calls**. Hand it a set of AWS keys and in seconds you'll know exactly what those credentials can access — S3 buckets, EC2 instances, IAM users, Lambda functions, databases, secrets, and dozens more. Built for pentesters, red teamers, and cloud security assessors who need fast, comprehensive credential reconnaissance.

## Quick Start

```bash
# Install via Homebrew
brew install --cask mattmillerai/tap/awtest

# Scan using your default AWS CLI profile
awtest

# Scan with explicit credentials
awtest --aki=AKIAEXAMPLE --sak=YourSecretAccessKey

# Fast scan with JSON output
awtest --speed=fast --format=json -o results.json
```

## Features

- **Broad AWS Coverage** -- 69 services, 143 API calls covering S3, EC2, IAM, Lambda, EKS, RDS, DynamoDB, GuardDuty, Security Hub, and more
- **Speed Presets** -- `--speed` presets (`safe`, `fast`, `insane`) for OPSEC-aware concurrent scanning, or fine-grained `--concurrency` control
- **Multiple Output Formats** -- Text, JSON, YAML, CSV, and table output with `--output-file` export
- **Service Filtering** -- Include or exclude specific services with `--services` and `--exclude-services`
- **Configurable Timeouts** -- Set scan duration limits with `--timeout`
- **Session Token Support** -- Works with temporary credentials (STS)
- **Signed macOS Binaries** -- Code-signed and Apple-notarized, no Gatekeeper warnings
- **Cross-Platform** -- Pre-built binaries for macOS, Linux, and Windows
- **Supported AWS SDK** -- Built on the AWS SDK for Go v2 (aws-sdk-go v1 reached end of support on 31 Jul 2025)

## Installation

### Homebrew (macOS/Linux)

```sh
brew install --cask mattmillerai/tap/awtest
```

> **macOS users:** Binaries are code-signed and notarized by Apple, so you won't see Gatekeeper warnings.

### Go Install

Requires Go 1.26+:

```sh
go install github.com/mattmillerai/awtest/cmd/awtest@latest
```

### Binary Download

Download pre-built binaries from [GitHub Releases](https://github.com/mattmillerai/awtest/releases):

| Platform | File |
|---|---|
| macOS (Intel) | `awtest_<version>_darwin_amd64.tar.gz` |
| macOS (Apple Silicon) | `awtest_<version>_darwin_arm64.tar.gz` |
| Linux (amd64) | `awtest_<version>_linux_amd64.tar.gz` |
| Linux (arm64) | `awtest_<version>_linux_arm64.tar.gz` |
| Windows | `awtest_<version>_windows_amd64.zip` |

## Usage Examples

```bash
# Scan with temporary credentials (STS)
awtest --aki=ASIAEXAMPLE --sak=YourSecretKey --st=YourSessionToken

# Scan only specific services
awtest --services=s3,ec2,iam,lambda

# Exclude noisy services
awtest --exclude-services=cloudwatch,cloudtrail

# Maximum speed with JSON output
awtest --speed=insane --format=json --output-file=results.json
```

### Example Output

```
[AWTest] [user-id] [info] AKIABCDEFGHIJKLMNO
[AWTest] [account-number] [info] 123456789012
[AWTest] [iam-arn] [info] arn:aws:iam::123456789012:user/exampleUser
[AWTest] [s3:ListBuckets] [info] S3 bucket: example-bucket-1
[AWTest] [s3:ListBuckets] [info] S3 bucket: example-bucket-2
[AWTest] [ec2:DescribeInstances] [info] EC2 instance: [ID: i-0abcdef1234567890, State: running, Type: t3.micro, IP: 10.0.1.5]
[AWTest] [iam:ListUsers] [info] IAM user: exampleUser1
[AWTest] [lambda:ListFunctions] [info] Lambda function: myFunction
[AWTest] [eks:ListClusters] [info] Found EKS Cluster: production (Status: ACTIVE, Version: 1.28)
[AWTest] [rds:DescribeDBInstances] [info] Found RDS instance: mydb (us-east-1a)
[AWTest] [appsync:ListGraphqlApis] [info] Error: Access denied to this service.
========================================
Scan Summary
========================================
Duration:           12s
Total Services:     69
Accessible:         8
Access Denied:      61
Resources Found:    11
========================================
```

## Flags

| Flag | Description | Default |
|---|---|---|
| `--aki`, `--access-key-id` | AWS Access Key ID | |
| `--sak`, `--secret-access-key` | AWS Secret Access Key | |
| `--st`, `--session-token` | AWS Session Token | |
| `--region` | AWS Region | `us-west-2` |
| `--format` | Output format: `text`, `json`, `yaml`, `csv`, `table` | `text` |
| `-o`, `--output-file` | Write output to file | |
| `--services` | Include only specific services (comma-separated) | all |
| `--exclude-services` | Exclude specific services (comma-separated) | none |
| `--endpoint-url` | Custom AWS-compatible endpoint URL used for every service (e.g. `http://localhost:4566`); also read from `AWS_ENDPOINT_URL` | none |
| `--replay` | Offline replay mode: serve canned responses from a fixture directory (or `builtin:sample`) on `127.0.0.1` and scan that; no AWS access (see [Replay mode](#replay-mode-offline-demos-and-labs)) | none |
| `--all-regions` | Scan every enabled AWS region instead of the default four US regions (opt-in; many more API calls) | `false` |
| `--timeout` | Maximum scan duration (e.g., `5m`, `300s`) | `5m` |
| `--speed` | Speed preset: `safe`, `fast`, `insane` (controls scan parallelism) | `safe` |
| `--concurrency` | Number of concurrent service scans (overrides speed preset when specified) | `1` |
| `--quiet` | Suppress info messages, show only findings | `false` |
| `--debug` | Enable debug output | `false` |
| `--check-update` | Check if a newer version is available | `false` |
| `-v`, `--version` | Print version and build info | |

Subcommand: `awtest diff [-format text|json] <before.json> <after.json>` compares two JSON scan outputs offline (see [Comparing scans](#comparing-scans-awtest-diff)).

### Using with LocalStack or mocks

Point awtest at any AWS-compatible endpoint instead of real AWS:

```bash
awtest -endpoint-url http://localhost:4566 -aki test -sak test
```

- The endpoint applies to all services, including regional loops. Every region in the loop hits the same endpoint, so you may see duplicate results.
- S3 switches to path-style addressing automatically.
- For `https` endpoints with self-signed certificates, set `AWS_CA_BUNDLE` to your CA file.
- A notice is printed to stderr unless `-quiet` is set; results do not come from real AWS.
- The scheme must be `http` or `https`; anything else exits with code 2.
- The flag takes precedence over the `AWS_ENDPOINT_URL` environment variable.

### Replay mode (offline demos and labs)

`-replay` runs the normal scan against canned API responses instead of AWS. awtest starts a small HTTP server in-process, serves your fixture files from it, and points every service at it (it builds on the `-endpoint-url` mechanism). Nothing is sent to AWS, so it is useful for demos, training, screenshots, and testing report tooling.

```bash
awtest -replay builtin:sample -services sts,s3,iam,ec2
awtest -replay ./my-fixtures -format json -quiet > scan.json
```

`builtin:sample` uses fixtures embedded in the binary (source: `cmd/awtest/replay/sample`). All sample data is **fictional**: account `123456789012`, `example-*` names, and documentation-range IPs.

**Fixture layout.** Only regular `.xml` and `.json` files are read (each up to 5 MB, all loaded at startup). `<service>` is the SigV4 signing name (`sts`, `iam`, `ec2`, `s3`, `lambda`, `dynamodb`, `monitoring` for CloudWatch, ...), `<region>` is the signing region and `<Action>` is the API action name:

```
<service>/<Action>.xml                        any region, any resource
<service>/<region>/<Action>.xml               one region
<service>/<Action>/<resource>.xml             one resource (bucket, user, role, ...)
<service>/<region>/<Action>/<resource>.xml
```

- `.xml` files are served as `text/xml` (query/S3 protocols), `.json` as `application/x-amz-json-1.1`. Put the raw wire-format response in the file, e.g. the XML an `iam:ListUsers` call returns. A duplicate (same service, region, action and resource) is an error that names both files.
- **Status suffix:** `<name>.<3-digit-status>.<ext>` sets the HTTP status (default `200`), e.g. `s3/GetBucketPolicy/example-logs.404.xml` containing a `NoSuchBucketPolicy` error.
- **Lookup order:** region + resource, region, resource, generic. Region-specific files win over generic ones. This makes a generic empty response a useful fallback: per-region scans stop at the first error, so give `ec2/DescribeInstances.xml` an empty reservation list and put real instances only in `ec2/us-east-1/DescribeInstances.xml`; the other regions then return "no instances" instead of an error.
- **Operation names:** JSON services use the `X-Amz-Target` action; query services (IAM, STS, EC2, ...) the `Action` parameter; S3 uses `ListBuckets`, `GetBucketLocation`, `GetBucketPolicy`, `GetBucketPolicyStatus`, `GetBucketAcl`, `GetPublicAccessBlock`, `ListObjectsV2` with the bucket as the resource; other REST calls are named `METHOD_path` (e.g. `lambda/GET_2015-03-31_functions.json`). For query and JSON services the resource is the `UserName`, `RoleName`, `GroupName`, `PolicyName`, `InstanceId`, `TableName`, `FunctionName` or `Name` parameter when present.
- **Unknown calls:** a request with no fixture gets an AccessDenied-style error in the right wire format for its protocol, so scans degrade the way a restricted account would. A single stderr line lists the calls that had no fixture (unless `-quiet`).

**Safety.**
- The server listens on `127.0.0.1` only (random port) and stops when awtest exits.
- Credentials are fake (`AKIAREPLAYFAKE000000`). Credential flags, `AWS_*` credential variables, `AWS_PROFILE` and `~/.aws` are ignored entirely (a note is printed if you supplied any), and `AWS_ENDPOINT_URL` is ignored. Combining `-replay` with `-endpoint-url` is a usage error (exit 2), as is a missing or invalid fixture directory.
- The HTTP client can only connect to the replay server and never uses a proxy, so nothing leaves the machine even if some code path ignored the endpoint.
- The background update check is skipped.
- A stderr notice states that results do not come from real AWS (suppressed by `-quiet`).

`-all-regions` works too: with no `ec2:DescribeRegions` fixture, awtest uses its built-in region list. Replay output is ordinary `-format json` output, so [`awtest diff`](#comparing-scans-awtest-diff) works on it, e.g. to check how a change to your fixtures alters the findings.

Recording fixtures from a real account is a possible future follow-up and is not part of this feature.

### Scanning all regions

By default regional services are scanned in four US regions (`us-east-1`, `us-east-2`, `us-west-1`, `us-west-2`). Use `-all-regions` to scan every enabled region:

```bash
awtest -all-regions -speed fast -timeout 20m
```

- Regions are discovered with one read-only `ec2:DescribeRegions` call, which returns only the regions enabled for the account. If that call is denied or unavailable, awtest falls back to a built-in list of the commercial regions enabled by default; opt-in regions (e.g. `af-south-1`, `ap-east-1`) are not included in the fallback.
- The `--region` value is scanned first.
- Global services (IAM, S3 bucket listing, STS, ...) run once, not once per region.
- A failure in one region does not discard results from the others. Failures are summarized in a single stderr line (opt-in region not enabled or service unavailable); use `-debug` for per-region details.
- Repeated identical per-region error lines (same call and error code, e.g. access denied) are collapsed to the first occurrence and counted in that summary line; use `-debug` to see all of them.
- Warning: this multiplies API calls roughly by the number of regions, generates far more CloudTrail activity (OPSEC) and takes much longer. Consider `-speed fast` and a larger `-timeout`.
- Composes with `-endpoint-url`: every region hits the same endpoint.
- GovCloud and China regions are not included.

## Speed Presets & OPSEC

AWTest provides named speed presets that control scan parallelism. Choose the right preset based on your OPSEC requirements:

| Preset | Concurrency | CloudTrail Profile | Use Case |
|---|---|---|---|
| `safe` | 1 worker | Minimal footprint -- sequential calls resemble normal console usage | Stealth engagements, red team ops, production infrastructure |
| `fast` | 5 workers | Moderate density -- more events in a shorter window, within normal operational patterns | Time-sensitive pentests where speed matters more than stealth |
| `insane` | 20 workers | Dense burst -- all 69 services hammered simultaneously, visible API call spike | Lab environments, time-critical bug bounty, OPSEC not a concern |

**Default behavior:** Running `awtest` with no `--speed` flag defaults to `safe` (sequential scanning, identical to Phase 1 behavior).

**Power-user override:** Use `--concurrency=N` to set an exact worker count, overriding the speed preset's mapping:

```bash
# Use fast preset (5 workers)
awtest --speed=fast

# Override: use fast preset label but with 10 workers
awtest --speed=fast --concurrency=10
```

## Output Formats

AWTest supports five output formats via the `--format` flag:

| Format | Best For | Example |
|---|---|---|
| `text` | Real-time terminal scanning (default) | `[AWTest] [s3:ListBuckets] [info] S3 bucket: my-bucket` |
| `json` | SIEM integration, automated pipelines, programmatic parsing | `{"service":"S3","method":"s3:ListBuckets","resource":"my-bucket"}` |
| `yaml` | Readable structured reports, documentation | `service: S3` &#124; `method: s3:ListBuckets` |
| `csv` | Spreadsheet analysis, data import, quick pivoting | `S3,s3:ListBuckets,bucket,my-bucket` |
| `table` | Structured terminal viewing, sharing in tickets | ASCII table with aligned columns |

```bash
# Save JSON results for SIEM ingestion
awtest --format=json --output-file=results.json

# Generate YAML report
awtest --format=yaml --output-file=report.yaml

# Export CSV for spreadsheet analysis
awtest --format=csv --output-file=findings.csv

# View results as a formatted table
awtest --format=table
```

### Comparing scans (`awtest diff`)

`awtest diff` compares two JSON scan outputs offline -- no AWS credentials or network access needed. Generate the two files with `-format json`, then diff them:

```bash
awtest -format json -o before.json
# ...time passes, permissions change...
awtest -format json -o after.json

awtest diff before.json after.json
```

```text
Comparing before.json -> after.json

Added resources (1):
  + sqs:ListQueues  orders-queue (queue)

Removed resources (1):
  - s3:ListBuckets  old-bucket (bucket)

Changed calls (2):
  ~ ec2:DescribeInstances  ok -> error (newly denied)
  ~ iam:ListUsers  error -> ok (newly allowed)

Summary: 1 added, 1 removed, 2 changed
```

Use `-format json` (`awtest diff -format json before.json after.json`) for machine-readable output with `identical`, `summary`, `added`, `removed` and `changedCalls` fields. Both the default envelope output and the bare-array output produced with `-quiet` are accepted.

What it compares:

- Resources are identified by service, method and resource name; resources only in the second scan are *added*, only in the first are *removed*.
- Calls that returned errors (access denied, etc.) are tracked per method: a call that went from error to success is reported as *newly allowed*, and the reverse as *newly denied*.

What it ignores:

- Timestamps and `details` contents, and the resource type (for identity).
- Calls that succeed with zero resources leave no trace in the output, so a call that was allowed but found nothing looks the same as a call that was never run.

| Exit code | Meaning |
|---|---|
| `0` | No differences |
| `1` | Differences found |
| `2` | Usage or input error (wrong arguments, unreadable or non-awtest JSON file) |

## Real-World Use Cases

### Penetration Testing

During a fintech engagement, you discover AWS keys in a public GitHub repo. Run awtest to quickly enumerate what the credentials can access:

```bash
awtest --aki=AKIAEXAMPLE --sak=YourSecretKey --speed=insane --format=json --output-file=findings.json
```

In seconds, awtest reveals an RDS instance with customer PII, S3 buckets with financial documents, and active Lambda functions -- a critical finding that would have taken hours to uncover manually.

### Bug Bounty

You find hardcoded credentials in client-side JavaScript. Use awtest to demonstrate the full impact:

```bash
awtest --aki=AKIAEXAMPLE --sak=YourSecretKey --speed=insane --services=s3,secretsmanager,iam,lambda
```

AWTest reveals S3 buckets with user uploads and Secrets Manager entries, transforming a medium-severity credential exposure into a critical-severity finding with concrete evidence.

### Incident Response

2 AM alert: credentials were committed to a public repo. Assess the blast radius before deciding whether to escalate -- use `--speed=safe` for a controlled scan with minimal CloudTrail footprint:

```bash
awtest --aki=AKIAEXAMPLE --sak=YourSecretKey --speed=safe --timeout=2m
```

AWTest shows the credentials only have access to CloudWatch logs and one S3 log bucket -- no customer data exposed, no emergency escalation needed.

## Supported AWS Services (69 services, 143 API calls)

<details>
<summary>Click to expand full service list (69 services, 143 API calls)</summary>

### Compute & Containers

| Service | API Calls |
|---|---|
| Batch | ListJobs |
| EC2 | DescribeInstances |
| ECS | ListClusters, ListServices, ListTaskDefinitions |
| EKS | ListClusters, ListNodegroups |
| Elastic Beanstalk | DescribeApplications, DescribeEvents |
| ELBv2 | DescribeLoadBalancers, DescribeTargetGroups, DescribeListeners |
| EMR | ListClusters, ListInstanceGroups, ListSecurityConfigurations |
| Fargate | ListFargateTasks |
| Lambda | ListFunctions |

### Databases

| Service | API Calls |
|---|---|
| DynamoDB | ListTables, ListBackups, ListExports |
| ElastiCache | DescribeCacheClusters |
| Neptune | DescribeDBClusters, DescribeDBInstances, DescribeDBClusterParameterGroups |
| OpenSearch | ListDomains, DescribeDomainAccessPolicies, DescribeDomainEncryption |
| RDS | DescribeDBInstances |
| Redshift | DescribeClusters |

### Security & Identity

| Service | API Calls |
|---|---|
| Access Analyzer | ListAnalyzers, ListFindingsV2 |
| Account | ListRegions, GetContactInformation |
| Certificate Manager (ACM) | ListCertificates |
| Cognito Identity | ListIdentityPools |
| Cognito User Pools | ListUserPools |
| ECR | DescribeRepositories, ListImages, GetRepositoryPolicy |
| GuardDuty | ListDetectors, GetFindings, ListFilters |
| IAM | ListUsers, ListRoles, ListGroups, ListPolicies, ListInstanceProfiles, ListOpenIDConnectProviders, ListSAMLProviders, GetAccountSummary, GetAccountPasswordPolicy, ListMFADevices |
| KMS | ListKeys |
| Macie | ListClassificationJobs, ListFindings, DescribeBuckets |
| Organizations | ListAccounts, ListOrganizationalUnits, ListPolicies |
| Secrets Manager | ListSecrets |
| Security Hub | GetEnabledStandards, GetFindings, ListEnabledProductsForImport |
| STS | GetCallerIdentity, GetAccessKeyInfo |
| WAF | ListWebACLs |
| WAFv2 | ListWebACLs, ListIPSets |

### Storage

| Service | API Calls |
|---|---|
| Backup | ListBackupVaults, ListBackupPlans, ListRecoveryPointsByBackupVault, GetBackupVaultAccessPolicy |
| EFS | DescribeFileSystems |
| Glacier | ListVaults |
| S3 | ListBuckets, GetBucketPolicy (policy, policy status, ACL, public access block, encryption, versioning) |

### Networking

| Service | API Calls |
|---|---|
| API Gateway | RestApis, GetApiKeys, GetDomainNames |
| API Gateway V2 | GetApis, GetDomainNames |
| CloudFront | ListDistributions |
| Direct Connect | DescribeConnections, DescribeVirtualInterfaces, DescribeDirectConnectGateways |
| Route53 | ListHostedZones, ListHealthChecks |
| VPC | DescribeVpcs |

### Management & Monitoring

| Service | API Calls |
|---|---|
| CloudFormation | ListStacks |
| CloudTrail | DescribeTrails, ListTrails |
| CloudWatch | DescribeAlarms |
| CloudWatch Logs | DescribeLogGroupsAndStreams, ListMetrics |
| Config | DescribeConfigurationRecorders |
| Resource Groups Tagging API | GetResources (capped at 500 resources), GetTagKeys |
| Systems Manager (SSM) | DescribeParameters |

### Application Services

| Service | API Calls |
|---|---|
| EventBridge | ListEventBuses |
| SES | ListIdentities |
| SNS | ListTopics |
| SQS | ListQueues |
| Step Functions | ListStateMachines |

### Developer Tools

| Service | API Calls |
|---|---|
| Amplify | ListApps |
| AppSync | ListGraphqlApis |
| CodeBuild | ListProjects, ListProjectEnvironmentVariables, ListBuilds |
| CodeCommit | ListRepositories, ListBranches |
| CodeDeploy | ListApplications, ListDeploymentGroups, ListDeploymentConfigs |
| CodePipeline | ListPipelines |
| Glue | ListJobs, ListWorkflows |

### Analytics

| Service | API Calls |
|---|---|
| Athena | ListWorkGroups, ListNamedQueries, ListQueryExecutions |
| Kinesis | ListStreams, ListShards, ListStreamConsumers |

### Media & ML

| Service | API Calls |
|---|---|
| IVS | ListChannels, ListStreams, ListStreamKeys |
| IVS Chat | ListRooms |
| IVS Realtime | ListStages |
| MediaConvert | ListQueues, ListJobs, ListPresets |
| Rekognition | ListCollections, DescribeProjects, ListStreamProcessors |
| SageMaker | ListNotebookInstances, ListEndpoints, ListModels, ListTrainingJobs |
| Transcribe | ListTranscriptionJobs, ListLanguageModels, ListVocabularies |

### IoT

| Service | API Calls |
|---|---|
| IoT | ListThings, ListCertificates, ListPolicies |

</details>

### IAM role / policy and S3 bucket policy coverage

- **IAM** (`iam:ListRoles`, `iam:ListGroups`, `iam:ListPolicies`): roles with ARN, path, creation date, session duration, description and a trust-policy summary (`trusted_principals`, `trust_findings`); groups; and customer managed policies that are attached to something. For the first 100 roles and the first 100 groups, the attached managed policies and inline policy names are also listed (`iam:ListAttached{Role,Group}Policies`, `iam:List{Role,Group}Policies`).
- **S3** (`s3:GetBucketPolicy`): for each bucket, the bucket region is resolved (`GetBucketLocation`) and then `GetBucketPolicy`, `GetBucketPolicyStatus`, `GetBucketAcl`, `GetPublicAccessBlock`, `GetBucketEncryption` and `GetBucketVersioning` are called in that region. That is 7 extra API calls per bucket.
- **Findings are factual statements**, not ratings, e.g. `wildcard principal (*) in Allow statement`, `cross-account principal: 222222222222 (role account is 111111111111)` or `grant to AllUsers (...): READ`. Only `Allow` statements are considered.
- **Bounds:** at most 1000 roles, 1000 groups, 1000 policies and 500 buckets are examined; per-role and per-group follow-ups cover the first 100 only. When a bound is hit a result with resource type `note` and `truncated: true` is recorded. If a follow-up call is denied for one role or group it is skipped for the rest.
- **Not collected:** AWS-managed policies (hundreds of entries, not account-specific), policy documents (`GetPolicyVersion`), and `RoleLastUsed` (not returned by `ListRoles`; `GetRole` is not called per role).
- **Statuses, not errors:** `denied` (access denied) and `none` / `no_policy` (no bucket policy or public access block) are reported as a `status` in the result details, not as errors. As a consequence, `awtest diff` shows these buckets as resources, and a permission change on these calls only appears as a status change in the details. Any other error (for example `NoSuchBucket` or `PermanentRedirect`) is reported as an error.
- Both are global services and run once per scan, regardless of `-all-regions`.

## Contributing

Contributions are welcome! The most common contribution is **adding support for a new AWS service**. A complete service implementation template is provided at [`cmd/awtest/services/_template/`](cmd/awtest/services/_template/) with step-by-step instructions and an annotated reference implementation. Services are written against [aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2): a `CallV2` function that builds its clients with `awsv2.NewRegionalClient(cfg, region, pkg.NewFromConfig)`, and a `Process` function that turns the response into `ScanResult`s.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full guide, including:

- Development workflow and prerequisites
- 10-step guide to adding a new AWS service
- Code standards and naming conventions
- Testing standards with table-driven test examples
- 16-item service validation checklist
- PR process and review expectations

## Support the Project

If you find this project helpful, please consider supporting its development:

[![Buy me a coffee](https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png)](https://www.buymeacoffee.com/yOd1JU9MQe)

## License

This project is licensed under the [MIT License](LICENSE).
