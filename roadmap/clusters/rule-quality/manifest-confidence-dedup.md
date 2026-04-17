# ManifestConfidenceDedup

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** low · **Default:** n/a

## What it does

Eliminates the 34× duplication of `Confidence() float64 { return 0.75 }`
and its identical doc comment across all ManifestRule implementations.
Replaces with a named constant and a default method on `ManifestBase`.

## Current cost

Every ManifestRule struct has:

```go
func (r *SomeManifestRule) Confidence() float64 { return 0.75 }
```

with an identical multi-line doc comment:

> Confidence reports a tier-2 (medium) base confidence. Android manifest
> security rule. Detection flags exported components, insecure flags, and
> overly-broad permissions via attribute presence checks on manifest
> nodes. Classified per roadmap/17.

This exact text appears 10+ times in `android_manifest_security.go`
alone, and ~34 times across all manifest rule files. Changes to the
default confidence require editing 34 places.

Additionally, confidence values across the codebase are magic numbers
with no taxonomy:
- 0.75 — most rules (dominant pattern)
- 0.95 — comments, emptyblocks, naming rules
- 0.60 — test rules

There is no explanation of why these specific values were chosen or what
the tiers mean.

Relevant files:
- `internal/rules/android_manifest_security.go`
- `internal/rules/android_manifest_features.go`
- `internal/rules/android_manifest_structure.go`
- `internal/rules/android_manifest_gradle.go`

## Proposed design

### Step 1: Named constants

```go
// internal/rules/confidence.go

// Confidence tiers classify rule detection precision.
//
// Tier 3 (high): rules with zero known false positives — purely
// structural checks where the AST guarantees correctness.
// Tier 2 (medium): rules with heuristic detection — manifest
// attribute checks, pattern matching with occasional FPs.
// Tier 1 (low): rules that require type info or cross-file context
// and may produce FPs without it.
const (
    ConfidenceHigh   = 0.95 // Tier 3: structural, no known FPs
    ConfidenceMedium = 0.75 // Tier 2: heuristic, occasional FPs
    ConfidenceLow    = 0.60 // Tier 1: needs type info, may FP
)
```

### Step 2: Default method on ManifestBase

```go
// ManifestBase already exists — add:
func (ManifestBase) Confidence() float64 { return ConfidenceMedium }
```

### Step 3: Remove per-rule Confidence() methods

Delete the 34 identical `Confidence()` methods and their doc comments
from individual manifest rules. Rules that need a non-default confidence
override the method.

### Step 4: Apply constants to other rule families

Replace bare `0.95` and `0.60` in other rules with `ConfidenceHigh`
and `ConfidenceLow`. This is opt-in per rule, not a mass rename.

## Acceptance criteria

- Zero bare `0.75` in ManifestRule `Confidence()` methods.
- The identical doc comment paragraph appears at most once (on
  `ManifestBase.Confidence()`).
- Named constants `ConfidenceHigh`, `ConfidenceMedium`, `ConfidenceLow`
  defined with doc comments explaining the tiers.
- All existing tests pass.

## Links

- Related: `roadmap/17-depth-over-breadth.md` (original confidence
  classification work)
- Related: [`ast-rewrite-audit.md`](ast-rewrite-audit.md) (overall
  rule quality audit)
