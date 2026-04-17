package arch

import (
	"testing"

	"github.com/kaeawc/krit/internal/module"
)

func buildGraph(modules map[string][]module.Dependency) *module.ModuleGraph {
	graph := module.NewModuleGraph("/tmp/test")
	for path, deps := range modules {
		graph.Modules[path] = &module.Module{
			Path:         path,
			Dir:          "/tmp/test/" + path,
			Dependencies: deps,
		}
	}
	return graph
}

func buildLayerConfig() *LayerConfig {
	return &LayerConfig{
		Layers: []Layer{
			{Name: "ui", Modules: []string{":app", ":feature-*"}},
			{Name: "domain", Modules: []string{":domain-*"}},
			{Name: "data", Modules: []string{":data-*"}},
		},
		Allowed: map[string][]string{
			"ui":     {"domain"},
			"domain": {"data"},
		},
	}
}

func TestValidateLayers_AllAllowed(t *testing.T) {
	cfg := buildLayerConfig()
	graph := buildGraph(map[string][]module.Dependency{
		":app": {
			{ModulePath: ":domain-auth", Configuration: "implementation"},
		},
		":domain-auth": {
			{ModulePath: ":data-network", Configuration: "implementation"},
		},
		":data-network": {},
	})

	violations := ValidateLayers(cfg, graph)
	if len(violations) != 0 {
		t.Errorf("expected no violations, got %d: %+v", len(violations), violations)
	}
}

func TestValidateLayers_SingleViolation(t *testing.T) {
	cfg := buildLayerConfig()
	graph := buildGraph(map[string][]module.Dependency{
		":app": {
			{ModulePath: ":data-network", Configuration: "implementation"},
		},
		":data-network": {},
	})

	violations := ValidateLayers(cfg, graph)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(violations), violations)
	}
	v := violations[0]
	if v.SourceModule != ":app" || v.TargetModule != ":data-network" {
		t.Errorf("unexpected violation: %+v", v)
	}
	if v.SourceLayer != "ui" || v.TargetLayer != "data" {
		t.Errorf("unexpected layers: source=%q target=%q", v.SourceLayer, v.TargetLayer)
	}
}

func TestValidateLayers_UnknownModule(t *testing.T) {
	cfg := buildLayerConfig()
	graph := buildGraph(map[string][]module.Dependency{
		":app": {
			{ModulePath: ":unknown-lib", Configuration: "implementation"},
		},
		":unknown-lib": {
			{ModulePath: ":data-network", Configuration: "implementation"},
		},
		":data-network": {},
	})

	violations := ValidateLayers(cfg, graph)
	if len(violations) != 0 {
		t.Errorf("expected no violations for unknown module, got %d: %+v", len(violations), violations)
	}
}

func TestValidateLayers_EmptyConfig(t *testing.T) {
	graph := buildGraph(map[string][]module.Dependency{
		":app": {
			{ModulePath: ":data-network", Configuration: "implementation"},
		},
	})

	violations := ValidateLayers(nil, graph)
	if len(violations) != 0 {
		t.Errorf("expected no violations for nil config, got %d", len(violations))
	}
}

func TestValidateLayers_SameLayer(t *testing.T) {
	cfg := buildLayerConfig()
	graph := buildGraph(map[string][]module.Dependency{
		":feature-login": {
			{ModulePath: ":feature-home", Configuration: "implementation"},
		},
		":feature-home": {},
	})

	violations := ValidateLayers(cfg, graph)
	if len(violations) != 0 {
		t.Errorf("expected no violations for same-layer deps, got %d: %+v", len(violations), violations)
	}
}
