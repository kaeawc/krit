package parity_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestWarmCleanMutationStructural runs the warm-versus-clean matrix without
// any compiler backend: the real krit binary scans a project with its on-disk
// caches (one-shot CLI) or through a resident `krit serve` daemon, absolute
// and relative scan roots, and every mutation's warm result must equal a fresh
// process over a copy of the final tree with an empty cache. It always runs;
// it needs no helper jar.
//
// The CLI path compares the JSON findings. The daemon path asks for the
// post-pipeline columns, so it also compares byte ranges, confidence and fix
// payloads (the fields the divergence comparator in internal/daemon
// deliberately ignores).
func TestWarmCleanMutationStructural(t *testing.T) {
	wcRequireMatrix(t)
	bin := wcBuildKrit(t, repoRoot(t))
	started := time.Now()
	defer func() { t.Logf("structural matrix wall time: %s", time.Since(started).Round(time.Second)) }()

	cleans := map[string]wcSnapshot{}
	for _, path := range []string{"cli", "daemon"} {
		for _, spelling := range []string{"abs", "rel"} {
			for _, m := range wcMutations() {
				if m.oracleOnly || (m.raceAnalyze && path != "daemon") {
					continue
				}
				t.Run(fmt.Sprintf("%s/%s/%s", path, spelling, m.name), func(t *testing.T) {
					c := &wcStructuralCase{bin: bin, path: path, rel: spelling == "rel", m: m, cleans: cleans}
					c.run(t)
				})
			}
		}
	}
}

func wcBuildKrit(t *testing.T, root string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "krit")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/krit/")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build krit: %v\n%s", err, out)
	}
	return bin
}

type wcStructuralCase struct {
	bin    string
	path   string
	rel    bool
	m      wcMutation
	cleans map[string]wcSnapshot
}

// wcStructuralRun is one analysis: the normalized snapshot plus how the run
// used its caches.
type wcStructuralRun struct {
	snap wcSnapshot
	// cached/total is the CLI's per-file analysis cache use; bundleHit is the
	// daemon's whole-run findings cache.
	cached, total int
	bundleHit     bool
	// dirty is the number of watcher-dirty files the daemon drained for the
	// call.
	dirty int
}

func (c *wcStructuralCase) run(t *testing.T) {
	home := t.TempDir()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "warm")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	wcWriteProject(t, root, c.m)

	var analyze, analyzeAfterEdit func(root, label string) wcStructuralRun
	if c.path == "daemon" {
		d := wcStartDaemon(t, c.bin, root, home)
		analyze = func(r, label string) wcStructuralRun { return c.analyzeDaemon(t, d, r, label) }
		// The daemon learns about edits from its filesystem watcher, and the
		// protocol offers no way to ask whether an event has landed that does
		// not itself consume it (analyze-project drains the dirty set, and an
		// analyze that races the event leaves the daemon stale, see the
		// raced-annotation row). So wait out the delivery, as an editor save
		// followed by a human-speed command would. Create, Remove and Rename
		// events are reported at once and Write events after a 50 ms debounce.
		analyzeAfterEdit = func(r, label string) wcStructuralRun {
			if c.m.raceAnalyze {
				c.analyzeDaemon(t, d, r, "race")
			}
			time.Sleep(wcDaemonSettle())
			run := c.analyzeDaemon(t, d, r, label)
			if run.dirty == 0 {
				t.Fatalf("%s: the daemon reported no dirty files %s after the edit; its watcher missed it", label, wcDaemonSettle())
			}
			return run
		}
	} else {
		analyze = func(r, label string) wcStructuralRun { return c.analyzeCLI(t, r, home, label) }
		analyzeAfterEdit = analyze
	}

	// 1. Seed: a cold run that must populate the cache.
	seed := analyze(root, "seed")
	if seed.cached != 0 || seed.bundleHit {
		t.Fatalf("seed run is not cold: cached=%d/%d bundleHit=%v", seed.cached, seed.total, seed.bundleHit)
	}
	if len(seed.snap) == 0 {
		t.Fatalf("seed run produced no findings; the matrix would compare nothing")
	}
	if !wcHasCacheFiles(root) {
		t.Fatalf("seed run left no cache under %s/.krit; a warm run would prove nothing", root)
	}

	// 2. No-op control: must be served from the cache and change nothing.
	control := analyze(root, "control")
	if c.path == "daemon" && !control.bundleHit {
		t.Fatalf("no-op control did not hit the findings bundle")
	}
	if c.path == "cli" && (control.cached == 0 || control.cached != control.total) {
		t.Fatalf("no-op control did not hit the analysis cache: cached=%d/%d", control.cached, control.total)
	}
	if diff := wcDiff(control.snap, seed.snap); diff != "" {
		t.Fatalf("no-op control changed the result relative to the seed run:\n%s", diff)
	}

	// 3. Mutate, then run warm on the same cache/session.
	if c.m.apply != nil {
		c.m.apply(t, root)
	}
	warm := analyzeAfterEdit(root, "warm")

	// 4. A genuinely fresh process over a copy of the final tree with an
	//    empty cache. Rows reaching the same final tree share the baseline.
	identity := wcTreeIdentity(root)
	key := c.path + "|" + c.m.name + "|" + identity
	cleanSnap, ok := c.cleans[key]
	cleanRoot := filepath.Join(dir, "clean")
	if !ok {
		if err := os.MkdirAll(cleanRoot, 0o755); err != nil {
			t.Fatal(err)
		}
		wcCopyTree(t, root, cleanRoot)
		cleanHome := t.TempDir()
		var fresh wcStructuralRun
		if c.path == "daemon" {
			d := wcStartDaemon(t, c.bin, cleanRoot, cleanHome)
			fresh = c.analyzeDaemon(t, d, cleanRoot, "clean")
		} else {
			fresh = c.analyzeCLI(t, cleanRoot, cleanHome, "clean")
		}
		if fresh.cached != 0 || fresh.bundleHit {
			t.Fatalf("clean baseline was not cold: cached=%d bundleHit=%v", fresh.cached, fresh.bundleHit)
		}
		cleanSnap = fresh.snap
		c.cleans[key] = cleanSnap
	}

	t.Logf("cache use: seed=cached %d/%d control=cached %d/%d bundleHit=%v warm=cached %d/%d bundleHit=%v",
		seed.cached, seed.total, control.cached, control.total, control.bundleHit, warm.cached, warm.total, warm.bundleHit)

	diff := wcDiff(warm.snap, cleanSnap)
	problem := ""
	if diff != "" {
		problem = "the warm result differs from a clean scan"
	}
	wcResolveRow(t, fmt.Sprintf("structural/%s/%s/%s", c.path, c.spellingKey(), c.m.name), problem, fmt.Sprintf(
		"STALE WARM RESULT\n  mutation: %s\n  backend:  structural (%s path, %s scan root)\n  warm tree:  %s\n  clean tree: %s (fresh process, empty cache)\n  warm cache: cached=%d/%d bundleHit=%v\n  differing keys (warm vs clean):\n%s",
		c.m.name, c.path, c.spelling(), identity, identity, warm.cached, warm.total, warm.bundleHit, diff))
	changed := wcSnapshotsDiffer(cleanSnap, seed.snap)
	switch c.m.effect {
	case wcEffectChanges:
		if !changed && c.m.structuralOnly {
			t.Errorf("mutation %s had no effect on the clean result; the case no longer tests anything", c.m.name)
		} else if !changed {
			t.Logf("note: mutation %s leaves the structural findings unchanged (it only moves compiler facts)", c.m.name)
		}
	case wcEffectSame:
		if changed {
			t.Errorf("mutation %s changed the clean result but was expected to be a no-op:\n%s", c.m.name, wcDiff(cleanSnap, seed.snap))
		}
	}
}

func (c *wcStructuralCase) spellingKey() string {
	if c.rel {
		return "rel"
	}
	return "abs"
}

func (c *wcStructuralCase) spelling() string {
	if c.rel {
		return "relative"
	}
	return "absolute"
}

func wcHasCacheFiles(root string) bool {
	found := false
	_ = filepath.WalkDir(filepath.Join(root, ".krit"), func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			found = true
		}
		return nil
	})
	return found
}

// wcKritEnv is the environment for a krit subprocess: an isolated HOME, and no
// helper jar, so the structural path never reaches a JVM oracle.
func wcKritEnv(home string) []string {
	var env []string
	for _, kv := range os.Environ() {
		switch {
		case strings.HasPrefix(kv, "HOME="),
			strings.HasPrefix(kv, "KRIT_TYPES_JAR="),
			strings.HasPrefix(kv, "KRIT_FIR_JAR="),
			strings.HasPrefix(kv, "KRIT_DAEMON_CACHE="):
		default:
			env = append(env, kv)
		}
	}
	return append(env, "HOME="+home)
}

// analyzeCLI runs `krit` as a one-shot process and normalizes its JSON.
func (c *wcStructuralCase) analyzeCLI(t *testing.T, root, home, label string) wcStructuralRun {
	t.Helper()
	scan := root
	if c.rel {
		scan = "."
	}
	out := filepath.Join(t.TempDir(), label+".json")
	cacheDir := filepath.Join(root, ".krit", "cache")
	args := []string{"--no-daemon", "--no-type-oracle", "--no-fir", "--all-rules", "-q", "-f", "json", "-o", out, "--cache-dir", cacheDir, scan}
	cmd := exec.Command(c.bin, args...)
	cmd.Dir = root
	cmd.Env = wcKritEnv(home)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if err != nil && (!errors.As(err, &exit) || exit.ExitCode() > 1) {
		t.Fatalf("%s: krit %v: %v\n%s", label, args, err, stderr.String())
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("%s: %v", label, err)
	}
	var doc struct {
		Findings []struct {
			File       string  `json:"file"`
			Line       int     `json:"line"`
			Column     int     `json:"column"`
			Rule       string  `json:"rule"`
			Severity   string  `json:"severity"`
			Message    string  `json:"message"`
			Fixable    bool    `json:"fixable"`
			Confidence float64 `json:"confidence"`
		} `json:"findings"`
		Cache struct {
			Cached int `json:"cached"`
			Total  int `json:"total"`
		} `json:"cache"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("%s: parse: %v", label, err)
	}
	run := wcStructuralRun{snap: wcSnapshot{}, cached: doc.Cache.Cached, total: doc.Cache.Total}
	for _, f := range doc.Findings {
		wcAddFinding(run.snap, wcNormPath(root, f.File), f.Line, f.Column, f.Rule,
			fmt.Sprintf("sev=%s conf=%.2f fixable=%v msg=%s", f.Severity, f.Confidence, f.Fixable, f.Message))
	}
	return run
}

// wcAddFinding adds a finding under a stable key; a repeated position+rule
// gets a numeric suffix so a duplicate row still shows up in the diff.
func wcAddFinding(snap wcSnapshot, file string, line, col int, rule, detail string) {
	key := fmt.Sprintf("finding|%s:%d:%d|%s", file, line, col, rule)
	for n := 2; ; n++ {
		if _, taken := snap[key]; !taken {
			break
		}
		key = fmt.Sprintf("finding|%s:%d:%d|%s#%d", file, line, col, rule, n)
	}
	snap[key] = detail
}

// wcDaemonProc is a running `krit serve` process.
type wcDaemonProc struct {
	socket string
	cmd    *exec.Cmd
}

func wcStartDaemon(t *testing.T, bin, root, home string) *wcDaemonProc {
	t.Helper()
	// Unix socket paths are short; keep the socket out of the long temp dirs.
	sockDir, err := os.MkdirTemp("", "wcd")
	if err != nil {
		t.Fatal(err)
	}
	d := &wcDaemonProc{socket: filepath.Join(sockDir, "s.sock")}
	var stderr bytes.Buffer
	d.cmd = exec.Command(bin, "serve", "--root", root, "--socket", d.socket, "--watch-backend", "fsnotify")
	d.cmd.Dir = root
	d.cmd.Env = wcKritEnv(home)
	d.cmd.Stderr = &stderr
	if err := d.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = daemon.Call(d.socket, daemon.VerbShutdown, nil, nil)
		done := make(chan struct{})
		go func() { _ = d.cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = d.cmd.Process.Kill()
			<-done
		}
		_ = os.RemoveAll(sockDir)
	})
	deadline := time.Now().Add(60 * time.Second)
	for {
		if err := daemon.Call(d.socket, daemon.VerbStatus, nil, nil); err == nil {
			return d
		}
		if time.Now().After(deadline) {
			t.Fatalf("krit serve did not come up:\n%s", stderr.String())
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// analyzeDaemon asks the resident daemon for the project's findings columns.
func (c *wcStructuralCase) analyzeDaemon(t *testing.T, d *wcDaemonProc, root, label string) wcStructuralRun {
	t.Helper()
	scan := root
	if c.rel {
		scan = "."
	}
	args := daemon.AnalyzeProjectArgs{
		Paths: []string{scan}, Cwd: root, Format: "json", AllRules: true,
		NoFir: true, IncludeColumns: true,
	}
	var res daemon.AnalyzeProjectResult
	if err := daemon.Call(d.socket, daemon.VerbAnalyzeProject, args, &res); err != nil {
		t.Fatalf("%s: analyze-project: %v", label, err)
	}
	var cols scanner.FindingColumns
	if len(res.Columns) > 0 {
		if err := json.Unmarshal(res.Columns, &cols); err != nil {
			t.Fatalf("%s: columns: %v", label, err)
		}
	}
	run := wcStructuralRun{snap: wcSnapshot{}, bundleHit: res.Stats.FindingsBundleHit, dirty: res.Stats.DirtyFiles}
	for i := 0; i < cols.Len(); i++ {
		detail := fmt.Sprintf("bytes=%d-%d sev=%s conf=%.2f msg=%s", cols.StartByteAt(i), cols.EndByteAt(i),
			cols.SeverityAt(i), cols.ConfidenceAt(i), cols.MessageAt(i))
		if cols.HasFix(i) {
			fix := cols.FixAt(i)
			detail += fmt.Sprintf(" fix={target=%q lines=%d-%d bytes=%d-%d byteMode=%v safety=%d repl=%q}",
				wcNormPath(root, fix.TargetFile), fix.StartLine, fix.EndLine, fix.StartByte, fix.EndByte, fix.ByteMode, fix.Safety, fix.Replacement)
		}
		if cols.HasSuggestedFixes(i) {
			var sugg []string
			for _, s := range cols.SuggestedFixesAt(i) {
				sugg = append(sugg, fmt.Sprintf("%+v", s))
			}
			sort.Strings(sugg)
			detail += " suggested=" + strings.Join(sugg, ";")
		}
		wcAddFinding(run.snap, wcNormPath(root, cols.FileAt(i)), cols.LineAt(i), cols.ColumnAt(i), cols.RuleAt(i), detail)
	}
	return run
}

// wcDaemonSettle is how long the matrix waits after an edit for the daemon's
// filesystem watcher to deliver it. KRIT_TEST_DAEMON_SETTLE overrides it (a
// duration) on a slow runner.
func wcDaemonSettle() time.Duration {
	if d, err := time.ParseDuration(os.Getenv("KRIT_TEST_DAEMON_SETTLE")); err == nil {
		return d
	}
	return 750 * time.Millisecond
}
