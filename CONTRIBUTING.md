# Contributing to awtest

Thank you for your interest in contributing to awtest! The most common contribution is adding support for new AWS services.

## Development Workflow

### Prerequisites

- **Go:** 1.19+ (must match version in `go.mod`)
- **make:** For build automation
- **golangci-lint:** For code linting (`go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`)

### Setup

```bash
git clone https://github.com/MillerMedia/awtest.git
cd awtest
go mod download
```

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
4. **Implement Call()** with the actual AWS SDK v1 API call (use `WithContext` variant)
5. **Implement Process()** to extract and format discovered resources
6. **Write table-driven tests** in `calls_test.go` (see [Testing Standards](#testing-standards) below; all new services must include tests)
7. **Register the service** in `cmd/awtest/services/services.go` (maintain alphabetical order; STS must stay first)
8. **Tidy dependencies:** `go mod tidy` (required if importing a new SDK package)
9. **Build and test:** `make test` (includes race detector) and `go build ./cmd/awtest`. Your service will be run concurrently by the worker pool — no concurrency code is needed on your part, but all tests must pass with the race detector enabled.
10. **Test manually:** `go run ./cmd/awtest --debug` to verify output with real AWS credentials

See `cmd/awtest/services/_template/README.md` for detailed instructions and `example_calls.go.reference` for an annotated real-world example.

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
- See any existing service's `calls_test.go` for reference (e.g., `cmd/awtest/services/sagemaker/calls_test.go`)

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
- Use `go func()` or spawn goroutines inside `Call()` or `Process()`
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
- [ ] `AWSService` struct fields are correctly populated (Name, Call, Process, ModuleName)
- [ ] `Call()` accepts `context.Context` and `*session.Session`
- [ ] `Call()` uses `WithContext` variant of the API method for timeout support
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
- [ ] `Call()` uses `WithContext` variant
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

Releases are fully automated via GitHub Actions and GoReleaser:

1. **Create a version tag:**
   ```bash
   git tag v0.x.y
   ```

2. **Push the tag:**
   ```bash
   git push origin v0.x.y
   ```

3. **GitHub Actions handles the rest:**
   - GoReleaser builds cross-platform binaries (Linux, macOS, Windows)
   - Release artifacts are uploaded to GitHub Releases
   - Homebrew cask auto-updates via push to `MillerMedia/homebrew-tap`

> **Note:** The release workflow requires a `GH_PAT` secret (not the default `GITHUB_TOKEN`) for cross-repo push to the Homebrew tap repository.

## Development Requirements

- **Go:** 1.19+
- **AWS SDK:** v1 (`github.com/aws/aws-sdk-go`) -- do not use SDK v2
- **Test framework:** standard `testing` package (no external test dependencies)
