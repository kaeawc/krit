package fsutil

import (
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
// type oracle output so a scan can run over a read-only source tree. Inputs
// that live under <project>/.krit, such as an exported Gradle model, are still
// discovered at their usual location.
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
// $KRIT_STATE_DIR when set, otherwise repoDir/.krit. The override is shared by
// every project root in the process, so one state directory serves one scan
// invocation. The directory is not created.
func ProjectStateDir(repoDir string) string {
	if dir := strings.TrimSpace(os.Getenv(StateDirEnv)); dir != "" {
		return absOrSelf(dir)
	}
	return filepath.Join(repoDir, ".krit")
}

func absOrSelf(dir string) string {
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}
