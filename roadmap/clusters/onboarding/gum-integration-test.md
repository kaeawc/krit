# GumIntegrationTest

**Cluster:** [onboarding](README.md) · **Status:** shipped 2026-04-15 ·
**Phase:** 1 · **Severity:** n/a (test)

## What it is

End-to-end validation of the gum onboarding script against real
codebases before declaring Phase 1 complete.

## Acceptance criteria

1. **Automated playground coverage.** ✅
   `cmd/krit/krit_init_integration_test.go:TestKritInitPlaygroundEndToEnd`
   copies `playground/android-app` and `playground/kotlin-webservice`
   into t.TempDir() for each of the 4 shipped profiles (8 subtests
   per run, 16 total) and asserts the full gum flow produces a
   valid `krit.yml` + `.krit/baseline.xml`. Finding counts match
   the direct `./krit --config ...` invocations from the
   profile-templates commit.

2. **Greenfield path.** ✅
   `cmd/krit/krit_init_integration_test.go:TestKritInitGreenfield`
   creates a temp directory with a single trivial Kotlin file and
   runs the strict profile against it. Config + baseline both land.

3. **Dogfood against krit's own repo.** Deliberately skipped —
   krit is a Go project, not a Kotlin project, so the script has no
   Kotlin files to scan. The playground tests are the equivalent
   coverage.

4. **External project adoption with real feedback.** ✅
   Run against `~/github/coil` (real-world repo, 403 Kotlin files)
   in an isolated git worktree during the 2026-04-15 session. The
   script ran end-to-end (scan → table → select → questionnaire →
   write → autofix → baseline → summary) and produced:
   - strict:        8185 findings
   - balanced:       154 findings   ← selected
   - relaxed:         73 findings
   - detekt-compat:  144 findings
   - 942-line krit.yml with 20 rule overrides
   - 119-line .krit/baseline.xml suppressing 141 post-fix findings
   - 8 autofixes applied across 7 files

   The feedback is load-bearing: **5 of the 7 autofixes were
   semantically broken** (UseRequireNotNull dropping disjunctions,
   RedundantSuspendModifier stripping `suspend` from `actual`
   declarations, UselessCallOnNotNull dropping null-safe calls,
   CheckNotNull variants losing guard clauses, and a catastrophic
   rewrite of a boolean function to return Int). These are
   bugs in krit's fix rules, not the onboarding script — the
   script surfaced them by exercising autofix on real code. They
   are tracked out-of-cluster as a rule-quality concern.

   The onboarding script passed the adoption test; krit's autofix
   at `--fix-level=idiomatic` did not.

Phase 2 is unblocked by criterion 4 being satisfied on 2026-04-15.

## Links

- Cluster root: [`README.md`](README.md)
