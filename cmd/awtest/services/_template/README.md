# Service Implementation Template

Use this template to add a new AWS service to awtest. Services are written against **aws-sdk-go-v2** (aws-sdk-go v1 has been removed). Follow the steps below to create a fully functional service scanner.

## Placeholder Reference

| Placeholder | Replace With | Example |
|---|---|---|
| `SERVICENAME` | Package name (lowercase, no underscores) | `certificatemanager` |
| `DISPLAYNAME` | PascalCase service name | `CertificateManager` |
| `SERVICECalls` | Exported var name (`DISPLAYNAMECalls`) | `CertificateManagerCalls` |
| `AWSSDKPACKAGE` | aws-sdk-go-v2 service package name (`github.com/aws/aws-sdk-go-v2/service/<pkg>`); its `types` subpackage is imported as `AWSSDKPACKAGEtypes` | `acm` |
| `AWSPREFIX` | AWS API prefix (lowercase) | `acm` |
| `APIMETHOD` | AWS API method name | `ListCertificates` |
| `APIMETHODINPUT` | SDK input struct name (replace it before `APIMETHOD`) | `ListCertificatesInput` |
| `RESULTTYPE` | SDK type (in `AWSSDKPACKAGEtypes`) for response items; v2 returns values, not pointers | `CertificateSummary` |
| `RESPONSEFIELD` | Field on API output containing items | `CertificateSummaryList` |
| `NAMEFIELD` | Field on each item for resource name | `DomainName` |
| `RESOURCETYPE` | Lowercase singular resource type | `certificate` |
| `AWSENVSUFFIX` | `<SUFFIX>` of `AWS_ENDPOINT_URL_<SUFFIX>` (test template only): the upper-cased SDK `ServiceID`, spaces to `_` | `ACM` |

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

### Step 4: Implement CallV2

Update the `CallV2` function, `func(ctx context.Context, cfg aws.Config) (interface{}, error)`, with the actual AWS SDK API call:
- Build the client **only** with `awsv2.NewRegionalClient(cfg, region, pkg.NewFromConfig)` (pass the constructor, do not call it). Global services use one fixed region (e.g. `"us-east-1"`); regional services loop over `types.RegionsFor(ctx)`. Never call `pkg.NewFromConfig(cfg)`, `config.LoadDefaultConfig` or set `BaseEndpoint`/`HTTPClient` yourself: `-endpoint-url` and credentials would not be honoured, and `TestNoV2ClientBypass` fails
- v2 API methods always take the context: `svc.ListCertificates(ctx, &acm.ListCertificatesInput{})`
- Use the v2 paginator (`pkg.NewXPaginator(svc, input)`, `HasMorePages`, `NextPage(ctx)`) when the call has to walk every page
- Extract the correct response field containing the resource list

`CallV2` is never called directly: the scan runs it through `types.AWSService.Invoke(ctx)`, which takes the `aws.Config` from the context (`types.WithAWSConfig`) and returns API errors as `<code>: <message>`.

### Step 5: Implement Process()

Update the `Process` function to extract resources from the API response:
- Type-assert the output to the correct slice type (`[]AWSSDKPACKAGEtypes.RESULTTYPE`)
- Extract resource names with the nil-safe `aws.ToString` (and `aws.ToInt32`, `aws.ToBool`, ...)
- Build `ScanResult` entries for each discovered resource
- Call `utils.PrintResult()` for console output

### Step 6: Register the service

Edit `cmd/awtest/services/services.go`:

1. Add the import (maintain alphabetical order):
   ```go
   "github.com/mattmillerai/awtest/cmd/awtest/services/SERVICENAME"
   ```

2. Add the append call in `AllServices()` (maintain alphabetical order):
   ```go
   allServices = append(allServices, SERVICENAME.DISPLAYNAMECalls...)
   ```

### Step 7: Add the service to the endpoint guards

Edit `cmd/awtest/services/v2_env_suffix_test.go`:

1. Add the SDK package's `ServiceID` (`const ServiceID` in `vendor/github.com/aws/aws-sdk-go-v2/service/<pkg>/api_client.go`, also exported as `<pkg>.ServiceID`) to `v2ServiceIDs`, under the service-name prefix (the part of `Name` before `:`). Add one entry per client the service builds.
2. Add the same prefix to the `v2EnvSuffix` table with the matching `AWS_ENDPOINT_URL_<SUFFIX>` parts, in the same order. Never guess the suffix from the service name: `certificatemanager` is `ACM`, and a wrong suffix makes the ambient-endpoint guard pass vacuously.

`TestAllServicesHonorCustomEndpoint` and `TestAllServicesHonorRegionScope` then run your service against a fake server with hostile ambient `AWS_ENDPOINT_URL*` settings and fail if any request leaves the custom endpoint or an unscoped region.

### Step 8: Tidy module dependencies

```bash
go mod tidy
go mod vendor
```

This is required if your service imports a new aws-sdk-go-v2 service module not already used by the project. Check `git diff go.mod` afterwards: the core, config, credentials and smithy-go pins must not change. Some service modules need a specific version to match the core pin (see the notes in `cmd/awtest/awsv2/doc.go`).

### Step 9: Verify compilation

```bash
go build ./cmd/awtest
```

### Step 10: Create tests

Copy the test template and replace placeholders:

```bash
cp cmd/awtest/services/_template/calls_test.go.tmpl cmd/awtest/services/SERVICENAME/calls_test.go
```

Follow the table-driven test pattern in the template. At minimum, include test cases for:
- Successful response with results
- Error response
- Empty results
- Wrong type assertion

The template also drives `CallV2` through `Invoke(types.WithAWSConfig(ctx, cfg))` against an `httptest` fake server, with `cfg` built by `awsv2.LoadConfig` (fake static keys, `Endpoint: srv.URL`, `HTTPClient: srv.Client()`, `MaxAttempts: 1`). Adapt the fake's response body to the service's wire protocol (JSON, query/XML or REST). The test proves the region loop, the explicit endpoint and the `<code>: <message>` error text.

See `calls_test.go.tmpl` for the full pattern, or refer to existing service tests (e.g., `certificatemanager/calls_v2_test.go`) for more detailed examples.

### Step 11: Run tests

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

Each `CallV2` invocation must be self-contained. The framework handles everything else.

## Reference Implementation

See `example_calls.go.reference` in this directory for an annotated copy of the CertificateManager service showing how each template section maps to real code.
