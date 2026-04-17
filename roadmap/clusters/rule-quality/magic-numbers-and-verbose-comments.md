# MagicNumbersAndVerboseComments

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** low · **Default:** n/a

## What it does

Two quick-fix cleanup passes across the codebase:

1. Replace magic numbers in rule implementations with named constants.
2. Remove verbose "what" comments that restate the next line of code,
   keeping only "why" comments.

## Magic numbers

| Location | Value | Meaning | Replacement |
|----------|-------|---------|-------------|
| `internal/rules/potentialbugs_nullsafety.go:150` | `castTarget[0] >= 'A' && castTarget[0] <= 'Z'` | Single-letter uppercase = type parameter | `isTypeParameter(castTarget)` helper |
| `internal/rules/potentialbugs_nullsafety.go:185` | `file.FlatChildCount(idx) >= 2` | Minimum children for a meaningful expression | `const minExpressionChildren = 2` |
| `internal/oracle/daemon.go:65` | `[:12]` | Hash truncation for cache key | `const cacheKeyHashLen = 12` with comment: "12 hex chars = 48 bits, collision-safe for per-repo cache keys" |
| `internal/lsp/server.go:25` | `100 * time.Millisecond` | Debounce delay | Already named `debounceDelay` — add a comment explaining why 100ms |

## Verbose comments to remove

These are in `cmd/krit/main.go` lines 350-582. They restate what
self-documenting code already says:

| Line | Comment | Why remove |
|------|---------|------------|
| ~350 | `// Load YAML configuration and apply to rules` | The function call says this |
| ~361 | `// Handle --validate-config: validate and exit` | The `if` condition says this |
| ~526 | `// Resolve cache directory and file path` | Variable names say this |
| ~550 | `// Collect files` | Function name says this |
| ~568 | `// Build disable/enable sets from CLI flags` | Code is self-evident |

**Keep** comments that explain "why" — e.g., why a particular order
matters, why a fallback exists, why a threshold was chosen.

## Process

1. Grep for bare numeric literals in rule files that lack context.
2. For each, define a named constant with a doc comment.
3. Grep `cmd/krit/main.go` for `// ` comments on lines 300-600.
4. For each, evaluate: does removing the comment make the code harder
   to understand? If not, remove it.
5. `go build && go vet && go test ./... -count=1`

## Acceptance criteria

- No bare numeric literals in rule `CheckNode` / `CheckFlatNode`
  bodies without a named constant or inline comment.
- `cmd/krit/main.go` lines 300-600 contain only "why" comments.
- All tests pass.

## Links

- Related: [`manifest-confidence-dedup.md`](manifest-confidence-dedup.md)
  (another magic-number cleanup)
