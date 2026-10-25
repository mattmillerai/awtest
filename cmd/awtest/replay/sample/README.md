# Sample replay fixtures

Used by `awtest -replay builtin:sample` (embedded in the binary). All data is
**fictional**: account `123456789012`, `example-*` names, and IPs from the
documentation ranges (192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24).

## Layout

```
<service>/<Action>.<ext>                         any region, any resource
<service>/<region>/<Action>.<ext>                one signing region
<service>/<Action>/<resource>.<ext>              one resource (bucket, user, role, ...)
<service>/<region>/<Action>/<resource>.<ext>     both
```

- `<service>` is the SigV4 signing name (`sts`, `iam`, `ec2`, `s3`, `lambda`, `dynamodb`, `monitoring`, ...).
- `<ext>` is `.xml` (Content-Type `text/xml`) or `.json` (`application/x-amz-json-1.1`). Other files are ignored.
- An optional status goes before the extension: `GetBucketPolicy/example-logs.404.xml` (default 200).
- Lookup order: region+resource, region, resource, generic. Region-specific files win, so a generic empty
  response can act as the fallback for the other regions.
- Calls without a fixture are answered with an AccessDenied-style error.

See the "Replay mode" section of the top-level README for details.
