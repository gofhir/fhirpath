# Changelog

## [1.2.0](https://github.com/gofhir/fhirpath/compare/v1.1.0...v1.2.0) (2026-03-01)


### Features

* add FHIR Model interface for version-specific type resolution ([7f6e07f](https://github.com/gofhir/fhirpath/commit/7f6e07f54c2747f9bbe1b3fe157476deefde10d9))
* wire model.IsResource() via isResourceType helper ([c8e76e2](https://github.com/gofhir/fhirpath/commit/c8e76e2c55548382ec90b52c7603e91cc2431b44))

## [1.1.0](https://github.com/gofhir/fhirpath/compare/v1.0.3...v1.1.0) (2026-03-01)


### Features

* add %rootResource built-in variable support ([#6](https://github.com/gofhir/fhirpath/issues/6)) ([9e8f4cc](https://github.com/gofhir/fhirpath/commit/9e8f4ccab3bc6d6338eb8a489b57c5c3cd80fc72))
* implement aggregate() with $total and $index support ([#8](https://github.com/gofhir/fhirpath/issues/8)) ([382ae63](https://github.com/gofhir/fhirpath/commit/382ae63eec3bde180d5b8dcffe4d382392c73bcc))


### Bug Fixes

* as operator now filters collections instead of requiring singleton ([#7](https://github.com/gofhir/fhirpath/issues/7)) ([b5d4f80](https://github.com/gofhir/fhirpath/commit/b5d4f807b6bd7f7f45868fcdedd024b6435162e0))
* **docs:** per-language menus and improved Quick Start styling ([54d0422](https://github.com/gofhir/fhirpath/commit/54d0422d01b59fa3b5ae11a3e0c79f7c83bdf499))
* **docs:** upgrade Hugo to v0.155.3 and add npm ci step ([8d4a255](https://github.com/gofhir/fhirpath/commit/8d4a2556b61510465d105f9557f5f7d80ba2864b))

## [1.0.3](https://github.com/gofhir/fhirpath/compare/v1.0.2...v1.0.3) (2026-02-17)


### Bug Fixes

* remove gofhir/fhir/r4 dependency from tests ([c3bf7e3](https://github.com/gofhir/fhirpath/commit/c3bf7e3b1a025fccf29ee2c1cda2bfe28dd5f022))

## [1.0.2](https://github.com/gofhir/fhirpath/compare/v1.0.1...v1.0.2) (2026-01-26)


### Bug Fixes

* allow as() function to work on collections ([cf3042e](https://github.com/gofhir/fhirpath/commit/cf3042e3a3eb24f0c17a7b863ed0902707eb0998)), closes [#2](https://github.com/gofhir/fhirpath/issues/2)

## [1.0.1](https://github.com/gofhir/fhirpath/compare/v1.0.0...v1.0.1) (2026-01-24)


### Bug Fixes

* resolve golangci-lint issues ([6021b6e](https://github.com/gofhir/fhirpath/commit/6021b6ee2d6fdaf5985529d64cf295ec949fdf62))

## [0.2.0](https://github.com/robertoAraneda/gofhir/compare/fhirpath/v0.1.0...fhirpath/v0.2.0) (2026-01-17)


### ⚠ BREAKING CHANGES

* Package import paths have changed.

### Features

* initial release ([82ec28c](https://github.com/robertoAraneda/gofhir/commit/82ec28c30a38afb26bbf7b2503945573606da517))


### Code Refactoring

* migrate to multi-module monorepo architecture ([42ae0de](https://github.com/robertoAraneda/gofhir/commit/42ae0de8aa2f98cbe6e94fcef4736a6a0184bfb7))
