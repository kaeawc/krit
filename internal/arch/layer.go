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
	if cfg == nil {
		return nil
	}
	data := cfg.Data()
	if data == nil {
		return nil
	}

	archData, ok := data["architecture"]
	if !ok {
		return nil
	}
	archMap, ok := archData.(map[string]interface{})
	if !ok {
		return nil
	}

	ruleData, ok := archMap["LayerDependencyViolation"]
	if !ok {
		return nil
	}
	ruleMap, ok := ruleData.(map[string]interface{})
	if !ok {
		return nil
	}

	// Parse layers
	layersRaw, ok := ruleMap["layers"]
	if !ok {
		return nil
	}
	layersList, ok := layersRaw.([]interface{})
	if !ok {
		return nil
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
		var modules []string
		if modsRaw, ok := itemMap["modules"]; ok {
			switch v := modsRaw.(type) {
			case []interface{}:
				for _, m := range v {
					if s, ok := m.(string); ok {
						modules = append(modules, s)
					}
				}
			case []string:
				modules = v
			}
		}
		layers = append(layers, Layer{Name: name, Modules: modules})
	}

	if len(layers) == 0 {
		return nil
	}

	// Parse allowed
	allowed := make(map[string][]string)
	if allowedRaw, ok := ruleMap["allowed"]; ok {
		if allowedMap, ok := allowedRaw.(map[string]interface{}); ok {
			for layerName, deps := range allowedMap {
				switch v := deps.(type) {
				case []interface{}:
					for _, d := range v {
						if s, ok := d.(string); ok {
							allowed[layerName] = append(allowed[layerName], s)
						}
					}
				case []string:
					allowed[layerName] = v
				}
			}
		}
	}

	return &LayerConfig{
		Layers:  layers,
		Allowed: allowed,
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
