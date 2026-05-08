// Package coroutines is a rules subpackage demonstrating the per-domain
// topology described in the architecture diagnosis. Rule packages depend on:
//
//   - internal/scanner            — flat AST and finding emission
//   - internal/rules/api           — registry and dispatch metadata
//   - internal/rules/base         — BaseRule scaffolding
//   - internal/analyzers/*        — shared analyzer primitives
//   - internal/typeinfer          — source-level type resolution (when needed)
//
// They MUST NOT import the parent internal/rules package; the parent imports
// them via blank import for side-effect registration. Cross-domain imports
// between sibling rule packages are also forbidden — promote shared helpers
// to internal/analyzers/* instead.
//
// Rules in this package register themselves in init(). The first argument to
// api.Register binds the rule's metadata; the Implementation field carries
// the rule struct so v2 can later look up Meta()/IsFixable() via type
// assertion.
package coroutines
