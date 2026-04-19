package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"time"

	"encoding/json"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/store"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/experiment"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/schema"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// version is set by goreleaser via ldflags: -X main.version=...
var version = "dev"

//go:embed completions
var completionsFS embed.FS

// resolvedStore returns a *store.FileStore for the given --store-dir flag
// pointer, or nil when no store directory is configured and the default
// .krit/store does not yet exist.
func resolvedStore(storeDirFlag *string) *store.FileStore {
	if storeDirFlag == nil {
		return nil
	}
	dir := *storeDirFlag
	if dir == "" {
		if _, err := os.Stat(".krit/store"); err != nil {
			return nil
		}
		dir = ".krit/store"
	}
	return store.New(dir)
}

func main() {
	baselineAuditVerb := len(os.Args) > 1 && os.Args[1] == "baseline-audit"
	harvestVerb := len(os.Args) > 1 && os.Args[1] == "harvest"
	renameVerb := len(os.Args) > 1 && os.Args[1] == "rename"
	initVerb := len(os.Args) > 1 && os.Args[1] == "init"
	apiSnapshotVerb := len(os.Args) > 1 && os.Args[1] == "api-snapshot"
	apiDiffVerb := len(os.Args) > 1 && os.Args[1] == "api-diff"
	cacheVerb := len(os.Args) > 1 && os.Args[1] == "cache"
	if cacheVerb {
		os.Exit(runCacheSubcommand(os.Args[2:]))
	}
	if harvestVerb {
		os.Exit(runHarvestSubcommand(os.Args[2:]))
	}
	if renameVerb {
		os.Exit(runRenameSubcommand(os.Args[2:]))
	}
	if initVerb {
		os.Exit(runInitSubcommand(os.Args[2:]))
	}
	if apiSnapshotVerb {
		os.Exit(runAPISnapshotSubcommand(os.Args[2:]))
	}
	if apiDiffVerb {
		os.Exit(runAPIDiffSubcommand(os.Args[2:]))
	}
	if baselineAuditVerb {
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	}

	formatFlag := flag.String("f", "json", "Output format: json, plain, sarif, checkstyle (auto: plain in terminal, json when piped)")
	flag.StringVar(formatFlag, "format", "json", "Alias for -f")
	reportFlag := flag.String("report", "", "Report format: json, plain, sarif, checkstyle (alias for -f, takes precedence)")
	perfFlag := flag.Bool("perf", false, "Include performance timing in output")
	cpuProfileFlag := flag.String("cpuprofile", "", "Write CPU profile to file")
	memProfileFlag := flag.String("memprofile", "", "Write memory profile to file")
	profileDispatchFlag := flag.Bool("profile-dispatch", false, "Debug: emit per-file dispatch timing distribution to stderr")
	includeGeneratedFlag := flag.Bool("include-generated", false, "Include files under */generated/* directories (default: skipped, not user-maintained code)")
	outputFlag := flag.String("o", "", "Write output to file")
	jobsFlag := flag.Int("j", runtime.NumCPU(), "Number of parallel jobs")
	quietFlag := flag.Bool("q", false, "Only print findings")
	verboseFlag := flag.Bool("v", false, "Verbose output")
	fixFlag := flag.Bool("fix", false, "Apply auto-fixes to files")
	fixSuffix := flag.String("fix-suffix", "", "Write fixed files with this suffix instead of editing in place (e.g., '.new')")
	fixLevelFlag := flag.String("fix-level", "idiomatic", "Maximum fix level: cosmetic, idiomatic, semantic")
	dryRunFlag := flag.Bool("dry-run", false, "Show what --fix would change without modifying files")
	allRulesFlag := flag.Bool("all-rules", false, "Enable all rules including opt-in (like detekt allRules=true)")
	baselineFlag := flag.String("baseline", "", "Baseline file to suppress known issues (detekt XML format)")
	createBaselineFlag := flag.String("create-baseline", "", "Create a baseline file from current findings")
	basePathFlag := flag.String("base-path", "", "Base path for relative file paths in baselines and reports (default: first scan path)")
	editorConfigFlag := flag.Bool("enable-editorconfig", false, "Read .editorconfig for max_line_length, indent_size, etc.")
	versionFlag := flag.Bool("version", false, "Print version")
	noCacheFlag := flag.Bool("no-cache", false, "Disable incremental analysis cache")
	clearCacheFlag := flag.Bool("clear-cache", false, "Delete the cache file and exit")
	noMatrixCacheFlag := flag.Bool("no-matrix-cache", false, "Disable the experiment-matrix baseline cache (no read, no write)")
	clearMatrixCacheFlag := flag.Bool("clear-matrix-cache", false, "Delete the experiment-matrix baseline cache and exit")
	cacheDirFlag := flag.String("cache-dir", "", "Shared cache directory (cache file named by hash of scan paths)")
	storeDirFlag := flag.String("store-dir", "", "Unified store directory (enables store-backed incremental cache; default: .krit/store when present)")
	configFlag := flag.String("config", "", "Path to YAML config file (default: auto-detect krit.yml or .krit.yml)")
	listFlag := flag.Bool("list-rules", false, "List all rules (add -v to show fixable)")
	noTypeInferFlag := flag.Bool("no-type-inference", false, "Disable type inference (faster but less precise)")
	validateConfigFlag := flag.Bool("validate-config", false, "Validate config file and exit")
	generateSchemaFlag := flag.Bool("generate-schema", false, "Print JSON Schema for krit.yml to stdout")
	inputTypesFlag := flag.String("input-types", "", "Load pre-built type oracle JSON (skip JVM invocation)")
	outputTypesFlag := flag.String("output-types", "", "Run krit-types and write oracle JSON to this path, then exit")
	noTypeOracleFlag := flag.Bool("no-type-oracle", false, "Skip the JVM type oracle entirely (faster, less precise)")
	noCacheOracleFlag := flag.Bool("no-cache-oracle", false, "Disable the on-disk incremental oracle cache (forces a full JVM run)")
	daemonFlag := flag.Bool("daemon", false, "Use long-lived krit-types daemon instead of one-shot invocation")
	noOracleFilterFlag := flag.Bool("no-oracle-filter", false, "Disable the rule-classification oracle filter (feeds every file to krit-types, matching the pre-filter baseline; used to validate findings-equivalence)")
	fixBinaryFlag := flag.Bool("fix-binary", false, "Apply binary file fixes (image conversion, optimization, file operations)")
	warningsAsErrorsFlag := flag.Bool("warnings-as-errors", false, "Treat warnings as errors (exit code 1)")
	minConfidenceFlag := flag.Float64("min-confidence", 0, "Minimum confidence (0.0-1.0) for findings to be reported; 0 keeps all. Rules that don't set confidence are treated as 0 and dropped when this flag is > 0.")
	completionsFlag := flag.String("completions", "", "Print shell completions (bash, zsh, fish)")
	initFlag := flag.Bool("init", false, "Generate a starter krit.yml config file")
	doctorFlag := flag.Bool("doctor", false, "Check environment: Java, config, tools")
	removeDeadCodeFlag := flag.Bool("remove-dead-code", false, "Bulk-remove directly fixable dead code findings; pair with --dry-run to preview")
	diffFlag := flag.String("diff", "", "Only report findings in files changed since git ref (e.g., HEAD~1, main, origin/main)")
	disableRulesFlag := flag.String("disable-rules", "", "Comma-separated rules to disable (e.g., MagicNumber,MaxLineLength)")
	enableRulesFlag := flag.String("enable-rules", "", "Comma-separated rules to enable (overrides config)")
	experimentFlag := flag.String("experiment", "", "Comma-separated experiment feature flags to enable")
	experimentOffFlag := flag.String("experiment-off", "", "Comma-separated experiment feature flags to disable")
	listExperimentsFlag := flag.Bool("list-experiments", false, "List available experiment feature flags")
	experimentCandidatesFlag := flag.String("experiment-candidates", "", "Comma-separated experiment flags used as matrix candidates")
	experimentIntentFlag := flag.String("experiment-intent", "", "Filter experiment candidates by intent (e.g. performance, fp-reduction, correctness)")
	experimentMatrixFlag := flag.String("experiment-matrix", "", "Run an experiment matrix: baseline,singles,pairs,cumulative or explicit cases separated by ';' and '+'")
	experimentRunsFlag := flag.Int("experiment-runs", 3, "Number of runs per experiment-matrix case; findings are intersected across runs for run-to-run stability filtering (pass 1 to disable)")
	experimentTargetsFlag := flag.String("experiment-targets", "", "Comma-separated repo targets to run matrix cases against separately")
	// lifecycle flags
	promoteExperimentFlag := flag.String("promote-experiment", "", "Promote NAME in internal/experiment/experiment.go to Status: \"promoted\" (rewrite + go build; revert on failure)")
	deprecateExperimentFlag := flag.String("deprecate-experiment", "", "Mark NAME in internal/experiment/experiment.go as Status: \"deprecated\" (rewrite + go build; revert on failure)")
	// end lifecycle flags

	// scaffold flags
	newExperimentFlag := flag.String("new-experiment", "", "Scaffold a new experiment: kebab-case name to register in the catalog")
	newExperimentDescriptionFlag := flag.String("new-experiment-description", "", "Scaffold: human-readable description for the new experiment")
	newExperimentIntentFlag := flag.String("new-experiment-intent", "fp-reduction", "Scaffold: experiment intent (fp-reduction or performance)")
	newExperimentTargetRulesFlag := flag.String("new-experiment-target-rules", "", "Scaffold: comma-separated target rule names the new experiment gates")
	newExperimentWireFileFlag := flag.String("new-experiment-wire-file", "", "Scaffold: rule file (relative to repo root) where the experiment guard will be wired")
	// end scaffold flags
	sampleRuleFlag := flag.String("sample-rule", "", "Sample findings for a rule (suppresses normal output)")
	sampleCountFlag := flag.Int("sample-count", 10, "Number of finding samples to print for --sample-rule")
	sampleContextFlag := flag.Int("sample-context", 3, "Source context lines shown above/below each --sample-rule sample")

	// rule audit
	ruleAuditFlag := flag.Bool("rule-audit", false, "Print a prioritized per-rule audit of findings (count, existing experiment, sample) and exit")
	ruleAuditMinFlag := flag.Int("rule-audit-min-findings", 1, "Minimum findings for a rule to appear in --rule-audit")
	ruleAuditDetailsFlag := flag.Int("rule-audit-details", 5, "Number of top unexperimented rules to print samples for in --rule-audit")
	ruleAuditSamplesFlag := flag.Int("rule-audit-samples", 2, "Samples per rule printed in the --rule-audit details section")
	ruleAuditContextFlag := flag.Int("rule-audit-context", 2, "Source context lines above/below each --rule-audit sample")
	ruleAuditClusterFlag := flag.String("rule-audit-cluster", "", "Filter --rule-audit to rules whose cluster label contains this substring (e.g. 'res/', 'manifest', 'kt')")
	baselineAuditFlag := flag.Bool("baseline-audit", false, "Audit a baseline file for dead entries and removed rules, then exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: krit [flags] [paths...]\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nSubcommands:\n")
		fmt.Fprintf(os.Stderr, "  krit baseline-audit [flags] [paths...]\n")
		fmt.Fprintf(os.Stderr, "  krit harvest SOURCE:LINE --rule RuleName --out fixture.kt\n")
		fmt.Fprintf(os.Stderr, "  krit rename [flags] <from-fqn> <to-fqn> [paths...]\n")
		fmt.Fprintf(os.Stderr, "\nSARIF upload example:\n")
		fmt.Fprintf(os.Stderr, "  krit --report=sarif -o results.sarif src/\n")
		fmt.Fprintf(os.Stderr, "  # Then upload to GitHub Code Scanning:\n")
		fmt.Fprintf(os.Stderr, "  # gh api repos/{owner}/{repo}/code-scanning/sarifs -f sarif=@results.sarif\n")
	}

	flag.Parse()
	if baselineAuditVerb {
		*baselineAuditFlag = true
	}

	// Scaffold mode: -new-experiment short-circuits the normal scan path.
	if *newExperimentFlag != "" {
		os.Exit(runNewExperimentScaffold(newExperimentOpts{
			Name:        *newExperimentFlag,
			Description: *newExperimentDescriptionFlag,
			Intent:      *newExperimentIntentFlag,
			TargetRules: experiment.ParseCSV(*newExperimentTargetRulesFlag),
			WireFile:    *newExperimentWireFileFlag,
		}))
	}

	// Resolve output format: --report takes precedence over -f
	// If no format explicitly set and writing to terminal, use plain (colored) output
	effectiveFormat := *formatFlag
	if *reportFlag != "" {
		effectiveFormat = *reportFlag
	} else if *formatFlag == "json" && *outputFlag == "" {
		// Auto-detect: plain (colored) in terminal, json when piped. Respects NO_COLOR.
		if _, noColor := os.LookupEnv("NO_COLOR"); !noColor {
			if fi, err := os.Stdout.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
				effectiveFormat = "plain"
			}
		}
	}

	if *versionFlag {
		fmt.Println("krit", version)
		os.Exit(0)
	}

	if *clearMatrixCacheFlag {
		if err := clearMatrixCache(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, "info: Matrix baseline cache cleared.")
		os.Exit(0)
	}

	if *promoteExperimentFlag != "" {
		os.Exit(promoteExperiment(*promoteExperimentFlag, experiment.StatusPromoted))
	}

	if *deprecateExperimentFlag != "" {
		os.Exit(promoteExperiment(*deprecateExperimentFlag, experiment.StatusDeprecated))
	}

	if *listExperimentsFlag {
		if effectiveFormat == "plain" {
			fmt.Print(listExperimentsLifecyclePlain())
		} else {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(struct {
				Version     string                  `json:"version"`
				Experiments []experiment.Definition `json:"experiments"`
			}{
				Version:     version,
				Experiments: experiment.Definitions(),
			})
		}
		os.Exit(0)
	}

	// Handle --completions: print shell completion script and exit
	if *completionsFlag != "" {
		var filename string
		switch *completionsFlag {
		case "bash":
			filename = "completions/krit.bash"
		case "zsh":
			filename = "completions/krit.zsh"
		case "fish":
			filename = "completions/krit.fish"
		default:
			fmt.Fprintf(os.Stderr, "Unknown shell %q; supported: bash, zsh, fish\n", *completionsFlag)
			os.Exit(1)
		}
		data, err := completionsFS.ReadFile(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading completion script: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(string(data))
		os.Exit(0)
	}

	// Handle --init: generate a starter krit.yml
	if *initFlag {
		for _, name := range []string{"krit.yml", ".krit.yml"} {
			if _, err := os.Stat(name); err == nil {
				fmt.Fprintf(os.Stderr, "Config already exists: %s\n", name)
				os.Exit(0)
			}
		}
		starter := `# Krit configuration — https://kaeawc.github.io/krit/configuration/
style:
  MagicNumber:
    excludes: ['**/test/**', '**/*Test.kt', '**/*Spec.kt']
    ignorePropertyDeclaration: true
    ignoreAnnotation: true
    ignoreEnums: true
    ignoreNumbers: ['-1', '0', '1', '2']
  MaxLineLength:
    maxLineLength: 120
    excludeCommentStatements: true
    excludes: ['**/test/**']
  ReturnCount:
    max: 3
    excludeGuardClauses: true
complexity:
  LongMethod:
    threshold: 60
  CyclomaticComplexMethod:
    allowedComplexity: 15
naming:
  FunctionNaming:
    ignoreAnnotated: ['Composable', 'Test']
potential-bugs:
  UnsafeCast:
    excludes: ['**/test/**']
`
		if err := os.WriteFile("krit.yml", []byte(starter), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing krit.yml: %v\n", err)
			os.Exit(2)
		}
		fmt.Println("Created krit.yml with recommended defaults.")
		fmt.Println("Run 'krit .' to analyze your project.")
		os.Exit(0)
	}

	// Handle --doctor: check environment
	if *doctorFlag {
		fmt.Println("krit doctor")
		fmt.Println()
		// Version
		fmt.Printf("  krit version: %s\n", version)
		// Rules
		fmt.Printf("  rules: %d registered (%d active by default)\n", len(v2rules.Registry), countActiveV2(v2rules.Registry))
		// Config
		configFound := false
		for _, name := range []string{"krit.yml", ".krit.yml"} {
			if _, err := os.Stat(name); err == nil {
				fmt.Printf("  config: %s (found)\n", name)
				configFound = true
				break
			}
		}
		if !configFound {
			fmt.Println("  config: none (run --init to create)")
		}
		// Java
		if javaPath, err := exec.LookPath("java"); err == nil {
			fmt.Printf("  java: %s\n", javaPath)
		} else {
			fmt.Println("  java: not found (optional — needed for type oracle)")
		}
		// cwebp
		if cwebpPath, err := exec.LookPath("cwebp"); err == nil {
			fmt.Printf("  cwebp: %s (WebP conversion available)\n", cwebpPath)
		} else {
			fmt.Println("  cwebp: not found (optional — needed for --fix-binary WebP)")
		}
		// krit-types
		jarPaths := []string{
			"tools/krit-types/build/libs/krit-types.jar",
			filepath.Join(os.Getenv("HOME"), ".krit", "krit-types.jar"),
		}
		jarFound := false
		for _, p := range jarPaths {
			if _, err := os.Stat(p); err == nil {
				fmt.Printf("  krit-types: %s\n", p)
				jarFound = true
				break
			}
		}
		if !jarFound {
			fmt.Println("  krit-types: not found (optional — needed for type oracle)")
		}
		fmt.Println()
		fmt.Println("  Everything looks good!")
		os.Exit(0)
	}

	// Handle --generate-schema: print JSON Schema and exit
	if *generateSchemaFlag {
		metas := schema.CollectRuleMeta()
		s := schema.GenerateSchema(metas)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(s); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(2)
		}
		os.Exit(0)
	}

	// Load YAML configuration and apply to rules
	defaultCfgPath := config.FindDefaultConfig()
	cfg, cfgErr := config.LoadAndMerge(*configFlag, defaultCfgPath)
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "warning: config: %v\n", cfgErr)
	}
	if cfg == nil {
		cfg = config.NewConfig()
	}
	rules.ApplyConfig(cfg)

	// Handle --validate-config: validate and exit
	if *validateConfigFlag {
		errs := schema.ValidateConfig(cfg)
		hasError := false
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "%s\n", e)
			if e.Level == "error" {
				hasError = true
			}
		}
		if hasError {
			fmt.Fprintf(os.Stderr, "info: Config validation failed with %d issue(s).\n", len(errs))
			os.Exit(1)
		}
		if len(errs) > 0 {
			fmt.Fprintf(os.Stderr, "info: Config validation passed with %d warning(s).\n", len(errs))
		} else {
			fmt.Fprintf(os.Stderr, "info: Config validation passed.\n")
		}
		os.Exit(0)
	}

	// Apply .editorconfig AFTER YAML config — editorconfig takes precedence
	// (matches ktfmt behavior where .editorconfig overrides --style)
	if *editorConfigFlag {
		scanDir := "."
		if len(flag.Args()) > 0 {
			scanDir = flag.Args()[0]
		}
		ec := config.LoadEditorConfig(scanDir)
		ec.ApplyToConfig(cfg)
		rules.ApplyConfig(cfg) // re-apply with editorconfig overrides
	}

	if *listFlag {
		fmt.Println("Available rules:")
		fixable := 0
		active := 0
		stubs := 0
		for _, r := range v2rules.Registry {
			markers := ""
			if rules.IsDefaultActive(r.ID) {
				markers += "A"
				active++
			} else {
				markers += " "
			}
			implemented := rules.IsImplementedV2(r)
			if !implemented {
				stubs++
			}
			stubMarker := ""
			if !implemented {
				stubMarker = " (stub)"
			}
			if fixLvl, isFixable := rules.GetV2FixLevel(r); isFixable {
				markers += "F"
				fixable++
				if *verboseFlag {
					fmt.Printf("  %s %-40s [%-15s] %s (fix: %s, precision: %s)%s\n", markers, r.ID, r.Category, string(r.Sev), fixLvl, rules.V2RulePrecision(r), stubMarker)
					if r.Description != "" {
						fmt.Printf("    %s\n", r.Description)
					}
				} else {
					fmt.Printf("  %s %-40s [%-15s] %s%s\n", markers, r.ID, r.Category, string(r.Sev), stubMarker)
				}
			} else {
				markers += " "
				if *verboseFlag {
					fmt.Printf("  %s %-40s [%-15s] %s (precision: %s)%s\n", markers, r.ID, r.Category, string(r.Sev), rules.V2RulePrecision(r), stubMarker)
					if r.Description != "" {
						fmt.Printf("    %s\n", r.Description)
					}
				} else {
					fmt.Printf("  %s %-40s [%-15s] %s%s\n", markers, r.ID, r.Category, string(r.Sev), stubMarker)
				}
			}
		}
		implemented := len(v2rules.Registry) - stubs
		if stubs > 0 {
			fmt.Printf("\nTotal: %d rules (%d implemented, %d stubs, %d active by default, %d fixable)\n", len(v2rules.Registry), implemented, stubs, active, fixable)
			fmt.Println("A=active by default, F=fixable, (stub)=placeholder without implementation. Use -v for fix levels, --all-rules to enable all.")
		} else {
			fmt.Printf("\nTotal: %d rules (%d active by default, %d fixable)\n", len(v2rules.Registry), active, fixable)
			fmt.Println("A=active by default, F=fixable. Use -v for fix levels, --all-rules to enable all.")
		}
		os.Exit(0)
	}

	// Parse fix level
	maxFixLevel := rules.FixIdiomatic
	if *fixFlag || *dryRunFlag {
		if parsed, ok := rules.ParseFixLevel(*fixLevelFlag); ok {
			maxFixLevel = parsed
		} else {
			fmt.Fprintf(os.Stderr, "error: invalid fix level '%s'. Use: cosmetic, idiomatic, semantic\n", *fixLevelFlag)
			os.Exit(2)
		}
	}

	paths := flag.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}

	userEnabledExperiments := experiment.ParseCSV(*experimentFlag)
	// Strip (and warn on) any deprecated experiments the user tried to enable.
	filteredUserEnabled := userEnabledExperiments[:0]
	for _, name := range userEnabledExperiments {
		if experiment.IsDeprecated(name) {
			fmt.Fprintf(os.Stderr, "warning: experiment %q is deprecated and will be ignored\n", name)
			continue
		}
		filteredUserEnabled = append(filteredUserEnabled, name)
	}
	enabledExperiments := experiment.MergeEnabled(
		experiment.DefaultEnabled(),
		filteredUserEnabled,
		experiment.ParseCSV(*experimentOffFlag),
	)
	experiment.SetCurrent(enabledExperiments)

	if *experimentMatrixFlag != "" {
		candidates := experiment.ParseCSV(*experimentCandidatesFlag)
		intentCandidates := experiment.NamesForIntent(*experimentIntentFlag)
		if len(intentCandidates) > 0 {
			if len(candidates) == 0 {
				candidates = intentCandidates
			} else {
				allowed := make(map[string]bool, len(intentCandidates))
				for _, name := range intentCandidates {
					allowed[name] = true
				}
				var filtered []string
				for _, name := range candidates {
					if allowed[name] {
						filtered = append(filtered, name)
					}
				}
				candidates = filtered
			}
		}
		if len(candidates) == 0 {
			candidates = sortedDefinitionNames()
		}
		if *experimentRunsFlag < 1 {
			fmt.Fprintf(os.Stderr, "error: --experiment-runs must be >= 1\n")
			os.Exit(2)
		}
		matrixTargets := paths
		if *experimentTargetsFlag != "" {
			matrixTargets = experiment.ParseCSV(*experimentTargetsFlag)
		}
		if len(matrixTargets) == 0 {
			fmt.Fprintf(os.Stderr, "error: experiment matrix needs at least one target path\n")
			os.Exit(2)
		}
		flagArgsForMatrix := append([]string(nil), os.Args[1:len(os.Args)-len(paths)]...)
		code := runExperimentMatrix(matrixRunOptions{
			format:     effectiveFormat,
			outputPath: *outputFlag,
			matrixSpec: *experimentMatrixFlag,
			candidates: candidates,
			runs:       *experimentRunsFlag,
			flagArgs:   flagArgsForMatrix,
			targets:    matrixTargets,
			noCache:    *noMatrixCacheFlag,
			store:      resolvedStore(storeDirFlag),
		})
		os.Exit(code)
	}

	androidProject := android.DetectAndroidProject(paths)

	// Resolve cache directory and file path
	_, cacheFilePath := cache.ResolveCacheDir(*cacheDirFlag, paths)

	// Handle --clear-cache
	if *clearCacheFlag {
		if *cacheDirFlag != "" {
			// Clear all cache files in the shared cache directory
			if err := cache.ClearSharedCache(*cacheDirFlag); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(2)
			}
		} else {
			if err := cache.Clear(cacheFilePath); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(2)
			}
		}
		fmt.Fprintln(os.Stderr, "info: Cache cleared.")
		os.Exit(0)
	}

	start := time.Now()
	tracker := perf.New(*perfFlag)

	// cpuProfileFile is stopped explicitly alongside the memory profile write —
	// defer won't fire through os.Exit, so we manage the lifecycle manually.
	var cpuProfileFile *os.File
	if *cpuProfileFlag != "" {
		f, err := os.Create(*cpuProfileFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: could not create CPU profile: %v\n", err)
			os.Exit(2)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "error: could not start CPU profile: %v\n", err)
			f.Close()
			os.Exit(2)
		}
		cpuProfileFile = f
	}

	// Collect files
	var files []string
	var err error
	tracker.Track("collectFiles", func() error {
		files, err = scanner.CollectKotlinFiles(paths, nil)
		return err
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	if len(files) == 0 && androidProject.IsEmpty() {
		if !*quietFlag {
			fmt.Fprintln(os.Stderr, "info: No Kotlin or Android project files found.")
		}
		os.Exit(0)
	}

	// Build disable/enable sets from CLI flags
	disabledSet := make(map[string]bool)
	enabledSet := make(map[string]bool)
	if *disableRulesFlag != "" {
		for _, name := range strings.Split(*disableRulesFlag, ",") {
			disabledSet[strings.TrimSpace(name)] = true
		}
	}
	if *enableRulesFlag != "" {
		for _, name := range strings.Split(*enableRulesFlag, ",") {
			enabledSet[strings.TrimSpace(name)] = true
		}
	}

	// Filter rules by active status + CLI overrides (native v2 path).
	activeRules := rules.ActiveRulesV2(disabledSet, enabledSet, *allRulesFlag)

	// Create type resolver unless disabled
	var resolver typeinfer.TypeResolver
	if !*noTypeInferFlag {
		resolver = typeinfer.NewResolver()
	}

	// Handle --output-types: run krit-types only, write JSON, exit
	if *outputTypesFlag != "" {
		jarPath := oracle.FindJar(flag.Args())
		if jarPath == "" {
			fmt.Fprintf(os.Stderr, "error: krit-types.jar not found. Build it with: cd tools/krit-types && ./gradlew shadowJar\n")
			os.Exit(2)
		}
		sourceDirs := oracle.FindSourceDirs(flag.Args())
		if len(sourceDirs) == 0 {
			fmt.Fprintf(os.Stderr, "error: no Kotlin source directories found\n")
			os.Exit(2)
		}
		if *verboseFlag {
			fmt.Fprintf(os.Stderr, "verbose: Found %d source directories\n", len(sourceDirs))
		}
		var err error
		// --output-types is a standalone oracle dump: no rules are loaded
		// so there's no rule-classification filter to apply. Pass "" for
		// the filter list path; both paths handle that as "no filter".
		if *noCacheOracleFlag {
			_, err = oracle.Invoke(jarPath, sourceDirs, *outputTypesFlag, *verboseFlag)
		} else {
			_, err = oracle.InvokeCached(jarPath, sourceDirs, oracle.FindRepoDir(flag.Args()), *outputTypesFlag, "", *verboseFlag, resolvedStore(storeDirFlag))
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Type oracle + incremental cache load both run inside IndexPhase.
	// Oracle does type-resolver wrapping (auto-detect / --input-types /
	// --daemon); the cache block loads the JSON, attaches the store, and
	// runs per-file hit/miss lookup. Both operate on the raw file path
	// list so they can run before ParsePhase.
	var daemon *oracle.Daemon
	var analysisCache *cache.Cache
	var cacheResult *cache.CacheResult
	var ruleHash string
	var cacheStats *cache.CacheStats
	useCache := !*noCacheFlag
	{
		oracleIdxInput := pipeline.IndexInput{
			// ParseResult carries only ActiveRules here: IndexPhase runs
			// in oracle-only mode below (SkipModules + SkipAndroid +
			// SkipResolverIndex) so it doesn't need KotlinFiles. Paths
			// and ActiveRules are threaded via the OracleScanPaths and
			// ParseResult.ActiveRules knobs instead.
			ParseResult:     pipeline.ParseResult{ActiveRules: activeRules},
			Logger:          nil, // oracle logs directly to stderr, matching pre-refactor
			Tracker:         tracker,
			OracleEnabled:   resolver != nil && !*noTypeOracleFlag,
			BaseResolver:    resolver,
			OracleScanPaths: flag.Args(),
			KotlinFilePaths: files,
			InputTypesPath:  *inputTypesFlag,
			NoCacheOracle:   *noCacheOracleFlag,
			NoOracleFilter:  *noOracleFilterFlag,
			UseDaemon:       *daemonFlag,
			Store:           resolvedStore(storeDirFlag),
			Verbose:         *verboseFlag,

			CacheEnabled:             useCache,
			CacheFilePath:            cacheFilePath,
			CacheDirExplicit:         *cacheDirFlag != "",
			CacheScanPaths:           paths,
			CacheFilePaths:           files,
			CacheConfig:              cfg,
			CacheEditorConfigEnabled: *editorConfigFlag,
		}
		oracleIdxResult, oracleErr := (pipeline.IndexPhase{
			SkipModules:       true,
			SkipAndroid:       true,
			SkipResolverIndex: true,
		}).Run(context.Background(), oracleIdxInput)
		if oracleErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", oracleErr)
			os.Exit(2)
		}
		if oracleIdxResult.Resolver != nil {
			resolver = oracleIdxResult.Resolver
		}
		if oracleIdxResult.Daemon != nil {
			daemon = oracleIdxResult.Daemon
			defer daemon.Close()
		}
		if useCache {
			analysisCache = oracleIdxResult.Cache
			cacheResult = oracleIdxResult.CacheResult
			ruleHash = oracleIdxResult.RuleHash
			cacheStats = oracleIdxResult.CacheStats
		}
	}

	// Stats come from a throwaway dispatcher so the verbose banner can
	// report per-family rule counts before the phase runs. Construction
	// is side-effect free (beyond classifying rules by capability).
	dispatchCount, aggregateCount, lineCount, crossFileCount, moduleAwareCount, legacyCount := rules.NewDispatcherV2(activeRules, resolver).Stats()

	if *verboseFlag {
		fmt.Fprintf(os.Stderr, "verbose: Found %d Kotlin files\n", len(files))
		if resolver != nil {
			fmt.Fprintf(os.Stderr, "verbose: Type resolver active\n")
		} else {
			fmt.Fprintf(os.Stderr, "verbose: Type resolver disabled\n")
		}
		fmt.Fprintf(os.Stderr, "verbose: Running %d rules with %d workers (%d dispatch, %d aggregate, %d line, %d cross-file, %d module-aware, %d legacy)\n",
			len(activeRules), *jobsFlag, dispatchCount, aggregateCount, lineCount, crossFileCount, moduleAwareCount, legacyCount)
	}

	androidDeps := pipeline.CollectAndroidDependenciesV2(activeRules)
	androidProviders := pipeline.NewAndroidProjectProviders(androidProject, androidDeps, *jobsFlag)

	// Cache load + per-file lookup moved into the IndexPhase call above.
	// analysisCache / cacheResult / ruleHash / cacheStats are populated
	// there; the write-back below still uses them directly.

	// Parse + filter + LPT sort + suppression index now live in ParsePhase.
	// Java collection stays in the cross-file block (SkipJavaCollection=true)
	// so the "javaIndexing" perf label remains nested under "crossFileAnalysis".
	parseWorkers := phaseWorkerCount("parse", *jobsFlag, len(files))
	var verboseLogger func(format string, args ...any)
	if *verboseFlag {
		verboseLogger = func(format string, args ...any) {
			fmt.Fprintf(os.Stderr, format, args...)
		}
	}
	parseResult, err := pipeline.ParsePhase{}.Run(context.Background(), pipeline.ParseInput{
		Config:             cfg,
		Paths:              paths,
		KotlinPaths:        files,
		Workers:            parseWorkers,
		IncludeGenerated:   *includeGeneratedFlag,
		SkipJavaCollection: true,
		Logger:             verboseLogger,
		Tracker:            tracker,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	parsedFiles := parseResult.KotlinFiles
	_ = parseResult.ParseErrors
	parseResult.ActiveRules = activeRules

	hasTypeAwareRule := false
	for _, r := range activeRules {
		if r != nil && r.Needs.Has(v2rules.NeedsResolver) {
			hasTypeAwareRule = true
			break
		}
	}

	// Index all files for type inference (parallel two-phase: index per-file, then merge)
	if resolver != nil && hasTypeAwareRule {
		indexStart := time.Now()
		indexWorkers := phaseWorkerCount("typeIndex", *jobsFlag, len(parsedFiles))
		if indexer, ok := resolver.(interface {
			IndexFilesParallelWithTracker([]*scanner.File, int, perf.Tracker)
		}); ok {
			typeTracker := tracker.Serial("typeIndex")
			indexer.IndexFilesParallelWithTracker(parsedFiles, indexWorkers, typeTracker)
			typeTracker.End()
		} else if indexer, ok := resolver.(interface {
			IndexFilesParallel([]*scanner.File, int)
		}); ok {
			tracker.Track("typeIndex", func() error {
				indexer.IndexFilesParallel(parsedFiles, indexWorkers)
				return nil
			})
		}
		if *verboseFlag {
			fmt.Fprintf(os.Stderr, "verbose: Type-indexed %d files in %v\n",
				len(parsedFiles), time.Since(indexStart).Round(time.Millisecond))
		}
	} else if *verboseFlag && resolver != nil {
		fmt.Fprintln(os.Stderr, "verbose: Skipped type index (no active type-aware rules)")
	}

	// Run per-file rules through DispatchPhase. The phase owns the
	// "ruleExecution" tracker, per-family timing entries, the top-
	// DispatchRules breakdown, cached-findings merge, and cache write-back
	// (as a "cacheSave" sibling entry). main.go still owns the
	// -profile-dispatch reporting because it pulls in CLI-only output.
	ruleStart := time.Now()
	ruleWorkers := phaseWorkerCount("ruleExecution", *jobsFlag, len(parsedFiles))
	dispatchIdx := pipeline.IndexResult{
		ParseResult:            parseResult,
		Resolver:               resolver,
		CacheResult:            cacheResult,
		Cache:                  analysisCache,
		RuleHash:               ruleHash,
		CacheFilePath:          cacheFilePath,
		CacheStats:             cacheStats,
		Logger:                 verboseLogger,
		Tracker:                tracker,
		Jobs:                   *jobsFlag,
		ProfileDispatch:        *profileDispatchFlag,
		Version:                version,
		CacheScanPaths:         paths,
		EmitPerFileStats:       true,
	}
	dispatchResult, err := (pipeline.DispatchPhase{Workers: ruleWorkers}).Run(context.Background(), dispatchIdx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	allFindings := dispatchResult.Findings.Findings()
	if *profileDispatchFlag && len(dispatchResult.FileTimings) > 0 {
		reportDispatchProfile(dispatchResult.FileTimings, ruleWorkers, time.Since(ruleStart))
	}

	// Cross-file analysis (dead code detection) + Module-aware analysis.
	// IndexPhase now owns the Java collection, CodeIndex build, module
	// discovery, and PerModuleIndex construction. Rule execution
	// (crossRules / moduleRuleExecution) stays here because it needs the
	// dispatcher and allFindings accumulator. Main.go creates the
	// "crossFileAnalysis" and "moduleAwareAnalysis" parent trackers so
	// both the indexing children (in IndexPhase) and the rule-execution
	// siblings (below) nest under the same parent, preserving the perf
	// tree byte-for-byte.
	var codeIndex *scanner.CodeIndex
	hasIndexBackedCrossFileRule := false
	hasParsedFilesRule := false
	for _, r := range activeRules {
		if r == nil {
			continue
		}
		if r.Needs.Has(v2rules.NeedsParsedFiles) {
			hasParsedFilesRule = true
			continue
		}
		if r.Needs.Has(v2rules.NeedsCrossFile) {
			hasIndexBackedCrossFileRule = true
		}
	}
	hasModuleAwareRule := false
	for _, r := range activeRules {
		if r != nil && r.Needs.Has(v2rules.NeedsModuleIndex) {
			hasModuleAwareRule = true
			break
		}
	}

	var crossTracker perf.Tracker
	if hasIndexBackedCrossFileRule || hasParsedFilesRule {
		crossTracker = tracker.Serial("crossFileAnalysis")
	}
	moduleTracker := tracker.Serial("moduleAwareAnalysis")

	scanRoot := "."
	if len(paths) > 0 {
		scanRoot = paths[0]
	}

	indexResult2, err := (pipeline.IndexPhase{
		SkipModules:       true,
		SkipAndroid:       true,
		SkipResolverIndex: true,
	}).Run(context.Background(), pipeline.IndexInput{
		ParseResult:            parseResult,
		Logger:                 verboseLogger,
		Tracker:                tracker,
		SkipOracle:             true,
		SkipCache:              true,
		Verbose:                *verboseFlag,
		BuildCodeIndex:         hasIndexBackedCrossFileRule,
		CrossFileParentTracker: crossTracker,
		CrossFileJobsFlag:      *jobsFlag,
		BuildModuleIndex:       true,
		ModuleParentTracker:    moduleTracker,
		ModuleScanRoot:         scanRoot,
		ModuleJobsFlag:         *jobsFlag,
		ModuleHasAwareRule:     hasModuleAwareRule,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	codeIndex = indexResult2.CodeIndex
	parsedJavaFiles := indexResult2.JavaFiles
	moduleGraph := indexResult2.ModuleGraph
	pmi := indexResult2.ModuleIndex

	// Hand off cross-file + module-aware rule execution to CrossFilePhase.
	// The phase owns the crossRuleExecution / crossRules / moduleRuleExecution
	// tracker labels (nested under the crossTracker / moduleTracker parents
	// main.go already created), the verbose "Cross-file analysis in ..." and
	// "Module-aware analysis in ..." log lines, and the unified
	// ApplySuppression pass. Suppression now covers cross-file findings the
	// same way it covers per-file ones (phase-pipeline acceptance #3).
	dispatchForCross := dispatchResult
	dispatchForCross.IndexResult = indexResult2
	dispatchForCross.IndexResult.ActiveRules = activeRules
	dispatchForCross.IndexResult.Logger = verboseLogger
	dispatchForCross.IndexResult.Tracker = tracker
	dispatchForCross.IndexResult.CrossFileParentTracker = crossTracker
	dispatchForCross.IndexResult.ModuleParentTracker = moduleTracker
	dispatchForCross.Findings = scanner.CollectFindings(allFindings)
	crossResult, err := (pipeline.CrossFilePhase{Workers: *jobsFlag}).Run(context.Background(), dispatchForCross)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	if crossTracker != nil {
		crossTracker.End()
	}
	moduleTracker.End()
	_ = parsedJavaFiles
	_ = codeIndex
	_ = moduleGraph
	_ = pmi
	allFindings = crossResult.Findings.Findings()

	if *verboseFlag {
		fmt.Fprintf(os.Stderr, "verbose: Analyzed in %v\n", time.Since(ruleStart).Round(time.Millisecond))
	}

	// Project-level Android analysis: manifest/resource/Gradle/icon files.
	androidStart := time.Now()
	androidTracker := tracker.Serial("androidProjectAnalysis")
	androidDispatcher := rules.NewDispatcherV2(activeRules, resolver)
	androidRes, err := (pipeline.AndroidPhase{}).Run(context.Background(), pipeline.AndroidInput{
		Project:     androidProject,
		ActiveRules: activeRules,
		Dispatcher:  androidDispatcher,
		Providers:   androidProviders,
		Tracker:     androidTracker,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	androidColumns := androidRes.Findings
	androidTracker.End()
	if androidColumns.Len() > 0 {
		allFindings = append(allFindings, androidColumns.Findings()...)
	}
	if *verboseFlag && !androidProject.IsEmpty() {
		fmt.Fprintf(os.Stderr, "verbose: Android project analysis in %v (%d findings across %d manifests, %d res dirs, %d Gradle files)\n",
			time.Since(androidStart).Round(time.Millisecond), androidColumns.Len(),
			len(androidProject.ManifestPaths), len(androidProject.ResDirs), len(androidProject.GradlePaths))
	}

	columns := scanner.CollectFindings(allFindings)
	allColumns := &columns

	// Resolve base path
	basePath := *basePathFlag
	if basePath == "" && len(paths) > 0 {
		basePath, _ = filepath.Abs(paths[0]) // best-effort: error means relative path used
	}

	// Create baseline if requested
	if *createBaselineFlag != "" {
		if err := scanner.WriteBaselineColumns(*createBaselineFlag, allColumns, basePath); err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to write baseline: %v\n", err)
			os.Exit(2)
		}
		if !*quietFlag {
			fmt.Fprintf(os.Stderr, "info: Created baseline with %d issue(s) at %s\n", allColumns.Len(), *createBaselineFlag)
		}
		os.Exit(0)
	}

	if *baselineAuditFlag {
		baselinePath, err := resolveBaselineAuditPath(*baselineFlag, paths)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(2)
		}
		baseline, err := scanner.LoadBaseline(baselinePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to load baseline: %v\n", err)
			os.Exit(2)
		}
		os.Exit(runBaselineAuditColumns(allColumns, baseline, baselinePath, basePath, paths, effectiveFormat))
	}

	// Apply baseline filtering
	if *baselineFlag != "" {
		baseline, err := scanner.LoadBaseline(*baselineFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to load baseline: %v\n", err)
			os.Exit(2)
		}
		beforeCount := allColumns.Len()
		filtered := scanner.FilterColumnsByBaseline(allColumns, baseline, basePath)
		allColumns = &filtered
		if *verboseFlag {
			fmt.Fprintf(os.Stderr, "verbose: Baseline suppressed %d of %d findings\n",
				beforeCount-allColumns.Len(), beforeCount)
		}
	}

	// Filter by git diff (only report findings in changed files)
	if *diffFlag != "" {
		changedFiles, err := getChangedFiles(*diffFlag, paths)
		if err != nil {
			if *verboseFlag {
				fmt.Fprintf(os.Stderr, "verbose: git diff failed: %v (showing all findings)\n", err)
			}
		} else {
			beforeCount := allColumns.Len()
			filtered := scanner.FilterColumnsByFilePaths(allColumns, changedFiles)
			allColumns = &filtered
			if *verboseFlag {
				fmt.Fprintf(os.Stderr, "verbose: --diff %s filtered %d → %d findings (only changed files)\n",
					*diffFlag, beforeCount, allColumns.Len())
			}
		}
	}

	if *removeDeadCodeFlag {
		os.Exit(runDeadCodeRemovalColumns(allColumns, effectiveFormat, *dryRunFlag, *fixSuffix))
	}

	// Apply (or dry-run) fixes if requested via the Fixup pipeline phase.
	if *fixFlag || *dryRunFlag {
		fixRes, _ := (pipeline.FixupPhase{}).Run(context.Background(), pipeline.FixupInput{
			CrossFileResult: pipeline.CrossFileResult{
				DispatchResult: pipeline.DispatchResult{
					Findings: *allColumns,
				},
			},
			Apply:        *fixFlag && !*dryRunFlag,
			ApplyBinary:  *fixBinaryFlag,
			Suffix:       *fixSuffix,
			MaxFixLevel:  maxFixLevel,
			DryRunBinary: *dryRunFlag,
			CountOnly:    *dryRunFlag,
		})
		postColumns := fixRes.Findings
		allColumns = &postColumns

		fixableCount := fixRes.FixableCount
		strippedByLevel := fixRes.StrippedByLevel

		if fixableCount == 0 {
			if !*quietFlag {
				if strippedByLevel > 0 {
					fmt.Fprintf(os.Stderr, "info: No auto-fixable issues at level %s. %d fix(es) available at higher levels (use --fix-level=semantic).\n",
						*fixLevelFlag, strippedByLevel)
				} else {
					fmt.Fprintln(os.Stderr, "info: No auto-fixable issues found.")
				}
			}
		} else if *dryRunFlag {
			// Show what would be fixed
			seen := make(map[string]bool)
			for row := 0; row < allColumns.Len(); row++ {
				if !allColumns.HasFix(row) {
					continue
				}
				file := allColumns.FileAt(row)
				if !seen[file] {
					seen[file] = true
					fmt.Println(file)
				}
			}
			if !*quietFlag {
				fmt.Fprintf(os.Stderr, "info: %d fix(es) available across %d file(s).\n", fixableCount, len(seen))
			}
		} else {
			// Text fix errors: everything in FixErrors minus BinaryErrors.
			binarySet := make(map[error]bool, len(fixRes.BinaryErrors))
			for _, e := range fixRes.BinaryErrors {
				binarySet[e] = true
			}
			for _, e := range fixRes.FixErrors {
				if binarySet[e] {
					continue
				}
				fmt.Fprintf(os.Stderr, "error: %v\n", e)
			}
			if !*quietFlag {
				suffix := "in place"
				if *fixSuffix != "" {
					suffix = "with suffix '" + *fixSuffix + "'"
				}
				fmt.Fprintf(os.Stderr, "info: Applied %d fix(es) across %d file(s) %s in %v.\n",
					fixRes.TextApplied, len(fixRes.ModifiedFiles), suffix, time.Since(start).Round(time.Millisecond))
			}
		}

		// Binary fix reporting
		if *fixBinaryFlag {
			for _, e := range fixRes.BinaryErrors {
				fmt.Fprintf(os.Stderr, "error: binary fix: %v\n", e)
			}
			if fixRes.BinaryApplied > 0 && !*quietFlag {
				mode := "applied"
				if *dryRunFlag {
					mode = "available"
				}
				fmt.Fprintf(os.Stderr, "info: %d binary fix(es) %s.\n", fixRes.BinaryApplied, mode)
			}
		}

		// If --fix was used alone (no explicit output format), exit
		if *outputFlag == "" && effectiveFormat == "json" && *reportFlag == "" && *fixFlag {
			if allColumns.Len()-fixableCount > 0 {
				if !*quietFlag {
					fmt.Fprintf(os.Stderr, "info: %d unfixable issue(s) remain.\n", allColumns.Len()-fixableCount)
				}
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	// Output
	var w *os.File
	if *outputFlag != "" {
		w, err = os.Create(*outputFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(2)
		}
		defer w.Close()
	} else {
		w = os.Stdout
	}

	// Add total timing
	// Record total wall-clock time
	perf.AddEntry(tracker, "total", time.Since(start))

	if cpuProfileFile != nil {
		pprof.StopCPUProfile()
		cpuProfileFile.Close()
	}

	if *memProfileFlag != "" {
		f, err := os.Create(*memProfileFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: could not create memory profile: %v\n", err)
		} else {
			runtime.GC()
			if err := pprof.WriteHeapProfile(f); err != nil {
				fmt.Fprintf(os.Stderr, "error: could not write memory profile: %v\n", err)
			}
			f.Close()
		}
	}

	// Surface Oracle.Stats() hit/miss counters to stderr when --perf is on.
	// This is a diagnostic-only output (no JSON schema change). Counts are
	// incremented atomically during LookupClass/LookupExpression/LookupFunction
	// calls by rules during dispatch.
	if *perfFlag {
		if cr, ok := resolver.(*oracle.CompositeResolver); ok {
			if o, ok := cr.Oracle().(*oracle.Oracle); ok {
				s := o.Stats()
				exprTotal := s.ExprHits + s.ExprMisses
				classTotal := s.ClassHits + s.ClassMisses
				funcTotal := s.FuncHits + s.FuncMisses
				if exprTotal+classTotal+funcTotal > 0 {
					exprRate := 0
					if exprTotal > 0 {
						exprRate = int(s.ExprHits * 100 / exprTotal)
					}
					classRate := 0
					if classTotal > 0 {
						classRate = int(s.ClassHits * 100 / classTotal)
					}
					funcRate := 0
					if funcTotal > 0 {
						funcRate = int(s.FuncHits * 100 / funcTotal)
					}
					fmt.Fprintf(os.Stderr,
						"perf: oracle lookups — expr %d hit / %d miss (%d%%), class %d hit / %d miss (%d%%), func %d hit / %d miss (%d%%)\n",
						s.ExprHits, s.ExprMisses, exprRate,
						s.ClassHits, s.ClassMisses, classRate,
						s.FuncHits, s.FuncMisses, funcRate)
				}
			}
		}
	}

	// Resolve perf timings for JSON output
	var perfTimings []perf.TimingEntry
	if *perfFlag && tracker.IsEnabled() {
		perfTimings = tracker.GetTimings()
	}

	// --sample-rule short-circuits normal output: it prints a deterministic
	// sample of findings for the requested rule and exits with the sampler's
	// own exit code.
	if *sampleRuleFlag != "" {
		os.Exit(runSampleFindingsColumns(allColumns, *sampleRuleFlag, *sampleCountFlag, *sampleContextFlag, basePath))
	}
	// --rule-audit short-circuits normal output: it prints a per-rule
	// audit table and sample details, then exits. Passes the full set of
	// scan paths so multi-target audits can partition findings per repo,
	// and honors -f=json for scripting.
	if *ruleAuditFlag {
		os.Exit(runRuleAuditColumns(allColumns, ruleAuditOpts{
			MinFindings:    *ruleAuditMinFlag,
			DetailRules:    *ruleAuditDetailsFlag,
			SamplesPerRule: *ruleAuditSamplesFlag,
			SampleContext:  *ruleAuditContextFlag,
			ClusterFilter:  *ruleAuditClusterFlag,
			Targets:        paths,
			Format:         effectiveFormat,
		}))
	}

	warningsAsErrors := *warningsAsErrorsFlag || cfg.GetTopLevelBool("warningsAsErrors", false)
	outRes, outErr := (pipeline.OutputPhase{}).Run(context.Background(), pipeline.OutputInput{
		FixupResult: pipeline.FixupResult{
			CrossFileResult: pipeline.CrossFileResult{
				DispatchResult: pipeline.DispatchResult{
					IndexResult: pipeline.IndexResult{
						ParseResult: pipeline.ParseResult{
							KotlinFiles: parsedFiles,
							Paths:       paths,
							ActiveRules: activeRules,
						},
					},
					Findings: *allColumns,
				},
			},
		},
		Writer:           w,
		Format:           effectiveFormat,
		BasePath:         basePath,
		StartTime:        start,
		Version:          version,
		ExperimentNames:  experiment.Current().Names(),
		PerfTimings:      perfTimings,
		CacheStats:       cacheStats,
		WarningsAsErrors: warningsAsErrors,
		MinConfidence:    *minConfidenceFlag,
	})
	if outErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", outErr)
		os.Exit(2)
	}
	finalColumns := outRes.FinalFindings
	allColumns = &finalColumns

	if !*quietFlag {
		findingCount := allColumns.Len()
		fmt.Fprintf(os.Stderr, "info: Found %d issue(s) in %v.\n",
			findingCount, time.Since(start).Round(time.Millisecond))
	}

	if allColumns.Len() > 0 {
		os.Exit(1)
	}
}

func filterFixesByLevelColumns(columns *scanner.FindingColumns, registry []*v2rules.Rule, maxLevel rules.FixLevel) (fixableCount, strippedByLevel int) {
	if columns == nil {
		return 0, 0
	}

	ruleLevels := make(map[string]rules.FixLevel, len(registry))
	for _, r := range registry {
		if r == nil {
			continue
		}
		if lvl, ok := rules.GetV2FixLevel(r); ok {
			ruleLevels[r.ID] = lvl
		}
	}

	strippedByLevel = columns.StripTextFixes(func(row int) bool {
		return ruleLevels[columns.RuleAt(row)] > maxLevel
	})
	return columns.CountTextFixes(), strippedByLevel
}



// fileTiming is an alias for pipeline.FileTiming so profile-dispatch
// reporting can stay in main.go while the phase owns the capture path.
type fileTiming = pipeline.FileTiming

// reportDispatchProfile prints a distribution analysis of per-file dispatch
// timings. Used to diagnose parallelism-collapse on large corpora.
//
// This function is intentionally verbose and noisy — it's debug output behind
// the -profile-dispatch flag. Shipped on oracle-fixes-integration branch only.
func reportDispatchProfile(timings []fileTiming, workers int, wall time.Duration) {
	if len(timings) == 0 {
		return
	}
	n := len(timings)

	// Totals
	var sumRun, sumQueue, sumLock, sumAgg, sumTotal int64
	var maxRun, maxTotal int64
	for _, t := range timings {
		sumRun += t.RunMs
		sumQueue += t.QueueMs
		sumLock += t.LockMs
		sumAgg += t.AggMs
		sumTotal += t.TotalMs
		if t.RunMs > maxRun {
			maxRun = t.RunMs
		}
		if t.TotalMs > maxTotal {
			maxTotal = t.TotalMs
		}
	}

	// Duration distribution (percentiles of runMs)
	runs := make([]int64, n)
	for i, t := range timings {
		runs[i] = t.RunMs
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i] < runs[j] })
	pct := func(p float64) int64 {
		if n == 0 {
			return 0
		}
		idx := int(float64(n-1) * p)
		if idx < 0 {
			idx = 0
		}
		if idx >= n {
			idx = n - 1
		}
		return runs[idx]
	}

	// Top 20 slowest files
	sorted := make([]int, n)
	for i := range sorted {
		sorted[i] = i
	}
	sort.Slice(sorted, func(i, j int) bool {
		return timings[sorted[i]].RunMs > timings[sorted[j]].RunMs
	})

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "=== dispatch profile ===")
	fmt.Fprintf(os.Stderr, "files: %d   workers: %d   wall: %dms\n", n, workers, wall.Milliseconds())
	fmt.Fprintf(os.Stderr, "cumulative runMs: %d   cumulative queueMs: %d   cumulative lockMs: %d   cumulative aggMs: %d   cumulative totalMs: %d\n",
		sumRun, sumQueue, sumLock, sumAgg, sumTotal)
	fmt.Fprintf(os.Stderr, "parallelism (cumRun/wall): %.2fx   ceiling %d\n", float64(sumRun)/float64(wall.Milliseconds()), workers)
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "runMs distribution:")
	fmt.Fprintf(os.Stderr, "  p50=%dms  p75=%dms  p90=%dms  p95=%dms  p99=%dms  p99.9=%dms  max=%dms\n",
		pct(0.50), pct(0.75), pct(0.90), pct(0.95), pct(0.99), pct(0.999), pct(1.0))
	fmt.Fprintf(os.Stderr, "  mean=%dms\n", sumRun/int64(n))
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintf(os.Stderr, "per-file lock-wait (p50/p95/max): %dms / %dms / %dms   (cum %dms)\n",
		percentileInt(lockWaits(timings), 0.50),
		percentileInt(lockWaits(timings), 0.95),
		percentileInt(lockWaits(timings), 1.0),
		sumLock)
	fmt.Fprintf(os.Stderr, "per-file agg-hold  (p50/p95/max): %dms / %dms / %dms   (cum %dms)\n",
		percentileInt(aggHolds(timings), 0.50),
		percentileInt(aggHolds(timings), 0.95),
		percentileInt(aggHolds(timings), 1.0),
		sumAgg)
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "top 20 slowest files by runMs:")
	limit := 20
	if n < limit {
		limit = n
	}
	for i := 0; i < limit; i++ {
		t := timings[sorted[i]]
		fmt.Fprintf(os.Stderr, "  %6dms  %7dkb  %4d findings  %s\n",
			t.RunMs, t.Size/1024, t.Findings, t.Path)
	}
	// Sum of top 20 vs total: what fraction of work comes from the tail?
	var topSum int64
	for i := 0; i < limit; i++ {
		topSum += timings[sorted[i]].RunMs
	}
	fmt.Fprintf(os.Stderr, "  top %d account for %d%% of cumulative runMs\n",
		limit, int(topSum*100/sumRun))
	// How long would dispatch take if we had perfect scheduling (largest-first)?
	// Lower bound = max(cumRun/workers, maxFile)
	perfectWall := sumRun / int64(workers)
	if maxRun > perfectWall {
		perfectWall = maxRun
	}
	fmt.Fprintf(os.Stderr, "lower bound (perfect scheduling): wall = max(cumRun/workers, maxFile) = max(%d, %d) = %dms\n",
		sumRun/int64(workers), maxRun, perfectWall)
	fmt.Fprintln(os.Stderr, "=== end dispatch profile ===")
	fmt.Fprintln(os.Stderr, "")
}

// lockWaits extracts LockMs values for percentile computation.
func lockWaits(timings []fileTiming) []int64 {
	out := make([]int64, len(timings))
	for i, t := range timings {
		out[i] = t.LockMs
	}
	return out
}

// aggHolds extracts AggMs values for percentile computation.
func aggHolds(timings []fileTiming) []int64 {
	out := make([]int64, len(timings))
	for i, t := range timings {
		out[i] = t.AggMs
	}
	return out
}

// percentileInt returns the p-th percentile of a slice of ints (not in-place).
func percentileInt(xs []int64, p float64) int64 {
	if len(xs) == 0 {
		return 0
	}
	sorted := make([]int64, len(xs))
	copy(sorted, xs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * p)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func phaseWorkerCount(phase string, maxWorkers, workItems int) int {
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	if workItems < 1 {
		return 1
	}

	workers := maxWorkers
	if workItems < workers {
		workers = workItems
	}

	var phaseCap int
	switch phase {
	case "moduleAwareAnalysis":
		phaseCap = 8
	case "ruleExecution", "parse", "typeIndex", "crossFileAnalysis":
		phaseCap = 16
	default:
		phaseCap = workers
	}
	if phaseCap < 1 {
		phaseCap = 1
	}
	if workers > phaseCap {
		workers = phaseCap
	}
	return workers
}




func countActiveV2(registry []*v2rules.Rule) int {
	count := 0
	for _, r := range registry {
		if rules.IsDefaultActive(r.ID) {
			count++
		}
	}
	return count
}

// getChangedLines runs git diff and returns a map of absolute file path → set of changed line numbers.
// getChangedFiles returns the set of absolute file paths that have changed
// since the given git ref. Uses git diff --name-only for robust file discovery.
func getChangedFiles(ref string, scanPaths []string) (map[string]bool, error) {
	// Get changed files: staged + unstaged modifications since ref
	args := []string{"diff", "--name-only", "--diff-filter=ACMR", ref, "--"}
	for _, p := range scanPaths {
		args = append(args, p)
	}
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s: %w", ref, err)
	}

	result := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		absPath, _ := filepath.Abs(line)
		if absPath != "" {
			result[absPath] = true
		}
	}
	return result, nil
}

