package oracle

import (
	"os"
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
	ExtraJVMArgs []string
	// ForcedMisses lists absolute .kt paths that the cache classifier
	// must treat as misses without consulting the per-file content
	// hash. Used by the warm-path freshness gate to route a known-stale
	// file (e.g. an ABI edit detected by the bundle manifest comparator)
	// through a partial JVM reanalyze instead of letting it ride on a
	// stale cached entry. Empty preserves the pre-#NNN behavior.
	ForcedMisses []string
	// Classpath is the user-configured JVM classpath (krit.yml
	// `oracle.classpath` + the `CLASSPATH` env var). Forwarded to
	// krit-fir as `--classpath <joined>`. Empty preserves the existing
	// source-tree-discovery fallback.
	Classpath []string
	JvmTarget string
	// Kotlin is the project's effective Kotlin compiler settings (language and
	// API level, opt-ins, allowlisted flags), forwarded to krit-fir as
	// `--kotlin-arg` flags. The zero value keeps the embedded compiler's
	// defaults. It is part of the cache scope (NewStoreScopeWithSettings).
	Kotlin gradlemodel.KotlinSettings
	// Backend is the JVM backend being invoked. When set, cached entries
	// written any other way are misses (see Backend.CacheApproximation), and
	// a whole-compilation backend's entries are checked against the current
	// compilation (see Backend.ReturnsWholeCompilation).
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
	raw := strings.TrimSpace(os.Getenv("KRIT_ORACLE_EXTRA_JVM_ARGS"))
	if raw == "" {
		return nil
	}
	return strings.Fields(raw)
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

func recordOracleJVMArgs(t perf.Tracker, extra []string) {
	if t == nil || !t.IsEnabled() {
		return
	}
	addOracleInstant(t, "oracleJVMArgs", map[string]int64{
		"extraArgs": int64(len(extra)),
	}, map[string]string{
		"args": strings.Join(extra, " "),
	})
}
