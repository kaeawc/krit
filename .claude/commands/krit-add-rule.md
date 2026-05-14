# Krit Add Rule

Add a new built-in Krit rule end to end.

ARGUMENTS: `$ARGUMENTS` — typically the rule name (e.g. `UselessElvisOnNonNull`) and optional hint about category or kotlinc analog.

## Instructions

Invoke the `krit-add-rule` skill and walk it end to end for the rule named in ARGUMENTS.

1. **Qualify the rule.** Open `docs/rule-scope.md` and confirm the rule meets every qualification (documented standard, agreed-upon problem, high confidence achievable, evidence local enough). If any fails, surface the gap and stop before writing code.
2. **Pick the base and capability.** Choose `FlatDispatchBase` / `LineBase` / `ManifestBase` / `ResourceBase` / `GradleBase` based on where evidence lives. Declare the narrowest `Needs*` capability. For precompile (kotlinc-mirror) rules, use `api.CategoryPrecompile`, set `Level`, and set `KotlincAnalog`.
3. **Implement the rule** in `internal/rules/<category>.go` and register it in `internal/rules/registry_<category>.go`. Include `Languages`, `Confidence`, `DefaultActive=false` by default, and a `Meta()` descriptor for any user-facing options.
4. **Fixtures.** Add at least one positive and one negative fixture under `tests/fixtures/<category>/`. Include at least one local-lookalike negative. If the rule supports Java, add both Java positive and Java local-lookalike negative.
5. **Autofix (optional).** If adding a fix, invoke `krit-autofix-safety` to pick the tier and add fixable fixtures with `.expected` partners.
6. **Schema regen.** If `Meta()` changed, run `make schema` to regenerate `schemas/krit-config.schema.json`.
7. **Validate.** Run focused tests first, then full validation. Do not skip any of these — CI fails on all four:
   ```bash
   go build -o krit ./cmd/krit/
   go vet ./...
   golangci-lint run ./...
   make lint-rules
   go test ./... -count=1
   ```
8. **Integration.** Run `make integration` before pushing if the rule touches dispatch, suppression, output, or daemon code.

Report the rule ID, the category, the capability declaration, the fixture counts (positive/negative/fixable, Kotlin/Java), and the validation result.
