---
name: krit-capability-migration
description: Use when choosing the evidence a Krit rule needs — flat AST, source type inference (NeedsResolver/NeedsTypeInfo), imports, parsed files, cross-file or module indexes, Gradle-derived library facts, or the krit-fir type oracle (NeedsOracle) — and when replacing oracle facts with cheaper source-visible evidence.
---

# Krit Capability Migration

Use this when reducing type-oracle (krit-fir) workload or clarifying a rule's `Needs*` declaration.

## Capability Definitions

- **No Needs**: file-local AST, line text, imports, and cheap helpers are enough.
- **NeedsLinePass**: the rule scans `file.Lines` instead of specific AST nodes.
- **NeedsResolver**: source-level type inference is required, but the oracle is not.
- **NeedsTypeInfo**: legacy type-aware bucket. It does not by itself mean the oracle.
- **NeedsOracle**: the rule requires krit-fir (K2 FIR) oracle facts that source inference cannot safely provide.
- **NeedsParsedFiles**: the rule needs all parsed source files.
- **NeedsCrossFile**: the rule needs the source symbol/reference index.
- **NeedsModuleIndex**: the rule needs Gradle module boundaries or module-level graph facts.
- **NeedsManifest**, **NeedsResources**, **NeedsGradle**: Android or Gradle data is required.
- **LibraryFacts**: rules can read project-derived library facts without an extra `Needs` flag.

Prefer the narrowest capability that proves the finding. Dropping `NeedsOracle` is only an improvement when the replacement evidence is at least as precise.

## Evidence Order

1. Prefer flat AST nodes, identifiers, navigation chains, imports, and source indexes.
2. Use source inference for receiver, owner, and nullability facts when it is sufficient.
3. Use library model facts for dependency, SDK, Compose, Room, Hilt, or similar project context.
4. Use the FIR oracle (`NeedsOracle`) only for facts that cannot be proven with source-visible data.
5. If text matching is unavoidable, make it lexical-state aware and test comments, strings, raw strings, and local lookalikes.

## Migration Workflow

1. Find rules that may involve the oracle:

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

4. If the oracle remains necessary, declare `NeedsOracle` and consume only the specific facts the rule needs (call targets, declarations, or diagnostics).

5. Add tests that lock the capability boundary: rule behavior, local lookalikes, and Java/Kotlin parity where applicable.

## Validation

Use focused tests while iterating, then run:

```bash
go build -o krit ./cmd/krit/
go vet ./...
make test
```
