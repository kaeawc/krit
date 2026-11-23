// Command corpusbench runs krit against the pinned public corpus in
// tests/corpus/repos.json and records, per repository: whether the scan
// completed, wall time, peak RSS, files with tree-sitter syntax errors,
// krit warnings, build-system / module detection, and findings by rule.
//
//	go build -o krit ./cmd/krit/
//	go run ./internal/devtools/corpusbench -fetch -out corpus-out
//	go run ./internal/devtools/corpusbench -out corpus-new -compare corpus-out/results.json
//
// Clones live outside the repository (default $KRIT_CORPUS_ROOT, else
// ~/.krit/corpus) and are pinned to the manifest commit.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

type manifest struct {
	Repos []repo `json:"repos"`
}

type repo struct {
	Name   string   `json:"name"`
	URL    string   `json:"url"`
	Commit string   `json:"commit"`
	Build  string   `json:"build"`
	Shape  []string `json:"shape"`
}

// Result is one repository's measurements. Field names are part of the
// results.json format that -compare reads back.
type Result struct {
	Name     string `json:"name"`
	Commit   string `json:"commit"`
	Build    string `json:"build"`
	Status   string `json:"status"` // ok, crash, timeout, bad-output, missing
	ExitCode int    `json:"exitCode"`
	Error    string `json:"error,omitempty"`

	WallMs     int64   `json:"wallMs"`
	MaxRSSMB   float64 `json:"maxRssMB"`
	KritFiles  int     `json:"kritFiles"`
	KotlinLOC  int     `json:"kotlinLoc"`
	KtFiles    int     `json:"ktFiles"`
	JavaFiles  int     `json:"javaFiles"`
	GradleKts  int     `json:"gradleKtsFiles"`
	GradleGrv  int     `json:"gradleGroovyFiles"`
	BazelFiles int     `json:"bazelFiles"`

	SyntaxErrorFiles   int      `json:"syntaxErrorFiles"`
	SyntaxErrorSamples []string `json:"syntaxErrorSamples,omitempty"`
	Panics             int      `json:"panics"`
	Warnings           []string `json:"warnings,omitempty"`

	DetectedBuild      string  `json:"detectedBuild"`
	Modules            int     `json:"modules"`
	ModulesWithSources int     `json:"modulesWithSources"`
	KtFilesInModules   int     `json:"ktFilesInModules"`
	ModuleCoverage     float64 `json:"moduleCoverage"`
	AndroidManifests   int     `json:"androidManifests"`
	CatalogSources     int     `json:"catalogSources"`
	Dependencies       int     `json:"dependencies"`
	DepExtraction      bool    `json:"dependencyExtractionComplete"`

	Findings      int            `json:"findings"`
	FindingsByRul map[string]int `json:"findingsByRule"`
}

type report struct {
	KritVersion string    `json:"kritVersion"`
	KritCommit  string    `json:"kritCommit"`
	Args        []string  `json:"args"`
	StartedAt   time.Time `json:"startedAt"`
	Results     []Result  `json:"results"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "corpusbench:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fl := flag.NewFlagSet("corpusbench", flag.ContinueOnError)
	manifestPath := fl.String("manifest", "tests/corpus/repos.json", "corpus manifest")
	root := fl.String("root", defaultRoot(), "directory holding the corpus clones")
	kritBin := fl.String("krit", "./krit", "krit binary to run")
	fetch := fl.Bool("fetch", false, "clone missing repos (shallow, pinned commit)")
	only := fl.String("only", "", "comma-separated repo names to run")
	out := fl.String("out", "corpus-out", "output directory for results.json and summary.md")
	compare := fl.String("compare", "", "previous results.json to diff against")
	timeout := fl.Duration("timeout", 10*time.Minute, "per-repo krit timeout")
	extra := fl.String("krit-args", "", "extra space-separated krit flags")
	skipSyntax := fl.Bool("skip-syntax", false, "skip the per-file tree-sitter syntax-error pass")
	if err := fl.Parse(args); err != nil {
		return err
	}

	var m manifest
	data, err := os.ReadFile(*manifestPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("%s: %w", *manifestPath, err)
	}
	selected := map[string]bool{}
	for _, n := range strings.Split(*only, ",") {
		if n = strings.TrimSpace(n); n != "" {
			selected[n] = true
		}
	}

	kritArgs := []string{"-no-cache", "-no-daemon", "-no-type-oracle", "-no-fir", "-f", "json"}
	if *extra != "" {
		kritArgs = append(kritArgs, strings.Fields(*extra)...)
	}
	rep := report{
		KritVersion: kritVersion(*kritBin),
		KritCommit:  gitHead("."),
		Args:        kritArgs,
		StartedAt:   time.Now().UTC(),
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	for _, r := range m.Repos {
		if len(selected) > 0 && !selected[r.Name] {
			continue
		}
		dir := filepath.Join(*root, r.Name)
		if *fetch {
			if err := ensureClone(dir, r); err != nil {
				return fmt.Errorf("%s: %w", r.Name, err)
			}
		}
		res := measure(dir, r, *kritBin, kritArgs, *timeout, *out, !*skipSyntax)
		fmt.Fprintf(stdout, "%-24s %-10s %7dms %7.0fMB files=%-6d findings=%-6d syntaxErr=%-4d modules=%d/%d cov=%.0f%%\n",
			res.Name, res.Status, res.WallMs, res.MaxRSSMB, res.KritFiles, res.Findings,
			res.SyntaxErrorFiles, res.ModulesWithSources, res.Modules, res.ModuleCoverage*100)
		rep.Results = append(rep.Results, res)
	}

	enc, _ := json.MarshalIndent(rep, "", "  ")
	if err := os.WriteFile(filepath.Join(*out, "results.json"), append(enc, '\n'), 0o644); err != nil {
		return err
	}
	var prev *report
	if *compare != "" {
		prev = &report{}
		data, err := os.ReadFile(*compare)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, prev); err != nil {
			return fmt.Errorf("%s: %w", *compare, err)
		}
	}
	summary := renderSummary(rep, prev)
	return os.WriteFile(filepath.Join(*out, "summary.md"), []byte(summary), 0o644)
}

func defaultRoot() string {
	if v := os.Getenv("KRIT_CORPUS_ROOT"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".krit", "corpus")
}

func ensureClone(dir string, r repo) error {
	if head := gitHead(dir); head == r.Commit {
		return nil
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if err := gitRun("", "init", "-q", dir); err != nil {
			return err
		}
	}
	if err := gitRun(dir, "fetch", "-q", "--depth", "1", r.URL, r.Commit); err != nil {
		return err
	}
	return gitRun(dir, "-c", "advice.detachedHead=false", "checkout", "-q", "--force", "FETCH_HEAD")
}

func gitRun(dir string, args ...string) error {
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	// LFS payloads (screenshots, binaries) are never analyzed, and a
	// missing LFS endpoint must not fail the checkout.
	cmd.Env = append(os.Environ(), "GIT_LFS_SKIP_SMUDGE=1")
	return cmd.Run()
}

func gitHead(dir string) string {
	out, err := exec.CommandContext(context.Background(), "git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func kritVersion(bin string) string {
	out, err := exec.CommandContext(context.Background(), bin, "-version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

var (
	panicRe      = regexp.MustCompile(`(?i)\bpanic\b|goroutine \d+ \[`)
	warningRe    = regexp.MustCompile(`^(warning|warn|krit|error)\b`)
	digitsRe     = regexp.MustCompile(`\d+`)
	maxWarnLines = 20
)

func measure(dir string, r repo, kritBin string, kritArgs []string, timeout time.Duration, outDir string, syntax bool) Result {
	res := Result{Name: r.Name, Commit: r.Commit, Build: r.Build, FindingsByRul: map[string]int{}}
	if head := gitHead(dir); head == "" {
		res.Status = "missing"
		res.Error = "clone not found; run with -fetch"
		return res
	} else if head != r.Commit {
		res.Error = "checkout is at " + head + ", manifest pins " + r.Commit
	}
	inventory(dir, &res, syntax)
	projectModel(dir, &res)

	// Cold run: drop krit's on-disk caches so timing and findings never
	// depend on a previous invocation.
	_ = os.RemoveAll(filepath.Join(dir, ".krit"))
	jsonOut := filepath.Join(outDir, r.Name+".json")
	stderrPath := filepath.Join(outDir, r.Name+".stderr")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	args := append(append([]string{}, kritArgs...), "-o", jsonOut, dir)
	cmd := exec.CommandContext(ctx, kritBin, args...)
	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		res.Status, res.Error = "crash", err.Error()
		return res
	}
	cmd.Stderr = stderrFile
	start := time.Now()
	runErr := cmd.Run()
	res.WallMs = time.Since(start).Milliseconds()
	stderrFile.Close()
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
		if ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
			res.MaxRSSMB = maxRSSMB(ru)
		}
	}
	_ = os.RemoveAll(filepath.Join(dir, ".krit"))

	stderr, _ := os.ReadFile(stderrPath)
	scanStderr(string(stderr), &res)
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		res.Status = "timeout"
		return res
	case runErr != nil && res.ExitCode != 1:
		// Exit 1 means "findings present"; anything else is a failure.
		res.Status = "crash"
		res.Error = strings.TrimSpace(lastLines(string(stderr), 5))
		return res
	}
	var parsed struct {
		Files    int `json:"files"`
		Findings []struct {
			Rule string `json:"rule"`
		} `json:"findings"`
	}
	data, err := os.ReadFile(jsonOut)
	if err == nil {
		err = json.Unmarshal(data, &parsed)
	}
	if err != nil {
		res.Status, res.Error = "bad-output", err.Error()
		return res
	}
	res.Status = "ok"
	if res.Panics > 0 {
		res.Status = "crash"
	}
	res.KritFiles = parsed.Files
	res.Findings = len(parsed.Findings)
	for _, f := range parsed.Findings {
		res.FindingsByRul[f.Rule]++
	}
	return res
}

func scanStderr(stderr string, res *Result) {
	seen := map[string]bool{}
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if panicRe.MatchString(line) {
			res.Panics++
		}
		if !warningRe.MatchString(line) {
			continue
		}
		key := digitsRe.ReplaceAllString(line, "#")
		if seen[key] || len(res.Warnings) >= maxWarnLines {
			continue
		}
		seen[key] = true
		res.Warnings = append(res.Warnings, truncate(line, 240))
	}
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

var skipDirs = map[string]bool{".git": true, ".krit": true, "build": true, ".gradle": true, "node_modules": true, ".idea": true}

// inventory counts sources and, when syntax is set, parses every Kotlin
// file with the same tree-sitter grammar krit uses and records files
// whose tree contains ERROR or MISSING recovery nodes.
func inventory(dir string, res *Result, syntax bool) {
	var ktFiles []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // best-effort inventory
		}
		if d.IsDir() {
			if skipDirs[d.Name()] && path != dir {
				return filepath.SkipDir
			}
			return nil
		}
		if countSource(d.Name(), res) {
			ktFiles = append(ktFiles, path)
		}
		return nil
	})
	res.KtFiles = len(ktFiles)
	for _, path := range ktFiles {
		syntaxCheck(dir, path, res, syntax)
	}
}

// countSource tallies build and source files by kind and reports whether
// name is a Kotlin source file.
func countSource(name string, res *Result) bool {
	switch {
	case strings.HasSuffix(name, ".gradle.kts"):
		res.GradleKts++
	case strings.HasSuffix(name, ".gradle"):
		res.GradleGrv++
	case strings.HasSuffix(name, ".kt"):
		return true
	case strings.HasSuffix(name, ".java"):
		res.JavaFiles++
	case name == "BUILD" || name == "BUILD.bazel" || name == "WORKSPACE" || name == "WORKSPACE.bazel" || name == "MODULE.bazel":
		res.BazelFiles++
	}
	return false
}

func syntaxCheck(dir, path string, res *Result, syntax bool) {
	var file *scanner.File
	var err error
	if syntax {
		file, err = scanner.ParseFile(context.Background(), path)
	}
	if !syntax || err != nil || file == nil {
		if data, rerr := os.ReadFile(path); rerr == nil {
			res.KotlinLOC += strings.Count(string(data), "\n")
		}
		return
	}
	res.KotlinLOC += len(file.Lines)
	if line, ok := firstErrorLine(file.FlatTree); ok {
		res.SyntaxErrorFiles++
		if len(res.SyntaxErrorSamples) < 25 {
			rel, _ := filepath.Rel(dir, path)
			res.SyntaxErrorSamples = append(res.SyntaxErrorSamples, fmt.Sprintf("%s:%d", rel, line))
		}
	}
}

func firstErrorLine(t *scanner.FlatTree) (int, bool) {
	if t == nil {
		return 0, false
	}
	for i := 0; i < t.Len(); i++ {
		if t.Node(uint32(i)).IsErrorNode() {
			return int(t.StartRows[i]) + 1, true
		}
	}
	return 0, false
}

// projectModel reports what krit's own project discovery sees: the
// Gradle module graph, Android manifests, and the library profile.
func projectModel(dir string, res *Result) {
	res.DetectedBuild = detectBuild(dir)
	graph, err := module.DiscoverModules(context.Background(), dir)
	if err == nil && graph != nil {
		res.Modules = len(graph.Modules)
		var roots []string
		for _, m := range graph.Modules {
			if len(m.SourceRoots) > 0 {
				res.ModulesWithSources++
			}
			roots = append(roots, m.Dir+string(filepath.Separator))
		}
		sort.Strings(roots)
		res.KtFilesInModules = countFilesUnder(dir, roots)
	}
	if res.KtFiles > 0 {
		res.ModuleCoverage = float64(res.KtFilesInModules) / float64(res.KtFiles)
	}
	proj := android.DetectProject([]string{dir})
	if proj != nil {
		res.AndroidManifests = len(proj.ManifestPaths)
		profile := librarymodel.ProfileFromGradlePaths(proj.GradlePaths)
		res.CatalogSources = len(profile.CatalogSources)
		res.Dependencies = len(profile.Dependencies)
		res.DepExtraction = profile.DependencyExtractionComplete
	}
}

func detectBuild(dir string) string {
	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(dir, name))
		return err == nil
	}
	var kinds []string
	switch {
	case exists("settings.gradle.kts"):
		kinds = append(kinds, "gradle-kts")
	case exists("settings.gradle"):
		kinds = append(kinds, "gradle-groovy")
	}
	if exists("MODULE.bazel") || exists("WORKSPACE") || exists("WORKSPACE.bazel") {
		kinds = append(kinds, "bazel")
	}
	if len(kinds) == 0 {
		return "none"
	}
	return strings.Join(kinds, "+")
}

func countFilesUnder(dir string, roots []string) int {
	n := 0
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // best-effort inventory
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".kt") {
			return nil
		}
		for _, r := range roots {
			if strings.HasPrefix(path, r) {
				n++
				break
			}
		}
		return nil
	})
	return n
}
