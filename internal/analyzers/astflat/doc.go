// Package astflat provides flat-AST navigation primitives that are general
// across rule and analyzer packages: call/navigation/argument decomposition,
// scope-walk helpers, condition-operand iteration. It depends only on the
// scanner package.
//
// This is the foundation tier between scanner (raw flat AST) and the
// higher-level analyzers (nullflow, future control/dataflow packages, etc.).
// Helpers that operate purely on AST shape — without semantic interpretation —
// belong here.
//
// Helpers that need rule-package context (config, metadata, registries) do
// NOT belong here. Helpers that interpret AST shape semantically (e.g.
// "is this a logging call?", "is this a Compose modifier?") belong in their
// own analyzer packages, not in astflat.
package astflat
