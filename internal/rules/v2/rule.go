// Package v2 provides the unified rule interface for krit.
//
// Instead of 12+ interface types (FlatDispatchRule, LineRule, AggregateRule,
// CrossFileRule, ParsedFilesRule, ModuleAwareRule, ManifestRule, ResourceRule,
// GradleRule, TypeAwareRule, ConfidenceProvider, OracleFilterProvider, FixLevelRule),
// a single Rule struct declares its dependencies via a Capabilities bitfield and
// provides a single Check function that receives a Context carrying everything the
// rule needs.
package v2

import (
	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// Capabilities declares what the dispatcher must provide to a rule.
type Capabilities uint32

const (
	// NeedsResolver requests a TypeResolver in Context.
	NeedsResolver Capabilities = 1 << iota
	// NeedsModuleIndex requests a PerModuleIndex in Context.
	NeedsModuleIndex
	// NeedsCrossFile requests a CodeIndex in Context.
	NeedsCrossFile
	// NeedsLinePass marks this rule as a line-scanning rule (receives Lines, not nodes).
	NeedsLinePass
	// NeedsParsedFiles marks this rule as needing all parsed files (project-scope).
	NeedsParsedFiles
	// NeedsManifest marks this rule as needing AndroidManifest.xml data.
	NeedsManifest
	// NeedsResources marks this rule as needing the Android resource index.
	NeedsResources
	// NeedsGradle marks this rule as needing Gradle build config data.
	NeedsGradle
	// NeedsAggregate marks this rule as having an aggregate lifecycle
	// (Collect per node, Finalize per file, Reset between files).
	NeedsAggregate
)

// Has reports whether c includes all bits in flag.
func (c Capabilities) Has(flag Capabilities) bool {
	return c&flag == flag
}

// IsPerFile reports whether this rule runs per-file (dispatch or line pass).
// Aggregate rules are per-file (they collect during per-file walks and
// finalize after each file) so they are considered per-file here.
func (c Capabilities) IsPerFile() bool {
	return !c.Has(NeedsCrossFile) && !c.Has(NeedsModuleIndex) &&
		!c.Has(NeedsParsedFiles) && !c.Has(NeedsManifest) &&
		!c.Has(NeedsResources) && !c.Has(NeedsGradle)
}

// FixLevel indicates how safe an auto-fix is.
type FixLevel int

const (
	FixNone      FixLevel = 0
	FixCosmetic  FixLevel = 1
	FixIdiomatic FixLevel = 2
	FixSemantic  FixLevel = 3
)

func (l FixLevel) String() string {
	switch l {
	case FixCosmetic:
		return "cosmetic"
	case FixIdiomatic:
		return "idiomatic"
	case FixSemantic:
		return "semantic"
	default:
		return "none"
	}
}

// Severity levels.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// OracleFilter declares when a rule needs oracle type information.
type OracleFilter struct {
	Identifiers []string
	AllFiles    bool
}

// NeverNeedsOracle returns true when the filter declares the rule is
// purely tree-sitter and will never consult the oracle.
func (f *OracleFilter) NeverNeedsOracle() bool {
	if f == nil {
		return false
	}
	return !f.AllFiles && len(f.Identifiers) == 0
}

// Rule is the unified rule descriptor. Every analysis rule in krit is
// represented as a single Rule value. Dependencies and capabilities are
// declared via the Needs bitfield rather than through interface implementations.
type Rule struct {
	// Identity
	ID          string
	Category    string
	Description string
	Sev         Severity

	// Dispatch routing
	NodeTypes []string     // nil + no NeedsLinePass → legacy; nil + NeedsLinePass → line rule
	Needs     Capabilities // zero → no extra deps

	// Fix metadata
	Fix FixLevel // FixNone → not fixable

	// Confidence tier (0 = use family default)
	Confidence float64

	// Oracle filtering (nil = conservative AllFiles default)
	Oracle *OracleFilter

	// Check is the rule's analysis function. It receives a Context
	// populated according to the rule's Needs bitfield.
	Check func(*Context)

	// SetResolverHook is an optional callback that forwards the v1
	// dispatcher's SetResolver call to the underlying rule struct.
	// Populated by AdaptFlatDispatch/AdaptLine when the rule needs
	// resolver wiring for the captured closure to work correctly.
	SetResolverHook func(typeinfer.TypeResolver)

	// AndroidDeps carries the AndroidDataDependency bitfield (stored as
	// uint32 to avoid an import cycle with the rules package) for rules
	// that implement AndroidDependencyProvider. The rules package reads
	// this when constructing v1 compat wrappers so the
	// AndroidDependencies() method returns the same value as the
	// original rule. Zero means "not an Android data rule".
	AndroidDeps uint32

	// OriginalV1 stores a pointer to the underlying v1 rule struct, if
	// any. This lets test code and advanced callers recover the concrete
	// rule for type assertions (e.g. reading config fields after
	// ApplyConfig). Typed as interface{} to avoid an import cycle; the
	// rules package type-asserts back to its v1 rule types.
	OriginalV1 interface{}

	// Aggregate carries the collect/finalize/reset lifecycle hooks
	// for rules declared with NeedsAggregate. Non-aggregate rules
	// leave this nil.
	Aggregate *Aggregate
}

// Aggregate describes the three-phase lifecycle of an aggregate rule:
// Collect is called for each matching node during the AST walk; Finalize
// is called once per file after the walk to produce findings; Reset is
// called between files to clear any accumulated state.
type Aggregate struct {
	// Collect is invoked for each matching node. The rule accumulates
	// state internally via this callback (typically via a closure over
	// fields in its adapter).
	Collect func(ctx *Context)
	// Finalize is invoked after the walk completes for a file. It
	// should append any findings to ctx.Findings.
	Finalize func(ctx *Context)
	// Reset clears any per-file state. Called before Collect for the
	// next file.
	Reset func()
}

// Name returns the rule ID (compatibility with v1 Rule interface).
func (r *Rule) Name() string { return r.ID }

// Context carries everything a rule could need. Fields are populated
// conditionally based on the rule's declared Capabilities.
type Context struct {
	// Always available for per-file rules:
	File *scanner.File
	Node *scanner.FlatNode // nil for line-pass rules
	Idx  uint32            // flat tree index of Node (0 for line-pass rules)

	// Finding output — rules append findings here:
	Findings []scanner.Finding

	// Populated only when the rule declares NeedsResolver:
	Resolver typeinfer.TypeResolver

	// Populated only when the rule declares NeedsModuleIndex:
	ModuleIndex *module.PerModuleIndex

	// Populated only when the rule declares NeedsCrossFile:
	CodeIndex *scanner.CodeIndex

	// Populated only when the rule declares NeedsParsedFiles:
	ParsedFiles []*scanner.File

	// Populated only when the rule declares NeedsManifest:
	Manifest interface{} // *rules.Manifest (avoids import cycle)

	// Populated only when the rule declares NeedsResources:
	ResourceIndex *android.ResourceIndex

	// Populated only when the rule declares NeedsGradle:
	GradlePath    string
	GradleContent string
	GradleConfig  *android.BuildConfig
}

// Emit appends a finding to the context. This is the primary way rules
// report issues.
func (c *Context) Emit(f scanner.Finding) {
	c.Findings = append(c.Findings, f)
}

// EmitAt is a convenience for emitting a finding at a specific location.
func (c *Context) EmitAt(line, col int, msg string) {
	c.Findings = append(c.Findings, scanner.Finding{
		File:    c.File.Path,
		Line:    line,
		Col:     col,
		Message: msg,
	})
}

// Registry holds all registered v2 rules.
var Registry []*Rule

// Register adds a rule to the global v2 registry.
func Register(r *Rule) {
	if r.ID == "" {
		panic("v2.Register: rule has no ID")
	}
	if r.Description == "" {
		panic("v2.Register: rule " + r.ID + " has no description")
	}
	// Aggregate rules may omit Check (lifecycle lives on r.Aggregate).
	// All other families must supply a Check function.
	if r.Check == nil && !r.Needs.Has(NeedsAggregate) {
		panic("v2.Register: rule " + r.ID + " has no Check function")
	}
	if r.Needs.Has(NeedsAggregate) && r.Aggregate == nil {
		panic("v2.Register: rule " + r.ID + " declares NeedsAggregate but Aggregate is nil")
	}
	Registry = append(Registry, r)
}
