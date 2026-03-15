# Service Implementation Template

Use this template to add a new AWS service to awtest. Follow the steps below to create a fully functional service scanner.

## Placeholder Reference

| Placeholder | Replace With | Example |
|---|---|---|
| `SERVICENAME` | Package name (lowercase, no underscores) | `certificatemanager` |
| `DISPLAYNAME` | PascalCase service name | `CertificateManager` |
| `SERVICECalls` | Exported var name (`DISPLAYNAMECalls`) | `CertificateManagerCalls` |
| `AWSSDKPACKAGE` | AWS SDK v1 service package name | `acm` |
| `AWSPREFIX` | AWS API prefix (lowercase) | `acm` |
| `APIMETHOD` | AWS API method name | `ListCertificates` |
| `APIMETHODINPUT` | SDK input struct name | `ListCertificatesInput` |
| `RESULTTYPE` | SDK type for response items | `CertificateSummary` |
| `RESPONSEFIELD` | Field on API output containing items | `CertificateSummaryList` |
| `NAMEFIELD` | Field on each item for resource name | `DomainName` |
| `RESOURCETYPE` | Lowercase singular resource type | `certificate` |

## Steps

### Step 1: Create service directory

```bash
mkdir cmd/awtest/services/SERVICENAME/
```

### Step 2: Copy the template

```bash
cp cmd/awtest/services/_template/calls.go.tmpl cmd/awtest/services/SERVICENAME/calls.go
```

### Step 3: Replace all placeholders

Open `calls.go` and replace every placeholder from the table above with your actual values. Remove all `// TODO:` comments once addressed.

### Step 4: Implement Call()

Update the `Call` function with the actual AWS SDK API call:
- Use the correct SDK client constructor (e.g., `acm.New(sess)`)
- Use the `WithContext` variant of the API call for timeout support
- Extract the correct response field containing the resource list

### Step 5: Implement Process()

Update the `Process` function to extract resources from the API response:
- Type-assert the output to the correct slice type
- Extract resource names using nil-safe pointer access
- Build `ScanResult` entries for each discovered resource
- Call `utils.PrintResult()` for console output

### Step 6: Register the service

Edit `cmd/awtest/services/services.go`:

1. Add the import (maintain alphabetical order):
   ```go
   "github.com/MillerMedia/awtest/cmd/awtest/services/SERVICENAME"
   ```

2. Add the append call in `AllServices()` (maintain alphabetical order):
   ```go
   allServices = append(allServices, SERVICENAME.DISPLAYNAMECalls...)
   ```

### Step 7: Tidy module dependencies

```bash
go mod tidy
```

This is required if your service imports a new AWS SDK package not already used by the project.

### Step 8: Verify compilation

```bash
go build ./cmd/awtest
```

### Step 9: Create tests

Copy the test template and replace placeholders:

```bash
cp cmd/awtest/services/_template/calls_test.go.tmpl cmd/awtest/services/SERVICENAME/calls_test.go
```

Follow the table-driven test pattern in the template. At minimum, include test cases for:
- Successful response with results
- Error response
- Empty results
- Wrong type assertion

See `calls_test.go.tmpl` for the full pattern, or refer to existing service tests (e.g., `sagemaker/calls_test.go`) for more detailed examples.

### Step 10: Run tests

```bash
make test
```

Ensure all existing tests pass with no regressions. Note that `make test` runs with the `-race` flag by default, which will catch any race conditions.

## Concurrent Safety

Services are automatically executed concurrently when using `--speed=fast` or `--speed=insane`. The worker pool and `safeScan` wrapper handle all parallelism and panic recovery transparently. **No concurrency-specific code is needed in your service.**

**What NOT to do:**

- Do not import `sync` or `sync/atomic`
- Do not spawn goroutines
- Do not use global mutable state
- Do not write directly to stdout (use `utils.PrintResult()`)

Each `Call()` invocation must be self-contained. The framework handles everything else.

## Reference Implementation

See `example_calls.go.reference` in this directory for an annotated copy of the CertificateManager service showing how each template section maps to real code.
