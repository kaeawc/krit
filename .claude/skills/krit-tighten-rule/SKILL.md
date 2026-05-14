---
name: krit-tighten-rule
description: Use when fixing a false positive (or a missed positive) in an existing Krit rule. Distilled from the "Rule Implementation Guardrails" section of CLAUDE.md. Drives the regression-fixture-first workflow: reproduce on a fixture, identify the missing evidence, lock the regression with a failing test, narrow the rule, re-run.
---

# Krit Tighten Rule

Use this when an existing rule fires wrongly (false positive), misses a clear positive, or needs its evidence narrowed before being broadened. Cross-link with `krit-rule-vetting` for the audit step that produces the candidate list, and `krit-project-analysis` when the cause is missing project context rather than rule logic.

## Reproduce First

Confirm the rule actually fires (or fails to fire) on a representative input. Add a minimal fixture under the rule's category that reproduces the problem before changing any rule code:

```bash
mkdir -p tests/fixtures/negative/<category>/<RuleName>
$EDITOR tests/fixtures/negative/<category>/<RuleName>/local_lookalike.kt
```

Run the focused fixture test to confirm the failure exists:

```bash
go test ./internal/rules/ -run TestNegativeFixtures -v
```

The regression test must fail *before* the fix lands. If it passes immediately, you have not actually reproduced the issue — go narrower.

## Diagnose The Evidence Gap

Identify which class of bug this is. Most FP fixes in the last 3 months landed in one of these buckets:

- **Lexical state ignored.** Rule matched inside a comment, KDoc, escaped string, raw string, or Gradle string literal.
- **Identifier overlap without receiver proof.** `System.out`, Android `Context`, lifecycle, DB, logging APIs have common method names. The rule needs structural receiver, owner, import, source-index, or type evidence — not just the method name.
- **Local lookalike shadow.** A local `val Log = ...`, parameter named `Context`, or nested class shadows the global identifier. The walk did not stop at the shadowing scope.
- **Body walk crossed a scope boundary.** Body walks must stop at nested functions, lambdas, anonymous functions, classes/objects, and shadowing declarations.
- **First-child bias.** The rule looked at only the first operand/sibling/ancestor and missed where the real evidence lived.
- **Wrong parser shape.** `!!`, Elvis, safe calls, infix, raw strings have specific flat-AST node kinds. Verify with a focused parser/helper test.
- **Config/runtime drift.** Schema metadata, validation, defaults, and runtime matching disagree — especially with implicit-anchor regex options.

Pick the bucket before writing the fix. If you cannot, the fixture is not minimal enough.

## Narrow With The Right Evidence

Prefer, in order:

1. flat AST node kinds, identifiers, navigation chains, imports, source-index facts
2. source-level type inference (`NeedsResolver`)
3. library/project profile facts (`ctx.LibraryFacts`) — see `krit-project-analysis`
4. Kotlin Analysis API (`NeedsOracle`) — only when source-visible evidence cannot prove it

Do not broaden a heuristic when a project-profile guard or a receiver check is the real missing condition.

## Lock The Regression

Every fix lands with the fixture that would have failed before the fix. For FPs this means a negative fixture; for missed positives, a positive fixture; for autofix bugs, a fixable fixture plus its `.expected` partner.

If the rule supports Java, add the Java parity case for the same bug class — Java local-lookalike negatives are the single most common gap.

## Validate

Focused tests first, then full validation:

```bash
go test ./internal/rules/ -run TestPositiveFixtures -v
go test ./internal/rules/ -run TestNegativeFixtures -v

go build -o krit ./cmd/krit/
go vet ./...
golangci-lint run ./...
make lint-rules
go test ./... -count=1
```

Run `make integration` before pushing if the fix touches dispatch, suppression, or pipeline behavior.

## When To Stop

A rule tightening is done when:

- the new fixture fails on the pre-fix code and passes on the post-fix code
- no existing positive fixture regressed
- the rule's `Confidence` and `DefaultActive` still match reality (downgrade them if the FP class was widespread)
- if a project corpus surfaced the FP, the rule's findings on that corpus dropped to expected — see `krit-dogfood`
