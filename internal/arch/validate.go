package arch

import (
	"sort"

	"github.com/kaeawc/krit/internal/module"
)

// LayerViolation represents a dependency that crosses an unauthorized layer boundary.
type LayerViolation struct {
	SourceModule string
	TargetModule string
	SourceLayer  string
	TargetLayer  string
}

// ValidateLayers checks all module dependencies against the layer config.
// Returns violations where a module in one layer depends on a module
// in a layer that is not in the allowed list.
func ValidateLayers(cfg *LayerConfig, graph *module.ModuleGraph) []LayerViolation {
	if cfg == nil || graph == nil {
		return nil
	}

	// Resolve each module's layer once; LayerForModule is O(layers × patterns).
	layerOf := make(map[string]string, len(graph.Modules))
	resolve := func(path string) string {
		if l, ok := layerOf[path]; ok {
			return l
		}
		l := cfg.LayerForModule(path)
		layerOf[path] = l
		return l
	}

	var violations []LayerViolation

	for modPath, mod := range graph.Modules {
		srcLayer := resolve(modPath)
		if srcLayer == "" {
			continue
		}

		for _, dep := range mod.Dependencies {
			tgtLayer := resolve(dep.ModulePath)
			if tgtLayer == "" {
				continue
			}
			// Same layer is always allowed
			if srcLayer == tgtLayer {
				continue
			}
			if !isAllowed(cfg, srcLayer, tgtLayer) {
				violations = append(violations, LayerViolation{
					SourceModule: modPath,
					TargetModule: dep.ModulePath,
					SourceLayer:  srcLayer,
					TargetLayer:  tgtLayer,
				})
			}
		}
	}

	// Sort for deterministic output
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].SourceModule != violations[j].SourceModule {
			return violations[i].SourceModule < violations[j].SourceModule
		}
		return violations[i].TargetModule < violations[j].TargetModule
	})

	return violations
}

func isAllowed(cfg *LayerConfig, srcLayer, tgtLayer string) bool {
	allowed, ok := cfg.Allowed[srcLayer]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == tgtLayer {
			return true
		}
	}
	return false
}
