package librarymodel

import (
	"os"
	"strconv"

	"github.com/kaeawc/krit/internal/android"
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
	MinSdkVersion     int
	TargetSdkVersion  int
	CompileSdkVersion int

	Kotlin  KotlinProfile
	KSP     KSPProfile
	Android AndroidProfile
	JVM     JVMProfile

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
	var profile ProjectProfile
	catalogs := make(map[string]VersionCatalog)
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var catalog VersionCatalog
		if catalogSources, ok := FindVersionCatalogSourcesForGradlePath(path); ok {
			catalogKey := catalogSourceCacheKey(catalogSources)
			catalog, ok = catalogs[catalogKey]
			if !ok {
				catalog, _ = LoadVersionCatalogForGradlePath(path)
				catalogs[catalogKey] = catalog
			}
		}
		profile.Merge(ProfileFromGradleContentWithCatalog(path, string(content), catalog))
	}
	return profile
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
	catalogContent := stripGradleComments(content)
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
	applyCatalogProfileFacts(&profile, catalogContent, cfg.Plugins, catalog, path)
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
