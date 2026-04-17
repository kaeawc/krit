# UnifiedFileModel

**Cluster:** [core-infra](README.md) · **Status:** planned ·
**Severity:** n/a (infra) · **Default:** n/a

## What it does

Replaces the current four separate parsing pipelines (Kotlin/Java,
AndroidManifest XML, Android resource XML, Gradle) with a single
`ParsedFile` type carrying a `Language` tag. All rule families
(including manifest, resource, and Gradle rules) dispatch through the
main dispatcher rather than through separate loops in the CLI.

## Current cost

Android rules exist in a parallel universe:
- `ManifestRule`, `ResourceRule`, `GradleRule` each define their own
  interface, their own registry (`ManifestRules`, `ResourceRules`,
  `GradleRules`), and their own execution loop
  (`runAndroidProjectAnalysis()` / `runAndroidProjectAnalysisColumns()`
  in `cmd/krit/main.go`, ~100 lines).
- Config exclusions, suppression, confidence values, and baseline
  suppression are applied differently (or not at all) for Android rules
  vs Kotlin rules.
- The Android dependency enum (`AndroidDependencies()`,
  `AndroidDepNone`) is a separate capability layer with no counterpart
  in the main rule interface.
- Java files are only parsed when a `CrossFileRule` is active — they
  have no first-class representation and cannot be targeted by rules.

Adding a new language (e.g., TOML for Gradle version catalogs) would
require a fifth parallel pipeline.

Relevant files:
- `internal/rules/rule.go` — `ManifestRule`, `ResourceRule`,
  `GradleRule` interfaces
- `cmd/krit/main.go:runAndroidProjectAnalysisColumns()` — separate
  Android execution loop
- `internal/android/` — parsing is sound, execution wiring is not

## Proposed design

Extend `scanner.ParsedFile` with a `Language` field:

```go
type Language uint8

const (
    LangKotlin   Language = iota
    LangJava
    LangXML        // manifest + resource — sub-type in Metadata
    LangGradle
    LangVersionCatalog
)

type ParsedFile struct {
    Path     string
    Language Language
    Content  []byte
    FlatTree []FlatNode
    Lines    []string
    Metadata any // *android.ManifestMeta, *android.ResourceMeta, etc.
}
```

Rules declare which languages they apply to in their descriptor:

```go
rules.Rule{
    ID:        "AllowBackupManifest",
    Languages: []scanner.Language{scanner.LangXML},
    NodeTypes: []string{"element"},
    Check:     checkAllowBackup,
}
```

The dispatcher filters files by `rule.Languages` before dispatching.
Android rules that currently live in separate registries are
re-registered as normal rules with `Languages: []Language{LangXML}`.
The `runAndroidProjectAnalysis()` loop in `main.go` is deleted.

## Migration path

1. Add `Language` field to `scanner.ParsedFile`; update the parser to
   populate it.
2. Add `Languages []scanner.Language` to the rule descriptor
   (defaults to `[]Language{LangKotlin}` if unset).
3. Update the dispatcher to skip rules whose `Languages` does not
   include the file's language.
4. Migrate `ManifestRule`, `ResourceRule`, `GradleRule`
   implementations to the new struct, one family at a time.
5. Delete `runAndroidProjectAnalysis()` from `cmd/krit/main.go`.
6. Delete `ManifestRule`, `ResourceRule`, `GradleRule` interfaces.

## Acceptance criteria

- `runAndroidProjectAnalysis()` deleted from `cmd/krit/main.go`.
- `ManifestRule`, `ResourceRule`, `GradleRule` interfaces deleted.
- All existing Android rule fixtures continue to pass.
- Config exclusions and `@Suppress` annotations apply to Android rules
  identically to Kotlin rules (verified by new fixtures).
- Java files are parsed as first-class `LangJava` entries, not
  as a side effect of `CrossFileRule` activation.

## Links

- Depends on: [`unified-rule-interface.md`](unified-rule-interface.md),
  [`phase-pipeline.md`](phase-pipeline.md)
- Related: `internal/android/`, `internal/rules/rule.go`,
  `cmd/krit/main.go`
