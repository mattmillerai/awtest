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
  <strong>AWS Credential Permission Scanner for Security Assessments</strong>
</p>

<p align="center">
  <a href="https://github.com/MillerMedia/awtest/releases/latest"><img src="https://img.shields.io/github/v/release/MillerMedia/awtest?color=ff9900&style=flat-square" alt="Latest Release"></a>
  <a href="https://github.com/MillerMedia/awtest/actions/workflows/test.yml"><img src="https://img.shields.io/github/actions/workflow/status/MillerMedia/awtest/test.yml?label=tests&style=flat-square" alt="Tests"></a>
  <a href="https://github.com/MillerMedia/awtest/blob/main/LICENSE"><img src="https://img.shields.io/github/license/MillerMedia/awtest?style=flat-square" alt="License"></a>
  <a href="https://goreportcard.com/report/github.com/MillerMedia/awtest"><img src="https://goreportcard.com/badge/github.com/MillerMedia/awtest" alt="Go Report Card"></a>
  <img src="https://img.shields.io/badge/go-1.19+-00ADD8?style=flat-square&logo=go" alt="Go Version">
</p>

---

AWTest quickly enumerates the permissions of AWS credentials by performing read-only list/describe operations across **63 AWS services** with **117 API calls**. Built for pentesters, red teamers, and cloud security assessors.

## Features

- **Broad AWS Coverage** -- 63 services, 117 API calls covering S3, EC2, IAM, Lambda, EKS, RDS, DynamoDB, GuardDuty, Security Hub, and more
- **Speed Presets** -- `--speed` presets (`safe`, `fast`, `insane`) for OPSEC-aware scan parallelism
- **Multiple Output Formats** -- Text, JSON, YAML, CSV, and table output
- **File Export** -- Write results directly to a file with `--output-file`
- **Service Filtering** -- Include or exclude specific services with `--services` and `--exclude-services`
- **Configurable Timeouts** -- Set scan duration limits with `--timeout`
- **Concurrent Scanning** -- `--speed` presets or fine-grained `--concurrency` control
- **Session Token Support** -- Works with temporary credentials (STS)
- **Cross-Platform** -- Pre-built binaries for macOS, Linux, and Windows

## Installation

### Homebrew (macOS/Linux)

```sh
brew install --cask MillerMedia/tap/awtest
```

### Go Install

Requires Go 1.19+:

```sh
go install github.com/MillerMedia/awtest/cmd/awtest@latest
```

### Binary Download

Download pre-built binaries from [GitHub Releases](https://github.com/MillerMedia/awtest/releases):

| Platform | File |
|---|---|
| macOS (Intel) | `awtest_<version>_darwin_amd64.tar.gz` |
| macOS (Apple Silicon) | `awtest_<version>_darwin_arm64.tar.gz` |
| Linux (amd64) | `awtest_<version>_linux_amd64.tar.gz` |
| Linux (arm64) | `awtest_<version>_linux_arm64.tar.gz` |
| Windows | `awtest_<version>_windows_amd64.zip` |

## Usage

### Scan using current AWS CLI profile

```bash
awtest
```

### Scan with explicit credentials

```bash
awtest --aki=AKIAEXAMPLE --sak=YourSecretAccessKey
```

### Scan with temporary credentials (STS)

```bash
awtest --aki=ASIAEXAMPLE --sak=YourSecretKey --st=YourSessionToken
```

### Output as JSON to a file

```bash
awtest --format=json --output-file=results.json
```

### Scan only specific services

```bash
awtest --services=s3,ec2,iam,lambda
```

### Exclude noisy services

```bash
awtest --exclude-services=cloudwatch,cloudtrail
```

### Fast scan with moderate parallelism

```bash
awtest --speed=fast
```

### Maximum speed with JSON output

```bash
awtest --speed=insane --format=json --output-file=results.json
```

### Example Output

```
[AWTest] [user-id] [info] AKIABCDEFGHIJKLMNO
[AWTest] [account-number] [info] 123456789012
[AWTest] [iam-arn] [info] arn:aws:iam::123456789012:user/exampleUser
[AWTest] [s3:ListBuckets] [info] Found S3 bucket: example-bucket-1
[AWTest] [s3:ListBuckets] [info] Found S3 bucket: example-bucket-2
[AWTest] [ec2:DescribeInstances] [info] Found EC2 instance: i-0abcdef1234567890
[AWTest] [iam:ListUsers] [info] Found IAM user: exampleUser1
[AWTest] [lambda:ListFunctions] [info] Found Lambda function: myFunction
[AWTest] [eks:ListClusters] [info] Found EKS cluster: production
[AWTest] [rds:DescribeDBInstances] [info] Found RDS instance: mydb
[AWTest] [appsync:ListGraphqlApis] [info] Error: Access denied to this service.
```

## Flags

| Flag | Description | Default |
|---|---|---|
| `--aki`, `--access-key-id` | AWS Access Key ID | |
| `--sak`, `--secret-access-key` | AWS Secret Access Key | |
| `--st`, `--session-token` | AWS Session Token | |
| `--region` | AWS Region | `us-west-2` |
| `--format` | Output format: `text`, `json`, `yaml`, `csv`, `table` | `text` |
| `--output-file` | Write output to file | |
| `--services` | Include only specific services (comma-separated) | all |
| `--exclude-services` | Exclude specific services (comma-separated) | none |
| `--timeout` | Maximum scan duration (e.g., `5m`, `300s`) | `5m` |
| `--speed` | Speed preset: `safe`, `fast`, `insane` (controls scan parallelism) | `safe` |
| `--concurrency` | Number of concurrent service scans (overrides speed preset when specified) | `1` |
| `--quiet` | Suppress info messages, show only findings | `false` |
| `--debug` | Enable debug output | `false` |
| `--version` | Print version and build info | |

## Speed Presets & OPSEC

AWTest provides named speed presets that control scan parallelism. Choose the right preset based on your OPSEC requirements:

| Preset | Concurrency | CloudTrail Profile | Use Case |
|---|---|---|---|
| `safe` | 1 worker | Minimal footprint -- sequential calls resemble normal console usage | Stealth engagements, red team ops, production infrastructure |
| `fast` | 5 workers | Moderate density -- more events in a shorter window, within normal operational patterns | Time-sensitive pentests where speed matters more than stealth |
| `insane` | 20 workers | Dense burst -- all 63 services hammered simultaneously, visible API call spike | Lab environments, time-critical bug bounty, OPSEC not a concern |

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
| `text` | Real-time terminal scanning (default) | `[AWTest] [s3:ListBuckets] [info] Found S3 bucket: my-bucket` |
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

## Supported AWS Services (63 services, 117 API calls)

<details>
<summary>Click to expand full service list (63 services, 117 API calls)</summary>

### Compute & Containers

| Service | API Calls |
|---|---|
| Batch | ListJobs |
| EC2 | DescribeInstances |
| ECS | ListClusters |
| EKS | ListClusters |
| Elastic Beanstalk | DescribeApplications, DescribeEvents |
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
| Certificate Manager (ACM) | ListCertificates |
| Cognito Identity | ListIdentityPools |
| Cognito User Pools | ListUserPools |
| ECR | DescribeRepositories, ListImages, GetRepositoryPolicy |
| GuardDuty | ListDetectors, GetFindings, ListFilters |
| IAM | ListUsers |
| KMS | ListKeys |
| Macie | ListClassificationJobs, ListFindings, DescribeBuckets |
| Organizations | ListAccounts, ListOrganizationalUnits, ListPolicies |
| Secrets Manager | ListSecrets |
| Security Hub | GetEnabledStandards, GetFindings, ListEnabledProductsForImport |
| STS | GetCallerIdentity |
| WAF | ListWebACLs |

### Storage

| Service | API Calls |
|---|---|
| Backup | ListBackupVaults, ListBackupPlans, ListRecoveryPointsByBackupVault, GetBackupVaultAccessPolicy |
| EFS | DescribeFileSystems |
| Glacier | ListVaults |
| S3 | ListBuckets |

### Networking

| Service | API Calls |
|---|---|
| API Gateway | RestApis, GetApiKeys, GetDomainNames |
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
| Transcribe | ListTranscriptionJobs, ListLanguageModels, ListVocabularies, StartTranscriptionJob |

### IoT

| Service | API Calls |
|---|---|
| IoT | ListThings, ListCertificates, ListPolicies |

</details>

## Contributing

Contributions are welcome! The most common contribution is **adding support for a new AWS service**. A complete service implementation template is provided at [`cmd/awtest/services/_template/`](cmd/awtest/services/_template/) with step-by-step instructions and an annotated reference implementation.

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
