package fixer

import (
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// writeStaticPNG creates a minimal valid static PNG file at path.
func writeStaticPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.White)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create PNG: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
}

// writeAnimatedGIF creates a GIF file with multiple frames at path.
func writeAnimatedGIF(t *testing.T, path string) {
	t.Helper()
	g := &gif.GIF{}
	for i := 0; i < 3; i++ {
		frame := image.NewPaletted(image.Rect(0, 0, 2, 2), []color.Color{color.White, color.Black})
		g.Image = append(g.Image, frame)
		g.Delay = append(g.Delay, 10)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create GIF: %v", err)
	}
	defer f.Close()
	if err := gif.EncodeAll(f, g); err != nil {
		t.Fatalf("encode GIF: %v", err)
	}
}

// writeStaticGIF creates a single-frame GIF file at path.
func writeStaticGIF(t *testing.T, path string) {
	t.Helper()
	frame := image.NewPaletted(image.Rect(0, 0, 2, 2), []color.Color{color.White, color.Black})
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create GIF: %v", err)
	}
	defer f.Close()
	if err := gif.Encode(f, frame, nil); err != nil {
		t.Fatalf("encode GIF: %v", err)
	}
}

func TestApplyBinaryFixes_SkipsNilBinaryFix(t *testing.T) {
	findings := []scanner.Finding{
		{
			File:    "test.kt",
			Rule:    "SomeRule",
			Message: "some message",
			// BinaryFix is nil
		},
		{
			File:    "test2.kt",
			Rule:    "AnotherRule",
			Message: "another message",
			Fix:     &scanner.Fix{StartLine: 1, EndLine: 1, Replacement: "x"},
			// BinaryFix is nil
		},
	}

	applied, errors := ApplyBinaryFixes(findings, false)
	if applied != 0 {
		t.Errorf("expected 0 applied, got %d", applied)
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(errors), errors)
	}
}

func TestApplyBinaryFixes_EmptyFindings(t *testing.T) {
	applied, errors := ApplyBinaryFixes(nil, false)
	if applied != 0 {
		t.Errorf("expected 0 applied, got %d", applied)
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errors))
	}
}

func TestApplyBinaryFixes_CwebpNotFound(t *testing.T) {
	// Override PATH to ensure cwebp is not found
	t.Setenv("PATH", t.TempDir())

	findings := []scanner.Finding{
		{
			File:    "res/drawable-xxhdpi/large_bg.png",
			Rule:    "ConvertToWebp",
			Message: "PNG file is large",
			BinaryFix: &scanner.BinaryFix{
				Type:        scanner.BinaryFixConvertWebP,
				SourcePath:  "res/drawable-xxhdpi/large_bg.png",
				TargetPath:  "",
				Description: "Convert to WebP format",
			},
		},
	}

	applied, errors := ApplyBinaryFixes(findings, false)
	if applied != 0 {
		t.Errorf("expected 0 applied when cwebp missing, got %d", applied)
	}
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errors), errors)
	}
	if errors[0] == nil || !containsStr(errors[0].Error(), "cwebp not found") {
		t.Errorf("expected 'cwebp not found' error, got: %v", errors[0])
	}
}

func TestApplyBinaryFixes_DryRunWithCwebp(t *testing.T) {
	// Check if cwebp is available; skip test otherwise since dry-run
	// still validates cwebp presence.
	_, err := exec.LookPath("cwebp")
	if err != nil {
		t.Skip("cwebp not available, skipping dry-run test")
	}

	findings := []scanner.Finding{
		{
			File:    "res/drawable-xxhdpi/large_bg.png",
			Rule:    "ConvertToWebp",
			Message: "PNG file is large",
			BinaryFix: &scanner.BinaryFix{
				Type:        scanner.BinaryFixConvertWebP,
				SourcePath:  "res/drawable-xxhdpi/large_bg.png",
				TargetPath:  "",
				Description: "Convert to WebP format",
			},
		},
	}

	applied, errors := ApplyBinaryFixes(findings, true)
	if applied != 1 {
		t.Errorf("expected 1 applied in dry-run, got %d", applied)
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors in dry-run, got %d: %v", len(errors), errors)
	}
}

func TestApplyBinaryFixes_MultipleFindingsMixed(t *testing.T) {
	// Override PATH to ensure cwebp is not found
	t.Setenv("PATH", t.TempDir())

	findings := []scanner.Finding{
		{
			File:    "test.kt",
			Rule:    "SomeRule",
			Message: "no binary fix",
		},
		{
			File:    "res/icon.png",
			Rule:    "ConvertToWebp",
			Message: "large PNG",
			BinaryFix: &scanner.BinaryFix{
				Type:        scanner.BinaryFixConvertWebP,
				SourcePath:  "res/icon.png",
				TargetPath:  "",
				Description: "Convert to WebP format",
			},
		},
		{
			File:    "another.kt",
			Rule:    "AnotherRule",
			Message: "another non-binary finding",
		},
	}

	applied, errors := ApplyBinaryFixes(findings, false)
	if applied != 0 {
		t.Errorf("expected 0 applied (cwebp missing), got %d", applied)
	}
	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}
}

func TestConvertToWebP_TargetPathGeneration(t *testing.T) {
	// Override PATH to ensure cwebp is not found so we get a predictable error
	t.Setenv("PATH", t.TempDir())

	err := convertToWebP("/some/path/icon.png", "", false)
	if err == nil {
		t.Fatal("expected error when cwebp not found")
	}
	if !containsStr(err.Error(), "cwebp not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConvertToWebP_CustomTargetPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := convertToWebP("/some/path/icon.png", "/other/path/icon.webp", false)
	if err == nil {
		t.Fatal("expected error when cwebp not found")
	}
	if !containsStr(err.Error(), "cwebp not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Milestone 2 tests ---

func TestApplyBinaryFixes_DeleteFile(t *testing.T) {
	// Create a temporary file to delete.
	tmp := t.TempDir()
	target := filepath.Join(tmp, "obsolete.png")
	if err := os.WriteFile(target, []byte("png data"), 0644); err != nil {
		t.Fatal(err)
	}

	findings := []scanner.Finding{
		{
			File:    target,
			Rule:    "ConvertToWebp",
			Message: "delete old png",
			BinaryFix: &scanner.BinaryFix{
				Type:        scanner.BinaryFixDeleteFile,
				SourcePath:  target,
				Description: "Remove obsolete PNG",
			},
		},
	}

	applied, errors := ApplyBinaryFixes(findings, false)
	if applied != 1 {
		t.Errorf("expected 1 applied, got %d", applied)
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(errors), errors)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("expected file to be deleted, but it still exists")
	}
}

func TestApplyBinaryFixes_DeleteFile_DryRun(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "keep.png")
	if err := os.WriteFile(target, []byte("png data"), 0644); err != nil {
		t.Fatal(err)
	}

	findings := []scanner.Finding{
		{
			File:    target,
			Rule:    "ConvertToWebp",
			Message: "delete old png",
			BinaryFix: &scanner.BinaryFix{
				Type:        scanner.BinaryFixDeleteFile,
				SourcePath:  target,
				Description: "Remove obsolete PNG",
			},
		},
	}

	applied, errors := ApplyBinaryFixes(findings, true)
	if applied != 1 {
		t.Errorf("expected 1 applied in dry-run, got %d", applied)
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(errors), errors)
	}
	// File must still exist in dry-run mode.
	if _, err := os.Stat(target); err != nil {
		t.Errorf("expected file to still exist in dry-run, got error: %v", err)
	}
}

func TestApplyBinaryFixes_DeleteFile_Missing(t *testing.T) {
	findings := []scanner.Finding{
		{
			File:    "/nonexistent/path/gone.png",
			Rule:    "ConvertToWebp",
			Message: "delete missing file",
			BinaryFix: &scanner.BinaryFix{
				Type:        scanner.BinaryFixDeleteFile,
				SourcePath:  "/nonexistent/path/gone.png",
				Description: "Remove missing file",
			},
		},
	}

	applied, errors := ApplyBinaryFixes(findings, false)
	if applied != 0 {
		t.Errorf("expected 0 applied for missing file, got %d", applied)
	}
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	if !containsStr(errors[0].Error(), "delete") {
		t.Errorf("expected delete error, got: %v", errors[0])
	}
}

func TestApplyBinaryFixes_DeleteSourceAfterConvert_NoCwebp(t *testing.T) {
	// When cwebp is missing, conversion fails and source must NOT be deleted.
	t.Setenv("PATH", t.TempDir())

	tmp := t.TempDir()
	src := filepath.Join(tmp, "icon.png")
	if err := os.WriteFile(src, []byte("png data"), 0644); err != nil {
		t.Fatal(err)
	}

	findings := []scanner.Finding{
		{
			File:    src,
			Rule:    "ConvertToWebp",
			Message: "convert",
			BinaryFix: &scanner.BinaryFix{
				Type:         scanner.BinaryFixConvertWebP,
				SourcePath:   src,
				DeleteSource: true,
				Description:  "Convert and delete source",
			},
		},
	}

	applied, errors := ApplyBinaryFixes(findings, false)
	if applied != 0 {
		t.Errorf("expected 0 applied (cwebp missing), got %d", applied)
	}
	if len(errors) == 0 {
		t.Fatal("expected error for missing cwebp")
	}
	// Source must still exist since conversion failed.
	if _, err := os.Stat(src); err != nil {
		t.Errorf("source should still exist after failed conversion: %v", err)
	}
}

func TestApplyBinaryFixesBatch_EmptyFindings(t *testing.T) {
	applied, errors := ApplyBinaryFixesBatch(nil, false)
	if applied != 0 {
		t.Errorf("expected 0 applied, got %d", applied)
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errors))
	}
}

func TestApplyBinaryFixesBatch_DeletesOnly(t *testing.T) {
	tmp := t.TempDir()
	f1 := filepath.Join(tmp, "a.png")
	f2 := filepath.Join(tmp, "b.png")
	os.WriteFile(f1, []byte("a"), 0644)
	os.WriteFile(f2, []byte("b"), 0644)

	findings := []scanner.Finding{
		{
			File: f1, Rule: "ConvertToWebp", Message: "del",
			BinaryFix: &scanner.BinaryFix{
				Type: scanner.BinaryFixDeleteFile, SourcePath: f1,
			},
		},
		{
			File: f2, Rule: "ConvertToWebp", Message: "del",
			BinaryFix: &scanner.BinaryFix{
				Type: scanner.BinaryFixDeleteFile, SourcePath: f2,
			},
		},
	}

	applied, errors := ApplyBinaryFixesBatch(findings, false)
	if applied != 2 {
		t.Errorf("expected 2 applied, got %d", applied)
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %v", errors)
	}
	for _, p := range []string{f1, f2} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expected %s to be deleted", p)
		}
	}
}

func TestApplyBinaryFixesBatch_DryRunDeletesPreserved(t *testing.T) {
	tmp := t.TempDir()
	f1 := filepath.Join(tmp, "a.png")
	os.WriteFile(f1, []byte("a"), 0644)

	findings := []scanner.Finding{
		{
			File: f1, Rule: "ConvertToWebp", Message: "del",
			BinaryFix: &scanner.BinaryFix{
				Type: scanner.BinaryFixDeleteFile, SourcePath: f1,
			},
		},
	}

	applied, errors := ApplyBinaryFixesBatch(findings, true)
	if applied != 1 {
		t.Errorf("expected 1 applied in dry-run, got %d", applied)
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %v", errors)
	}
	// File must still exist.
	if _, err := os.Stat(f1); err != nil {
		t.Errorf("file should still exist in dry-run: %v", err)
	}
}

func TestApplyBinaryFixesBatch_OrderConvertBeforeDelete(t *testing.T) {
	// When cwebp is missing, conversions fail and DeleteSource should not
	// remove the original. The batch function must process conversions first.
	t.Setenv("PATH", t.TempDir())

	tmp := t.TempDir()
	src := filepath.Join(tmp, "icon.png")
	os.WriteFile(src, []byte("png"), 0644)

	findings := []scanner.Finding{
		// Explicit delete finding placed FIRST to verify ordering.
		{
			File: src, Rule: "ConvertToWebp", Message: "del",
			BinaryFix: &scanner.BinaryFix{
				Type: scanner.BinaryFixDeleteFile, SourcePath: src,
			},
		},
		// Conversion with DeleteSource.
		{
			File: src, Rule: "ConvertToWebp", Message: "convert",
			BinaryFix: &scanner.BinaryFix{
				Type:         scanner.BinaryFixConvertWebP,
				SourcePath:   src,
				DeleteSource: true,
				Description:  "Convert and remove",
			},
		},
	}

	applied, errors := ApplyBinaryFixesBatch(findings, false)
	// Conversion fails (no cwebp) -> 0 from conversion.
	// Explicit delete should still succeed -> 1.
	if applied != 1 {
		t.Errorf("expected 1 applied (only the explicit delete), got %d", applied)
	}
	// 1 error from cwebp missing.
	if len(errors) != 1 {
		t.Errorf("expected 1 error (cwebp), got %d: %v", len(errors), errors)
	}
	// Source should be deleted by the explicit BinaryFixDeleteFile.
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("expected file to be deleted by explicit delete finding")
	}
}

func TestApplyBinaryFixesBatch_SkipsNilBinaryFix(t *testing.T) {
	findings := []scanner.Finding{
		{File: "test.kt", Rule: "SomeRule", Message: "no fix"},
	}

	applied, errors := ApplyBinaryFixesBatch(findings, false)
	if applied != 0 {
		t.Errorf("expected 0 applied, got %d", applied)
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errors))
	}
}

// --- Milestone 3 tests ---

func TestApplyBinaryFixes_CreateFile(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "subdir", "new_icon.webp")
	content := []byte("fake webp data")

	findings := []scanner.Finding{
		{
			File:    "source.png",
			Rule:    "ConvertToWebp",
			Message: "create file",
			BinaryFix: &scanner.BinaryFix{
				Type:       scanner.BinaryFixCreateFile,
				TargetPath: target,
				Content:    content,
			},
		},
	}

	applied, errors := ApplyBinaryFixes(findings, false)
	if applied != 1 {
		t.Errorf("expected 1 applied, got %d", applied)
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(errors), errors)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("file content mismatch: got %q, want %q", got, content)
	}
}

func TestApplyBinaryFixes_CreateFile_DryRun(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "subdir", "new_icon.webp")

	findings := []scanner.Finding{
		{
			File:    "source.png",
			Rule:    "ConvertToWebp",
			Message: "create file dry-run",
			BinaryFix: &scanner.BinaryFix{
				Type:       scanner.BinaryFixCreateFile,
				TargetPath: target,
				Content:    []byte("data"),
			},
		},
	}

	applied, errors := ApplyBinaryFixes(findings, true)
	if applied != 1 {
		t.Errorf("expected 1 applied in dry-run, got %d", applied)
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(errors), errors)
	}
	// File must NOT exist in dry-run mode.
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("expected file to not exist in dry-run")
	}
}

func TestApplyBinaryFixes_MoveFile(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "old_location.png")
	dst := filepath.Join(tmp, "new_dir", "new_location.png")
	if err := os.WriteFile(src, []byte("image data"), 0644); err != nil {
		t.Fatal(err)
	}

	findings := []scanner.Finding{
		{
			File:    src,
			Rule:    "IconLocation",
			Message: "move file",
			BinaryFix: &scanner.BinaryFix{
				Type:       scanner.BinaryFixMoveFile,
				SourcePath: src,
				TargetPath: dst,
			},
		},
	}

	applied, errors := ApplyBinaryFixes(findings, false)
	if applied != 1 {
		t.Errorf("expected 1 applied, got %d", applied)
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(errors), errors)
	}
	// Source should not exist.
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("expected source to be removed after move")
	}
	// Destination should exist.
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("expected destination to exist: %v", err)
	}
	if string(got) != "image data" {
		t.Errorf("content mismatch: got %q", got)
	}
}

func TestApplyBinaryFixes_MoveFile_DryRun(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "old.png")
	dst := filepath.Join(tmp, "moved", "old.png")
	if err := os.WriteFile(src, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	findings := []scanner.Finding{
		{
			File:    src,
			Rule:    "IconLocation",
			Message: "move dry-run",
			BinaryFix: &scanner.BinaryFix{
				Type:       scanner.BinaryFixMoveFile,
				SourcePath: src,
				TargetPath: dst,
			},
		},
	}

	applied, errors := ApplyBinaryFixes(findings, true)
	if applied != 1 {
		t.Errorf("expected 1 applied in dry-run, got %d", applied)
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(errors), errors)
	}
	// Source must still exist.
	if _, err := os.Stat(src); err != nil {
		t.Errorf("source should still exist in dry-run: %v", err)
	}
	// Destination must NOT exist.
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Errorf("destination should not exist in dry-run")
	}
}

func TestApplyBinaryFixes_MoveFile_MissingSource(t *testing.T) {
	tmp := t.TempDir()
	findings := []scanner.Finding{
		{
			File:    "/nonexistent/src.png",
			Rule:    "IconLocation",
			Message: "move missing source",
			BinaryFix: &scanner.BinaryFix{
				Type:       scanner.BinaryFixMoveFile,
				SourcePath: "/nonexistent/src.png",
				TargetPath: filepath.Join(tmp, "dst.png"),
			},
		},
	}

	applied, errors := ApplyBinaryFixes(findings, false)
	if applied != 0 {
		t.Errorf("expected 0 applied, got %d", applied)
	}
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	if !containsStr(errors[0].Error(), "move") {
		t.Errorf("expected move error, got: %v", errors[0])
	}
}

func TestApplyBinaryFixes_OptimizePNG_NoTool(t *testing.T) {
	// Override PATH to ensure neither optipng nor pngcrush is found.
	t.Setenv("PATH", t.TempDir())

	tmp := t.TempDir()
	src := filepath.Join(tmp, "icon.png")
	if err := os.WriteFile(src, []byte("png"), 0644); err != nil {
		t.Fatal(err)
	}

	findings := []scanner.Finding{
		{
			File:    src,
			Rule:    "OptimizePNG",
			Message: "optimize",
			BinaryFix: &scanner.BinaryFix{
				Type:       scanner.BinaryFixOptimizePNG,
				SourcePath: src,
			},
		},
	}

	applied, errors := ApplyBinaryFixes(findings, false)
	if applied != 0 {
		t.Errorf("expected 0 applied when no PNG optimizer, got %d", applied)
	}
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errors), errors)
	}
	if !containsStr(errors[0].Error(), "no PNG optimizer found") {
		t.Errorf("expected 'no PNG optimizer found' error, got: %v", errors[0])
	}
}

func TestApplyBinaryFixes_OptimizePNG_DryRun_NoTool(t *testing.T) {
	// With no tool, dry-run should still error (tool check happens first).
	t.Setenv("PATH", t.TempDir())

	findings := []scanner.Finding{
		{
			File:    "/some/icon.png",
			Rule:    "OptimizePNG",
			Message: "optimize",
			BinaryFix: &scanner.BinaryFix{
				Type:       scanner.BinaryFixOptimizePNG,
				SourcePath: "/some/icon.png",
			},
		},
	}

	applied, errors := ApplyBinaryFixes(findings, true)
	if applied != 0 {
		t.Errorf("expected 0 applied, got %d", applied)
	}
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
}

func TestApplyBinaryFixes_HintOnly_Skipped(t *testing.T) {
	findings := []scanner.Finding{
		{
			File:    "/some/icon.png",
			Rule:    "IconExpectedSize",
			Message: "wrong size",
			BinaryFix: &scanner.BinaryFix{
				Type:       scanner.BinaryFixCreateFile,
				SourcePath: "/some/icon.png",
				TargetPath: "/some/icon.png",
				HintOnly:   true,
				Content:    []byte("should not be written"),
			},
		},
	}

	applied, errors := ApplyBinaryFixes(findings, false)
	if applied != 0 {
		t.Errorf("expected 0 applied for hint-only fix, got %d", applied)
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(errors), errors)
	}
}

func TestApplyBinaryFixesBatch_HintOnly_Skipped(t *testing.T) {
	findings := []scanner.Finding{
		{
			File:    "/some/icon.png",
			Rule:    "IconExpectedSize",
			Message: "wrong size",
			BinaryFix: &scanner.BinaryFix{
				Type:       scanner.BinaryFixCreateFile,
				SourcePath: "/some/icon.png",
				TargetPath: "/some/icon.png",
				HintOnly:   true,
			},
		},
	}

	applied, errors := ApplyBinaryFixesBatch(findings, false)
	if applied != 0 {
		t.Errorf("expected 0 applied for hint-only fix, got %d", applied)
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(errors), errors)
	}
}

func TestApplyBinaryFixesBatch_CreateFile(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "deep", "dir", "icon.webp")

	findings := []scanner.Finding{
		{
			File:    "source.png",
			Rule:    "ConvertToWebp",
			Message: "create",
			BinaryFix: &scanner.BinaryFix{
				Type:       scanner.BinaryFixCreateFile,
				TargetPath: target,
				Content:    []byte("webp bytes"),
			},
		},
	}

	applied, errors := ApplyBinaryFixesBatch(findings, false)
	if applied != 1 {
		t.Errorf("expected 1 applied, got %d", applied)
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %v", errors)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("file should exist: %v", err)
	}
	if string(got) != "webp bytes" {
		t.Errorf("content mismatch: %q", got)
	}
}

func TestApplyBinaryFixesBatch_MoveFile(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "drawable", "icon.png")
	dst := filepath.Join(tmp, "mipmap", "icon.png")
	os.MkdirAll(filepath.Dir(src), 0755)
	os.WriteFile(src, []byte("icon"), 0644)

	findings := []scanner.Finding{
		{
			File:    src,
			Rule:    "IconLocation",
			Message: "move",
			BinaryFix: &scanner.BinaryFix{
				Type:       scanner.BinaryFixMoveFile,
				SourcePath: src,
				TargetPath: dst,
			},
		},
	}

	applied, errors := ApplyBinaryFixesBatch(findings, false)
	if applied != 1 {
		t.Errorf("expected 1 applied, got %d", applied)
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %v", errors)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source should not exist after move")
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("dest should exist: %v", err)
	}
	if string(got) != "icon" {
		t.Errorf("content mismatch")
	}
}

func TestApplyBinaryFixesBatch_MixedTypes(t *testing.T) {
	// Test a batch with create, move, delete to verify ordering.
	tmp := t.TempDir()

	delTarget := filepath.Join(tmp, "delete_me.png")
	os.WriteFile(delTarget, []byte("del"), 0644)

	moveSrc := filepath.Join(tmp, "move_src.png")
	moveDst := filepath.Join(tmp, "moved", "move_dst.png")
	os.WriteFile(moveSrc, []byte("move"), 0644)

	createTarget := filepath.Join(tmp, "created.webp")

	findings := []scanner.Finding{
		{
			File: delTarget, Rule: "Cleanup", Message: "del",
			BinaryFix: &scanner.BinaryFix{
				Type: scanner.BinaryFixDeleteFile, SourcePath: delTarget,
			},
		},
		{
			File: moveSrc, Rule: "IconLocation", Message: "move",
			BinaryFix: &scanner.BinaryFix{
				Type: scanner.BinaryFixMoveFile, SourcePath: moveSrc, TargetPath: moveDst,
			},
		},
		{
			File: "source.png", Rule: "ConvertToWebp", Message: "create",
			BinaryFix: &scanner.BinaryFix{
				Type: scanner.BinaryFixCreateFile, TargetPath: createTarget, Content: []byte("new"),
			},
		},
	}

	applied, errors := ApplyBinaryFixesBatch(findings, false)
	if applied != 3 {
		t.Errorf("expected 3 applied, got %d", applied)
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %v", errors)
	}

	// Verify outcomes.
	if _, err := os.Stat(delTarget); !os.IsNotExist(err) {
		t.Errorf("delete target should be gone")
	}
	if _, err := os.Stat(moveSrc); !os.IsNotExist(err) {
		t.Errorf("move source should be gone")
	}
	if got, err := os.ReadFile(moveDst); err != nil || string(got) != "move" {
		t.Errorf("move dest check failed: err=%v got=%q", err, got)
	}
	if got, err := os.ReadFile(createTarget); err != nil || string(got) != "new" {
		t.Errorf("create target check failed: err=%v got=%q", err, got)
	}
}

func TestApplyBinaryFixesBatchColumns_MatchesSliceBehavior(t *testing.T) {
	makeFixture := func(t *testing.T) ([]scanner.Finding, scanner.FindingColumns, string, string, string) {
		t.Helper()

		tmp := t.TempDir()
		delTarget := filepath.Join(tmp, "delete_me.png")
		moveSrc := filepath.Join(tmp, "move_src.png")
		moveDst := filepath.Join(tmp, "moved", "move_dst.png")
		createTarget := filepath.Join(tmp, "created.webp")

		if err := os.WriteFile(delTarget, []byte("del"), 0o644); err != nil {
			t.Fatalf("write delete target: %v", err)
		}
		if err := os.WriteFile(moveSrc, []byte("move"), 0o644); err != nil {
			t.Fatalf("write move source: %v", err)
		}

		findings := []scanner.Finding{
			{
				File: delTarget, Rule: "Cleanup", Message: "del",
				BinaryFix: &scanner.BinaryFix{
					Type: scanner.BinaryFixDeleteFile, SourcePath: delTarget,
				},
			},
			{
				File: moveSrc, Rule: "IconLocation", Message: "move",
				BinaryFix: &scanner.BinaryFix{
					Type: scanner.BinaryFixMoveFile, SourcePath: moveSrc, TargetPath: moveDst,
				},
			},
			{
				File: "source.png", Rule: "ConvertToWebp", Message: "create",
				BinaryFix: &scanner.BinaryFix{
					Type: scanner.BinaryFixCreateFile, TargetPath: createTarget, Content: []byte("new"),
				},
			},
		}
		return findings, scanner.CollectFindings(findings), delTarget, moveDst, createTarget
	}

	sliceFindings, _, wantDeleted, wantMoved, wantCreated := makeFixture(t)
	wantApplied, wantErrors := ApplyBinaryFixesBatch(sliceFindings, false)
	if len(wantErrors) != 0 {
		t.Fatalf("slice ApplyBinaryFixesBatch errors: %v", wantErrors)
	}

	_, columns, gotDeleted, gotMoved, gotCreated := makeFixture(t)
	gotApplied, gotErrors := ApplyBinaryFixesBatchColumns(&columns, false)
	if len(gotErrors) != 0 {
		t.Fatalf("columnar ApplyBinaryFixesBatchColumns errors: %v", gotErrors)
	}

	if gotApplied != wantApplied {
		t.Fatalf("applied mismatch: want %d, got %d", wantApplied, gotApplied)
	}
	if _, err := os.Stat(wantDeleted); !os.IsNotExist(err) {
		t.Fatalf("slice delete target should be gone, err=%v", err)
	}
	if got, err := os.ReadFile(wantMoved); err != nil || string(got) != "move" {
		t.Fatalf("slice moved file mismatch: err=%v got=%q", err, got)
	}
	if got, err := os.ReadFile(wantCreated); err != nil || string(got) != "new" {
		t.Fatalf("slice created file mismatch: err=%v got=%q", err, got)
	}
	if _, err := os.Stat(gotDeleted); !os.IsNotExist(err) {
		t.Fatalf("columnar delete target should be gone, err=%v", err)
	}
	if got, err := os.ReadFile(gotMoved); err != nil || string(got) != "move" {
		t.Fatalf("columnar moved file mismatch: err=%v got=%q", err, got)
	}
	if got, err := os.ReadFile(gotCreated); err != nil || string(got) != "new" {
		t.Fatalf("columnar created file mismatch: err=%v got=%q", err, got)
	}
}

func TestApplyBinaryFixesBatch_DryRun_MixedTypes(t *testing.T) {
	tmp := t.TempDir()

	delTarget := filepath.Join(tmp, "keep.png")
	os.WriteFile(delTarget, []byte("keep"), 0644)

	moveSrc := filepath.Join(tmp, "stay.png")
	os.WriteFile(moveSrc, []byte("stay"), 0644)

	createTarget := filepath.Join(tmp, "should_not_exist.webp")

	findings := []scanner.Finding{
		{
			File: delTarget, Rule: "Cleanup", Message: "del",
			BinaryFix: &scanner.BinaryFix{
				Type: scanner.BinaryFixDeleteFile, SourcePath: delTarget,
			},
		},
		{
			File: moveSrc, Rule: "IconLocation", Message: "move",
			BinaryFix: &scanner.BinaryFix{
				Type: scanner.BinaryFixMoveFile, SourcePath: moveSrc, TargetPath: filepath.Join(tmp, "moved.png"),
			},
		},
		{
			File: "source.png", Rule: "ConvertToWebp", Message: "create",
			BinaryFix: &scanner.BinaryFix{
				Type: scanner.BinaryFixCreateFile, TargetPath: createTarget, Content: []byte("new"),
			},
		},
	}

	applied, errors := ApplyBinaryFixesBatch(findings, true)
	if applied != 3 {
		t.Errorf("expected 3 applied in dry-run, got %d", applied)
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %v", errors)
	}

	// Nothing should have changed.
	if _, err := os.Stat(delTarget); err != nil {
		t.Errorf("delete target should still exist in dry-run")
	}
	if _, err := os.Stat(moveSrc); err != nil {
		t.Errorf("move source should still exist in dry-run")
	}
	if _, err := os.Stat(createTarget); !os.IsNotExist(err) {
		t.Errorf("create target should not exist in dry-run")
	}
}

// --- Phase 5: Reference scanner validation tests ---

func TestValidateBinaryFix_FindsDirectReferences(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "Activity.kt"), []byte(`val img = "icon.png"`), 0644)

	// Create a real PNG file so animated check doesn't error on missing file.
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "icon.png")
	writeStaticPNG(t, srcPath)

	fix := &scanner.BinaryFix{
		Type:       scanner.BinaryFixConvertWebP,
		SourcePath: srcPath,
	}

	refs, err := ValidateBinaryFix(fix, []string{tmp})
	if err != nil {
		t.Fatalf("unexpected safety error: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(refs))
	}
	if refs[0].Line != 1 {
		t.Errorf("expected line 1, got %d", refs[0].Line)
	}
}

func TestValidateBinaryFix_NoRefsForSafeNames(t *testing.T) {
	tmp := t.TempDir()
	// Only uses the resource name without extension -- safe.
	os.WriteFile(filepath.Join(tmp, "layout.xml"), []byte(`<ImageView android:src="@drawable/icon" />`), 0644)

	// Create a real PNG file so animated check works.
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "icon.png")
	writeStaticPNG(t, srcPath)

	fix := &scanner.BinaryFix{
		Type:       scanner.BinaryFixConvertWebP,
		SourcePath: srcPath,
	}

	refs, err := ValidateBinaryFix(fix, []string{tmp})
	if err != nil {
		t.Fatalf("unexpected safety error: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 references for resource-name-only usage, got %d", len(refs))
	}
}

func TestValidateBinaryFix_NilFixReturnsNil(t *testing.T) {
	refs, err := ValidateBinaryFix(nil, []string{"/tmp"})
	if err != nil {
		t.Errorf("expected nil error for nil fix, got %v", err)
	}
	if refs != nil {
		t.Errorf("expected nil for nil fix, got %v", refs)
	}
}

func TestValidateBinaryFix_NonWebPFixReturnsNil(t *testing.T) {
	fix := &scanner.BinaryFix{
		Type:       scanner.BinaryFixDeleteFile,
		SourcePath: "/res/drawable/icon.png",
	}
	refs, err := ValidateBinaryFix(fix, []string{"/tmp"})
	if err != nil {
		t.Errorf("expected nil error for non-WebP fix, got %v", err)
	}
	if refs != nil {
		t.Errorf("expected nil for non-WebP fix, got %v", refs)
	}
}

func TestApplyBinaryFixesBatch_SkipsConversionWithDirectRefs(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "App.kt"), []byte(`val f = "icon.png"`), 0644)

	resDir := filepath.Join(tmp, "res")
	os.MkdirAll(resDir, 0755)
	src := filepath.Join(resDir, "icon.png")
	os.WriteFile(src, []byte("png data"), 0644)

	findings := []scanner.Finding{
		{
			File: src, Rule: "ConvertToWebp", Message: "convert",
			BinaryFix: &scanner.BinaryFix{
				Type:         scanner.BinaryFixConvertWebP,
				SourcePath:   src,
				DeleteSource: true,
			},
		},
	}

	applied, errors := ApplyBinaryFixesBatch(findings, false, []string{srcDir})

	// Conversion should be skipped due to direct references.
	if applied != 0 {
		t.Errorf("expected 0 applied (direct refs), got %d", applied)
	}
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errors), errors)
	}
	if !strings.Contains(errors[0].Error(), "direct file reference") {
		t.Errorf("expected direct file reference error, got: %v", errors[0])
	}
	// Fix should be downgraded to HintOnly.
	if !findings[0].BinaryFix.HintOnly {
		t.Errorf("expected fix to be downgraded to HintOnly")
	}
	// Source file must still exist.
	if _, err := os.Stat(src); err != nil {
		t.Errorf("source should still exist after skipped conversion: %v", err)
	}
}

func TestApplyBinaryFixesBatch_AllowsConversionWithNoRefs(t *testing.T) {
	// When no direct refs exist but cwebp is missing, the conversion should
	// still be attempted (and fail for the expected reason).
	t.Setenv("PATH", t.TempDir())

	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	os.MkdirAll(srcDir, 0755)
	// Only resource-name references -- safe.
	os.WriteFile(filepath.Join(srcDir, "layout.xml"), []byte(`<ImageView android:src="@drawable/icon" />`), 0644)

	resDir := filepath.Join(tmp, "res")
	os.MkdirAll(resDir, 0755)
	src := filepath.Join(resDir, "icon.png")
	os.WriteFile(src, []byte("png data"), 0644)

	findings := []scanner.Finding{
		{
			File: src, Rule: "ConvertToWebp", Message: "convert",
			BinaryFix: &scanner.BinaryFix{
				Type:       scanner.BinaryFixConvertWebP,
				SourcePath: src,
			},
		},
	}

	_, errors := ApplyBinaryFixesBatch(findings, false, []string{srcDir})

	// Should attempt conversion (fail because no cwebp, not because of refs).
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errors), errors)
	}
	if !strings.Contains(errors[0].Error(), "cwebp") {
		t.Errorf("expected cwebp error, got: %v", errors[0])
	}
	// Fix should NOT be downgraded.
	if findings[0].BinaryFix.HintOnly {
		t.Errorf("fix should not be HintOnly when no direct refs found")
	}
}

// --- Phase 4: Safety model tests ---

func TestValidateBinaryFix_BlocksAnimatedGIF(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "anim.gif")
	writeAnimatedGIF(t, src)

	fix := &scanner.BinaryFix{
		Type:       scanner.BinaryFixConvertWebP,
		SourcePath: src,
	}

	refs, err := ValidateBinaryFix(fix, nil)
	if err == nil {
		t.Fatal("expected safety error for animated GIF")
	}
	if !strings.Contains(err.Error(), "animated GIF") {
		t.Errorf("expected animated GIF error, got: %v", err)
	}
	if refs != nil {
		t.Errorf("expected nil refs on safety error, got %v", refs)
	}
}

func TestValidateBinaryFix_AllowsStaticGIF(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "static.gif")
	writeStaticGIF(t, src)

	fix := &scanner.BinaryFix{
		Type:       scanner.BinaryFixConvertWebP,
		SourcePath: src,
	}

	_, err := ValidateBinaryFix(fix, nil)
	if err != nil {
		t.Fatalf("unexpected safety error for static GIF: %v", err)
	}
}

func TestValidateBinaryFix_BlocksAnimatedPNG(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "anim.png")
	writeAPNG(t, src)

	fix := &scanner.BinaryFix{
		Type:       scanner.BinaryFixConvertWebP,
		SourcePath: src,
	}

	refs, err := ValidateBinaryFix(fix, nil)
	if err == nil {
		t.Fatal("expected safety error for animated PNG")
	}
	if !strings.Contains(err.Error(), "animated PNG") {
		t.Errorf("expected animated PNG error, got: %v", err)
	}
	if refs != nil {
		t.Errorf("expected nil refs on safety error, got %v", refs)
	}
}

func TestValidateBinaryFix_AllowsStaticPNG(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "static.png")
	writeStaticPNG(t, src)

	fix := &scanner.BinaryFix{
		Type:       scanner.BinaryFixConvertWebP,
		SourcePath: src,
	}

	_, err := ValidateBinaryFix(fix, nil)
	if err != nil {
		t.Fatalf("unexpected safety error for static PNG: %v", err)
	}
}

func TestValidateBinaryFix_BlocksLowMinSdk(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "icon.png")
	writeStaticPNG(t, src)

	fix := &scanner.BinaryFix{
		Type:       scanner.BinaryFixConvertWebP,
		SourcePath: src,
		MinSdk:     10,
	}

	refs, err := ValidateBinaryFix(fix, nil)
	if err == nil {
		t.Fatal("expected safety error for low minSdk")
	}
	if !strings.Contains(err.Error(), "minSdk") {
		t.Errorf("expected minSdk error, got: %v", err)
	}
	if refs != nil {
		t.Errorf("expected nil refs on safety error, got %v", refs)
	}
}

func TestValidateBinaryFix_AllowsSufficientMinSdk(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "icon.png")
	writeStaticPNG(t, src)

	fix := &scanner.BinaryFix{
		Type:       scanner.BinaryFixConvertWebP,
		SourcePath: src,
		MinSdk:     21,
	}

	_, err := ValidateBinaryFix(fix, nil)
	if err != nil {
		t.Fatalf("unexpected safety error for minSdk 21: %v", err)
	}
}

func TestValidateBinaryFix_MinSdkZeroMeansNoRestriction(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "icon.png")
	writeStaticPNG(t, src)

	fix := &scanner.BinaryFix{
		Type:       scanner.BinaryFixConvertWebP,
		SourcePath: src,
		MinSdk:     0,
	}

	_, err := ValidateBinaryFix(fix, nil)
	if err != nil {
		t.Fatalf("unexpected safety error for minSdk 0: %v", err)
	}
}

func TestApplyBinaryFixesBatch_SkipsAnimatedGIF(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	tmp := t.TempDir()
	src := filepath.Join(tmp, "anim.gif")
	writeAnimatedGIF(t, src)

	findings := []scanner.Finding{
		{
			File: src, Rule: "GifUsage", Message: "convert",
			BinaryFix: &scanner.BinaryFix{
				Type:         scanner.BinaryFixConvertWebP,
				SourcePath:   src,
				DeleteSource: true,
			},
		},
	}

	applied, errors := ApplyBinaryFixesBatch(findings, false)
	if applied != 0 {
		t.Errorf("expected 0 applied for animated GIF, got %d", applied)
	}
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errors), errors)
	}
	if !strings.Contains(errors[0].Error(), "animated GIF") {
		t.Errorf("expected animated GIF error, got: %v", errors[0])
	}
	// Fix should be downgraded to HintOnly.
	if !findings[0].BinaryFix.HintOnly {
		t.Error("expected fix to be downgraded to HintOnly")
	}
	// Source file must still exist.
	if _, err := os.Stat(src); err != nil {
		t.Errorf("source should still exist after skipped conversion: %v", err)
	}
}

func TestApplyBinaryFixesBatch_SkipsLowMinSdk(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	tmp := t.TempDir()
	src := filepath.Join(tmp, "icon.png")
	writeStaticPNG(t, src)

	findings := []scanner.Finding{
		{
			File: src, Rule: "ConvertToWebp", Message: "convert",
			BinaryFix: &scanner.BinaryFix{
				Type:       scanner.BinaryFixConvertWebP,
				SourcePath: src,
				MinSdk:     10,
			},
		},
	}

	applied, errors := ApplyBinaryFixesBatch(findings, false)
	if applied != 0 {
		t.Errorf("expected 0 applied for low minSdk, got %d", applied)
	}
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errors), errors)
	}
	if !strings.Contains(errors[0].Error(), "minSdk") {
		t.Errorf("expected minSdk error, got: %v", errors[0])
	}
	if !findings[0].BinaryFix.HintOnly {
		t.Error("expected fix to be downgraded to HintOnly")
	}
}

// writeAPNG creates a minimal APNG file (PNG with acTL chunk) at path.
func writeAPNG(t *testing.T, path string) {
	t.Helper()
	// Build a valid PNG with an acTL chunk inserted before IDAT.
	// First, encode a normal PNG to a buffer.
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.White)

	var buf strings.Builder
	// We'll build the APNG manually by taking a normal PNG and inserting acTL.
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create APNG: %v", err)
	}
	defer f.Close()
	_ = buf

	// Write PNG signature.
	pngSig := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	f.Write(pngSig)

	// Write IHDR chunk (13 bytes of data).
	writeChunk(f, "IHDR", []byte{
		0, 0, 0, 1, // width = 1
		0, 0, 0, 1, // height = 1
		8, // bit depth
		2, // color type (RGB)
		0, // compression
		0, // filter
		0, // interlace
	})

	// Write acTL chunk (8 bytes: num_frames=2, num_plays=0).
	writeChunk(f, "acTL", []byte{
		0, 0, 0, 2, // num_frames = 2
		0, 0, 0, 0, // num_plays = 0 (infinite)
	})

	// Write a minimal IDAT chunk (zlib-compressed single RGB pixel).
	// zlib header (78 01) + deflate block with filter byte 0 + RGB(255,255,255) + adler32
	idatData := []byte{0x78, 0x01, 0x62, 0xF8, 0xCF, 0xC0, 0x00, 0x00, 0x01, 0x01, 0x01, 0x00}
	writeChunk(f, "IDAT", idatData)

	// Write IEND chunk.
	writeChunk(f, "IEND", nil)
}

// writeChunk writes a PNG chunk with the given type and data.
func writeChunk(w *os.File, chunkType string, data []byte) {
	// Length (4 bytes big-endian).
	length := uint32(len(data))
	lenBytes := []byte{byte(length >> 24), byte(length >> 16), byte(length >> 8), byte(length)}
	w.Write(lenBytes)

	// Chunk type.
	typeBytes := []byte(chunkType)
	w.Write(typeBytes)

	// Chunk data.
	if len(data) > 0 {
		w.Write(data)
	}

	// CRC32 over type + data (we use a dummy CRC since we only need structural validity
	// for the acTL detection, not for PNG decoding).
	crc := crc32ForChunk(typeBytes, data)
	crcBytes := []byte{byte(crc >> 24), byte(crc >> 16), byte(crc >> 8), byte(crc)}
	w.Write(crcBytes)
}

// crc32ForChunk computes CRC32 over type+data for a PNG chunk.
func crc32ForChunk(chunkType, data []byte) uint32 {
	// Use the standard CRC-32 table used by PNG.
	var table [256]uint32
	for i := 0; i < 256; i++ {
		c := uint32(i)
		for j := 0; j < 8; j++ {
			if c&1 != 0 {
				c = 0xEDB88320 ^ (c >> 1)
			} else {
				c >>= 1
			}
		}
		table[i] = c
	}
	crc := uint32(0xFFFFFFFF)
	for _, b := range chunkType {
		crc = table[(crc^uint32(b))&0xFF] ^ (crc >> 8)
	}
	for _, b := range data {
		crc = table[(crc^uint32(b))&0xFF] ^ (crc >> 8)
	}
	return crc ^ 0xFFFFFFFF
}

func containsStr(s, substr string) bool {
	return strings.Contains(s, substr)
}
