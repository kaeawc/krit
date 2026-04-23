package oracle

import (
	"encoding/json"
	"fmt"
	"os"
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

func extraJVMArgsFromEnv() []string {
	raw := strings.TrimSpace(os.Getenv("KRIT_TYPES_EXTRA_JVM_ARGS"))
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
