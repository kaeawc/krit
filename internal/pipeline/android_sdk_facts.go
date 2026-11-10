package pipeline

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/hashutil"
)

// sdkLevelsFingerprint hashes the minSdk and targetSdk that every build script, manifest and resource
// directory in the project resolves to. Rules read these per module through android.ResolveSDKLevels, which a
// file's own content hash does not cover, so an edit to one module's SDK level must change the cache key of
// every unit it can gate.
func sdkLevelsFingerprint(project *android.Project) string {
	if project == nil {
		return ""
	}
	seen := map[string]bool{}
	var probes []string
	add := func(path string) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		probes = append(probes, path)
	}
	for _, path := range project.GradlePaths {
		add(filepath.Join(filepath.Dir(path), "x"))
	}
	for _, path := range project.ManifestPaths {
		add(filepath.Join(moduleDirOfManifest(path), "x"))
		add(path)
	}
	for _, dir := range project.ResDirs {
		add(filepath.Join(dir, "x"))
		add(filepath.Join(moduleDirOfFile(filepath.Join(dir, "x")), "x"))
	}
	if len(probes) == 0 {
		return ""
	}
	sort.Strings(probes)
	android.WarmSDKLevels(probes)
	var resolver android.SDKLevelResolver
	var b strings.Builder
	for _, probe := range probes {
		// An unknown level adds nothing, so a project that declares none keeps its keys.
		if levels := resolver.Resolve(probe); !levels.IsZero() {
			fmt.Fprintf(&b, "%s|%d|%d\n", filepath.Dir(probe), levels.MinSdk, levels.TargetSdk)
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return hashutil.HashHex([]byte(b.String()))[:16]
}

// withSDKLevels folds the project's SDK levels into the rule hash that keys every Android findings unit.
func (in AndroidInput) withSDKLevels() AndroidInput {
	if in.RuleHash == "" {
		return in
	}
	if fp := sdkLevelsFingerprint(in.Project); fp != "" {
		in.RuleHash += "|sdk=" + fp
	}
	return in
}

// withProjectFacts folds the project-level facts that per-file rules read beyond the file itself into the rule
// hash that keys the per-file analysis cache: the module SDK levels (android.ResolveSDKLevels) and the Gradle
// library facts. A Gradle or manifest edit then replaces the cached findings instead of replaying them. A scan
// without an Android project keeps the hash as is, so every site that computes it must pass the same project.
func withProjectFacts(ruleHash string, project *android.Project, libraryFactsFP string) string {
	if project == nil {
		return ruleHash
	}
	return hashutil.HashHex([]byte(ruleHash + "|sdk=" + sdkLevelsFingerprint(project) + "|lib=" + libraryFactsFP))[:16]
}
