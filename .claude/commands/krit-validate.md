# Krit Validate

Run Krit's fast local validation pass: build, vet, golangci-lint, lint-rules, full unit tests. CLAUDE.md flags that `golangci-lint run ./...` and `make lint-rules` are easy to forget — this command bundles all of them so you never push a CI round-trip you could have caught locally.

## Instructions

Run the four required validations in sequence. Do not skip any of them; CI enforces all four.

```bash
go build -o krit ./cmd/krit/
go vet ./...
golangci-lint run ./...
make lint-rules
go test ./... -count=1
```

If any step fails:

- **`go build` fails** — fix the compile error, then re-run from the failing step.
- **`go vet` fails** — fix and re-run from `go vet`.
- **`golangci-lint` fails** — most common failures are gofmt drift, unused functions/imports, gosec. Fix and re-run from `golangci-lint`.
- **`make lint-rules` fails** — the capability-declaration gate (`NeedsResolver` / `NeedsOracle`) caught a drift. Update the rule's registry entry to declare the missing capability or remove the unused use. Re-run from `make lint-rules`.
- **`go test` fails** — investigate the failing package with focused tests:
  ```bash
  go test ./internal/rules/ -run <TestName> -v
  ```

Report a one-line summary of each step's pass/fail status. Do NOT report success until all five steps pass.

This command does NOT run `make integration` or `make regression` — for those, use `/krit-ci`.
