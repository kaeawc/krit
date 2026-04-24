package rules

import (
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules/registry"
)

// ConfigAdapter wraps *config.Config so it satisfies registry.ConfigSource.
//
// The registry runtime (ApplyConfig) reads YAML-derived values through a
// small interface so the registry package itself stays free of any YAML or
// detekt-specific plumbing. This adapter bridges the gap between the
// existing internal/config package and the Meta() descriptors.
//
// The adapter is a thin pass-through: every ConfigSource method delegates
// directly to the corresponding *config.Config method. The one piece of
// new surface area is HasKey, which is backed by config.Config.Has (added
// alongside this adapter for exactly that purpose).
type ConfigAdapter struct {
	cfg *config.Config
}

// NewConfigAdapter wraps cfg in a registry.ConfigSource-compatible adapter.
// A nil cfg is allowed: every method short-circuits to "no override", which
// is the same behavior as *config.Config's own nil-safe accessors.
func NewConfigAdapter(cfg *config.Config) *ConfigAdapter {
	return &ConfigAdapter{cfg: cfg}
}

// Unwrap returns the wrapped *config.Config. Primarily useful for tests
// that want to inspect the underlying data after running the adapter.
func (a *ConfigAdapter) Unwrap() *config.Config {
	if a == nil {
		return nil
	}
	return a.cfg
}

// HasKey implements registry.ConfigSource.
func (a *ConfigAdapter) HasKey(ruleSet, rule, key string) bool {
	if a == nil {
		return false
	}
	return a.cfg.Has(ruleSet, rule, key)
}

// GetInt implements registry.ConfigSource.
func (a *ConfigAdapter) GetInt(ruleSet, rule, key string, def int) int {
	if a == nil {
		return def
	}
	return a.cfg.GetInt(ruleSet, rule, key, def)
}

// GetBool implements registry.ConfigSource.
func (a *ConfigAdapter) GetBool(ruleSet, rule, key string, def bool) bool {
	if a == nil {
		return def
	}
	return a.cfg.GetBool(ruleSet, rule, key, def)
}

// GetString implements registry.ConfigSource.
func (a *ConfigAdapter) GetString(ruleSet, rule, key, def string) string {
	if a == nil {
		return def
	}
	return a.cfg.GetString(ruleSet, rule, key, def)
}

// GetStringList implements registry.ConfigSource.
func (a *ConfigAdapter) GetStringList(ruleSet, rule, key string) []string {
	if a == nil {
		return nil
	}
	return a.cfg.GetStringList(ruleSet, rule, key)
}

// IsRuleActive implements registry.ConfigSource.
func (a *ConfigAdapter) IsRuleActive(ruleSet, rule string) *bool {
	if a == nil {
		return nil
	}
	return a.cfg.IsRuleActive(ruleSet, rule)
}

// IsRuleSetActive implements registry.ConfigSource.
func (a *ConfigAdapter) IsRuleSetActive(ruleSet string) *bool {
	if a == nil {
		return nil
	}
	return a.cfg.IsRuleSetActive(ruleSet)
}

// Compile-time check that ConfigAdapter satisfies the registry contract.
var _ registry.ConfigSource = (*ConfigAdapter)(nil)
