package snapshot

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

// CaptureOptions controls a single Capture invocation.
type CaptureOptions struct {
	RepoRoot    string
	CommitSHA   string
	KritVersion string
	Workers     int
	// Now allows tests to inject a deterministic capture time.
	Now func() time.Time
	// WithFindings, when true, runs krit's rule pipeline against the
	// worktree and attaches a per-rule findings rollup to Result. The
	// scan dominates capture wall-time, so this is opt-in.
	WithFindings bool
	// Redact, when true, post-processes the captured blob (and the
	// findings rollup, if WithFindings) through RedactBlob /
	// RedactFindings before returning. Use this for repos with
	// strict-secrecy requirements: snapshots persist Symbol.FQN /
	// Owner / Package / Signature plus File.Path, all of which can
	// embed proprietary identifiers. Redact replaces them with
	// stable one-way hashes so the snapshot still supports diff /
	// timeline / metrics but cannot be reverse-engineered to a
	// source identifier. The flag is captured into Blob.Redacted /
	// Findings.Redacted / Manifest.Redacted so Diff can guard
	// against comparing redacted vs raw.
	Redact bool
}

// Result pairs the structural blob with the metrics rollup derived from
// the same parse. Findings is populated only when CaptureOptions.WithFindings
// is set.
type Result struct {
	Blob     *Blob
	Metrics  *Metrics
	Findings *Findings
}

func Capture(opts CaptureOptions) (*Result, error) {
	if opts.RepoRoot == "" {
		return nil, fmt.Errorf("snapshot: RepoRoot required")
	}
	if opts.CommitSHA == "" {
		return nil, fmt.Errorf("snapshot: CommitSHA required")
	}
	root, err := filepath.Abs(opts.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("snapshot: abs RepoRoot: %w", err)
	}
	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	version := opts.KritVersion
	if version == "" {
		version = "dev"
	}

	graph, err := module.DiscoverModules(root)
	if err != nil {
		return nil, fmt.Errorf("snapshot: discover modules: %w", err)
	}

	ktPaths, javaPaths, fileToModule := collectSources(graph, root)
	ktFiles, _ := scanner.ScanFilesCached(context.Background(), ktPaths, workers, nil)
	javaFiles, _ := scanner.ScanJavaFilesCached(context.Background(), javaPaths, workers, nil)

	idx := scanner.BuildIndex(ktFiles, workers, javaFiles...)

	blob := &Blob{
		SchemaVersion: SchemaVersion,
		KritVersion:   version,
		CommitSHA:     opts.CommitSHA,
		CapturedAt:    now().UnixMilli(),
		RepoRoot:      root,
		Modules:       buildModules(graph, root),
		Files:         buildFiles(ktFiles, javaFiles, fileToModule, root),
		Symbols:       buildSymbols(idx, root),
	}

	allFiles := make([]*scanner.File, 0, len(ktFiles)+len(javaFiles))
	allFiles = append(allFiles, ktFiles...)
	allFiles = append(allFiles, javaFiles...)
	metrics := computeMetrics(blob, allFiles)
	result := &Result{Blob: blob, Metrics: metrics}

	if opts.WithFindings {
		findings, err := RunFindings(context.Background(), root, opts.CommitSHA, FindingsRunOptions{
			RepoRelativeTo: root,
			Workers:        workers,
		})
		if err != nil {
			return nil, fmt.Errorf("snapshot: findings: %w", err)
		}
		result.Findings = findings
	}

	if opts.Redact {
		RedactBlob(result.Blob)
		if result.Findings != nil {
			RedactFindings(result.Findings)
		}
	}

	return result, nil
}

// collectSources walks each module's source roots once via
// CollectKotlinAndJavaFiles and records a path -> gradle-module
// attribution for downstream rollups. When the project has no Gradle
// modules (e.g. a plain Kotlin tree) it falls back to a single
// repo-root walk.
func collectSources(graph *module.Graph, repoRoot string) (kotlin, java []string, fileToModule map[string]string) {
	fileToModule = make(map[string]string)
	seenKt := make(map[string]bool)
	seenJv := make(map[string]bool)

	if graph != nil {
		paths := make([]string, 0, len(graph.Modules))
		for path := range graph.Modules {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, modPath := range paths {
			mod := graph.Modules[modPath]
			roots := mod.SourceRoots
			if len(roots) == 0 {
				roots = []string{filepath.Join(mod.Dir, "src", "main", "kotlin"), filepath.Join(mod.Dir, "src", "main", "java")}
			}
			ktForMod, jvForMod, _ := scanner.CollectKotlinAndJavaFiles(context.Background(), roots, nil)
			for _, p := range ktForMod {
				if !seenKt[p] {
					seenKt[p] = true
					kotlin = append(kotlin, p)
					fileToModule[p] = mod.Path
				}
			}
			for _, p := range jvForMod {
				if !seenJv[p] {
					seenJv[p] = true
					java = append(java, p)
					fileToModule[p] = mod.Path
				}
			}
		}
	}

	if len(kotlin) == 0 && len(java) == 0 {
		ktForRoot, jvForRoot, _ := scanner.CollectKotlinAndJavaFiles(context.Background(), []string{repoRoot}, nil)
		for _, p := range ktForRoot {
			if !seenKt[p] {
				seenKt[p] = true
				kotlin = append(kotlin, p)
			}
		}
		for _, p := range jvForRoot {
			if !seenJv[p] {
				seenJv[p] = true
				java = append(java, p)
			}
		}
	}
	return kotlin, java, fileToModule
}

func buildModules(graph *module.Graph, repoRoot string) []Module {
	if graph == nil {
		return nil
	}
	paths := make([]string, 0, len(graph.Modules))
	for p := range graph.Modules {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	out := make([]Module, 0, len(paths))
	for _, p := range paths {
		m := graph.Modules[p]
		deps := make([]ModuleDep, 0, len(m.Dependencies))
		for _, d := range m.Dependencies {
			deps = append(deps, ModuleDep{Path: d.ModulePath, Configuration: d.Configuration})
		}
		sort.Slice(deps, func(i, j int) bool {
			if deps[i].Path != deps[j].Path {
				return deps[i].Path < deps[j].Path
			}
			return deps[i].Configuration < deps[j].Configuration
		})
		consumers := append([]string(nil), graph.Consumers[p]...)
		sort.Strings(consumers)
		out = append(out, Module{
			Path:         m.Path,
			Dir:          relPath(m.Dir, repoRoot),
			Dependencies: deps,
			Consumers:    consumers,
		})
	}
	return out
}

func buildFiles(kt, jv []*scanner.File, fileToModule map[string]string, repoRoot string) []File {
	out := make([]File, 0, len(kt)+len(jv))
	add := func(f *scanner.File, lang string) {
		out = append(out, File{
			Path:     relPath(f.Path, repoRoot),
			Module:   fileToModule[f.Path],
			Language: lang,
			Lines:    countLines(f),
			Bytes:    len(f.Content),
		})
	}
	for _, f := range kt {
		if f != nil {
			add(f, "kotlin")
		}
	}
	for _, f := range jv {
		if f != nil {
			add(f, "java")
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func countLines(f *scanner.File) int {
	if f == nil {
		return 0
	}
	if len(f.Lines) > 0 {
		return len(f.Lines)
	}
	if len(f.Content) == 0 {
		return 0
	}
	return strings.Count(string(f.Content), "\n") + 1
}

func buildSymbols(idx *scanner.CodeIndex, repoRoot string) []Symbol {
	if idx == nil {
		return nil
	}
	out := make([]Symbol, 0, len(idx.Symbols))
	for _, s := range idx.Symbols {
		out = append(out, Symbol{
			Name:       s.Name,
			Kind:       s.Kind,
			Visibility: s.Visibility,
			File:       relPath(s.File, repoRoot),
			Line:       s.Line,
			Language:   s.Language.String(),
			Package:    s.Package,
			FQN:        s.FQN,
			Owner:      s.Owner,
			Signature:  s.Signature,
			IsOverride: s.IsOverride,
			IsTest:     s.IsTest,
		})
	}
	// Symbols come from worker goroutines and arrive in non-deterministic
	// order; sort so the captured blob is stable across runs.
	sort.Slice(out, func(i, j int) bool {
		if out[i].FQN != out[j].FQN {
			return out[i].FQN < out[j].FQN
		}
		return out[i].Signature < out[j].Signature
	})
	return out
}

// relPath returns absPath relative to repoRoot in slash form. repoRoot
// must already be absolute (Capture absolutises it once on entry).
// Returns absPath unchanged when it falls outside repoRoot.
func relPath(absPath, repoRoot string) string {
	if absPath == "" {
		return ""
	}
	rel, err := filepath.Rel(repoRoot, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return absPath
	}
	return filepath.ToSlash(rel)
}
