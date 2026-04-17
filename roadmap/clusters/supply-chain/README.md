# Supply chain hygiene cluster

Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)

Introduces `BuildGraph` (multi-module walker) and a version-catalog
TOML parser. See the parent doc for the infra design.

## Dependency source

- [`dependency-from-jcenter.md`](dependency-from-jcenter.md)
- [`dependency-from-bintray.md`](dependency-from-bintray.md)
- [`dependency-from-http.md`](dependency-from-http.md)
- [`dependency-snapshot-in-release.md`](dependency-snapshot-in-release.md)
- [`dependency-dynamic-version.md`](dependency-dynamic-version.md)
- [`dependency-without-group.md`](dependency-without-group.md)

## Version catalog

- [`version-catalog-unused.md`](version-catalog-unused.md)
- [`version-catalog-duplicate-version.md`](version-catalog-duplicate-version.md)
- [`version-catalog-build-src-mismatch.md`](version-catalog-build-src-mismatch.md)
- [`version-catalog-raw-version-in-build.md`](version-catalog-raw-version-in-build.md)

## Multi-module drift

- [`kotlin-version-mismatch-across-modules.md`](kotlin-version-mismatch-across-modules.md)
- [`compile-sdk-mismatch-across-modules.md`](compile-sdk-mismatch-across-modules.md)
- [`jvm-target-mismatch.md`](jvm-target-mismatch.md)
- [`apply-plugin-twice.md`](apply-plugin-twice.md)

## Verification / integrity

- [`dependency-verification-disabled.md`](dependency-verification-disabled.md)
- [`missing-gradle-checksums.md`](missing-gradle-checksums.md)
- [`gradle-wrapper-validation-action.md`](gradle-wrapper-validation-action.md)

## Plugin / convention

- [`convention-plugin-applied-to-wrong-target.md`](convention-plugin-applied-to-wrong-target.md)
- [`all-projects-block.md`](all-projects-block.md)
- [`configurations-all-side-effect.md`](configurations-all-side-effect.md)
- [`dependencies-in-root-project.md`](dependencies-in-root-project.md)
