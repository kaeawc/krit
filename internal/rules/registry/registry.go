package registry

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ConfigSource is the abstraction the registry runtime uses to read
// configuration values. The real implementation lives in internal/config;
// tests use FakeConfigSource in this package.
//
// The GetXxx methods all accept a default value and return it when the
// requested key is not present. HasKey exists so ApplyConfig can
// distinguish "present but empty list" from "not configured", which is
// needed to implement the existing "apply only if present" semantics for
// string-list and regex options.
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
	// IsRuleActive returns the explicit active override for a rule, or
	// nil when the config does not override. Non-nil true enables a
	// default-inactive rule; non-nil false disables a default-active rule.
	IsRuleActive(ruleSet, rule string) *bool
	// IsRuleSetActive returns the explicit active override for a ruleset,
	// or nil when the config does not override. Non-nil false disables
	// every rule in the set regardless of rule-level overrides.
	IsRuleSetActive(ruleSet string) *bool
}

// ApplyConfig applies a config source to a rule via its descriptor.
//
// The return value is the effective active state after ruleset + rule
// overrides:
//
//   - If IsRuleSetActive(ruleSet) is non-nil and false, the rule is
//     inactive and option overrides are NOT applied. This mirrors the
//     `continue` branch in the legacy internal/rules/config.go#ApplyConfig
//     (lines 47–51): disabling a ruleset short-circuits everything else.
//
//   - Otherwise, if IsRuleActive(ruleSet, rule) is non-nil, it overrides
//     d.DefaultActive. Options are still applied — a rule-level disable
//     does not stop the option pass.
//
//   - When no active override is present, d.DefaultActive is the
//     effective state and options are applied.
//
// For each option, ApplyConfig tries the primary Name first, then each
// alias in order. When a key is found (checked via HasKey), the value is
// read via the appropriate GetXxx method and passed to opt.Apply with the
// rule as the target. For OptRegex, the raw pattern is anchored and
// compiled via CompileAnchoredPattern before being passed to Apply; an
// invalid pattern logs to stderr and leaves the target field untouched.
//
// ApplyConfig is pure: no globals are read or written. The caller owns
// the rule pointer and the config source.
func ApplyConfig(rule interface{}, d RuleDescriptor, cfg ConfigSource) (active bool) {
	if cfg == nil {
		return d.DefaultActive
	}

	// Ruleset-level disable short-circuits everything, matching the
	// legacy `continue` in internal/rules/config.go.
	if rs := cfg.IsRuleSetActive(d.RuleSet); rs != nil && !*rs {
		return false
	}

	// Start from the declared default, overriding with the rule-level
	// setting if present.
	active = d.DefaultActive
	if r := cfg.IsRuleActive(d.RuleSet, d.ID); r != nil {
		active = *r
	}

	// Apply option overrides. Note: the legacy code applies options even
	// when a rule is disabled at the rule level (so that re-enabling via a
	// different code path would see the overrides). We preserve that.
	for _, opt := range d.Options {
		applyOption(rule, d, opt, cfg)
	}

	// CustomApply runs after the Options loop so it can override
	// option-applied fields when a rule has config that cannot be expressed
	// as discrete Options (e.g. LayerDependencyViolationRule, which reads
	// the whole config tree to build its LayerConfig).
	if d.CustomApply != nil {
		d.CustomApply(rule, cfg)
	}

	return active
}

// applyOption looks up a single option's value (trying Name then each
// alias in order) and invokes opt.Apply when a value is present. For
// OptRegex, the raw string is anchored and compiled before being passed
// to Apply; compile failures are logged and the field is left untouched.
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
				// HasKey said yes but the getter returned nil — treat
				// as "not set" to match the legacy behavior where
				// GetStringList returning nil leaves the field alone.
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

		// Stop after the first key that produced a value, so that the
		// primary Name wins when both it and an alias are present.
		return
	}
}

// ApplyConfigActiveOnly mirrors ApplyConfig but skips the Options loop.
// Use it when a rule publishes a Meta() descriptor but the concrete
// struct pointer is unreachable (e.g. an adapter-wrapped rule that dropped
// its OriginalV1 pointer). The ruleset-disable short-circuit and the
// rule-level active override are still honored.
//
// Rules passed through this path MUST have no configurable options, since
// the Apply closures cannot run without a concrete target. ApplyConfig is
// the right choice when a concrete pointer is available.
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

// DefaultInactiveSet returns the set of rule IDs that are opt-in based on
// the given descriptor slice. The returned map is safe for the caller to
// mutate (it's not shared).
//
// This replaces the hand-maintained internal/rules/defaults.go map: every
// rule publishes a descriptor and the set is derived directly from the
// DefaultActive field.
func DefaultInactiveSet(descs []RuleDescriptor) map[string]bool {
	m := make(map[string]bool, len(descs))
	for _, d := range descs {
		if !d.DefaultActive {
			m[d.ID] = true
		}
	}
	return m
}

// CompileAnchoredPattern compiles a regex pattern, anchoring it with ^ and
// $ when those anchors are missing. This matches detekt's semantics (full-
// string match rather than substring match) and mirrors the legacy
// compilePattern helper in internal/rules/config.go. Invalid patterns
// log a warning to stderr and return nil — callers must treat nil as
// "leave the existing field alone".
//
// Exposed so generated code and migrated rules can share a single
// implementation without importing internal/rules.
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
