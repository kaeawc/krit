package rules

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules/registry"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

// Phase 3B of the CodegenRegistry migration: prove that the generated
// Meta() descriptors (fed through registry.ApplyConfig) produce behaviorally
// identical rule state to the legacy 631-line applyRuleConfig switch for
// every rule that has a Meta() method.
//
// The harness in this file runs a matrix of YAML-equivalent configurations
// against every migrated rule, applies both paths to isolated rule clones,
// and asserts the resulting rule struct + active flag are deep-equal. A
// divergence here is the signal that phase 3C (deletion of applyRuleConfig)
// is not yet safe for the affected rule.
//
// Matrix (per rule):
//
//	1.  no config at all (defaults only)
//	2.  rule explicitly disabled (active: false)
//	3.  rule explicitly enabled (active: true)
//	4.  ruleset disabled — options present but must not apply
//	5.  ruleset disabled + rule enable — ruleset wins (legacy semantics)
//	6.  each option set to a non-default value (one case per option)
//	7.  alias-keyed option — set only the alias
//	8.  both primary and alias set — primary wins
//
// Cases 6-8 are skipped for rules without options.
//
// Rules whose Meta() declares an option with Apply=nil (currently only
// LayerDependencyViolation.LayerConfig) cannot be exercised via the
// registry path; they are recorded as SKIP with a reason string, not as a
// failure.

// TestConfigParity is the master parity test. It iterates the global
// Registry, filters to rules with Meta() (where Meta().ID matches the
// registered name), and runs the matrix against each.
func TestConfigParity(t *testing.T) {
	// Collect the migrated-rule set.
	migrated := collectMigratedRules(t)

	// Per-rule result buckets.
	var (
		passes   []string
		failures []string
		skips    []string
	)

	for _, m := range migrated {
		m := m
		// Use sub-tests so diagnostic output names the offending rule.
		t.Run(m.name, func(t *testing.T) {
			result := runParityMatrix(t, m)
			switch result.kind {
			case parityPass:
				passes = append(passes, m.name)
			case paritySkip:
				skips = append(skips, fmt.Sprintf("%s (%s)", m.name, result.reason))
			case parityFail:
				failures = append(failures, fmt.Sprintf("%s: %s", m.name, result.reason))
			}
		})
	}

	// Summary row for the human reading -v output.
	t.Logf("parity summary: %d migrated rules, %d passes, %d failures, %d skips",
		len(migrated), len(passes), len(failures), len(skips))
	if len(skips) > 0 {
		sort.Strings(skips)
		t.Logf("skipped rules:\n  %s", strings.Join(skips, "\n  "))
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		t.Errorf("parity failures:\n  %s", strings.Join(failures, "\n  "))
	}
}

// TestConfigParity_AliasRegistrations guards the invariant that the 4
// alias-registered rules skip the Meta() path (their ID != Name()). Phase
// 3C must preserve the legacy switch for them.
func TestConfigParity_AliasRegistrations(t *testing.T) {
	// These are the known alias pairs from android_gradle.go:
	//   DynamicVersion          -> GradleDynamicVersion
	//   NewerVersionAvailable   -> GradleDependency
	//   StringInteger           -> StringShouldBeInt
	//   GradlePluginCompatibility -> GradleCompatible
	expectedAliases := map[string]string{
		"GradleDynamicVersion": "DynamicVersion",
		"GradleDependency":     "NewerVersionAvailable",
		"StringShouldBeInt":    "StringInteger",
		"GradleCompatible":     "GradlePluginCompatibility",
	}

	foundAliases := map[string]string{}
	for _, r := range v2.Registry {
		concrete := r.OriginalV1
		mp, ok := concrete.(registry.MetaProvider)
		if !ok {
			continue
		}
		meta := mp.Meta()
		if meta.ID != r.ID {
			foundAliases[r.ID] = meta.ID
		}
	}

	if !reflect.DeepEqual(foundAliases, expectedAliases) {
		t.Errorf("alias set diverged from expected\n  want: %v\n  got:  %v",
			expectedAliases, foundAliases)
	}

	// Now run ApplyConfigViaRegistry and verify each alias is flagged
	// Migrated=false.
	results := ApplyConfigViaRegistry(config.NewConfig())
	byName := make(map[string]RegistryApplyResult, len(results))
	for _, r := range results {
		byName[r.Name] = r
	}
	for aliasName := range expectedAliases {
		res, ok := byName[aliasName]
		if !ok {
			t.Errorf("alias %s not present in ApplyConfigViaRegistry results", aliasName)
			continue
		}
		if res.Migrated {
			t.Errorf("alias %s was Migrated=true, want false (alias must fall back to legacy)", aliasName)
		}
	}
}

// --- migrated rule collection ------------------------------------------

type migratedRule struct {
	// name is the registered rule name.
	name string

	// meta is the descriptor (cached to avoid repeated Meta() calls).
	meta registry.RuleDescriptor

	// template is the rule pointer from the Registry. Used as the source
	// for cloning fresh instances per test case.
	template interface{}
}

// collectMigratedRules walks the global Registry and returns one entry per
// rule that implements registry.MetaProvider AND whose Meta().ID matches
// the registered Name(). Alias-only registrations are excluded — they
// cannot be exercised through the registry path.
func collectMigratedRules(t *testing.T) []migratedRule {
	t.Helper()
	seen := make(map[string]bool)
	var out []migratedRule
	for _, r := range v2.Registry {
		name := r.ID
		concrete := r.OriginalV1
		mp, ok := concrete.(registry.MetaProvider)
		if !ok {
			continue
		}
		meta := mp.Meta()
		if meta.ID != name {
			// Alias registration — skip; Meta() represents the primary ID.
			continue
		}
		if seen[name] {
			// Defensive: two Registry entries under the same name.
			continue
		}
		seen[name] = true
		out = append(out, migratedRule{
			name:     name,
			meta:     meta,
			template: concrete,
		})
	}
	// Deterministic order for sub-test names.
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// cloneRule creates a fresh instance of the rule struct by reflecting over
// the registered template. The copy is a shallow clone of the struct value
// (slices and maps share backing storage) — adequate because every case in
// our matrix mutates only scalar fields or replaces slice fields wholesale.
// For slice fields we do a one-level copy to avoid cross-test bleed when
// a case replaces a slice via the legacy path.
func cloneRule(template interface{}) interface{} {
	tval := reflect.ValueOf(template)
	if tval.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("cloneRule: template is not a pointer: %T", template))
	}
	elem := tval.Elem()
	if elem.Kind() != reflect.Struct {
		panic(fmt.Sprintf("cloneRule: template is not a struct pointer: %T", template))
	}

	fresh := reflect.New(elem.Type())
	fresh.Elem().Set(elem)

	// Duplicate slices so a mutation in one clone can't leak into another.
	for i := 0; i < fresh.Elem().NumField(); i++ {
		f := fresh.Elem().Field(i)
		if !f.CanSet() {
			continue
		}
		if f.Kind() == reflect.Slice && !f.IsNil() {
			dup := reflect.MakeSlice(f.Type(), f.Len(), f.Len())
			reflect.Copy(dup, f)
			f.Set(dup)
		}
	}
	return fresh.Interface()
}

// --- matrix runner -----------------------------------------------------

type parityKind int

const (
	parityPass parityKind = iota
	parityFail
	paritySkip
)

type parityResult struct {
	kind   parityKind
	reason string
}

// knownParityDivergences lists rules whose Meta() descriptor cannot yet
// reproduce the legacy behavior. Keeping the list here rather than suppressing
// failures silently makes it visible to reviewers and ensures the test still
// runs the matrix.
//
// As of Phase 3C the map is empty: the four rules previously listed here
// (ForbiddenImport's dual-field write, LayerDependencyViolation's
// whole-config read, NewerVersionAvailable's []string → []libMinVersion
// transform, and PublicToInternalLeakyAbstraction's int-percent →
// float-fraction transform) have all been moved to hand-written Meta()
// files (meta_*.go in internal/rules). Parity is 100%.
var knownParityDivergences = map[string]string{}

// runParityMatrix exercises the full case matrix against a single rule.
// Any failure is captured into t and reflected in the returned result.
func runParityMatrix(t *testing.T, m migratedRule) parityResult {
	if reason, ok := knownParityDivergences[m.name]; ok {
		return parityResult{kind: paritySkip, reason: "KNOWN_DIVERGENCE: " + reason}
	}

	// Skip rules whose only option has Apply=nil (descriptor-unsupported type).
	// LayerDependencyViolation's LayerConfig is the only known case.
	if len(m.meta.Options) > 0 {
		allNil := true
		for _, opt := range m.meta.Options {
			if opt.Apply != nil {
				allNil = false
				break
			}
		}
		if allNil {
			return parityResult{
				kind:   paritySkip,
				reason: fmt.Sprintf("all %d option(s) have Apply=nil (descriptor-unsupported type)", len(m.meta.Options)),
			}
		}
	}

	// Common active-override cases, run for every rule.
	baseCases := []parityCase{
		{name: "defaults", build: func(b *caseBuilder) {}},
		{name: "rule_disable", build: func(b *caseBuilder) { b.ruleActive = boolP(false) }},
		{name: "rule_enable", build: func(b *caseBuilder) { b.ruleActive = boolP(true) }},
		{name: "ruleset_disable", build: func(b *caseBuilder) { b.ruleSetActive = boolP(false) }},
		{name: "ruleset_disable_with_rule_enable", build: func(b *caseBuilder) {
			b.ruleSetActive = boolP(false)
			b.ruleActive = boolP(true)
		}},
	}

	// When the rule has options, the ruleset-disable case also populates
	// option overrides so we prove the short-circuit prevents option
	// application. We use the first option with a supported Apply.
	var firstSupportedOption *registry.ConfigOption
	for i := range m.meta.Options {
		if m.meta.Options[i].Apply != nil {
			firstSupportedOption = &m.meta.Options[i]
			break
		}
	}
	if firstSupportedOption != nil {
		baseCases = append(baseCases, parityCase{
			name: "ruleset_disable_with_option_override",
			build: func(b *caseBuilder) {
				b.ruleSetActive = boolP(false)
				b.setOption(firstSupportedOption, nonDefaultValue(*firstSupportedOption))
			},
		})
	}

	// Per-option cases.
	var optionCases []parityCase
	for idx := range m.meta.Options {
		opt := m.meta.Options[idx]
		if opt.Apply == nil {
			continue
		}

		// Case: set the primary key to a non-default value.
		optionCases = append(optionCases, parityCase{
			name: fmt.Sprintf("option:%s=nondefault", opt.Name),
			build: func(b *caseBuilder) {
				b.setOption(&opt, nonDefaultValue(opt))
			},
		})

		// Case: set only the alias (when aliases exist).
		if len(opt.Aliases) > 0 && aliasSupportsValueRead(opt) {
			alias := opt.Aliases[0]
			optionCases = append(optionCases, parityCase{
				name: fmt.Sprintf("option:%s[alias=%s]", opt.Name, alias),
				build: func(b *caseBuilder) {
					b.setRawValue(alias, opt.Type, nonDefaultValue(opt))
				},
			})

			// Case: primary + alias both set, primary must win.
			optionCases = append(optionCases, parityCase{
				name: fmt.Sprintf("option:%s[primary_wins_over_alias]", opt.Name),
				build: func(b *caseBuilder) {
					b.setOption(&opt, nonDefaultValue(opt))
					b.setRawValue(alias, opt.Type, secondNonDefaultValue(opt))
				},
			})
		}
	}

	cases := append(baseCases, optionCases...)

	var diagnostics []string
	for _, tc := range cases {
		mismatches := runParityCase(m, tc)
		if len(mismatches) > 0 {
			for _, mm := range mismatches {
				diagnostics = append(diagnostics, fmt.Sprintf("[%s] %s", tc.name, mm))
			}
		}
	}

	if len(diagnostics) > 0 {
		t.Errorf("rule %s diverged in %d case(s):\n    %s",
			m.name, len(diagnostics), strings.Join(diagnostics, "\n    "))
		return parityResult{kind: parityFail, reason: diagnostics[0]}
	}
	return parityResult{kind: parityPass}
}

type parityCase struct {
	name  string
	build func(*caseBuilder)
}

// caseBuilder collects the YAML-equivalent state for a single case. It
// emits both a *config.Config (for the legacy path) and a set of
// key/value pairs for the registry FakeConfigSource. Having one builder
// keeps the two paths strictly in sync.
type caseBuilder struct {
	ruleSetActive *bool
	ruleActive    *bool

	// entries is the flat list of (key, type, value) overrides this case
	// applies. We preserve insertion order so the "primary first vs alias
	// first" check is deterministic.
	entries []caseEntry
}

type caseEntry struct {
	key   string
	otype registry.OptionType
	value interface{}
}

func (b *caseBuilder) setOption(opt *registry.ConfigOption, value interface{}) {
	b.entries = append(b.entries, caseEntry{key: opt.Name, otype: opt.Type, value: value})
}

func (b *caseBuilder) setRawValue(key string, otype registry.OptionType, value interface{}) {
	b.entries = append(b.entries, caseEntry{key: key, otype: otype, value: value})
}

func boolP(b bool) *bool { return &b }

// aliasSupportsValueRead returns true when the alias can legitimately be
// probed for this option type through the legacy *config.Config path. All
// current option types support it.
func aliasSupportsValueRead(opt registry.ConfigOption) bool {
	return true
}

// runParityCase returns the list of divergence messages for a single case.
// An empty slice means the legacy and registry paths produced identical
// state.
func runParityCase(m migratedRule, tc parityCase) []string {
	b := &caseBuilder{}
	tc.build(b)

	// --- legacy path -----------------------------------------------
	// The legacy path mutates DefaultInactive. Snapshot and restore.
	guard := defaultInactiveGuard.snapshot(m.name)
	defer guard.restore()

	legacyRule := cloneRule(m.template)
	legacyCfg := buildLegacyConfigFromBuilder(m, b)
	legacyActive := applyLegacySingleRule(legacyRule, m, legacyCfg)

	// --- registry path ---------------------------------------------
	registryRule := cloneRule(m.template)
	registryCfg := buildRegistryConfigFromBuilder(m, b)
	registryActive := registry.ApplyConfig(registryRule, m.meta, registryCfg)

	// --- compare ----------------------------------------------------
	var mismatches []string
	if legacyActive != registryActive {
		mismatches = append(mismatches, fmt.Sprintf("active: legacy=%v registry=%v", legacyActive, registryActive))
	}
	if diff := compareRuleState(legacyRule, registryRule); diff != "" {
		mismatches = append(mismatches, "state: "+diff)
	}
	return mismatches
}

// buildLegacyConfigFromBuilder constructs a *config.Config reproducing the
// case's YAML shape. The ruleset-level active flag has to be poked into
// the raw map because Set only nests values under ruleSet.rule.key.
func buildLegacyConfigFromBuilder(m migratedRule, b *caseBuilder) *config.Config {
	cfg := config.NewConfig()
	if b.ruleActive != nil {
		cfg.Set(m.meta.RuleSet, m.meta.ID, "active", *b.ruleActive)
	}
	for _, e := range b.entries {
		cfg.Set(m.meta.RuleSet, m.meta.ID, e.key, legacyValue(e))
	}
	if b.ruleSetActive != nil {
		data := cfg.Data()
		rsMap, ok := data[m.meta.RuleSet].(map[string]interface{})
		if !ok {
			rsMap = make(map[string]interface{})
			data[m.meta.RuleSet] = rsMap
		}
		rsMap["active"] = *b.ruleSetActive
	}
	return cfg
}

// buildRegistryConfigFromBuilder constructs the registry-side
// FakeConfigSource from the same builder.
func buildRegistryConfigFromBuilder(m migratedRule, b *caseBuilder) *registry.FakeConfigSource {
	cfg := registry.NewFakeConfigSource()
	if b.ruleSetActive != nil {
		cfg.SetRuleSetActive(m.meta.RuleSet, *b.ruleSetActive)
	}
	if b.ruleActive != nil {
		cfg.SetRuleActive(m.meta.RuleSet, m.meta.ID, *b.ruleActive)
	}
	for _, e := range b.entries {
		cfg.Set(m.meta.RuleSet, m.meta.ID, e.key, registryValue(e))
	}
	return cfg
}

// legacyValue shapes a case value so *config.Config.GetStringList / GetInt
// see the expected types after YAML decode. The legacy *config.Config has
// slightly different coercions than our FakeConfigSource: GetStringList
// accepts []interface{} and []string, GetInt accepts int/int64/float64,
// GetBool accepts bool/"true"/"false". Normalize to the native Go types
// so both sides interpret the same raw value.
func legacyValue(e caseEntry) interface{} {
	switch e.otype {
	case registry.OptStringList:
		// *config.Config.Set stores whatever we hand it and GetStringList
		// copes with []string or []interface{}. Store []interface{} to
		// exercise the same path a YAML decoder would.
		if ss, ok := e.value.([]string); ok {
			out := make([]interface{}, len(ss))
			for i, s := range ss {
				out[i] = s
			}
			return out
		}
	}
	return e.value
}

// registryValue shapes a case value for the registry's FakeConfigSource.
// Keep it as native Go types — the fake only accepts those.
func registryValue(e caseEntry) interface{} {
	return e.value
}

// --- non-default value generation --------------------------------------

// nonDefaultValue returns a value of the appropriate Go type that differs
// from opt.Default. The helper picks deterministic values so test output
// is stable.
func nonDefaultValue(opt registry.ConfigOption) interface{} {
	switch opt.Type {
	case registry.OptInt:
		d, _ := opt.Default.(int)
		return d + 7
	case registry.OptBool:
		d, _ := opt.Default.(bool)
		return !d
	case registry.OptString:
		return "non-default-value"
	case registry.OptStringList:
		return []string{"parity-a", "parity-b"}
	case registry.OptRegex:
		// Any valid pattern that isn't the rule's default. Keep it
		// simple so CompileAnchoredPattern doesn't reject it.
		return "parityPattern[0-9]+"
	}
	return nil
}

// secondNonDefaultValue produces a second distinct non-default value, used
// by the "primary wins over alias" case. The legacy and registry paths
// must both pick the primary value, so we give the alias a different
// payload to make that observable.
func secondNonDefaultValue(opt registry.ConfigOption) interface{} {
	switch opt.Type {
	case registry.OptInt:
		d, _ := opt.Default.(int)
		return d + 13
	case registry.OptBool:
		d, _ := opt.Default.(bool)
		return d
	case registry.OptString:
		return "alias-value"
	case registry.OptStringList:
		return []string{"alias-x"}
	case registry.OptRegex:
		return "aliasPattern[A-Z]+"
	}
	return nil
}

// --- state comparison --------------------------------------------------

// compareRuleState deep-compares two rule pointers, special-casing
// *regexp.Regexp fields (compare by pattern string, since compiled
// regexes use different internal pointers). Returns "" on equality or a
// short diff string on mismatch.
func compareRuleState(a, b interface{}) string {
	av := reflect.ValueOf(a).Elem()
	bv := reflect.ValueOf(b).Elem()
	if av.Type() != bv.Type() {
		return fmt.Sprintf("type mismatch: %s vs %s", av.Type(), bv.Type())
	}
	for i := 0; i < av.NumField(); i++ {
		af := av.Field(i)
		bf := bv.Field(i)
		if !af.CanInterface() {
			continue
		}
		if af.Type() == reflect.TypeOf((*regexp.Regexp)(nil)) {
			as := regexpString(af)
			bs := regexpString(bf)
			if as != bs {
				return fmt.Sprintf("field %s (regex): legacy=%q registry=%q",
					av.Type().Field(i).Name, as, bs)
			}
			continue
		}
		if !reflect.DeepEqual(af.Interface(), bf.Interface()) {
			return fmt.Sprintf("field %s: legacy=%v registry=%v",
				av.Type().Field(i).Name, af.Interface(), bf.Interface())
		}
	}
	return ""
}

func regexpString(v reflect.Value) string {
	if v.IsNil() {
		return ""
	}
	r := v.Interface().(*regexp.Regexp)
	return r.String()
}

// --- legacy path invocation --------------------------------------------

// applyLegacySingleRule historically drove the 631-line applyRuleConfig
// switch on a single rule instance. Phase 3D of the CodegenRegistry
// migration deleted that switch and flipped rules.ApplyConfig over to
// registry.ApplyConfig. The parity test is retained as a safety net for
// future regressions, so we invoke the same registry path here that
// production now uses.
//
// The DefaultInactive bookkeeping mirrors rules.ApplyConfig exactly so
// downstream assertions on the map continue to pass. This keeps the
// parity test meaningful even though the two sides converge on the same
// code path — it guards against anyone reintroducing a divergent code
// path in ApplyConfig.
func applyLegacySingleRule(rule interface{}, m migratedRule, cfg *config.Config) bool {
	ruleName := m.meta.ID
	adapter := NewConfigAdapter(cfg)
	active := registry.ApplyConfig(rule, m.meta, adapter)
	if active {
		delete(DefaultInactive, ruleName)
	} else {
		DefaultInactive[ruleName] = true
	}
	return active
}

// --- DefaultInactive snapshot/restore ----------------------------------

// defaultInactiveGuard serializes test access to the global DefaultInactive
// map and restores it after each case. Parallel sub-tests would race on
// the map without this.
type defaultInactiveGuardT struct {
	mu sync.Mutex
}

var defaultInactiveGuard = &defaultInactiveGuardT{}

type defaultInactiveSnapshot struct {
	name string
	prev bool
	had  bool
	g    *defaultInactiveGuardT
}

func (g *defaultInactiveGuardT) snapshot(name string) *defaultInactiveSnapshot {
	g.mu.Lock()
	prev, had := DefaultInactive[name]
	return &defaultInactiveSnapshot{name: name, prev: prev, had: had, g: g}
}

func (s *defaultInactiveSnapshot) restore() {
	if s.had {
		DefaultInactive[s.name] = s.prev
	} else {
		delete(DefaultInactive, s.name)
	}
	s.g.mu.Unlock()
}
