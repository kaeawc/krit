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
	"github.com/kaeawc/krit/internal/javafacts"
	"github.com/kaeawc/krit/internal/librarymodel"
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
	// NeedsOracle marks this rule as requiring the JVM oracle
	// (krit-types) for accurate analysis. Use this only for KAA-only
	// facts such as resolved call targets, dependency annotations,
	// compiler diagnostics, suspend-call identity, or declaration data.
	// Source-level type facts should use NeedsResolver instead.
	NeedsOracle
	// NeedsConcurrent marks this rule as safe to execute in parallel
	// across worker goroutines at a phase boundary. The dispatcher
	// hands each concurrent rule its own Context carrying a worker-
	// local FindingCollector; collectors are serially merged into the
	// phase's output after all workers stop appending.
	//
	// Rules declaring this bit must not rely on package-level mutable
	// state or shared maps. They read the phase's immutable inputs
	// (CodeIndex, ParsedFiles, ModuleIndex) and emit only through
	// ctx.Emit / ctx.EmitAt. Finding order is recovered by the phase
	// owner after the merge (SortByFileLine), so worker interleavings
	// do not affect JSON / SARIF output.
	NeedsConcurrent
)

// NeedsTypeInfo is a source type-information alias for NeedsResolver only:
// rules that need KAA must declare NeedsOracle or explicit oracle metadata
// (Oracle, OracleCallTargets, OracleDeclarationNeeds, diagnostics). This keeps
// source-level AT/typeinfer rules from accidentally widening the Kotlin
// Analysis API workload.
//
// Prefer NeedsResolver for new rules unless the implementation consumes
// KAA-only facts.
const NeedsTypeInfo Capabilities = NeedsResolver

// TypeInfoBackend is a per-rule hint telling the dispatcher which
// type-information backend to prefer when both the in-process resolver
// and the JVM oracle are wired. The zero value is PreferAny — rules
// that do not care get the composite resolver (oracle-first) just as
// they did before the hint was introduced.
//
// Decision matrix for rule authors:
//
//	PreferResolver — cheap source-level lookups: imports, local
//	  declarations, type hierarchy/nullability. No resolved overload,
//	  external annotation, compiler diagnostic, or suspend-call truth.
//	PreferOracle   — requires dependency metadata the in-process
//	  resolver cannot see: full FQN resolution, call-target / overload
//	  resolution, annotation-argument lookups against library types.
//	PreferAny      — don't care / equally well served (default).
type TypeInfoBackend uint8

const (
	// PreferAny leaves routing to the dispatcher — the composite
	// resolver is wired and backends answer in oracle > source order.
	PreferAny TypeInfoBackend = iota
	// PreferResolver asks the dispatcher to wire the in-process
	// source-level resolver only. Cheaper, avoids oracle IPC.
	PreferResolver
	// PreferOracle asks the dispatcher to route through the oracle-
	// backed composite. Strictly equivalent to PreferAny when the
	// composite is wired today; declared explicitly so rule authors
	// can document intent and the linter can cross-check usage.
	PreferOracle
)

// TypeInfoHint groups the per-rule type-information routing hints on
// Rule. The zero value (PreferAny, Required=false) is backwards
// compatible — existing rules see identical behaviour.
type TypeInfoHint struct {
	// PreferBackend names the backend the rule would rather use when
	// the dispatcher has both wired.
	PreferBackend TypeInfoBackend
	// Required controls what happens when PreferBackend is set but
	// the preferred backend is NOT available:
	//
	//   false (default): skip the rule silently — the rule author
	//     asserts that running against the non-preferred backend
	//     would produce wrong or misleading findings.
	//   true           : fall through to whatever backend IS wired;
	//     the preference was a perf hint, not a correctness lever.
	//
	// PreferAny ignores Required — any wired backend satisfies the
	// rule.
	Required bool
}

// JavaFactProfile declares optional javac-backed facts a Java-aware rule can
// consume. The pipeline may use this profile to run the Java helper for a
// narrow set of sites; rules must still keep conservative source-only
// fallbacks when facts are unavailable.
type JavaFactProfile struct {
	ReceiverTypesForCallees []string
	ReturnTypesForCallees   []string
	ClassSupertypes         []string
	Annotations             []string
	DeclarationNames        []string
}

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

// OracleDeclarationProfile narrows which KAA symbol fields the JVM extracts
// for this rule. The pipeline takes the union across all active rules before
// passing --declaration-profile to krit-types; a nil value means "no opinion"
// (the rule does not constrain extraction). If every active oracle rule
// supplies a non-nil profile, the JVM skips fields outside the union and may
// run significantly faster.
//
// Rules that only use LookupCallTarget or LookupExpression (expression-level
// APIs) and never walk the declarations map should set an empty struct
// OracleDeclarationProfile{} — this signals the rule contributes no
// declaration interest and allows the union to stay narrow.
//
// Fields mirror oracle.DeclarationProfile (no import needed here; the bridge
// in oracle_filter_bridge.go converts).
type OracleDeclarationProfile struct {
	ClassShell              bool
	Supertypes              bool
	ClassAnnotations        bool
	Members                 bool
	MemberSignatures        bool
	MemberAnnotations       bool
	SourceDependencyClosure bool
}

// OracleCallTargetFilter declares which call targets a rule may ask the
// JVM oracle to resolve via LookupCallTarget. Nil means the rule does not
// contribute call-target interest. AllCalls is conservative and disables
// callee-name filtering when the rule is enabled.
type OracleCallTargetFilter struct {
	AllCalls bool
	// DiscardedOnly means the rule only needs call targets for calls whose
	// result is used as a standalone statement.
	DiscardedOnly bool
	// TargetFQNs are fully-qualified callable targets the rule may query.
	// The Go bridge derives their simple names for the JVM-side lexical
	// callee filter.
	TargetFQNs []string
	// CalleeNames are lexical callee names the rule may query directly.
	CalleeNames []string
	// LexicalHintsByCallee optionally adds cheap file-level evidence that
	// must be present before the JVM resolves broad call names. Absence keeps
	// name-only behavior for that callee.
	LexicalHintsByCallee map[string][]string
	// LexicalSkipByCallee optionally declares cheap receiver evidence where
	// the Go rule can classify the call structurally and does not need a JVM
	// call target for that site.
	LexicalSkipByCallee map[string][]string
	// AnnotatedIdentifiers asks the bridge to derive CalleeNames from
	// source declarations annotated with these annotation identifiers.
	// This is for rules that call LookupCallTarget only so they can call
	// LookupAnnotations on annotated symbols; it avoids resolving every
	// call expression when the annotated declaration names are knowable
	// from raw source.
	AnnotatedIdentifiers []string
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
	NodeTypes []string     // nil + no NeedsLinePass means every AST node; nil + NeedsLinePass means line rule
	Needs     Capabilities // zero → no extra deps

	// Languages declares which source languages this rule applies to.
	// When nil the effective default is computed from Needs:
	//   NeedsManifest   → [LangXML]
	//   NeedsResources  → [LangXML]
	//   NeedsGradle     → [LangGradle]
	//   otherwise       → [LangKotlin]
	// The dispatcher uses RuleLanguages() to skip rules whose language
	// list does not include the current file's Language.
	Languages []scanner.Language

	// Fix metadata
	Fix FixLevel // FixNone → not fixable

	// Confidence tier (0 = use family default)
	Confidence float64

	// Oracle filtering (nil = conservative AllFiles default)
	Oracle *OracleFilter

	// OracleCallTargets optionally narrows JVM call-target resolution to
	// lexical callees this rule can consume. Broad consumers set
	// AllCalls=true, which disables the optimization for the active rule set.
	OracleCallTargets *OracleCallTargetFilter

	// OracleDeclarationNeeds, when non-nil, declares which declaration
	// fields this rule reads from the JVM oracle. The pipeline unions these
	// across all active rules to compute the --declaration-profile flag
	// passed to krit-types. A nil value means the rule has not opted in to
	// narrowing (conservative: treated as needing all fields for the union).
	// Rules that only use expression-level APIs (LookupCallTarget,
	// LookupExpression) and never touch the declarations map should set
	// &OracleDeclarationProfile{} (empty, no fields needed).
	OracleDeclarationNeeds *OracleDeclarationProfile

	// TypeInfo carries per-rule routing hints for type-information
	// lookups. Zero value (PreferAny, Required=false) is backwards
	// compatible — the dispatcher wires the composite resolver just
	// as it did before the hint existed. See TypeInfoHint.
	TypeInfo TypeInfoHint

	// JavaFacts optionally requests javac-backed facts for Java files.
	// This does not make javac mandatory; unavailable facts are a warning
	// and rules must fall back to source AST/index evidence.
	JavaFacts *JavaFactProfile

	// Check is the rule's analysis function. It receives a Context
	// populated according to the rule's Needs bitfield. Rules report
	// findings by calling ctx.Emit or ctx.EmitAt.
	Check func(*Context)

	// AndroidDeps carries the AndroidDataDependency bitfield (stored as
	// uint32 to avoid an import cycle with the rules package) for Android
	// project-data rules. Zero means "not an Android data rule".
	AndroidDeps uint32

	// Implementation stores the concrete rule instance captured by this v2
	// registration. Config descriptors apply option overrides to this value,
	// and tests may type-assert it to inspect configured fields. Typed as
	// interface{} to avoid an import cycle with the rules package.
	Implementation interface{}

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
	// should report findings via ctx.Emit / ctx.EmitAt.
	Finalize func(ctx *Context)
	// Reset clears any per-file state. Called before Collect for the
	// next file.
	Reset func()
}

// Name returns the rule ID.
func (r *Rule) Name() string { return r.ID }

// Context carries everything a rule could need. Fields are populated
// conditionally based on the rule's declared Capabilities.
type Context struct {
	// Always available for per-file rules:
	File *scanner.File
	Node *scanner.FlatNode // nil for line-pass rules
	Idx  uint32            // flat tree index of Node (0 for line-pass rules)

	// Rule is the rule whose Check is currently executing. Populated by
	// the dispatcher before invoking Check so Emit can stamp
	// Rule/RuleSet/Severity/Confidence defaults without the rule body
	// having to know them.
	Rule *Rule

	// DefaultConfidence is the family-level fallback confidence applied
	// to findings emitted through Emit when the rule doesn't set its own
	// Confidence. Set by the dispatcher (0.95 node-dispatch, 0.75 line).
	DefaultConfidence float64

	// Collector receives findings written via Emit/EmitAt in columnar form.
	// Must be set before Check is called.
	Collector *scanner.FindingCollector

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

	// Populated only when the rule declares AndroidDepIcons:
	IconIndex *android.IconIndex

	// Populated only when the rule declares NeedsGradle:
	GradlePath    string
	GradleContent string
	GradleConfig  *android.BuildConfig

	// Populated for Java source files with cheap source-level facts
	// derived from the parsed Java file. JavaSourceIndex is populated
	// when the dispatcher has a project/source file set; single-file
	// dispatch receives a one-file index.
	JavaFacts       *javafacts.JavaFileFacts
	JavaSourceIndex *javafacts.SourceIndex
	// Populated when at least one enabled Java rule requested optional
	// javac-backed semantic facts and the helper was available.
	JavaSemanticFacts *javafacts.Facts

	// Project-wide library and platform facts derived from Gradle where
	// available. Rules should use this instead of baking library-version
	// assumptions directly into AST heuristics.
	LibraryFacts *librarymodel.Facts
}

// Emit reports a finding. The finding is stamped with rule metadata and
// written to the Collector.
func (c *Context) Emit(f scanner.Finding) {
	c.stamp(&f)
	c.Collector.Append(f)
}

// EmitAt is a convenience for emitting a finding at a specific location.
func (c *Context) EmitAt(line, col int, msg string) {
	c.Emit(scanner.Finding{
		Line:    line,
		Col:     col,
		Message: msg,
	})
}

// stamp fills in Rule/RuleSet/Severity/File/Confidence fields that the
// rule body didn't populate, using Context.Rule / Context.File /
// Context.DefaultConfidence as the source of truth.
func (c *Context) stamp(f *scanner.Finding) {
	if c.Rule != nil {
		if f.Rule == "" {
			f.Rule = c.Rule.ID
		}
		if f.RuleSet == "" {
			f.RuleSet = c.Rule.Category
		}
		if f.Severity == "" {
			f.Severity = string(c.Rule.Sev)
		}
		if f.Confidence == 0 {
			if c.Rule.Confidence != 0 {
				f.Confidence = c.Rule.Confidence
			} else {
				f.Confidence = c.DefaultConfidence
			}
		}
	}
	if c.File != nil && f.File == "" {
		f.File = c.File.Path
	}
}

// RuleLanguages returns the languages a rule applies to, falling back to
// the sensible default derived from Needs when Languages is nil.
func RuleLanguages(r *Rule) []scanner.Language {
	if r == nil {
		return nil
	}
	if len(r.Languages) > 0 {
		return r.Languages
	}
	switch {
	case r.Needs.Has(NeedsManifest):
		return []scanner.Language{scanner.LangXML}
	case r.Needs.Has(NeedsResources):
		return []scanner.Language{scanner.LangXML}
	case r.Needs.Has(NeedsGradle):
		return []scanner.Language{scanner.LangGradle}
	default:
		return []scanner.Language{scanner.LangKotlin}
	}
}

// RuleAppliesToLanguage reports whether a rule should run on a file of
// the given language. Used by the dispatcher to filter rules per file.
func RuleAppliesToLanguage(r *Rule, lang scanner.Language) bool {
	for _, l := range RuleLanguages(r) {
		if l == lang {
			return true
		}
	}
	return false
}

func NeedsJavaFacts(rules []*Rule) bool {
	for _, r := range rules {
		if r != nil && r.JavaFacts != nil && RuleAppliesToLanguage(r, scanner.LangJava) {
			return true
		}
	}
	return false
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
