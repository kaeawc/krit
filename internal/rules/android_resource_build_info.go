package rules

import (
	"path/filepath"

	"github.com/kaeawc/krit/internal/rules/api"
)

// ResourceBuildInfoReader is implemented by a resource rule whose findings for a res directory depend on build files
// beyond the res files: the build script of the module that owns it (an app or a library, its SDK levels, namespace
// or applicationId), read through lookupManifestGradleInfo or readManifestGradleInfo, the version catalogs that
// script's aliases resolve through, and files the rule reads itself. Result caches over resource rules key on what
// every active one reads; a resource rule that reads build files without implementing it caches stale findings
// (#1883).
type ResourceBuildInfoReader interface {
	ResourceBuildInfoReads(resDir string) ResourceBuildInfoReads
}

// ResourceBuildInfoReads is what a resource rule reads for one res directory beyond its files.
type ResourceBuildInfoReads struct {
	// Lookups are paths the rule reads module build info for as lookupManifestGradleInfo does for a manifest: the
	// build script it reads is the first of ManifestBuildScriptCandidates(lookup) that exists, and through it the
	// version catalogs its aliases resolve against (BuildScriptCatalogReads). A cache key hashes them as it hashes the
	// manifests, through the same fingerprint.
	Lookups []string
	// Files are the files the rule reads itself, whether or not they exist: creating one changes what is read.
	Files []string
}

// ResourceBuildInfoLocalReader is a ResourceBuildInfoReader whose findings for a res directory depend only on what it
// reads for that directory, ResourceBuildInfoReads(resDir), and not on its reads for the project's other res
// directories. Each res directory's cache entry then keys on those reads alone, so an edit of one module's build
// script re-runs the rule for that module's res directories only (#1943). A reader that does not implement it, such as
// the unused-resource graph that scans the files beside every res root, has every res directory keyed on its reads
// for all of them.
type ResourceBuildInfoLocalReader interface {
	ResourceBuildInfoReader
	ResourceBuildInfoReadsAreLocal()
}

// ResourceBuildInfoReadersOf returns the IDs and readers of the active resource rules that read build files for a
// res directory (ResourceBuildInfoReader), in rule order; none when no such rule is active.
func ResourceBuildInfoReadersOf(active []*api.Rule) (ids []string, readers []ResourceBuildInfoReader) {
	for _, r := range active {
		if r == nil || RuleScope(r) != api.ScopeResource {
			continue
		}
		if reader, ok := r.Implementation.(ResourceBuildInfoReader); ok {
			ids = append(ids, r.ID)
			readers = append(readers, reader)
		}
	}
	return ids, readers
}

// resModuleDir is the module directory that owns a res directory: `<module>` of `<module>/src/<set>/res`, or the
// parent of any other res directory (`<module>/res`).
func resModuleDir(resDir string) string {
	module := filepath.Dir(resDir)
	if filepath.Base(filepath.Dir(module)) == "src" {
		module = filepath.Dir(filepath.Dir(module))
	}
	return module
}

// resSourceSetManifest is the manifest beside a res directory, in its source set.
func resSourceSetManifest(resDir string) string {
	return filepath.Join(filepath.Dir(resDir), "AndroidManifest.xml")
}

// moduleDirLookup is a lookup path (ResourceBuildInfoReads.Lookups) whose search starts at module, for a rule that
// reads the build script of module itself through readManifestGradleInfo(module): manifestModuleSearchDirs starts
// at the grandparent of the path. When module has no build script the search reads one further up, which only keys
// a file the rule does not read.
func moduleDirLookup(module string) string {
	return filepath.Join(module, "src", "AndroidManifest.xml")
}

// moduleBuildScripts lists both build scripts a module directory can hold. A rule reads the first that exists, so
// creating the other one can change what it reads.
func moduleBuildScripts(module string) []string {
	files := make([]string, 0, len(manifestBuildScriptNames))
	for _, name := range manifestBuildScriptNames {
		files = append(files, filepath.Join(module, name))
	}
	return files
}

// moduleIdentityManifests are the manifests readModuleIdentity reads a module's namespace from when its build script
// declares none.
func moduleIdentityManifests(resDir string) []string {
	return []string{resSourceSetManifest(resDir), filepath.Join(resModuleDir(resDir), "src", "main", "AndroidManifest.xml")}
}

// unusedResourceRootFiles are the files beside a res root that buildResourceGraph scans for resource references: the
// manifest of its source set and the build scripts one and two directories above it.
func unusedResourceRootFiles(resRoot string) []string {
	sourceSet := filepath.Dir(resRoot)
	return []string{
		filepath.Join(sourceSet, "AndroidManifest.xml"),
		filepath.Join(filepath.Dir(sourceSet), "build.gradle"), filepath.Join(filepath.Dir(sourceSet), "build.gradle.kts"),
		filepath.Join(filepath.Dir(filepath.Dir(sourceSet)), "build.gradle"), filepath.Join(filepath.Dir(filepath.Dir(sourceSet)), "build.gradle.kts"),
	}
}

// ResourceBuildInfoReads: LibraryCustomView looks up the module of every layout (lookupManifestGradleInfo of the
// layout path), whose search starts at the res directory for a layout in any of its folders.
func (r *LibraryCustomViewRule) ResourceBuildInfoReads(resDir string) ResourceBuildInfoReads {
	return ResourceBuildInfoReads{Lookups: []string{filepath.Join(resDir, "layout", "view.xml")}}
}

// ResourceBuildInfoReads: the module minSdk (resourceModuleMinSdk) is read through the manifest beside the res
// directory, and from its uses-sdk when the build script declares none.
func (r *LabelForResourceRule) ResourceBuildInfoReads(resDir string) ResourceBuildInfoReads {
	return resourceMinSdkReads(resDir)
}

// ResourceBuildInfoReads: see LabelForResourceRule.ResourceBuildInfoReads.
func (r *UnusedAttributeResourceRule) ResourceBuildInfoReads(resDir string) ResourceBuildInfoReads {
	return resourceMinSdkReads(resDir)
}

func resourceMinSdkReads(resDir string) ResourceBuildInfoReads {
	manifest := resSourceSetManifest(resDir)
	return ResourceBuildInfoReads{Lookups: []string{manifest}, Files: []string{manifest}}
}

// ResourceBuildInfoReads: the module manifest gives the activities and themes, and the target SDK is read through it
// (translucentTargetSdk).
func (r *TranslucentOrientationRule) ResourceBuildInfoReads(resDir string) ResourceBuildInfoReads {
	manifest := translucentManifestPath(resDir)
	return ResourceBuildInfoReads{Lookups: []string{manifest}, Files: []string{manifest}}
}

// ResourceBuildInfoReads: readModuleIdentity reads the module's build script (namespace, applicationId, plugins,
// through catalog aliases too) and, for the namespace, its manifests.
func (r *RelativeClassResolutionRule) ResourceBuildInfoReads(resDir string) ResourceBuildInfoReads {
	module := resModuleDir(resDir)
	return ResourceBuildInfoReads{
		Lookups: []string{moduleDirLookup(module)},
		Files:   append(moduleBuildScripts(module), moduleIdentityManifests(resDir)...),
	}
}

// ResourceBuildInfoReads: readModuleVectorSettings reads the module's build script (minSdk, useSupportLibrary) and
// names the module by the Gradle path the nearest settings script above it gives.
func (r *VectorDrawableCompatRule) ResourceBuildInfoReads(resDir string) ResourceBuildInfoReads {
	module := resModuleDir(resDir)
	return ResourceBuildInfoReads{
		Lookups: []string{moduleDirLookup(module)},
		Files:   append(moduleBuildScripts(module), gradleSettingsCandidates(module)...),
	}
}

// ResourceBuildInfoReads: the resource graph scans the files beside every res root for references. Each root is a
// res directory of the project, whose reads the cache keys take from this method too.
func (r *UnusedResourcesRule) ResourceBuildInfoReads(resDir string) ResourceBuildInfoReads {
	return ResourceBuildInfoReads{Files: unusedResourceRootFiles(resDir)}
}

// ResourceBuildInfoReads: see UnusedResourcesRule.ResourceBuildInfoReads.
func (r *UnusedIDsRule) ResourceBuildInfoReads(resDir string) ResourceBuildInfoReads {
	return ResourceBuildInfoReads{Files: unusedResourceRootFiles(resDir)}
}

// ResourceBuildInfoReadsAreLocal: it looks up the module of the layouts of the res directory it runs on.
func (r *LibraryCustomViewRule) ResourceBuildInfoReadsAreLocal() {}

// ResourceBuildInfoReadsAreLocal: the minSdk is that of the res directory's own module.
func (r *LabelForResourceRule) ResourceBuildInfoReadsAreLocal() {}

// ResourceBuildInfoReadsAreLocal: the minSdk is that of the res directory's own module.
func (r *UnusedAttributeResourceRule) ResourceBuildInfoReadsAreLocal() {}

// ResourceBuildInfoReadsAreLocal: it reads the manifest and target SDK of the res directory's own module.
func (r *TranslucentOrientationRule) ResourceBuildInfoReadsAreLocal() {}

// ResourceBuildInfoReadsAreLocal: it reads the identity of the res directory's own module.
func (r *RelativeClassResolutionRule) ResourceBuildInfoReadsAreLocal() {}

// ResourceBuildInfoReadsAreLocal: it reads the res directory's own module and the settings script that names it.
func (r *VectorDrawableCompatRule) ResourceBuildInfoReadsAreLocal() {}
