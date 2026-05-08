---
name: krit-rule-vetting
description: Use when auditing Krit static-analysis rules for false positives, missing project context, library lookalikes, config mismatches, Java/Kotlin coverage gaps, or comparing findings against representative Kotlin/Java/Android projects. Covers both rule precision and Gradle/version-catalog false positives.
---

# Krit Rule Vetting

Use this workflow before broadening a rule, trusting a noisy result, or changing rule metadata.

## Evidence Patterns To Check

- **Comments and strings**: source text scans must not match comments, KDoc, escaped strings, raw strings, or Gradle string literals unless the rule is explicitly about those nodes.
- **Token boundaries**: avoid substring and prefix matches for identifiers, imports, URLs, method names, and local lookalikes.
- **Receiver/owner proof**: common method names need structural receiver, owner, import, source-index, or type evidence.
- **Scope boundaries**: walks must stop at nested functions, lambdas, anonymous functions, classes/objects, and declarations that can shadow names.
- **Traversal completeness**: inspect all relevant operands, siblings, and ancestors instead of only the first convenient child.
- **Parser shape**: verify flat AST node kinds for important Kotlin/Java constructs with focused tests.
- **Java parity**: if a rule declares Java support, add Java positives and Java local-lookalike negatives.
- **Config parity**: schema metadata, validation, defaults, and runtime matching must agree.

## Start With Evidence

Run the target repo with JSON. Add performance metadata when cost matters:

```bash
go build -o krit ./cmd/krit/
./krit -no-cache -perf -perf-rules -f json -q -o /tmp/krit_target.json /path/to/project || true
```

Inspect samples for the rule:

```bash
jq -r '.findings[] | select(.rule=="RuleName") | [.file,.line,.message] | @tsv' /tmp/krit_target.json
jq -r '.perfRuleStats[] | select(.rule=="RuleName")' /tmp/krit_target.json
```

Before judging findings, verify project config is applied. If uncertain, pass `--config`.

## Check Project Context

High-volume or surprising findings are often caused by missing project facts. Inspect:

```bash
jq '.projectProfile | {hasGradle, dependencyExtractionComplete, hasUnresolvedDependencyRefs, catalogCompleteness}' /tmp/krit_target.json
```

If Gradle or dependency extraction is incomplete, rules that depend on library absence should stay conservative.

## Vet The Rule

For each suspected issue:

1. Confirm the registry entry has accurate `NodeTypes`, `Languages`, `Needs`, fix level, confidence, and implementation.
2. Confirm the rule uses the narrowest available evidence: AST, imports, source inference, project profile, cross-file index, module index, or KAA.
3. Add the regression fixture that would have failed before the fix.
4. Include Java/Kotlin parity fixtures when the rule supports both languages.
5. Run focused tests first, then full validation when the change is no longer local.

## Validation

```bash
go build -o krit ./cmd/krit/
go vet ./...
go test ./... -count=1
```
