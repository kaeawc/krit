package librarymodel

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
)

type JavaSourceProfile struct {
	SourceRoots             []string
	TestSourceRoots         []string
	AndroidTestSourceRoots  []string
	GeneratedSourceRoots    []string
	ClasspathCandidates     []string
	BootClasspathCandidates []string
	ClasspathComplete       bool
}

var sourceSetJavaDirsRe = regexp.MustCompile(`(?s)(\w+)\s*\{[^{}]*(?:java\.srcDirs?|java\s*\{\s*srcDirs?)\s*(?:=|\+=|\()\s*([^}\n]+)`)
var namedSourceSetJavaDirsRe = regexp.MustCompile(`(?s)(?:getByName|named)\(\s*["']([A-Za-z0-9_]+)["']\s*\)\s*\{[^{}]*(?:java\.srcDirs?|java\s*\{\s*srcDirs?)\s*(?:=|\+=|\()\s*([^}\n]+)`)
var quotedPathRe = regexp.MustCompile(`["']([^"']+)["']`)

func JavaSourceProfileFromGradleContent(path, content string, cfg *android.BuildConfig) JavaSourceProfile {
	if cfg == nil {
		return JavaSourceProfile{}
	}
	root := filepath.Dir(path)
	if root == "." || root == "" {
		root = "."
	}
	profile := JavaSourceProfile{ClasspathComplete: cfg.IsAndroid && cfg.CompileSdkVersion > 0}
	if cfg.IsAndroid {
		profile.SourceRoots = append(profile.SourceRoots,
			absJoin(root, "src/main/java"),
			absJoin(root, "src/debug/java"),
			absJoin(root, "src/release/java"),
		)
		profile.TestSourceRoots = append(profile.TestSourceRoots, absJoin(root, "src/test/java"))
		profile.AndroidTestSourceRoots = append(profile.AndroidTestSourceRoots, absJoin(root, "src/androidTest/java"))
		profile.GeneratedSourceRoots = append(profile.GeneratedSourceRoots,
			absJoin(root, "build/generated/source"),
			absJoin(root, "build/generated/ksp"),
		)
		if cfg.CompileSdkVersion > 0 {
			bootClasspath := androidJarCandidates(cfg.CompileSdkVersion)
			profile.BootClasspathCandidates = append(profile.BootClasspathCandidates, bootClasspath...)
			if len(existingPathCandidates(bootClasspath)) == 0 {
				profile.ClasspathComplete = false
			}
		}
	}
	for sourceSet, dirs := range declaredJavaSourceSetDirs(content) {
		for _, dir := range dirs {
			switch sourceSet {
			case "test":
				profile.TestSourceRoots = append(profile.TestSourceRoots, absJoin(root, dir))
			case "androidTest":
				profile.AndroidTestSourceRoots = append(profile.AndroidTestSourceRoots, absJoin(root, dir))
			default:
				profile.SourceRoots = append(profile.SourceRoots, absJoin(root, dir))
			}
		}
	}
	profile.ClasspathCandidates = append(profile.ClasspathCandidates, javaClasspathCandidatesFromBuildDependencies(cfg.Dependencies)...)
	for _, dep := range cfg.Dependencies {
		if dep.Group == "" || dep.Name == "" {
			profile.ClasspathComplete = false
		}
	}
	profile.SourceRoots = uniqueStrings(profile.SourceRoots)
	profile.TestSourceRoots = uniqueStrings(profile.TestSourceRoots)
	profile.AndroidTestSourceRoots = uniqueStrings(profile.AndroidTestSourceRoots)
	profile.GeneratedSourceRoots = uniqueStrings(profile.GeneratedSourceRoots)
	profile.ClasspathCandidates = uniqueStrings(profile.ClasspathCandidates)
	profile.BootClasspathCandidates = uniqueExistingFirst(profile.BootClasspathCandidates)
	return profile
}

func (p JavaSourceProfile) SourceRootsForScan(includeTests, includeAndroidTests, includeGenerated bool) []string {
	roots := append([]string{}, p.SourceRoots...)
	if includeTests {
		roots = append(roots, p.TestSourceRoots...)
	}
	if includeAndroidTests {
		roots = append(roots, p.AndroidTestSourceRoots...)
	}
	if includeGenerated {
		roots = append(roots, p.GeneratedSourceRoots...)
	}
	return uniqueStrings(roots)
}

func (p JavaSourceProfile) JavacClasspathCandidates() []string {
	candidates := append([]string{}, p.BootClasspathCandidates...)
	candidates = append(candidates, p.ClasspathCandidates...)
	return existingPathCandidates(candidates)
}

func (p JavaSourceProfile) JavacClasspathArg() string {
	return strings.Join(p.JavacClasspathCandidates(), string(os.PathListSeparator))
}

func declaredJavaSourceSetDirs(content string) map[string][]string {
	out := make(map[string][]string)
	addMatches := func(matches [][]string) {
		for _, match := range matches {
			sourceSet := match[1]
			for _, pathMatch := range quotedPathRe.FindAllStringSubmatch(match[2], -1) {
				out[sourceSet] = append(out[sourceSet], pathMatch[1])
			}
		}
	}
	addMatches(sourceSetJavaDirsRe.FindAllStringSubmatch(content, -1))
	addMatches(namedSourceSetJavaDirsRe.FindAllStringSubmatch(content, -1))
	return out
}

func javaClasspathCandidatesFromDependencies(deps []Dependency) []string {
	var out []string
	for _, dep := range deps {
		out = append(out, dependencyCoordinate(dep.Group, dep.Name, dep.Version))
	}
	return uniqueStrings(out)
}

func javaClasspathCandidatesFromBuildDependencies(deps []android.Dependency) []string {
	var out []string
	for _, dep := range deps {
		out = append(out, dependencyCoordinate(dep.Group, dep.Name, dep.Version))
	}
	return uniqueStrings(out)
}

func dependencyCoordinate(group, name, version string) string {
	if group == "" || name == "" {
		return ""
	}
	coord := group + ":" + name
	if version != "" {
		coord += ":" + version
	}
	return coord
}

func mergeJavaDependencyClasspath(profile *JavaSourceProfile, deps []Dependency) {
	if profile == nil {
		return
	}
	profile.ClasspathCandidates = uniqueStrings(append(profile.ClasspathCandidates, javaClasspathCandidatesFromDependencies(deps)...))
	for _, dep := range deps {
		if dep.Group == "" || dep.Name == "" {
			profile.ClasspathComplete = false
		}
	}
}

func (p JavaSourceProfile) empty() bool {
	return len(p.SourceRoots) == 0 &&
		len(p.TestSourceRoots) == 0 &&
		len(p.AndroidTestSourceRoots) == 0 &&
		len(p.GeneratedSourceRoots) == 0 &&
		len(p.ClasspathCandidates) == 0 &&
		len(p.BootClasspathCandidates) == 0 &&
		!p.ClasspathComplete
}

func androidJarCandidates(compileSDK int) []string {
	if compileSDK <= 0 {
		return nil
	}
	var roots []string
	for _, env := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if value := os.Getenv(env); value != "" {
			roots = append(roots, value)
		}
	}
	var out []string
	for _, root := range uniqueStrings(roots) {
		out = append(out, filepath.Join(root, "platforms", "android-"+strconv.Itoa(compileSDK), "android.jar"))
	}
	return out
}

func mergeJavaSourceProfile(current, next JavaSourceProfile) JavaSourceProfile {
	if current.empty() {
		return next
	}
	if next.empty() {
		return current
	}
	current.SourceRoots = uniqueStrings(append(current.SourceRoots, next.SourceRoots...))
	current.TestSourceRoots = uniqueStrings(append(current.TestSourceRoots, next.TestSourceRoots...))
	current.AndroidTestSourceRoots = uniqueStrings(append(current.AndroidTestSourceRoots, next.AndroidTestSourceRoots...))
	current.GeneratedSourceRoots = uniqueStrings(append(current.GeneratedSourceRoots, next.GeneratedSourceRoots...))
	current.ClasspathCandidates = uniqueStrings(append(current.ClasspathCandidates, next.ClasspathCandidates...))
	current.BootClasspathCandidates = uniqueExistingFirst(append(current.BootClasspathCandidates, next.BootClasspathCandidates...))
	current.ClasspathComplete = current.ClasspathComplete && next.ClasspathComplete
	return current
}

func absJoin(root, rel string) string {
	if filepath.IsAbs(rel) {
		return filepath.Clean(rel)
	}
	return filepath.Clean(filepath.Join(root, rel))
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	out := values[:0]
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func uniqueExistingFirst(values []string) []string {
	values = uniqueStrings(values)
	sort.SliceStable(values, func(i, j int) bool {
		_, iErr := os.Stat(values[i])
		_, jErr := os.Stat(values[j])
		return iErr == nil && jErr != nil
	})
	return values
}

func existingPathCandidates(values []string) []string {
	values = uniqueStrings(values)
	out := values[:0]
	for _, value := range values {
		if _, err := os.Stat(value); err == nil {
			out = append(out, value)
		}
	}
	return out
}
