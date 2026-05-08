package android

import (
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// writePNG creates a PNG file with the given dimensions.
func writePNG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Fill with a color based on size so different-size PNGs have different hashes
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

// writeDuplicatePNG creates identical PNG files at two paths.
func writeDuplicatePNG(t *testing.T, path1, path2 string) {
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

// setupIconTestResDir creates a temporary res/ directory with density-qualified icon files.
func setupIconTestResDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	// Create density directories with launcher icons at proper sizes
	densities := map[string]int{
		"drawable-mdpi":    48,
		"drawable-hdpi":    72,
		"drawable-xhdpi":   96,
		"drawable-xxhdpi":  144,
		"drawable-xxxhdpi": 192,
	}

	for dir, size := range densities {
		dirPath := filepath.Join(resDir, dir)
		os.MkdirAll(dirPath, 0o755)
		writePNG(t, filepath.Join(dirPath, "ic_launcher.png"), size, size)
	}

	return resDir
}

func TestScanIconDirs_Basic(t *testing.T) {
	resDir := setupIconTestResDir(t)
	idx, err := ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	if len(idx.Icons) != 5 {
		t.Errorf("expected 5 icons, got %d", len(idx.Icons))
	}

	if len(idx.Densities) != 5 {
		t.Errorf("expected 5 density buckets, got %d", len(idx.Densities))
	}

	// Check that dimensions were decoded
	for _, ic := range idx.Icons {
		if ic.Width == 0 || ic.Height == 0 {
			t.Errorf("icon %s in %s has zero dimensions", ic.Name, ic.Density)
		}
		if ic.Format != "png" {
			t.Errorf("expected format png, got %s", ic.Format)
		}
		if ic.Hash == "" {
			t.Error("expected non-empty hash")
		}
	}
}

func TestScanIconDirs_Mipmap(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "mipmap-hdpi")
	os.MkdirAll(dirPath, 0o755)
	writePNG(t, filepath.Join(dirPath, "ic_launcher.png"), 72, 72)

	idx, err := ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	if len(idx.Icons) != 1 {
		t.Fatalf("expected 1 icon, got %d", len(idx.Icons))
	}
	if idx.Icons[0].Density != "hdpi" {
		t.Errorf("expected density hdpi, got %s", idx.Icons[0].Density)
	}
}

func TestScanIconDirs_XMLFormat(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-mdpi")
	os.MkdirAll(dirPath, 0o755)
	os.WriteFile(filepath.Join(dirPath, "ic_vector.xml"), []byte("<vector/>"), 0o644)

	idx, err := ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	if len(idx.Icons) != 1 {
		t.Fatalf("expected 1 icon, got %d", len(idx.Icons))
	}
	ic := idx.Icons[0]
	if ic.Format != "xml" {
		t.Errorf("expected format xml, got %s", ic.Format)
	}
	if ic.Width != 0 || ic.Height != 0 {
		t.Errorf("expected 0x0 for xml, got %dx%d", ic.Width, ic.Height)
	}
}

func TestScanIconDirs_SkipsUnknownDensity(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	// "night" is not a density qualifier
	dirPath := filepath.Join(resDir, "drawable-night")
	os.MkdirAll(dirPath, 0o755)
	writePNG(t, filepath.Join(dirPath, "icon.png"), 48, 48)

	idx, err := ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	if len(idx.Icons) != 0 {
		t.Errorf("expected 0 icons (unknown density), got %d", len(idx.Icons))
	}
}

func TestScanIconDirs_SkipsUnknownFormat(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-mdpi")
	os.MkdirAll(dirPath, 0o755)
	os.WriteFile(filepath.Join(dirPath, "readme.txt"), []byte("hi"), 0o644)

	idx, err := ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	if len(idx.Icons) != 0 {
		t.Errorf("expected 0 icons (unknown format), got %d", len(idx.Icons))
	}
}

func TestScanIconDirs_NotExist(t *testing.T) {
	_, err := ScanIconDirs("/nonexistent/res")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestScanIconDirs_NotDir(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "notadir")
	os.WriteFile(f, []byte("x"), 0o644)
	_, err := ScanIconDirs(f)
	if err == nil {
		t.Error("expected error for non-directory path")
	}
}

func TestIconsByName(t *testing.T) {
	resDir := setupIconTestResDir(t)
	idx, err := ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	byName := idx.IconsByName()
	launchers := byName["ic_launcher"]
	if len(launchers) != 5 {
		t.Errorf("expected 5 ic_launcher variants, got %d", len(launchers))
	}
}

func TestDensityMultiplier(t *testing.T) {
	tests := []struct {
		density string
		want    float64
	}{
		{"ldpi", 0.75},
		{"mdpi", 1.0},
		{"hdpi", 1.5},
		{"xhdpi", 2.0},
		{"xxhdpi", 3.0},
		{"xxxhdpi", 4.0},
		{"unknown", 0},
	}
	for _, tt := range tests {
		got := DensityMultiplier(tt.density)
		if got != tt.want {
			t.Errorf("DensityMultiplier(%q) = %v, want %v", tt.density, got, tt.want)
		}
	}
}

func TestIconDuplicateHashes(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	dir1 := filepath.Join(resDir, "drawable-mdpi")
	dir2 := filepath.Join(resDir, "drawable-hdpi")
	os.MkdirAll(dir1, 0o755)
	os.MkdirAll(dir2, 0o755)

	// Write identical PNGs
	writeDuplicatePNG(t, filepath.Join(dir1, "icon.png"), filepath.Join(dir2, "icon.png"))

	idx, err := ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	byName := idx.IconsByName()
	icons := byName["icon"]
	if len(icons) != 2 {
		t.Fatalf("expected 2 icon variants, got %d", len(icons))
	}
	if icons[0].Hash != icons[1].Hash {
		t.Error("expected identical hashes for duplicate PNGs")
	}
}

func TestIsAnimatedGIF_Animated(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "anim.gif")

	// Create a multi-frame GIF.
	g := &gif.GIF{}
	for i := 0; i < 3; i++ {
		frame := image.NewPaletted(image.Rect(0, 0, 2, 2), []color.Color{color.White, color.Black})
		g.Image = append(g.Image, frame)
		g.Delay = append(g.Delay, 10)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := gif.EncodeAll(f, g); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	animated, err := IsAnimatedGIF(path)
	if err != nil {
		t.Fatalf("IsAnimatedGIF error: %v", err)
	}
	if !animated {
		t.Error("expected animated=true for multi-frame GIF")
	}
}

func TestIsAnimatedGIF_Static(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "static.gif")

	// Create a single-frame GIF.
	frame := image.NewPaletted(image.Rect(0, 0, 2, 2), []color.Color{color.White, color.Black})
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := gif.Encode(f, frame, nil); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	animated, err := IsAnimatedGIF(path)
	if err != nil {
		t.Fatalf("IsAnimatedGIF error: %v", err)
	}
	if animated {
		t.Error("expected animated=false for single-frame GIF")
	}
}

func TestIsAnimatedGIF_MissingFile(t *testing.T) {
	_, err := IsAnimatedGIF("/nonexistent/file.gif")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestIsAnimatedPNG_APNG(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "anim.png")

	// Build a minimal APNG: PNG with acTL chunk before IDAT.
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// PNG signature.
	f.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	// IHDR chunk.
	writeTestChunk(t, f, "IHDR", []byte{
		0, 0, 0, 1, 0, 0, 0, 1, 8, 2, 0, 0, 0,
	})
	// acTL chunk (animation control).
	writeTestChunk(t, f, "acTL", []byte{
		0, 0, 0, 2, 0, 0, 0, 0,
	})
	// IDAT chunk (minimal).
	writeTestChunk(t, f, "IDAT", []byte{0x78, 0x01, 0x62, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01})
	// IEND chunk.
	writeTestChunk(t, f, "IEND", nil)
	f.Close()

	animated, err := IsAnimatedPNG(path)
	if err != nil {
		t.Fatalf("IsAnimatedPNG error: %v", err)
	}
	if !animated {
		t.Error("expected animated=true for APNG with acTL chunk")
	}
}

func TestIsAnimatedPNG_Static(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "static.png")

	// Create a normal PNG (no acTL chunk).
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	animated, err := IsAnimatedPNG(path)
	if err != nil {
		t.Fatalf("IsAnimatedPNG error: %v", err)
	}
	if animated {
		t.Error("expected animated=false for regular PNG")
	}
}

func TestIsAnimatedPNG_MissingFile(t *testing.T) {
	_, err := IsAnimatedPNG("/nonexistent/file.png")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// writeTestChunk writes a PNG chunk to a file (for test APNG construction).
func writeTestChunk(t *testing.T, f *os.File, chunkType string, data []byte) {
	t.Helper()
	length := uint32(len(data))
	f.Write([]byte{byte(length >> 24), byte(length >> 16), byte(length >> 8), byte(length)})
	f.Write([]byte(chunkType))
	if len(data) > 0 {
		f.Write(data)
	}
	// Write dummy CRC (4 bytes) -- sufficient for structural detection.
	f.Write([]byte{0, 0, 0, 0})
}

func TestExtensionFormat(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/res/drawable-mdpi/icon.png", "png"},
		{"/res/drawable-mdpi/icon.webp", "webp"},
		{"/res/drawable-mdpi/icon.jpg", "jpg"},
		{"/res/drawable-mdpi/icon.jpeg", "jpg"},
		{"/res/drawable-mdpi/icon.gif", "gif"},
		{"/res/drawable-mdpi/icon.xml", "xml"},
		{"/res/drawable-mdpi/icon.bmp", ""},
	}
	for _, tt := range tests {
		ic := &IconFile{Path: tt.path}
		got := ic.ExtensionFormat()
		if got != tt.want {
			t.Errorf("ExtensionFormat() for %q = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestIsNinePatch(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/res/drawable-mdpi/button.9.png", true},
		{"/res/drawable-mdpi/icon.png", false},
		{"/res/drawable-mdpi/file.9.jpg", false},
		{"/res/drawable-mdpi/bg.9.PNG", true},
	}
	for _, tt := range tests {
		ic := &IconFile{Path: tt.path}
		got := ic.IsNinePatch()
		if got != tt.want {
			t.Errorf("IsNinePatch() for %q = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestConfigIconsByName(t *testing.T) {
	idx := &IconIndex{
		ConfigIcons: []*IconFile{
			{Name: "ic_launcher", Path: "/res/drawable-en/ic_launcher.png"},
			{Name: "ic_logo", Path: "/res/drawable-fr/ic_logo.png"},
			{Name: "ic_launcher", Path: "/res/drawable-fr/ic_launcher.png"},
		},
	}
	byName := idx.ConfigIconsByName()
	if len(byName) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(byName))
	}
	if len(byName["ic_launcher"]) != 2 {
		t.Errorf("expected 2 ic_launcher entries, got %d", len(byName["ic_launcher"]))
	}
	if len(byName["ic_logo"]) != 1 {
		t.Errorf("expected 1 ic_logo entry, got %d", len(byName["ic_logo"]))
	}
}

func TestGifDetection(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	dirPath := filepath.Join(resDir, "drawable-mdpi")
	os.MkdirAll(dirPath, 0o755)

	// Write a minimal valid GIF file
	gifData := []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\xff\xff\xff\x00\x00\x00!\xf9\x04\x00\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;")
	os.WriteFile(filepath.Join(dirPath, "animation.gif"), gifData, 0o644)

	idx, err := ScanIconDirs(resDir)
	if err != nil {
		t.Fatalf("ScanIconDirs: %v", err)
	}

	if len(idx.Icons) != 1 {
		t.Fatalf("expected 1 icon, got %d", len(idx.Icons))
	}
	if idx.Icons[0].Format != "gif" {
		t.Errorf("expected format gif, got %s", idx.Icons[0].Format)
	}
}
