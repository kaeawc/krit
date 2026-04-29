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

- [x] AddJavascriptInterface
- [x] GetInstance
- [x] SecureRandom
- [x] TrustedServer
- [x] GetSignatures

---

## P1 — Correctness

Runtime crashes or incorrect behaviour at the call site.

- [x] ShowToast
- [x] FragmentConstructor
- [x] ViewConstructor
- [x] WrongImport
- [x] ServiceCast
- [x] LayoutInflation
- [x] ViewTag
- [x] ViewHolder

---

## P2 — Performance

Rules that catch common Android performance anti-patterns.

- [x] UseSparseArrays
- [x] UseValueOf
- [x] ObsoleteLayoutParam

---

## P3 — Other source rules

Lint, style, and less-critical correctness rules.

- [x] LongLogTag
- [x] LogTagMismatch
- [x] NonInternationalizedSms
- [x] PluralsCandidate
- [x] PropertyEscape
- [ ] WrongViewCast (negative-only — needs ResourceIndex support in fixture harness)
- [x] ObjectAnimatorBinding
- [x] MissingPermission
- [x] WrongConstant
- [x] TrulyRandom
- [x] RtlAware
- [x] RtlFieldAccess
- [x] GridLayout
- [x] MangledCRLF
- [x] ResourceName

---

## P4 — Hard / type-dependent (deferred)

These rules require type inference, API-level data, or whole-program
analysis that krit does not yet support. Fixture files should be added
only after the underlying infra lands.

- [x] NewApi
- [x] InlinedApi
- [x] Deprecated
- [x] Override
- [ ] Range
- [x] OverrideAbstract
- [ ] SwitchIntDef
- [x] UnusedResources
- [x] Registered
- [ ] LocalSuppress
- [ ] SupportAnnotationUsage
- [ ] CustomViewStyleable
- [ ] ResourceType
- [x] Instantiatable
- [ ] IconColors
- [ ] IconLauncherShape

---

## Progress summary

| Priority | Total | Done | Remaining |
|---|---:|---:|---:|
| P0 Security | 5 | 5 | 0 |
| P1 Correctness | 8 | 8 | 0 |
| P2 Performance | 3 | 3 | 0 |
| P3 Other source | 15 | 14 | 1 |
| P4 Deferred | 16 | 8 | 8 |
| **Total** | **47** | **38** | **9** |

Note: the total here is 47 rather than 43 because the P4 group contains
4 rules that were not yet counted in the original item-24 audit (they
were added during the item-22 gap analysis). Mark a checkbox done and
update the table above as fixtures land.
