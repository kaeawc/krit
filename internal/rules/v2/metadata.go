package v2

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// RuleDescriptor is the metadata a rule publishes via its Meta method.
//
// A descriptor is metadata rather than rule state, and carries no runtime
// behavior other than the per-option Apply closures. Treat map and slice fields
// as immutable after construction so descriptors remain safe to copy and share.
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

	// LanguageSupport records per-source-language support status for a rule.
	// It is product/support metadata rather than dispatcher routing: Languages
	// on Rule controls where a rule runs, while LanguageSupport explains whether
	// that behavior counts as full, partial, pending, or inapplicable support.
	LanguageSupport map[string]LanguageSupport

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
}

// LanguageSupportStatus is the stable support classification for a rule or
// ruleset in a source language. These values are intended to be serialized in
// docs and tooling, so prefer adding values over renaming existing ones.
type LanguageSupportStatus string

const (
	// LanguageSupportSupported means the language path is implemented and
	// covered by evidence or fixtures.
	LanguageSupportSupported LanguageSupportStatus = "supported"
	// LanguageSupportPartial means important coverage exists, but known gaps
	// remain before the rule or ruleset can be counted as complete support.
	LanguageSupportPartial LanguageSupportStatus = "partial"
	// LanguageSupportPending means the language path has not yet been reviewed
	// or implemented.
	LanguageSupportPending LanguageSupportStatus = "pending"
	// LanguageSupportNotApplicable means the rule intentionally does not apply
	// to the language.
	LanguageSupportNotApplicable LanguageSupportStatus = "not-applicable"
	// LanguageSupportNeedsDesign means applicability is plausible, but the
	// implementation approach needs design before work can be estimated.
	LanguageSupportNeedsDesign LanguageSupportStatus = "needs-design"
)

// Valid reports whether s is one of the known support statuses.
func (s LanguageSupportStatus) Valid() bool {
	switch s {
	case LanguageSupportSupported,
		LanguageSupportPartial,
		LanguageSupportPending,
		LanguageSupportNotApplicable,
		LanguageSupportNeedsDesign:
		return true
	default:
		return false
	}
}

// LanguageSupport captures the source-of-truth support classification and the
// evidence used to justify it.
type LanguageSupport struct {
	Status   LanguageSupportStatus `json:"status" yaml:"status"`
	Reason   string                `json:"reason,omitempty" yaml:"reason,omitempty"`
	Issue    int                   `json:"issue,omitempty" yaml:"issue,omitempty"`
	Evidence []string              `json:"evidence,omitempty" yaml:"evidence,omitempty"`
	Fixtures []string              `json:"fixtures,omitempty" yaml:"fixtures,omitempty"`
}

// ConfigOption describes a single configurable field on a rule.
//
// Apply closures downcast the target interface to the concrete rule struct and
// assign the parsed value. At runtime ApplyConfig iterates the descriptor's
// options, reads each from the ConfigSource (primary Name first, then each alias
// in order), and invokes Apply when a value is present.
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

// String returns a human-readable name for the option type. Used by the schema
// generator and by test diagnostics.
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

// MetaProvider is the interface rules satisfy when they publish metadata for
// config, defaults, schema, and registry validation.
type MetaProvider interface {
	Meta() RuleDescriptor
}

// ConfigSource is the abstraction the v2 metadata runtime uses to read
// configuration values. The real implementation lives in internal/config; tests
// use FakeConfigSource in the compatibility registry package.
//
// The GetXxx methods all accept a default value and return it when the
// requested key is not present. HasKey exists so ApplyConfig can distinguish
// "present but empty list" from "not configured", which is needed to implement
// the existing "apply only if present" semantics for string-list and regex
// options.
type ConfigSource interface {
	// GetInt reads an int override. If the key is not set, def is returned.
	GetInt(ruleSet, rule, key string, def int) int
	// GetBool reads a bool override. If the key is not set, def is returned.
	GetBool(ruleSet, rule, key string, def bool) bool
	// GetString reads a string override. If the key is not set, def is returned.
	GetString(ruleSet, rule, key, def string) string
	// GetStringList reads a []string override. Returns nil when not set.
	GetStringList(ruleSet, rule, key string) []string
	// HasKey reports whether the given key is present in the config.
	HasKey(ruleSet, rule, key string) bool
	// IsRuleActive returns the explicit active override for a rule, or nil when
	// the config does not override. Non-nil true enables a default-inactive rule;
	// non-nil false disables a default-active rule.
	IsRuleActive(ruleSet, rule string) *bool
	// IsRuleSetActive returns the explicit active override for a ruleset, or nil
	// when the config does not override. Non-nil false disables every rule in the
	// set regardless of rule-level overrides.
	IsRuleSetActive(ruleSet string) *bool
}

// ApplyConfig applies a config source to a rule via its descriptor.
//
// The return value is the effective active state after ruleset + rule
// overrides:
//
//   - If IsRuleSetActive(ruleSet) is non-nil and false, the rule is inactive
//     and option overrides are NOT applied: disabling a ruleset short-circuits
//     everything else.
//
//   - Otherwise, if IsRuleActive(ruleSet, rule) is non-nil, it overrides
//     d.DefaultActive. Options are still applied — a rule-level disable does not
//     stop the option pass.
//
//   - When no active override is present, d.DefaultActive is the effective
//     state and options are applied.
//
// For each option, ApplyConfig tries the primary Name first, then each alias in
// order. When a key is found (checked via HasKey), the value is read via the
// appropriate GetXxx method and passed to opt.Apply with the rule as the
// target. For OptRegex, the raw pattern is anchored and compiled via
// CompileAnchoredPattern before being passed to Apply; an invalid pattern logs
// to stderr and leaves the target field untouched.
//
// ApplyConfig is pure: no globals are read or written. The caller owns the rule
// pointer and the config source.
func ApplyConfig(rule interface{}, d RuleDescriptor, cfg ConfigSource) (active bool) {
	if cfg == nil {
		return d.DefaultActive
	}

	// Ruleset-level disable short-circuits everything.
	if rs := cfg.IsRuleSetActive(d.RuleSet); rs != nil && !*rs {
		return false
	}

	// Start from the declared default, overriding with the rule-level setting if
	// present.
	active = d.DefaultActive
	if r := cfg.IsRuleActive(d.RuleSet, d.ID); r != nil {
		active = *r
	}

	// Apply option overrides even when a rule is disabled at the rule level, so
	// that re-enabling via a different code path sees the overrides.
	for _, opt := range d.Options {
		applyOption(rule, d, opt, cfg)
	}

	// CustomApply runs after the Options loop so it can override option-applied
	// fields when a rule has config that cannot be expressed as discrete Options.
	if d.CustomApply != nil {
		d.CustomApply(rule, cfg)
	}

	return active
}

// applyOption looks up a single option's value (trying Name then each alias in
// order) and invokes opt.Apply when a value is present. For OptRegex, the raw
// string is anchored and compiled before being passed to Apply; compile failures
// are logged and the field is left untouched.
func applyOption(rule interface{}, d RuleDescriptor, opt ConfigOption, cfg ConfigSource) {
	if opt.Apply == nil {
		return
	}

	// keys is primary + aliases, in the order we should probe them.
	keys := append([]string{opt.Name}, opt.Aliases...)

	for _, key := range keys {
		if key == "" {
			continue
		}
		if !cfg.HasKey(d.RuleSet, d.ID, key) {
			continue
		}

		switch opt.Type {
		case OptInt:
			def := 0
			if dv, ok := opt.Default.(int); ok {
				def = dv
			}
			opt.Apply(rule, cfg.GetInt(d.RuleSet, d.ID, key, def))

		case OptBool:
			def := false
			if dv, ok := opt.Default.(bool); ok {
				def = dv
			}
			opt.Apply(rule, cfg.GetBool(d.RuleSet, d.ID, key, def))

		case OptString:
			def := ""
			if dv, ok := opt.Default.(string); ok {
				def = dv
			}
			opt.Apply(rule, cfg.GetString(d.RuleSet, d.ID, key, def))

		case OptStringList:
			list := cfg.GetStringList(d.RuleSet, d.ID, key)
			if list == nil {
				// HasKey said yes but the getter returned nil — treat it as "not
				// set" so GetStringList returning nil leaves the field alone.
				continue
			}
			opt.Apply(rule, list)

		case OptRegex:
			pat := cfg.GetString(d.RuleSet, d.ID, key, "")
			if pat == "" {
				continue
			}
			compiled := CompileAnchoredPattern(d.ID, opt.Name, pat)
			if compiled == nil {
				// Bad pattern — leave the field alone.
				continue
			}
			opt.Apply(rule, compiled)
		}

		// Stop after the first key that produced a value, so that the primary
		// Name wins when both it and an alias are present.
		return
	}
}

// ApplyConfigActiveOnly mirrors ApplyConfig but skips the Options loop. Use it
// when a rule publishes a Meta descriptor but the concrete struct pointer is
// unreachable. The ruleset-disable short-circuit and the rule-level active
// override are still honored.
//
// Rules passed through this path MUST have no configurable options, since the
// Apply closures cannot run without a concrete target. ApplyConfig is the right
// choice when a concrete pointer is available.
func ApplyConfigActiveOnly(d RuleDescriptor, cfg ConfigSource) (active bool) {
	if cfg == nil {
		return d.DefaultActive
	}

	if rs := cfg.IsRuleSetActive(d.RuleSet); rs != nil && !*rs {
		return false
	}

	active = d.DefaultActive
	if r := cfg.IsRuleActive(d.RuleSet, d.ID); r != nil {
		active = *r
	}
	return active
}

// DefaultInactiveSet returns the set of rule IDs that are opt-in based on the
// given descriptor slice. The returned map is safe for the caller to mutate
// (it's not shared).
func DefaultInactiveSet(descs []RuleDescriptor) map[string]bool {
	m := make(map[string]bool, len(descs))
	for _, d := range descs {
		if !d.DefaultActive {
			m[d.ID] = true
		}
	}
	return m
}

// CompileAnchoredPattern compiles a regex pattern, anchoring it with ^ and $
// when those anchors are missing. This matches detekt's semantics (full-string
// match rather than substring match). Invalid patterns log a warning to stderr
// and return nil — callers must treat nil as "leave the existing field alone".
func CompileAnchoredPattern(ruleName, field, pattern string) *regexp.Regexp {
	compiled, err := regexp.Compile(anchoredPattern(pattern))
	if err != nil {
		fmt.Fprintf(os.Stderr, "krit: invalid regex in config %s.%s: %q: %v\n", ruleName, field, pattern, err)
		return nil
	}
	return compiled
}

// ValidateAnchoredPattern validates a regex pattern with the same implicit
// full-string anchoring used by CompileAnchoredPattern, but without logging.
func ValidateAnchoredPattern(pattern string) error {
	_, err := regexp.Compile(anchoredPattern(pattern))
	return err
}

func anchoredPattern(pattern string) string {
	anchored := pattern
	if !strings.HasPrefix(anchored, "^") {
		anchored = "^" + anchored
	}
	if !strings.HasSuffix(anchored, "$") {
		anchored = anchored + "$"
	}
	return anchored
}
