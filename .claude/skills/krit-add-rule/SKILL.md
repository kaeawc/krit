---
name: krit-add-rule
description: Use when adding a new built-in Krit rule end to end — picking the category and base, declaring capabilities, registering the rule, writing positive/negative fixtures, adding an autofix when applicable, and ensuring Java parity when the rule covers both languages. Covers precompile (kotlinc-mirror) rules as a Level-tagged variant of the same workflow.
---

# Krit Add Rule

Use this when introducing a new built-in rule. Cross-link with `krit-autofix-safety` for fix work, `krit-capability-migration` for `Needs*` choice, `krit-project-analysis` for library/profile guards, and `krit-rule-vetting` for the FP audit before broadening.

## Qualify The Rule First

Before writing code, confirm the rule belongs in the built-in registry. The checklist in `docs/rule-scope.md` is canonical:

- enforces a documented standard (Kotlin/Android style guides, Compose/Coroutines/Jetpack guidance, JEPs, CVEs, release notes)
- detects something a reasonable team agrees is a problem
- implementable at a high `Confidence` tier
- evidence is local enough for the hot path, or declares the capability honestly

If any of these fail, it belongs as an opt-in (`DefaultActive=false`), `MaturityExperimental`, project-local, or simply outside Krit. Do not skip this.

## Pick The Base And Dispatch Shape

Choose the base by where evidence lives:

- **`FlatDispatchBase`** — structural AST checks. Declare `NodeTypes`.
- **`LineBase`** — line-oriented text checks (rare; must be lexical-state aware).
- **`ManifestBase`** — AndroidManifest.xml.
- **`ResourceBase`** — Android resources (XML).
- **`GradleBase`** — Gradle build files / settings / catalogs.

Never write a standalone `Check(file)` tree walk. Always go through the v2 dispatcher.

## Capability Declaration

Declare the narrowest capability that proves the finding. See `krit-capability-migration` for the full menu. The default-deny gates enforced by `make lint-rules` are `NeedsResolver` and `NeedsOracle` — declare them when used, omit them when not.

Other capabilities to consider: `NeedsCrossFile`, `NeedsModuleIndex`, `NeedsParsedFiles`, `NeedsManifest`, `NeedsResources`, `NeedsGradle`, `NeedsTypeInfo`. Library facts (`ctx.LibraryFacts`) need no extra `Needs` flag.

## Registry Entry

Rules live in `internal/rules/<category>.go` (or `<category>_*.go`). Register them in the matching `internal/rules/registry_<category>.go`. Fields:

- `ID`, `Category`, `Description`, `Sev`
- `NodeTypes` (for FlatDispatch) or omit (for line/manifest/resource/gradle bases)
- `Languages` — `LanguageKotlin`, `LanguageJava`, or both
- `Needs` — capabilities declared above
- `Confidence` — `ConfidenceHigh` is the bar for `DefaultActive=true`
- `Fix` — omit when no autofix; otherwise `FixCosmetic` / `FixIdiomatic` / `FixSemantic` (see `krit-autofix-safety`)
- `DefaultActive` — `false` until vetted on real corpora
- `Implementation` — pointer to the rule struct
- `Check` — the local `check` method

### Precompile Rules

Precompile rules (kotlinc-mirror diagnostics) are regular rules with two extras:

- `Level: api.LevelFunction | LevelFile | LevelModule` — scope hint for the analyzer
- `KotlincAnalog: "UNREACHABLE_CODE"` — the kotlinc diagnostic name being mirrored

Use `api.CategoryPrecompile`. They live in `internal/rules/registry_precompile_*.go` and follow every other convention in this skill. There is no K#### prefix anymore — name them by what they detect.

## Meta() And Config

If the rule has user-facing options, add a `Meta()` descriptor with the option schema and defaults. Keep schema validation and runtime matching in sync — especially for regex options and implicit anchoring. Run `make schema` to regenerate `schemas/krit-config.schema.json`.

## Fixtures

Create paired fixtures under `tests/fixtures/<category>/`:

- `positive/` — must fire
- `negative/` — must not fire (include local-lookalike negatives: shadowed identifiers, comments, KDoc, escaped strings, raw strings)
- `fixable/` — for autofix rules; each fixture has a `.expected` partner showing the fixed output

For each fixture, run focused tests first:

```bash
go test ./internal/rules/ -run TestPositiveFixtures -v
go test ./internal/rules/ -run TestNegativeFixtures -v
go test ./internal/rules/ -run TestFixableFixtures -v
```

## Java Parity

If the rule declares Java support (`Languages` includes `LanguageJava`):

1. Add at least one Java positive fixture.
2. Add at least one Java local-lookalike negative (shadowed name, nested scope, unrelated method with the same identifier).
3. Use Java AST evidence (tree-sitter Java nodes, Java imports, Java type inference). Do not rely on Kotlin-only helpers.
4. Verify Java participates in every pipeline phase the rule depends on: parse discovery, generated-source filtering, suppression indexing, dispatch, cross-file references, and module grouping.

If the rule is Kotlin-only today but the underlying problem exists in Java, file a follow-up rather than silently leaving Java out.

## Evidence Guardrails

Before considering the rule done, walk the "Rule Implementation Guardrails" checklist from `CLAUDE.md`:

- prefer flat AST, identifiers, navigation chains, imports, source-index facts over `strings.Contains` and broad regexes
- if line/text scanning is unavoidable, skip comments, KDoc, escaped strings, raw strings, Gradle string literals
- require receiver/owner proof for common method names (`System.out`, Android `Context`/`Activity`, lifecycle, DB, logging)
- stop body walks at nested functions, lambdas, anonymous functions, classes/objects, local declarations that shadow names
- walk all relevant operands/siblings/ancestors, not just the first child
- verify parser shape (`!!`, Elvis, safe calls, infix, raw strings) with focused parser/helper tests

## Validation

Iterate with focused tests, then run the full validation set before pushing:

```bash
go build -o krit ./cmd/krit/
go vet ./...
golangci-lint run ./...
make lint-rules
go test ./... -count=1
```

`golangci-lint run ./...` and `make lint-rules` are both required — `go vet` alone misses gofmt drift, unused helpers, and the capability-declaration gate. Run `make integration` before pushing if the change touches dispatch, pipeline, output, or daemon code.
