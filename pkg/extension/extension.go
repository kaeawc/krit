// Package extension is the stable, public surface that downstream
// projects use to register additional rules with Krit's analyzer.
//
// Krit's rule contract lives in internal/rules/api so the built-in
// rule set can iterate it freely without committing to a public
// import path. This package re-exports the subset that out-of-tree
// rule authors need, so a project that vendors or builds against
// Krit can register a rule without importing internal/.
//
// Today, registration is in-process only: a downstream project
// imports this package, calls Register inside an init() block (or at
// startup before the dispatcher is built), and the rule joins
// api.Registry alongside the built-ins. True dynamic loading
// (Go plugins, WASM rule sandboxes, sidecar protocols) is an open
// design question — see docs/external-rules.md.
//
// Example:
//
//	package myrules
//
//	import (
//	    "github.com/kaeawc/krit/pkg/extension"
//	)
//
//	func init() {
//	    extension.Register(&extension.Rule{
//	        ID:          "MyTeamRule",
//	        Category:    "myteam",
//	        Description: "Checks an internal convention.",
//	        Sev:         extension.SeverityWarning,
//	        NodeTypes:   []string{"call_expression"},
//	        Maturity:    extension.MaturityExperimental,
//	        Check: func(ctx *extension.Context) {
//	            // ...
//	        },
//	    })
//	}
//
// External rules participate in every analyzer feature on equal
// footing with built-ins: maturity gating, RunAfter ordering,
// cross-file phases, suppression, and finding output.
package extension

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

// Rule is the descriptor an external author registers. See
// internal/rules/api.Rule for the full field documentation.
type Rule = api.Rule

// Context is the per-invocation context passed to a rule's Check.
type Context = api.Context

// Severity, FixLevel, Maturity, and Capabilities mirror the api
// types so external code does not import internal/.
type (
	Severity     = api.Severity
	FixLevel     = api.FixLevel
	Maturity     = api.Maturity
	Capabilities = api.Capabilities
	Scope        = api.Scope
)

// Severity constants.
const (
	SeverityError   = api.SeverityError
	SeverityWarning = api.SeverityWarning
	SeverityInfo    = api.SeverityInfo
)

// FixLevel constants.
const (
	FixNone      = api.FixNone
	FixCosmetic  = api.FixCosmetic
	FixIdiomatic = api.FixIdiomatic
	FixSemantic  = api.FixSemantic
)

// Maturity constants.
const (
	MaturityStable       = api.MaturityStable
	MaturityExperimental = api.MaturityExperimental
	MaturityDeprecated   = api.MaturityDeprecated
)

// Capability bits commonly set by external rules. The full list lives
// on api.Capabilities; the names below cover the typical cases.
const (
	NeedsResolver    = api.NeedsResolver
	NeedsLinePass    = api.NeedsLinePass
	NeedsCrossFile   = api.NeedsCrossFile
	NeedsParsedFiles = api.NeedsParsedFiles
	NeedsConcurrent  = api.NeedsConcurrent
)

// Register adds a rule to Krit's registry. Panics with the same
// preconditions as api.Register: ID and Description are required, and
// Check (or Aggregate when NeedsAggregate is set) must be non-nil.
//
// Call this from init() in the downstream package, or from main()
// before constructing the dispatcher. The order of registration
// follows package init order; use Rule.RunAfter to declare any
// dependency on a built-in rule by ID.
func Register(r *Rule) {
	api.Register(r)
}

// RegisterAll registers every rule in rs. nil entries are ignored.
// Convenience for constructing a slice of rules in one place and
// registering them in a single call.
func RegisterAll(rs []*Rule) {
	for _, r := range rs {
		if r == nil {
			continue
		}
		api.Register(r)
	}
}
