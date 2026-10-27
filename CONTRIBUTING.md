# Contributing to awtest

Thank you for your interest in contributing to awtest! The most common contribution is adding support for new AWS services.

## Development Workflow

### Prerequisites

- **Go:** 1.26+ (must match version in `go.mod`)
- **make:** For build automation
- **golangci-lint:** For code linting (`go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`)

### Setup

```bash
git clone https://github.com/mattmillerai/awtest.git
cd awtest
go mod download
```

### Commit Messages & Releases

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/); [release-please](https://github.com/googleapis/release-please) uses them to compute the next version and changelog automatically:

- `feat:` -> minor version bump
- `fix:` / `perf:` -> patch version bump
- `feat!:` or a `BREAKING CHANGE:` footer -> major version bump
- `deps:` (used by Dependabot) -> shown under "Dependencies" in the changelog
- `chore:` / `ci:` / `docs:` / `test:` / `refactor:` -> no release triggered

When a PR is squash-merged, **the PR title becomes the commit message**, so PR titles must follow this format too.

**Release flow:** release-please keeps a `chore(main): release X.Y.Z` PR open with the next changelog. Merging it tags `vX.Y.Z`, which `sync-public.yml` mirrors to the public repo, where GoReleaser builds, signs, and publishes the release and Homebrew cask.

### Common Commands

| Command | Description |
|---|---|
| `make build` | Build the awtest binary |
| `make test` | Run all tests with race detector and coverage |
| `make lint` | Run golangci-lint |
| `make clean` | Remove build artifacts |
| `go run ./cmd/awtest --debug` | Run with debug output |

## Adding a New AWS Service

A complete service implementation template is provided at `cmd/awtest/services/_template/`. Follow these steps:

1. **Create your service directory:** `cmd/awtest/services/yourservice/`
2. **Copy the template:** Copy `cmd/awtest/services/_template/calls.go.tmpl` to `cmd/awtest/services/yourservice/calls.go`
3. **Replace all placeholders** using the reference table in `cmd/awtest/services/_template/README.md`
4. **Implement CallV2** with the actual aws-sdk-go-v2 API call (`func(ctx context.Context, cfg aws.Config) (interface{}, error)`): build the client with `awsv2.NewRegionalClient(cfg, region, pkg.NewFromConfig)`, call `svc.Method(ctx, &pkg.MethodInput{})` (or the v2 paginator) and read pointer fields with `aws.ToString`
5. **Implement Process()** to extract and format discovered resources
6. **Write table-driven tests** in `calls_test.go` (see [Testing Standards](#testing-standards) below; all new services must include tests)
7. **Register the service** in `cmd/awtest/services/services.go` (maintain alphabetical order; STS must stay first)
8. **Tidy dependencies:** `go mod tidy` (required if importing a new SDK package)
9. **Build and test:** `make test` (includes race detector) and `go build ./cmd/awtest`. Your service will be run concurrently by the worker pool — no concurrency code is needed on your part, but all tests must pass with the race detector enabled.
10. **Test manually:** `go run ./cmd/awtest --debug` to verify output with real AWS credentials

See `cmd/awtest/services/_template/README.md` for detailed instructions and `example_calls.go.reference` for an annotated real-world example.

## Pinned aws-sdk-go-v2 service modules

Recent releases of many `github.com/aws/aws-sdk-go-v2/service/*` modules include a `schemas` package whose `init()` builds the whole API shape schema at program start. The linker cannot dead-code-eliminate it, so awtest (which links only ~176 of the ~6,000 SDK operations) pays for every operation of every service. Measured on linux/amd64 with release flags (`-trimpath -ldflags "-s -w"`):

| | binary | `-version` startup | `-version` RSS |
|---|---|---|---|
| Latest service modules | 48.3 MB | 41.9 ms | 65 MB |
| Pinned (below) | 33.9 MB | 9.9 ms | 21 MB |

Upstream tracking issue: [aws/aws-sdk-go-v2#3601](https://github.com/aws/aws-sdk-go-v2/issues/3601).

So every service module below is pinned to the **last release that has no `schemas` package**. Modules not listed (for example `s3`, `iam`, `ec2`, `sts`, `rds`, `sns`) have no schema-bearing release yet and track upstream normally. `aws-sdk-go-v2` core (`v1.47.4`), `config`, `credentials`, `smithy-go` and `aws/protocol/eventstream` are pinned separately and must not move with these.

Guards:

- Dependabot ignores each pinned module (see `.github/dependabot.yml`).
- `TestNoSDKSchemasPackages` (`cmd/awtest/awsv2/schemas_guard_test.go`) fails if any vendored service module contains a `schemas` package.
- The `size-guard` CI job (`internal/sizeguard`, run locally with `AWTEST_SIZE_GUARD=1 go test -count=1 -v ./internal/sizeguard/`) builds a release-flag linux/amd64 binary and fails above 36 MB, or if `awtest -version` uses more than 40 MB RSS.

Notes: `cloudwatch` is also held back because v1.53+ uses the CBOR protocol, which the replay fixtures cannot speak; `cloudwatchlogs`/`kinesis` newer than the pins also need an eventstream bump.

### Adding or bumping a service module

Pick the newest version of the module that has no `schemas` directory in its module zip (`go mod download -json <module>@<version>`, then check `<Dir>/schemas`). Then run `go get <module>@<version>`, `go mod tidy`, `go mod vendor`, and verify the core/config/credentials/smithy pins in `go.mod` did not move.

### Unpinning

When upstream fixes [#3601](https://github.com/aws/aws-sdk-go-v2/issues/3601) (schemas become dead-code-eliminable), or a security fix requires a newer release:

1. `go get` the newer version, `go mod tidy`, `go mod vendor`.
2. Add the service to `allowedSchemas` in `cmd/awtest/awsv2/schemas_guard_test.go` with a reason (or empty the map and delete the guard once upstream is fixed).
3. Re-run `AWTEST_SIZE_GUARD=1 go test -count=1 -v ./internal/sizeguard/`; raise the limit in `internal/sizeguard/sizeguard_test.go` only deliberately.
4. Remove the module from the `ignore:` list in `.github/dependabot.yml` and from the table below.

### Pin table

| Module (`github.com/aws/aws-sdk-go-v2/service/...`) | Pinned version |
|---|---|
| `acm` | `v1.46.1` |
| `amplify` | `v1.44.1` |
| `apigateway` | `v1.47.0` |
| `appsync` | `v1.61.0` |
| `athena` | `v1.62.1` |
| `backup` | `v1.65.0` |
| `batch` | `v1.76.0` |
| `cloudtrail` | `v1.60.1` |
| `cloudwatch` | `v1.52.8` |
| `cloudwatchlogs` | `v1.87.0` |
| `codebuild` | `v1.72.7` |
| `codecommit` | `v1.36.5` |
| `codedeploy` | `v1.40.1` |
| `codepipeline` | `v1.49.7` |
| `cognitoidentity` | `v1.38.1` |
| `cognitoidentityprovider` | `v1.70.0` |
| `configservice` | `v1.70.1` |
| `directconnect` | `v1.51.0` |
| `dynamodb` | `v1.68.0` |
| `ecr` | `v1.56.0` |
| `ecs` | `v1.98.0` |
| `efs` | `v1.49.4` |
| `eks` | `v1.99.0` |
| `emr` | `v1.66.1` |
| `eventbridge` | `v1.48.7` |
| `glacier` | `v1.37.1` |
| `glue` | `v1.160.0` |
| `guardduty` | `v1.93.0` |
| `iot` | `v1.80.1` |
| `ivs` | `v1.55.7` |
| `ivschat` | `v1.29.4` |
| `ivsrealtime` | `v1.37.7` |
| `kinesis` | `v1.54.0` |
| `kms` | `v1.52.0` |
| `lambda` | `v1.108.0` |
| `mediaconvert` | `v1.104.0` |
| `opensearch` | `v1.80.0` |
| `organizations` | `v1.56.1` |
| `rekognition` | `v1.60.3` |
| `sagemaker` | `v1.270.1` |
| `secretsmanager` | `v1.49.0` |
| `securityhub` | `v1.81.0` |
| `sfn` | `v1.50.0` |
| `sqs` | `v1.48.1` |
| `ssm` | `v1.79.3` |
| `transcribe` | `v1.60.1` |
| `waf` | `v1.38.4` |


## Code Standards

### Naming Conventions

| Element | Convention | Example |
|---|---|---|
| Package name | lowercase, no underscores | `certificatemanager` |
| Exported variable | PascalCase + "Calls" suffix | `CertificateManagerCalls` |
| Service Name field | PascalCase | `"CertificateManager"` |
| Method Name field | `prefix:APIMethod` | `"acm:ListCertificates"` |
| File name | always `calls.go` | `calls.go` |
| Test file | co-located in same package | `calls_test.go` |
| Exported types | PascalCase | `ScanResult` |
| Unexported variables | camelCase | `allResults` |

Do **not** use `SCREAMING_SNAKE_CASE` for constants.

### Error Handling

All AWS API errors must be handled using `utils.HandleAWSError`:

```go
if err != nil {
    if awsErr := utils.HandleAWSError(debug, "service:Method", err); awsErr != nil {
        return []types.ScanResult{{
            ServiceName: "ServiceName",
            MethodName:  "service:Method",
            Error:       awsErr,
            Timestamp:   time.Now(),
        }}
    }
    return []types.ScanResult{{
        ServiceName: "ServiceName",
        MethodName:  "service:Method",
        Error:       err,
        Timestamp:   time.Now(),
    }}
}
```

- Never use `panic()` for expected errors
- `HandleAWSError` returns an `error` -- when it returns a non-nil value (e.g., `*types.InvalidKeyError` for invalid credentials), propagate it in the `ScanResult.Error` field so the main loop can detect abort conditions
- Return a single `ScanResult` with the `Error` field set on failure
- `HandleAWSError` classifies errors: invalid credentials return `InvalidKeyError` (abort scan), access denied prints a message and returns nil (continue), other errors are pretty-printed

### Testing Standards

- **Table-driven tests** with `t.Run(tt.name, ...)`:

```go
func TestServiceProcess(t *testing.T) {
    tests := []struct {
        name     string
        input    interface{}
        expected []types.ScanResult
    }{
        {name: "success with resources", ...},
        {name: "empty response", ...},
        {name: "error handling", ...},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic
        })
    }
}
```

- Use the standard `testing` package with direct comparisons and `t.Errorf`/`t.Fatalf` for assertions
- Co-locate tests in `calls_test.go` in the **same package** (not `package X_test`)
- Besides the table-driven `Process()` test, drive `CallV2` through `Invoke` against an `httptest` fake: build the config with `awsv2.LoadConfig` (static fake keys, `Endpoint: srv.URL`, `HTTPClient: srv.Client()`, `MaxAttempts: 1`), carry it with `types.WithAWSConfig(ctx, cfg)` and call `svc.Invoke(ctx)`. `Invoke` returns API errors as `<code>: <message>` (via `awsv2.Wrap`), which is what `Process()` receives
- See any existing service's `calls_test.go` for reference (e.g., `cmd/awtest/services/certificatemanager/calls_v2_test.go`)

#### Race Detection

`make test` runs with the **`-race` flag** by default:

```bash
go test -v -race -coverprofile=coverage.out ./...
```

The Go race detector instruments memory accesses at compile time and reports data races at runtime. Since services are executed concurrently by the worker pool (up to 20 goroutines at `--speed=insane`), the race detector catches any unsafe shared-state access that would cause non-deterministic behavior in production.

All tests must pass with the race detector enabled. Do not use `-race=false` to bypass it. To run race detection manually:

```bash
go test -v -race ./...
```

#### Concurrent Comparison Testing

Service scan results must be **identical regardless of concurrency level**. Running the same scan with `--speed=safe` (1 worker) and `--speed=insane` (20 workers) must produce the same set of results (order may differ). Automated comparison tests verify this invariant. When adding a new service, ensure your implementation is deterministic — the same inputs always produce the same outputs, regardless of how many other services are running in parallel.

### Concurrent Architecture — Service Contract

Services are **concurrency-unaware by design**. The concurrency layer is fully encapsulated in the worker pool and supporting infrastructure:

- `cmd/awtest/worker_pool.go` — spawns N worker goroutines and feeds services via a buffered channel
- `cmd/awtest/safe_scan.go` — wraps each service call with `defer/recover` panic recovery and error classification (throttle/denied/error)
- `cmd/awtest/backoff.go` — per-service exponential backoff with jitter for throttled API calls
- `cmd/awtest/speed.go` — speed preset resolution (`safe`=1, `fast`=5, `insane`=20 workers)

When a user runs `--speed=fast` or `--speed=insane`, your service will automatically be executed concurrently alongside other services. You do not need to write any concurrency-specific code.

**Rule:** Services must **not** import `sync`, `sync/atomic`, or any concurrency primitives. Services must not spawn goroutines (`go func()`) or maintain global mutable state.

**Why:** The worker pool coordinates all parallelism. If a service introduces its own synchronization primitives, it creates hidden coupling with the concurrency layer and risks deadlocks, race conditions, or unpredictable behavior under concurrent execution.

**Anti-patterns** (do not do these in a service):
- Import `sync` or `sync/atomic`
- Use `go func()` or spawn goroutines inside `CallV2` or `Process()`
- Add global mutable state that could be accessed by multiple goroutines
- Write to stdout (only formatters write to stdout; progress writes to stderr)

Error handling within services still uses `utils.HandleAWSError` as described in [Error Handling](#error-handling). The `safeScan` wrapper provides an additional layer of protection — recovering from panics and classifying AWS errors for retry/skip decisions — but this is transparent to service authors.

### Documentation

- Add doc comments on all exported functions, types, and variables
- Inline comments for non-obvious logic

## Service Validation Checklist

Before submitting a new service, verify:

- [ ] Package name is lowercase with no underscores
- [ ] Exported variable follows `DISPLAYNAMECalls` naming convention
- [ ] `AWSService` struct fields are correctly populated (Name, CallV2, Process, ModuleName)
- [ ] `CallV2` has the signature `func(context.Context, aws.Config) (interface{}, error)` and is run through `types.AWSService.Invoke(ctx)` (the config comes from `types.WithAWSConfig`)
- [ ] Clients are built only with `awsv2.NewRegionalClient` / `awsv2.NewClient`, never `pkg.NewFromConfig(cfg)`, `config.LoadDefaultConfig` or hand-set `BaseEndpoint`/`HTTPClient`, so `-endpoint-url` and credentials are inherited and ambient `AWS_ENDPOINT_URL*` settings are ignored (enforced by `TestNoV2ClientBypass` and `TestAllServicesHonorCustomEndpoint`)
- [ ] The service prefix has an entry in the `v2EnvSuffix` and `v2ServiceIDs` tables in `cmd/awtest/services/v2_env_suffix_test.go` (the SDK `ServiceID` gives the `AWS_ENDPOINT_URL_<SUFFIX>` part; never guess it)
- [ ] Regional services loop over `types.RegionsFor(ctx)` (never `types.Regions` directly) so `-all-regions` is honored (enforced by `TestServicesUseRegionsFor`); global services must not call it
- [ ] API calls are read-only and take the context (`svc.Method(ctx, ...)`) for timeout support (enforced by `TestAllCallsAreReadOnly`)
- [ ] `Process()` accepts `(interface{}, error, bool)` and returns `[]types.ScanResult`
- [ ] `Process()` handles errors via `utils.HandleAWSError`
- [ ] `Process()` returns proper `ScanResult` entries with all fields populated:
  - `ServiceName` (PascalCase), `MethodName` (prefix:Method), `ResourceType` (lowercase singular), `ResourceName`, `Details` (map), `Error` (nil on success), `Timestamp`
- [ ] `Process()` calls `utils.PrintResult()` for each discovered resource
- [ ] Service Name field uses correct PascalCase format
- [ ] Table-driven tests exist in `calls_test.go`
- [ ] Service is registered in `services.go` in alphabetical order (STS stays first)
- [ ] Service does not import `sync` or `sync/atomic` packages
- [ ] `go build ./cmd/awtest` succeeds
- [ ] `make test` passes with race detector (default — do not use `-race=false`)
- [ ] Manual testing completed with `go run ./cmd/awtest --debug`

## Pull Request Process

### PR Title Format

Use the format: **"Add [Service Name] enumeration"**

Examples:
- "Add CertificateManager enumeration"
- "Add ElastiCache enumeration"

### PR Description

Include the following in your PR description:

```markdown
## Summary

Brief description of the AWS service being added and what resources it enumerates.

## Checklist

- [ ] Service follows `AWSService` interface pattern
- [ ] `CallV2` passes the context to every API call
- [ ] Error handling uses `utils.HandleAWSError`
- [ ] Table-driven tests added in `calls_test.go`
- [ ] Service registered in `services.go` (alphabetical order)
- [ ] `make test` passes
- [ ] `go build ./cmd/awtest` succeeds
- [ ] Tested manually with `go run ./cmd/awtest --debug`
```

### Review Process

- PRs are reviewed for correctness, adherence to project patterns, and test coverage
- Ensure all CI checks pass before requesting review
- Address review feedback promptly
- See the [Service Validation Checklist](#service-validation-checklist) for what reviewers look for

## Release Process

> **Note:** This section is for maintainers only.

Releases are fully automated via GitHub Actions, release-please, and GoReleaser. Do not create tags by hand.

1. **Merge the release-please PR:** release-please keeps a `chore(main): release X.Y.Z` PR open against `main` with the next version and changelog. Merging it tags `vX.Y.Z` on this (private) repo.
2. **Sync to the public repo:** the `v*` tag triggers `sync-public.yml`, which pushes a cleaned commit (private-only artifacts stripped) and only that tag to `mattmillerai/awtest`.
3. **Build and publish:** the pushed tag triggers `release.yml` in the public repo, which runs GoReleaser to build cross-platform binaries, sign and notarize the macOS binaries, publish the GitHub Release, and update the cask in `mattmillerai/homebrew-tap`.

> **Note:** A `GH_PAT` secret (not the default `GITHUB_TOKEN`) is required at each cross-repo step: release-please uses it because tags created with `GITHUB_TOKEN` do not trigger other workflows, and the sync must fire on its tag; the sync step uses it to push to the public repo; and GoReleaser uses it to push the cask update to the Homebrew tap.

## Development Requirements

- **Go:** 1.26+ (must match version in `go.mod`)
- **AWS SDK:** v2 (`github.com/aws/aws-sdk-go-v2`) -- aws-sdk-go v1 has been removed and must not be reintroduced (enforced by `TestNoV1SDKImports`)
- **Test framework:** standard `testing` package (no external test dependencies)
