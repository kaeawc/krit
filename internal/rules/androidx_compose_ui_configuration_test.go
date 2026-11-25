package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

var usesFeatureNameRe = regexp.MustCompile(`<uses-feature[^>]*android:name="([^"]+)"`)

// TestConfigurationScreenWidthHeightLintParity replays the Lint CLI's findings for ConfigurationScreenWidthHeight
// over a two-module project, recorded per module with each module's manifest: Lint skips the wear module because
// its merged manifest declares android.hardware.type.watch, and files importing androidx.wear in the app module.
// The rule joins source with manifests, so it runs through RunManifestSource rather than the per-file dispatcher.
func TestConfigurationScreenWidthHeightLintParity(t *testing.T) {
	dir := filepath.Join("testdata", "androidx_compose_ui_manifest", "configuration_screen")
	var files []*scanner.File
	var manifests []*manifest.Manifest
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		switch {
		case strings.HasSuffix(path, ".kt"):
			f, err := scanner.ParseFile(context.Background(), path)
			if err != nil {
				return err
			}
			files = append(files, f)
		case filepath.Base(path) == "AndroidManifest.xml":
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			m := &manifest.Manifest{Path: path}
			for _, match := range usesFeatureNameRe.FindAllStringSubmatch(string(data), -1) {
				m.UsesFeatures = append(m.UsesFeatures, manifest.UsesFeature{Name: match[1]})
			}
			manifests = append(manifests, m)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "ConfigurationScreenWidthHeight" {
			rule = r
		}
	}
	if rule == nil {
		t.Fatal("ConfigurationScreenWidthHeight is not registered")
	}
	d := rules.NewDispatcher([]*api.Rule{rule}, nil)
	if len(d.ManifestSourceRules()) != 1 {
		t.Fatal("ConfigurationScreenWidthHeight is not a manifest-source rule")
	}
	cols, ok := d.RunManifestSource(manifests, files)
	if !ok {
		t.Fatal("RunManifestSource recovered a panic")
	}
	var got []string
	for _, f := range cols.Findings() {
		rel, _ := filepath.Rel(dir, f.File)
		got = append(got, fmt.Sprintf("%s:%d:%d: %s: %s", filepath.ToSlash(rel), f.Line, f.Col, f.Rule, f.Message))
	}
	sort.Strings(got)
	want := parityLines(readParityFile(t, filepath.Join(dir, "lint-1.12.1.txt")))
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("krit findings differ from Lint 1.12.1\n--- want\n%s\n--- got\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}

	// The pipeline may hand over absolute manifest paths with relative source paths (krit run on a relative
	// directory); the watch module must still be matched.
	absManifests := make([]*manifest.Manifest, 0, len(manifests))
	for _, m := range manifests {
		abs, err := filepath.Abs(m.Path)
		if err != nil {
			t.Fatal(err)
		}
		absManifests = append(absManifests, &manifest.Manifest{Path: abs, UsesFeatures: m.UsesFeatures})
	}
	if mixed, _ := d.RunManifestSource(absManifests, files); mixed.Len() != len(want) {
		t.Errorf("with absolute manifest paths: got %d findings, want %d", mixed.Len(), len(want))
	}

	// Without manifests the rule does not run, so a non-Android project reports nothing.
	if cols, _ := d.RunManifestSource(nil, files); cols.Len() != 0 {
		t.Errorf("expected no findings without manifests, got %d", cols.Len())
	}
}
