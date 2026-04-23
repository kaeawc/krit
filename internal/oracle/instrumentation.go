package oracle

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/perf"
)

// InvocationOptions carries optional diagnostics for oracle invocations.
// A nil or disabled tracker keeps the existing low-overhead behavior.
type InvocationOptions struct {
	Tracker      perf.Tracker
	CacheWriter  *OracleCacheWriter
	CallFilter   *CallTargetFilterSummary
	ExtraJVMArgs []string
	// DeclarationProfile narrows which fields krit-types populates per
	// class/member. Nil or a full profile preserves pre-profile extraction;
	// narrow profiles skip KAA traversal for unused sections.
	DeclarationProfile *DeclarationProfileSummary
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

// activeProcessorCountForKritTypesShard returns the -XX:ActiveProcessorCount
// value for a single sharded KAA worker. Zero means no cap (shards <= 1).
//
// The formula divides available logical CPUs evenly across shards, then backs
// off by one to leave headroom for the Go runtime, OS scheduling, I/O, and GC.
// Backing off prevents the sharded JVMs from fully saturating all cores, which
// empirically reduced user CPU without increasing wall time on 16-core M3 Max.
func activeProcessorCountForKritTypesShard(shards int) int {
	if shards <= 1 {
		return 0
	}
	cpus := runtime.GOMAXPROCS(0)
	if cpus <= 0 {
		cpus = runtime.NumCPU()
	}
	perShard := (cpus + shards - 1) / shards
	if perShard > 1 {
		perShard--
	}
	if perShard < 1 {
		perShard = 1
	}
	return perShard
}

// jvmArgsForKritTypesShard returns the adaptive JVM flags for a sharded
// krit-types worker. Returns nil when shards <= 1 (no policy applies).
func jvmArgsForKritTypesShard(shards int) []string {
	active := activeProcessorCountForKritTypesShard(shards)
	if active <= 0 {
		return nil
	}
	return []string{fmt.Sprintf("-XX:ActiveProcessorCount=%d", active)}
}

// adaptiveShardJVMArgs merges the adaptive per-shard policy args with any
// caller- or env-supplied overrides. Policy args come first; overrides follow
// so they can supersede individual flags (e.g. a manual ActiveProcessorCount
// in KRIT_TYPES_EXTRA_JVM_ARGS shadows the computed one in the JVM last-wins
// sense — identical flags, last one wins for most JVM implementations).
func adaptiveShardJVMArgs(shards int, opts InvocationOptions) []string {
	adaptive := jvmArgsForKritTypesShard(shards)
	override := configuredExtraJVMArgs(opts)
	if len(adaptive) == 0 {
		return override
	}
	if len(override) == 0 {
		return adaptive
	}
	out := make([]string, 0, len(adaptive)+len(override))
	out = append(out, adaptive...)
	out = append(out, override...)
	return out
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
// one-shot krit-types analysis. The default is 4, based on Signal-Android cold
// KAA benchmarks. Explicit KRIT_TYPES_SHARDS disables the default to avoid
// nesting file-level parallelism inside every JVM shard; setting
// KRIT_TYPES_PARALLEL_FILES explicitly still wins.
func configuredKritTypesParallelFiles() int {
	raw := strings.TrimSpace(os.Getenv("KRIT_TYPES_PARALLEL_FILES"))
	if raw == "" {
		if strings.TrimSpace(os.Getenv("KRIT_TYPES_SHARDS")) != "" {
			return 0
		}
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
