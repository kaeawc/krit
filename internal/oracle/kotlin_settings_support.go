package oracle

import (
	"archive/zip"
	"fmt"
	"os"
	"strings"
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

// compilerPluginsClass is the entry only a krit-fir jar that understands
// `--compiler-plugin` contains. krit-types does not load compiler plugins.
const compilerPluginsClass = "dev/jasonpearson/krit/fir/runner/CompilerPlugins.class"

// firJarPrefix marks a krit-fir jar, so a krit-fir jar that predates plugin
// support can be told from a krit-types jar (which never supports it).
const firJarPrefix = "dev/jasonpearson/krit/fir/"

type jarSupportKey struct {
	path  string
	size  int64
	mtime int64
}

// jarSupport is what a helper jar understands, read once per jar identity.
type jarSupport struct {
	kotlinSettings  bool
	compilerPlugins bool
	fir             bool
}

var (
	jarSupportMu    sync.Mutex
	jarSupportCache = map[jarSupportKey]jarSupport{}
	jarSupportWarn  sync.Map
)

// inspectJar reads which flags jarPath understands. A jar that cannot be
// inspected (missing, not a zip) is assumed to understand all of them: the
// launch then fails for its own reason rather than being silently downgraded.
func inspectJar(jarPath string) jarSupport {
	all := jarSupport{kotlinSettings: true, compilerPlugins: true, fir: true}
	info, err := os.Stat(jarPath)
	if err != nil {
		return all
	}
	key := jarSupportKey{jarPath, info.Size(), info.ModTime().UnixNano()}
	jarSupportMu.Lock()
	defer jarSupportMu.Unlock()
	if v, ok := jarSupportCache[key]; ok {
		return v
	}
	support := all
	if zr, err := zip.OpenReader(jarPath); err == nil {
		support = jarSupport{}
		for _, f := range zr.File {
			support.kotlinSettings = support.kotlinSettings || kotlinSettingsClasses[f.Name]
			support.compilerPlugins = support.compilerPlugins || f.Name == compilerPluginsClass
			support.fir = support.fir || strings.HasPrefix(f.Name, firJarPrefix)
		}
		_ = zr.Close()
	}
	jarSupportCache[key] = support
	return support
}

// JarSupportsKotlinSettings reports whether jarPath understands `--kotlin-arg`.
// A jar that cannot be inspected (missing, not a zip) is assumed to support
// it: the launch then fails for its own reason rather than being silently
// downgraded here.
func JarSupportsKotlinSettings(jarPath string) bool { return inspectJar(jarPath).kotlinSettings }

// JarSupportsCompilerPlugins reports whether jarPath understands
// `--compiler-plugin` / `--compiler-plugin-option` (a current krit-fir jar).
func JarSupportsCompilerPlugins(jarPath string) bool { return inspectJar(jarPath).compilerPlugins }

// KotlinLaunchArgs returns the `--kotlin-arg` flag pairs that launch jarPath
// under kt, followed by kt's compiler plugins as `--compiler-plugin` /
// `--compiler-plugin-option` pairs for a krit-fir jar; nil when kt is empty.
// For a jar that predates the Kotlin settings flags it returns nil and warns
// once on stderr: the oracle then runs under the embedded compiler's defaults
// for that jar, which the warning states, rather than the launch failing on an
// unknown argument. A krit-fir jar that predates plugin support gets the
// settings without the plugins, with a warning; krit-types never loads
// compiler plugins, so it gets the settings alone.
func KotlinLaunchArgs(jarPath string, kt gradlemodel.KotlinSettings) []string {
	args := kt.CompilerArgs()
	if len(args) == 0 && kt.Plugins.IsZero() {
		return nil
	}
	support := inspectJar(jarPath)
	if !support.kotlinSettings {
		if _, warned := jarSupportWarn.LoadOrStore(jarPath, true); !warned {
			fmt.Fprintf(os.Stderr, "warning: %s predates Kotlin compiler settings support; analyzing under the embedded compiler defaults instead of %s (rebuild or update the jar)\n", jarPath, kt.Describe())
		}
		return nil
	}
	out := make([]string, 0, 2*(len(args)+len(kt.Plugins.Classpath)+len(kt.Plugins.Options)))
	for _, arg := range args {
		out = append(out, "--kotlin-arg", arg)
	}
	if kt.Plugins.IsZero() || !support.fir {
		return out
	}
	if !support.compilerPlugins {
		if _, warned := jarSupportWarn.LoadOrStore(jarPath+"#plugins", true); !warned {
			fmt.Fprintf(os.Stderr, "warning: %s predates compiler plugin support; analyzing without the project's compiler plugins (%s) (rebuild or update the jar)\n", jarPath, kt.Plugins.Describe())
		}
		return out
	}
	for _, jar := range kt.Plugins.Classpath {
		out = append(out, "--compiler-plugin", AbsolutePath(jar))
	}
	for _, option := range kt.Plugins.Options {
		out = append(out, "--compiler-plugin-option", option)
	}
	return out
}
