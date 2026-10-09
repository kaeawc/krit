//go:build unix

// Command benchanalysis benchmarks krit analysis on a local corpus with an
// explicit backend mode and cache state for every run, and records enough
// metadata to reproduce and verify the result. See docs/benchmarking.md.
//
//	go run ./internal/devtools/benchanalysis -corpus ~/src/AutoMobile \
//	    -modes structural,fir-oracle,kaa-oracle -states cold,warm-disk -runs 5
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
)

// Mode selects what analysis a run does. Modes only compare meaningfully
// when they share a workload; see docs/benchmarking.md.
type Mode struct {
	Name    string
	Args    []string
	Backend string // oracle backend the run must report; "" when no oracle runs
	FIR     bool   // the FIR checker pass must run and succeed
}

var modes = map[string]Mode{
	"structural":   {Name: "structural", Args: []string{"--no-type-oracle", "--no-fir"}},
	"fir-oracle":   {Name: "fir-oracle", Args: []string{"--oracle-backend=fir", "--no-fir"}, Backend: "fir"},
	"kaa-oracle":   {Name: "kaa-oracle", Args: []string{"--oracle-backend=kaa", "--no-fir"}, Backend: "kaa"},
	"fir-checkers": {Name: "fir-checkers", Args: []string{"--oracle-backend=fir", "--fir"}, Backend: "fir", FIR: true},
}

// State is a cache/process state. Every state names exactly which layers it
// resets and which it keeps, instead of relying on --no-cache.
type State struct {
	Name     string
	Resets   []string // layers removed before each measured run
	Retains  []string // layers deliberately kept warm
	Daemon   bool     // route runs through the krit daemon (warm process)
	Fill     bool     // run once, unmeasured, to fill the retained layers
	FreshJVM bool     // give each run its own JVM helper registry
	Edit     string   // "", "body" or "abi"
}

var states = map[string]State{
	"cold": {
		Name:     "cold",
		Resets:   []string{"<corpus>/.krit (incremental, parse, oracle, FIR and bundle caches)", "krit daemon", "JVM helper daemons (fresh registry)"},
		FreshJVM: true,
	},
	"warm-disk": {
		Name:     "warm-disk",
		Resets:   []string{"krit daemon", "JVM helper daemons (fresh registry)"},
		Retains:  []string{"<corpus>/.krit on-disk caches"},
		Fill:     true,
		FreshJVM: true,
	},
	"warm-daemon": {
		Name:    "warm-daemon",
		Retains: []string{"<corpus>/.krit on-disk caches", "krit daemon", "JVM helper daemons"},
		Daemon:  true, Fill: true,
	},
	"body-edit": {
		Name:    "body-edit",
		Retains: []string{"<corpus>/.krit on-disk caches", "krit daemon", "JVM helper daemons"},
		Daemon:  true, Fill: true, Edit: "body",
	},
	"abi-edit": {
		Name:    "abi-edit",
		Retains: []string{"<corpus>/.krit on-disk caches", "krit daemon", "JVM helper daemons"},
		Daemon:  true, Fill: true, Edit: "abi",
	},
}

// Run is one krit invocation and everything measured about it.
type Run struct {
	Mode        string             `json:"mode"`
	State       string             `json:"state"`
	Index       int                `json:"index"`
	Measured    bool               `json:"measured"`
	Command     []string           `json:"command"`
	Env         []string           `json:"env"`
	ExitCode    int                `json:"exitCode"`
	WallMs      int64              `json:"wallMs"`
	UserCPUMs   int64              `json:"userCpuMs"`
	SysCPUMs    int64              `json:"sysCpuMs"`
	MaxRSSKB    int64              `json:"maxRssKb"`
	LoadAvg     string             `json:"loadAvg,omitempty"`
	DurationMs  int64              `json:"durationMs"`
	Phases      map[string]int64   `json:"phasesMs,omitempty"`
	Attributes  map[string]string  `json:"attributes,omitempty"`
	Caches      map[string]CacheIO `json:"caches,omitempty"`
	Findings    int                `json:"findings"`
	Checksum    string             `json:"findingsChecksum"`
	Rules       int                `json:"rules"`
	Warnings    []string           `json:"warnings,omitempty"`
	Rejected    string             `json:"rejected,omitempty"`
	Verified    *bool              `json:"verifiedAgainstClean,omitempty"`
	CleanSum    string             `json:"cleanChecksum,omitempty"`
	EditApplied string             `json:"editApplied,omitempty"`
}

// CacheIO is the hit/miss counters krit reports for one cache layer.
type CacheIO struct {
	Hits   int64 `json:"hits"`
	Misses int64 `json:"misses"`
}

type options struct {
	corpus, krit, out, editFile, extraArgs string
	modes, states                          []string
	runs                                   int
	isolate, verify                        bool
}

func main() {
	var o options
	var modeList, stateList string
	flag.StringVar(&o.corpus, "corpus", "", "local corpus checkout to analyze (required)")
	flag.StringVar(&o.krit, "krit", "./krit", "krit binary to benchmark")
	flag.StringVar(&o.out, "out", "bench-out", "directory for results.json, summary.md and the corpus copy")
	flag.StringVar(&modeList, "modes", "structural,fir-oracle", "comma-separated: structural, fir-oracle, kaa-oracle, fir-checkers")
	flag.StringVar(&stateList, "states", "cold,warm-disk", "comma-separated: cold, warm-disk, warm-daemon, body-edit, abi-edit")
	flag.IntVar(&o.runs, "runs", 5, "measured runs per mode and state")
	flag.StringVar(&o.editFile, "edit-file", "", "corpus-relative .kt file mutated by body-edit and abi-edit")
	flag.StringVar(&o.extraArgs, "extra-args", "", "extra krit arguments, space separated, added to every run")
	flag.BoolVar(&o.isolate, "isolate", true, "copy the corpus into -out and mutate only the copy")
	flag.BoolVar(&o.verify, "verify", true, "check warm and edit runs against a clean scan of the same corpus contents")
	flag.Parse()
	o.modes = splitList(modeList)
	o.states = splitList(stateList)
	if err := run(o); err != nil {
		fmt.Fprintln(os.Stderr, "benchanalysis:", err)
		os.Exit(1)
	}
}

func run(o options) error {
	if o.corpus == "" {
		return errors.New("-corpus is required")
	}
	for _, m := range o.modes {
		if _, ok := modes[m]; !ok {
			return fmt.Errorf("unknown mode %q", m)
		}
	}
	for _, s := range o.states {
		st, ok := states[s]
		if !ok {
			return fmt.Errorf("unknown state %q", s)
		}
		if st.Edit != "" && !strings.HasSuffix(o.editFile, ".kt") {
			return fmt.Errorf("state %s needs -edit-file naming a corpus .kt file", s)
		}
	}
	corpus, err := filepath.Abs(o.corpus)
	if err != nil {
		return err
	}
	kritBin, err := filepath.Abs(o.krit)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(o.out, 0o755); err != nil {
		return err
	}
	out, err := filepath.Abs(o.out)
	if err != nil {
		return err
	}
	meta := collectMetadata(corpus, kritBin, o)
	work := corpus
	if o.isolate {
		work = filepath.Join(out, "corpus")
		_ = os.RemoveAll(work)
		if err := copyTree(corpus, work); err != nil {
			return fmt.Errorf("copy corpus: %w", err)
		}
	}
	h := &harness{o: o, krit: kritBin, work: work, out: out}
	var results []Run
	for _, m := range o.modes {
		for _, s := range o.states {
			runs, err := h.runState(modes[m], states[s])
			results = append(results, runs...)
			if err != nil {
				return fmt.Errorf("%s/%s: %w", m, s, err)
			}
		}
	}
	report := map[string]any{"metadata": meta, "runs": results}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "results.json"), data, 0o644); err != nil {
		return err
	}
	summary := summarize(results, o)
	if err := os.WriteFile(filepath.Join(out, "summary.md"), []byte(summary), 0o644); err != nil {
		return err
	}
	fmt.Print(summary)
	return nil
}

type harness struct {
	o                     options
	krit, work, out       string
	registry              string
	original              []byte
	editPath              string
	editsApplied, cleanID int
}

func (h *harness) runState(m Mode, s State) ([]Run, error) {
	var runs []Run
	defer func() {
		h.stopDaemon()
		_ = os.RemoveAll(h.registry)
		h.registry = ""
	}()
	if s.Edit != "" {
		h.editPath = filepath.Join(h.work, h.o.editFile)
		data, err := os.ReadFile(h.editPath)
		if err != nil {
			return nil, err
		}
		h.original = data
		defer func() { _ = os.WriteFile(h.editPath, h.original, 0o644) }()
	}
	h.resetDisk()
	h.newRegistry()
	if s.Fill {
		fill := h.invoke(m, s, 0, false)
		runs = append(runs, fill)
		if fill.Rejected != "" {
			// Nothing warm to measure; report the state as skipped and go on.
			fmt.Fprintf(os.Stderr, "benchanalysis: %s/%s skipped, fill run rejected: %s\n", m.Name, s.Name, fill.Rejected)
			return runs, nil
		}
	}
	for i := 1; i <= h.o.runs; i++ {
		if s.Name == "cold" {
			h.stopDaemon()
			h.resetDisk()
		}
		if s.FreshJVM {
			h.newRegistry()
		}
		edit := ""
		if s.Edit != "" {
			var err error
			if edit, err = h.applyEdit(s.Edit); err != nil {
				return runs, err
			}
		}
		r := h.invoke(m, s, i, true)
		r.EditApplied = edit
		if r.Rejected == "" && h.o.verify && s.Name != "cold" {
			h.verify(m, &r)
		}
		runs = append(runs, r)
	}
	return runs, nil
}

// resetDisk removes every krit cache layer stored under the corpus.
func (h *harness) resetDisk() {
	_ = os.RemoveAll(filepath.Join(h.work, ".krit"))
}

// newRegistry points JVM helper daemons at an empty registry, so the next
// run cannot reuse a JVM started by an earlier one.
func (h *harness) newRegistry() {
	if h.registry != "" {
		_ = os.RemoveAll(h.registry)
	}
	dir, err := os.MkdirTemp(h.out, "registry-")
	if err == nil {
		h.registry = dir
	}
}

func (h *harness) stopDaemon() {
	cmd := exec.CommandContext(context.Background(), h.krit, "daemon", "stop")
	cmd.Dir = h.work
	cmd.Env = h.env(true)
	_ = cmd.Run()
}

// env scopes JVM helpers to the krit process that started them
// (KRIT_EPHEMERAL_DAEMONS), so fresh-process runs really get fresh JVMs and
// stopping the krit daemon stops its helpers; nothing outlives a state.
func (h *harness) env(daemon bool) []string {
	env := append(os.Environ(), "KRIT_DAEMON_REGISTRY_DIR="+h.registry, "KRIT_EPHEMERAL_DAEMONS=1")
	if !daemon {
		env = append(env, "KRIT_NO_DAEMON_AUTOSTART=1")
	}
	return env
}

// applyEdit appends the n-th edit. A body edit adds a comment line, which
// changes the file's content hash but not its structure (krit's delta
// planner treats it as a body-only edit). An ABI edit adds a public
// top-level function, which changes the file's public API.
func (h *harness) applyEdit(kind string) (string, error) {
	h.editsApplied++
	text := editText(kind, h.editsApplied)
	data := append(append([]byte{}, h.original...), []byte(text)...)
	return strings.TrimSpace(text), os.WriteFile(h.editPath, data, 0o644)
}

func editText(kind string, n int) string {
	if kind == "abi" {
		return fmt.Sprintf("\nfun kritBenchAbiEdit%d(): Int = %d\n", n, n)
	}
	return fmt.Sprintf("\n// krit-bench body edit %d\n", n)
}

func (h *harness) args(m Mode, daemon bool) []string {
	args := []string{"-f", "json", "-perf", "-q"}
	if !daemon {
		args = append(args, "--no-daemon")
	}
	args = append(args, m.Args...)
	args = append(args, strings.Fields(h.o.extraArgs)...)
	return append(args, ".")
}

func (h *harness) invoke(m Mode, s State, index int, measured bool) Run {
	return execRun(h.krit, h.work, h.args(m, s.Daemon), h.env(s.Daemon), m, Run{
		Mode: m.Name, State: s.Name, Index: index, Measured: measured,
	})
}

// verify scans a fresh copy of the current corpus contents with every cache
// layer and process cold, and compares findings with the measured run.
func (h *harness) verify(m Mode, r *Run) {
	h.cleanID++
	dir := filepath.Join(h.out, fmt.Sprintf("clean-%d", h.cleanID))
	defer func() { _ = os.RemoveAll(dir) }()
	if err := copyTree(h.work, dir); err != nil {
		r.Rejected = "verify copy: " + err.Error()
		return
	}
	registry, _ := os.MkdirTemp(h.out, "registry-clean-")
	defer func() { _ = os.RemoveAll(registry) }()
	env := append(os.Environ(), "KRIT_DAEMON_REGISTRY_DIR="+registry, "KRIT_NO_DAEMON_AUTOSTART=1", "KRIT_EPHEMERAL_DAEMONS=1")
	args := []string{"-f", "json", "-perf", "-q", "--no-daemon"}
	args = append(append(append(args, m.Args...), strings.Fields(h.o.extraArgs)...), ".")
	clean := execRun(h.krit, dir, args, env, m, Run{})
	ok := clean.Rejected == "" && clean.Checksum == r.Checksum
	r.Verified = &ok
	r.CleanSum = clean.Checksum
	if clean.Rejected != "" {
		r.Rejected = "clean baseline failed: " + clean.Rejected
	} else if !ok {
		r.Rejected = fmt.Sprintf("findings differ from a clean scan (%d vs %d findings)", r.Findings, clean.Findings)
	}
}

func execRun(bin, dir string, args, env []string, m Mode, r Run) Run {
	r.Command = append([]string{bin}, args...)
	r.Env = benchEnv(env)
	cmd := exec.CommandContext(context.Background(), bin, args...)
	cmd.Dir = dir
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	r.LoadAvg = loadAvg()
	start := time.Now()
	err := cmd.Run()
	r.WallMs = time.Since(start).Milliseconds()
	if cmd.ProcessState != nil {
		r.ExitCode = cmd.ProcessState.ExitCode()
		r.UserCPUMs = cmd.ProcessState.UserTime().Milliseconds()
		r.SysCPUMs = cmd.ProcessState.SystemTime().Milliseconds()
		if ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
			r.MaxRSSKB = maxRSSKB(ru.Maxrss)
		}
	} else if err != nil {
		r.Rejected = "start: " + err.Error()
		return r
	}
	r.Warnings = diagnosticLines(stderr.String())
	// krit exits 1 when it reports findings; anything else is a failure.
	if r.ExitCode != 0 && r.ExitCode != 1 {
		r.Rejected = fmt.Sprintf("exit code %d: %s", r.ExitCode, lastLine(stderr.String()))
		return r
	}
	if reason := parseOutput(stdout.Bytes(), dir, &r); reason != "" {
		r.Rejected = reason
		return r
	}
	r.Rejected = checkMode(m, r)
	return r
}

// parseOutput fills r from krit's JSON report and returns a rejection
// reason when the report is missing or malformed. (The report's "success"
// field only means "no findings", so it is not a failure signal.)
func parseOutput(data []byte, root string, r *Run) string {
	var report struct {
		DurationMs int64             `json:"durationMs"`
		Rules      json.RawMessage   `json:"rules"`
		Findings   []json.RawMessage `json:"findings"`
		Caches     []struct {
			Name  string  `json:"name"`
			Stats CacheIO `json:"stats"`
		} `json:"caches"`
		PerfTiming []timing `json:"perfTiming"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return "unparseable JSON report: " + err.Error()
	}
	r.DurationMs = report.DurationMs
	r.Findings = len(report.Findings)
	r.Checksum = findingsChecksum(report.Findings, root)
	r.Rules = countRules(report.Rules)
	r.Phases = map[string]int64{}
	r.Attributes = map[string]string{}
	flattenTimings(report.PerfTiming, "", r.Phases, r.Attributes)
	r.Caches = map[string]CacheIO{}
	for _, c := range report.Caches {
		r.Caches[c.Name] = c.Stats
	}
	return ""
}

// checkMode rejects a run whose report doesn't show the requested backend.
func checkMode(m Mode, r Run) string {
	got := attr(r.Attributes, "oracleBackend", "backend")
	switch {
	case m.Backend == "" && got != "":
		return "oracle ran (" + got + ") in a mode that disables it"
	case m.Backend != "" && got != m.Backend:
		if got == "" {
			return "no oracleBackend in the perf report: the " + m.Backend + " oracle did not run (missing jar or no Kotlin sources?)"
		}
		return "oracle backend " + got + ", want " + m.Backend
	}
	if m.FIR {
		if status := attr(r.Attributes, "firCheckOutcome", "status"); status != "ok" {
			return "FIR checker pass did not succeed (status " + fmt.Sprintf("%q", status) + ")"
		}
	}
	return ""
}

type timing struct {
	Name       string            `json:"name"`
	DurationMs int64             `json:"durationMs"`
	Attributes map[string]string `json:"attributes"`
	Children   []timing          `json:"children"`
}

// flattenTimings maps "parent/child" paths to durations, summing repeated
// names, and keeps each entry's attributes as "path.key".
func flattenTimings(entries []timing, prefix string, phases map[string]int64, attrs map[string]string) {
	for _, e := range entries {
		path := e.Name
		if prefix != "" {
			path = prefix + "/" + e.Name
		}
		phases[path] += e.DurationMs
		for k, v := range e.Attributes {
			attrs[path+"."+k] = v
		}
		flattenTimings(e.Children, path, phases, attrs)
	}
}

// attr finds an attribute recorded on an entry with the given name at any
// depth of the perf tree.
func attr(attrs map[string]string, entry, key string) string {
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if k == entry+"."+key || strings.HasSuffix(k, "/"+entry+"."+key) {
			return attrs[k]
		}
	}
	return ""
}

// findingsChecksum hashes findings independent of report order, of
// timing-dependent fields and of where the corpus copy lives (file paths are
// made relative to root, since clean baselines scan a different copy).
func findingsChecksum(findings []json.RawMessage, root string) string {
	lines := make([]string, 0, len(findings))
	for _, raw := range findings {
		var f map[string]any
		if json.Unmarshal(raw, &f) != nil {
			lines = append(lines, string(raw))
			continue
		}
		file, _ := f["file"].(string)
		if rel, err := filepath.Rel(root, file); err == nil && filepath.IsAbs(file) {
			file = rel
		}
		message, _ := f["message"].(string)
		message = strings.ReplaceAll(message, root+string(filepath.Separator), "")
		lines = append(lines, fmt.Sprintf("%v|%v|%v|%v|%v|%v", file, f["line"], f["column"], f["ruleSet"], f["rule"], message))
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:8])
}

func countRules(raw json.RawMessage) int {
	var list []any
	if json.Unmarshal(raw, &list) == nil {
		return len(list)
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) == nil {
		return len(m)
	}
	var n int
	_ = json.Unmarshal(raw, &n)
	return n
}

func diagnosticLines(stderr string) []string {
	var out []string
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "error:") || strings.HasPrefix(line, "warning:") {
			out = append(out, line)
		}
	}
	return out
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}

// benchEnv records only the krit-relevant environment of a run.
func benchEnv(env []string) []string {
	var out []string
	for _, kv := range env {
		if strings.HasPrefix(kv, "KRIT_") || strings.HasPrefix(kv, "JAVA_HOME=") {
			out = append(out, kv)
		}
	}
	sort.Strings(out)
	return out
}

func maxRSSKB(maxrss int64) int64 {
	if runtime.GOOS == "darwin" {
		return maxrss / 1024 // bytes on macOS, KiB on Linux
	}
	return maxrss
}

func loadAvg() string {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		out, err := exec.CommandContext(context.Background(), "sysctl", "-n", "vm.loadavg").Output()
		if err != nil {
			return ""
		}
		return strings.Trim(strings.TrimSpace(string(out)), "{} ")
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return ""
	}
	return strings.Join(fields[:3], " ")
}

// copyTree copies a corpus, skipping build outputs and krit's own caches.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			switch d.Name() {
			case ".krit", ".gradle", "build", ".idea":
				if rel != "." {
					return filepath.SkipDir
				}
			}
			return os.MkdirAll(target, 0o755)
		}
		if d.Type()&fs.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
