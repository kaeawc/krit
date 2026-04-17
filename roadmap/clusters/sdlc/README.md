# SDLC cluster

Features beyond per-file rule firing. Most concepts here graduate
krit from "a linter" to a broader code-intelligence tool, so each
sub-cluster also introduces a subcommand surface.

No single parent overview doc — scoped from the assistant's "what
else could krit provide" survey.

## Sub-clusters

- [`pr-workflow/`](pr-workflow/) — diff-mode reporting, blast radius,
  reviewer routing, churn-complexity risk, fail-on-new.
- [`migration/`](migration/) — codemod recipes, API migration assist,
  dead-code batch removal, rename, fixture harvesting.
- [`documentation/`](documentation/) — module READMEs, KDoc link
  validation, `@Sample` freshness, dependency graph export, codebase
  walkthrough generation.
- [`testing-infra/`](testing-infra/) — untested public API, mock
  inventory, test-to-code mapping, test selection.
- [`metrics/`](metrics/) — health score, per-module scorecard, SLO
  rules, rule-level time-series.
- [`lsp-integration/`](lsp-integration/) — inline codelens, go-to-test,
  hover docs, quick-fix previews.
- [`build-config/`](build-config/) — editorconfig drift, baseline
  drift, module template conformance, convention-plugin dead code.
