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
	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func countRuleFindings(fc scanner.FindingColumns, ruleID string) int {
	n := 0
	for _, f := range fc.Findings() {
		if f.Rule == ruleID {
			n++
		}
	}
	return n
}

// TestRunProject_AffectedSetReplay_NoAndroidDoubleCount is the regression for
// the Android-findings double-count on the warm+ABI replay path. The findings
// bundle includes Android-phase findings, and the replay path returns
// bundleHit=false — so ApplyDelta carries the prior Android rows forward (their
// files are never in the affected set) while runAndroidPhaseAndMerge re-runs.
// The fix makes that phase replace its own rules' rows instead of appending;
// without it the IconColors finding is emitted twice on the second replay run.
func TestRunProject_AffectedSetReplay_NoAndroidDoubleCount(t *testing.T) {
	dir := t.TempDir()
	bundleRoot := t.TempDir()

	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a := filepath.Join(dir, "A.kt")
	write("A.kt", "package p\n\nclass Helper\n")
	write("B.kt", "package p\n\nclass Client {\n  fun make(): Helper = Helper()\n}\n")
	write("C.kt", "package p\n\nclass U1\n")
	write("D.kt", "package p\n\nclass U2\n")
	write("E.kt", "package p\n\nclass U3\n")
	write("AndroidManifest.xml", `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example"/>`)

	// A red 48x48 icon makes the IconColors rule emit exactly one finding.
	resDir := filepath.Join(dir, "res", "drawable-mdpi")
	if err := os.MkdirAll(resDir, 0o755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 48, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 48; x++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	pf, err := os.Create(filepath.Join(resDir, "ic_action_share.png"))
	if err != nil {
		t.Fatal(err)
	}
	_ = png.Encode(pf, img)
	pf.Close()

	icon := findV2RuleForTest(t, "IconColors")
	activeRules := []*api.Rule{crossFileHelperRule(), icon}
	proj := &android.Project{
		ManifestPaths: []string{filepath.Join(dir, "AndroidManifest.xml")},
		ResDirs:       []string{filepath.Join(dir, "res")},
	}

	run := func() ProjectResult {
		res, err := RunProject(context.Background(), ProjectInput{
			Args: ProjectArgs{Config: config.NewConfig(), Paths: []string{dir}, ActiveRules: activeRules, Format: "json", Version: "test"},
			Host: ProjectHostState{
				PrebuiltAndroidProject:  proj,
				CrossFileCacheDir:       scanner.CrossFileCacheDir(dir),
				FindingsBundleStore:     scanner.DiskFindingsBundleStore{},
				FindingsBundleCacheRoot: bundleRoot,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	r1 := run()
	if got := countRuleFindings(r1.FinalFindings, "IconColors"); got != 1 {
		t.Fatalf("cold run IconColors = %d, want 1", got)
	}

	// ABI edit on A.kt only; the icon is untouched.
	if err := os.WriteFile(a, []byte("package p\n\nclass Helper2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r2 := run()
	if got := countRuleFindings(r2.FinalFindings, "IconColors"); got != 1 {
		t.Errorf("warm replay IconColors = %d, want 1 (no double-count of Android findings)", got)
	}
}
