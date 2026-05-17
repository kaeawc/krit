# Changelog

All notable changes to Krit will be documented in this file.

## [Unreleased]

### Added
- Linux fanotify watcher backend for the daemon: filesystem-wide kernel mark
  with FAN_REPORT_DFID_NAME + open_by_handle_at path resolution.
  Auto-selected on Linux when `CAP_SYS_ADMIN` + `CAP_DAC_READ_SEARCH` are
  available (`setcap cap_sys_admin,cap_dac_read_search+ep $(command -v krit)`),
  fsnotify otherwise. Skips the per-directory inotify walk on huge repos.
- `--watch-backend` flag on `krit serve`: `auto` (default), `fsnotify`, or
  `fanotify`.
- `docs/perf.md`: catalog of every performance-tuning option, with when to
  use each and what you give up.
- `make test-fanotify` Docker target + CI job to exercise the fanotify
  path on platforms / users without the required capabilities.

### Changed
- `krit daemon: ready` startup line now reports the resolved watcher
  backend (`watcher=fsnotify` / `watcher=fanotify` / `watcher=off`).

## [0.2.0] - 2026-05-11

### Added
- Broad lint-rule coverage across detekt-style, Android Lint-style, and Krit-specific checks
- LSP server for editor diagnostics and code intelligence
- MCP server for AI agent integration
- `--diff` flag: only report findings on lines changed since a git ref (perfect for PRs)
- `--init` flag: generate starter krit.yml with recommended defaults
- `--doctor` flag: check environment (Java, config, tools, krit-types)
- `--completions` flag: print shell completions (bash, zsh, fish)
- `--warnings-as-errors` flag: elevate warning severity to error
- Per-rule `excludes` glob patterns (detekt-compatible: `**/test/**`, `**/*Test.kt`)
- BracesOnIfStatements/When `consistent` mode
- UnderscoresInNumericLiterals `acceptableLength` config (default: 5 digits)
- Colored terminal output (auto-detected, respects NO_COLOR)
- Binary autofix for image assets (WebP conversion, PNG optimization, animated detection)
- Compiler diagnostic support through the Kotlin Analysis API oracle
- Compiled tree-sitter queries for performance
- Symbol-indexed dispatch (array lookup, no string hashing)
- Parser pooling via sync.Pool with incremental reparse in LSP
- Confidence field on findings, with rule-specific confidence carried through
  the v2 registry
- GitHub Action with SARIF upload, checksum verification, and `diff` input
- Gradle plugin (check/format/baseline, reports DSL, per-source-set tasks)
- VS Code extension with binary auto-download
- Neovim and IntelliJ editor configs, Cursor MCP config
- GoReleaser with GPG signing, SBOM, SLSA provenance
- Homebrew, Scoop, winget package manager support
- One-shot install scripts (bash with gum TUI + PowerShell)
- Pre-commit hook support (.pre-commit-hooks.yaml)
- MkDocs documentation site with migration guide
- Playground projects (Kotlin web service + Android app) for integration testing
- Tests, fixtures, regression checks, and benchmarks
- Shell completions for bash, zsh, fish (embedded via go:embed)
- Version injection via ldflags (GoReleaser + Makefile)

### Fixed
- Daemon startup hang (added 30s timeout to scanner.Scan)
- EmptyFunctionBlock false positive on expression bodies
- MagicNumber duplicate reporting for typed literals
- UtilityClass false positive on interfaces
- MissingSuperCall false positive on interface overrides
- PropertyUsedBeforeDeclaration moved to node-dispatched analysis on class bodies
- ClassNaming false positive on backtick-enclosed test class names
- ElseCaseInsteadOfExhaustiveWhen requires type resolver (0 FPs without)
- MatchingDeclarationName false positive on .kts script files
- JSON fixable flag only true when finding has actual fix data
- Exit code regression: always exit 1 on findings
- ReturnCount excludeReturnFromLambda uses AST ancestry check

### Architecture
- Node-dispatched, line-pass, aggregate, and project-scope rule pipelines
- Specialized pipelines for manifest, resource, Gradle, and icon analysis
- Source-level type inference plus optional JVM-backed Kotlin Analysis API/FIR helpers
- Module-aware dead code detection with bloom filter
- Cross-file reference indexing (Kotlin + Java + XML)
- Per-rule file exclusion via glob patterns in dispatcher
- Precision-oriented validation against large Kotlin/Android codebases
