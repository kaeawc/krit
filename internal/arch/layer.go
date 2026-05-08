package arch

import (
	"strings"

	"github.com/kaeawc/krit/internal/config"
)

// Layer represents a named architectural layer with module glob patterns.
type Layer struct {
	Name    string
	Modules []string // glob patterns like ":feature-*"
}

// LayerConfig holds the full layer dependency configuration.
type LayerConfig struct {
	Layers  []Layer
	Allowed map[string][]string // layer name -> allowed dependency layers
}

// ParseLayerConfig extracts layer configuration from a krit Config.
// The config structure is nested under architecture.LayerDependencyViolation.
// Returns nil if no layer config is present.
func ParseLayerConfig(cfg *config.Config) *LayerConfig {
	ruleMap, ok := layerDependencyRuleMap(cfg)
	if !ok {
		return nil
	}
	layers, ok := parseLayerList(ruleMap)
	if !ok {
		return nil
	}
	return &LayerConfig{
		Layers:  layers,
		Allowed: parseAllowedMap(ruleMap),
	}
}

func layerDependencyRuleMap(cfg *config.Config) (map[string]interface{}, bool) {
	if cfg == nil {
		return nil, false
	}
	data := cfg.Data()
	if data == nil {
		return nil, false
	}
	archData, ok := data["architecture"]
	if !ok {
		return nil, false
	}
	archMap, ok := archData.(map[string]interface{})
	if !ok {
		return nil, false
	}
	ruleData, ok := archMap["LayerDependencyViolation"]
	if !ok {
		return nil, false
	}
	ruleMap, ok := ruleData.(map[string]interface{})
	return ruleMap, ok
}

func parseLayerList(ruleMap map[string]interface{}) ([]Layer, bool) {
	layersRaw, ok := ruleMap["layers"]
	if !ok {
		return nil, false
	}
	layersList, ok := layersRaw.([]interface{})
	if !ok {
		return nil, false
	}
	var layers []Layer
	for _, item := range layersList {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := itemMap["name"].(string)
		if name == "" {
			continue
		}
		layers = append(layers, Layer{Name: name, Modules: parseStringSlice(itemMap["modules"])})
	}
	if len(layers) == 0 {
		return nil, false
	}
	return layers, true
}

func parseAllowedMap(ruleMap map[string]interface{}) map[string][]string {
	allowed := make(map[string][]string)
	allowedRaw, ok := ruleMap["allowed"]
	if !ok {
		return allowed
	}
	allowedMap, ok := allowedRaw.(map[string]interface{})
	if !ok {
		return allowed
	}
	for layerName, deps := range allowedMap {
		allowed[layerName] = parseStringSlice(deps)
	}
	return allowed
}

func parseStringSlice(raw interface{}) []string {
	switch v := raw.(type) {
	case []interface{}:
		var out []string
		for _, m := range v {
			if s, ok := m.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	default:
		return nil
	}
}

// LayerForModule returns the layer name for a given module path,
// or "" if the module doesn't match any layer.
func (lc *LayerConfig) LayerForModule(modulePath string) string {
	if lc == nil {
		return ""
	}
	for _, layer := range lc.Layers {
		for _, pattern := range layer.Modules {
			if matchModuleGlob(pattern, modulePath) {
				return layer.Name
			}
		}
	}
	return ""
}

// matchModuleGlob checks if a module path matches a glob pattern.
// Supports * wildcards (e.g., ":feature-*" matches ":feature-login").
func matchModuleGlob(pattern, modulePath string) bool {
	// Simple glob: split on * and check that all parts appear in order
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		// No wildcard — exact match
		return pattern == modulePath
	}

	remaining := modulePath
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(remaining, part)
		if idx < 0 {
			return false
		}
		// First part must be a prefix
		if i == 0 && idx != 0 {
			return false
		}
		remaining = remaining[idx+len(part):]
	}
	// Last part must be a suffix (if non-empty)
	lastPart := parts[len(parts)-1]
	if lastPart != "" && !strings.HasSuffix(modulePath, lastPart) {
		return false
	}
	return true
}
