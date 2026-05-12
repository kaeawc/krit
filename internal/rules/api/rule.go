// Package api defines the krit rule contract: a single Rule struct that
// declares its dependencies via a Capabilities bitfield and provides a
// single Check function which receives a Context carrying everything the
// rule needs. Replaces the prior tangle of family-specific interfaces
// (FlatDispatchRule, LineRule, AggregateRule, CrossFileRule, ManifestRule,
// ResourceRule, GradleRule, TypeAwareRule, ConfidenceProvider, etc.)
// with one descriptor that the dispatcher classifies by Needs / NodeTypes.
package api

import (
	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/javafacts"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// Capabilities is a bitfield carrying both the rule's primary scope
// (which dispatcher bucket it lands in) and its orthogonal aspects
// (resolver/oracle wiring, concurrency safety, etc.).
//
// Conceptually the bits split into two groups:
//
//   - Scope bits (mutually exclusive in practice — the dispatcher
//     classifier picks at most one): NeedsCrossFile, NeedsModuleIndex,
//     NeedsParsedFiles, NeedsManifest, NeedsResources, NeedsGradle,
//     NeedsAggregate, NeedsLinePass. A rule may also leave all of these
//     unset and supply NodeTypes — that lands in the per-file AST
//     dispatch path.
//
//   - Aspect bits (orthogonal, additive): NeedsResolver, NeedsOracle,
//     NeedsConcurrent. These layer on top of any scope.
//
// New rules can also set Rule.Scope explicitly to declare the primary
// scope as a typed value; when Scope is unset the dispatcher derives it
// from the bits above. See the Scope type for the enumeration.
type Capabilities uint32

const (
	// NeedsResolver requests a TypeResolver in Context. (Aspect.)
	NeedsResolver Capabilities = 1 << iota
	// NeedsModuleIndex requests a PerModuleIndex in Context. (Scope.)
	NeedsModuleIndex
	// NeedsCrossFile requests a CodeIndex in Context. (Scope.)
	NeedsCrossFile
	// NeedsLinePass marks this rule as a line-scanning rule (receives
	// Lines, not nodes). (Scope.)
	NeedsLinePass
	// NeedsParsedFiles marks this rule as needing all parsed files
	// (project-scope). (Scope.)
	NeedsParsedFiles
	// NeedsManifest marks this rule as needing AndroidManifest.xml data.
	// (Scope.)
	NeedsManifest
	// NeedsResources marks this rule as needing the Android resource
	// index. (Scope.)
	NeedsResources
	// NeedsGradle marks this rule as needing Gradle build config data.
	// (Scope.)
	NeedsGradle
	// NeedsAggregate marks this rule as having an aggregate lifecycle
	// (Collect per node, Finalize per file, Reset between files).
	// (Scope.)
	NeedsAggregate
	// (reserved bit slot — formerly the single NeedsOracle umbrella. The
	// narrow NeedsOracle* bits below replaced it; NeedsOracle is now an
	// OR-alias of the narrow bits.)
	_
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
	// do not affect JSON / SARIF output. (Aspect.)
	NeedsConcurrent

	// Oracle fact-category bits. Each names a single class of KAA fact
	// the rule consumes. Declaring narrow bits lets the bridge skip JVM
	// extraction work no rule asked for: declaration walk, JAR closure,
	// compiler diagnostics, etc. Use the umbrella NeedsOracle only as a
	// transitional declaration when migrating off the legacy bit.

	// NeedsOracleCallTargets requests resolved overload FQN / receiver
	// type for call expressions selected by OracleCallTargetFilter.
	// Pair with a non-nil OracleCallTargets to narrow the JVM-side scan.
	NeedsOracleCallTargets
	// NeedsOracleSuspendMarkers requests the suspend / inline / operator
	// properties on the resolved callable. Independent of declaration
	// extraction.
	NeedsOracleSuspendMarkers
	// NeedsOracleExprType requests expression type / nullability at
	// selected positions (LookupExpression / ExprPositions).
	NeedsOracleExprType
	// NeedsOracleExprAnnotations requests annotation FQNs at expression
	// positions (e.g. @CheckResult on the resolved callable).
	NeedsOracleExprAnnotations
	// NeedsOracleSupertypes requests the supertype walk for declaration
	// symbols. Implies ClassShell when projected onto the declaration
	// profile.
	NeedsOracleSupertypes
	// NeedsOracleMembers requests the member list for declaration
	// symbols. Implied by NeedsOracleMemberSignatures and
	// NeedsOracleMemberAnnotations.
	NeedsOracleMembers
	// NeedsOracleMemberSignatures requests parameter / return-type
	// rendering for members. Implies NeedsOracleMembers when projected.
	NeedsOracleMemberSignatures
	// NeedsOracleClassAnnotations requests class-level annotation FQNs.
	NeedsOracleClassAnnotations
	// NeedsOracleMemberAnnotations requests member-level annotation
	// FQNs. Implies NeedsOracleMembers when projected.
	NeedsOracleMemberAnnotations
	// NeedsOracleDiagnostics requests KAA compiler diagnostics
	// (UNREACHABLE_CODE, USELESS_ELVIS, etc.). Skipped JVM-side when no
	// active rule declares this bit.
	NeedsOracleDiagnostics
	// NeedsOracleLibraryClasses requests the JAR / library closure
	// (Dependencies map) — required for resolving against types that
	// do not appear in source. Skipped JVM-side when no active rule
	// declares this bit.
	NeedsOracleLibraryClasses
)

// NeedsOracle is the back-compat umbrella: the OR of every narrow
// oracle fact bit. Rules that declare it consent to the broadest JVM
// workload. New rules should declare only the narrow bits they read in
// their Check function so the bridge can compute a tight union across
// the active rule set.
const NeedsOracle Capabilities = NeedsOracleCallTargets |
	NeedsOracleSuspendMarkers |
	NeedsOracleExprType |
	NeedsOracleExprAnnotations |
	NeedsOracleSupertypes |
	NeedsOracleMembers |
	NeedsOracleMemberSignatures |
	NeedsOracleClassAnnotations |
	NeedsOracleMemberAnnotations |
	NeedsOracleDiagnostics |
	NeedsOracleLibraryClasses

// Scope is the typed enumeration of the dispatcher buckets a rule can
// land in. Exactly one Scope applies to each rule; the dispatcher's
// classifier picks one based on Rule.Scope, falling back to a derivation
// over Capabilities and NodeTypes when Scope is ScopeUnset.
//
// Scope is intentionally separate from Capabilities: aspects like
// NeedsResolver, NeedsOracle, and NeedsConcurrent layer on top of any
// scope and remain on the Capabilities bitfield.
type Scope int8

const (
	// ScopeUnset means the rule has not declared a Scope and the
	// dispatcher should derive one from Capabilities / NodeTypes. This
	// is the zero value, so existing rule registrations remain valid
	// without changes.
	ScopeUnset Scope = iota
	// ScopePerFileNode runs during the per-file AST walk, dispatched by
	// FlatNode.Type via NodeTypes. The hot path of the analyzer.
	ScopePerFileNode
	// ScopePerFileAllNodes runs during the per-file AST walk and
	// receives every node (NodeTypes is nil and no other scope flag is
	// set).
	ScopePerFileAllNodes
	// ScopeLinePass runs once per file over file.Lines after the AST
	// walk completes.
	ScopeLinePass
	// ScopeAggregate runs per file with a Collect/Finalize/Reset
	// lifecycle. Collect is called for each matching node; Finalize
	// produces findings after the walk; Reset clears state between
	// files.
	ScopeAggregate
	// ScopeCrossFile runs at project scope after the cross-file index
	// is built; the rule receives ctx.CodeIndex.
	ScopeCrossFile
	// ScopeModuleIndex runs at project scope after the per-module
	// dependency graph is built; the rule receives ctx.ModuleIndex.
	ScopeModuleIndex
	// ScopeParsedFiles runs at project scope and receives the full
	// ctx.ParsedFiles slice without an index pass.
	ScopeParsedFiles
	// ScopeManifest runs once per parsed AndroidManifest.xml; the rule
	// receives ctx.Manifest.
	ScopeManifest
	// ScopeResource runs once per merged Android res/ ResourceIndex;
	// XML-targeted.
	ScopeResource
	// ScopeResourceSource is a Kotlin/Java source rule that consults
	// the merged Android ResourceIndex. It dispatches per-file via
	// NodeTypes but is deferred from the per-file phase because the
	// resource index is assembled later in the Android phase.
	ScopeResourceSource
	// ScopeIcons runs once per parsed Android IconIndex.
	ScopeIcons
	// ScopeGradle runs once per parsed Gradle build script; the rule
	// receives ctx.GradleConfig.
	ScopeGradle
)

// String returns a human-readable label for the Scope, used in
// diagnostics and verbose logging.
func (s Scope) String() string {
	switch s {
	case ScopePerFileNode:
		return "per-file-node"
	case ScopePerFileAllNodes:
		return "per-file-all-nodes"
	case ScopeLinePass:
		return "line-pass"
	case ScopeAggregate:
		return "aggregate"
	case ScopeCrossFile:
		return "cross-file"
	case ScopeModuleIndex:
		return "module-index"
	case ScopeParsedFiles:
		return "parsed-files"
	case ScopeManifest:
		return "manifest"
	case ScopeResource:
		return "resource"
	case ScopeResourceSource:
		return "resource-source"
	case ScopeIcons:
		return "icons"
	case ScopeGradle:
		return "gradle"
	default:
		return "unset"
	}
}

// RuleLevel categorizes a rule by the analytical scope it requires.
// Independent of dispatcher routing (Scope) and capability bits (Needs):
// Level is a coarse, filterable label for docs, dashboards, CLI filters,
// and IDE pickers. Originally introduced for the precompile taxonomy
// (see docs/precompile/taxonomy.md), but applicable to any rule.
type RuleLevel int8

const (
	// LevelUnset is the zero value: the rule has not declared a Level.
	LevelUnset RuleLevel = iota
	// LevelFunction: function-local analysis, no cross-file or external
	// resolution required.
	LevelFunction
	// LevelFile: single-file analysis, may require an in-process source
	// resolver (NeedsResolver) but no cross-file index.
	LevelFile
	// LevelModule: cross-file, source-only analysis using the project's
	// CodeIndex.
	LevelModule
	// LevelExternal: requires binary/library resolution via the JVM
	// oracle (NeedsOracle*).
	LevelExternal
	// LevelGenerated: generated sources or build-system surface
	// (annotation processors, Gradle outputs).
	LevelGenerated
	// LevelMeta: infrastructure signals (budget exceeded, oracle
	// unavailable). Exempt from category severity floors because they
	// describe analyzer state, not user defects.
	LevelMeta
)

// String returns a stable lowercase label for the level, suitable for
// CLI flags, config keys, and JSON output.
func (l RuleLevel) String() string {
	switch l {
	case LevelFunction:
		return "function"
	case LevelFile:
		return "file"
	case LevelModule:
		return "module"
	case LevelExternal:
		return "external"
	case LevelGenerated:
		return "generated"
	case LevelMeta:
		return "meta"
	default:
		return "unset"
	}
}

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

// HasAny reports whether c includes any bit in flag. Useful for
// umbrella unions like NeedsOracle, where Has would require every
// constituent bit and miss rules that declared only narrow bits.
func (c Capabilities) HasAny(flag Capabilities) bool {
	return c&flag != 0
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

// Maturity describes a rule's lifecycle stage. The default zero value is
// MaturityStable so existing rule registrations remain unchanged.
//
// Experimental rules ship dark: they are filtered out of the default-active
// set and only run when the user opts in via --experimental, the
// `experimental: true` top-level config key, --all-rules, or by naming the
// rule explicitly with --enable-rules.
//
// Deprecated rules are also default-inactive and never re-enabled by the
// experimental flag — the only way to run them is to name them explicitly
// with --enable-rules. This gives rule authors a one-release deprecation
// window without surprising default users.
type Maturity uint8

const (
	// MaturityStable is the zero value: the rule is part of the supported
	// built-in rule set and runs by default unless DefaultActive=false.
	MaturityStable Maturity = iota
	// MaturityExperimental marks a rule that has not yet soaked under
	// real-world usage. It is default-inactive and only runs when the
	// user explicitly enables experimental rules.
	MaturityExperimental
	// MaturityDeprecated marks a rule that is on its way out. It is
	// default-inactive and is NOT re-enabled by --experimental; users who
	// still want it must pass --enable-rules <ID> or set it in config.
	MaturityDeprecated
)

// String returns a human-readable label for the Maturity value.
func (m Maturity) String() string {
	switch m {
	case MaturityExperimental:
		return "experimental"
	case MaturityDeprecated:
		return "deprecated"
	default:
		return "stable"
	}
}

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

	// Aliases are legacy or alternate IDs for this rule. They do NOT
	// appear in the registry as separate rules; they only affect
	// suppression: @Suppress("<alias>") (and inline `// krit:ignore[<alias>]`)
	// silences findings emitted under the canonical ID. Use when
	// renaming a rule so existing user suppressions keep working.
	Aliases []string

	// Tags are advisory cross-cutting labels that group rules across
	// RuleSets. Unlike Category/RuleSet (which is the rule's primary
	// configuration group and is load-bearing for activation, suppression
	// keys, and option lookup), Tags are purely descriptive metadata that
	// consumers (docs, CLI filters, IDE pickers) can use to surface
	// thematic groupings without renaming a rule. Examples: "precompile"
	// for rules that approximate compiler diagnostics, "android" for
	// Android-specific checks. Tags do NOT affect dispatcher routing or
	// config activation today.
	Tags []string

	// EnabledByDefaultSince records the krit version in which this rule
	// became default-active (DefaultActive transitioned from false to
	// true). Empty string means the rule has been default-active since
	// inception, or the version was not recorded. Used by docs and
	// release-note generation; the runtime does not key behavior on it.
	EnabledByDefaultSince string

	// Deprecated, when non-nil, marks the rule as scheduled for removal.
	// Consumers (docs, output formatters, CI gates) read this to surface
	// migration guidance. The dispatcher does NOT skip deprecated rules
	// — they continue to fire so existing baselines stay valid until the
	// user migrates.
	Deprecated *Deprecation

	// Dispatch routing.
	//
	// Scope, when set, declares the rule's primary dispatcher bucket
	// directly. When ScopeUnset (the zero value), the dispatcher
	// derives the scope from the bits Needs carries plus the shape of
	// NodeTypes. Aspects (NeedsResolver, NeedsOracle, NeedsConcurrent)
	// remain on Needs and layer on top of any scope.
	Scope     Scope
	NodeTypes []string     // nil + no NeedsLinePass means every AST node; nil + NeedsLinePass means line rule
	Needs     Capabilities // zero → no extra deps

	// LexicalCalleeNames narrows call_expression dispatch to calls whose
	// syntactic callee name matches one of these names. It is intentionally
	// lexical and AST-only; semantic call-target filtering for the Kotlin
	// oracle remains in OracleCallTargets below.
	LexicalCalleeNames []string

	// Languages declares which source languages this rule applies to.
	// When nil the effective default is computed from Needs:
	//   NeedsManifest   → [LangXML]
	//   NeedsResources  → [LangXML]
	//   NeedsGradle     → [LangGradle]
	//   otherwise       → [LangKotlin]
	// The dispatcher uses RuleLanguages() to skip rules whose language
	// list does not include the current file's Language.
	Languages []scanner.Language

	// Maturity is the rule's lifecycle stage. The zero value is
	// MaturityStable. Experimental and deprecated rules are
	// default-inactive; see the Maturity type docs for the full contract.
	Maturity Maturity

	// Level categorizes the rule's analytical scope (function, file,
	// module, external, generated, meta). Filterable metadata for docs,
	// CLI filters, dashboards, and IDE pickers; the dispatcher does not
	// key behavior on it. Zero value (LevelUnset) means the rule has
	// not declared a level.
	Level RuleLevel

	// KotlincAnalog names the closest standard kotlinc diagnostic that
	// this rule approximates, e.g. "UNREACHABLE_CODE". Informational
	// only — krit is not bug-for-bug compatible with kotlinc. Empty
	// when there is no analog.
	KotlincAnalog string

	// RunAfter declares rule IDs that must run before this rule within
	// the dispatcher. The dispatcher topologically sorts active rules by
	// these constraints once at construction; rules that name a non-active
	// dependency are unconstrained (the missing dependency is silently
	// ignored). Cycles among active rules are a programmer error and
	// cause NewDispatcher to panic with the offending rule IDs.
	//
	// Use cases:
	//   - A fixable rule whose autofix output another rule reads.
	//   - Two rules emitting findings on the same node where downstream
	//     tooling expects a stable ordering.
	//
	// Most rules do not need RunAfter and should leave it nil.
	RunAfter []string

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

	// NeedsLibraryFacts declares that the rule reads Context.LibraryFacts
	// and should receive project-derived library facts instead of the
	// conservative defaults when Gradle metadata is available.
	NeedsLibraryFacts bool

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
	// and tests may type-assert it to inspect configured fields.
	//
	// Typed as any because the registry is heterogeneous: every rule has a
	// different concrete struct type, but they all share the same Rule
	// metadata shape. Consumers that need a typed pointer assert through
	// the Implementation field (e.g. r.Implementation.(*MyRule)).
	Implementation any

	// Aggregate carries the collect/finalize/reset lifecycle hooks
	// for rules declared with NeedsAggregate. Non-aggregate rules
	// leave this nil.
	Aggregate *Aggregate

	// ExprPositions optionally returns FlatNode indices whose oracle
	// expression types this rule wants resolved before dispatch runs.
	// Used by the targeted-resolution pre-pass under --depth=thorough:
	// the pipeline walks every file once, accumulates the union of
	// requested positions across rules, sends them to krit-types in a
	// single batched RPC, then injects the resulting types into the
	// oracle's expression map. Rules then query ctx.Resolver normally
	// during dispatch and find the precomputed facts via
	// CompositeResolver.ResolveByNameFlat.
	//
	// Selectors must be cheap (AST-only, no resolver calls) — they are
	// invoked for every file regardless of whether any positions match.
	// Returning nil or an empty slice skips the file. A nil ExprPositions
	// means the rule does not participate in targeted resolution.
	ExprPositions ExpressionPositionSelector

	// Descriptor metadata (formerly only on RuleDescriptor returned by
	// Meta()). Rules may set these directly on the Rule literal at
	// registration time instead of implementing MetaProvider. The
	// MetaForRule merge prefers these fields when set; absent that, it
	// falls back to the rule's MetaProvider implementation.

	// DefaultActive reports whether the rule runs by default. Rules with
	// DefaultActive == false are opt-in (must be enabled via config or
	// --all-rules).
	DefaultActive bool

	// Options are the configurable fields the rule exposes via YAML.
	Options []ConfigOption

	// CustomApply is an optional escape hatch for rules whose config cannot
	// be expressed as a list of Options. See RuleDescriptor.CustomApply for
	// the contract.
	CustomApply func(target interface{}, cfg ConfigSource)

	// LanguageSupport records per-source-language support status for a rule.
	// See RuleDescriptor.LanguageSupport.
	LanguageSupport map[string]LanguageSupport
}

// ExpressionPositionSelector returns FlatNode indices in file whose
// oracle expression type a rule wants resolved before dispatch. See
// Rule.ExprPositions for the contract and lifecycle.
type ExpressionPositionSelector func(file *scanner.File) []uint32

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

	// Populated only when the rule declares NeedsManifest. Now strongly
	// typed: the rule-facing manifest model lives in the leaf
	// internal/manifest package, so v2 can import it without creating a
	// cycle through internal/rules.
	Manifest *manifest.Manifest

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

	// Facts is the per-run shared cache of derived per-file facts
	// (imports, references, declaration summaries). Always non-nil for
	// contexts produced by the dispatcher; helpers that build their own
	// mini-contexts may leave it nil — filefacts accessors are nil-safe
	// and recompute without caching in that case.
	Facts *filefacts.Cache
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

// Register adds a rule to the global rule registry.
func Register(r *Rule) {
	if r.ID == "" {
		panic("api.Register: rule has no ID")
	}
	if r.Description == "" {
		panic("api.Register: rule " + r.ID + " has no description")
	}
	// Aggregate rules may omit Check (lifecycle lives on r.Aggregate).
	// All other families must supply a Check function.
	if r.Check == nil && !r.Needs.Has(NeedsAggregate) {
		panic("api.Register: rule " + r.ID + " has no Check function")
	}
	if r.Needs.Has(NeedsAggregate) && r.Aggregate == nil {
		panic("api.Register: rule " + r.ID + " declares NeedsAggregate but Aggregate is nil")
	}
	Registry = append(Registry, r)
}
