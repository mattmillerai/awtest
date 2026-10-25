# Changelog

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
