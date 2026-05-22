package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func parseFixtureB(b *testing.B, relPath string) *scanner.File {
	b.Helper()
	abs, err := filepath.Abs(relPath)
	if err != nil {
		b.Fatalf("cannot resolve path %s: %v", relPath, err)
	}
	if _, err := os.Stat(abs); err != nil {
		b.Skipf("fixture not found: %s", abs)
	}
	f, err := scanner.ParseFile(context.Background(), abs)
	if err != nil {
		b.Fatalf("ParseFile(%s): %v", abs, err)
	}
	return f
}

func BenchmarkDispatcherRun_SmallFile(b *testing.B) {
	file := parseFixtureB(b, "../../tests/fixtures/positive/style/WildcardImport.kt")
	d := rules.NewDispatcher(api.Registry, nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Run(file)
	}
}

func BenchmarkDispatcherRun_LargeFile(b *testing.B) {
	file := parseFixtureB(b, "../../tests/fixtures/positive/complexity/LargeClass.kt")
	d := rules.NewDispatcher(api.Registry, nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Run(file)
	}
}

func BenchmarkDispatcherRun_SingleRule(b *testing.B) {
	file := parseFixtureB(b, "../../tests/fixtures/positive/complexity/LargeClass.kt")
	// Find a single dispatch rule to isolate overhead
	var single []*api.Rule
	for _, r := range api.Registry {
		if r.ID == "MagicNumber" {
			single = append(single, r)
			break
		}
	}
	if len(single) == 0 {
		b.Skip("MagicNumber rule not found in registry")
	}
	d := rules.NewDispatcher(single, nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Run(file)
	}
}

func BenchmarkDispatcherRun_AllRules(b *testing.B) {
	// Use ALL registered rules on the largest fixture
	file := parseFixtureB(b, "../../tests/fixtures/positive/complexity/LargeClass.kt")
	d := rules.NewDispatcher(api.Registry, nil)
	dispatched, lineRules, crossFile, moduleAware := d.Stats()
	b.Logf("rules: dispatched=%d line=%d cross-file=%d module-aware=%d",
		dispatched, lineRules, crossFile, moduleAware)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Run(file)
	}
}

func BenchmarkDispatcherConstruction(b *testing.B) {
	// Measure the cost of building the dispatcher from all rules
	for i := 0; i < b.N; i++ {
		_ = rules.NewDispatcher(api.Registry, nil)
	}
}

func BenchmarkDispatcherRun_SampleFile(b *testing.B) {
	file := parseFixtureB(b, "../../tests/fixtures/Sample.kt")
	d := rules.NewDispatcher(api.Registry, nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Run(file)
	}
}
