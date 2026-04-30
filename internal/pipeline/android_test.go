package pipeline

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/rules"
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

func TestRunActiveIconChecksColumnsIncludesIconColors(t *testing.T) {
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

	columns := RunActiveIconChecksColumns(idx, map[string]bool{"IconColors": true})
	if got := columns.Findings(); len(got) != 1 || got[0].Rule != "IconColors" {
		t.Fatalf("expected one IconColors finding, got %#v", got)
	}
}
