package librarymodel

import (
	"path/filepath"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/bazel"
	"github.com/kaeawc/krit/internal/fsutil"
)

// ProfileFromPaths builds a profile from a project's library inputs
// (android.Project.LibraryInputPaths): Gradle build and settings files go
// through ProfileFromGradlePaths, and Bazel workspace and rules_jvm_external
// lock files contribute their Maven artifacts as dependencies.
//
// Bazel artifacts only add presence facts. They do not make dependency
// extraction complete, so a library missing from them is never treated as
// proof of absence.
func ProfileFromPaths(paths []string) ProjectProfile {
	var gradle, bazelFiles []string
	for _, path := range paths {
		base := filepath.Base(path)
		if bazel.IsWorkspaceFile(base) || bazel.IsLockFile(base) {
			bazelFiles = append(bazelFiles, path)
		} else {
			gradle = append(gradle, path)
		}
	}
	profile := ProfileFromGradlePaths(gradle)
	profile.Merge(profileFromBazelFiles(bazelFiles))
	profile.InputBase = fsutil.CommonDir(paths)
	return profile
}

// ProfileFromProject is ProfileFromPaths over proj.LibraryInputPaths().
func ProfileFromProject(proj *android.Project) ProjectProfile {
	return ProfileFromPaths(proj.LibraryInputPaths())
}

func profileFromBazelFiles(paths []string) ProjectProfile {
	var profile ProjectProfile
	seen := map[string]bool{}
	for _, path := range paths {
		var artifacts []bazel.Artifact
		if bazel.IsLockFile(filepath.Base(path)) {
			artifacts = bazel.ArtifactsFromLockFile(path)
		} else {
			artifacts = bazel.ArtifactsFromWorkspaceFile(path)
		}
		for _, a := range artifacts {
			key := a.Group + ":" + a.Name + ":" + a.Version
			if seen[key] {
				continue
			}
			seen[key] = true
			profile.Dependencies = append(profile.Dependencies, Dependency{
				Group:         a.Group,
				Name:          a.Name,
				Version:       a.Version,
				Configuration: "implementation",
				Source:        path,
			})
		}
	}
	return profile
}
