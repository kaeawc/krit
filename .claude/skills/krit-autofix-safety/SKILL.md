---
name: krit-autofix-safety
description: Use when writing, broadening, or fixing a Krit autofix. Covers the FixCosmetic / FixIdiomatic / FixSemantic safety tiers, ktfmt-compatible output, comment and inline preservation, the fix-drift lint gate, and when to neuter an unsafe autofix rather than ship it.
---

# Krit Autofix Safety

Use this when a rule has a `Fix` field, when adding an autofix to an existing rule, or when investigating a regression from an autofix in production. Cross-link with `krit-tighten-rule` when the underlying rule logic is also wrong, and `krit-add-rule` when scaffolding a brand-new fixable rule.

## Safety Tiers

Every fix must declare exactly one:

- **`FixCosmetic`** — whitespace, indentation, brace placement, trailing commas. Never changes program behavior. Must produce ktfmt-compatible output.
- **`FixIdiomatic`** — replaces a construct with an equivalent more-idiomatic one (e.g. `if (x) a else b` → `if (x) { a } else { b }` when style demands, `!= null` → `?.let`). Semantics are preserved, but the rewrite is observable in source.
- **`FixSemantic`** — changes runtime behavior in a way the rule has proven is safe (e.g. removing a redundant null check, removing `System.gc()`). Hardest to justify. Must clear the highest evidence bar.

If you cannot prove the fix belongs in the declared tier, drop a tier or remove the fix entirely. Production data shows unsafe fixes are how Krit loses trust the fastest — see `fix(rules): neuter unsafe autofixes and add fix-drift lint gate` (#172).

## The Fix-Drift Gate

`make lint-rules` enforces a fix-drift check: rules that declare a `Fix` tier must produce output compatible with that tier's contract. The gate runs as part of `make lint-rules` and `make ci`. If a fix lands that violates its tier (e.g. a `FixCosmetic` that changes brace semantics), the gate blocks it. Treat a gate failure as a real bug, not a lint-config problem.

## ktfmt Compatibility

Cosmetic and idiomatic fixes must produce output that survives a subsequent ktfmt pass without diff. The recurring failure modes from recent commits:

- **Brace wraps that lose comments.** Wrapping `if (x) a else b` into a block must preserve any trailing/inline comments and any inline RHS that ktfmt would keep on one line. See `fix(rules) tighten brace-wrap fix to preserve comments and inline RHS` (#173) and `fix(rules) ktfmt-compatible brace wraps and postfix rewrites` (#162).
- **Shared-line and in-expression positions.** Removing a statement that shares a line with another statement, or sits inside an expression, must guard against breaking the surrounding line. See `fix(ExplicitGarbageCollectionCall) guard autofix against in-expression and shared-line positions` (#171).
- **Postfix rewrites.** Postfix operators (`!!`, `?.`, infix chains) have non-obvious parser shapes — verify with focused parser/helper tests before rewriting them.

When in doubt, run ktfmt on the post-fix output of the fixable fixture and diff against `.expected`. If ktfmt produces a different result, the fix is not ktfmt-compatible.

## Fixable Fixtures

Every autofix rule needs paired fixtures under `tests/fixtures/fixable/<category>/<RuleName>/`:

- `case.kt` — input
- `case.kt.expected` — expected output after the fix

Run:

```bash
go test ./internal/rules/ -run TestFixableFixtures -v
```

Add Java fixable fixtures when the rule declares Java support — Java autofix gaps are easy to miss. See `test(java) plug Java fixable-rule coverage gaps + fix System.gc() removal` (#163).

## When To Neuter A Fix

Remove the `Fix` field (or downgrade the tier) when:

- the fix cannot prove tier compliance on real corpora
- the fix breaks ktfmt-compat on common idioms
- the fix would require evidence the rule does not currently gather (e.g. a `FixSemantic` that needs type info the rule does not have)

A rule without a fix is a flag; a rule with a wrong fix is a bug. Prefer the flag.

## Validation

```bash
go build -o krit ./cmd/krit/
go vet ./...
golangci-lint run ./...
make lint-rules
go test ./... -count=1
```

Run `make integration` if the fix changes how the fixer writes output paths or coordinates with the snapshot sidecar.
