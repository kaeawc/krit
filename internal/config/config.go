package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the parsed YAML configuration for krit rules.
// The data structure is: ruleSet -> ruleName -> key -> value
type Config struct {
	data map[string]interface{}
}

// ModuleTemplateConfig describes the optional top-level module_template block.
type ModuleTemplateConfig struct {
	FeatureRoot        string
	RequiredSubmodules []string
	RequiredPlugins    []string
}

type SLOConfig struct {
	Module             string
	MaxWarningsPerKLOC *float64
	MaxErrorsPerKLOC   *float64
}

// NewConfig creates an empty Config.
func NewConfig() *Config {
	return &Config{data: make(map[string]interface{})}
}

// LoadConfig loads a YAML config file and returns a Config.
// If path is empty, it auto-detects krit.yml or .krit.yml from the project root,
// falling back to config/default-krit.yml relative to the executable.
func LoadConfig(path string) (*Config, error) {
	if path != "" {
		return loadFile(path)
	}
	return autoDetect()
}

// LoadAndMerge loads defaults first, then merges user config on top.
// If userPath is empty, auto-detection is used for the user config.
// If no user config is found, defaults alone are returned.
func LoadAndMerge(userPath string, defaultPath string) (*Config, error) {
	var base *Config
	if defaultPath != "" {
		var err error
		base, err = loadFile(defaultPath)
		if err != nil {
			// If default file doesn't exist, start with empty config
			base = &Config{data: make(map[string]interface{})}
		}
	} else {
		base = &Config{data: make(map[string]interface{})}
	}

	var user *Config
	var err error
	if userPath != "" {
		user, err = loadFile(userPath)
		if err != nil {
			return nil, fmt.Errorf("loading config %s: %w", userPath, err)
		}
	} else {
		user, err = autoDetect()
		if err != nil || user == nil {
			// No user config found, return defaults only
			return base, nil //nolint:nilerr // autoDetect failure is non-fatal: krit.yml is optional, caller proceeds with built-in defaults
		}
	}

	// Merge user config over defaults
	merged := mergeMaps(base.data, user.data)
	return &Config{data: merged}, nil
}

func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if raw == nil {
		raw = make(map[string]interface{})
	}
	return &Config{data: raw}, nil
}

// Filenames lists the krit configuration filenames, in the order
// callers should probe for them. Centralised so adding a third name
// (e.g. krit.yaml) is a one-line change.
var Filenames = []string{"krit.yml", ".krit.yml"}

func autoDetect() (*Config, error) {
	for _, name := range Filenames {
		if _, err := os.Stat(name); err == nil {
			return loadFile(name)
		}
	}
	return nil, nil
}

// FindDefaultConfig locates the default config file.
// It checks relative to the executable path first, then the current directory.
func FindDefaultConfig() string {
	// Check relative to executable
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		candidate := filepath.Join(exeDir, "config", "default-krit.yml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		// Also check one level up (for development: binary in project root)
		candidate = filepath.Join(exeDir, "..", "config", "default-krit.yml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	// Check current directory
	candidate := filepath.Join("config", "default-krit.yml")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

// IsRuleActive returns whether a rule is active in the config.
// Returns nil if the rule is not mentioned in config (caller should use default).
func (c *Config) IsRuleActive(ruleSet, rule string) *bool {
	if c == nil || c.data == nil {
		return nil
	}
	ruleConfig := c.getRuleConfig(ruleSet, rule)
	if ruleConfig == nil {
		return nil
	}
	v, ok := ruleConfig["active"]
	if !ok {
		return nil
	}
	b, ok := toBool(v)
	if !ok {
		return nil
	}
	return &b
}

// IsRuleSetActive returns whether an entire ruleset is active.
// Returns nil if not specified.
func (c *Config) IsRuleSetActive(ruleSet string) *bool {
	if c == nil || c.data == nil {
		return nil
	}
	rsData, ok := c.data[ruleSet]
	if !ok {
		return nil
	}
	rsMap, ok := rsData.(map[string]interface{})
	if !ok {
		return nil
	}
	v, ok := rsMap["active"]
	if !ok {
		return nil
	}
	b, ok := toBool(v)
	if !ok {
		return nil
	}
	return &b
}

// GetInt returns an integer config value for a rule, or defaultVal if not found.
func (c *Config) GetInt(ruleSet, rule, key string, defaultVal int) int {
	if c == nil || c.data == nil {
		return defaultVal
	}
	ruleConfig := c.getRuleConfig(ruleSet, rule)
	if ruleConfig == nil {
		return defaultVal
	}
	v, ok := ruleConfig[key]
	if !ok {
		return defaultVal
	}
	return toInt(v, defaultVal)
}

// GetBool returns a bool config value for a rule, or defaultVal if not found.
func (c *Config) GetBool(ruleSet, rule, key string, defaultVal bool) bool {
	if c == nil || c.data == nil {
		return defaultVal
	}
	ruleConfig := c.getRuleConfig(ruleSet, rule)
	if ruleConfig == nil {
		return defaultVal
	}
	v, ok := ruleConfig[key]
	if !ok {
		return defaultVal
	}
	b, ok := toBool(v)
	if !ok {
		return defaultVal
	}
	return b
}

// GetString returns a string config value for a rule, or defaultVal if not found.
func (c *Config) GetString(ruleSet, rule, key string, defaultVal string) string {
	if c == nil || c.data == nil {
		return defaultVal
	}
	ruleConfig := c.getRuleConfig(ruleSet, rule)
	if ruleConfig == nil {
		return defaultVal
	}
	v, ok := ruleConfig[key]
	if !ok {
		return defaultVal
	}
	s, ok := v.(string)
	if !ok {
		return defaultVal
	}
	return s
}

// GetStringList returns a string slice config value for a rule.
// Returns nil if not found.
func (c *Config) GetStringList(ruleSet, rule, key string) []string {
	if c == nil || c.data == nil {
		return nil
	}
	ruleConfig := c.getRuleConfig(ruleSet, rule)
	if ruleConfig == nil {
		return nil
	}
	v, ok := ruleConfig[key]
	if !ok {
		return nil
	}
	return toStringSlice(v)
}

// Has reports whether a given rule config key is set in the YAML (i.e. the
// key exists under ruleSet.rule in the config map). Used by the registry-
// driven ApplyConfig path to distinguish "key absent" from "key set to the
// zero value" — GetInt/GetString/GetBool all return the default when the
// key is missing, which is the same shape they return when a legitimate
// override happens to equal the default. HasKey on ConfigSource is the
// contract that lets descriptor Apply closures know whether to mutate the
// rule struct.
func (c *Config) Has(ruleSet, rule, key string) bool {
	if c == nil || c.data == nil {
		return false
	}
	ruleConfig := c.getRuleConfig(ruleSet, rule)
	if ruleConfig == nil {
		return false
	}
	_, ok := ruleConfig[key]
	return ok
}

// getRuleConfig returns the config map for a specific rule within a ruleset.
func (c *Config) getRuleConfig(ruleSet, rule string) map[string]interface{} {
	rsData, ok := c.data[ruleSet]
	if !ok {
		return nil
	}
	rsMap, ok := rsData.(map[string]interface{})
	if !ok {
		return nil
	}
	ruleData, ok := rsMap[rule]
	if !ok {
		return nil
	}
	ruleMap, ok := ruleData.(map[string]interface{})
	if !ok {
		return nil
	}
	return ruleMap
}

// GetTopLevelString returns a string value from a top-level config section.
// For example, GetTopLevelString("android", "enabled", "auto") reads android.enabled.
func (c *Config) GetTopLevelString(section, key, defaultVal string) string {
	if c == nil || c.data == nil {
		return defaultVal
	}
	secData, ok := c.data[section]
	if !ok {
		return defaultVal
	}
	secMap, ok := secData.(map[string]interface{})
	if !ok {
		return defaultVal
	}
	v, ok := secMap[key]
	if !ok {
		return defaultVal
	}
	s, ok := v.(string)
	if !ok {
		// Handle bool values serialized as native YAML types
		if b, ok := v.(bool); ok {
			if b {
				return "true"
			}
			return "false"
		}
		return defaultVal
	}
	return s
}

// GetTopLevelStringList returns a string slice from a top-level section.
func (c *Config) GetTopLevelStringList(section, key string) []string {
	if c == nil || c.data == nil {
		return nil
	}
	secData, ok := c.data[section]
	if !ok {
		return nil
	}
	secMap, ok := secData.(map[string]interface{})
	if !ok {
		return nil
	}
	v, ok := secMap[key]
	if !ok {
		return nil
	}
	return toStringSlice(v)
}

// ModuleTemplate returns the configured feature-module template, if any.
func (c *Config) ModuleTemplate() ModuleTemplateConfig {
	return ModuleTemplateConfig{
		FeatureRoot:        c.GetTopLevelString("module_template", "feature_root", ""),
		RequiredSubmodules: c.GetTopLevelStringList("module_template", "required_submodules"),
		RequiredPlugins:    c.GetTopLevelStringList("module_template", "required_plugins"),
	}
}

func (c *Config) TestSourcePaths() []string {
	return c.GetTopLevelList("testSourcePaths")
}

func (c *Config) TestSourcePathsOverride() []string {
	return c.GetTopLevelList("testSourcePathsOverride")
}

func (c *Config) SLOs() []SLOConfig {
	if c == nil || c.data == nil {
		return nil
	}
	raw, ok := c.data["slos"]
	if !ok {
		return nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]SLOConfig, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		module, _ := m["module"].(string)
		if module == "" {
			continue
		}
		slo := SLOConfig{Module: module}
		if v, ok := toFloat64(m["max_warnings_per_kloc"]); ok {
			slo.MaxWarningsPerKLOC = &v
		}
		if v, ok := toFloat64(m["max_errors_per_kloc"]); ok {
			slo.MaxErrorsPerKLOC = &v
		}
		out = append(out, slo)
	}
	return out
}

func (c *Config) GetTopLevelList(key string) []string {
	if c == nil || c.data == nil {
		return nil
	}
	v, ok := c.data[key]
	if !ok {
		return nil
	}
	return toStringSlice(v)
}

// GetTopLevelInt returns an int value nested under a top-level section.
// For example, GetTopLevelInt("parseCache", "maxSizeMB", 200) reads
// parseCache.maxSizeMB.
func (c *Config) GetTopLevelInt(section, key string, defaultVal int) int {
	if c == nil || c.data == nil {
		return defaultVal
	}
	secData, ok := c.data[section]
	if !ok {
		return defaultVal
	}
	secMap, ok := secData.(map[string]interface{})
	if !ok {
		return defaultVal
	}
	v, ok := secMap[key]
	if !ok {
		return defaultVal
	}
	return toInt(v, defaultVal)
}

// GetTopLevelKeyString returns a string value from a top-level config key.
// For example, GetTopLevelKeyString("maxCost", "") reads config.maxCost.
func (c *Config) GetTopLevelKeyString(key, defaultVal string) string {
	if c == nil || c.data == nil {
		return defaultVal
	}
	s, ok := c.data[key].(string)
	if !ok {
		return defaultVal
	}
	return s
}

// GetTopLevelBool returns a bool value from a top-level config key.
// For example, GetTopLevelBool("warningsAsErrors", false) reads config.warningsAsErrors.
func (c *Config) GetTopLevelBool(key string, defaultVal bool) bool {
	if c == nil || c.data == nil {
		return defaultVal
	}
	v, ok := c.data[key]
	if !ok {
		return defaultVal
	}
	b, ok := toBool(v)
	if !ok {
		return defaultVal
	}
	return b
}

// Data returns the underlying config data map for serialization (e.g., cache hashing).
func (c *Config) Data() map[string]interface{} {
	if c == nil {
		return nil
	}
	return c.data
}

// Set writes a value into the config for a specific rule.
func (c *Config) Set(ruleSet, rule, key string, value interface{}) {
	if c == nil {
		return
	}
	if c.data == nil {
		c.data = make(map[string]interface{})
	}
	rsData, ok := c.data[ruleSet]
	if !ok {
		rsData = make(map[string]interface{})
		c.data[ruleSet] = rsData
	}
	rsMap, ok := rsData.(map[string]interface{})
	if !ok {
		rsMap = make(map[string]interface{})
		c.data[ruleSet] = rsMap
	}
	ruleData, ok := rsMap[rule]
	if !ok {
		ruleData = make(map[string]interface{})
		rsMap[rule] = ruleData
	}
	ruleMap, ok := ruleData.(map[string]interface{})
	if !ok {
		ruleMap = make(map[string]interface{})
		rsMap[rule] = ruleMap
	}
	ruleMap[key] = value
}

// mergeMaps deep-merges src into dst, returning a new map.
// src values override dst values at leaf level.
func mergeMaps(dst, src map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range dst {
		result[k] = v
	}
	for k, v := range src {
		if dstVal, exists := result[k]; exists {
			dstMap, dstOk := dstVal.(map[string]interface{})
			srcMap, srcOk := v.(map[string]interface{})
			if dstOk && srcOk {
				result[k] = mergeMaps(dstMap, srcMap)
				continue
			}
		}
		result[k] = v
	}
	return result
}

// Type conversion helpers

func toBool(v interface{}) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	case string:
		if b == "true" {
			return true, true
		}
		if b == "false" {
			return false, true
		}
	}
	return false, false
}

func toInt(v interface{}, defaultVal int) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		var i int
		if _, err := fmt.Sscanf(n, "%d", &i); err == nil {
			return i
		}
	}
	return defaultVal
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case string:
		var parsed float64
		if _, err := fmt.Sscanf(n, "%f", &parsed); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func toStringSlice(v interface{}) []string {
	switch s := v.(type) {
	case []interface{}:
		result := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				result = append(result, str)
			} else {
				result = append(result, fmt.Sprintf("%v", item))
			}
		}
		return result
	case []string:
		return s
	case string:
		return []string{s}
	}
	return nil
}
