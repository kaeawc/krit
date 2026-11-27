package scan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/pipeline"
)

// --delta's base scan runs in a fresh worktree of the base ref, where the
// exported build model (.krit/gradle-model or .krit/bazel-model) is missing:
// it is untracked. Without it FIR is skipped there and PR mode cannot compile
// the changed files' modules, so the two sides would judge files differently
// (#1660). forwardModelToBase gives the base scan a copy of the model the
// current scan reads, rewritten for the worktree:
//
//   - The project directories and source directories (`dir`, `sourceDirs`)
//     and `rootDir` are the base ref's: they are rebased into the worktree.
//   - Every other path under the repository is a build output (generated
//     sources and jars, R.jar, project jars, compiler-plugin jars). The fresh
//     worktree has none of them, so a path the worktree lacks keeps pointing
//     at this checkout's output, where the base compiles against the outputs
//     this checkout built.
//
// A model the current scan discovered goes where the base scan discovers it,
// unless the base ref commits its own there. A --gradle-model directory is
// copied outside the worktree and passed with --gradle-model, so each side
// finds the model the same way (discovery or the flag).

// baseModel is the model the base scan reads: GradleModelArg is the
// --gradle-model directory to pass, "" when the base discovers it (or there
// is none); Forwarded is false when the current scan has no model.
type baseModel struct {
	GradleModelArg string
	Forwarded      bool
}

// forwardModelToBase copies the current scan's model for the base scan in
// worktree tmp; scratch is a directory outside the worktree.
func forwardModelToBase(f *scanFlags, paths []string, repoRoot, tmp, scratch string) (baseModel, error) {
	if *f.NoGradleModel && *f.GradleModel == "" {
		return baseModel{}, nil
	}
	dir := pipeline.GradleModelDir(*f.GradleModel, false, firstScanPath(paths))
	if dir == "" {
		return baseModel{}, nil
	}
	src, err := filepath.Abs(dir)
	if err != nil {
		return baseModel{}, err
	}
	if *f.GradleModel != "" {
		dest := filepath.Join(scratch, "model", filepath.Base(src))
		if err := copyModelForWorktree(src, dest, repoRoot, tmp); err != nil {
			return baseModel{}, err
		}
		return baseModel{GradleModelArg: dest, Forwarded: true}, nil
	}
	rel, err := filepath.Rel(repoRoot, src)
	if err != nil {
		return baseModel{}, err
	}
	if rel == "." || strings.HasPrefix(rel, "..") {
		return baseModel{}, fmt.Errorf("model %s is outside the repository", src)
	}
	dest := filepath.Join(tmp, rel)
	if _, err := os.Stat(dest); err == nil {
		// The base ref commits its own model: that one describes its build.
		return baseModel{Forwarded: true}, nil
	}
	if err := copyModelForWorktree(src, dest, repoRoot, tmp); err != nil {
		return baseModel{}, err
	}
	return baseModel{Forwarded: true}, nil
}

// copyModelForWorktree copies the model directory src to dest, rewriting the
// paths of its JSON files from the checkout at repoRoot to the worktree tmp.
// Other files are copied as they are.
func copyModelForWorktree(src, dest, repoRoot, tmp string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		out := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if filepath.Ext(p) == ".json" {
			if data, err = rewriteModelJSON(data, src, repoRoot, tmp); err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
		}
		return os.WriteFile(out, data, 0o600)
	})
}

// sourceModelKeys name the model fields that hold the base ref's own inputs.
var sourceModelKeys = map[string]bool{"rootDir": true, "dir": true, "sourceDirs": true}

// rewriteModelJSON rewrites one model file read from modelDir. Its paths are
// first rebased from the recorded rootDir onto the checkout the model belongs
// to (as gradlemodel.Load does for <root>/.krit/gradle-model), then from
// repoRoot into tmp. Unknown fields are kept.
func rewriteModelJSON(data []byte, modelDir, repoRoot, tmp string) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	obj, ok := doc.(map[string]any)
	if !ok {
		return data, nil
	}
	recorded, _ := obj["rootDir"].(string)
	actual := recorded
	if root, ok := modelCheckoutRoot(modelDir); ok {
		actual = root
	}
	r := modelRewriter{
		recorded: slashPath(recorded), actual: slashPath(actual),
		repoRoot: slashPath(repoRoot), tmp: slashPath(tmp),
	}
	return json.MarshalIndent(r.value(doc, ""), "", "  ")
}

// modelCheckoutRoot is <root> for a model directory <root>/.krit/<name>-model.
func modelCheckoutRoot(dir string) (string, bool) {
	parent := filepath.Dir(dir)
	if filepath.Base(parent) != ".krit" || !strings.HasSuffix(filepath.Base(dir), "-model") {
		return "", false
	}
	return filepath.Dir(parent), true
}

type modelRewriter struct{ recorded, actual, repoRoot, tmp string }

func (r modelRewriter) value(v any, key string) any {
	switch v := v.(type) {
	case map[string]any:
		for k, child := range v {
			v[k] = r.value(child, k)
		}
		return v
	case []any:
		for i, child := range v {
			v[i] = r.value(child, key)
		}
		return v
	case string:
		if key == "compilerPluginOptions" {
			// `plugin:<id>:<key>=<path>`: a file option holds a path.
			if name, value, ok := strings.Cut(v, "="); ok {
				return name + "=" + r.path(value, key)
			}
			return v
		}
		return r.path(v, key)
	default:
		return v
	}
}

// path rewrites p when it lies in the checkout: into the worktree for the
// base ref's inputs, and for a build output only when the worktree has it.
func (r modelRewriter) path(p, key string) string {
	p = rebase(slashPath(p), r.recorded, r.actual)
	in := rebase(p, r.repoRoot, r.tmp)
	if in == p {
		return p
	}
	if sourceModelKeys[key] {
		return in
	}
	if _, err := os.Stat(filepath.FromSlash(in)); err == nil {
		return in
	}
	if _, err := os.Stat(filepath.FromSlash(p)); err == nil {
		return p
	}
	return in
}

// rebase rewrites p from under from to the same relative path under to.
func rebase(p, from, to string) string {
	if from == "" || from == to || !path.IsAbs(p) {
		return p
	}
	if p == from {
		return to
	}
	if rest, ok := strings.CutPrefix(p, from+"/"); ok {
		return to + "/" + rest
	}
	return p
}

func slashPath(p string) string {
	if p == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(p))
}

// buildInputNames are the files whose changes alter what the model
// describes (dependencies, compiler options, plugins, modules).
var buildInputNames = map[string]bool{
	"settings.gradle": true, "settings.gradle.kts": true, "gradle.properties": true,
	"BUILD": true, "BUILD.bazel": true, "MODULE.bazel": true, "WORKSPACE": true, "WORKSPACE.bazel": true,
}

// isBuildInput reports whether a repository-relative path is a build input.
func isBuildInput(rel string) bool {
	rel = filepath.ToSlash(rel)
	base := path.Base(rel)
	switch {
	case buildInputNames[base],
		strings.HasSuffix(base, ".gradle"), strings.HasSuffix(base, ".gradle.kts"),
		strings.HasSuffix(base, ".versions.toml"), strings.HasSuffix(base, ".bzl"),
		strings.HasPrefix(rel, "buildSrc/"), strings.HasPrefix(rel, "build-logic/"),
		strings.Contains(rel, "/buildSrc/"), strings.Contains(rel, "/build-logic/"):
		return true
	}
	return false
}

// changedBuildInputs lists the build inputs that changed since ref (git's
// repository-relative names), for the warning that the base scan compiles
// with this checkout's model.
func changedBuildInputs(ref string, repoRoot string) ([]string, error) {
	out, err := exec.CommandContext(context.Background(), "git", "-C", repoRoot, "diff", "--name-only", ref, "--").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("git diff %s: %s", ref, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}
	var changed []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" && isBuildInput(line) {
			changed = append(changed, line)
		}
	}
	return changed, nil
}
