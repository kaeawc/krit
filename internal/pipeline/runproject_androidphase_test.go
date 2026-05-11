package pipeline

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestRunProject_RunsAndroidPhaseAndMergesFindings is the contract
// that RunProject (the daemon's entry point) executes the Android
// phase and folds its findings into the result, mirroring the CLI
// runner's androidPhase step. Without this wiring, daemon-served
// runs over Android projects silently drop manifest/resource/gradle
// findings.
func TestRunProject_RunsAndroidPhaseAndMergesFindings(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "src", "main", "res", "drawable-mdpi")
	if err := os.MkdirAll(resDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 48, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 48; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	f, err := os.Create(filepath.Join(resDir, "ic_action_share.png"))
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

	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{root},
			ActiveRules: []*api.Rule{iconColorsRule},
			Format:      "json",
			Version:     "test",
		},
	})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}

	got := res.FinalFindings.Findings()
	hits := 0
	for _, f := range got {
		if f.Rule == "IconColors" {
			hits++
		}
	}
	if hits == 0 {
		t.Fatalf("expected at least one IconColors finding via RunProject, got %d findings: %#v", len(got), got)
	}
}
