package oracle

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/perf"
)

// InvocationOptions carries optional diagnostics for oracle invocations.
// A nil or disabled tracker keeps the existing low-overhead behavior.
type InvocationOptions struct {
	Tracker      perf.Tracker
	CacheWriter  *CacheWriter
	CallFilter   *CallTargetFilterSummary
	ExtraJVMArgs []string
	// DisableDiagnostics skips krit-types collectDiagnostics(), which is
	// expensive on large projects and only needed by rules that consume
	// compiler diagnostic facts.
	DisableDiagnostics bool
	// DeclarationProfile narrows which fields krit-types populates per
	// class/member. Nil or a full profile preserves pre-profile extraction;
	// narrow profiles skip KAA traversal for unused sections.
	DeclarationProfile *DeclarationProfileSummary
	// ForcedMisses lists absolute .kt paths that the cache classifier
	// must treat as misses without consulting the per-file content
	// hash. Used by the warm-path freshness gate to route a known-stale
	// file (e.g. an ABI edit detected by the bundle manifest comparator)
	// through a partial JVM reanalyze instead of letting it ride on a
	// stale cached entry. Empty preserves the pre-#NNN behavior.
	ForcedMisses []string
	// Classpath is the user-configured JVM classpath (krit.yml
	// `oracle.classpath` + the `CLASSPATH` env var). Forwarded to
	// the daemon as `--classpath <joined>`. Both backends accept
	// this flag today — the krit-types daemon resolved it before
	// the FIR migration; the krit-fir one-shot CLI gains a
	// `--classpath` parser alongside this field landing. Empty
	// preserves the existing source-tree-discovery fallback.
	Classpath []string
	JvmTarget string
	// Kotlin is the project's effective Kotlin compiler settings (language and
	// API level, opt-ins, allowlisted flags), forwarded to either backend as
	// `--kotlin-arg` flags. The zero value keeps the embedded compiler's
	// defaults. It is part of the cache scope (NewStoreScopeWithSettings).
	Kotlin gradlemodel.KotlinSettings
	// Backend is the JVM backend being invoked. When set, cached entries
	// written by the other backend are misses (see Backend.CacheApproximation),
	// and a whole-compilation backend's entries are checked against the
	// current compilation (see Backend.ReturnsWholeCompilation).
	Backend Backend
	// CheckRider, for the krit-fir backend, runs a FIR check request on
	// the oracle's one-shot compilation (#739). Sharded and daemon miss
	// analysis never carry it.
	CheckRider *CheckRider
}

func (o InvocationOptions) tracker() perf.Tracker {
	if o.Tracker == nil {
		return perf.New(false)
	}
	return o.Tracker
}

func trackOracle(t perf.Tracker, name string, fn func() error) error {
	if t == nil {
		return fn()
	}
	return t.Track(name, fn)
}

func addOracleEntry(t perf.Tracker, name string, start time.Time, metrics map[string]int64, attrs map[string]string) {
	perf.AddEntryDetails(t, name, time.Since(start), metrics, attrs)
}

func addOracleInstant(t perf.Tracker, name string, metrics map[string]int64, attrs map[string]string) {
	perf.AddEntryDetails(t, name, 0, metrics, attrs)
}

func extraJVMArgsFromEnv() []string {
	raw := strings.TrimSpace(os.Getenv("KRIT_TYPES_EXTRA_JVM_ARGS"))
	if raw == "" {
		return nil
	}
	return strings.Fields(raw)
}

const defaultKritTypesParallelFiles = 4

// configuredKritTypesParallelFiles returns the in-JVM file worker count for
// one-shot oracle analysis; KRIT_TYPES_PARALLEL_FILES overrides the default.
func configuredKritTypesParallelFiles() int {
	raw := strings.TrimSpace(os.Getenv("KRIT_TYPES_PARALLEL_FILES"))
	if raw == "" {
		return defaultKritTypesParallelFiles
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 1 {
		return 0
	}
	return n
}

// experimentalParallelFilesArg returns the --experimental-parallel-files N
// args to append after -jar. Returns nil when disabled.
func experimentalParallelFilesArg() []string {
	n := configuredKritTypesParallelFiles()
	if n <= 1 {
		return nil
	}
	return []string{"--experimental-parallel-files", strconv.Itoa(n)}
}

func configuredExtraJVMArgs(opts InvocationOptions) []string {
	if len(opts.ExtraJVMArgs) > 0 {
		return append([]string(nil), opts.ExtraJVMArgs...)
	}
	return extraJVMArgsFromEnv()
}

func appendExtraJVMArgsBeforeJar(args []string, extra []string) []string {
	if len(extra) == 0 {
		return args
	}
	idx := len(args)
	for i, arg := range args {
		if arg == "-jar" {
			idx = i
			break
		}
	}
	out := make([]string, 0, len(args)+len(extra))
	out = append(out, args[:idx]...)
	out = append(out, extra...)
	out = append(out, args[idx:]...)
	return out
}

func recordKritTypesJVMArgs(t perf.Tracker, extra []string) {
	if t == nil || !t.IsEnabled() {
		return
	}
	addOracleInstant(t, "kritTypesJVMArgs", map[string]int64{
		"extraArgs": int64(len(extra)),
	}, map[string]string{
		"args": strings.Join(extra, " "),
	})
}

func addKotlinTimingsFromFile(t perf.Tracker, path string) {
	if t == nil || !t.IsEnabled() || path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		addOracleInstant(t, "kotlinTimingsReadError", nil, map[string]string{"error": err.Error()})
		return
	}
	addKotlinTimings(t, data)
}

func addKotlinTimings(t perf.Tracker, data []byte) {
	if t == nil || !t.IsEnabled() || len(data) == 0 {
		return
	}
	var entries []perf.TimingEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		addOracleInstant(t, "kotlinTimingsParseError", nil, map[string]string{"error": err.Error()})
		return
	}
	if len(entries) == 0 {
		return
	}
	kt := t.Serial("kotlinTimings")
	perf.AddEntries(kt, entries)
	kt.End()
}

func tempTimingsPath() (string, func(), error) {
	f, err := os.CreateTemp("", "krit-types-timings-*.json")
	if err != nil {
		return "", func() {}, fmt.Errorf("tempfile (timings): %w", err)
	}
	name := f.Name()
	_ = f.Close()
	return name, func() { _ = os.Remove(name) }, nil
}

func callFilterFingerprint(opts InvocationOptions) string {
	if opts.CallFilter == nil || !opts.CallFilter.Enabled {
		return ""
	}
	return opts.CallFilter.Fingerprint
}

// declarationProfileFingerprint returns the cache scope for the profile.
// An empty string means "full profile — no narrowing", which writes
// unfingerprinted cache entries compatible with any later lookup.
func declarationProfileFingerprint(opts InvocationOptions) string {
	if opts.DeclarationProfile == nil {
		return ""
	}
	return opts.DeclarationProfile.Fingerprint
}

// diagnosticsOmittedScope marks cache entries written with compiler
// diagnostics disabled. It rides on the declaration-profile scope because
// omitting diagnostics is another narrowing of the emitted facts.
const diagnosticsOmittedScope = "no-diagnostics"

// factProfileScope is the cache scope for the facts an invocation emits:
// the declaration profile, narrowed further when diagnostics are disabled.
// Under the "empty = broad superset" convention, an entry written without
// diagnostics never satisfies a lookup that needs them, while a full entry
// still satisfies a diagnostics-off lookup.
func factProfileScope(opts InvocationOptions) string {
	scope := declarationProfileFingerprint(opts)
	if !opts.DisableDiagnostics {
		return scope
	}
	if scope == "" {
		return diagnosticsOmittedScope
	}
	return scope + "+" + diagnosticsOmittedScope
}

// declarationProfileCLIValue returns the comma-separated feature list to
// pass via --declaration-profile, or "" when the profile is full/absent
// so callers can omit the flag.
func declarationProfileCLIValue(opts InvocationOptions) string {
	if opts.DeclarationProfile == nil {
		return ""
	}
	return opts.DeclarationProfile.Profile.CLIValue()
}

func writeCallFilterArg(opts InvocationOptions, tracker perf.Tracker) (string, func(), error) {
	if opts.CallFilter == nil || !opts.CallFilter.Enabled {
		return "", func() {}, nil
	}
	var path string
	err := trackOracle(tracker, "oracleCallFilterWrite", func() error {
		var werr error
		path, werr = WriteCallTargetFilterFile(*opts.CallFilter, "")
		return werr
	})
	if err != nil {
		return "", func() {}, err
	}
	return path, func() { removeCallTargetFilterFile(path) }, nil
}
