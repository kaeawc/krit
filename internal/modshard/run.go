package modshard

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/jvmaot"
	"github.com/kaeawc/krit/internal/oracle"
)

// Input describes one sharded oracle run.
type Input struct {
	ScanPaths []string
	// JarPath is the krit-fir jar. JavaPath is the JVM to run it with.
	JarPath, JavaPath string
	// Workers is the number of worker JVMs; 0 means half the CPUs, between 2 and 8.
	Workers int
	// CacheDir holds one facts file per unit, keyed by everything the facts depend on.
	CacheDir string
	// OutputPath is where the merged oracle JSON is written.
	OutputPath string
	// Required lists the Kotlin files the caller expects facts for. When more than MaxUncovered of
	// them have none, the run does not apply.
	Required []string
	// MaxUncovered is the tolerated fraction of Required files without facts (default 0.005).
	MaxUncovered float64
	// CheckRequest, when set, is the FIR check request (the rider's JSON) for the whole scan. Each
	// module runs the checkers over its own requested files in the same compile, and Result.Check
	// holds the merged response.
	CheckRequest []byte
	// Isolate runs only when it separates source sets of one project that the whole-repository compile
	// merges (redeclarations between them), and only when every unit has its full dependency context.
	// Otherwise the run does not apply and the caller keeps the whole-repository compile.
	Isolate bool
	// Logf receives verbose progress lines; may be nil.
	Logf func(format string, args ...any)
}

// Result reports what a run did.
type Result struct {
	Applied            bool
	Reason             string // why it did not apply
	Units, CachedUnits int
	AnalyzedUnits      int
	FailedUnits        int
	MissingJars        int
	Covered, Uncovered int
	Workers            int
	// Check is the merged FIR check response, when Input.CheckRequest was set and the run applied.
	Check []byte
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]`)

func factsFileName(id string) string { return unsafeName.ReplaceAllString(id, "_") + ".json" }

// Run produces the oracle JSON at in.OutputPath from per-module facts, analyzing only the modules
// whose cached facts are stale. It returns Applied=false (and no error) when the repository cannot
// be analyzed this way, so the caller can use the whole-repository compile.
func Run(ctx context.Context, in Input) (Result, error) {
	logf := in.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	var res Result
	if len(in.ScanPaths) == 0 || in.JarPath == "" || in.JavaPath == "" || in.CacheDir == "" || in.OutputPath == "" {
		res.Reason = "incomplete input"
		return res, nil
	}
	model, repoRoot, reason := loadModel(in.ScanPaths[0])
	if model == nil {
		res.Reason = reason
		return res, nil
	}
	stamp := time.Now()
	lap := func(what string) {
		logf("verbose: module oracle: %s took %s\n", what, time.Since(stamp).Round(time.Millisecond))
		stamp = time.Now()
	}
	units, missing := Plan(model, repoRoot, in.ScanPaths)
	lap("planning")
	res.Units, res.MissingJars = len(units), missing
	if len(units) == 0 {
		res.Reason = "no modules under the scan paths"
		return res, nil
	}
	if reason := isolationDeclined(in, units); reason != "" {
		res.Reason = reason
		return res, nil
	}
	if err := os.MkdirAll(in.CacheDir, 0o755); err != nil {
		return res, err
	}
	ident := newIdentities()
	check := parseCheckContext(in.CheckRequest)
	keys, stale, keyOf, requested := keyUnits(in, units, ident, check)
	res.CachedUnits = len(units) - len(stale)
	lap("cache keys")
	logf("verbose: module oracle: %d modules (%d cached, %d to analyze), %d dependency projects without a built jar\n", len(units), res.CachedUnits, len(stale), missing)
	if len(stale) > 0 {
		failed, workers, err := analyze(ctx, in, stale, keyOf)
		if err != nil {
			return res, err
		}
		res.AnalyzedUnits, res.FailedUnits, res.Workers = len(stale)-failed, failed, workers
	}
	lap("analysis")
	required := requiredFiles(in)
	covered, err := mergeCached(in, units, keys)
	if err != nil {
		return res, err
	}
	lap("merge")
	state := &residualState{units: units, keys: keys, requested: requested, covered: covered}
	if err := state.cover(ctx, in, model, required, ident, logf, &res); err != nil {
		return res, err
	}
	units, keys, requested, covered = state.units, state.keys, state.requested, state.covered
	res.Covered = len(covered)
	remaining := uncoveredFiles(required, covered)
	res.Uncovered = len(remaining)
	reportUncovered(logf, remaining)
	if !coverageAcceptable(in, len(required), res.Uncovered) {
		res.Reason = fmt.Sprintf("%d of %d files have no module facts", res.Uncovered, len(required))
		_ = os.Remove(in.OutputPath)
		return res, nil
	}
	res.Applied = true
	prune(in.CacheDir, keys, pruneAge)
	if check != nil {
		if res.Check, err = mergeChecks(in, units, keys, requested, check); err != nil {
			return res, err
		}
	}
	return res, nil
}

// isolationDeclined says why an Isolate run does not apply, or "" when it does (or is not Isolate).
func isolationDeclined(in Input, units []Unit) string {
	if !in.Isolate {
		return ""
	}
	if !NeedsIsolation(units) {
		return "no project has several scanned source sets"
	}
	if id := incompleteContext(units); id != "" {
		return "dependency sources of " + id + " exceed the context limit"
	}
	return ""
}

// residualState carries the run's units and coverage through the residual pass.
type residualState struct {
	units     []Unit
	keys      []string
	requested map[string][]string
	covered   map[string]bool
}

// cover analyzes residual source roots for required files no unit covered, and merges again.
func (st *residualState) cover(ctx context.Context, in Input, model *gradlemodel.Model, required map[string]bool, ident *identities, logf func(string, ...any), res *Result) error {
	missingFiles := uncoveredFiles(required, st.covered)
	if len(missingFiles) == 0 {
		return nil
	}
	units, keys, extra, extraRequested, err := addResidualUnits(ctx, in, model, st.units, st.keys, missingFiles, ident, logf, res)
	if err != nil {
		return err
	}
	st.units, st.keys = units, keys
	for id, files := range extraRequested {
		st.requested[id] = files
	}
	if len(extra) == 0 {
		return nil
	}
	st.covered, err = mergeCached(in, st.units, st.keys)
	return err
}

// coverageAcceptable reports whether few enough required files lack facts for the run to apply.
func coverageAcceptable(in Input, required, uncovered int) bool {
	maxUncovered := in.MaxUncovered
	if maxUncovered == 0 {
		maxUncovered = 0.005
	}
	return required == 0 || float64(uncovered) <= maxUncovered*float64(required)
}

// loadModel finds and loads the Gradle model for the scan root, and the repository root it sits in.
func loadModel(scanRoot string) (model *gradlemodel.Model, repoRoot, reason string) {
	dir := gradlemodel.Discover(scanRoot)
	if dir == "" {
		return nil, "", "no Gradle model"
	}
	model, _, err := gradlemodel.Load(dir)
	if err != nil || model == nil {
		return nil, "", "Gradle model not loadable"
	}
	return model, filepath.Dir(filepath.Dir(dir)), "" // <repo>/.krit/gradle-model
}

// keyUnits computes each unit's cache key and splits off the units whose facts are not cached.
func keyUnits(in Input, units []Unit, ident *identities, check *checkContext) (keys []string, stale []Unit, keyOf map[string]string, requested map[string][]string) {
	keys = make([]string, len(units))
	keyOf = make(map[string]string, len(units))
	requested = make(map[string][]string, len(units))
	for i, u := range units {
		var own []string
		keys[i], own = unitKey(u, in.JarPath, ident, check)
		keyOf[u.Spec.ID] = keys[i]
		requested[u.Spec.ID] = own
		if !cached(in.CacheDir, keys[i], len(own) > 0 && check != nil) {
			stale = append(stale, u)
		}
	}
	return keys, stale, keyOf, requested
}

// cached reports whether the unit's facts (and its check response, when it has one) are stored.
func cached(dir, key string, needsCheck bool) bool {
	if _, err := os.Stat(filepath.Join(dir, key+".json")); err != nil {
		return false
	}
	if needsCheck {
		if _, err := os.Stat(filepath.Join(dir, key+".check.json")); err != nil {
			return false
		}
	}
	return true
}

// requiredFiles is the caller's file list narrowed to absolute paths under the scan paths: the list
// also holds dependency sources compiled as context, which are not this run's to cover.
func requiredFiles(in Input) map[string]bool {
	roots := make([]string, 0, len(in.ScanPaths))
	for _, sp := range in.ScanPaths {
		if abs, err := filepath.Abs(sp); err == nil {
			roots = append(roots, abs)
		}
	}
	required := map[string]bool{}
	for _, f := range in.Required {
		abs, err := filepath.Abs(f)
		if err != nil {
			continue
		}
		for _, root := range roots {
			if rel, err := filepath.Rel(root, abs); err == nil && !strings.HasPrefix(rel, "..") {
				required[abs] = true
				break
			}
		}
	}
	return required
}

func uncoveredFiles(required, covered map[string]bool) []string {
	var out []string
	for f := range required {
		if !covered[f] {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// addResidualUnits analyzes the source roots no exported source set owns, for the uncovered files.
func addResidualUnits(ctx context.Context, in Input, model *gradlemodel.Model, units []Unit, keys, uncovered []string, ident *identities, logf func(string, ...any), res *Result) ([]Unit, []string, []Unit, map[string][]string, error) {
	residual := ResidualUnits(model, units, uncovered)
	if len(residual) == 0 {
		return units, keys, nil, nil, nil
	}
	rkeys, rstale, rkeyOf, rrequested := keyUnits(in, residual, ident, parseCheckContext(in.CheckRequest))
	logf("verbose: module oracle: %d residual source roots for %d uncovered files (%d to analyze)\n", len(residual), len(uncovered), len(rstale))
	if len(rstale) > 0 {
		failed, _, err := analyze(ctx, in, rstale, rkeyOf)
		if err != nil {
			return units, keys, nil, nil, err
		}
		res.FailedUnits += failed
	}
	return append(units, residual...), append(keys, rkeys...), residual, rrequested, nil
}

// reportUncovered logs a sample of the files without facts and where they are.
func reportUncovered(logf func(string, ...any), files []string) {
	if len(files) == 0 {
		return
	}
	byDir := map[string]int{}
	for _, f := range files {
		parts := strings.Split(filepath.ToSlash(f), "/")
		byDir[strings.Join(parts[max(0, len(parts)-6):max(0, len(parts)-1)], "/")]++
	}
	logf("verbose: module oracle: %d files without facts, e.g. %s\n", len(files), strings.Join(files[:min(5, len(files))], ", "))
	dirs := make([]string, 0, len(byDir))
	for d := range byDir {
		dirs = append(dirs, d)
	}
	sort.Slice(dirs, func(i, j int) bool { return byDir[dirs[i]] > byDir[dirs[j]] })
	for _, d := range dirs[:min(6, len(dirs))] {
		logf("verbose: module oracle:   %4d under .../%s\n", byDir[d], d)
	}
}

// analyze runs the stale units on worker JVMs. It returns how many units failed.
func analyze(ctx context.Context, in Input, stale []Unit, keyOf map[string]string) (failed, workers int, err error) {
	workers = in.Workers
	if workers <= 0 {
		workers = defaultWorkers()
	}
	workers = max(1, min(workers, len(stale)))
	tmp, err := os.MkdirTemp("", "krit-modshard-")
	if err != nil {
		return 0, 0, err
	}
	defer os.RemoveAll(tmp)
	factsDir := filepath.Join(tmp, "facts")
	if err := os.MkdirAll(factsDir, 0o755); err != nil {
		return 0, 0, err
	}
	checkPath := ""
	if len(in.CheckRequest) > 0 {
		checkPath = filepath.Join(tmp, "check-request.json")
		if err := os.WriteFile(checkPath, in.CheckRequest, 0o644); err != nil {
			return 0, 0, err
		}
	}
	jvm := chooseWorkerJVM(in)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for i, group := range Shards(stale, workers) {
		if len(group) == 0 {
			continue
		}
		request, err := writeRequest(tmp, i, group)
		if err != nil {
			// Running workers still use tmp; wait for them before the deferred cleanup removes it.
			wg.Wait()
			return 0, 0, err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := runWorker(ctx, in, jvm, i, workers, len(group), request, factsDir, checkPath); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if firstErr != nil && ctx.Err() != nil {
		return 0, workers, firstErr
	}
	if firstErr != nil && in.Logf != nil {
		in.Logf("verbose: module worker failed; its unfinished modules have no facts: %v\n", firstErr)
	}
	for _, u := range stale {
		src := filepath.Join(factsDir, factsFileName(u.Spec.ID))
		dst := filepath.Join(in.CacheDir, keyOf[u.Spec.ID]+".json")
		if err := os.Rename(src, dst); err != nil && copyFile(src, dst) != nil {
			failed++
			continue
		}
		// The check response is written first, so a facts file implies it is complete.
		checkSrc := filepath.Join(factsDir, strings.TrimSuffix(factsFileName(u.Spec.ID), ".json")+".check.json")
		if _, err := os.Stat(checkSrc); err == nil {
			checkDst := filepath.Join(in.CacheDir, keyOf[u.Spec.ID]+".check.json")
			if err := os.Rename(checkSrc, checkDst); err != nil {
				_ = copyFile(checkSrc, checkDst)
			}
		}
	}
	return failed, workers, nil
}

func writeRequest(dir string, shard int, group []Unit) (string, error) {
	specs := make([]firchecks.ModuleSpec, len(group))
	for j, u := range group {
		specs[j] = u.Spec
	}
	data, err := json.Marshal(firchecks.AnalyzeModulesRequest{ID: 1, Command: "analyzeModules", Modules: specs})
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "request."+strconv.Itoa(shard)+".json")
	return path, os.WriteFile(path, data, 0o644)
}

// runWorker runs one krit-fir JVM over a request. A JVM that exits abnormally is an error; modules
// it did not finish simply have no facts file.
func runWorker(ctx context.Context, in Input, jvm workerJVM, shard, workers, modules int, request, factsDir, checkPath string) error {
	args := []string{
		"-Xms256m", "-Xmx" + strconv.Itoa(workerHeapMB(workers)) + "m", "-XX:+UseG1GC", "-XX:ParallelGCThreads=2", "-XX:ConcGCThreads=1",
		"-XX:CICompilerCount=2", "-Djava.awt.headless=true",
	}
	// The recording worker (the first) also trains the AOT cache for later runs; the others use a ready cache.
	args = append(args, jvm.argsFor(shard)...)
	args = append(args, "-jar", in.JarPath, "--module-facts-file", request, "--facts-dir", factsDir)
	if checkPath != "" {
		args = append(args, "--check-request", checkPath)
	}
	cmd := exec.CommandContext(ctx, jvm.java, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	began := time.Now()
	runErr := cmd.Run()
	jvmaot.FinalizeRecording(cmd.Args, runErr == nil)
	if in.Logf != nil {
		var moduleMS int
		for _, line := range strings.Split(stderr.String(), "\n") {
			if f := strings.Split(line, "\t"); len(f) == 4 && f[0] == "FACTS" && f[2] == "ok" {
				ms, _ := strconv.Atoi(f[3])
				moduleMS += ms
			}
		}
		in.Logf("verbose: module worker %d: %d modules, wall %s, module time %dms\n", shard, modules, time.Since(began).Round(time.Millisecond), moduleMS)
		_ = os.WriteFile(filepath.Join(in.CacheDir, "worker-"+strconv.Itoa(shard)+".log"), stderr.Bytes(), 0o644)
	}
	if runErr != nil {
		return fmt.Errorf("module worker %d: %w: %s", shard, runErr, tail(stderr.String(), 400))
	}
	return nil
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	// Copy to a temp name so a failed copy never leaves a truncated file under a cache key.
	out, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".tmp.")
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(out.Name())
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(out.Name())
		return err
	}
	if err := os.Rename(out.Name(), dst); err != nil {
		os.Remove(out.Name())
		return err
	}
	return nil
}

// merge writes the oracle JSON from the units' fact files, in unit order; the first unit that has a
// file wins. It returns the set of files that have facts.
func merge(in Input, units []Unit, keys []string) (map[string]bool, error) {
	tmp := in.OutputPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return nil, err
	}
	w := bufio.NewWriterSize(f, 1<<20)
	covered := map[string]bool{}
	deps := map[string]json.RawMessage{}
	kotlinVersion := ""
	fmt.Fprint(w, `{"version":1,"kotlinVersion":"`)
	var body bytes.Buffer
	first := true
	for i := range units {
		data, err := os.ReadFile(filepath.Join(in.CacheDir, keys[i]+".json"))
		if err != nil {
			continue // a failed unit: its files stay uncovered
		}
		var facts struct {
			KotlinVersion string                     `json:"kotlinVersion"`
			Files         map[string]json.RawMessage `json:"files"`
			Dependencies  map[string]json.RawMessage `json:"dependencies"`
		}
		if err := json.Unmarshal(data, &facts); err != nil {
			continue
		}
		if kotlinVersion == "" {
			kotlinVersion = facts.KotlinVersion
		}
		paths := make([]string, 0, len(facts.Files))
		for p := range facts.Files {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			if covered[p] {
				continue
			}
			covered[p] = true
			if !first {
				_ = body.WriteByte(',')
			}
			first = false
			key, _ := json.Marshal(p)
			body.Write(key)
			_ = body.WriteByte(':')
			body.Write(facts.Files[p])
		}
		for k, v := range facts.Dependencies {
			deps[k] = v
		}
	}
	fmt.Fprint(w, strings.ReplaceAll(kotlinVersion, `"`, ""), `","files":{`)
	_, _ = w.Write(body.Bytes())
	fmt.Fprint(w, `},"dependencies":{`)
	depKeys := make([]string, 0, len(deps))
	for k := range deps {
		depKeys = append(depKeys, k)
	}
	sort.Strings(depKeys)
	for i, k := range depKeys {
		if i > 0 {
			_ = w.WriteByte(',')
		}
		key, _ := json.Marshal(k)
		_, _ = w.Write(key)
		_ = w.WriteByte(':')
		_, _ = w.Write(deps[k])
	}
	fmt.Fprint(w, `}}`)
	if err := w.Flush(); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(in.OutputPath), 0o755); err != nil {
		return nil, err
	}
	return covered, os.Rename(tmp, in.OutputPath)
}

// identities memoizes file identities (size and modification time) across many units.
type identities struct {
	mu   sync.Mutex
	seen map[string]string
}

func newIdentities() *identities { return &identities{seen: map[string]string{}} }

func (c *identities) of(path string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.seen[path]; ok {
		return v
	}
	v := path + "|missing"
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			v = dirIdentity(path)
		} else {
			v = fmt.Sprintf("%s|%d|%d", path, info.Size(), info.ModTime().UnixNano())
		}
	}
	c.seen[path] = v
	return v
}

// dirIdentity fingerprints a classpath directory by the relative path, size and modification time
// of every file under it, since a directory's own mtime does not change when a nested class does.
// Results are memoized per path by identities.of, so each directory is walked once per run.
func dirIdentity(root string) string {
	h := sha256.New()
	// Unreadable entries are skipped rather than aborting the walk.
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr == nil && !d.IsDir() {
			hashDirEntry(h, root, p, d)
		}
		return nil
	})
	return fmt.Sprintf("%s|dir|%x", root, h.Sum(nil)[:12])
}

// hashDirEntry writes the relative path, size and modification time of d to h, skipping it when its info is unreadable.
func hashDirEntry(h io.Writer, root, p string, d fs.DirEntry) {
	info, err := d.Info()
	if err != nil {
		return
	}
	rel, _ := filepath.Rel(root, p)
	fmt.Fprintf(h, "%s|%d|%d;", rel, info.Size(), info.ModTime().UnixNano())
}

// unitKey identifies a unit's facts: the krit-fir jar, the module's settings and plugins, its
// classpath (built dependency jars included), and the content of every source file.
func unitKey(u Unit, jarPath string, ident *identities, check *checkContext) (string, []string) {
	h := sha256.New()
	add := func(parts ...string) {
		for _, p := range parts {
			fmt.Fprintf(h, "%d:%s;", len(p), p)
		}
	}
	add("v1", ident.of(jarPath), u.Spec.ID, u.Spec.Kind, u.Spec.Platform, u.Spec.JVMTarget)
	add(u.Spec.CompilerArgs...)
	add("|plugins")
	for _, p := range u.Spec.CompilerPluginClasspath {
		add(ident.of(p))
	}
	add(u.Spec.CompilerPluginOptions...)
	add("|classpath")
	for _, p := range u.Spec.Classpath {
		add(ident.of(p))
	}
	add("|friends")
	for _, p := range u.Spec.Friends {
		add(ident.of(p))
	}
	add("|sources")
	files := sourceFiles(append(append([]string{}, u.Spec.SourceRoots...), u.Spec.GeneratedSourceRoots...))
	hashAll := func(paths []string) {
		for _, p := range paths {
			hash, err := oracle.ContentHash(p)
			if err != nil {
				hash = "unreadable"
			}
			add(p, hash)
		}
	}
	hashAll(files)
	if len(u.Spec.ContextSourceRoots) > 0 {
		add("|context")
		hashAll(sourceFiles(u.Spec.ContextSourceRoots))
	}
	own := check.requestedAmong(files)
	if check != nil {
		add("|check", check.contextFor(own))
	}
	return hex.EncodeToString(h.Sum(nil))[:40], own
}

// pruneAge is how long an unreferenced fact file is kept: long enough that switching between a few
// branches reuses their modules' facts, short enough that the cache does not grow without bound.
const pruneAge = 7 * 24 * time.Hour

// prune removes fact and check files whose key is not in liveKeys and that are older than maxAge.
func prune(dir string, liveKeys []string, maxAge time.Duration) {
	live := make(map[string]bool, len(liveKeys))
	for _, k := range liveKeys {
		live[k] = true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		name := e.Name()
		key := strings.TrimSuffix(strings.TrimSuffix(name, ".json"), ".check")
		if e.IsDir() || !strings.HasSuffix(name, ".json") || strings.HasPrefix(name, "worker-") || live[key] {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}

// defaultWorkers is half the CPUs, between 2 and 8. More JVMs than that spent more CPU warming up
// and contending than they saved in wall time.
func defaultWorkers() int { return max(2, min(runtime.NumCPU()/2, 8)) }

// sourceFiles lists the Kotlin, Java and script files under roots, sorted.
func sourceFiles(roots []string) []string {
	var files []string
	for _, root := range roots {
		_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && (strings.HasSuffix(p, ".kt") || strings.HasSuffix(p, ".java") || strings.HasSuffix(p, ".kts")) {
				files = append(files, p)
			}
			return nil
		})
	}
	sort.Strings(files)
	return files
}

// mergeCached is merge, skipped when the output already holds the merge of exactly these units: the
// merged oracle is hundreds of MB, and an unchanged repository would otherwise rewrite it every run.
// The marker records the output file's size and time, so an output replaced by anything else
// (for example the whole-repository compile) is never mistaken for ours.
func mergeCached(in Input, units []Unit, keys []string) (map[string]bool, error) {
	h := sha256.New()
	for _, k := range keys {
		fmt.Fprintf(h, "%d:%s;", len(k), k)
	}
	id := hex.EncodeToString(h.Sum(nil))[:32]
	markerPath, coveredPath := in.OutputPath+".merged", in.OutputPath+".covered"
	if covered, ok := readMergeMarker(markerPath, coveredPath, in.OutputPath, id); ok {
		return covered, nil
	}
	covered, err := merge(in, units, keys)
	if err != nil {
		return nil, err
	}
	writeMergeMarker(markerPath, coveredPath, in.OutputPath, id, covered)
	return covered, nil
}

type mergeMarker struct {
	ID    string `json:"id"`
	Size  int64  `json:"size"`
	MTime int64  `json:"mtime"`
}

func readMergeMarker(markerPath, coveredPath, output, id string) (map[string]bool, bool) {
	data, err := os.ReadFile(markerPath)
	if err != nil {
		return nil, false
	}
	var m mergeMarker
	info, statErr := os.Stat(output)
	if json.Unmarshal(data, &m) != nil || statErr != nil || m.ID != id || m.Size != info.Size() || m.MTime != info.ModTime().UnixNano() {
		return nil, false
	}
	list, err := os.ReadFile(coveredPath)
	if err != nil {
		return nil, false
	}
	covered := map[string]bool{}
	for _, line := range strings.Split(string(list), "\n") {
		if line != "" {
			covered[line] = true
		}
	}
	return covered, true
}

func writeMergeMarker(markerPath, coveredPath, output, id string, covered map[string]bool) {
	info, err := os.Stat(output)
	if err != nil {
		return
	}
	paths := make([]string, 0, len(covered))
	for p := range covered {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	if os.WriteFile(coveredPath, []byte(strings.Join(paths, "\n")), 0o644) != nil {
		return
	}
	data, _ := json.Marshal(mergeMarker{ID: id, Size: info.Size(), MTime: info.ModTime().UnixNano()})
	_ = os.WriteFile(markerPath, data, 0o644)
}
