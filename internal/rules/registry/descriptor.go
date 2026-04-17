// Package registry is the foundation of the code-generated rule registry.
//
// Every rule type will eventually expose a Meta() RuleDescriptor method. A
// code generator reads those methods and emits the registration glue, config
// application, and JSON schema. This package defines the types and runtime
// used by both the generator and the generated code.
//
// The package has no dependencies on internal/rules or internal/rules/v2 —
// the dependency must flow in one direction (rules depends on registry, not
// vice versa) so the generator can run on any rule source file without
// pulling in the whole analysis graph.
package registry

// RuleDescriptor is the metadata a rule publishes via its Meta() method.
//
// A descriptor is a pure value: it contains no pointers to rule state and
// carries no runtime behavior other than the per-option Apply closures. The
// descriptor must be cheap to copy and safe to share across goroutines.
type RuleDescriptor struct {
	// ID is the stable rule identifier (matches the rule's Name()).
	ID string

	// RuleSet is the configuration group this rule belongs to
	// (e.g. "complexity", "naming", "performance").
	RuleSet string

	// Severity is "error", "warning", or "info".
	Severity string

	// Description is the human-readable rule summary.
	Description string

	// DefaultActive reports whether the rule runs by default. Rules with
	// DefaultActive == false are opt-in (must be enabled via config or
	// --all-rules).
	DefaultActive bool

	// FixLevel is "", "cosmetic", "idiomatic", or "semantic".
	// Empty string means the rule does not provide an auto-fix.
	FixLevel string

	// Confidence is the base confidence tier (0 = use family default).
	Confidence float64

	// Oracle declares when the rule needs oracle type information.
	// nil means the conservative default (always needed).
	Oracle *OracleFilter

	// Options are the configurable fields the rule exposes via YAML.
	Options []ConfigOption

	// CustomApply is an optional escape hatch for rules whose config cannot
	// be expressed as a list of Options. It runs AFTER the Options loop, so
	// it can override option-applied fields if needed. Rules that can be
	// fully expressed via Options leave this nil.
	//
	// A common use case is a rule that needs to read the whole config tree
	// (e.g. LayerDependencyViolationRule.LayerConfig) rather than a single
	// scalar key. Such hooks typically assert on a concrete ConfigSource
	// implementation (e.g. the real ConfigAdapter) and no-op on the fake
	// sources used by unit tests.
	CustomApply func(target interface{}, cfg ConfigSource)

	// SourceHash is a content hash of the rule's source, used as a
	// cache-unification key. Empty until the generator populates it.
	SourceHash string
}

// ConfigOption describes a single configurable field on a rule.
//
// The generator produces Apply closures that downcast the target interface
// to the concrete rule struct and assign the parsed value. At runtime
// ApplyConfig iterates the descriptor's options, reads each from the
// ConfigSource (primary Name first, then each alias in order), and invokes
// Apply when a value is present.
type ConfigOption struct {
	// Name is the primary YAML key for this option.
	Name string

	// Aliases are alternate YAML keys accepted for back-compat (e.g.
	// detekt's "threshold" aliasing allowedLines). Checked in order
	// after Name.
	Aliases []string

	// Type declares how the ConfigSource should read the value.
	Type OptionType

	// Default is the value used when no override is present. Retained
	// for schema generation; the runtime does not re-apply the default
	// (the rule struct literal already carries the default).
	Default interface{}

	// Description is the schema-level documentation for this option.
	Description string

	// Apply is invoked with the target rule and the parsed value when a
	// config override is present. The closure downcasts target to the
	// rule's concrete struct type and assigns the field. For OptRegex
	// the value is *regexp.Regexp (compiled via CompileAnchoredPattern);
	// for all other types the value matches Type's Go representation.
	Apply func(target interface{}, value interface{})
}

// OptionType declares the kind of value a ConfigOption carries.
type OptionType int

const (
	// OptInt is a scalar int read via ConfigSource.GetInt.
	OptInt OptionType = iota
	// OptBool is a scalar bool read via ConfigSource.GetBool.
	OptBool
	// OptString is a scalar string read via ConfigSource.GetString.
	OptString
	// OptStringList is a []string read via ConfigSource.GetStringList.
	OptStringList
	// OptRegex is a pattern string that is anchored and compiled to
	// *regexp.Regexp before being passed to Apply.
	OptRegex
)

// String returns a human-readable name for the option type. Used by the
// schema generator and by test diagnostics.
func (t OptionType) String() string {
	switch t {
	case OptInt:
		return "int"
	case OptBool:
		return "bool"
	case OptString:
		return "string"
	case OptStringList:
		return "string[]"
	case OptRegex:
		return "regex"
	default:
		return "unknown"
	}
}

// OracleFilter mirrors the v1/v2 oracle filter shape without importing
// either package. The rules layer converts between the two representations.
type OracleFilter struct {
	Identifiers []string
	AllFiles    bool
}

// MetaProvider is the interface rules satisfy once they've been migrated
// to the code-generated registry. The generator walks the rule source
// files to discover struct tags; at runtime MetaProvider is used by
// DefaultInactiveSet and other helpers that operate on descriptors.
type MetaProvider interface {
	Meta() RuleDescriptor
}
