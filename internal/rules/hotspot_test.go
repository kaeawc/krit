package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestGodClassOrModuleRule_FlagsBroadImportModule(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AppCoordinator.kt")
	code := `package test

import alpha.analytics.AnalyticsClient
import beta.auth.SessionStore
import gamma.cache.MemoryCache
import delta.config.RuntimeConfig

class AppCoordinator
`
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	file, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	rule := &GodClassOrModuleRule{
		BaseRule:                BaseRule{RuleName: "GodClassOrModule", RuleSetName: "architecture", Sev: "warning"},
		AllowedDistinctPackages: 3,
	}
	findings := rule.CheckLines(file)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "4 distinct packages") {
		t.Fatalf("expected distinct package count in message, got %q", findings[0].Message)
	}
}

func TestGodClassOrModuleRule_IgnoresRepeatedImportPackages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "FeatureModule.kt")
	code := `package test

import alpha.analytics.AnalyticsClient
import alpha.analytics.AnalyticsEvent
import beta.auth.SessionStore
import beta.auth.SessionToken

class FeatureModule
`
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	file, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	rule := &GodClassOrModuleRule{
		BaseRule:                BaseRule{RuleName: "GodClassOrModule", RuleSetName: "architecture", Sev: "warning"},
		AllowedDistinctPackages: 2,
	}
	findings := rule.CheckLines(file)
	if len(findings) != 0 {
		t.Fatalf("expected repeated packages to stay below threshold, got %d findings", len(findings))
	}
}

func TestFanInFanOutHotspotRule_FlagsHighFanInClass(t *testing.T) {
	rule := &FanInFanOutHotspotRule{
		BaseRule:                BaseRule{RuleName: "FanInFanOutHotspot", RuleSetName: "architecture", Sev: "info"},
		AllowedFanIn:            2,
		IgnoreCommentReferences: true,
	}
	idx := scanner.BuildIndexFromData(
		[]scanner.Symbol{
			{Name: "SharedFormatter", Kind: "class", Visibility: "public", File: "SharedFormatter.kt", Line: 4},
		},
		[]scanner.Reference{
			{Name: "SharedFormatter", File: "SharedFormatter.kt", Line: 4, InComment: false},
			{Name: "SharedFormatter", File: "feature/A.kt", Line: 10, InComment: false},
			{Name: "SharedFormatter", File: "feature/B.kt", Line: 11, InComment: false},
		},
	)

	findings := rule.CheckCrossFile(idx)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "fan-in 2") {
		t.Fatalf("expected fan-in count in message, got %q", findings[0].Message)
	}
	if findings[0].File != "SharedFormatter.kt" {
		t.Fatalf("expected finding on declaration file, got %q", findings[0].File)
	}
}

func TestFanInFanOutHotspotRule_SkipsFrameworkEntryTypes(t *testing.T) {
	rule := &FanInFanOutHotspotRule{
		BaseRule:                BaseRule{RuleName: "FanInFanOutHotspot", RuleSetName: "architecture", Sev: "info"},
		AllowedFanIn:            2,
		IgnoreCommentReferences: true,
	}
	idx := scanner.BuildIndexFromData(
		[]scanner.Symbol{
			{Name: "MainActivity", Kind: "class", Visibility: "public", File: "MainActivity.kt", Line: 2},
		},
		[]scanner.Reference{
			{Name: "MainActivity", File: "MainActivity.kt", Line: 2, InComment: false},
			{Name: "MainActivity", File: "nav.xml", Line: 8, InComment: false},
			{Name: "MainActivity", File: "manifest.xml", Line: 3, InComment: false},
		},
	)

	findings := rule.CheckCrossFile(idx)
	if len(findings) != 0 {
		t.Fatalf("expected framework entry type to be skipped, got %d findings", len(findings))
	}
}

func TestFanInFanOutHotspotRule_IgnoresCommentOnlyUsageByDefault(t *testing.T) {
	rule := &FanInFanOutHotspotRule{
		BaseRule:                BaseRule{RuleName: "FanInFanOutHotspot", RuleSetName: "architecture", Sev: "info"},
		AllowedFanIn:            1,
		IgnoreCommentReferences: true,
	}
	idx := scanner.BuildIndexFromData(
		[]scanner.Symbol{
			{Name: "UtilityObject", Kind: "object", Visibility: "public", File: "UtilityObject.kt", Line: 1},
		},
		[]scanner.Reference{
			{Name: "UtilityObject", File: "UtilityObject.kt", Line: 1, InComment: false},
			{Name: "UtilityObject", File: "notes.md.kt", Line: 5, InComment: true},
		},
	)

	findings := rule.CheckCrossFile(idx)
	if len(findings) != 0 {
		t.Fatalf("expected comment-only usage to be ignored, got %d findings", len(findings))
	}
}
