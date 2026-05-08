---
name: krit-capability-migration
description: Use when deciding whether a Krit rule truly needs Kotlin Analysis API, NeedsTypeInfo, NeedsResolver, parsed files, cross-file analysis, or library model facts, and when moving rules off KAA by replacing oracle facts with AST, source type inference, imports, project indexes, or Gradle-derived library profile.
---

# Krit Capability Migration

Use this when reducing Kotlin Analysis API workload or clarifying a rule's `Needs*` declaration.

## Capability Definitions

- **No Needs**: file-local AST, line text, imports, and cheap helpers are enough.
- **NeedsLinePass**: the rule scans `file.Lines` instead of specific AST nodes.
- **NeedsResolver**: source-level type inference is required, but KAA is not.
- **NeedsTypeInfo**: legacy type-aware bucket. It does not by itself mean KAA.
- **NeedsOracle**: the rule requires KAA facts that source inference cannot safely provide.
- **NeedsParsedFiles**: the rule needs all parsed source files.
- **NeedsCrossFile**: the rule needs the source symbol/reference index.
- **NeedsModuleIndex**: the rule needs Gradle module boundaries or module-level graph facts.
- **NeedsManifest**, **NeedsResources**, **NeedsGradle**: Android or Gradle data is required.
- **LibraryFacts**: rules can read project-derived library facts without an extra `Needs` flag.

Prefer the narrowest capability that proves the finding. Removing KAA is only an improvement when the replacement evidence is at least as precise.

## Evidence Order

1. Prefer flat AST nodes, identifiers, navigation chains, imports, and source indexes.
2. Use source inference for receiver, owner, and nullability facts when it is sufficient.
3. Use library model facts for dependency, SDK, Compose, Room, Hilt, or similar project context.
4. Use KAA only for facts that cannot be proven with source-visible data.
5. If text matching is unavoidable, make it lexical-state aware and test comments, strings, raw strings, and local lookalikes.

## Migration Workflow

1. Find rules that may involve KAA:

```bash
rg -n "NeedsOracle|NeedsTypeInfo|OracleCallTargets|OracleDeclarationNeeds|PreferOracle" internal/rules/registry_*.go internal/rules/*.go
```

2. Write down the exact fact being requested: call target, annotations, suspend marker, expression type/nullability, supertypes, members, or diagnostics.

3. Check whether source-visible evidence can replace it:

- imports or FQNs
- tree-sitter node shape
- source-level type inference
- parsed-file summaries
- cross-file or module indexes
- Gradle/library profile facts

4. If KAA remains necessary, narrow the oracle request with bounded call targets, declaration needs, lexical hints, or diagnostics-only needs.

5. Add tests that lock the capability boundary: rule behavior, local lookalikes, Java/Kotlin parity where applicable, and oracle-filter narrowing for KAA rules.

## Validation

Use focused tests while iterating, then run:

```bash
go build -o krit ./cmd/krit/
go vet ./...
go test ./... -count=1
```
