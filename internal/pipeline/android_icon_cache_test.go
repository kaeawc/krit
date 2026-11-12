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
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func runIconPhase(t *testing.T, resDir, cacheDir string) []scanner.Finding {
	t.Helper()
	rule := findV2RuleForTest(t, "IconLauncherShape")
	writer := scanner.NewAndroidCacheWriter(2)
	res, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
		Project:        &android.Project{ResDirs: []string{resDir}},
		ActiveRules:    []*api.Rule{rule},
		Dispatcher:     rules.NewDispatcher([]*api.Rule{rule}, nil),
		RuleHash:       "test-rule-hash",
		LibraryFactsFP: "test-lf",
		CacheDir:       cacheDir,
		CacheWriter:    writer,
	})
	if err != nil {
		t.Fatalf("AndroidPhase.Run: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("flush writer: %v", err)
	}
	return res.Findings.Findings()
}

// A manifest-only edit that makes an existing bitmap the application icon must recompute icon findings.
func TestAndroidIconCache_ManifestEditInvalidates(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dir := filepath.Join(resDir, "drawable-mdpi")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 48, 48))
	for y := range 48 {
		for x := range 48 {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	f, err := os.Create(filepath.Join(dir, "app_logo.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")

	writeManifest(t, root, `<manifest xmlns:android="http://schemas.android.com/apk/res/android"><application/></manifest>`)
	if got := runIconPhase(t, resDir, cacheDir); len(got) != 0 {
		t.Fatalf("want no findings before the icon is assigned, got %#v", got)
	}

	writeManifest(t, root, `<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <application android:icon="@drawable/app_logo"/>
</manifest>`)
	got := runIconPhase(t, resDir, cacheDir)
	if len(got) != 1 || got[0].Rule != "IconLauncherShape" {
		t.Fatalf("want one IconLauncherShape finding after the manifest assigns the icon, got %#v", got)
	}
}
