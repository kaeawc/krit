package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func findResourceRule(t *testing.T, name string) *api.Rule {
	t.Helper()
	for _, r := range api.Registry {
		if r.Needs.Has(api.NeedsResources) && r.ID == name {
			return r
		}
	}
	t.Fatalf("resource rule %q not found in rule registry (NeedsResources)", name)
	return nil
}

func runResourceRule(r *api.Rule, idx *android.ResourceIndex) []scanner.Finding {
	collector := scanner.NewFindingCollector(0)
	ctx := &api.Context{
		ResourceIndex: idx,
		Rule:          r,
		Collector:     collector,
	}
	r.Check(ctx)
	return api.ContextFindings(ctx)
}

// helper: build a ResourceIndex with a single layout containing the given root view.
func indexWithLayout(name, filePath string, root *android.View) *android.ResourceIndex {
	layout := &android.Layout{Name: name, FilePath: filePath, RootView: root}
	return &android.ResourceIndex{
		Layouts: map[string]*android.Layout{
			name: layout,
		},
		LayoutConfigs: map[string]map[string]*android.Layout{
			name: {"": layout},
		},
		Strings:            make(map[string]string),
		Colors:             make(map[string]string),
		Dimensions:         make(map[string]string),
		DimensionsLocation: make(map[string]android.StringLocation),
		Styles:             make(map[string]*android.Style),
		StringArrays:       make(map[string][]string),
		Plurals:            make(map[string]map[string]string),
		Integers:           make(map[string]string),
		Booleans:           make(map[string]string),
		IDs:                make(map[string]bool),
	}
}

func emptyIndex() *android.ResourceIndex {
	return &android.ResourceIndex{
		Layouts:            make(map[string]*android.Layout),
		LayoutConfigs:      make(map[string]map[string]*android.Layout),
		Strings:            make(map[string]string),
		StringsLocation:    make(map[string]android.StringLocation),
		Colors:             make(map[string]string),
		Dimensions:         make(map[string]string),
		DimensionsLocation: make(map[string]android.StringLocation),
		Styles:             make(map[string]*android.Style),
		StringArrays:       make(map[string][]string),
		Plurals:            make(map[string]map[string]string),
		Integers:           make(map[string]string),
		Booleans:           make(map[string]string),
		IDs:                make(map[string]bool),
	}
}
