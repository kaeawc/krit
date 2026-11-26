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
// directory: the InputsKey of the build the model was exported from, then one
// `<content hash> <path>` line per input it covers, so a later scan can check the
// stamp by re-hashing those files instead of walking the tree. Load reads only
// *.json, so the stamp is not a model file. kritExportModel deletes it whenever it
// rewrites a model.
const InputsKeyFile = "inputs.key"

// inputsKeyVersion changes whenever the file set or the encoding below changes,
// so a key from an older krit never matches.
const inputsKeyVersion = "gm1"

// inputsKeyPruned are directories never searched for model inputs: outputs,
// caches and VCS metadata. They can be large, and nothing in them configures the
// build. "build" and "out" are pruned only outside src/, where they can be source
// packages (com/acme/build).
var inputsKeyPruned = map[string]bool{
	".git": true, ".gradle": true, ".krit": true, ".idea": true, ".kotlin": true,
	"build": true, "out": true, "node_modules": true, "bazel-out": true,
}

// isBuildConfig reports whether a root-relative, slash-separated file path is a
// build configuration file that can change what kritExportModel writes. The
// tests pin this list. A false positive only costs a needless re-export; a false
// negative is a stale model, so the list leans wide.
func isBuildConfig(rel string) bool {
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
	return false
}

// buildLogicRoots are build-logic directories that need no settings file of their
// own to be found: buildSrc, and the conventional included build-logic build.
var buildLogicRoots = []string{"buildSrc", "build-logic"}

// InputsKey hashes the files under root that can change the Gradle model:
// settings and build scripts, version catalogs, lockfiles, dependency
// verification metadata, gradle.properties, local.properties, the wrapper's
// Gradle version, and the sources of build logic (buildSrc, and every included
// build under the root: a directory with its own settings file). A CI job keys a
// cache of `.krit/gradle-model` on it, and skips Gradle on a hit. The key depends
// only on root-relative paths and file contents, so checkouts at different paths
// agree.
func InputsKey(root string) (string, error) {
	key, _, err := hashedInputs(root)
	return key, err
}

// hashedInputs returns InputsKey and the `<hash> <path>` lines of its inputs.
func hashedInputs(root string) (string, []string, error) {
	inputs, err := ModelInputs(root)
	if err != nil {
		return "", nil, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", inputsKeyVersion, hashutil.HasherName())
	lines := make([]string, 0, len(inputs))
	for _, rel := range inputs {
		sum, err := hashInput(root, rel)
		if err != nil {
			return "", nil, fmt.Errorf("gradle model inputs key: %w", err)
		}
		b.WriteString(rel)
		b.WriteByte(0)
		b.WriteString(sum)
		b.WriteByte('\n')
		lines = append(lines, sum+" "+rel)
	}
	return inputsKeyVersion + "-" + hashutil.HashHex([]byte(b.String()))[:40], lines, nil
}

// hashInput hashes one input through the process-wide memo: krit hashes the
// same build files for other keys.
func hashInput(root, rel string) (string, error) {
	return hashutil.Default().HashFile(filepath.Join(root, filepath.FromSlash(rel)), nil)
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
	walk := &inputsWalk{root: root, buildRoots: append([]string(nil), buildLogicRoots...)}
	if err := filepath.WalkDir(root, walk.visit); err != nil {
		return nil, fmt.Errorf("gradle model inputs key: %w", err)
	}
	inputs := walk.inputs
	for _, rel := range walk.mainSources {
		if slices.ContainsFunc(walk.buildRoots, func(dir string) bool { return strings.HasPrefix(rel, dir+"/") }) {
			inputs = append(inputs, rel)
		}
	}
	slices.Sort(inputs)
	return inputs, nil
}

// inputsWalk collects build configuration files, the directories of nested
// builds (build logic), and the src/main files that may belong to them.
type inputsWalk struct {
	root                string
	inputs, mainSources []string
	buildRoots          []string
}

func (w *inputsWalk) visit(path string, entry fs.DirEntry, err error) error {
	if err != nil {
		if errors.Is(err, fs.ErrPermission) && path != w.root {
			return nil
		}
		return err
	}
	if path == w.root {
		return nil
	}
	rel, err := filepath.Rel(w.root, path)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)
	if entry.IsDir() {
		name := entry.Name()
		if inputsKeyPruned[name] && ((name != "build" && name != "out") || !underSrc(rel)) {
			return filepath.SkipDir
		}
		return nil
	}
	if !isRegularFile(path, entry) {
		return nil
	}
	switch {
	case isBuildConfig(rel):
		w.inputs = append(w.inputs, rel)
		if dir, name := splitRel(rel); dir != "" && (name == "settings.gradle" || name == "settings.gradle.kts") {
			// A nested build: build logic included with includeBuild, or a build of its own.
			w.buildRoots = append(w.buildRoots, dir)
		}
	case strings.Contains("/"+rel, "/src/main/"):
		w.mainSources = append(w.mainSources, rel)
	}
	return nil
}

// underSrc reports whether a root-relative directory is inside a src/ tree.
func underSrc(rel string) bool {
	return strings.HasPrefix(rel, "src/") || strings.Contains(rel, "/src/")
}

// isRegularFile follows a symlink, so a build file linked into the tree counts.
func isRegularFile(path string, entry fs.DirEntry) bool {
	if entry.Type().IsRegular() {
		return true
	}
	if entry.Type()&fs.ModeSymlink == 0 {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func splitRel(rel string) (dir, name string) {
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		return rel[:i], rel[i+1:]
	}
	return "", rel
}

// ReadInputsKey returns the key stamped in modelDir, or "" when there is none.
func ReadInputsKey(modelDir string) string {
	data, err := os.ReadFile(filepath.Join(modelDir, InputsKeyFile))
	if err != nil {
		return ""
	}
	key, _, _ := strings.Cut(string(data), "\n")
	return strings.TrimSpace(key)
}

// WriteInputsStamp computes the InputsKey of root and stamps it, with the
// hashes of its inputs, in modelDir.
func WriteInputsStamp(modelDir, root string) (string, error) {
	key, lines, err := hashedInputs(root)
	if err != nil {
		return "", err
	}
	body := key + "\n" + strings.Join(lines, "\n") + "\n"
	return key, os.WriteFile(filepath.Join(modelDir, InputsKeyFile), []byte(body), 0o644)
}

// CheckInputsStamp compares the build under root with the stamp in modelDir by
// re-hashing the inputs the stamp lists, without walking the tree. stamped is
// false when modelDir has no stamp. changed names the first listed input that is
// gone or has other content, or is "" when all match. A build file added since
// the stamp is not seen here, but it changes InputsKey, so a CI cache keyed on it
// misses.
func CheckInputsStamp(modelDir, root string) (stamped bool, changed string, err error) {
	data, err := os.ReadFile(filepath.Join(modelDir, InputsKeyFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, "", nil
		}
		return false, "", err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if !strings.HasPrefix(lines[0], inputsKeyVersion+"-") {
		return true, InputsKeyFile + " (written by another krit version)", nil
	}
	for _, line := range lines[1:] {
		sum, rel, ok := strings.Cut(line, " ")
		if !ok {
			return true, InputsKeyFile + " (malformed)", nil
		}
		if inputChanged(root, rel, sum) {
			return true, rel, nil
		}
	}
	return true, "", nil
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

// inputChanged reports whether a stamped input is gone, unreadable, or has
// content other than sum.
func inputChanged(root, rel, sum string) bool {
	current, err := hashInput(root, rel)
	return err != nil || current != sum
}
