package fsutil

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// HomeEnv relocates the per-user Krit directory, which otherwise defaults to
// ~/.krit. It holds installed jars, JVM AOT/CDS archives and the daemon
// registry, so hermetic runners (sandboxed build actions, CI containers) can
// keep every per-user write inside a directory they own.
const HomeEnv = "KRIT_HOME"

// StateDirEnv relocates the per-project generated state directory, which
// otherwise defaults to <project>/.krit. It moves the analysis caches and the
// type oracle output so a scan can run over a read-only source tree. Each
// project root gets its own subdirectory, so projects may share one value.
// Inputs that live under <project>/.krit, such as an exported Gradle model,
// are still discovered at their usual location.
const StateDirEnv = "KRIT_STATE_DIR"

// UserStateDir returns the per-user Krit directory: $KRIT_HOME when set,
// otherwise ~/.krit. The directory is not created.
func UserStateDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv(HomeEnv)); dir != "" {
		return absOrSelf(dir), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if home == "" {
		return "", errors.New("empty home directory")
	}
	return filepath.Join(home, ".krit"), nil
}

// ProjectStateDir returns the generated state directory for repoDir:
// repoDir/.krit, or $KRIT_STATE_DIR/<name>-<hash>/.krit when the override is
// set. The hash is of the resolved absolute repoDir, so two roots map to the
// same directory exactly when their default .krit directories would be the
// same, and caches keyed by relative paths cannot leak between projects. The
// directory is not created.
func ProjectStateDir(repoDir string) string {
	if dir := strings.TrimSpace(os.Getenv(StateDirEnv)); dir != "" {
		// The .krit leaf keeps <state>/.krit/fir-incremental in the shape
		// krit-fir accepts for persisted incremental bundles.
		return filepath.Join(absOrSelf(dir), projectStateKey(repoDir), ".krit")
	}
	return filepath.Join(repoDir, ".krit")
}

// projectStateKey names repoDir's subdirectory of $KRIT_STATE_DIR: its base
// name for readability plus a hash of its resolved absolute path.
func projectStateKey(repoDir string) string {
	root := absOrSelf(repoDir)
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	sum := sha256.Sum256([]byte(root))
	name := filepath.Base(root)
	if name == "." || name == ".." || strings.ContainsAny(name, `/\:`) {
		name = "project"
	}
	return name + "-" + hex.EncodeToString(sum[:6])
}

func absOrSelf(dir string) string {
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}
