---
name: krit-project-analysis
description: Use when analyzing Krit findings in context of a project's Gradle version catalog, library dependencies, SDK versions, or project model. Covers understanding what the librarymodel package knows about a project, diagnosing rules that fire incorrectly due to absent/present library context, and deciding which rules should be guarded by profile facts.
---

# Krit Project Analysis

Use this when a finding depends on Gradle, dependency, SDK, module, source-index, or Android project facts.

## First Checks

Run Krit with JSON and performance metadata:

```bash
go build -o krit ./cmd/krit/
./krit -no-cache -perf -f json -q -o /tmp/krit_project.json /path/to/project || true
```

Inspect the project profile:

```bash
jq '.projectProfile' /tmp/krit_project.json
```

Key fields:

- `hasGradle`
- `dependencyExtractionComplete`
- `hasUnresolvedDependencyRefs`
- `catalogCompleteness`
- Kotlin compiler profile
- Android min/compile/target SDK values
- resolved dependency coordinates

If dependency extraction is incomplete, library absence is not proof. Rules should assume relevant libraries may be present unless the profile can prove absence.

## Mixed Source Coverage

For Java/Kotlin mixed projects, confirm every source kind the rule claims to support participates in:

- parse discovery
- generated-source filtering
- suppression indexing
- dispatch
- source-symbol indexing
- cross-file references
- module grouping
- output paths and counts

Rules that claim Java support need Java positives and Java local-lookalike negatives, not only Kotlin fixtures.

## Library Model Use

Rules should use `librarymodel.EnsureFacts(ctx.LibraryFacts)` for project-derived facts such as:

- database libraries
- Compose
- dependency injection libraries
- test frameworks
- Android SDK versions
- catalog completeness and unresolved dependency refs

Avoid broadening source heuristics when a project-profile guard is the real missing condition.

## Module And Dead-Code Checks

Module-aware rules require complete module and reference data. Before acting on findings:

1. Confirm Gradle/module discovery succeeded.
2. Confirm source references are indexed for every relevant language.
3. Sample findings for references from XML, generated code, reflection, dynamic dispatch, or framework entry points that the index cannot see.
4. Add fixture coverage for any missing reference source that should be modeled.

## Validation

After changing a rule or project model behavior:

```bash
go build -o krit ./cmd/krit/
go vet ./...
go test ./... -count=1
```
