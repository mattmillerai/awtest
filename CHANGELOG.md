# Changelog

## [1.4.0](https://github.com/Miller-Media/awtest-private/compare/v1.3.0...v1.4.0) (2026-10-24)


### Features

* **cli:** add --endpoint-url to point awtest at a custom AWS-compatible endpoint ([#10](https://github.com/Miller-Media/awtest-private/issues/10)) ([0cd312f](https://github.com/Miller-Media/awtest-private/commit/0cd312fe31dc84438da062dafed00c92cbc24722))

### Behavior changes

* **sts / sqs:** the sts and sqs follow-up calls previously ignored `-aki`/`-sak` and used the ambient default credential chain (environment, profile, instance metadata). They now use the credentials awtest was given, matching every other service. If you relied on the ambient chain for these two services, pass credentials explicitly.

## [1.3.0](https://github.com/Miller-Media/awtest-private/compare/v1.2.0...v1.3.0) (2026-10-24)


### Features

* **cli:** add offline diff command for comparing two JSON scan outputs ([#9](https://github.com/Miller-Media/awtest-private/issues/9)) ([d272d53](https://github.com/Miller-Media/awtest-private/commit/d272d530a065f4a54efdb68f95df5b3008917d8d))


### Bug Fixes

* **transcribe:** remove write-type StartTranscriptionJob call from read-only enumeration ([#7](https://github.com/Miller-Media/awtest-private/issues/7)) ([951ca04](https://github.com/Miller-Media/awtest-private/commit/951ca0429a4137a999ab6a954c3640a89cecc6ad))

## [1.2.0](https://github.com/Miller-Media/awtest-private/compare/v1.1.1...v1.2.0) (2026-10-24)


### Features

* **cli:** add -o and -v short flags ([48f8114](https://github.com/Miller-Media/awtest-private/commit/48f811456c6d9be88daaa5279fd3379081cadd58))

## [1.1.1](https://github.com/Miller-Media/awtest-private/compare/v1.1.0...v1.1.1) (2026-10-24)


### Bug Fixes

* **ci:** stop public sync tag leak/retrigger and repair CI ([#1](https://github.com/Miller-Media/awtest-private/issues/1)) ([ee3da10](https://github.com/Miller-Media/awtest-private/commit/ee3da1041ee06e2c8667943b5049f3f29e6e2f66))
* **deps:** build with supported Go toolchain 1.27.3 (go 1.26+) to pick up stdlib security fixes ([#3](https://github.com/Miller-Media/awtest-private/issues/3)) ([976b1ec](https://github.com/Miller-Media/awtest-private/commit/976b1ec5ceaa871a9180180ee1d7f2533226ebb3))
