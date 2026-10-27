# Changelog

## [1.8.0](https://github.com/mattmillerai/awtest/compare/v1.7.0...v1.8.0) (2026-10-27)


### Features

* add depth checks for STS, IAM, S3, ECS and EKS ([#52](https://github.com/mattmillerai/awtest/issues/52)) ([f393e11](https://github.com/mattmillerai/awtest/commit/f393e113959c80216b9d863a77b181fb55b66c64))
* add ELBv2, WAFv2, Resource Groups Tagging, Access Analyzer, Account and API Gateway V2 checks ([#50](https://github.com/mattmillerai/awtest/issues/50)) ([854fefb](https://github.com/mattmillerai/awtest/commit/854fefb70fe5f4028313bb4e443e7cd24a704bc9))

### Notes

* **Six new services:** ELBv2, WAFv2, Resource Groups Tagging API, IAM Access Analyzer, Account and API Gateway V2 checks (all read-only). awtest now covers 69 services with 143 API calls (was 63 services and 120 calls in 1.7.0).
* **Depth checks:** new read-only calls: `sts:GetAccessKeyInfo`; IAM instance profiles, OpenID Connect providers, SAML providers, account summary, account password policy and MFA devices; ECS services and task definitions; EKS nodegroups. S3 bucket scans now also call `GetBucketEncryption` and `GetBucketVersioning` for each bucket (up to 500 buckets).
* **More API calls:** a full scan makes more API calls than 1.7.0 (143 call types versus 120), plus the extra per-bucket S3 calls above.
* **Larger binary:** the release binary is about 35.9 MB on linux/amd64 (1.7.0 was 33.9 MB, 1.6.0 was 25.0 MB), and the release archive is about 11.4 MB (1.7.0 was 10.9 MB). The CI size guard stays at 36 MB.

## [1.7.0](https://github.com/mattmillerai/awtest/compare/v1.6.0...v1.7.0) (2026-10-27)


### Features

* migrate all AWS service checks to aws-sdk-go-v2 ([#46](https://github.com/mattmillerai/awtest/issues/46)) ([7161fb1](https://github.com/mattmillerai/awtest/commit/7161fb108cc4cf768204f6b91014d5336bdaa0fa))

### Notes

* **AWS SDK for Go v2:** awtest now uses the AWS SDK for Go v2. The v1 SDK reached end of support on 31 Jul 2025 and has been removed. All 59 services are still covered.
* **Error text changed:** AWS errors are now printed as `<code>: <message>` with no request-ID or struct-dump suffix. SQS errors now use JSON-protocol codes such as `AccessDeniedException`.
* **Retries:** each call makes up to 4 attempts in total. Throttling is left to awtest's own backoff.
* **Credential chain isolation:** the credential chain ignores ambient `AWS_ENDPOINT_URL*` settings. Remaining exceptions: IMDS and `credential_process`.
* **Empty values now print as 0/false:** a few optional numeric and boolean fields now print `0` or `false` instead of an empty value.
* **Connection fix:** a new HTTP client wrapper fixes an error caused by a closed request body.
* **Larger binary:** the release binary is about 33.9 MB on linux/amd64 (v1.6.0 was 25.0 MB, about +36%), and the release archive is about 10.8 MB (v1.6.0 was 6.8 MB).
* **Pinned SDK modules:** the cloudwatch, cloudwatchlogs and kinesis modules are pinned to specific v2 versions, and the service modules are pinned to pre-schema SDK releases (see CONTRIBUTING). Startup time is unchanged for practical purposes.

## [1.6.0](https://github.com/mattmillerai/awtest/compare/v1.5.1...v1.6.0) (2026-10-25)


### Features

* **cli:** add offline replay mode for demos and labs ([#18](https://github.com/mattmillerai/awtest/issues/18)) ([27fade6](https://github.com/mattmillerai/awtest/commit/27fade6ffe576c0756e2f45b9292379a90d081a1))
* **services:** add IAM role/policy and S3 bucket policy coverage ([#16](https://github.com/mattmillerai/awtest/issues/16)) ([e250931](https://github.com/mattmillerai/awtest/commit/e250931a9773312890939caf91e02f1a376e1cf0))

### Notes

* **More API calls:** the new IAM and S3 policy calls add API calls to a scan. Caps (documented in the README): IAM follow-up calls cover only the first 100 roles and first 100 groups (up to about 400 calls, 2 per role or group); S3 makes 5 extra calls per bucket, for up to 500 buckets.
* **`-replay` is fully offline:** it runs against canned, fictional responses with fake credentials and never touches real AWS.

## [1.5.1](https://github.com/mattmillerai/awtest/compare/v1.5.0...v1.5.1) (2026-10-25)


### Bug Fixes

* update module path and update-check URL to github.com/mattmillerai/awtest ([#14](https://github.com/mattmillerai/awtest/issues/14)) ([d336ad6](https://github.com/mattmillerai/awtest/commit/d336ad68d992897aa4ac2b08e4da092619d82a59))

### Module path change

* The Go module path is now `github.com/mattmillerai/awtest`. Anyone importing the module or running `go install` with `github.com/MillerMedia/awtest` must switch to the new path. CLI and Homebrew users are unaffected. The update check now uses the new repository.

## [1.5.0](https://github.com/mattmillerai/awtest/compare/v1.4.0...v1.5.0) (2026-10-24)


### Features

* **cli:** add --all-regions to scan every enabled AWS region ([#12](https://github.com/mattmillerai/awtest/issues/12)) ([58e77e3](https://github.com/mattmillerai/awtest/commit/58e77e385fd3d268749b7491a496dcd65db7de2e))

### Notes

* **--all-regions is opt-in:** default behavior is unchanged. When enabled, awtest scans every enabled AWS region, which makes many more API calls and generates many more CloudTrail events. Consider `-speed fast` and a longer `-timeout`.

## [1.4.0](https://github.com/mattmillerai/awtest/compare/v1.3.0...v1.4.0) (2026-10-24)


### Features

* **cli:** add --endpoint-url to point awtest at a custom AWS-compatible endpoint ([#10](https://github.com/mattmillerai/awtest/issues/10)) ([0cd312f](https://github.com/mattmillerai/awtest/commit/0cd312fe31dc84438da062dafed00c92cbc24722))

### Behavior changes

* **sts / sqs:** the sts and sqs follow-up calls previously ignored `-aki`/`-sak` and used the ambient default credential chain (environment, profile, instance metadata). They now use the credentials awtest was given, matching every other service. If you relied on the ambient chain for these two services, pass credentials explicitly.

## [1.3.0](https://github.com/mattmillerai/awtest/compare/v1.2.0...v1.3.0) (2026-10-24)


### Features

* **cli:** add offline diff command for comparing two JSON scan outputs ([#9](https://github.com/mattmillerai/awtest/issues/9)) ([d272d53](https://github.com/mattmillerai/awtest/commit/d272d530a065f4a54efdb68f95df5b3008917d8d))


### Bug Fixes

* **transcribe:** remove write-type StartTranscriptionJob call from read-only enumeration ([#7](https://github.com/mattmillerai/awtest/issues/7)) ([951ca04](https://github.com/mattmillerai/awtest/commit/951ca0429a4137a999ab6a954c3640a89cecc6ad))

## [1.2.0](https://github.com/mattmillerai/awtest/compare/v1.1.1...v1.2.0) (2026-10-24)


### Features

* **cli:** add -o and -v short flags ([48f8114](https://github.com/mattmillerai/awtest/commit/48f811456c6d9be88daaa5279fd3379081cadd58))

## [1.1.1](https://github.com/mattmillerai/awtest/compare/v1.1.0...v1.1.1) (2026-10-24)


### Bug Fixes

* **ci:** stop public sync tag leak/retrigger and repair CI ([#1](https://github.com/mattmillerai/awtest/issues/1)) ([ee3da10](https://github.com/mattmillerai/awtest/commit/ee3da1041ee06e2c8667943b5049f3f29e6e2f66))
* **deps:** build with supported Go toolchain 1.27.3 (go 1.26+) to pick up stdlib security fixes ([#3](https://github.com/mattmillerai/awtest/issues/3)) ([976b1ec](https://github.com/mattmillerai/awtest/commit/976b1ec5ceaa871a9180180ee1d7f2533226ebb3))
