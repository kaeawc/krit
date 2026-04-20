package rules

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

// testWritePNG creates a PNG file with the given dimensions and unique color.
func testWritePNG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(width % 256), G: uint8(height % 256), B: 128, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating PNG %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encoding PNG %s: %v", path, err)
	}
}

// testWriteIdenticalPNG creates identical PNG files (same content) at two paths.
func testWriteIdenticalPNG(t *testing.T, path1, path2 string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for _, p := range []string{path1, path2} {
		f, err := os.Create(p)
		if err != nil {
			t.Fatalf("creating PNG %s: %v", p, err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			t.Fatalf("encoding PNG %s: %v", p, err)
		}
		f.Close()
	}
}

// testWriteLargePNG creates a PNG file larger than webpThresholdBytes.
func testWriteLargePNG(t *testing.T, path string) {
	t.Helper()
	// Use a larger, noisier image so compression still leaves it above the threshold.
	const size = 512
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			v := uint8((x*37 + y*91 + (x*y)%251) % 256)
			img.Set(x, y, color.RGBA{R: v, G: v ^ 0x5a, B: v ^ 0xc3, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating PNG %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encoding PNG %s: %v", path, err)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatalf("stat PNG %s: %v", path, err)
	}
	if info.Size() < webpThresholdBytes {
		t.Fatalf("test PNG is too small: got %d bytes, need at least %d", info.Size(), webpThresholdBytes)
	}
}

func TestCheckIconDensities_MissingVariants(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	// Only provide mdpi and hdpi — missing others
	for _, d := range []struct {
		dir  string
		size int
	}{
		{"drawable-mdpi", 48},
		{"drawable-hdpi", 72},
	} {
		dirPath := filepath.Join(resDir, d.dir)
		os.MkdirAll(dirPath, 0o755)
		testWritePNG(t, filepath.Join(dirPath, "icon.png"), d.size, d.size)
	}

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconDensities(idx, c)
	findings := c.Columns().Findings()
	if len(findings) == 0 {
		t.Fatal("expected findings for missing density variants")
	}
	found := false
	for _, f := range findings {
		if strings.Contains(f.Message, "missing density variants") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'missing density variants' message, got: %v", findings)
	}
}

func TestCheckIconDensities_NilIndex(t *testing.T) {
	c := scanner.NewFindingCollector(0)
	CheckIconDensities(nil, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for nil index, got %d", len(findings))
	}
}

func TestCheckIconDipSize_WrongRatio(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	// mdpi=48, hdpi should be 72 but we provide 100
	for _, d := range []struct {
		dir  string
		size int
	}{
		{"drawable-mdpi", 48},
		{"drawable-hdpi", 100}, // wrong: should be 72
	} {
		dirPath := filepath.Join(resDir, d.dir)
		os.MkdirAll(dirPath, 0o755)
		testWritePNG(t, filepath.Join(dirPath, "icon.png"), d.size, d.size)
	}

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconDipSize(idx, c)
	findings := c.Columns().Findings()
	if len(findings) == 0 {
		t.Fatal("expected findings for wrong DPI ratio")
	}
	if !strings.Contains(findings[0].Message, "expected") {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestCheckIconDipSize_CorrectRatio(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	for _, d := range []struct {
		dir  string
		size int
	}{
		{"drawable-mdpi", 48},
		{"drawable-hdpi", 72},
		{"drawable-xhdpi", 96},
	} {
		dirPath := filepath.Join(resDir, d.dir)
		os.MkdirAll(dirPath, 0o755)
		testWritePNG(t, filepath.Join(dirPath, "icon.png"), d.size, d.size)
	}

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconDipSize(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for correct ratios, got %d: %v", len(findings), findings)
	}
}

func TestCheckIconDuplicates_IdenticalFiles(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	dir1 := filepath.Join(resDir, "drawable-mdpi")
	dir2 := filepath.Join(resDir, "drawable-hdpi")
	os.MkdirAll(dir1, 0o755)
	os.MkdirAll(dir2, 0o755)

	testWriteIdenticalPNG(t, filepath.Join(dir1, "icon.png"), filepath.Join(dir2, "icon.png"))

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconDuplicates(idx, c)
	findings := c.Columns().Findings()
	if len(findings) == 0 {
		t.Fatal("expected findings for duplicate icons")
	}
	if !strings.Contains(findings[0].Message, "identical across densities") {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestCheckIconDuplicates_DifferentFiles(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	for _, d := range []struct {
		dir  string
		size int
	}{
		{"drawable-mdpi", 48},
		{"drawable-hdpi", 72},
	} {
		dirPath := filepath.Join(resDir, d.dir)
		os.MkdirAll(dirPath, 0o755)
		testWritePNG(t, filepath.Join(dirPath, "icon.png"), d.size, d.size)
	}

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconDuplicates(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for different icons, got %d", len(findings))
	}
}

func TestCheckGifUsage(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-mdpi")
	os.MkdirAll(dirPath, 0o755)

	gifData := []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\xff\xff\xff\x00\x00\x00!\xf9\x04\x00\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;")
	os.WriteFile(filepath.Join(dirPath, "animation.gif"), gifData, 0o644)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckGifUsage(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "GIF file") {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestCheckGifUsage_NoPNG(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-mdpi")
	os.MkdirAll(dirPath, 0o755)
	testWritePNG(t, filepath.Join(dirPath, "icon.png"), 48, 48)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckGifUsage(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for PNG-only resources, got %d", len(findings))
	}
}

func TestCheckConvertToWebp_LargePNG(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-xxhdpi")
	os.MkdirAll(dirPath, 0o755)
	testWriteLargePNG(t, filepath.Join(dirPath, "large_bg.png"))

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckConvertToWebp(idx, c)
	findings := c.Columns().Findings()
	if len(findings) == 0 {
		t.Fatal("expected finding for large PNG")
	}
	if !strings.Contains(findings[0].Message, "WebP") {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
	// Verify BinaryFix is populated
	if findings[0].BinaryFix == nil {
		t.Fatal("expected BinaryFix to be set on ConvertToWebp finding")
	}
	if findings[0].BinaryFix.Type != scanner.BinaryFixConvertWebP {
		t.Errorf("expected BinaryFixConvertWebP type, got %d", findings[0].BinaryFix.Type)
	}
	if findings[0].BinaryFix.SourcePath == "" {
		t.Error("expected BinaryFix.SourcePath to be non-empty")
	}
	if findings[0].BinaryFix.Description != "Convert to WebP format and remove original PNG" {
		t.Errorf("unexpected BinaryFix.Description: %s", findings[0].BinaryFix.Description)
	}
	if !findings[0].BinaryFix.DeleteSource {
		t.Error("expected BinaryFix.DeleteSource to be true")
	}
}

func TestCheckConvertToWebp_SmallPNG(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-mdpi")
	os.MkdirAll(dirPath, 0o755)
	testWritePNG(t, filepath.Join(dirPath, "small.png"), 4, 4)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckConvertToWebp(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for small PNG, got %d", len(findings))
	}
}

func TestCheckIconMissingDensityFolder(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	// Only provide mdpi and ldpi — missing hdpi, xhdpi, xxhdpi
	for _, d := range []string{"drawable-mdpi", "drawable-ldpi"} {
		dirPath := filepath.Join(resDir, d)
		os.MkdirAll(dirPath, 0o755)
		testWritePNG(t, filepath.Join(dirPath, "icon.png"), 48, 48)
	}

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconMissingDensityFolder(idx, c)
	findings := c.Columns().Findings()
	if len(findings) == 0 {
		t.Fatal("expected findings for missing density folders")
	}

	// Should flag hdpi, xhdpi, xxhdpi as missing
	missingCount := 0
	for _, f := range findings {
		if strings.Contains(f.Message, "Missing density folder") {
			missingCount++
		}
	}
	if missingCount < 2 {
		t.Errorf("expected at least 2 missing density folder findings, got %d", missingCount)
	}
}

func TestCheckIconMissingDensityFolder_AllPresent(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	for _, d := range []string{"drawable-mdpi", "drawable-hdpi", "drawable-xhdpi", "drawable-xxhdpi"} {
		dirPath := filepath.Join(resDir, d)
		os.MkdirAll(dirPath, 0o755)
		testWritePNG(t, filepath.Join(dirPath, "icon.png"), 48, 48)
	}

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconMissingDensityFolder(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings when all required densities present, got %d", len(findings))
	}
}

func TestCheckIconExpectedSize_Correct(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	for _, d := range []struct {
		dir  string
		size int
	}{
		{"drawable-mdpi", 48},
		{"drawable-hdpi", 72},
		{"drawable-xhdpi", 96},
		{"drawable-xxhdpi", 144},
		{"drawable-xxxhdpi", 192},
	} {
		dirPath := filepath.Join(resDir, d.dir)
		os.MkdirAll(dirPath, 0o755)
		testWritePNG(t, filepath.Join(dirPath, "ic_launcher.png"), d.size, d.size)
	}

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconExpectedSize(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for correct launcher sizes, got %d: %v", len(findings), findings)
	}
}

func TestCheckIconExpectedSize_Wrong(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	// hdpi launcher should be 72 but we provide 100
	dirPath := filepath.Join(resDir, "drawable-hdpi")
	os.MkdirAll(dirPath, 0o755)
	testWritePNG(t, filepath.Join(dirPath, "ic_launcher.png"), 100, 100)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconExpectedSize(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "expected 72x72") {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestCheckIconExpectedSize_NonLauncherIgnored(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	// Non-launcher icon with "wrong" size should not trigger
	dirPath := filepath.Join(resDir, "drawable-hdpi")
	os.MkdirAll(dirPath, 0o755)
	testWritePNG(t, filepath.Join(dirPath, "bg_photo.png"), 500, 500)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconExpectedSize(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-launcher icon, got %d", len(findings))
	}
}

func TestRunAllIconChecks(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	// Only mdpi — triggers missing density folder + missing density variants
	dirPath := filepath.Join(resDir, "drawable-mdpi")
	os.MkdirAll(dirPath, 0o755)
	testWritePNG(t, filepath.Join(dirPath, "ic_launcher.png"), 48, 48)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	RunAllIconChecks(idx, c)
	findings := c.Columns().Findings()
	if len(findings) == 0 {
		t.Fatal("expected some findings from RunAllIconChecks")
	}

	// Should have at least missing density folder findings
	hasFolder := false
	for _, f := range findings {
		if f.Rule == "IconMissingDensityFolder" {
			hasFolder = true
		}
	}
	if !hasFolder {
		t.Error("expected IconMissingDensityFolder finding")
	}
}

func TestRunAllIconChecks_NilIndex(t *testing.T) {
	c := scanner.NewFindingCollector(0)
	RunAllIconChecks(nil, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for nil index, got %d", len(findings))
	}
}

// --- IconExtensionRule tests ---

func TestCheckIconExtension_Mismatch(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-mdpi")
	os.MkdirAll(dirPath, 0o755)

	// Write a JPEG file with .png extension
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	// Pad to make it a plausible file
	jpegData := append(jpegHeader, make([]byte, 100)...)
	os.WriteFile(filepath.Join(dirPath, "icon.png"), jpegData, 0o644)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconExtension(idx, c)
	findings := c.Columns().Findings()
	if len(findings) == 0 {
		t.Fatal("expected finding for extension mismatch")
	}
	if !strings.Contains(findings[0].Message, "extension 'png' but content is jpg") {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestCheckIconExtension_Match(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-mdpi")
	os.MkdirAll(dirPath, 0o755)
	testWritePNG(t, filepath.Join(dirPath, "icon.png"), 48, 48)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconExtension(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings when extension matches content, got %d", len(findings))
	}
}

func TestCheckIconExtension_NilIndex(t *testing.T) {
	c := scanner.NewFindingCollector(0)
	CheckIconExtension(nil, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for nil index, got %d", len(findings))
	}
}

// --- IconLocationRule tests ---

func TestCheckIconLocation_LauncherInDrawable(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-hdpi")
	os.MkdirAll(dirPath, 0o755)
	testWritePNG(t, filepath.Join(dirPath, "ic_launcher.png"), 72, 72)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconLocation(idx, c)
	findings := c.Columns().Findings()
	if len(findings) == 0 {
		t.Fatal("expected finding for launcher icon in drawable folder")
	}
	if !strings.Contains(findings[0].Message, "should be in mipmap-*") {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestCheckIconLocation_LauncherInMipmap(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "mipmap-hdpi")
	os.MkdirAll(dirPath, 0o755)
	testWritePNG(t, filepath.Join(dirPath, "ic_launcher.png"), 72, 72)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconLocation(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for launcher in mipmap, got %d", len(findings))
	}
}

func TestCheckIconLocation_NonLauncherInDrawable(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-hdpi")
	os.MkdirAll(dirPath, 0o755)
	testWritePNG(t, filepath.Join(dirPath, "bg_header.png"), 200, 100)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconLocation(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-launcher icon in drawable, got %d", len(findings))
	}
}

func TestCheckIconLocation_NilIndex(t *testing.T) {
	c := scanner.NewFindingCollector(0)
	CheckIconLocation(nil, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for nil index, got %d", len(findings))
	}
}

// --- IconMixedNinePatchRule tests ---

func TestCheckIconMixedNinePatch_Mixed(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	// Create a regular PNG in mdpi
	dir1 := filepath.Join(resDir, "drawable-mdpi")
	os.MkdirAll(dir1, 0o755)
	testWritePNG(t, filepath.Join(dir1, "btn_bg.png"), 48, 48)

	// Create a nine-patch in hdpi (same resource name "btn_bg")
	dir2 := filepath.Join(resDir, "drawable-hdpi")
	os.MkdirAll(dir2, 0o755)
	testWritePNG(t, filepath.Join(dir2, "btn_bg.9.png"), 72, 72)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconMixedNinePatch(idx, c)
	findings := c.Columns().Findings()
	if len(findings) == 0 {
		t.Fatal("expected finding for mixed nine-patch variants")
	}
	if !strings.Contains(findings[0].Message, "mix of nine-patch") {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestCheckIconMixedNinePatch_AllNinePatch(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	for _, d := range []struct {
		dir  string
		size int
	}{
		{"drawable-mdpi", 48},
		{"drawable-hdpi", 72},
	} {
		dirPath := filepath.Join(resDir, d.dir)
		os.MkdirAll(dirPath, 0o755)
		testWritePNG(t, filepath.Join(dirPath, "btn_bg.9.png"), d.size, d.size)
	}

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconMixedNinePatch(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings when all variants are nine-patch, got %d", len(findings))
	}
}

func TestCheckIconMixedNinePatch_NilIndex(t *testing.T) {
	c := scanner.NewFindingCollector(0)
	CheckIconMixedNinePatch(nil, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for nil index, got %d", len(findings))
	}
}

// --- IconXmlAndPngRule tests ---

func TestCheckIconXmlAndPng_Mixed(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	// XML vector in drawable-anydpi... but we need a known density for the scanner.
	// Use mdpi for the XML and hdpi for the PNG, same resource name.
	dir1 := filepath.Join(resDir, "drawable-mdpi")
	os.MkdirAll(dir1, 0o755)
	xmlContent := []byte(`<?xml version="1.0" encoding="utf-8"?>
<vector xmlns:android="http://schemas.android.com/apk/res/android"
    android:width="24dp" android:height="24dp"
    android:viewportWidth="24" android:viewportHeight="24">
    <path android:fillColor="#000" android:pathData="M12,2L2,22h20z"/>
</vector>`)
	os.WriteFile(filepath.Join(dir1, "ic_arrow.xml"), xmlContent, 0o644)

	dir2 := filepath.Join(resDir, "drawable-hdpi")
	os.MkdirAll(dir2, 0o755)
	testWritePNG(t, filepath.Join(dir2, "ic_arrow.png"), 36, 36)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconXmlAndPng(idx, c)
	findings := c.Columns().Findings()
	if len(findings) == 0 {
		t.Fatal("expected finding for XML + raster conflict")
	}
	if !strings.Contains(findings[0].Message, "both XML vector and raster") {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestCheckIconXmlAndPng_OnlyPng(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	for _, d := range []struct {
		dir  string
		size int
	}{
		{"drawable-mdpi", 48},
		{"drawable-hdpi", 72},
	} {
		dirPath := filepath.Join(resDir, d.dir)
		os.MkdirAll(dirPath, 0o755)
		testWritePNG(t, filepath.Join(dirPath, "icon.png"), d.size, d.size)
	}

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconXmlAndPng(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for PNG-only resources, got %d", len(findings))
	}
}

func TestCheckIconXmlAndPng_NilIndex(t *testing.T) {
	c := scanner.NewFindingCollector(0)
	CheckIconXmlAndPng(nil, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for nil index, got %d", len(findings))
	}
}

// --- IconNoDpi tests ---

func TestCheckIconNoDpi_Conflict(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	// Create icon in both nodpi and hdpi
	nodpiDir := filepath.Join(resDir, "drawable-nodpi")
	hdpiDir := filepath.Join(resDir, "drawable-hdpi")
	os.MkdirAll(nodpiDir, 0o755)
	os.MkdirAll(hdpiDir, 0o755)

	testWritePNG(t, filepath.Join(nodpiDir, "icon.png"), 48, 48)
	testWritePNG(t, filepath.Join(hdpiDir, "icon.png"), 72, 72)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconNoDpi(idx, c)
	findings := c.Columns().Findings()
	if len(findings) == 0 {
		t.Fatal("expected findings for icon in both nodpi and density folder")
	}
	found := false
	for _, f := range findings {
		if strings.Contains(f.Message, "nodpi") && strings.Contains(f.Message, "hdpi") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected finding mentioning nodpi and hdpi, got: %v", findings)
	}
}

func TestCheckIconNoDpi_OnlyNodpi(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	nodpiDir := filepath.Join(resDir, "drawable-nodpi")
	os.MkdirAll(nodpiDir, 0o755)
	testWritePNG(t, filepath.Join(nodpiDir, "icon.png"), 48, 48)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconNoDpi(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for nodpi-only icon, got %d", len(findings))
	}
}

func TestCheckIconNoDpi_NoDpiDifferentName(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	nodpiDir := filepath.Join(resDir, "drawable-nodpi")
	hdpiDir := filepath.Join(resDir, "drawable-hdpi")
	os.MkdirAll(nodpiDir, 0o755)
	os.MkdirAll(hdpiDir, 0o755)

	testWritePNG(t, filepath.Join(nodpiDir, "bg_tile.png"), 48, 48)
	testWritePNG(t, filepath.Join(hdpiDir, "icon.png"), 72, 72)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconNoDpi(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for different names, got %d", len(findings))
	}
}

func TestCheckIconNoDpi_NilIndex(t *testing.T) {
	c := scanner.NewFindingCollector(0)
	CheckIconNoDpi(nil, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for nil index, got %d", len(findings))
	}
}

// --- IconDuplicatesConfig tests ---

func TestCheckIconDuplicatesConfig_IdenticalAcrossConfigs(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	// Create identical icons in drawable-en and drawable-fr (config folders, not density)
	enDir := filepath.Join(resDir, "drawable-en")
	frDir := filepath.Join(resDir, "drawable-fr")
	os.MkdirAll(enDir, 0o755)
	os.MkdirAll(frDir, 0o755)

	testWriteIdenticalPNG(t, filepath.Join(enDir, "flag.png"), filepath.Join(frDir, "flag.png"))

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconDuplicatesConfig(idx, c)
	findings := c.Columns().Findings()
	if len(findings) == 0 {
		t.Fatal("expected findings for identical icons across config folders")
	}
	if !strings.Contains(findings[0].Message, "identical across configurations") {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestCheckIconDuplicatesConfig_DifferentAcrossConfigs(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	enDir := filepath.Join(resDir, "drawable-en")
	frDir := filepath.Join(resDir, "drawable-fr")
	os.MkdirAll(enDir, 0o755)
	os.MkdirAll(frDir, 0o755)

	// Different sized images will have different hashes
	testWritePNG(t, filepath.Join(enDir, "flag.png"), 48, 48)
	testWritePNG(t, filepath.Join(frDir, "flag.png"), 72, 72)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconDuplicatesConfig(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for different icons across configs, got %d", len(findings))
	}
}

func TestCheckIconDuplicatesConfig_SingleConfig(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	enDir := filepath.Join(resDir, "drawable-en")
	os.MkdirAll(enDir, 0o755)
	testWritePNG(t, filepath.Join(enDir, "flag.png"), 48, 48)

	idx, err := android.ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	c := scanner.NewFindingCollector(0)
	CheckIconDuplicatesConfig(idx, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for single config folder, got %d", len(findings))
	}
}

func TestCheckIconDuplicatesConfig_NilIndex(t *testing.T) {
	c := scanner.NewFindingCollector(0)
	CheckIconDuplicatesConfig(nil, c)
	findings := c.Columns().Findings()
	if len(findings) != 0 {
		t.Errorf("expected no findings for nil index, got %d", len(findings))
	}
}
