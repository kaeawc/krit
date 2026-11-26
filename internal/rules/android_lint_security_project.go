package rules

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"

	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// TileProviderPermissionsRule mirrors Lint's TileProviderPermissions (TileProviderDetector): a Wear tile service whose
// android:permission is not BIND_TILE_PROVIDER.
type TileProviderPermissionsRule struct {
	ManifestBase
	AndroidRule
}

// UnsafeNativeCodeLocationRule mirrors Lint's UnsafeNativeCodeLocation (UnsafeNativeCodeDetector): an ELF binary in a
// res/raw folder or in an assets folder of any (non-test) source set of the module.
type UnsafeNativeCodeLocationRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-3 (high) base confidence: the check reads the manifest elements it reports on.
func (r *TileProviderPermissionsRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a tier-3 (high) base confidence: the check reads the file's ELF magic number.
func (r *UnsafeNativeCodeLocationRule) Confidence() float64 { return api.ConfidenceHigh }

const (
	tileProviderBindAction     = "androidx.wear.tiles.action.BIND_TILE_PROVIDER"
	tileProviderBindPermission = "com.google.android.wearable.permission.BIND_TILE_PROVIDER"
)

func (r *TileProviderPermissionsRule) check(ctx *api.Context) {
	if ctx.Manifest == nil {
		return
	}
	manifestWalk(ctx.Manifest.Root, func(n, _ *manifest.Node) {
		if n.Tag != "service" {
			return
		}
		// Lint checks the service once for every BIND_TILE_PROVIDER action in its intent filters.
		for _, filter := range n.ChildrenByTag("intent-filter") {
			for _, action := range filter.ChildrenByTag("action") {
				if action.AttrValue("android:name") != tileProviderBindAction {
					continue
				}
				if n.AttrValue("android:permission") != tileProviderBindPermission {
					emitManifestAtName(ctx, n, "TileProvider does not specify BIND_TILE_PROVIDER permission")
				}
			}
		}
	})
}

const (
	unsafeSharedLibraryMessage = "Shared libraries should not be placed in the res or assets directories. Please use " +
		"the features of your development environment to place shared libraries in the lib directory of the compiled APK."
	unsafeExecutableMessage = "Embedding non-shared library native executables into applications should be avoided " +
		"when possible, as there is an increased risk that the executables could be tampered with after installation. " +
		"Instead, native code should be placed in a shared library, and the features of the development environment " +
		"should be used to place the shared library in the lib directory of the compiled APK."
)

func (r *UnsafeNativeCodeLocationRule) check(ctx *api.Context) {
	if ctx.ResourceIndex == nil {
		return
	}
	dirs := append([]string(nil), ctx.ResourceIndex.ResDirs...)
	sort.Strings(dirs)
	seen := map[string]bool{}
	for _, resDir := range dirs {
		var files []string
		// Lint checks the files directly inside every raw* folder of the module's resource directories, including
		// those of the other source sets (src/debug/res, src/<flavor>/res)...
		for _, res := range append([]string{resDir}, android.SiblingSourceSetResDirs(resDir)...) {
			for _, folder := range lintSortedEntries(res) {
				if folder.IsDir() && strings.HasPrefix(folder.Name(), "raw") {
					rawDir := filepath.Join(res, folder.Name())
					for _, f := range lintSortedEntries(rawDir) {
						files = append(files, filepath.Join(rawDir, f.Name()))
					}
				}
			}
		}
		// ...and the files directly inside every source set's assets folder (#1619).
		for _, assets := range android.SourceSetAssetDirs(resDir) {
			for _, f := range lintSortedEntries(assets) {
				files = append(files, filepath.Join(assets, f.Name()))
			}
		}
		for _, path := range files {
			if seen[path] {
				continue
			}
			seen[path] = true
			if !isELFFile(path) {
				continue
			}
			msg := unsafeExecutableMessage
			if strings.HasSuffix(path, ".so") {
				msg = unsafeSharedLibraryMessage
			}
			ctx.Emit(scanner.Finding{File: path, Line: 1, Col: 1, Message: msg})
		}
	}
}

// lintSortedEntries lists a directory in name order, or returns nil when it cannot be read.
func lintSortedEntries(dir string) []os.DirEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	return entries
}

// elfMagic is the first four bytes of an ELF object file.
var elfMagic = []byte{0x7f, 'E', 'L', 'F'}

// isELFFile reports whether path is a regular file that starts with the ELF magic number.
func isELFFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, len(elfMagic))
	if _, err := io.ReadFull(f, head); err != nil {
		return false
	}
	return bytes.Equal(head, elfMagic)
}
