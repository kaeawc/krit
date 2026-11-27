package librarymodel

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/sysres"
)

// Dependency is a normalized external library coordinate discovered from a
// target repository's build files.
type Dependency struct {
	Group         string
	Name          string
	Version       string
	Configuration string
	Source        string
}

// ProjectProfile contains project-wide library and platform facts. It is
// intentionally small: rules should consume derived Facts instead of parsing
// Gradle files or version catalogs themselves.
type ProjectProfile struct {
	// InputBase is the directory the build files the profile was read from
	// share. It is persisted with cached facts but left out of the
	// fingerprint, which spells paths under it relative to it, so a checkout
	// moved to another absolute path fingerprints the same (#1662).
	InputBase string `json:",omitempty"`

	MinSdkVersion     int
	TargetSdkVersion  int
	CompileSdkVersion int

	Kotlin  KotlinProfile
	KSP     KSPProfile
	Android AndroidProfile
	JVM     JVMProfile
	Java    JavaSourceProfile

	Dependencies []Dependency

	CatalogSources      []CatalogSource
	CatalogCompleteness CatalogCompleteness

	UnresolvedCatalogAliases []UnresolvedCatalogAlias

	// HasGradle is true when at least one Gradle file was parsed.
	HasGradle bool
	// DependencyExtractionComplete is true only when parsed Gradle files use
	// dependency forms this lightweight parser can treat as a complete-enough
	// graph for absence decisions.
	DependencyExtractionComplete bool
	// HasUnresolvedDependencyRefs is true when a build file uses unresolved
	// aliases, convention plugins, apply-from scripts, or other dependency
	// forms this parser cannot resolve yet. In that case models stay
	// conservative.
	HasUnresolvedDependencyRefs bool
}

type Coordinate struct {
	Group string
	Name  string
}

type CatalogAliasKind string

const (
	CatalogAliasPlugin  CatalogAliasKind = "plugin"
	CatalogAliasVersion CatalogAliasKind = "version"
	CatalogAliasLibrary CatalogAliasKind = "library"
	CatalogAliasBundle  CatalogAliasKind = "bundle"
)

type UnresolvedCatalogAlias struct {
	Kind   CatalogAliasKind
	Alias  string
	Source string
}

// ProfileFromGradlePaths reads Gradle build files and returns a merged project
// profile. Unreadable or unparsable files are ignored so library models remain
// best-effort and never block analysis.
func ProfileFromGradlePaths(paths []string) ProjectProfile {
	// Reading files and resolving catalogs share caches, so they stay
	// serial; parsing each build file is independent and dominates on
	// projects with hundreds of modules, so it runs in parallel. Profiles
	// are merged in path order, exactly as the serial loop did.
	type gradleInput struct {
		content string
		catalog VersionCatalog
		ok      bool
	}
	inputs := make([]gradleInput, len(paths))
	catalogs := make(map[string]VersionCatalog)
	discovery := NewCatalogDiscovery()
	for i, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var catalog VersionCatalog
		if catalogSources, ok := discovery.FindVersionCatalogSourcesForGradlePath(path); ok {
			catalogKey := catalogSourceCacheKey(catalogSources)
			catalog, ok = catalogs[catalogKey]
			if !ok {
				catalog, _ = loadVersionCatalogFromSources(catalogSources)
				catalogs[catalogKey] = catalog
			}
		}
		inputs[i] = gradleInput{content: string(content), catalog: catalog, ok: true}
	}
	profiles := make([]ProjectProfile, len(paths))
	forEachIndex(len(paths), func(i int) {
		if in := inputs[i]; in.ok {
			profiles[i] = ProfileFromGradleContentWithCatalog(paths[i], in.content, in.catalog)
		}
	})
	var profile ProjectProfile
	for i := range paths {
		if inputs[i].ok {
			profile.Merge(profiles[i])
		}
	}
	applyFoundrySdkProperties(&profile, paths)
	return profile
}

// forEachIndex calls fn(i) for every i in [0, n) on up to sysres.CPUs()
// goroutines and returns when all calls have finished.
func forEachIndex(n int, fn func(i int)) {
	workers := min(sysres.CPUs(), n)
	if workers <= 1 {
		for i := 0; i < n; i++ {
			fn(i)
		}
		return
	}
	var next atomic.Int64
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := int(next.Add(1) - 1); i < n; i = int(next.Add(1) - 1) {
				fn(i)
			}
		}()
	}
	wg.Wait()
}

// applyFoundrySdkProperties fills SDK levels that no build file declared from the Foundry Gradle plugin's
// gradle.properties keys (foundry.android.{min,target,compile}SdkVersion) in the nearest directory above the
// build files that sets them.
func applyFoundrySdkProperties(profile *ProjectProfile, paths []string) {
	seen := map[string]bool{}
	for _, path := range paths {
		for dir := filepath.Dir(path); !seen[dir]; dir = filepath.Dir(dir) {
			seen[dir] = true
			if data, err := os.ReadFile(filepath.Join(dir, "gradle.properties")); err == nil {
				for _, line := range strings.Split(string(data), "\n") {
					key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
					if !ok {
						continue
					}
					key = strings.TrimSpace(key)
					if level := parsePositiveInt(strings.TrimPrefix(strings.TrimSpace(value), "android-"), 0); level > 0 {
						applyIfUnset(profile, key, level)
					}
				}
			}
			if parent := filepath.Dir(dir); parent == dir {
				break
			}
		}
	}
}

func applyIfUnset(profile *ProjectProfile, key string, level int) {
	switch key {
	case "foundry.android.minSdkVersion":
		if profile.MinSdkVersion == 0 {
			profile.MinSdkVersion = level
			profile.Android.MinSDK = KnownVersion(strconv.Itoa(level))
		}
	case "foundry.android.targetSdkVersion":
		if profile.TargetSdkVersion == 0 {
			profile.TargetSdkVersion = level
			profile.Android.TargetSDK = KnownVersion(strconv.Itoa(level))
		}
	case "foundry.android.compileSdkVersion":
		if profile.CompileSdkVersion == 0 {
			profile.CompileSdkVersion = level
			profile.Android.CompileSDK = KnownVersion(strconv.Itoa(level))
		}
	}
}

// ProfileFromGradleContent parses one Gradle file into a profile fragment.
func ProfileFromGradleContent(path, content string) ProjectProfile {
	return ProfileFromGradleContentWithCatalog(path, content, VersionCatalog{})
}

func ProfileFromGradleContentWithCatalog(path, content string, catalog VersionCatalog) ProjectProfile {
	cfg, err := android.ParseBuildGradleContent(content)
	if err != nil {
		return ProjectProfile{}
	}
	catalogContent, _ := stripGradleComments(content, isGroovyScript(path))
	profile := ProfileFromBuildConfig(path, cfg)
	unresolvedAliases := unresolvedCatalogAliases(catalogContent, catalog, path)
	profile.UnresolvedCatalogAliases = unresolvedAliases
	profile.CatalogSources = appendCatalogSources(profile.CatalogSources, catalog.Sources...)
	profile.CatalogCompleteness = catalog.Completeness
	if unresolvedStructuralRefRe.MatchString(catalogContent) || len(unresolvedAliases) > 0 {
		profile.HasUnresolvedDependencyRefs = true
	}
	kotlinProfile, kspProfile, androidProfile, jvmProfile := extractToolingProfile(content, cfg.MinSdkVersion, cfg.TargetSdkVersion, cfg.CompileSdkVersion)
	profile.Kotlin = mergeKotlinProfile(profile.Kotlin, kotlinProfile)
	profile.KSP = mergeKSPProfile(profile.KSP, kspProfile)
	profile.Android = mergeAndroidProfile(profile.Android, androidProfile)
	profile.JVM = mergeJVMProfile(profile.JVM, jvmProfile)
	profile.Java = mergeJavaSourceProfile(profile.Java, JavaSourceProfileFromGradleContent(path, content, cfg))
	applyCatalogProfileFacts(&profile, catalogContent, cfg.Plugins, catalog, path)
	mergeJavaDependencyClasspath(&profile.Java, profile.Dependencies)
	if profile.HasUnresolvedDependencyRefs {
		profile.Java.ClasspathComplete = false
	}
	profile.DependencyExtractionComplete = profile.HasGradle && !profile.HasUnresolvedDependencyRefs
	return profile
}

// ProfileFromBuildConfig converts an android.BuildConfig into a library
// profile fragment.
func ProfileFromBuildConfig(source string, cfg *android.BuildConfig) ProjectProfile {
	if cfg == nil {
		return ProjectProfile{}
	}
	profile := ProjectProfile{
		MinSdkVersion:                cfg.MinSdkVersion,
		TargetSdkVersion:             cfg.TargetSdkVersion,
		CompileSdkVersion:            cfg.CompileSdkVersion,
		HasGradle:                    true,
		DependencyExtractionComplete: true,
	}
	if cfg.IsAndroid {
		profile.Android.AGP = PresentAssumeLatestStable()
	}
	if cfg.CompileSdkVersion > 0 {
		profile.Android.CompileSDK = KnownVersion(strconv.Itoa(cfg.CompileSdkVersion))
	}
	if cfg.TargetSdkVersion > 0 {
		profile.Android.TargetSDK = KnownVersion(strconv.Itoa(cfg.TargetSdkVersion))
	}
	if cfg.MinSdkVersion > 0 {
		profile.Android.MinSDK = KnownVersion(strconv.Itoa(cfg.MinSdkVersion))
	}
	for _, dep := range cfg.Dependencies {
		normalized := Dependency{
			Group:         dep.Group,
			Name:          dep.Name,
			Version:       dep.Version,
			Configuration: dep.Configuration,
			Source:        source,
		}
		profile.Dependencies = append(profile.Dependencies, normalized)
		if dep.Configuration == "ksp" {
			profile.KSP.Processors = append(profile.KSP.Processors, normalized)
			if profile.KSP.Tool.Presence == PresenceUnknown {
				profile.KSP.Tool = PresentAssumeLatestStable()
			}
		}
	}
	return profile
}

// Merge adds another profile fragment into p.
func (p *ProjectProfile) Merge(other ProjectProfile) {
	if other.MinSdkVersion > 0 && (p.MinSdkVersion == 0 || other.MinSdkVersion < p.MinSdkVersion) {
		p.MinSdkVersion = other.MinSdkVersion
	}
	if other.TargetSdkVersion > p.TargetSdkVersion {
		p.TargetSdkVersion = other.TargetSdkVersion
	}
	if other.CompileSdkVersion > p.CompileSdkVersion {
		p.CompileSdkVersion = other.CompileSdkVersion
	}
	p.Dependencies = append(p.Dependencies, other.Dependencies...)
	p.CatalogSources = appendCatalogSources(p.CatalogSources, other.CatalogSources...)
	if other.CatalogCompleteness > p.CatalogCompleteness {
		p.CatalogCompleteness = other.CatalogCompleteness
	}
	p.UnresolvedCatalogAliases = append(p.UnresolvedCatalogAliases, other.UnresolvedCatalogAliases...)
	p.Kotlin = mergeKotlinProfile(p.Kotlin, other.Kotlin)
	p.KSP = mergeKSPProfile(p.KSP, other.KSP)
	p.Android = mergeAndroidProfile(p.Android, other.Android)
	p.JVM = mergeJVMProfile(p.JVM, other.JVM)
	p.Java = mergeJavaSourceProfile(p.Java, other.Java)
	p.HasGradle = p.HasGradle || other.HasGradle
	p.HasUnresolvedDependencyRefs = p.HasUnresolvedDependencyRefs || other.HasUnresolvedDependencyRefs
	p.DependencyExtractionComplete = p.HasGradle && !p.HasUnresolvedDependencyRefs &&
		(p.DependencyExtractionComplete || other.DependencyExtractionComplete)
}

func (p ProjectProfile) HasDependency(group, name string) bool {
	for _, dep := range p.Dependencies {
		if dep.Group == group && dep.Name == name {
			return true
		}
	}
	return false
}

func (p ProjectProfile) HasDependencyGroup(group string) bool {
	for _, dep := range p.Dependencies {
		if dep.Group == group {
			return true
		}
	}
	return false
}

func (p ProjectProfile) HasAnyDependency(coords ...Coordinate) bool {
	for _, coord := range coords {
		if p.HasDependency(coord.Group, coord.Name) {
			return true
		}
	}
	return false
}

// ProvesDependencyAbsent returns true only when the parsed dependency graph is
// complete enough to use absence as evidence. Unknown or partial profiles stay
// conservative.
func (p ProjectProfile) ProvesDependencyAbsent(coords ...Coordinate) bool {
	if p.HasAnyDependency(coords...) {
		return false
	}
	return p.DependencyExtractionComplete
}

func (p ProjectProfile) MayUseAnyDependency(coords ...Coordinate) bool {
	return !p.ProvesDependencyAbsent(coords...)
}
