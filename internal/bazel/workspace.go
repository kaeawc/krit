package bazel

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// Target is a JVM or Android rule in a BUILD file.
type Target struct {
	// Package is the target's package directory relative to the scan
	// root, slash-separated ("" for the root package).
	Package string
	Name    string
	Kind    string
	// Deps are the target's dependencies within the scanned tree, as
	// scan-root-relative package directories. External repositories
	// (@maven//...) are not included.
	Deps []string
	// Test is set for test rules and testonly targets.
	Test bool
	// ResourceDirs are directories that resource_files globs cover,
	// relative to the scan root.
	ResourceDirs []string
	BuildFile    string
	Line         int
}

// Artifact is a Maven coordinate declared for rules_jvm_external.
type Artifact struct {
	Group, Name, Version string
	// Source is the file that declared it.
	Source string
}

// Workspace is what Discover found under a scan root.
type Workspace struct {
	Root string
	// WorkspaceFiles are MODULE.bazel, WORKSPACE and WORKSPACE.bazel files.
	WorkspaceFiles []string
	BuildFiles     []string
	Targets        []Target
	Artifacts      []Artifact
}

// IsEmpty reports whether no Bazel files were found.
func (w *Workspace) IsEmpty() bool {
	return w == nil || (len(w.WorkspaceFiles) == 0 && len(w.BuildFiles) == 0)
}

// IsBuildFile reports whether name is a Bazel BUILD file name.
func IsBuildFile(name string) bool { return name == "BUILD" || name == "BUILD.bazel" }

// WorkspaceFileNames are the file names that mark a Bazel workspace root.
var WorkspaceFileNames = []string{"MODULE.bazel", "WORKSPACE", "WORKSPACE.bazel", "REPO.bazel"}

// IsWorkspaceFile reports whether name marks a Bazel workspace root.
func IsWorkspaceFile(name string) bool {
	return slices.Contains(WorkspaceFileNames, name)
}

// IsLockFile reports whether name is a rules_jvm_external lock file
// (maven_install.json, or <repo>_install.json for a named repository).
func IsLockFile(name string) bool {
	return strings.HasSuffix(name, "_install.json")
}

// skipDir reports directories Discover never enters: hidden directories,
// Bazel's convenience symlinks and output trees, and node_modules.
func skipDir(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "bazel-") || name == "node_modules"
}

// Discover walks root for BUILD, workspace and lock files and parses them.
// It returns an empty Workspace for a tree without Bazel files.
func Discover(root string) *Workspace {
	abs, err := filepath.Abs(root)
	if err == nil {
		root = abs
	}
	ws := &Workspace{Root: root}
	var lockFiles []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // best-effort discovery skips unreadable entries
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && skipDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		switch {
		case IsBuildFile(name):
			ws.BuildFiles = append(ws.BuildFiles, p)
		case IsWorkspaceFile(name):
			ws.WorkspaceFiles = append(ws.WorkspaceFiles, p)
		case IsLockFile(name):
			lockFiles = append(lockFiles, p)
		}
		return nil
	})
	if ws.IsEmpty() {
		return ws
	}
	roots := workspaceRoots(root, ws.WorkspaceFiles)
	for _, f := range ws.BuildFiles {
		ws.Targets = append(ws.Targets, parseBuildFile(root, roots, f)...)
	}
	for _, f := range ws.WorkspaceFiles {
		ws.Artifacts = append(ws.Artifacts, ArtifactsFromWorkspaceFile(f)...)
	}
	for _, f := range lockFiles {
		ws.Artifacts = append(ws.Artifacts, ArtifactsFromLockFile(f)...)
	}
	return ws
}

// workspaceRoots returns the scan-root-relative directories that hold a
// workspace file, deepest first, so a BUILD file resolves `//` labels
// against its nearest workspace. The scan root always counts as one.
func workspaceRoots(root string, files []string) []string {
	seen := map[string]bool{"": true}
	roots := []string{""}
	for _, f := range files {
		rel, err := filepath.Rel(root, filepath.Dir(f))
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			rel = ""
		}
		if !seen[rel] {
			seen[rel] = true
			roots = append(roots, rel)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return len(roots[i]) > len(roots[j]) })
	return roots
}

func workspaceRootFor(roots []string, pkg string) string {
	for _, r := range roots {
		if r == "" || pkg == r || strings.HasPrefix(pkg, r+"/") {
			return r
		}
	}
	return ""
}

// jvmRuleKinds are rules whose sources krit analyzes. Macros and custom
// rules count too when their srcs name Kotlin or Java files.
var jvmRuleKinds = map[string]bool{
	"kt_jvm_library": true, "kt_jvm_binary": true, "kt_jvm_test": true,
	"kt_android_library": true, "kt_android_local_test": true,
	"kt_js_library": true, "kt_compiler_plugin": false,
	"android_library": true, "android_binary": true, "android_local_test": true,
	"android_instrumentation_test": true,
	"java_library":                 true, "java_binary": true, "java_test": true,
	"java_import": false, "kt_jvm_import": false,
}

func isJVMTarget(c Call) bool {
	if enabled, ok := jvmRuleKinds[c.Kind]; ok {
		return enabled
	}
	for _, s := range append(c.Attr("srcs").Strings(), c.Attr("srcs").Globs()...) {
		if strings.HasSuffix(s, ".kt") || strings.HasSuffix(s, ".java") || strings.HasSuffix(s, ".kts") {
			return true
		}
	}
	return false
}

func parseBuildFile(root string, roots []string, file string) []Target {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	pkg, err := filepath.Rel(root, filepath.Dir(file))
	if err != nil {
		return nil
	}
	pkg = filepath.ToSlash(pkg)
	if pkg == "." {
		pkg = ""
	}
	wsRoot := workspaceRootFor(roots, pkg)
	var targets []Target
	for _, c := range ParseFile(data) {
		if !isJVMTarget(c) {
			continue
		}
		t := Target{
			Package:   pkg,
			Name:      c.Attr("name").Str,
			Kind:      c.Kind,
			Test:      strings.HasSuffix(c.Kind, "_test") || c.Attr("testonly").Bool,
			BuildFile: file,
			Line:      c.Line,
		}
		seen := map[string]bool{}
		for _, attr := range []string{"deps", "exports", "runtime_deps", "associates", "plugins"} {
			for _, label := range c.Attr(attr).Strings() {
				dep, ok := resolveLabel(label, pkg, wsRoot)
				if ok && dep != pkg && !seen[dep] {
					seen[dep] = true
					t.Deps = append(t.Deps, dep)
				}
			}
		}
		t.ResourceDirs = resourceDirs(root, pkg, c.Attr("resource_files").Globs())
		targets = append(targets, t)
	}
	return targets
}

// resolveLabel maps a dependency label to the scan-root-relative package
// directory it names. External labels (@repo//...) resolve only for the
// main repository (@//, @@//).
func resolveLabel(label, pkg, wsRoot string) (string, bool) {
	switch {
	case strings.HasPrefix(label, "@@//"):
		label = label[2:]
	case strings.HasPrefix(label, "@//"):
		label = label[1:]
	case strings.HasPrefix(label, "@"):
		return "", false
	}
	if strings.HasPrefix(label, ":") || !strings.Contains(label, "//") && !strings.Contains(label, ":") {
		// Same package (":lib" or a bare "lib").
		return pkg, true
	}
	if !strings.HasPrefix(label, "//") {
		return "", false
	}
	target := strings.TrimPrefix(label, "//")
	if i := strings.IndexByte(target, ':'); i >= 0 {
		target = target[:i]
	}
	return path.Join(wsRoot, target), true
}

// resourceDirs returns the directories that resource_files globs such as
// "res/**" or "src/main/res/**/*.xml" cover.
func resourceDirs(root, pkg string, globs []string) []string {
	var dirs []string
	seen := map[string]bool{}
	for _, g := range globs {
		prefix := g
		if i := strings.IndexAny(prefix, "*?["); i >= 0 {
			prefix = prefix[:i]
		}
		prefix = strings.TrimSuffix(prefix, "/")
		if prefix == "" {
			continue
		}
		dir := path.Join(pkg, prefix)
		if seen[dir] {
			continue
		}
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(dir))); err == nil && info.IsDir() {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// coordinateRe matches a Maven coordinate string group:artifact[:packaging[:classifier]]:version.
var coordinateRe = regexp.MustCompile(`^([A-Za-z0-9_.\-]+):([A-Za-z0-9_.\-]+)(?::[A-Za-z0-9_.\-]+){0,3}$`)

// ArtifactsFromWorkspaceFile reads Maven artifacts from maven.install /
// maven.artifact calls in MODULE.bazel and maven_install calls in
// WORKSPACE.
func ArtifactsFromWorkspaceFile(file string) []Artifact {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var out []Artifact
	for _, c := range ParseFile(data) {
		kind := c.Kind
		if i := strings.LastIndexByte(kind, '.'); i >= 0 {
			kind = kind[i+1:]
		}
		switch kind {
		case "install", "maven_install":
			for _, s := range c.Attr("artifacts").Strings() {
				if a, ok := parseCoordinate(s); ok {
					a.Source = file
					out = append(out, a)
				}
			}
		case "artifact":
			if g, n := c.Attr("group").Str, c.Attr("artifact").Str; g != "" && n != "" {
				out = append(out, Artifact{Group: g, Name: n, Version: c.Attr("version").Str, Source: file})
			}
		}
	}
	return out
}

func parseCoordinate(s string) (Artifact, bool) {
	if !coordinateRe.MatchString(s) {
		return Artifact{}, false
	}
	parts := strings.Split(s, ":")
	a := Artifact{Group: parts[0], Name: parts[1]}
	if len(parts) > 2 {
		a.Version = parts[len(parts)-1]
	}
	return a, true
}

// ArtifactsFromLockFile reads the artifact keys of a rules_jvm_external
// lock file. Version 2 lock files key "artifacts" by "group:artifact";
// version 1 files list "dependency_tree.dependencies[].coord".
func ArtifactsFromLockFile(file string) []Artifact {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var lock struct {
		Artifacts      map[string]json.RawMessage `json:"artifacts"`
		DependencyTree struct {
			Dependencies []struct {
				Coord string `json:"coord"`
			} `json:"dependencies"`
		} `json:"dependency_tree"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil
	}
	var out []Artifact
	keys := make([]string, 0, len(lock.Artifacts))
	for k := range lock.Artifacts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if a, ok := parseCoordinate(k); ok {
			a.Source = file
			out = append(out, a)
		}
	}
	for _, d := range lock.DependencyTree.Dependencies {
		if a, ok := parseCoordinate(d.Coord); ok {
			a.Source = file
			out = append(out, a)
		}
	}
	return out
}

// ResourceDirsFromBuildFile returns the absolute directories that the
// resource_files globs of a BUILD file's rules cover.
func ResourceDirsFromBuildFile(file string) []string {
	data, err := os.ReadFile(file)
	if err != nil || !strings.Contains(string(data), "resource_files") {
		return nil
	}
	dir := filepath.Dir(file)
	var out []string
	for _, c := range ParseFile(data) {
		for _, rel := range resourceDirs(dir, "", c.Attr("resource_files").Globs()) {
			out = append(out, filepath.Join(dir, filepath.FromSlash(rel)))
		}
	}
	return out
}
