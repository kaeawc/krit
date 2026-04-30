# UnifiedRuleInterface

**Cluster:** [core-infra](README.md) · **Status:** ✅ shipped ·
**Severity:** n/a (infra) · **Default:** n/a

## What shipped

Krit now uses a single declarative `v2.Rule` model for rule execution. A rule
declares its ID, category, severity, dispatch nodes, languages, capabilities,
fix level, confidence, and callback in checked-in Go source. The dispatcher
uses that metadata to route work through the correct AST, line, project,
Android, Gradle, type-resolution, oracle, and aggregate phases.

## Current architecture

- `internal/rules/v2/rule.go` defines `Rule`, `Context`, `Capabilities`,
  `FixLevel`, `Severity`, oracle metadata, and aggregate lifecycle hooks.
- `internal/rules/registry_*.go` files register rules directly with
  `v2.Register`.
- `internal/rules/v2dispatcher.go` classifies v2 rules by `NodeTypes`,
  `Languages`, and `Needs`, then runs the single-pass flat-AST dispatcher.
- `internal/pipeline/` builds the Android, Gradle, cross-file, parsed-file,
  module, type-resolution, and oracle inputs requested by rule capabilities.
- CLI, LSP, MCP, schema, config, output, and autofix paths all read from the
  v2 registry.

## Acceptance criteria

- ✅ Rule execution is driven by the v2 registry.
- ✅ Per-file source rules run through the v2 dispatcher.
- ✅ Android manifest, resource, Gradle, icon, cross-file, parsed-file,
  module-aware, resolver, oracle, and aggregate rules declare capabilities
  instead of depending on entry-point-specific wiring.
- ✅ Duplicate registrations are rejected by invariant tests.
- ✅ Registered rules must have executable v2 implementations.
- ✅ No registered rules rely on empty callbacks.
- ✅ Local validation passes with `go build -o krit ./cmd/krit/ && go vet ./...`
  and `go test ./... -count=1`.

## Follow-up work

The rule execution migration is complete. Remaining core-infra work should be
tracked as separate projects, not as migration cleanup:

- Continue reducing generated metadata files when a replacement source format is
  ready.
- Keep rule author ergonomics focused on the local rule file, v2 registration,
  metadata descriptor, and fixtures.
- Consider moving the inventory parser from Python into `krit-gen` if generator
  maintenance becomes a bottleneck.
