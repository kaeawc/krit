# Changelog

All notable changes to Krit will be documented in this file.

## [Unreleased]

### Added
- 480 lint rules (230 detekt + 246 AOSP Android Lint + 4 misc)
- LSP server with 11 capabilities (diagnostics, code actions, formatting, hover, symbols, definition, references, rename, completion, incremental reparse, config)
- MCP server with 8 tools, 3 prompts, 2 resources for AI agent integration
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
- Compiler diagnostic support (UNREACHABLE_CODE/USELESS_ELVIS from oracle)
- 43 compiled tree-sitter queries for performance
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
- MkDocs documentation site (22 pages) with migration guide
- Playground projects (Kotlin web service + Android app) for integration testing
- 1,136 test+benchmark functions across 17 packages
- 607 fixture files (positive + negative for all source rules)
- 25-point release checklist, regression tests, benchmark suite
- Shell completions for bash, zsh, fish (embedded via go:embed)
- Version injection via ldflags (GoReleaser + Makefile)

### Fixed
- Daemon startup hang (added 30s timeout to scanner.Scan)
- EmptyFunctionBlock false positive on expression bodies
- MagicNumber duplicate reporting for typed literals
- UtilityClass false positive on interfaces
- MissingSuperCall false positive on interface overrides
- PropertyUsedBeforeDeclaration rewritten as DispatchRule on class_body
- ClassNaming false positive on backtick-enclosed test class names
- ElseCaseInsteadOfExhaustiveWhen requires type resolver (0 FPs without)
- MatchingDeclarationName false positive on .kts script files
- JSON fixable flag only true when finding has actual fix data
- Exit code regression: always exit 1 on findings (matches detekt)
- ReturnCount excludeReturnFromLambda uses AST ancestry check

### Architecture
- 251 DispatchRules (84%), 45 LineRules (inherently line-based)
- Specialized pipelines: ManifestRule, ResourceRule, GradleRule, IconRule
- Type inference with Kotlin Analysis API oracle
- Module-aware dead code detection with bloom filter
- Cross-file reference indexing (Kotlin + Java + XML)
- Per-rule file exclusion via glob patterns in dispatcher
- 98.6% precision verified on detekt's codebase (100% with oracle)
