package arch

import (
	"testing"

	"github.com/kaeawc/krit/internal/config"
)

func buildTestConfig() *config.Config {
	cfg := config.NewConfig()
	// Build the nested structure manually via Data()
	// architecture.LayerDependencyViolation.layers and .allowed
	layers := []interface{}{
		map[string]interface{}{
			"name":    "ui",
			"modules": []interface{}{":app", ":feature-*"},
		},
		map[string]interface{}{
			"name":    "domain",
			"modules": []interface{}{":domain-*"},
		},
		map[string]interface{}{
			"name":    "data",
			"modules": []interface{}{":data-*"},
		},
	}
	allowed := map[string]interface{}{
		"ui":     []interface{}{"domain"},
		"domain": []interface{}{"data"},
	}
	data := cfg.Data()
	data["architecture"] = map[string]interface{}{
		"LayerDependencyViolation": map[string]interface{}{
			"active":  true,
			"layers":  layers,
			"allowed": allowed,
		},
	}
	return cfg
}

func TestParseLayerConfig_Valid(t *testing.T) {
	cfg := buildTestConfig()
	lc := ParseLayerConfig(cfg)
	if lc == nil {
		t.Fatal("expected non-nil LayerConfig")
	}
	if len(lc.Layers) != 3 {
		t.Fatalf("expected 3 layers, got %d", len(lc.Layers))
	}
	if lc.Layers[0].Name != "ui" {
		t.Errorf("expected first layer name 'ui', got %q", lc.Layers[0].Name)
	}
	if len(lc.Layers[0].Modules) != 2 {
		t.Errorf("expected 2 modules in ui layer, got %d", len(lc.Layers[0].Modules))
	}
	if len(lc.Allowed) != 2 {
		t.Errorf("expected 2 allowed entries, got %d", len(lc.Allowed))
	}
	uiAllowed := lc.Allowed["ui"]
	if len(uiAllowed) != 1 || uiAllowed[0] != "domain" {
		t.Errorf("expected ui allowed=[domain], got %v", uiAllowed)
	}
}

func TestParseLayerConfig_Missing(t *testing.T) {
	// Nil config
	if lc := ParseLayerConfig(nil); lc != nil {
		t.Error("expected nil for nil config")
	}

	// Empty config
	cfg := config.NewConfig()
	if lc := ParseLayerConfig(cfg); lc != nil {
		t.Error("expected nil for empty config")
	}

	// Config with architecture but no LayerDependencyViolation
	cfg2 := config.NewConfig()
	cfg2.Data()["architecture"] = map[string]interface{}{
		"SomeOtherRule": map[string]interface{}{"active": true},
	}
	if lc := ParseLayerConfig(cfg2); lc != nil {
		t.Error("expected nil when LayerDependencyViolation is absent")
	}
}

func TestLayerForModule_ExactMatch(t *testing.T) {
	cfg := buildTestConfig()
	lc := ParseLayerConfig(cfg)
	if lc == nil {
		t.Fatal("expected non-nil LayerConfig")
	}

	layer := lc.LayerForModule(":app")
	if layer != "ui" {
		t.Errorf("expected 'ui' for :app, got %q", layer)
	}
}

func TestLayerForModule_GlobMatch(t *testing.T) {
	cfg := buildTestConfig()
	lc := ParseLayerConfig(cfg)
	if lc == nil {
		t.Fatal("expected non-nil LayerConfig")
	}

	layer := lc.LayerForModule(":feature-login")
	if layer != "ui" {
		t.Errorf("expected 'ui' for :feature-login, got %q", layer)
	}

	layer = lc.LayerForModule(":domain-auth")
	if layer != "domain" {
		t.Errorf("expected 'domain' for :domain-auth, got %q", layer)
	}

	layer = lc.LayerForModule(":data-network")
	if layer != "data" {
		t.Errorf("expected 'data' for :data-network, got %q", layer)
	}
}

func TestLayerForModule_NoMatch(t *testing.T) {
	cfg := buildTestConfig()
	lc := ParseLayerConfig(cfg)
	if lc == nil {
		t.Fatal("expected non-nil LayerConfig")
	}

	layer := lc.LayerForModule(":unknown-module")
	if layer != "" {
		t.Errorf("expected empty string for unmatched module, got %q", layer)
	}
}

func TestMatchModuleGlob(t *testing.T) {
	tests := []struct {
		pattern  string
		module   string
		expected bool
	}{
		// Exact match
		{":app", ":app", true},
		{":app", ":app2", false},
		// Wildcard suffix
		{":feature-*", ":feature-login", true},
		{":feature-*", ":feature-home", true},
		{":feature-*", ":feature-", true},
		{":feature-*", ":feat-login", false},
		// Wildcard prefix
		{"*-core", ":data-core", true},
		{"*-core", ":domain-core", true},
		{"*-core", ":core", false},
		// Wildcard in middle
		{":feature-*-impl", ":feature-login-impl", true},
		{":feature-*-impl", ":feature-login-api", false},
		// Just wildcard
		{"*", ":anything", true},
		{"*", "", true},
	}

	for _, tt := range tests {
		got := matchModuleGlob(tt.pattern, tt.module)
		if got != tt.expected {
			t.Errorf("matchModuleGlob(%q, %q) = %v, want %v",
				tt.pattern, tt.module, got, tt.expected)
		}
	}
}
