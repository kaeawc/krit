package gradlemodel

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kaeawc/krit/internal/hashutil"
)

// InputsKeyFile is the stamp `krit gradle-model key --write` leaves in a model
// directory: the InputsKey of the build the model was exported from. Load reads
// only *.json, so the stamp is not a model file.
const InputsKeyFile = "inputs.key"

// inputsKeyVersion changes whenever the file set or the encoding below changes,
// so a key from an older krit never matches.
const inputsKeyVersion = "gm1"

// inputsKeyPruned are directories never searched for model inputs: outputs,
// caches and VCS metadata. They can be large, and nothing in them configures the
// build.
var inputsKeyPruned = map[string]bool{
	".git": true, ".gradle": true, ".krit": true, ".idea": true, ".kotlin": true,
	"build": true, "out": true, "node_modules": true, "bazel-out": true,
}

// isModelInput reports whether a root-relative, slash-separated file path can
// change what kritExportModel writes. The tests pin this list. A false positive
// only costs a needless re-export; a false negative is a stale model, so the
// list leans wide.
func isModelInput(rel string) bool {
	name := rel[strings.LastIndex(rel, "/")+1:]
	switch {
	// Settings, build and init scripts, and precompiled script plugins.
	case strings.HasSuffix(name, ".gradle"), strings.HasSuffix(name, ".gradle.kts"):
		return true
	// Version catalogs (gradle/libs.versions.toml and any other catalog file).
	case strings.HasSuffix(name, ".versions.toml"):
		return true
	// Dependency locking: gradle.lockfile, settings-gradle.lockfile,
	// buildscript-gradle.lockfile and per-configuration lockfiles.
	case strings.HasSuffix(name, ".lockfile"):
		return true
	// gradle.properties (any project's), local.properties (Android sdk.dir).
	case name == "gradle.properties", name == "local.properties":
		return true
	// Dependency verification and the Gradle version.
	case rel == "gradle/verification-metadata.xml", rel == "gradle/wrapper/gradle-wrapper.properties":
		return true
	}
	// Build logic compiled from sources: buildSrc, and the conventional
	// included build-logic build.
	for _, dir := range []string{"buildSrc/", "build-logic/"} {
		if strings.HasPrefix(rel, dir) && strings.Contains(rel, "/src/main/") {
			return true
		}
	}
	return false
}

// InputsKey hashes the files under root that can change the Gradle model:
// settings and build scripts, version catalogs, lockfiles, dependency
// verification metadata, gradle.properties, local.properties, the wrapper's
// Gradle version, and buildSrc / build-logic sources. A CI job keys a cache of
// `.krit/gradle-model` on it, and skips Gradle on a hit. The key depends only on
// root-relative paths and file contents, so checkouts at different paths agree.
func InputsKey(root string) (string, error) {
	inputs, err := ModelInputs(root)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", inputsKeyVersion, hashutil.HasherName())
	for _, rel := range inputs {
		// The process-wide memo: krit hashes the same build files for other keys.
		sum, err := hashutil.Default().HashFile(filepath.Join(root, filepath.FromSlash(rel)), nil)
		if err != nil {
			return "", fmt.Errorf("gradle model inputs key: %w", err)
		}
		b.WriteString(rel)
		b.WriteByte(0)
		b.WriteString(sum)
		b.WriteByte('\n')
	}
	return inputsKeyVersion + "-" + hashutil.HashHex([]byte(b.String()))[:40], nil
}

// ModelInputs lists the files InputsKey hashes, root-relative with forward
// slashes, sorted.
func ModelInputs(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("gradle model inputs key: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("gradle model inputs key: %s is not a directory", root)
	}
	var inputs []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrPermission) && path != root {
				return nil
			}
			return err
		}
		if entry.IsDir() {
			if path != root && inputsKeyPruned[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel = filepath.ToSlash(rel); isModelInput(rel) {
			inputs = append(inputs, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("gradle model inputs key: %w", err)
	}
	slices.Sort(inputs)
	return inputs, nil
}

// ReadInputsKey returns the key stamped in modelDir, or "" when there is none.
func ReadInputsKey(modelDir string) string {
	data, err := os.ReadFile(filepath.Join(modelDir, InputsKeyFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// WriteInputsKey stamps key in modelDir.
func WriteInputsKey(modelDir, key string) error {
	return os.WriteFile(filepath.Join(modelDir, InputsKeyFile), []byte(key+"\n"), 0o644)
}

// ModelRoot returns the build root a model directory describes: <root> for
// <root>/.krit/gradle-model, otherwise (a model passed with --gradle-model) the
// nearest settings root above the first scan path.
func ModelRoot(modelDir string, scanPaths []string) string {
	if root, ok := modelDirRoot(modelDir); ok {
		return root
	}
	if len(scanPaths) > 0 {
		if roots := SettingsAncestors(scanPaths[0]); len(roots) > 0 {
			return roots[0]
		}
	}
	return filepath.Dir(filepath.Dir(modelDir))
}
