package rules

import (
	"path/filepath"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// packageNamingV2Rule looks up the PackageNamingConventionDrift rule from
// the rule registry, which has the correct dispatch wiring.
func packageNamingV2Rule(t *testing.T) *api.Rule {
	t.Helper()
	for _, r := range api.Registry {
		if r.ID == "PackageNamingConventionDrift" {
			return r
		}
	}
	t.Fatal("PackageNamingConventionDrift not found in api.Registry")
	return nil
}

func TestPackageNamingConventionDriftRule_FlagsPackageOutsideSourcePathPrefix(t *testing.T) {
	root := t.TempDir()
	file := writeAndParse(t,
		filepath.Join(root, "app", "src", "main", "kotlin", "com", "example", "feature", "foo"),
		"PackageNamingConventionDrift.kt",
		`package com.example.other.location

class PackageNamingConventionDrift
`)

	columns := NewDispatcher([]*api.Rule{packageNamingV2Rule(t)}, nil).Run(file)

	if columns.Len() != 1 {
		t.Fatalf("expected 1 finding, got %d", columns.Len())
	}
	if !strings.Contains(columns.MessageAt(0), "com.example.feature.foo") {
		t.Fatalf("expected finding to mention expected package prefix, got %q", columns.MessageAt(0))
	}
}

func TestPackageNamingConventionDriftRule_AcceptsNestedPackageBelowSourcePathPrefix(t *testing.T) {
	root := t.TempDir()
	file := writeAndParse(t,
		filepath.Join(root, "app", "src", "main", "kotlin", "com", "example", "feature", "foo"),
		"PackageNamingConventionDrift.kt",
		`package com.example.feature.foo.ui

class PackageNamingConventionDrift
`)

	columns := NewDispatcher([]*api.Rule{packageNamingV2Rule(t)}, nil).Run(file)

	if columns.Len() != 0 {
		t.Fatalf("expected 0 findings, got %d", columns.Len())
	}
}
