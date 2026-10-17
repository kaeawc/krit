package oracle

import (
	"archive/zip"
	"fmt"
	"os"
	"sync"

	"github.com/kaeawc/krit/internal/gradlemodel"
)

// kotlinSettingsClasses are the entries that only a krit-types / krit-fir jar
// built with Kotlin compiler settings support contains. A jar without them
// (an older KRIT_TYPES_JAR / KRIT_FIR_JAR override, or a stale cached jar)
// rejects `--kotlin-arg` as an unknown argument and would fail to start, which
// disables the oracle entirely; krit instead leaves the flags off and says so.
var kotlinSettingsClasses = map[string]bool{
	"dev/jasonpearson/krit/types/KotlinCompilerSettings.class":      true,
	"dev/jasonpearson/krit/fir/runner/KotlinCompilerSettings.class": true,
}

type jarSupportKey struct {
	path  string
	size  int64
	mtime int64
}

var (
	jarSupportMu    sync.Mutex
	jarSupportCache = map[jarSupportKey]bool{}
	jarSupportWarn  sync.Map
)

// JarSupportsKotlinSettings reports whether jarPath understands `--kotlin-arg`.
// A jar that cannot be inspected (missing, not a zip) is assumed to support
// it: the launch then fails for its own reason rather than being silently
// downgraded here.
func JarSupportsKotlinSettings(jarPath string) bool {
	info, err := os.Stat(jarPath)
	if err != nil {
		return true
	}
	key := jarSupportKey{jarPath, info.Size(), info.ModTime().UnixNano()}
	jarSupportMu.Lock()
	defer jarSupportMu.Unlock()
	if v, ok := jarSupportCache[key]; ok {
		return v
	}
	supported := true
	if zr, err := zip.OpenReader(jarPath); err == nil {
		supported = false
		for _, f := range zr.File {
			if kotlinSettingsClasses[f.Name] {
				supported = true
				break
			}
		}
		_ = zr.Close()
	}
	jarSupportCache[key] = supported
	return supported
}

// KotlinLaunchArgs returns the `--kotlin-arg` flag pairs that launch jarPath
// under kt, or nil when kt is empty. For a jar that predates the flags it
// returns nil and warns once on stderr: the oracle then runs under the embedded
// compiler's defaults for that jar, which the warning states, rather than the
// launch failing on an unknown argument.
func KotlinLaunchArgs(jarPath string, kt gradlemodel.KotlinSettings) []string {
	args := kt.CompilerArgs()
	if len(args) == 0 {
		return nil
	}
	if !JarSupportsKotlinSettings(jarPath) {
		if _, warned := jarSupportWarn.LoadOrStore(jarPath, true); !warned {
			fmt.Fprintf(os.Stderr, "warning: %s predates Kotlin compiler settings support; analyzing under the embedded compiler defaults instead of %s (rebuild or update the jar)\n", jarPath, kt.Describe())
		}
		return nil
	}
	out := make([]string, 0, 2*len(args))
	for _, arg := range args {
		out = append(out, "--kotlin-arg", arg)
	}
	return out
}
