package pipeline

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestAndroidValuesScanKinds(t *testing.T) {
	tests := []struct {
		name string
		deps rules.AndroidDataDependency
		want android.ValuesScanKind
	}{
		{
			name: "none",
			deps: rules.AndroidDepNone,
			want: android.ValuesScanNone,
		},
		{
			name: "layout-only",
			deps: rules.AndroidDepLayout,
			want: android.ValuesScanNone,
		},
		{
			name: "strings",
			deps: rules.AndroidDepValuesStrings,
			want: android.ValuesScanStrings,
		},
		{
			name: "dimensions",
			deps: rules.AndroidDepValuesDimensions,
			want: android.ValuesScanDimensions,
		},
		{
			name: "plurals",
			deps: rules.AndroidDepValuesPlurals,
			want: android.ValuesScanPlurals,
		},
		{
			name: "arrays",
			deps: rules.AndroidDepValuesArrays,
			want: android.ValuesScanArrays,
		},
		{
			name: "extra-text",
			deps: rules.AndroidDepValuesExtraText,
			want: android.ValuesScanExtraText,
		},
		{
			name: "mixed",
			deps: rules.AndroidDepValuesStrings | rules.AndroidDepValuesPlurals,
			want: android.ValuesScanStrings | android.ValuesScanPlurals,
		},
		{
			name: "all-values",
			deps: rules.AndroidDepValues,
			want: android.ValuesScanAll,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := androidValuesScanKinds(tt.deps); got != tt.want {
				t.Fatalf("androidValuesScanKinds(%v) = %v, want %v", tt.deps, got, tt.want)
			}
		})
	}
}

func TestRunIconsIncludesIconColors(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-mdpi")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 24, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 24; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	f, err := os.Create(filepath.Join(dirPath, "ic_action_share.png"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("Encode: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	var iconColorsRule *v2rules.Rule
	for _, rule := range v2rules.Registry {
		if rule.ID == "IconColors" {
			iconColorsRule = rule
			break
		}
	}
	if iconColorsRule == nil {
		t.Fatal("IconColors not registered")
	}

	dispatcher := rules.NewDispatcherV2([]*v2rules.Rule{iconColorsRule})
	columns := dispatcher.RunIcons(&scanner.File{Path: resDir, Language: scanner.LangXML}, idx)
	if got := columns.Findings(); len(got) != 1 || got[0].Rule != "IconColors" {
		t.Fatalf("expected one IconColors finding, got %#v", got)
	}
}

func TestAndroidPhaseRunsIconRulesThroughV2Dispatcher(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-mdpi")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 48, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 48; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	f, err := os.Create(filepath.Join(dirPath, "ic_action_share.png"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("Encode: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	iconColorsRule := findV2RuleForTest(t, "IconColors")
	dispatcher := rules.NewDispatcherV2([]*v2rules.Rule{iconColorsRule})
	result, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
		Project:     &android.AndroidProject{ResDirs: []string{resDir}},
		ActiveRules: []*v2rules.Rule{iconColorsRule},
		Dispatcher:  dispatcher,
	})
	if err != nil {
		t.Fatalf("AndroidPhase.Run: %v", err)
	}
	findings := result.Findings.Findings()
	if len(findings) != 1 || findings[0].Rule != "IconColors" {
		t.Fatalf("expected one IconColors finding, got %#v", findings)
	}
}

func findV2RuleForTest(t *testing.T, id string) *v2rules.Rule {
	t.Helper()
	for _, rule := range v2rules.Registry {
		if rule.ID == id {
			return rule
		}
	}
	t.Fatalf("%s not registered", id)
	return nil
}
