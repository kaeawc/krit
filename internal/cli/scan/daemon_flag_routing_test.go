package scan

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// scanFlagRoute is one scan flag's classification for daemon delegation
// (#1956). Every flag registerScanFlags defines is in exactly one of
// forwardedScanFlags, inProcessScanFlags and irrelevantScanFlags, and
// TestScanFlagRoutingMatchesDelegation checks the code does what the entry
// says.
type scanFlagRoute struct {
	name string
	// value is what the check sets the flag to. Empty for a boolean flag
	// means the opposite of its default.
	value string
	// with are flags set on both sides of the comparison, for a flag that
	// only matters next to another one.
	with []string
	// why says how the flag reaches the daemon, why it keeps the scan
	// in-process, or why delegation does not need it.
	why string
	// unchecked, when set, says why the comparison cannot observe the flag
	// and where it is tested instead.
	unchecked string
}

// forwardedScanFlags change what a delegated scan sends to the daemon: a
// field of the analyze-project request, or a meta verb instead of it.
var forwardedScanFlags = []scanFlagRoute{
	{name: "f", value: "sarif", why: "format"},
	{name: "format", value: "sarif", why: "format"},
	{name: "report", value: "sarif", why: "format; a FORMAT=PATH value runs in-process"},
	{name: "perf", why: "show_perf"},
	{name: "perf-rules", why: "perf_rules (implies show_perf)"},
	{name: "cpuprofile", value: "cpu.pprof", why: "cpu_profile_path, absolute; the daemon process is profiled"},
	{name: "memprofile", value: "mem.pprof", why: "mem_profile_path, absolute; the daemon process is profiled"},
	{name: "profile-dispatch", why: "profile_dispatch"},
	{name: "include-generated", why: "include_generated"},
	{name: "v", why: "verbose; the daemon streams the verbose output back"},
	{name: "fix", why: "include_columns; the CLI applies the fixes to the daemon's findings"},
	{name: "fix-level", value: "semantic", why: "fix_level"},
	{name: "dry-run", why: "dry_run"},
	{name: "all-rules", why: "all_rules"},
	{name: "experimental", why: "experimental"},
	{name: "enable-experimental", why: "experimental (alias of --experimental)"},
	{name: "strict", why: "strict"},
	{name: "baseline", value: "baseline.xml", why: "baseline"},
	{name: "create-baseline", value: "baseline.xml", why: "create_baseline; the CLI writes the file from the daemon's baseline IDs"},
	{name: "absolute-paths", why: "absolute_paths"},
	{name: "base-path", value: "src", why: "base_path, absolute"},
	{name: "no-cache", why: "no_cache"},
	{name: "no-parse-cache", why: "no_parse_cache, confirmed by parse_cache_bypassed"},
	{name: "list-rules", why: "list-rules meta verb"},
	{name: "no-type-inference", why: "no_type_oracle: in-process, the flag turns the type oracle off and source inference still runs"},
	{name: "validate-config", why: "validate-config meta verb"},
	{name: "input-types", value: "types.json", why: "input_types_path, absolute"},
	{name: "output-types", value: "types.json", why: "dump-types meta verb"},
	{name: "no-type-oracle", why: "no_type_oracle"},
	{name: "oracle-backend", value: "fir", why: "oracle_backend"},
	{name: "no-cache-oracle", why: "no_cache_oracle"},
	{name: "gradle-model", value: "model", why: "gradle_model"},
	{name: "no-gradle-model", why: "no_gradle_model"},
	{
		name: "generated-srcjar", value: "gen.srcjar",
		why:       "oracle_source_dirs and oracle_generated_source_dirs: the CLI unpacks the jars before delegating",
		unchecked: "the jars are unpacked by loadScanGradleContext, which runs before delegation; see TestWithGeneratedSrcjarsAddsCompileAndGeneratedRoots",
	},
	{name: "custom-rule-jars", value: "rules.jar", why: "custom_rule_jars"},
	{name: "oracle-filter-fingerprint", why: "oracle-filter-fingerprint meta verb"},
	{name: "fir", value: "false", why: "fir and no_fir"},
	{name: "no-fir", why: "fir and no_fir"},
	{name: "no-fir-daemon", why: "no_fir_daemon"},
	{name: "fix-binary", why: "include_columns; the CLI applies the binary fixes"},
	{name: "warnings-as-errors", why: "warnings_as_errors"},
	{name: "min-confidence", value: "0.5", why: "min_confidence"},
	{name: "remove-dead-code", why: "include_columns; the CLI removes the dead code"},
	{name: "diff", value: "HEAD", why: "diff; in PR mode (a JVM pass and a Java launcher) the scan runs in-process"},
	{name: "delta", value: "main", why: "include_columns; the CLI filters the daemon's findings against the base scan. In PR mode the scan runs in-process"},
	{name: "disable-rules", value: "MagicNumber", why: "disable_rules"},
	{name: "disable-related", with: []string{"--disable-rules=KaptUsage"}, why: "disable_rules: the CLI adds the related rules to the list it sends"},
	{name: "enable-rules", value: "MagicNumber", why: "enable_rules"},
	{name: "list-experiments", why: "list-experiments meta verb"},
	{name: "rule-audit", why: "include_columns; the CLI runs the audit"},
	{name: "baseline-audit", why: "include_columns; the CLI runs the audit"},
	{name: "depth", value: "fast", why: "the preset's flags (fast sets no_type_oracle, thorough sets fir)"},
}

// inProcessScanFlags keep the scan out of the daemon when set.
var inProcessScanFlags = []scanFlagRoute{
	{name: "reproducible", why: "the daemon renders the envelope with its own timings and cache statistics"},
	{name: "detekt-config", value: "detekt.yml", why: "the request carries no detekt options"},
	{name: "no-detekt-config", why: "the request carries no detekt options"},
	{name: "enable-editorconfig", why: "it rewrites the config, and the daemon keeps one resident config for every request"},
	{name: "version", why: "reports on the calling binary"},
	{name: "parse-cache-cap-mb", value: "64", why: "the daemon's parse cache is resident and keeps its default cap"},
	{name: "clear-cache", why: "the clear-cache verb serves it when a daemon is running; otherwise it clears in-process instead of starting a daemon"},
	{name: "clear-matrix-cache", why: "the clear-matrix-cache verb serves it when a daemon is running; otherwise it clears in-process instead of starting a daemon"},
	{name: "cache-dir", value: "custom-cache", why: "the daemon keeps its analysis cache in its own state directory"},
	{name: "store-dir", value: "custom-store", why: "the daemon keeps its caches in its own state directory"},
	{name: "config", value: filepath.Join("no-such-dir", "krit.yml"), why: "the daemon loads the config at its root; the same file is still delegated"},
	{name: "generate-schema", why: "describes the calling binary's rules"},
	{name: "dump-oracle-diagnostics", why: "meta command with no daemon verb"},
	{name: "gradle-project-outputs", value: "outputs.json", why: "the request carries no project outputs"},
	{name: "no-cross-file-cache", why: "the daemon also keeps a code index in memory; with --no-cache the scan is delegated"},
	{name: "no-oracle-filter", why: "it changes which files the resident type oracle loads; it is a findings-equivalence baseline"},
	{name: "min-fir-coverage", value: "0.8", why: "the daemon's delta and fix paths carry no coverage gate"},
	{name: "min-sdk", value: "21", why: "it sets a process-wide default a daemon must not keep"},
	{name: "target-sdk", value: "34", why: "it sets a process-wide default a daemon must not keep"},
	{name: "completions", value: "bash", why: "prints an asset of the calling binary"},
	{name: "init", why: "writes krit.yml in the caller's directory"},
	{name: "doctor", why: "reports on the calling environment"},
	{name: "max-cost", value: "ast", why: "the request carries no cost filter"},
	{name: "only-scope", value: "file", why: "the request carries no scope filter"},
	{name: "exclude-scope", value: "file", why: "the request carries no scope filter"},
	{name: "experiment", value: "return-count-skip-when-initializer-guards", why: "experiments are process-wide and no findings cache is keyed on them"},
	{name: "experiment-off", value: "return-count-skip-when-initializer-guards", why: "experiments are process-wide and no findings cache is keyed on them"},
	{name: "experiment-matrix", value: "baseline", why: "re-executes krit per matrix case"},
	{name: "promote-experiment", value: "x", why: "rewrites the experiment catalog and runs go build"},
	{name: "deprecate-experiment", value: "x", why: "rewrites the experiment catalog and runs go build"},
	{name: "new-experiment", value: "x", why: "scaffolds a new experiment in the source tree"},
	{name: "no-daemon", why: "asks for an in-process scan"},
}

// irrelevantScanFlags change nothing the daemon sees: the CLI applies them
// to the daemon's result, they only pick the daemon, or they only matter
// next to a flag that already runs in-process.
var irrelevantScanFlags = []scanFlagRoute{
	{name: "o", value: "out.json", why: "the CLI writes the daemon's report to the file"},
	{name: "j", value: "3", why: "worker count; the findings do not depend on it"},
	{name: "q", why: "the CLI prints the summary lines"},
	{name: "fix-suffix", value: ".new", why: "the CLI applies the fixes"},
	{name: "finding-id", value: "MagicNumber:a.kt:1:1", why: "the CLI applies the fixes"},
	{name: "maturity", value: "stable", why: "read by --list-rules, which sends it in the list-rules verb"},
	{name: "cwe", value: "CWE-79", why: "read by --list-rules, which sends it in the list-rules verb"},
	{name: "no-resource-cache", why: "the daemon keeps no on-disk resource index cache"},
	{name: "no-matrix-cache", why: "read by --experiment-matrix, which runs in-process"},
	{name: "daemon", why: "picks the krit-fir oracle daemon for an in-process scan; the krit daemon keeps its oracle resident"},
	{name: "fail-on-degraded", why: "the CLI applies it to the daemon's FIR status"},
	{name: "no-pr-mode", why: "without it a PR-mode scan runs in-process; with it the scan is delegated as usual"},
	{name: "pr-files", value: "files.txt", why: "PR mode for --delta's base scan, which runs in-process when there is JVM work"},
	{name: "experiment-candidates", value: "x", why: "read by --experiment-matrix, which runs in-process"},
	{name: "experiment-intent", value: "performance", why: "read by --experiment-matrix, which runs in-process"},
	{name: "experiment-runs", value: "1", why: "read by --experiment-matrix, which runs in-process"},
	{name: "experiment-targets", value: "x", why: "read by --experiment-matrix, which runs in-process"},
	{name: "new-experiment-description", value: "x", why: "read by --new-experiment, which runs in-process"},
	{name: "new-experiment-intent", value: "performance", why: "read by --new-experiment, which runs in-process"},
	{name: "new-experiment-target-rules", value: "x", why: "read by --new-experiment, which runs in-process"},
	{name: "new-experiment-wire-file", value: "x", why: "read by --new-experiment, which runs in-process"},
	{name: "sample-rule", value: "MagicNumber", why: "the CLI samples the daemon's JSON findings"},
	{name: "sample-count", value: "3", why: "the CLI samples the daemon's JSON findings"},
	{name: "sample-context", value: "1", why: "the CLI samples the daemon's JSON findings"},
	{name: "rule-audit-min-findings", value: "3", why: "the CLI runs the audit"},
	{name: "rule-audit-details", value: "3", why: "the CLI runs the audit"},
	{name: "rule-audit-samples", value: "3", why: "the CLI runs the audit"},
	{name: "rule-audit-context", value: "1", why: "the CLI runs the audit"},
	{name: "rule-audit-cluster", value: "kt", why: "the CLI runs the audit"},
	{name: "daemon-socket", value: "other.sock", why: "picks the daemon"},
}

// flagSpelling is how a flag name is written on the command line and in docs.
func flagSpelling(name string) string {
	if len(name) == 1 {
		return "-" + name
	}
	return "--" + name
}

const scanFlagRoutingFix = "add it to exactly one of forwardedScanFlags, inProcessScanFlags or irrelevantScanFlags " +
	"in internal/cli/scan/daemon_flag_routing_test.go, make the code match " +
	"(buildDaemonAnalyzeArgs sends it, or daemonCompatibleFlags keeps the scan in-process), " +
	"and add a row for it to the table in docs/daemon-flag-routing.md"

// TestScanFlagRoutingIsComplete fails when a scan flag has no daemon
// routing, so a new flag cannot be silently dropped by delegated scans.
func TestScanFlagRoutingIsComplete(t *testing.T) {
	fs := flag.NewFlagSet("krit", flag.ContinueOnError)
	registerScanFlags(fs)
	lists := map[string][]scanFlagRoute{
		"forwardedScanFlags":  forwardedScanFlags,
		"inProcessScanFlags":  inProcessScanFlags,
		"irrelevantScanFlags": irrelevantScanFlags,
	}
	seen := map[string][]string{}
	for list, routes := range lists {
		for _, r := range routes {
			seen[r.name] = append(seen[r.name], list)
			if strings.TrimSpace(r.why) == "" {
				t.Errorf("%s entry %s has no reason", list, flagSpelling(r.name))
			}
		}
	}
	fs.VisitAll(func(fl *flag.Flag) {
		switch in := seen[fl.Name]; len(in) {
		case 0:
			t.Errorf("scan flag %s has no daemon routing: %s", flagSpelling(fl.Name), scanFlagRoutingFix)
		case 1:
		default:
			sort.Strings(in)
			t.Errorf("scan flag %s is in %s; keep it in exactly one list", flagSpelling(fl.Name), strings.Join(in, " and "))
		}
		delete(seen, fl.Name)
	})
	for name := range seen {
		t.Errorf("routing entry %s names no scan flag; remove it", flagSpelling(name))
	}
}

// delegationOutcome is what tryDaemonDelegate does with a command line,
// decided from the flags alone: whether the daemon serves it, through which
// verb, and, for analyze-project, the request it sends as wire JSON, which is
// what the daemon sees.
type delegationOutcome struct {
	delegated bool
	verb      metaVerbName
	request   string
}

func outcomeFor(t *testing.T, repoDir string, args []string) delegationOutcome {
	t.Helper()
	fs := flag.NewFlagSet("krit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	f := registerScanFlags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse %q: %v", args, err)
	}
	if *f.PerfRules { // as Run does before delegating
		*f.Perf = true
	}
	verb, ok := scanDelegation(f, fs, fs.Args(), repoDir)
	if !ok {
		return delegationOutcome{}
	}
	out := delegationOutcome{delegated: true, verb: verb}
	if verb == metaVerbNone {
		raw, err := json.Marshal(buildDaemonAnalyzeArgs(f, fs.Args()))
		if err != nil {
			t.Fatal(err)
		}
		out.request = string(raw)
	}
	return out
}

// TestScanFlagRoutingMatchesDelegation sets each flag and checks the
// delegation decision does what its routing entry says: a forwarded flag
// changes the request (or its verb), an in-process flag keeps the scan out
// of the daemon, and an irrelevant flag changes neither.
func TestScanFlagRoutingMatchesDelegation(t *testing.T) {
	repoDir := t.TempDir()
	t.Chdir(repoDir)
	// A terminal stdout would turn the default json format into plain.
	t.Setenv("NO_COLOR", "1")
	t.Setenv("KRIT_FIR_DAEMON_POOL", "")
	origJava := javaForPRMode
	javaForPRMode = func() (string, error) { return "", errors.New("no java in this test") }
	t.Cleanup(func() { javaForPRMode = origJava })
	origSupported := daemonSupported
	daemonSupported = func() bool { return true }
	t.Cleanup(func() { daemonSupported = origSupported })

	defaults := flag.NewFlagSet("krit", flag.ContinueOnError)
	registerScanFlags(defaults)
	setting := func(r scanFlagRoute) string {
		value := r.value
		if value == "" {
			fl := defaults.Lookup(r.name)
			if fl == nil {
				t.Fatalf("%s is not a scan flag", flagSpelling(r.name))
			}
			if b, ok := fl.Value.(interface{ IsBoolFlag() bool }); !ok || !b.IsBoolFlag() {
				t.Fatalf("%s is not boolean; its routing entry needs a value", flagSpelling(r.name))
			}
			value = map[string]string{"false": "true", "true": "false"}[fl.DefValue]
		}
		return flagSpelling(r.name) + "=" + value
	}
	check := func(list string, routes []scanFlagRoute, verify func(t *testing.T, base, set delegationOutcome)) {
		for _, r := range routes {
			t.Run(list+"/"+r.name, func(t *testing.T) {
				if r.unchecked != "" {
					t.Skip(r.unchecked)
				}
				base := outcomeFor(t, repoDir, r.with)
				if !base.delegated {
					t.Fatalf("the scan without %s is not delegated", flagSpelling(r.name))
				}
				set := outcomeFor(t, repoDir, append(append([]string(nil), r.with...), setting(r)))
				verify(t, base, set)
			})
		}
	}
	check("forwarded", forwardedScanFlags, func(t *testing.T, base, set delegationOutcome) {
		if !set.delegated {
			t.Fatalf("a scan with this flag is not delegated; move it to inProcessScanFlags or forward it")
		}
		if set.verb == base.verb && set.request == base.request {
			t.Fatalf("the daemon request is the same with and without the flag, so a delegated scan drops it: send it in buildDaemonAnalyzeArgs or keep the scan in-process in daemonCompatibleFlags")
		}
	})
	check("inProcess", inProcessScanFlags, func(t *testing.T, _, set delegationOutcome) {
		if set.delegated {
			t.Fatalf("a scan with this flag is still delegated (verb %d); keep it in-process in daemonCompatibleFlags", set.verb)
		}
	})
	check("irrelevant", irrelevantScanFlags, func(t *testing.T, base, set delegationOutcome) {
		if !set.delegated {
			t.Fatalf("a scan with this flag is not delegated; move it to inProcessScanFlags")
		}
		if set.verb != base.verb || set.request != base.request {
			t.Fatalf("the flag changes the daemon request; move it to forwardedScanFlags\nwithout: %s\nwith:    %s", base.request, set.request)
		}
	})
}

// routingDocRow matches a row of the flag table in docs/daemon-flag-routing.md.
var routingDocRow = regexp.MustCompile("^\\| *((?:`-[^`]+`(?:, *)?)+) *\\| *(forwarded|in-process|not needed) *\\|")

// TestScanFlagRoutingDocTable keeps the flag table in
// docs/daemon-flag-routing.md in step with the routing lists.
func TestScanFlagRoutingDocTable(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "daemon-flag-routing.md"))
	if err != nil {
		t.Fatal(err)
	}
	documented := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		m := routingDocRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		for _, name := range regexp.MustCompile("`-+([^`= ]+)[^`]*`").FindAllStringSubmatch(m[1], -1) {
			documented[name[1]] = m[2]
		}
	}
	want := map[string]string{}
	for _, r := range forwardedScanFlags {
		want[r.name] = "forwarded"
	}
	for _, r := range inProcessScanFlags {
		want[r.name] = "in-process"
	}
	for _, r := range irrelevantScanFlags {
		want[r.name] = "not needed"
	}
	names := make([]string, 0, len(want))
	for name := range want {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		switch got, ok := documented[name]; {
		case !ok:
			t.Errorf("docs/daemon-flag-routing.md has no row for %s; add one with route %q", flagSpelling(name), want[name])
		case got != want[name]:
			t.Errorf("docs/daemon-flag-routing.md routes %s as %q, the code as %q", flagSpelling(name), got, want[name])
		}
	}
	for name := range documented {
		if _, ok := want[name]; !ok {
			t.Errorf("docs/daemon-flag-routing.md has a row for %s, which is not a scan flag", flagSpelling(name))
		}
	}
}
