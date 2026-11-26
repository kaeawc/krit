package pipeline

import (
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/hashutil"
)

// modulePluginPattern matches the plugin declarations of a build script: Android plugin ids, `id("...")` and
// `alias(libs.plugins....)`.
var modulePluginPattern = regexp.MustCompile(`com\.android\.[a-z-]+|\bid\s*\(?\s*["'][^"']+["']|alias\s*\(\s*[\w.]+\s*\)`)

// modulePluginsFingerprint hashes the plugins every build script of the project applies. Per-file rules such as
// AnnotateVersionCheck decide from the module's plugin whether it is a library, which the file's own content hash
// does not cover, so switching a module between application and library must change their cache keys. Other build
// script edits (dependencies, versions) leave it unchanged.
func modulePluginsFingerprint(project *android.Project) string {
	if project == nil || len(project.GradlePaths) == 0 {
		return ""
	}
	paths := append([]string(nil), project.GradlePaths...)
	sort.Strings(paths)
	var b strings.Builder
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		plugins := modulePluginPattern.FindAllString(string(data), -1)
		if len(plugins) == 0 {
			continue
		}
		sort.Strings(plugins)
		b.WriteString(path)
		b.WriteByte('|')
		b.WriteString(strings.Join(plugins, ","))
		b.WriteByte('\n')
	}
	if b.Len() == 0 {
		return ""
	}
	return hashutil.HashHex([]byte(b.String()))[:16]
}
