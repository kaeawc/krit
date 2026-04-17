package rules

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageNamingConventionDriftRule_FlagsPackageOutsideSourcePathPrefix(t *testing.T) {
	root := t.TempDir()
	file := writeAndParse(t,
		filepath.Join(root, "app", "src", "main", "kotlin", "com", "example", "feature", "foo"),
		"PackageNamingConventionDrift.kt",
		`package com.example.other.location

class PackageNamingConventionDrift
`)

	rule := &PackageNamingConventionDriftRule{
		FlatDispatchBase: FlatDispatchBase{},
		BaseRule:         BaseRule{RuleName: "PackageNamingConventionDrift", RuleSetName: "architecture", Sev: "info"},
	}
	findings := NewDispatcher([]Rule{rule}).Run(file)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "com.example.feature.foo") {
		t.Fatalf("expected finding to mention expected package prefix, got %q", findings[0].Message)
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

	rule := &PackageNamingConventionDriftRule{
		FlatDispatchBase: FlatDispatchBase{},
		BaseRule:         BaseRule{RuleName: "PackageNamingConventionDrift", RuleSetName: "architecture", Sev: "info"},
	}
	findings := NewDispatcher([]Rule{rule}).Run(file)

	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}
