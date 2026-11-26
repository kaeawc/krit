package gradlemodel

import (
	"os"
	"path/filepath"
	"slices"
)

// rClassJarFolders groups AGP's intermediates folders that hold a module's own R
// jar. The Gradle plugin records the path AGP writes the jar to
// (intermediates/<folder>/<component>/<task>/R.jar), predicted from AGP's major
// version and task names, because listing the build directory while Gradle
// configures the build would make the directory a configuration-cache input
// (#1645). The jar is looked up here instead, when krit runs: a folder of the same
// group is a different AGP major's name for the same artifact.
var rClassJarFolders = [][]string{
	{"compile_and_runtime_not_namespaced_r_class_jar", "compile_and_runtime_r_class_jar"},
	{"compile_r_class_jar"},
}

// resolveRClassJars replaces each predicted R jar that is absent on disk with the
// R jars AGP wrote for the same component: any task directory under the predicted
// folder or the other folders of its group. A prediction that matches nothing is
// kept, and dropped as missing like any other classpath entry.
func resolveRClassJars(project *Project) {
	for i := range project.SourceSets {
		set := &project.SourceSets[i]
		var resolved []string
		changed := false
		for _, entry := range set.GeneratedClasspath {
			found := findRClassJars(entry)
			if len(found) == 0 {
				resolved = append(resolved, entry)
				continue
			}
			changed = true
			for _, jar := range found {
				if !slices.Contains(resolved, jar) {
					resolved = append(resolved, jar)
				}
			}
		}
		if changed {
			set.GeneratedClasspath = resolved
		}
	}
}

// findRClassJars returns the R jars standing in for a missing predicted entry, or
// nil when the entry exists or is not an AGP R jar path.
func findRClassJars(entry string) []string {
	if filepath.Base(entry) != "R.jar" {
		return nil
	}
	if _, err := os.Stat(entry); err == nil {
		return nil
	}
	componentDir := filepath.Dir(filepath.Dir(entry))
	folderDir := filepath.Dir(componentDir)
	intermediates := filepath.Dir(folderDir)
	if filepath.Base(intermediates) != "intermediates" {
		return nil
	}
	group := rClassJarGroup(filepath.Base(folderDir))
	if group == nil {
		return nil
	}
	component := filepath.Base(componentDir)
	var found []string
	for _, folder := range group {
		// ReadDir, not Glob: a checkout path may contain Glob metacharacters ([, *, ?).
		dir := filepath.Join(intermediates, folder, component)
		tasks, _ := os.ReadDir(dir)
		for _, task := range tasks {
			if !task.IsDir() {
				continue
			}
			jar := filepath.Join(dir, task.Name(), "R.jar")
			if info, err := os.Stat(jar); err == nil && info.Mode().IsRegular() {
				found = append(found, filepath.ToSlash(jar))
			}
		}
	}
	slices.Sort(found)
	return found
}

func rClassJarGroup(folder string) []string {
	for _, group := range rClassJarFolders {
		if slices.Contains(group, folder) {
			return group
		}
	}
	return nil
}
