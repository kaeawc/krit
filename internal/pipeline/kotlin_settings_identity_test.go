package pipeline

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/diag"
	"github.com/kaeawc/krit/internal/gradlemodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// A settings-only change (same sources, same rules, same config) must refresh
// the findings bundle: the cached findings were produced under another
// language level, and the bundle fingerprint would otherwise still match.
func TestRunProject_FindingsBundleCache_RefreshesOnKotlinSettingsOnlyChange(t *testing.T) {
	dir := t.TempDir()
	cacheRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Sample.kt"), []byte("package test\n\nclass Sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rule := api.FakeRule("ClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) { ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared") }),
	)
	bundle := &recordingBundleStore{}
	var verbose bytes.Buffer
	run := func(t *testing.T, settings gradlemodel.KotlinSettings) []byte {
		t.Helper()
		verbose.Reset()
		res, err := RunProject(context.Background(), ProjectInput{
			Args: ProjectArgs{
				Config:       config.NewConfig(),
				Paths:        []string{dir},
				ActiveRules:  []*api.Rule{rule},
				Format:       "json",
				Version:      "test",
				OracleKotlin: settings,
			},
			Host: ProjectHostState{
				Reporter:                &diag.Reporter{Verbose: &verbose},
				FindingsBundleStore:     bundle,
				FindingsBundleCacheRoot: cacheRoot,
			},
		})
		if err != nil {
			t.Fatalf("RunProject: %v", err)
		}
		return scrubLines(res.JSON, []string{"\"durationMs\"", "\"timestamp\"", "\"wallSeconds\"", "\"startTime\"", "\"endTime\""})
	}

	old := gradlemodel.KotlinSettings{LanguageVersion: "1.9", APIVersion: "1.9"}
	modern := gradlemodel.KotlinSettings{LanguageVersion: "2.4", APIVersion: "2.4", Flags: []string{"-Xexplicit-backing-fields"}}

	coldOld := run(t, old)
	if bundle.saveCalls != 1 {
		t.Fatalf("cold run saves = %d, want 1", bundle.saveCalls)
	}
	warmOld := run(t, old)
	if bundle.saveCalls != 1 {
		t.Fatalf("identical settings must hit the bundle (saves = %d)", bundle.saveCalls)
	}
	if string(coldOld) != string(warmOld) {
		t.Fatalf("warm run differs from cold run under the same settings:\n%s\n%s", coldOld, warmOld)
	}

	run(t, modern)
	if bundle.saveCalls != 2 {
		t.Fatalf("a settings-only change must miss the bundle and rewrite it (saves = %d)", bundle.saveCalls)
	}
	if !strings.Contains(verbose.String(), "compilerSettings") {
		t.Fatalf("miss report does not name the changed component: %q", verbose.String())
	}

	// Switching back reuses the entry stored for the older settings, and no
	// settings at all is yet another identity.
	again := run(t, old)
	if bundle.saveCalls != 2 || string(again) != string(coldOld) {
		t.Fatalf("returning to the earlier settings must hit its own bundle (saves = %d)", bundle.saveCalls)
	}
	run(t, gradlemodel.KotlinSettings{})
	if bundle.saveCalls != 3 {
		t.Fatalf("dropping the settings must not reuse a settings-scoped bundle (saves = %d)", bundle.saveCalls)
	}
}

func TestKotlinSettingsAttrsReportDefaultsAndRejections(t *testing.T) {
	attrs := kotlinSettingsAttrs(gradlemodel.KotlinSettings{Notes: []string{"no model"}})
	if attrs["source"] != "embedded-default" || attrs["applied"] != "embedded compiler defaults" || attrs["notes"] != "no model" || attrs["identity"] != "" {
		t.Fatalf("defaults attrs = %v", attrs)
	}
	attrs = kotlinSettingsAttrs(gradlemodel.KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0", Rejected: []string{"-Werror (not an analysis-semantic setting krit applies)"}})
	if attrs["source"] != "gradle-model" || attrs["identity"] == "" || !strings.Contains(attrs["applied"], "-language-version=2.0") || !strings.Contains(attrs["notApplied"], "-Werror") {
		t.Fatalf("applied attrs = %v", attrs)
	}
}

func TestRunFingerprintDiffNamesCompilerSettings(t *testing.T) {
	a := scanner.RunFingerprint{Version: "v", CompilerSettings: "a"}
	b := a
	b.CompilerSettings = "b"
	if got := runFingerprintDiffFields(a, b); len(got) != 1 || got[0] != "compilerSettings" {
		t.Fatalf("diff = %v", got)
	}
}
