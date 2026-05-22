package scan

import (
	"flag"
	"runtime"
)

// scanFlags bundles every CLI flag the scan default verb declares.
// Pulled out of Run so the 80-line block of flag.Bool/String/Int calls
// has one named owner; downstream readers see the runner's flag dependencies
// as a single typed field instead of 60+ closed-over local variables.
//
// The struct holds pointers (not the post-Parse values) so registerScanFlags
// can be called once on a fresh FlagSet and the same struct can be shared
// with every helper that historically read *flagPointer.
type scanFlags struct {
	Format                   *string
	Report                   *string
	Perf                     *bool
	PerfRules                *bool
	CPUProfile               *string
	MemProfile               *string
	ProfileDispatch          *bool
	IncludeGenerated         *bool
	Output                   *string
	Jobs                     *int
	Quiet                    *bool
	Verbose                  *bool
	Fix                      *bool
	FixSuffix                *string
	FixLevel                 *string
	FixFindingID             *string
	DryRun                   *bool
	AllRules                 *bool
	Experimental             *bool
	Maturity                 *string
	Strict                   *bool
	Baseline                 *string
	CreateBaseline           *string
	BasePath                 *string
	EditorConfig             *bool
	Version                  *bool
	NoCache                  *bool
	NoParseCache             *bool
	ParseCacheCapMB          *int
	NoResourceCache          *bool
	ClearCache               *bool
	NoMatrixCache            *bool
	ClearMatrixCache         *bool
	CacheDir                 *string
	StoreDir                 *string
	Config                   *string
	List                     *bool
	ListRulesCWE             *string
	NoTypeInfer              *bool
	ValidateConfig           *bool
	GenerateSchema           *bool
	InputTypes               *string
	OutputTypes              *string
	NoTypeOracle             *bool
	OracleBackend            *string
	NoCacheOracle            *bool
	NoCrossFileCache         *bool
	CustomRuleJars           *string
	Daemon                   *bool
	NoOracleFilter           *bool
	OracleDiagnostics        *bool
	OracleFilterFingerprint  *bool
	Fir                      *bool
	NoFir                    *bool
	NoFirDaemon              *bool
	FixBinary                *bool
	WarningsAsErrors         *bool
	MinConfidence            *float64
	Completions              *string
	Init                     *bool
	Doctor                   *bool
	RemoveDeadCode           *bool
	Diff                     *string
	Delta                    *string
	DisableRules             *string
	DisableRelated           *bool
	EnableRules              *string
	MaxCost                  *string
	Experiment               *string
	ExperimentOff            *string
	ListExperiments          *bool
	ExperimentCandidates     *string
	ExperimentIntent         *string
	ExperimentMatrix         *string
	ExperimentRuns           *int
	ExperimentTargets        *string
	PromoteExperiment        *string
	DeprecateExperiment      *string
	NewExperiment            *string
	NewExperimentDescription *string
	NewExperimentIntent      *string
	NewExperimentTargetRules *string
	NewExperimentWireFile    *string
	SampleRule               *string
	SampleCount              *int
	SampleContext            *int
	RuleAudit                *bool
	RuleAuditMin             *int
	RuleAuditDetails         *int
	RuleAuditSamples         *int
	RuleAuditContext         *int
	RuleAuditCluster         *string
	BaselineAudit            *bool
	Depth                    *string
	NoDaemon                 *bool
	DaemonSocket             *string
}

// registerScanFlags declares every scan-verb flag against fs and returns a
// scanFlags struct holding pointers into fs's storage. Behavior matches the
// original inline declarations in Run bit-for-bit: same defaults, same
// usage strings, same flag names (including aliases like --format).
//
// fs is typically flag.CommandLine when called from Run, but tests can
// pass a fresh FlagSet to introspect defaults without polluting global state.
func registerScanFlags(fs *flag.FlagSet) *scanFlags {
	f := &scanFlags{}
	f.Format = fs.String("f", "json", "Output format: json, plain, sarif, checkstyle (auto: plain in terminal, json when piped)")
	fs.StringVar(f.Format, "format", "json", "Alias for -f")
	f.Report = fs.String("report", "", "Report format: json, plain, sarif, checkstyle (alias for -f, takes precedence)")
	f.Perf = fs.Bool("perf", false, "Include performance timing in output")
	f.PerfRules = fs.Bool("perf-rules", false, "Include per-rule execution ranking in JSON output and stderr table (implies --perf)")
	f.CPUProfile = fs.String("cpuprofile", "", "Write CPU profile to file (in daemon mode profiles the daemon process; pair with --no-daemon to profile the short-lived CLI)")
	f.MemProfile = fs.String("memprofile", "", "Write memory profile to file (in daemon mode profiles the daemon process; pair with --no-daemon to profile the short-lived CLI)")
	f.ProfileDispatch = fs.Bool("profile-dispatch", false, "Debug: emit per-file dispatch timing distribution to stderr (works in both daemon and in-process modes)")
	f.IncludeGenerated = fs.Bool("include-generated", false, "Include files under */generated/* directories (default: skipped, not user-maintained code)")
	f.Output = fs.String("o", "", "Write output to file")
	f.Jobs = fs.Int("j", runtime.NumCPU(), "Number of parallel jobs")
	f.Quiet = fs.Bool("q", false, "Only print findings")
	f.Verbose = fs.Bool("v", false, "Verbose output")
	f.Fix = fs.Bool("fix", false, "Apply auto-fixes to files")
	f.FixSuffix = fs.String("fix-suffix", "", "Write fixed files with this suffix instead of editing in place (e.g., '.new')")
	f.FixLevel = fs.String("fix-level", "idiomatic", "Maximum fix level: cosmetic, idiomatic, semantic")
	f.FixFindingID = fs.String("finding-id", "", "With --fix: restrict fix application to a single finding id (<rule>:<file>:<line>:<col>)")
	f.DryRun = fs.Bool("dry-run", false, "Show what --fix would change without modifying files")
	f.AllRules = fs.Bool("all-rules", false, "Enable all rules including opt-in")
	f.Experimental = fs.Bool("experimental", false, "Enable rules whose Maturity is experimental (does not enable deprecated rules)")
	fs.BoolVar(f.Experimental, "enable-experimental", false, "Alias for --experimental")
	f.Maturity = fs.String("maturity", "", "With --list-rules: filter to rules whose Maturity matches (stable, experimental, or deprecated)")
	f.Strict = fs.Bool("strict", false, "Strict preset: exclude rules declared NoisinessNoisy. Combine with config strict: true.")
	f.Baseline = fs.String("baseline", "", "Baseline file to suppress known issues (XML or JSON)")
	f.CreateBaseline = fs.String("create-baseline", "", "Create a baseline file from current findings")
	f.BasePath = fs.String("base-path", "", "Base path for relative file paths in baselines and reports (default: first scan path)")
	f.EditorConfig = fs.Bool("enable-editorconfig", false, "Read .editorconfig for max_line_length, indent_size, etc.")
	f.Version = fs.Bool("version", false, "Print version")
	f.NoCache = fs.Bool("no-cache", false, "Disable incremental analysis cache")
	f.NoParseCache = fs.Bool("no-parse-cache", false, "Disable the on-disk tree-sitter parse cache (forces re-parse of every file)")
	f.ParseCacheCapMB = fs.Int("parse-cache-cap-mb", 0, "Size cap in MB for .krit/parse-cache/ (0 = use config or default 1024; negative = unlimited)")
	f.NoResourceCache = fs.Bool("no-resource-cache", false, "Disable the on-disk Android values-XML ResourceIndex cache (forces re-parse of every values XML file)")
	f.ClearCache = fs.Bool("clear-cache", false, "Delete all on-disk caches (incremental, parse) and exit")
	f.NoMatrixCache = fs.Bool("no-matrix-cache", false, "Disable the experiment-matrix baseline cache (no read, no write)")
	f.ClearMatrixCache = fs.Bool("clear-matrix-cache", false, "Delete the experiment-matrix baseline cache and exit")
	f.CacheDir = fs.String("cache-dir", "", "Override incremental cache directory (default: <repo>/.krit/cache)")
	f.StoreDir = fs.String("store-dir", "", "Unified store directory (enables store-backed incremental cache; default: .krit/store when present)")
	f.Config = fs.String("config", "", "Path to YAML config file (default: auto-detect krit.yml or .krit.yml)")
	f.List = fs.Bool("list-rules", false, "List all rules (add -v to show fixable)")
	f.ListRulesCWE = fs.String("cwe", "", "Filter --list-rules by taxonomy ID (CWE/OWASP/SEI-CERT/MITRE; case-insensitive)")
	f.NoTypeInfer = fs.Bool("no-type-inference", false, "Disable type inference (faster but less precise)")
	f.ValidateConfig = fs.Bool("validate-config", false, "Validate config file and exit")
	f.GenerateSchema = fs.Bool("generate-schema", false, "Print JSON Schema for krit.yml to stdout")
	f.InputTypes = fs.String("input-types", "", "Load pre-built type oracle JSON (skip JVM invocation)")
	f.OutputTypes = fs.String("output-types", "", "Run krit-types and write oracle JSON to this path, then exit")
	f.NoTypeOracle = fs.Bool("no-type-oracle", false, "Skip the JVM type oracle entirely (faster, less precise)")
	f.OracleBackend = fs.String("oracle-backend", "", "Pick the JVM daemon for the type oracle: 'kaa' (krit-types, default) or 'fir' (krit-fir). Overrides the oracle.backend value in krit.yml.")
	f.NoCacheOracle = fs.Bool("no-cache-oracle", false, "Disable the on-disk incremental oracle cache (forces a full JVM run)")
	f.NoCrossFileCache = fs.Bool("no-cross-file-cache", false, "Disable the on-disk cross-file index cache (forces a full crossFileAnalysis rebuild)")
	f.CustomRuleJars = fs.String("custom-rule-jars", "", "Comma-separated Kotlin custom-rule jars to load through the krit-types daemon (experimental)")
	f.Daemon = fs.Bool("daemon", false, "Use long-lived krit-types daemon instead of one-shot invocation")
	f.NoOracleFilter = fs.Bool("no-oracle-filter", false, "Disable the rule-classification oracle filter (feeds every file to krit-types, matching the pre-filter baseline; used to validate findings-equivalence)")
	f.OracleDiagnostics = fs.Bool("oracle-diagnostics", false, "Collect Kotlin compiler diagnostics in the type oracle (slower; enables diagnostic-backed oracle findings)")
	f.OracleFilterFingerprint = fs.Bool("oracle-filter-fingerprint", false, "Compute the oracle filter input-set fingerprint for the given paths and print JSON to stdout; exits without running rules. Used by the CI drift gate.")
	f.Fir = fs.Bool("fir", false, "Enable FIR checker pass (krit-fir JVM subprocess); default off during pilot phase")
	f.NoFir = fs.Bool("no-fir", false, "Disable FIR checker pass even when enabled by config")
	f.NoFirDaemon = fs.Bool("no-fir-daemon", false, "Force one-shot mode for FIR checker (no persistent daemon; useful for hermetic CI runners)")
	f.FixBinary = fs.Bool("fix-binary", false, "Apply binary file fixes (image conversion, optimization, file operations)")
	f.WarningsAsErrors = fs.Bool("warnings-as-errors", false, "Treat warnings as errors (exit code 1)")
	f.MinConfidence = fs.Float64("min-confidence", 0, "Minimum confidence (0.0-1.0) for findings to be reported; 0 keeps all. Rules that don't set confidence are treated as 0 and dropped when this flag is > 0.")
	f.Completions = fs.String("completions", "", "Print shell completions (bash, zsh, fish)")
	f.Init = fs.Bool("init", false, "Generate a starter krit.yml config file")
	f.Doctor = fs.Bool("doctor", false, "Check environment: Java, config, tools")
	f.RemoveDeadCode = fs.Bool("remove-dead-code", false, "Bulk-remove directly fixable dead code findings; pair with --dry-run to preview")
	f.Diff = fs.String("diff", "", "Only report findings in files changed since git ref (e.g., HEAD~1, main, origin/main)")
	f.Delta = fs.String("delta", "", "Only fail/report findings newly introduced since git ref (e.g., main, origin/main)")
	f.DisableRules = fs.String("disable-rules", "", "Comma-separated rules to disable (e.g., MagicNumber,MaxLineLength)")
	f.DisableRelated = fs.Bool("disable-related", false, "Also disable every rule listed in the RelatedRules metadata of each --disable-rules entry (non-transitive: only one hop is followed).")
	f.EnableRules = fs.String("enable-rules", "", "Comma-separated rules to enable (overrides config)")
	f.MaxCost = fs.String("max-cost", "", "Maximum rule weight class to run: trivial, line, ast (fast), crossfile (balanced), oracle, fir (thorough). Filters the active rule set so higher-cost rules are skipped.")
	f.Experiment = fs.String("experiment", "", "Comma-separated experiment feature flags to enable")
	f.ExperimentOff = fs.String("experiment-off", "", "Comma-separated experiment feature flags to disable")
	f.ListExperiments = fs.Bool("list-experiments", false, "List available experiment feature flags")
	f.ExperimentCandidates = fs.String("experiment-candidates", "", "Comma-separated experiment flags used as matrix candidates")
	f.ExperimentIntent = fs.String("experiment-intent", "", "Filter experiment candidates by intent (e.g. performance, fp-reduction, correctness)")
	f.ExperimentMatrix = fs.String("experiment-matrix", "", "Run an experiment matrix: baseline,singles,pairs,cumulative or explicit cases separated by ';' and '+'")
	f.ExperimentRuns = fs.Int("experiment-runs", 3, "Number of runs per experiment-matrix case; findings are intersected across runs for run-to-run stability filtering (pass 1 to disable)")
	f.ExperimentTargets = fs.String("experiment-targets", "", "Comma-separated repo targets to run matrix cases against separately")
	f.PromoteExperiment = fs.String("promote-experiment", "", "Promote NAME in internal/experiment/experiment.go to Status: \"promoted\" (rewrite + go build; revert on failure)")
	f.DeprecateExperiment = fs.String("deprecate-experiment", "", "Mark NAME in internal/experiment/experiment.go as Status: \"deprecated\" (rewrite + go build; revert on failure)")
	f.NewExperiment = fs.String("new-experiment", "", "Scaffold a new experiment: kebab-case name to register in the catalog")
	f.NewExperimentDescription = fs.String("new-experiment-description", "", "Scaffold: human-readable description for the new experiment")
	f.NewExperimentIntent = fs.String("new-experiment-intent", "fp-reduction", "Scaffold: experiment intent (fp-reduction or performance)")
	f.NewExperimentTargetRules = fs.String("new-experiment-target-rules", "", "Scaffold: comma-separated target rule names the new experiment gates")
	f.NewExperimentWireFile = fs.String("new-experiment-wire-file", "", "Scaffold: rule file (relative to repo root) where the experiment guard will be wired")
	f.SampleRule = fs.String("sample-rule", "", "Sample findings for a rule (suppresses normal output)")
	f.SampleCount = fs.Int("sample-count", 10, "Number of finding samples to print for --sample-rule")
	f.SampleContext = fs.Int("sample-context", 3, "Source context lines shown above/below each --sample-rule sample")
	f.RuleAudit = fs.Bool("rule-audit", false, "Print a prioritized per-rule audit of findings (count, existing experiment, sample) and exit")
	f.RuleAuditMin = fs.Int("rule-audit-min-findings", 1, "Minimum findings for a rule to appear in --rule-audit")
	f.RuleAuditDetails = fs.Int("rule-audit-details", 5, "Number of top unexperimented rules to print samples for in --rule-audit")
	f.RuleAuditSamples = fs.Int("rule-audit-samples", 2, "Samples per rule printed in the --rule-audit details section")
	f.RuleAuditContext = fs.Int("rule-audit-context", 2, "Source context lines above/below each --rule-audit sample")
	f.RuleAuditCluster = fs.String("rule-audit-cluster", "", "Filter --rule-audit to rules whose cluster label contains this substring (e.g. 'res/', 'manifest', 'kt')")
	f.BaselineAudit = fs.Bool("baseline-audit", false, "Audit a baseline file for dead entries and removed rules, then exit")
	f.Depth = fs.String("depth", "", "Analysis depth preset: fast (skip JVM oracle), balanced (default), thorough. Overrides krit.yml analysis.depth; individual --no-* flags still take precedence.")
	f.NoDaemon = fs.Bool("no-daemon", false, "Force in-process execution; do not contact the krit daemon even if a socket is reachable.")
	f.DaemonSocket = fs.String("daemon-socket", "", "Override the krit daemon socket path (default <repoRoot>/.krit/daemon.sock).")
	return f
}
