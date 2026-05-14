# Krit Tighten Rule

Fix a false positive (or missed positive) in an existing Krit rule.

ARGUMENTS: `$ARGUMENTS` — typically the rule name and optionally the offending file or fixture path / external repo path.

## Instructions

Invoke the `krit-tighten-rule` skill for the rule named in ARGUMENTS.

1. **Reproduce.** Add a minimal failing fixture under `tests/fixtures/negative/<category>/<RuleName>/` (or `positive/` for a missed positive) before changing rule code. Run `go test ./internal/rules/ -run TestNegativeFixtures -v` and confirm it fails on the pre-fix code.
2. **Diagnose.** Identify which evidence-gap bucket this is:
   - lexical state ignored (comments, KDoc, strings)
   - identifier overlap without receiver proof
   - local lookalike shadow
   - body walk crossed a scope boundary
   - first-child bias
   - wrong parser shape
   - config / runtime drift
3. **Narrow.** Replace the broken evidence with the narrowest available: flat AST → source-level type inference → library/profile facts → KAA. Do not broaden when a project-profile guard is the real missing condition (see `krit-project-analysis`).
4. **Java parity.** If the rule supports Java, add the Java version of the same negative/positive case.
5. **If a corpus surfaced the FP**, re-run `krit-dogfood` against that corpus and confirm the FP cluster dropped.
6. **Validate.**
   ```bash
   go build -o krit ./cmd/krit/
   go vet ./...
   golangci-lint run ./...
   make lint-rules
   go test ./... -count=1
   ```

Report: rule name, FP bucket, fixture(s) added, evidence change, before/after finding counts if a corpus was involved.
