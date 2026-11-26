package gradlemodel

import (
	"os"
	"path/filepath"
	"strings"
)

// A model holds absolute paths: the build root (`rootDir`) and the Gradle user
// home (`gradleUserHome`, where dependency jars live) of the machine that
// exported it. A CI job that restores `.krit/gradle-model` from a cache, instead
// of running Gradle, can have both somewhere else. Load rebases them so the
// model keeps working:
//
//   - The build root moves with the model. A model read from
//     `<root>/.krit/gradle-model` was exported for `<root>`, so paths under the
//     recorded rootDir are rewritten under `<root>`. A model directory elsewhere
//     (a copy passed with --gradle-model) is read as written.
//   - The Gradle user home is rebased only when the recorded one is gone: a
//     build that ran Gradle with another user home than krit sees (`-g`,
//     `org.gradle.user.home`) still has its jars where the model says. The
//     current one is $GRADLE_USER_HOME, else ~/.gradle.
//
// Gradle's dependency caches keep the same layout under any user home, so a
// rebased jar path is the same artifact when the cache was restored too.

// relocation rewrites paths under from to the same relative path under to.
type relocation struct{ from, to string }

func (r relocation) apply(path string) string {
	if r.from == "" || r.from == r.to {
		return path
	}
	if path == r.from {
		return r.to
	}
	if rest, ok := strings.CutPrefix(path, r.from+"/"); ok {
		return r.to + "/" + rest
	}
	return path
}

// modelRelocations returns the rewrites for a model file read from dir that
// records rootDir and gradleUserHome, in the order they apply.
func modelRelocations(dir, rootDir, gradleUserHome string) []relocation {
	var out []relocation
	root := relocation{}
	if rootDir != "" {
		if actual, ok := modelDirRoot(dir); ok && !samePath(rootDir, actual) {
			root = relocation{from: slashClean(rootDir), to: slashClean(actual)}
			out = append(out, root)
		}
	}
	if gradleUserHome != "" {
		recorded := root.apply(slashClean(gradleUserHome))
		if _, err := os.Stat(filepath.FromSlash(recorded)); err != nil {
			if current := currentGradleUserHome(); current != "" && !samePath(recorded, current) {
				out = append(out, relocation{from: recorded, to: slashClean(current)})
			}
		}
	}
	return out
}

// modelDirRoot returns <root> for a model directory <root>/.krit/gradle-model.
func modelDirRoot(dir string) (string, bool) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	if filepath.Base(abs) != "gradle-model" || filepath.Base(filepath.Dir(abs)) != ".krit" {
		return "", false
	}
	return filepath.Dir(filepath.Dir(abs)), true
}

func currentGradleUserHome() string {
	if env := os.Getenv("GRADLE_USER_HOME"); env != "" {
		if abs, err := filepath.Abs(env); err == nil {
			return abs
		}
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".gradle")
}

// samePath reports whether a and b name the same directory, through symlinks
// (macOS's /var and /private/var) when both exist.
func samePath(a, b string) bool {
	if slashClean(a) == slashClean(b) {
		return true
	}
	ai, errA := os.Stat(filepath.FromSlash(a))
	bi, errB := os.Stat(filepath.FromSlash(b))
	return errA == nil && errB == nil && os.SameFile(ai, bi)
}

func slashClean(path string) string { return filepath.ToSlash(filepath.Clean(path)) }

// relocateProject applies rewrites to every path a project lists.
func relocateProject(project *Project, rewrites []relocation) {
	if len(rewrites) == 0 {
		return
	}
	one := func(path string) string {
		for _, r := range rewrites {
			path = r.apply(path)
		}
		return path
	}
	all := func(paths []string) {
		for i, path := range paths {
			paths[i] = one(path)
		}
	}
	project.Dir = one(filepath.ToSlash(project.Dir))
	for i := range project.SourceSets {
		set := &project.SourceSets[i]
		all(set.SourceDirs)
		all(set.GeneratedSourceDirs)
		all(set.GeneratedSourceJars)
		all(set.ClasspathEntries)
		all(set.GeneratedClasspath)
		all(set.BootClasspath)
		if set.Kotlin != nil {
			all(set.Kotlin.CompilerPluginClasspath)
			// `plugin:<id>:<key>=<value>`: a file option (KGP's FilesSubpluginOption, such
			// as Compose's stabilityConfigurationPath) holds an absolute path.
			for i, option := range set.Kotlin.CompilerPluginOptions {
				if name, value, ok := strings.Cut(option, "="); ok {
					set.Kotlin.CompilerPluginOptions[i] = name + "=" + one(value)
				}
			}
		}
	}
}
