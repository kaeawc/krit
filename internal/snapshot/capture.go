package snapshot

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

// CaptureOptions controls a single snapshot capture invocation.
type CaptureOptions struct {
	// RepoRoot is the absolute path to the project root. Required.
	RepoRoot string
	// CommitSHA is the git commit being captured. Required (callers
	// resolve it via the git helper or pass it explicitly).
	CommitSHA string
	// KritVersion is stamped on the blob. Empty defaults to "dev"
	// to match the CLI's untagged build behavior.
	KritVersion string
	// Workers, when zero, falls back to runtime.NumCPU().
	Workers int
	// Now allows tests to inject a deterministic capture time. When
	// nil the wall clock is used.
	Now func() time.Time
}

// Result is the output of a Capture invocation: the structural graph
// blob plus the per-file/per-module metrics rollup derived from the
// same parse. Callers that only need the blob can read .Blob; callers
// driving a timeline query also need .Metrics.
type Result struct {
	Blob    *Blob
	Metrics *Metrics
}

// Capture walks RepoRoot, builds a structural graph (module discovery +
// cross-file scanner index over Kotlin and Java sources), and returns a
// Result containing both the cold-path graph blob and the dense scalar
// rollup. No findings are computed — this is deliberately separate from
// rule dispatch.
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
	ktFiles := parseKotlin(ktPaths)
	javaFiles := parseJava(javaPaths)

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
	return &Result{Blob: blob, Metrics: metrics}, nil
}

// collectSources walks each discovered module's source roots. fileToModule
// maps absolute source path -> gradle module path so per-file rollups can
// attribute the file without re-walking the graph later.
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
			ktForMod, _ := scanner.CollectKotlinFiles(roots, nil)
			jvForMod, _ := scanner.CollectJavaFiles(roots, nil)
			for _, p := range ktForMod {
				abs, _ := filepath.Abs(p)
				if !seenKt[abs] {
					seenKt[abs] = true
					kotlin = append(kotlin, p)
					fileToModule[abs] = mod.Path
				}
			}
			for _, p := range jvForMod {
				abs, _ := filepath.Abs(p)
				if !seenJv[abs] {
					seenJv[abs] = true
					java = append(java, p)
					fileToModule[abs] = mod.Path
				}
			}
		}
	}

	if len(kotlin) == 0 && len(java) == 0 {
		ktForRoot, _ := scanner.CollectKotlinFiles([]string{repoRoot}, nil)
		jvForRoot, _ := scanner.CollectJavaFiles([]string{repoRoot}, nil)
		for _, p := range ktForRoot {
			abs, _ := filepath.Abs(p)
			if !seenKt[abs] {
				seenKt[abs] = true
				kotlin = append(kotlin, p)
			}
		}
		for _, p := range jvForRoot {
			abs, _ := filepath.Abs(p)
			if !seenJv[abs] {
				seenJv[abs] = true
				java = append(java, p)
			}
		}
	}
	return kotlin, java, fileToModule
}

func parseKotlin(paths []string) []*scanner.File {
	out := make([]*scanner.File, 0, len(paths))
	for _, p := range paths {
		f, err := scanner.ParseFile(p)
		if err != nil || f == nil {
			continue
		}
		out = append(out, f)
	}
	return out
}

func parseJava(paths []string) []*scanner.File {
	out := make([]*scanner.File, 0, len(paths))
	for _, p := range paths {
		f, err := scanner.ParseJavaFile(p)
		if err != nil || f == nil {
			continue
		}
		out = append(out, f)
	}
	return out
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
		abs, _ := filepath.Abs(f.Path)
		out = append(out, File{
			Path:     relPath(f.Path, repoRoot),
			Module:   fileToModule[abs],
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
	sort.Slice(out, func(i, j int) bool {
		if out[i].FQN != out[j].FQN {
			return out[i].FQN < out[j].FQN
		}
		return out[i].Signature < out[j].Signature
	})
	return out
}

func relPath(absPath, repoRoot string) string {
	if absPath == "" {
		return ""
	}
	abs, err := filepath.Abs(absPath)
	if err != nil {
		return absPath
	}
	rel, err := filepath.Rel(repoRoot, abs)
	if err != nil {
		return absPath
	}
	if strings.HasPrefix(rel, "..") {
		return absPath
	}
	return filepath.ToSlash(rel)
}
