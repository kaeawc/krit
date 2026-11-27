# Contributing to Krit

## Quick Start

```bash
make build          # Build krit, krit-lsp, krit-mcp
make test           # Run all tests
make ci             # Full CI: build + vet + test + integration + regression
make hooks          # Install the git hooks (pre-commit, pre-push) once per clone
```

## Build

```bash
make build          # Recommended (injects version via ldflags)
go build -o krit ./cmd/krit/   # Manual
go vet ./...
```

## Run Tests

```bash
make test           # All tests
make integration    # Playground + CLI + LSP + MCP integration tests
make regression     # Verify playground regression expectations
make bench          # Performance benchmarks
make watch          # Re-run tests on file changes (requires fswatch)
```

## Pre-push Hook

`make hooks` (`scripts/install-hooks.sh`) installs `.githooks/pre-push` and
`.githooks/pre-commit` as small wrappers in the clone's shared hooks
directory, so one run covers every worktree. It doesn't set
`core.hooksPath`, which would silently disable hooks you already have. An
existing hook with the same name is kept as `<name>.local` and runs first.
`scripts/install-hooks.sh --uninstall` restores it. After that, every
`git push` runs `scripts/pre-push.sh`. You can also run it any time with
`make prepush`. It checks the files your branch changes since its merge-base
with `origin/main`. Each check mirrors a CI job and runs only when the
changed files make it relevant:

| Check | Runs when | CI job it mirrors |
|---|---|---|
| `merge`: origin/main merges cleanly; the merged tree passes `go build` + `go vet` | always (build only for Go changes) | every Go job, after both PRs land |
| `gofmt`, `build` (`go build ./...`), `vet` (`go vet ./...`, compiles every `_test.go`) | Go files or go.mod change | build-verify, golangci-lint |
| `lint`: `golangci-lint run` on changed packages | Go files change | golangci-lint |
| `gosec`: the standalone `gosec` binary on changed packages, with `ci.yml`'s `-exclude` list. It ignores `//nolint:gosec`; annotate with `// #nosec G<id> -- reason` (install: `go install github.com/securego/gosec/v2/cmd/gosec@latest`) | Go files change | golangci-lint ("Run gosec") |
| `rules`: `make lint-rules` | `internal/rules/`, `internal/ruleslinter/` | build-verify |
| `oraclefp`: `go run ./internal/devtools/oraclefpcheck` | rules, oracle, FIR checks, type inference, playground | build-verify |
| `schema`: `schemas/krit-config.schema.json` matches `krit --generate-schema` | rules, schema, config, `cmd/krit/` | (generated file) |
| `gomod`: `go mod tidy -diff` | go.mod / go.sum | build |
| `test`: changed packages through `scripts/go-test.sh`, plus `./internal/rules` and `TestFirGoldenGoLines` for fixture edits, and `cmd/krit`'s `TestAndroidLintParityFixtures` (Android Lint fixtures) and `TestChangelogLintParityFixtureCounts` (those fixtures or `CHANGELOG.md`), and `internal/cache`'s `TestRulePathHeuristicsReadProjectPath` and `TestRulePathMarkersCoverRules`, which scan the rules' sources (`internal/rules/`; the markers gate also runs for the shared path classifiers it reads). A new test outside `internal/rules` that reads the rules' sources must also be planned in `test_targets`, or `TestRuleChangePlansRulesSourceGates` fails | Go files, fixtures, testdata, `CHANGELOG.md` | test |
| `actions`: `scripts/lint-actions.sh` | `.github/` | actionlint |
| `docs`: `scripts/validate_mkdocs_nav.sh`, `mkdocs build --strict` if installed | docs, mkdocs.yml, docgen | Documentation |
| `winvet`: `GOOS=windows go vet ./...` | Go files change and mingw-w64 is on PATH | windows-vet |
| `bazele2e`: `scripts/bazel-e2e-local.sh` | `bazel/`, `scripts/bazel-*`, FIR verdict merging (`internal/firchecks/{verdict,pass,coverage}`, `internal/output/fir_coverage.go`), when bazel or bazelisk, java and a krit-fir jar are present; otherwise it names the CI job | bazel-e2e |
| `jvme2e`: `tests/externalrule` + `tests/playgroundrules` | custom-rule inputs (`tests/externalrule/`, `examples/external-rule/`, `tools/krit-rule-api/`, `krit-custom-rule-plugin/`) — runs when `java`, krit-rule-api in `~/.m2` and a krit-fir jar exist, otherwise prints a loud SKIPPED line (these tests `t.Skip` silently) | external-rule-example, playground-rules |
| `warmclean`: the FIR warm-versus-clean mutation rows (`TestWarmCleanMutationFIR//^abs$`, ~150s) | cache, hashing, oracle, FIR checks, Gradle model, `tools/krit-fir/` — when `java`, a krit-fir jar for the current sources (`make fir-jar`) and kotlin-stdlib are available; skipped otherwise | warm-clean-matrix (1/2) |

JVM, Gradle and Bazel jobs are too slow for a hook. When those trees change,
the hook names the CI job to watch. Skip a check with
`KRIT_PREPUSH_SKIP=lint,test git push`. Bypass the whole hook once with
`git push --no-verify`. `scripts/pre-push.sh --dry-run` prints the plan.

When CI catches something the hook missed, open an issue labeled
`prepush-gap` with the failing job, the error line, and the local command
that would have caught it. Add the check to `scripts/pre-push.sh`, with a
fixture in `internal/devtools/prepush/testdata/plans/`.

## Maven Central in CI

GitHub runners share IP addresses, and Maven Central often answers them with
429 Too Many Requests. CI therefore tries
[Google's mirror of Maven Central](https://storage-download.googleapis.com/maven-central/index.html)
first:

- Gradle jobs use `./.github/actions/maven-mirror` after `setup-java` (and
  after `setup-gradle` when the job has it). It installs an init script that
  puts the mirror in front of the first Maven Central or plugin portal
  repository. `mavenLocal()` and included builds still win, and an artifact
  the mirror has not synced yet falls through to Maven Central on its 404.
  Add the action to any new job that runs Gradle. Release and publish
  workflows don't use it, so published artifacts resolve from Maven Central.
- Bazel jobs pass `--downloader_config=scripts/bazel-ci-downloader.cfg`,
  which tries the mirror first for every Maven Central URL Bazel downloads.
- rules_jvm_external's coursier resolution of unpinned `maven.install`
  repositories (bazel_worker_java's `bazel_worker_maven`, rules_android's
  `android_ide_common_*`) goes to Maven Central directly, and can still hit
  429. A coursier mirror (`COURSIER_MIRRORS`) does not work there:
  rules_jvm_external requires each downloaded URL to start with one of the
  install's declared repositories.

## Adding a New Rule

Before implementing, confirm the rule belongs in the built-in registry: see
[`docs/rule-scope.md`](docs/rule-scope.md) for the qualification checklist
and the rules that should live as opt-in / experimental / project-local
instead.

1. Create the rule struct in the appropriate `internal/rules/*.go` file.
2. Embed the appropriate base (`FlatDispatchBase`, `LineBase`, `ManifestBase`,
   `ResourceBase`, `GradleBase`, or a resource-specific base).
3. Register the rule in the relevant `internal/rules/registry_*.go` file with the appropriate
   v2 metadata (`NodeTypes`, `Needs`, `Languages`, `Fix`, and Android data
   dependencies where relevant).
4. If the rule has config options, add or update its `Meta()` descriptor.
   Keep metadata, defaults, and registry fields in sync with tests.
5. Rule default-activity is declared via `DefaultActive` in the checked-in
   `Meta()` descriptor.
6. Create `tests/fixtures/positive/<category>/YourRule.kt` (code that triggers the rule).
7. Create `tests/fixtures/negative/<category>/YourRule.kt` (code that does not trigger).
8. Optionally add auto-fix: populate `f.Fix` in the rule and set the v2
   registry `Fix` safety level.

Fixes must declare a safety level: `FixCosmetic`, `FixIdiomatic`, or `FixSemantic`.
Auto-fixes must produce ktfmt-compatible output.

## Project Structure

```
cmd/krit/          CLI entry point
cmd/krit-lsp/      LSP server
cmd/krit-mcp/      MCP server for AI agents
internal/rules/    Rule implementations, registry metadata, dispatcher
internal/scanner/  Tree-sitter parsing, queries, helpers
internal/lsp/      LSP protocol + server
internal/mcp/      MCP protocol + tools + prompts
internal/fixer/    Auto-fix engine (text + binary)
internal/android/  Android project analysis
internal/typeinfer/ Source-level type inference + parallel indexing
internal/oracle/   JVM-backed krit-fir type-oracle daemon
internal/module/   Gradle module discovery
tests/fixtures/    Positive, negative, and fixable fixtures
playground/        Sample Kotlin projects for integration tests
editors/           VS Code, Neovim, IntelliJ configs
docs/              MkDocs documentation site
scripts/           Build, test, install, CI scripts
tools/krit-fir/    JVM/FIR helper for type facts and compiler checks
```

## Rule Output Stability

Each rule carries a `Stability` field (`api.Stability`) that describes
its *output-shape* commitment — separate from `Maturity` (lifecycle).
Consumers of krit JSON, SARIF, baseline files, and CI gates read this
to decide whether the rule is safe to pin against:

- `StabilityFrozen` — message text, finding location, and fix range
  will not change.
- `StabilityStable` — bug-fix changes only.
- `StabilityEvolving` — shape may change between minor versions.

The default for unannotated rules is `StabilityStable` (experimental
rules default to `StabilityEvolving`). Promotions (Evolving → Stable →
Frozen) are always allowed. **Downgrading a rule from `StabilityFrozen`
to `StabilityEvolving` requires a major-version bump** — the field
exists precisely so external baselines and dashboards can rely on it.

`baseline-audit` warns when a baselined finding belongs to a rule whose
effective stability is `StabilityEvolving`.

## PR Conventions

- Keep PRs focused on a single change.
- Include positive and negative test fixtures for new rules.
- Run `make ci` before submitting (build + vet + test + integration + regression).
  The pre-push hook (`make hooks`) runs the cheap subset on every push.
- Use clear, descriptive commit messages.
- New rules must use the v2 registry and the appropriate local base.
