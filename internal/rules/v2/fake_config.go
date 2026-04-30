package v2

// FakeConfigSource is an in-memory ConfigSource for tests. It is also a
// useful starting point for rule authors experimenting with the v2 metadata
// runtime — it has no YAML dependency.
//
// The three-level Values map is keyed as Values[ruleSet][rule][key] so
// tests can store arbitrary scalar values (int, bool, string,
// []string). The GetXxx methods perform best-effort coercion: int/bool
// require the matching Go type; GetString accepts string values;
// GetStringList accepts either []string or []interface{} (the shape
// produced by a generic YAML decoder) so tests can set values with the
// same types they would get from the real config loader.
type FakeConfigSource struct {
	// Values stores the configured overrides, keyed as
	// Values[ruleSet][rule][key]. Nil is safe to read from but not
	// safe to write to; use Set to populate.
	Values map[string]map[string]map[string]interface{}

	// RuleActive stores explicit rule-level active overrides.
	// Presence of the key indicates an override; the bool is the
	// override value. Absence means "no override".
	RuleActive map[string]bool

	// RuleSetActive stores explicit ruleset-level active overrides.
	// Same presence semantics as RuleActive.
	RuleSetActive map[string]bool
}

// NewFakeConfigSource returns a FakeConfigSource with empty maps ready
// to be populated via Set.
func NewFakeConfigSource() *FakeConfigSource {
	return &FakeConfigSource{
		Values:        make(map[string]map[string]map[string]interface{}),
		RuleActive:    make(map[string]bool),
		RuleSetActive: make(map[string]bool),
	}
}

// Set records an override value for (ruleSet, rule, key). Pass the Go
// type matching the option type — int for OptInt, bool for OptBool,
// string for OptString and OptRegex, []string for OptStringList.
func (f *FakeConfigSource) Set(ruleSet, rule, key string, value interface{}) {
	if f.Values == nil {
		f.Values = make(map[string]map[string]map[string]interface{})
	}
	if f.Values[ruleSet] == nil {
		f.Values[ruleSet] = make(map[string]map[string]interface{})
	}
	if f.Values[ruleSet][rule] == nil {
		f.Values[ruleSet][rule] = make(map[string]interface{})
	}
	f.Values[ruleSet][rule][key] = value
}

// SetRuleActive records an explicit rule-level active override.
func (f *FakeConfigSource) SetRuleActive(ruleSet, rule string, active bool) {
	if f.RuleActive == nil {
		f.RuleActive = make(map[string]bool)
	}
	f.RuleActive[ruleActiveKey(ruleSet, rule)] = active
}

// SetRuleSetActive records an explicit ruleset-level active override.
func (f *FakeConfigSource) SetRuleSetActive(ruleSet string, active bool) {
	if f.RuleSetActive == nil {
		f.RuleSetActive = make(map[string]bool)
	}
	f.RuleSetActive[ruleSet] = active
}

func ruleActiveKey(ruleSet, rule string) string {
	return ruleSet + "." + rule
}

// lookup returns the raw override value for (ruleSet, rule, key) along
// with a presence flag.
func (f *FakeConfigSource) lookup(ruleSet, rule, key string) (interface{}, bool) {
	if f == nil || f.Values == nil {
		return nil, false
	}
	rs, ok := f.Values[ruleSet]
	if !ok {
		return nil, false
	}
	r, ok := rs[rule]
	if !ok {
		return nil, false
	}
	v, ok := r[key]
	return v, ok
}

// HasKey implements ConfigSource.
func (f *FakeConfigSource) HasKey(ruleSet, rule, key string) bool {
	_, ok := f.lookup(ruleSet, rule, key)
	return ok
}

// GetInt implements ConfigSource.
func (f *FakeConfigSource) GetInt(ruleSet, rule, key string, def int) int {
	v, ok := f.lookup(ruleSet, rule, key)
	if !ok {
		return def
	}
	if iv, ok := v.(int); ok {
		return iv
	}
	return def
}

// GetBool implements ConfigSource.
func (f *FakeConfigSource) GetBool(ruleSet, rule, key string, def bool) bool {
	v, ok := f.lookup(ruleSet, rule, key)
	if !ok {
		return def
	}
	if bv, ok := v.(bool); ok {
		return bv
	}
	return def
}

// GetString implements ConfigSource.
func (f *FakeConfigSource) GetString(ruleSet, rule, key, def string) string {
	v, ok := f.lookup(ruleSet, rule, key)
	if !ok {
		return def
	}
	if sv, ok := v.(string); ok {
		return sv
	}
	return def
}

// GetStringList implements ConfigSource. Accepts either []string or
// []interface{} (the shape a generic YAML decoder produces).
func (f *FakeConfigSource) GetStringList(ruleSet, rule, key string) []string {
	v, ok := f.lookup(ruleSet, rule, key)
	if !ok {
		return nil
	}
	switch s := v.(type) {
	case []string:
		return s
	case []interface{}:
		out := make([]string, 0, len(s))
		for _, el := range s {
			if sv, ok := el.(string); ok {
				out = append(out, sv)
			}
		}
		return out
	}
	return nil
}

// IsRuleActive implements ConfigSource.
func (f *FakeConfigSource) IsRuleActive(ruleSet, rule string) *bool {
	if f == nil || f.RuleActive == nil {
		return nil
	}
	v, ok := f.RuleActive[ruleActiveKey(ruleSet, rule)]
	if !ok {
		return nil
	}
	return &v
}

// IsRuleSetActive implements ConfigSource.
func (f *FakeConfigSource) IsRuleSetActive(ruleSet string) *bool {
	if f == nil || f.RuleSetActive == nil {
		return nil
	}
	v, ok := f.RuleSetActive[ruleSet]
	if !ok {
		return nil
	}
	return &v
}
