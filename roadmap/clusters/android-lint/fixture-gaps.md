# Android Lint fixture gaps

This file tracks the **43 source rules** that have inline Go unit tests
but no `.kt` fixture files. It supersedes Phase 4 of
[`../../24-android-lint-fixture-audit.md`](../../24-android-lint-fixture-audit.md).

## Why fixture files matter

Inline tests (table-driven Go tests calling the rule's `CheckNode` or
`CheckLines` directly) are fast and precise, but they bypass the full
scanner pipeline. `.kt` fixture files exercise:

- Tree-sitter parsing end-to-end
- The dispatcher's node-type routing
- Suppression via `@Suppress`
- JSON/SARIF output formatting
- The `go test ./internal/rules/ -run TestPositiveFixtures` harness

A rule is considered fully tested only when both forms exist. Fixture
files live in:

- `tests/fixtures/positive/android-lint/<RuleName>.kt` — code that
  **triggers** the rule (must produce at least one finding)
- `tests/fixtures/negative/android-lint/<RuleName>.kt` — code that
  **does not trigger** the rule (must produce zero findings)

---

## P0 — Security (address first)

These rules catch vulnerabilities. Fixture gaps here are the highest risk.

- [ ] AddJavascriptInterface
- [ ] GetInstance
- [ ] SecureRandom
- [ ] TrustedServer
- [ ] GetSignatures

---

## P1 — Correctness

Runtime crashes or incorrect behaviour at the call site.

- [ ] ShowToast
- [ ] FragmentConstructor
- [ ] ViewConstructor
- [ ] WrongImport
- [ ] ServiceCast
- [ ] LayoutInflation
- [ ] ViewTag
- [ ] ViewHolder

---

## P2 — Performance

Rules that catch common Android performance anti-patterns.

- [ ] UseSparseArrays
- [ ] UseValueOf
- [ ] ObsoleteLayoutParam

---

## P3 — Other source rules

Lint, style, and less-critical correctness rules.

- [ ] LongLogTag
- [ ] LogTagMismatch
- [ ] NonInternationalizedSms
- [ ] PluralsCandidate
- [ ] PropertyEscape
- [ ] WrongViewCast
- [ ] ObjectAnimatorBinding
- [ ] MissingPermission
- [ ] WrongConstant
- [ ] TrulyRandom
- [ ] RtlAware
- [ ] RtlFieldAccess
- [ ] GridLayout
- [ ] MangledCRLF
- [ ] ResourceName

---

## P4 — Hard / type-dependent (deferred)

These rules require type inference, API-level data, or whole-program
analysis that krit does not yet support. Fixture files should be added
only after the underlying infra lands.

- [ ] NewApi
- [ ] InlinedApi
- [ ] Deprecated
- [ ] Override
- [ ] Range
- [ ] OverrideAbstract
- [ ] SwitchIntDef
- [ ] UnusedResources
- [ ] Registered
- [ ] LocalSuppress
- [ ] SupportAnnotationUsage
- [ ] CustomViewStyleable
- [ ] ResourceType
- [ ] Instantiatable
- [ ] IconColors
- [ ] IconLauncherShape

---

## Progress summary

| Priority | Total | Done | Remaining |
|---|---:|---:|---:|
| P0 Security | 5 | 0 | 5 |
| P1 Correctness | 8 | 0 | 8 |
| P2 Performance | 3 | 0 | 3 |
| P3 Other source | 15 | 0 | 15 |
| P4 Deferred | 16 | 0 | 16 |
| **Total** | **47** | **0** | **47** |

Note: the total here is 47 rather than 43 because the P4 group contains
4 rules that were not yet counted in the original item-24 audit (they
were added during the item-22 gap analysis). Mark a checkbox done and
update the table above as fixtures land.
