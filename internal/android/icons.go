package android

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/hashutil"
)

// Density represents an Android screen density bucket.
type Density struct {
	Name       string  // e.g., "mdpi", "hdpi"
	Multiplier float64 // relative to mdpi (1.0)
}

// KnownDensities lists standard Android density buckets in order.
var KnownDensities = []Density{
	{"ldpi", 0.75},
	{"mdpi", 1.0},
	{"hdpi", 1.5},
	{"xhdpi", 2.0},
	{"xxhdpi", 3.0},
	{"xxxhdpi", 4.0},
}

// IconFile represents a single icon resource file.
type IconFile struct {
	Path    string // absolute file path
	Name    string // resource name without extension
	Width   int    // 0 if unknown (xml, etc.)
	Height  int    // 0 if unknown
	Density string // density bucket from directory name
	Format  string // png, jpg, gif, webp, xml
	Size    int64  // file size in bytes
	Hash    string // SHA-256 hex digest for duplicate detection
}

// IconIndex holds all icon resources organized by density.
type IconIndex struct {
	Densities   map[string][]*IconFile // density -> list of icon files
	Icons       []*IconFile            // all icon files
	ConfigIcons []*IconFile            // icons in non-density config folders (e.g., drawable-en, drawable-fr)
}

// ScanIconDirs scans res/drawable-* and res/mipmap-* directories for icon resources.
func ScanIconDirs(resDir string) (*IconIndex, error) {
	return ScanIconDirsWithWorkers(resDir, runtime.NumCPU())
}

func ScanIconDirsWithWorkers(resDir string, maxWorkers int) (*IconIndex, error) {
	idx := &IconIndex{
		Densities: make(map[string][]*IconFile),
	}

	info, err := os.Stat(resDir)
	if err != nil {
		return nil, fmt.Errorf("cannot access res directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", resDir)
	}

	entries, err := os.ReadDir(resDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read res directory: %w", err)
	}

	type dirInput struct {
		density string
		path    string
	}
	type dirResult struct {
		icons       []*IconFile
		configIcons []*IconFile
	}

	var inputs []dirInput
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := entry.Name()

		var density string
		switch {
		case strings.HasPrefix(dirName, "drawable-"):
			density = strings.TrimPrefix(dirName, "drawable-")
		case strings.HasPrefix(dirName, "mipmap-"):
			density = strings.TrimPrefix(dirName, "mipmap-")
		default:
			continue
		}
		inputs = append(inputs, dirInput{density: density, path: filepath.Join(resDir, dirName)})
	}

	workers := clampWorkers(maxWorkers, len(inputs))
	results := make([]dirResult, len(inputs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	for i, input := range inputs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, input dirInput) {
			defer wg.Done()
			defer func() { <-sem }()
			files, err := os.ReadDir(input.path)
			if err != nil {
				return
			}
			isKnown := isKnownDensity(input.density) || input.density == "nodpi"
			var res dirResult
			for _, f := range files {
				if f.IsDir() {
					continue
				}
				filePath := filepath.Join(input.path, f.Name())
				icon := parseIconFile(filePath, input.density)
				if icon == nil {
					continue
				}
				if isKnown {
					res.icons = append(res.icons, icon)
				} else {
					res.configIcons = append(res.configIcons, icon)
				}
			}
			results[i] = res
		}(i, input)
	}
	wg.Wait()

	for i, input := range inputs {
		for _, icon := range results[i].icons {
			idx.Densities[input.density] = append(idx.Densities[input.density], icon)
			idx.Icons = append(idx.Icons, icon)
		}
		idx.ConfigIcons = append(idx.ConfigIcons, results[i].configIcons...)
	}

	return idx, nil
}

// detectFormatFromContent inspects the first bytes of data to determine the actual image format.
// Returns "png", "jpg", "gif", "webp", "xml", or "" if unknown.
func detectFormatFromContent(data []byte) string {
	if len(data) < 4 {
		return ""
	}
	// PNG: 89 50 4E 47
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "png"
	}
	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "jpg"
	}
	// GIF: GIF8
	if data[0] == 'G' && data[1] == 'I' && data[2] == 'F' && data[3] == '8' {
		return "gif"
	}
	// WebP: RIFF....WEBP
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "webp"
	}
	// XML: starts with '<' (possibly with BOM or whitespace)
	for _, b := range data {
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		if b == '<' {
			return "xml"
		}
		break
	}
	// UTF-8 BOM followed by '<'
	if len(data) >= 4 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF && data[3] == '<' {
		return "xml"
	}
	return ""
}

// parseIconFile reads an image file and extracts metadata.
func parseIconFile(path string, density string) *IconFile {
	ext := strings.ToLower(filepath.Ext(path))
	// Determine extension-based format for filtering supported files
	extFormat := ""
	switch ext {
	case ".png":
		extFormat = "png"
	case ".jpg", ".jpeg":
		extFormat = "jpg"
	case ".gif":
		extFormat = "gif"
	case ".webp":
		extFormat = "webp"
	case ".xml":
		extFormat = "xml"
	default:
		return nil // skip unknown formats
	}

	fi, err := os.Stat(path)
	if err != nil {
		return nil
	}

	base := filepath.Base(path)
	name := strings.TrimSuffix(base, ext)
	// Handle nine-patch: strip .9 from name if present
	if strings.HasSuffix(name, ".9") && ext == ".png" {
		name = strings.TrimSuffix(name, ".9")
	}

	icon := &IconFile{
		Path:    path,
		Name:    name,
		Density: density,
		Format:  extFormat,
		Size:    fi.Size(),
	}

	// Read file content for hash and content-based format detection
	data, err := os.ReadFile(path)
	if err != nil {
		return icon
	}
	icon.Hash = hashutil.HashHex(data)

	// Detect actual format from content (overrides extension-based format)
	if detected := detectFormatFromContent(data); detected != "" {
		icon.Format = detected
	}

	// Decode image dimensions for raster formats
	if icon.Format == "png" || icon.Format == "jpg" || icon.Format == "gif" {
		file, err := os.Open(path)
		if err != nil {
			return icon
		}
		defer file.Close()

		cfg, _, err := image.DecodeConfig(file)
		if err != nil {
			return icon
		}
		icon.Width = cfg.Width
		icon.Height = cfg.Height
	}

	return icon
}

// isKnownDensity returns true if the density string is a standard Android density.
func isKnownDensity(d string) bool {
	for _, kd := range KnownDensities {
		if kd.Name == d {
			return true
		}
	}
	return false
}

// DensityMultiplier returns the multiplier for a given density name, or 0 if unknown.
func DensityMultiplier(density string) float64 {
	for _, kd := range KnownDensities {
		if kd.Name == density {
			return kd.Multiplier
		}
	}
	return 0
}

// ExtensionFormat returns the format implied by the file extension of the icon's path.
func (ic *IconFile) ExtensionFormat() string {
	ext := strings.ToLower(filepath.Ext(ic.Path))
	switch ext {
	case ".png":
		return "png"
	case ".jpg", ".jpeg":
		return "jpg"
	case ".gif":
		return "gif"
	case ".webp":
		return "webp"
	case ".xml":
		return "xml"
	}
	return ""
}

// IsNinePatch returns true if the file has a .9.png extension.
func (ic *IconFile) IsNinePatch() bool {
	return strings.HasSuffix(strings.ToLower(ic.Path), ".9.png")
}

// IconsByName groups icons by their resource name across densities.
func (idx *IconIndex) IconsByName() map[string][]*IconFile {
	result := make(map[string][]*IconFile)
	for _, icon := range idx.Icons {
		result[icon.Name] = append(result[icon.Name], icon)
	}
	return result
}

// ConfigIconsByName groups config icons by their resource name.
func (idx *IconIndex) ConfigIconsByName() map[string][]*IconFile {
	result := make(map[string][]*IconFile)
	for _, icon := range idx.ConfigIcons {
		result[icon.Name] = append(result[icon.Name], icon)
	}
	return result
}

// IsAnimatedGIF returns true if the GIF file at path contains multiple frames
// (i.e., is an animated GIF). Returns false for single-frame GIFs.
func IsAnimatedGIF(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	g, err := gif.DecodeAll(f)
	if err != nil {
		return false, err
	}
	return len(g.Image) > 1, nil
}

// IsAnimatedPNG returns true if the PNG file at path is an APNG (Animated PNG).
// APNG files contain an acTL (animation control) chunk before the first IDAT
// chunk. Regular PNG files do not have this chunk.
func IsAnimatedPNG(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// Skip 8-byte PNG signature.
	sig := make([]byte, 8)
	if _, err := io.ReadFull(f, sig); err != nil {
		return false, nil
	}

	for {
		// Read chunk length (4 bytes) + type (4 bytes).
		header := make([]byte, 8)
		if _, err := io.ReadFull(f, header); err != nil {
			return false, nil
		}
		chunkType := string(header[4:8])
		if chunkType == "acTL" {
			return true, nil
		}
		if chunkType == "IDAT" {
			return false, nil
		}
		length := binary.BigEndian.Uint32(header[:4])
		// Skip chunk data + CRC (4 bytes).
		if _, err := f.Seek(int64(length)+4, io.SeekCurrent); err != nil {
			return false, nil
		}
	}
}
