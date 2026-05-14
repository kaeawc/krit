# Krit CI

Run a complete pre-push pass that mirrors what GitHub CI enforces.

GitHub CI runs `make ci` (build + vet + test + integration + regression), `golangci-lint run ./...`, and `make lint-rules` in separate jobs. Running `make ci` alone is not enough — it does not invoke golangci-lint or lint-rules. This command runs both.

## Instructions

Run the lint pass and the CI pass. Both are required for a faithful pre-push check:

```bash
golangci-lint run ./...
make lint-rules
make ci
```

If the user has uncommitted changes that may affect validation, run `git status` first and surface them.

When something fails, identify the failing step:

- **golangci-lint** — most common failures are gofmt drift, unused functions/imports, gosec. Fix and re-run from `golangci-lint`.
- **lint-rules** — capability-declaration gate or ad-hoc-cache gate caught drift. Update the rule's registry entry or remove the unused construct.
- **build** — fix compile errors first.
- **vet** — fix and re-run.
- **test** — run focused package tests to isolate. Report the failing package and test names.
- **integration** — the LSP/MCP/CLI integration suite caught a regression. Investigate `tests/integration/` or the playground harness.
- **regression** — playground regression expectations diverged. Inspect `playground/` expected outputs and confirm whether the divergence is intended (snapshot update needed) or a real regression.

For faster feedback during iteration that skips integration/regression, use `/krit-validate`.

Report a one-line pass/fail summary for each step and surface the first failing log lines.
