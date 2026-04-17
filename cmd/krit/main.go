package main

import (
	"embed"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"encoding/json"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/experiment"
	"github.com/kaeawc/krit/internal/fixer"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/output"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/rules"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/schema"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// version is set by goreleaser via ldflags: -X main.version=...
var version = "dev"

// Local structural interfaces used to detect Android manifest/resource/Gradle
// rules without referencing the named v1 family interfaces in rules/. These
// mirror the manifest/resource/gradle rule contracts so the main binary can
// iterate them without importing the legacy family interface names.
type manifestRuleIface interface {
	rules.Rule
	CheckManifest(m *rules.Manifest) []scanner.Finding
}

type resourceRuleIface interface {
	rules.Rule
	CheckResources(idx *android.ResourceIndex) []scanner.Finding
}

type gradleRuleIface interface {
	rules.Rule
	CheckGradle(path string, content string, cfg *android.BuildConfig) []scanner.Finding
}

//go:embed completions
var completionsFS embed.FS

func main() {
	// Bridge any v2 rules into the v1 Registry before anything reads it.
	rules.RegisterV2Rules()

	baselineAuditVerb := len(os.Args) > 1 && os.Args[1] == "baseline-audit"
	harvestVerb := len(os.Args) > 1 && os.Args[1] == "harvest"
	renameVerb := len(os.Args) > 1 && os.Args[1] == "rename"
	initVerb := len(os.Args) > 1 && os.Args[1] == "init"
	apiSnapshotVerb := len(os.Args) > 1 && os.Args[1] == "api-snapshot"
	apiDiffVerb := len(os.Args) > 1 && os.Args[1] == "api-diff"
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
		fmt.Printf("  rules: %d registered (%d active by default)\n", len(rules.Registry), countActive(rules.Registry))
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
		for _, r := range rules.Registry {
			markers := ""
			if rules.IsDefaultActive(r.Name()) {
				markers += "A"
				active++
			} else {
				markers += " "
			}
			implemented := rules.IsImplemented(r)
			if !implemented {
				stubs++
			}
			stubMarker := ""
			if !implemented {
				stubMarker = " (stub)"
			}
			if fr, ok := r.(rules.FixableRule); ok && fr.IsFixable() {
				markers += "F"
				fixable++
				if *verboseFlag {
					level := rules.GetFixLevel(fr)
					fmt.Printf("  %s %-40s [%-15s] %s (fix: %s, precision: %s)%s\n", markers, r.Name(), r.RuleSet(), r.Severity(), level, rules.RulePrecision(r), stubMarker)
					if desc := r.Description(); desc != "" {
						fmt.Printf("    %s\n", desc)
					}
				} else {
					fmt.Printf("  %s %-40s [%-15s] %s%s\n", markers, r.Name(), r.RuleSet(), r.Severity(), stubMarker)
				}
			} else {
				markers += " "
				if *verboseFlag {
					fmt.Printf("  %s %-40s [%-15s] %s (precision: %s)%s\n", markers, r.Name(), r.RuleSet(), r.Severity(), rules.RulePrecision(r), stubMarker)
					if desc := r.Description(); desc != "" {
						fmt.Printf("    %s\n", desc)
					}
				} else {
					fmt.Printf("  %s %-40s [%-15s] %s%s\n", markers, r.Name(), r.RuleSet(), r.Severity(), stubMarker)
				}
			}
		}
		implemented := len(rules.Registry) - stubs
		if stubs > 0 {
			fmt.Printf("\nTotal: %d rules (%d implemented, %d stubs, %d active by default, %d fixable)\n", len(rules.Registry), implemented, stubs, active, fixable)
			fmt.Println("A=active by default, F=fixable, (stub)=placeholder without implementation. Use -v for fix levels, --all-rules to enable all.")
		} else {
			fmt.Printf("\nTotal: %d rules (%d active by default, %d fixable)\n", len(rules.Registry), active, fixable)
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

	// Filter rules by active status + CLI overrides
	var activeRules []rules.Rule
	for _, r := range rules.Registry {
		name := r.Name()
		if disabledSet[name] {
			continue // explicitly disabled via --disable-rules
		}
		if enabledSet[name] || *allRulesFlag || rules.IsDefaultActive(name) {
			activeRules = append(activeRules, r)
		}
	}

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
			_, err = oracle.InvokeCached(jarPath, sourceDirs, oracle.FindRepoDir(flag.Args()), *outputTypesFlag, "", *verboseFlag)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Type oracle: auto-detect, --input-types, --daemon, or --no-type-oracle
	var daemon *oracle.Daemon
	if resolver != nil && !*noTypeOracleFlag {
		oracleTracker := tracker.Serial("typeOracle")

		if *daemonFlag {
			// Daemon mode: start a long-lived JVM process
			var d *oracle.Daemon
			var daemonErr error
			oracleTracker.Track("jvmStart", func() error {
				d, daemonErr = oracle.InvokeDaemon(flag.Args(), *verboseFlag)
				return daemonErr
			})
			if daemonErr != nil {
				fmt.Fprintf(os.Stderr, "warning: daemon: %v\n", daemonErr)
			} else {
				daemon = d
				defer daemon.Close()

				var oracleData *oracle.OracleData
				oracleTracker.Track("jvmAnalyze", func() error {
					od, err := daemon.AnalyzeAll()
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: daemon analyzeAll: %v\n", err)
						return err
					}
					oracleData = od
					return nil
				})
				if oracleData != nil {
					var oracleLoaded *oracle.Oracle
					oracleTracker.Track("indexBuild", func() error {
						ol, err := oracle.LoadFromData(oracleData)
						if err != nil {
							fmt.Fprintf(os.Stderr, "warning: daemon oracle: %v\n", err)
							return err
						}
						oracleLoaded = ol
						return nil
					})
					if oracleLoaded != nil {
						resolver = oracle.NewCompositeResolver(oracleLoaded, resolver)
						if *verboseFlag {
							depCount := len(oracleLoaded.Dependencies())
							fmt.Fprintf(os.Stderr, "verbose: Type oracle loaded from daemon (%d dependency types)\n", depCount)
						}
					}
				}
			}
		} else {
			var oraclePath string

			oracleTracker.Track("findSources", func() error {
				if *inputTypesFlag != "" {
					// Explicit input path
					oraclePath = *inputTypesFlag
					return nil
				}
				// Auto-detect: try cached types.json
				cached := oracle.CachePath(flag.Args())
				if cached != "" {
					if _, err := os.Stat(cached); err == nil {
						oraclePath = cached
					}
				}
				return nil
			})

			// If no cached oracle, try to run krit-types automatically
			if oraclePath == "" {
				oracleTracker.Track("jvmAnalyze", func() error {
					jarPath := oracle.FindJar(flag.Args())
					if jarPath == "" {
						return nil
					}
					sourceDirs := oracle.FindSourceDirs(flag.Args())
					if len(sourceDirs) == 0 {
						return nil
					}
					cacheDest := oracle.CachePath(flag.Args())
					if cacheDest == "" {
						cacheDest = filepath.Join(os.TempDir(), "krit-types.json")
					}
					if *verboseFlag {
						fmt.Fprintf(os.Stderr, "verbose: Running krit-types (%d source dirs)...\n", len(sourceDirs))
					}
					// Pre-scan step: compute the subset of files any enabled
					// rule has declared (via OracleFilter) it actually needs
					// oracle access on. Files where no filter matches are
					// tree-sitter-sufficient and can be dropped from the
					// krit-types analyze loop. Unclassified rules fall
					// through to AllFiles: true, so this is a no-op until
					// a meaningful fraction of rules has been audited. Guard
					// with -no-oracle-filter for diagnostics / baseline
					// reproduction.
					var filterListPath string
					if !*noOracleFilterFlag {
						filterRules := rules.BuildOracleFilterRules(activeRules)
						lightFiles := loadFilesForOracleFilter(files)
						summary := oracle.CollectOracleFiles(filterRules, lightFiles)
						if *verboseFlag {
							switch {
							case summary.AllFiles:
								fmt.Fprintf(os.Stderr, "verbose: Oracle filter: %d/%d files (AllFiles short-circuit — no reduction)\n",
									summary.MarkedFiles, summary.TotalFiles)
							case summary.MarkedFiles == summary.TotalFiles:
								fmt.Fprintf(os.Stderr, "verbose: Oracle filter: %d/%d files (no reduction)\n",
									summary.MarkedFiles, summary.TotalFiles)
							default:
								fmt.Fprintf(os.Stderr, "verbose: Oracle filter: %d/%d files (%.1f%% of corpus)\n",
									summary.MarkedFiles, summary.TotalFiles,
									100*float64(summary.MarkedFiles)/float64(maxInt(summary.TotalFiles, 1)))
							}
						}
						// Skip the filter entirely if it doesn't reduce the
						// file set (no benefit, just overhead of writing the
						// temp file and a krit-types flag).
						if !summary.AllFiles && summary.MarkedFiles < summary.TotalFiles {
							p, werr := oracle.WriteFilterListFile(summary, "")
							if werr != nil {
								fmt.Fprintf(os.Stderr, "warning: oracle filter list: %v\n", werr)
							} else if p != "" {
								filterListPath = p
								defer os.Remove(p)
							}
						}
					}

					// Route to cache-aware path unless the user opted out.
					// Both paths accept filterListPath so rule filtering and
					// per-file caching compose: the filter narrows the
					// universe first, then the cache classifies what's left.
					var result string
					var err error
					if *noCacheOracleFlag {
						result, err = oracle.InvokeWithFiles(jarPath, sourceDirs, cacheDest, filterListPath, *verboseFlag)
					} else {
						result, err = oracle.InvokeCached(jarPath, sourceDirs, oracle.FindRepoDir(flag.Args()), cacheDest, filterListPath, *verboseFlag)
					}
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: krit-types: %v\n", err)
						return nil
					}
					oraclePath = result
					return nil
				})
			}

			if oraclePath != "" {
				var oracleData *oracle.Oracle
				oracleTracker.Track("jsonLoad", func() error {
					od, err := oracle.Load(oraclePath)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: type oracle: %v\n", err)
						return err
					}
					oracleData = od
					return nil
				})
				if oracleData != nil {
					resolver = oracle.NewCompositeResolver(oracleData, resolver)
					if *verboseFlag {
						depCount := len(oracleData.Dependencies())
						fmt.Fprintf(os.Stderr, "verbose: Type oracle loaded from %s (%d dependency types)\n", oraclePath, depCount)
					}
				}
			}
		}

		oracleTracker.End()
	}

	// Build dispatcher for single-pass execution
	dispatcher := rules.NewDispatcher(activeRules, resolver)
	dispatchCount, aggregateCount, lineCount, crossFileCount, moduleAwareCount, legacyCount := dispatcher.Stats()

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

	androidDeps := collectAndroidDependencies(activeRules)
	androidProviders := newAndroidProjectProviders(androidProject, androidDeps, *jobsFlag)

	// Load cache and determine which files can be skipped
	var analysisCache *cache.Cache
	var cacheResult *cache.CacheResult
	var ruleHash string
	var cacheStats *cache.CacheStats
	useCache := !*noCacheFlag

	if useCache {
		ruleNames := make([]string, len(activeRules))
		for i, r := range activeRules {
			ruleNames[i] = r.Name()
		}
		ruleHash = cache.ComputeConfigHash(ruleNames, cfg, *editorConfigFlag)

		var loadStart time.Time
		tracker.Track("cacheLoad", func() error {
			loadStart = time.Now()
			analysisCache = cache.Load(cacheFilePath)
			return nil
		})
		loadDur := time.Since(loadStart).Milliseconds()

		cacheResult = analysisCache.CheckFiles(files, ruleHash, paths...)

		cacheStats = &cache.CacheStats{
			Cached:    cacheResult.TotalCached,
			Total:     cacheResult.TotalFiles,
			LoadDurMs: loadDur,
		}
		if cacheResult.TotalFiles > 0 {
			cacheStats.HitRate = float64(cacheResult.TotalCached) / float64(cacheResult.TotalFiles)
		}

		if *verboseFlag && cacheResult.TotalCached > 0 {
			pct := 100 * cacheResult.TotalCached / cacheResult.TotalFiles
			fmt.Fprintf(os.Stderr, "verbose: Cache: %d/%d files cached (%d%% hit rate)\n",
				cacheResult.TotalCached, cacheResult.TotalFiles, pct)
			if *cacheDirFlag != "" {
				fmt.Fprintf(os.Stderr, "verbose: Cache file: %s\n", cacheFilePath)
			}
		}
	}

	// Parse all files in parallel (all files needed for cross-file indexing)
	parseStart := time.Now()
	var parsedFiles []*scanner.File
	var parseErrs []error
	parseWorkers := phaseWorkerCount("parse", *jobsFlag, len(files))
	tracker.Track("parse", func() error {
		parsedFiles, parseErrs = scanner.ScanFiles(files, parseWorkers)
		return nil
	})
	if *verboseFlag {
		fmt.Fprintf(os.Stderr, "verbose: Parsed %d files in %v (%d errors, %d workers)\n",
			len(parsedFiles), time.Since(parseStart).Round(time.Millisecond), len(parseErrs), parseWorkers)
	}

	// Filter out generated files unless --include-generated was passed.
	// Generated directories (`*/generated/*`) contain codegen output that users
	// don't maintain and typically dwarfs hand-written sources in size. On
	// kotlinlang.org/kotlin itself a single generated file (stdlib _Arrays.kt,
	// ~814KB with thousands of array-extension functions) consumes ~87% of
	// total dispatch wall time, stranding 15 of 16 workers waiting for it to
	// finish. Skipping generated dirs is consistent with krit's existing skip
	// list for vendor/third-party/build output.
	//
	// This filter runs BEFORE typeIndex so that both typeIndex and
	// ruleExecution skip generated files — otherwise typeIndex still pays
	// the full cost of parsing thousands of generated declarations.
	if !*includeGeneratedFlag {
		filtered := parsedFiles[:0]
		var droppedGenerated int
		for _, f := range parsedFiles {
			if strings.Contains(f.Path, "/generated/") {
				droppedGenerated++
				continue
			}
			filtered = append(filtered, f)
		}
		parsedFiles = filtered
		if *verboseFlag && droppedGenerated > 0 {
			fmt.Fprintf(os.Stderr, "verbose: Skipped %d files in */generated/* dirs (pass --include-generated to re-enable)\n", droppedGenerated)
		}
	}

	// Sort files by content size descending (LPT — longest processing time
	// first scheduling). For long-tailed corpora, having workers pick up the
	// largest files first prevents them from draining the small-file queue and
	// stranding idle while one worker chews on a giant file. Applied before
	// typeIndex so that phase also benefits. O(N log N) sort on parsedFiles;
	// cost is negligible vs the dispatch work.
	sort.Slice(parsedFiles, func(i, j int) bool {
		return len(parsedFiles[i].Content) > len(parsedFiles[j].Content)
	})

	// Build each file's SuppressionIndex once here so both per-file
	// dispatch and cross-file/module-aware rules (which flow through
	// pipeline.ApplySuppression below) consult the same @Suppress map.
	// Prior to this, cross-file rules bypassed the suppression index
	// entirely — see roadmap/clusters/core-infra/phase-pipeline.md
	// acceptance criterion #3.
	for _, f := range parsedFiles {
		if f.FlatTree == nil {
			continue
		}
		f.SuppressionIdx = scanner.BuildSuppressionIndexFlat(f.FlatTree, f.Content)
	}

	hasTypeAwareRule := false
	for _, rule := range activeRules {
		if _, ok := rule.(interface {
			SetResolver(resolver typeinfer.TypeResolver)
		}); ok {
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

	// Run per-file rules only on uncached files
	ruleStart := time.Now()

	// Per-file dispatch timing instrumentation (opt-in via -profile-dispatch).
	// Used to investigate parallelism-collapse hypotheses on large corpora.
	var fileTimings []fileTiming

	var (
		mu             sync.Mutex
		allFindings    []scanner.Finding
		findingsByFile = make(map[string]scanner.FindingColumns)
		wg             sync.WaitGroup
		ruleWorkers    = phaseWorkerCount("ruleExecution", *jobsFlag, len(parsedFiles))
		sem            = make(chan struct{}, ruleWorkers)
		ruleStats      = rules.RunStats{DispatchRuleNsByRule: make(map[string]int64)}
	)

	ruleTracker := tracker.Serial("ruleExecution")
	for _, f := range parsedFiles {
		// Skip rule execution for cached files
		if useCache && cacheResult != nil && cacheResult.CachedPaths[f.Path] {
			continue
		}
		wg.Add(1)
		var dispatchedAt time.Time
		if *profileDispatchFlag {
			dispatchedAt = time.Now()
		}
		sem <- struct{}{}
		go func(file *scanner.File, dispatched time.Time) {
			defer wg.Done()
			defer func() { <-sem }()

			var startedAt, finishedRunAt, lockedAt time.Time
			if *profileDispatchFlag {
				startedAt = time.Now()
			}

			fileFindings, fileStats := dispatcher.RunWithStats(file)

			if *profileDispatchFlag {
				finishedRunAt = time.Now()
			}

			mu.Lock()

			if *profileDispatchFlag {
				lockedAt = time.Now()
			}

			if len(fileFindings) > 0 {
				allFindings = append(allFindings, fileFindings...)
			}
			if useCache && analysisCache != nil {
				findingsByFile[file.Path] = scanner.CollectFindings(fileFindings)
			}
			ruleStats.SuppressionIndexMs += fileStats.SuppressionIndexMs
			ruleStats.DispatchWalkMs += fileStats.DispatchWalkMs
			ruleStats.DispatchRuleNs += fileStats.DispatchRuleNs
			ruleStats.AggregateCollectNs += fileStats.AggregateCollectNs
			ruleStats.AggregateFinalizeMs += fileStats.AggregateFinalizeMs
			ruleStats.LineRuleMs += fileStats.LineRuleMs
			ruleStats.LegacyRuleMs += fileStats.LegacyRuleMs
			ruleStats.SuppressionFilterMs += fileStats.SuppressionFilterMs
			for name, dur := range fileStats.DispatchRuleNsByRule {
				ruleStats.DispatchRuleNsByRule[name] += dur
			}
			if len(fileStats.Errors) > 0 {
				ruleStats.Errors = append(ruleStats.Errors, fileStats.Errors...)
			}

			if *profileDispatchFlag {
				endAt := time.Now()
				fileTimings = append(fileTimings, fileTiming{
					path:     file.Path,
					size:     len(file.Content),
					queueMs:  startedAt.Sub(dispatched).Milliseconds(),
					runMs:    finishedRunAt.Sub(startedAt).Milliseconds(),
					lockMs:   lockedAt.Sub(finishedRunAt).Milliseconds(),
					aggMs:    endAt.Sub(lockedAt).Milliseconds(),
					totalMs:  endAt.Sub(dispatched).Milliseconds(),
					findings: len(fileFindings),
				})
			}

			mu.Unlock()
		}(f, dispatchedAt)
	}
	wg.Wait()

	if *profileDispatchFlag && len(fileTimings) > 0 {
		reportDispatchProfile(fileTimings, ruleWorkers, time.Since(ruleStart))
	}
	if len(ruleStats.Errors) > 0 {
		for _, de := range ruleStats.Errors {
			fmt.Fprintln(os.Stderr, de.Error())
		}
		fmt.Fprintf(os.Stderr, "krit: %d rule panic(s) during scan\n", len(ruleStats.Errors))
	}

	perf.AddEntry(ruleTracker, "suppressionIndex", time.Duration(ruleStats.SuppressionIndexMs)*time.Millisecond)
	perf.AddEntry(ruleTracker, "dispatchWalk", time.Duration(ruleStats.DispatchWalkMs)*time.Millisecond)
	perf.AddEntry(ruleTracker, "dispatchRuleCallbacks", time.Duration(ruleStats.DispatchRuleNs))
	perf.AddEntry(ruleTracker, "aggregateCollect", time.Duration(ruleStats.AggregateCollectNs))
	perf.AddEntry(ruleTracker, "aggregateFinalize", time.Duration(ruleStats.AggregateFinalizeMs)*time.Millisecond)
	perf.AddEntry(ruleTracker, "lineRules", time.Duration(ruleStats.LineRuleMs)*time.Millisecond)
	perf.AddEntry(ruleTracker, "legacyRules", time.Duration(ruleStats.LegacyRuleMs)*time.Millisecond)
	perf.AddEntry(ruleTracker, "suppressionFilter", time.Duration(ruleStats.SuppressionFilterMs)*time.Millisecond)
	if len(ruleStats.DispatchRuleNsByRule) > 0 {
		type timedRule struct {
			name string
			dur  int64
		}
		var topRules []timedRule
		for name, dur := range ruleStats.DispatchRuleNsByRule {
			topRules = append(topRules, timedRule{name: name, dur: dur})
		}
		sort.Slice(topRules, func(i, j int) bool {
			if topRules[i].dur == topRules[j].dur {
				return topRules[i].name < topRules[j].name
			}
			return topRules[i].dur > topRules[j].dur
		})
		if len(topRules) > 10 {
			topRules = topRules[:10]
		}
		topDispatchTracker := ruleTracker.Serial("topDispatchRules")
		for _, tr := range topRules {
			perf.AddEntry(topDispatchTracker, tr.name, time.Duration(tr.dur))
		}
		topDispatchTracker.End()
	}
	ruleTracker.End()

	// Merge cached findings
	if useCache && cacheResult != nil && cacheResult.CachedColumns.Len() > 0 {
		allFindings = append(allFindings, cacheResult.CachedColumns.Findings()...)
	}

	// Update cache with new analysis results
	if useCache && analysisCache != nil {
		// Update entries for newly analyzed files
		for _, pf := range parsedFiles {
			if cacheResult == nil || !cacheResult.CachedPaths[pf.Path] {
				fileColumns := findingsByFile[pf.Path]
				analysisCache.UpdateEntryColumns(pf.Path, &fileColumns)
			}
		}
		analysisCache.Version = version
		analysisCache.RuleHash = ruleHash
		analysisCache.ScanPaths = paths
		analysisCache.Prune()

		var saveStart time.Time
		tracker.Track("cacheSave", func() error {
			saveStart = time.Now()
			if err := analysisCache.Save(cacheFilePath); err != nil {
				fmt.Fprintf(os.Stderr, "warning: Failed to save cache: %v\n", err)
			}
			return nil
		})
		if cacheStats != nil {
			cacheStats.SaveDurMs = time.Since(saveStart).Milliseconds()
		}
	}

	// Cross-file analysis (dead code detection)
	// Also index Java files for references — Kotlin calls Java and Java calls Kotlin
	var codeIndex *scanner.CodeIndex
	hasIndexBackedCrossFileRule := false
	hasParsedFilesRule := false
	for _, rule := range activeRules {
		if _, ok := rule.(interface {
			CheckParsedFiles(files []*scanner.File) []scanner.Finding
		}); ok {
			hasParsedFilesRule = true
			continue
		}
		if _, ok := rule.(interface {
			CheckCrossFile(index *scanner.CodeIndex) []scanner.Finding
		}); ok {
			hasIndexBackedCrossFileRule = true
		}
	}
	if hasIndexBackedCrossFileRule || hasParsedFilesRule {
		crossStart := time.Now()
		var javaFilePaths []string
		var parsedJavaFiles []*scanner.File
		crossWorkers := phaseWorkerCount("crossFileAnalysis", *jobsFlag, len(parsedFiles))
		crossTracker := tracker.Serial("crossFileAnalysis")

		if hasIndexBackedCrossFileRule {
			_ = crossTracker.Track("javaIndexing", func() error {
				javaFilePaths, err = scanner.CollectJavaFiles(paths, nil) // err non-fatal: Java indexing is best-effort
				if err != nil && *verboseFlag {
					fmt.Fprintf(os.Stderr, "verbose: Java file collection: %v\n", err)
				}
				if len(javaFilePaths) > 0 {
					crossWorkers = phaseWorkerCount("crossFileAnalysis", *jobsFlag, len(parsedFiles)+len(javaFilePaths))
					var javaErrs []error
					parsedJavaFiles, javaErrs = scanner.ScanJavaFiles(javaFilePaths, crossWorkers)
					if len(javaErrs) > 0 && *verboseFlag {
						fmt.Fprintf(os.Stderr, "verbose: Java file parsing: %d errors\n", len(javaErrs))
					}
					if *verboseFlag {
						fmt.Fprintf(os.Stderr, "verbose: Parsed %d Java files for cross-reference indexing\n", len(parsedJavaFiles))
					}
				}
				return nil
			})

			_ = crossTracker.Track("codeIndexBuild", func() error {
				indexTracker := crossTracker.Serial("indexBuild")
				codeIndex = scanner.BuildIndexWithTracker(parsedFiles, crossWorkers, indexTracker, parsedJavaFiles...)
				indexTracker.End()
				return nil
			})
		}

		_ = crossTracker.Track("crossRuleExecution", func() error {
			ruleTracker := crossTracker.Serial("crossRules")
			for _, rule := range activeRules {
				if pfr, ok := rule.(interface {
					CheckParsedFiles(files []*scanner.File) []scanner.Finding
				}); ok {
					ruleName := rule.Name()
					_ = ruleTracker.Track(ruleName, func() error {
						findings := pfr.CheckParsedFiles(parsedFiles)
						rules.ApplyRuleConfidence(findings, rule, 0.95)
						allFindings = append(allFindings, findings...)
						return nil
					})
				} else if cfr, ok := rule.(interface {
					CheckCrossFile(index *scanner.CodeIndex) []scanner.Finding
				}); ok {
					ruleName := rule.Name()
					_ = ruleTracker.Track(ruleName, func() error {
						findings := cfr.CheckCrossFile(codeIndex)
						rules.ApplyRuleConfidence(findings, rule, 0.95)
						allFindings = append(allFindings, findings...)
						return nil
					})
				}
			}
			ruleTracker.End()
			return nil
		})
		crossTracker.End()
		if *verboseFlag {
			if codeIndex != nil {
				fmt.Fprintf(os.Stderr, "verbose: Cross-file analysis in %v (indexed %d symbols, %d references from %d kt + %d java files)\n",
					time.Since(crossStart).Round(time.Millisecond), len(codeIndex.Symbols), len(codeIndex.References),
					len(parsedFiles), len(parsedJavaFiles))
			} else {
				fmt.Fprintf(os.Stderr, "verbose: Cross-file analysis in %v (%d kt files, no shared code index needed)\n",
					time.Since(crossStart).Round(time.Millisecond), len(parsedFiles))
			}
		}
	} else if *verboseFlag {
		fmt.Fprintln(os.Stderr, "verbose: Skipped cross-file analysis (no active cross-file rules)")
	}

	// Module-aware analysis: auto-detect Gradle modules and run module-aware rule implementations
	{
		moduleStart := time.Now()
		moduleTracker := tracker.Serial("moduleAwareAnalysis")
		scanRoot := "."
		if len(paths) > 0 {
			scanRoot = paths[0]
		}
		var (
			graph  *module.ModuleGraph
			modErr error
		)
		_ = moduleTracker.Track("moduleDiscovery", func() error {
			graph, modErr = module.DiscoverModules(scanRoot)
			return nil
		})
		if modErr != nil && *verboseFlag {
			fmt.Fprintf(os.Stderr, "verbose: Module discovery error: %v\n", modErr)
		}
		hasModuleAwareRule := false
		for _, rule := range activeRules {
			if _, ok := rule.(interface {
				SetModuleIndex(pmi *module.PerModuleIndex)
				CheckModuleAware() []scanner.Finding
			}); ok {
				hasModuleAwareRule = true
				break
			}
		}
		if graph != nil && len(graph.Modules) > 0 && hasModuleAwareRule {
			moduleNeeds := rules.CollectModuleAwareNeeds(activeRules)
			moduleWorkers := phaseWorkerCount("moduleAwareAnalysis", *jobsFlag, len(graph.Modules))

			var pmi *module.PerModuleIndex
			if moduleNeeds.NeedsDependencies {
				_ = moduleTracker.Track("moduleDependencies", func() error {
					if err := module.ParseAllDependencies(graph); err != nil {
						if *verboseFlag {
							fmt.Fprintf(os.Stderr, "verbose: Module dependency parse error: %v\n", err)
						}
					}
					return nil
				})
			}
			if *verboseFlag {
				fmt.Fprintf(os.Stderr, "verbose: Detected %d Gradle modules\n", len(graph.Modules))
			}

			_ = moduleTracker.Track("moduleIndexBuild", func() error {
				pmi = &module.PerModuleIndex{Graph: graph}
				switch {
				case moduleNeeds.NeedsIndex:
					pmi = module.BuildPerModuleIndexWithGlobal(graph, parsedFiles, moduleWorkers, codeIndex)
				case moduleNeeds.NeedsFiles:
					pmi.ModuleFiles = module.GroupFilesByModule(graph, parsedFiles)
				}
				return nil
			})

			_ = moduleTracker.Track("moduleRuleExecution", func() error {
				// The v2 bridge threads ModuleIndex through the rule's Context
				// automatically, so no explicit SetModuleIndex call is required.
				for _, r := range dispatcher.V2Rules().ModuleAware {
					ctx := &v2.Context{ModuleIndex: pmi}
					r.Check(ctx)
					rules.ApplyV2Confidence(ctx.Findings, r, 0.95)
					allFindings = append(allFindings, ctx.Findings...)
				}
				return nil
			})

			if *verboseFlag {
				fmt.Fprintf(os.Stderr, "verbose: Module-aware analysis in %v\n",
					time.Since(moduleStart).Round(time.Millisecond))
			}
		}
		moduleTracker.End()
	}

	// Apply per-file @Suppress to findings produced by cross-file and
	// module-aware rules. The per-file dispatcher already honours
	// @Suppress for its own findings (via v2dispatcher.go); before this
	// refactor, findings produced by CheckCrossFile / CheckParsedFiles /
	// CheckModuleAware bypassed suppression entirely. See
	// roadmap/clusters/core-infra/phase-pipeline.md acceptance criterion #3.
	allFindings = pipeline.ApplySuppression(allFindings, parsedFiles)

	if *verboseFlag {
		fmt.Fprintf(os.Stderr, "verbose: Analyzed in %v\n", time.Since(ruleStart).Round(time.Millisecond))
	}

	// Project-level Android analysis: manifest/resource/Gradle/icon files.
	androidStart := time.Now()
	androidTracker := tracker.Serial("androidProjectAnalysis")
	androidColumns := runAndroidProjectAnalysisColumns(androidProject, activeRules, androidTracker, androidProviders)
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

	// Filter fixes by level and count
	fixableCount := 0
	strippedByLevel := 0
	if *fixFlag || *dryRunFlag {
		fixableCount, strippedByLevel = filterFixesByLevelColumns(allColumns, rules.Registry, maxFixLevel)
	}

	// Apply fixes if requested
	if *fixFlag || *dryRunFlag {
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
			totalFixes, filesModified, fixErrs := fixer.ApplyAllFixesColumns(allColumns, *fixSuffix)
			for _, e := range fixErrs {
				fmt.Fprintf(os.Stderr, "error: %v\n", e)
			}
			if !*quietFlag {
				suffix := "in place"
				if *fixSuffix != "" {
					suffix = "with suffix '" + *fixSuffix + "'"
				}
				fmt.Fprintf(os.Stderr, "info: Applied %d fix(es) across %d file(s) %s in %v.\n",
					totalFixes, filesModified, suffix, time.Since(start).Round(time.Millisecond))
			}
		}

		// Apply binary fixes if requested
		if *fixBinaryFlag {
			binaryApplied, binaryErrs := fixer.ApplyBinaryFixesBatchColumns(allColumns, *dryRunFlag)
			for _, e := range binaryErrs {
				fmt.Fprintf(os.Stderr, "error: binary fix: %v\n", e)
			}
			if binaryApplied > 0 && !*quietFlag {
				mode := "applied"
				if *dryRunFlag {
					mode = "available"
				}
				fmt.Fprintf(os.Stderr, "info: %d binary fix(es) %s.\n", binaryApplied, mode)
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

	// Elevate warnings to errors before output if requested
	warningsAsErrors := *warningsAsErrorsFlag || cfg.GetTopLevelBool("warningsAsErrors", false)
	if warningsAsErrors {
		allColumns.PromoteWarningsToErrors()
	}

	// Drop findings below the configured confidence threshold, if any.
	if *minConfidenceFlag > 0 {
		filtered := allColumns.FilterByMinConfidence(*minConfidenceFlag)
		allColumns = &filtered
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

	var formatErr error
	switch effectiveFormat {
	case "json":
		formatErr = output.FormatJSONColumns(w, allColumns, version, len(parsedFiles), len(activeRules),
			start, perfTimings, activeRules, experiment.Current().Names(), cacheStats)
	case "plain":
		output.FormatPlainColumns(w, allColumns)
	case "sarif":
		formatErr = output.FormatSARIFColumns(w, allColumns, version)
	case "checkstyle":
		output.FormatCheckstyleColumns(w, allColumns)
	default:
		fmt.Fprintf(os.Stderr, "error: unknown format: %s\n", effectiveFormat)
		os.Exit(2)
	}
	if formatErr != nil {
		fmt.Fprintf(os.Stderr, "error: output format %s: %v\n", effectiveFormat, formatErr)
		os.Exit(2)
	}

	if !*quietFlag {
		findingCount := allColumns.Len()
		fmt.Fprintf(os.Stderr, "info: Found %d issue(s) in %v.\n",
			findingCount, time.Since(start).Round(time.Millisecond))
	}

	if allColumns.Len() > 0 {
		os.Exit(1)
	}
}

func filterFixesByLevelColumns(columns *scanner.FindingColumns, registry []rules.Rule, maxLevel rules.FixLevel) (fixableCount, strippedByLevel int) {
	if columns == nil {
		return 0, 0
	}

	ruleLevels := make(map[string]rules.FixLevel, len(registry))
	for _, r := range registry {
		ruleLevels[r.Name()] = rules.GetFixLevel(r)
	}

	strippedByLevel = columns.StripTextFixes(func(row int) bool {
		return ruleLevels[columns.RuleAt(row)] > maxLevel
	})
	return columns.CountTextFixes(), strippedByLevel
}

func runAndroidProjectAnalysis(project *android.AndroidProject, activeRules []rules.Rule, tracker perf.Tracker, providers *androidProjectProviders) []scanner.Finding {
	columns := runAndroidProjectAnalysisColumns(project, activeRules, tracker, providers)
	return columns.Findings()
}

func runAndroidProjectAnalysisColumns(project *android.AndroidProject, activeRules []rules.Rule, tracker perf.Tracker, providers *androidProjectProviders) scanner.FindingColumns {
	if project == nil || project.IsEmpty() {
		return scanner.FindingColumns{}
	}

	var (
		manifestRules []manifestRuleIface
		resourceRules []resourceRuleIface
		gradleRules   []gradleRuleIface
		activeNames   = make(map[string]bool, len(activeRules))
	)
	collector := scanner.NewFindingCollector(len(project.ManifestPaths)*4 + len(project.ResDirs)*8 + len(project.GradlePaths)*4)

	for _, rule := range activeRules {
		activeNames[rule.Name()] = true
		if mr, ok := rule.(manifestRuleIface); ok {
			manifestRules = append(manifestRules, mr)
		}
		if rr, ok := rule.(resourceRuleIface); ok {
			resourceRules = append(resourceRules, rr)
		}
		if gr, ok := rule.(gradleRuleIface); ok {
			gradleRules = append(gradleRules, gr)
		}
	}
	var resourceDeps rules.AndroidDataDependency
	for _, rule := range resourceRules {
		resourceDeps |= rules.AndroidDependenciesOf(rule)
	}
	valueKinds := androidValuesScanKinds(resourceDeps)
	if providers == nil {
		providers = newAndroidProjectProviders(project, collectAndroidDependencies(activeRules), runtime.NumCPU())
	}

	if tracker == nil {
		tracker = perf.New(false)
	}

	manifestTracker := tracker.Serial("manifestAnalysis")
	var (
		manifestParseDur time.Duration
		manifestRuleDur  time.Duration
	)
	for _, path := range project.ManifestPaths {
		start := time.Now()
		parsed, err := android.ParseManifest(path)
		manifestParseDur += time.Since(start)
		if err != nil {
			continue
		}
		manifest := convertManifestForRules(android.ConvertManifest(parsed, path))
		start = time.Now()
		manifestColumns := ruleCheckManifestColumns(manifestRules, manifest)
		collector.AppendColumns(&manifestColumns)
		manifestRuleDur += time.Since(start)
	}
	perf.AddEntry(manifestTracker, "manifestParse", manifestParseDur)
	perf.AddEntry(manifestTracker, "manifestRuleChecks", manifestRuleDur)
	manifestTracker.End()

	resourceTracker := tracker.Serial("resourceAnalysis")
	var (
		resourceScanDur         time.Duration
		resourceWaitDur         time.Duration
		resourceLayoutScanDur   time.Duration
		resourceValuesScanDur   time.Duration
		resourceDrawableScanDur time.Duration
		resourceValuesReadDur   time.Duration
		resourceValuesParseDur  time.Duration
		resourceValuesIndexDur  time.Duration
		resourceMergeDur        time.Duration
		resourceMaxLayoutDur    time.Duration
		resourceMaxValuesDur    time.Duration
		resourceMaxDrawableDur  time.Duration
		resourceRulesDur        time.Duration
		iconScanDur             time.Duration
		iconWaitDur             time.Duration
		iconRulesDur            time.Duration
	)
	for _, resDir := range project.ResDirs {
		var (
			partialIndexes []*android.ResourceIndex
			hadResourceErr bool
		)
		if resourceDeps&rules.AndroidDepLayout != 0 {
			start := time.Now()
			var (
				idx   *android.ResourceIndex
				stats android.ResourceScanStats
				err   error
			)
			if future := providers.layout(resDir); future != nil {
				idx, stats, err = future.Await()
				resourceScanDur += future.Duration()
			} else {
				idx, stats, err = android.ScanLayoutResourcesWithStatsWorkers(resDir, runtime.NumCPU())
				resourceScanDur += time.Since(start)
			}
			resourceWaitDur += time.Since(start)
			if err == nil {
				partialIndexes = append(partialIndexes, idx)
				resourceLayoutScanDur += time.Duration(stats.LayoutScanMs) * time.Millisecond
				resourceDrawableScanDur += time.Duration(stats.DrawableScanMs) * time.Millisecond
				resourceMergeDur += time.Duration(stats.MergeMs) * time.Millisecond
				if d := time.Duration(stats.MaxLayoutScanMs) * time.Millisecond; d > resourceMaxLayoutDur {
					resourceMaxLayoutDur = d
				}
				if d := time.Duration(stats.MaxDrawableScanMs) * time.Millisecond; d > resourceMaxDrawableDur {
					resourceMaxDrawableDur = d
				}
			} else {
				hadResourceErr = true
			}
		}
		if resourceDeps&rules.AndroidDepValues != 0 {
			start := time.Now()
			var (
				idx   *android.ResourceIndex
				stats android.ResourceScanStats
				err   error
			)
			if future := providers.values(resDir); future != nil {
				idx, stats, err = future.Await()
				resourceScanDur += future.Duration()
			} else {
				idx, stats, err = android.ScanValuesResourcesWithStatsKindsWorkers(resDir, runtime.NumCPU(), valueKinds)
				resourceScanDur += time.Since(start)
			}
			resourceWaitDur += time.Since(start)
			if err == nil {
				partialIndexes = append(partialIndexes, idx)
				resourceValuesScanDur += time.Duration(stats.ValuesScanMs) * time.Millisecond
				resourceValuesReadDur += time.Duration(stats.ValuesReadMs) * time.Millisecond
				resourceValuesParseDur += time.Duration(stats.ValuesParseMs) * time.Millisecond
				resourceValuesIndexDur += time.Duration(stats.ValuesIndexMs) * time.Millisecond
				resourceMergeDur += time.Duration(stats.MergeMs) * time.Millisecond
				if d := time.Duration(stats.MaxValuesScanMs) * time.Millisecond; d > resourceMaxValuesDur {
					resourceMaxValuesDur = d
				}
			} else {
				hadResourceErr = true
			}
		}
		if !hadResourceErr && len(partialIndexes) > 0 {
			start := time.Now()
			resourceColumns := runResourceRulesColumns(resourceRules, android.MergeResourceIndexes(partialIndexes...))
			collector.AppendColumns(&resourceColumns)
			resourceRulesDur += time.Since(start)
		}

		start := time.Now()
		var err error
		var iconIdx *android.IconIndex
		if future := providers.icon(resDir); future != nil {
			iconIdx, err = future.Await()
			iconScanDur += future.Duration()
		} else {
			iconIdx, err = android.ScanIconDirs(resDir)
			iconScanDur += time.Since(start)
		}
		iconWaitDur += time.Since(start)
		if err != nil {
			continue
		}
		start = time.Now()
		iconColumns := runActiveIconChecksColumns(iconIdx, activeNames)
		collector.AppendColumns(&iconColumns)
		iconRulesDur += time.Since(start)
	}
	perf.AddEntry(resourceTracker, "resourceDirScan", resourceScanDur)
	perf.AddEntry(resourceTracker, "resourceProviderWait", resourceWaitDur)
	perf.AddEntry(resourceTracker, "layoutDirScan", resourceLayoutScanDur)
	perf.AddEntry(resourceTracker, "valuesDirScan", resourceValuesScanDur)
	perf.AddEntry(resourceTracker, "valuesFileRead", resourceValuesReadDur)
	perf.AddEntry(resourceTracker, "valuesXMLParse", resourceValuesParseDur)
	perf.AddEntry(resourceTracker, "valuesIndexBuild", resourceValuesIndexDur)
	perf.AddEntry(resourceTracker, "resourceMerge", resourceMergeDur)
	perf.AddEntry(resourceTracker, "maxLayoutDirScan", resourceMaxLayoutDur)
	perf.AddEntry(resourceTracker, "maxValuesDirScan", resourceMaxValuesDur)
	perf.AddEntry(resourceTracker, "maxDrawableDirScan", resourceMaxDrawableDur)
	perf.AddEntry(resourceTracker, "drawableDirScan", resourceDrawableScanDur)
	perf.AddEntry(resourceTracker, "resourceRuleChecks", resourceRulesDur)
	perf.AddEntry(resourceTracker, "iconScan", iconScanDur)
	perf.AddEntry(resourceTracker, "iconProviderWait", iconWaitDur)
	perf.AddEntry(resourceTracker, "iconRuleChecks", iconRulesDur)
	resourceTracker.End()

	gradleTracker := tracker.Serial("gradleAnalysis")
	var (
		gradleReadParseDur time.Duration
		gradleRulesDur     time.Duration
	)
	for _, path := range project.GradlePaths {
		start := time.Now()
		content, err := os.ReadFile(path)
		if err != nil {
			gradleReadParseDur += time.Since(start)
			continue
		}
		cfg, err := android.ParseBuildGradleContent(string(content))
		gradleReadParseDur += time.Since(start)
		if err != nil {
			continue
		}
		start = time.Now()
		for _, rule := range gradleRules {
			collector.AppendAll(rule.CheckGradle(path, string(content), cfg))
		}
		gradleRulesDur += time.Since(start)
	}
	perf.AddEntry(gradleTracker, "gradleReadParse", gradleReadParseDur)
	perf.AddEntry(gradleTracker, "gradleRuleChecks", gradleRulesDur)
	gradleTracker.End()

	return *collector.Columns()
}

func ruleCheckManifest[R manifestRuleIface](ruleset []R, manifest *rules.Manifest) []scanner.Finding {
	columns := ruleCheckManifestColumns(ruleset, manifest)
	return columns.Findings()
}

func ruleCheckManifestColumns[R manifestRuleIface](ruleset []R, manifest *rules.Manifest) scanner.FindingColumns {
	collector := scanner.NewFindingCollector(len(ruleset))
	for _, rule := range ruleset {
		collector.AppendAll(rule.CheckManifest(manifest))
	}
	return *collector.Columns()
}

func runResourceRules[R resourceRuleIface](ruleset []R, idx *android.ResourceIndex) []scanner.Finding {
	columns := runResourceRulesColumns(ruleset, idx)
	return columns.Findings()
}

func runResourceRulesColumns[R resourceRuleIface](ruleset []R, idx *android.ResourceIndex) scanner.FindingColumns {
	collector := scanner.NewFindingCollector(len(ruleset))
	for _, rule := range ruleset {
		collector.AppendAll(rule.CheckResources(idx))
	}
	return *collector.Columns()
}

type androidProjectProviders struct {
	project *android.AndroidProject
	deps    rules.AndroidDataDependency

	valuesFutures map[string]*android.ResourceScanFuture
	layoutFutures map[string]*android.ResourceScanFuture
	iconFutures   map[string]*android.IconScanFuture
}

func newAndroidProjectProviders(project *android.AndroidProject, deps rules.AndroidDataDependency, maxWorkers int) *androidProjectProviders {
	p := &androidProjectProviders{
		project:       project,
		deps:          deps,
		valuesFutures: make(map[string]*android.ResourceScanFuture),
		layoutFutures: make(map[string]*android.ResourceScanFuture),
		iconFutures:   make(map[string]*android.IconScanFuture),
	}
	if project == nil {
		return p
	}
	resourceWorkers := androidProviderWorkerCount(maxWorkers)
	iconWorkers := androidIconProviderWorkerCount(maxWorkers)
	resourceLimiter := make(chan struct{}, androidProviderStartConcurrency(maxWorkers))
	iconLimiter := make(chan struct{}, androidIconProviderStartConcurrency(maxWorkers))
	if deps&rules.AndroidDepValues != 0 {
		valueKinds := androidValuesScanKinds(deps)
		for _, resDir := range project.ResDirs {
			p.valuesFutures[resDir] = android.NewValuesScanFuture(resDir, resourceLimiter, valueKinds, resourceWorkers)
		}
	}
	if deps&rules.AndroidDepLayout != 0 {
		for _, resDir := range project.ResDirs {
			p.layoutFutures[resDir] = android.NewLayoutScanFuture(resDir, resourceLimiter, resourceWorkers)
		}
	}
	if deps&rules.AndroidDepIcons != 0 {
		for _, resDir := range project.ResDirs {
			p.iconFutures[resDir] = android.NewIconScanFuture(resDir, iconLimiter, iconWorkers)
		}
	}
	return p
}

func (p *androidProjectProviders) Start() {
	if p == nil {
		return
	}
	for _, future := range p.valuesFutures {
		future.Start()
	}
	for _, future := range p.layoutFutures {
		future.Start()
	}
	for _, future := range p.iconFutures {
		future.Start()
	}
}

func (p *androidProjectProviders) values(resDir string) *android.ResourceScanFuture {
	if p == nil {
		return nil
	}
	return p.valuesFutures[resDir]
}

func (p *androidProjectProviders) layout(resDir string) *android.ResourceScanFuture {
	if p == nil {
		return nil
	}
	return p.layoutFutures[resDir]
}

func (p *androidProjectProviders) icon(resDir string) *android.IconScanFuture {
	if p == nil {
		return nil
	}
	return p.iconFutures[resDir]
}

func collectAndroidDependencies(activeRules []rules.Rule) rules.AndroidDataDependency {
	var deps rules.AndroidDataDependency
	for _, rule := range activeRules {
		deps |= rules.AndroidDependenciesOf(rule)
	}
	return deps
}

// fileTiming captures per-file dispatch timing for profile-dispatch mode.
type fileTiming struct {
	path     string
	size     int
	queueMs  int64 // time between loop dispatch and worker start
	runMs    int64 // time spent in dispatcher.RunWithStats
	lockMs   int64 // time spent waiting for the aggregation mutex
	aggMs    int64 // time spent under the mutex (aggregation)
	totalMs  int64 // total from dispatch to unlock
	findings int
}

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
		sumRun += t.runMs
		sumQueue += t.queueMs
		sumLock += t.lockMs
		sumAgg += t.aggMs
		sumTotal += t.totalMs
		if t.runMs > maxRun {
			maxRun = t.runMs
		}
		if t.totalMs > maxTotal {
			maxTotal = t.totalMs
		}
	}

	// Duration distribution (percentiles of runMs)
	runs := make([]int64, n)
	for i, t := range timings {
		runs[i] = t.runMs
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
		return timings[sorted[i]].runMs > timings[sorted[j]].runMs
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
			t.runMs, t.size/1024, t.findings, t.path)
	}
	// Sum of top 20 vs total: what fraction of work comes from the tail?
	var topSum int64
	for i := 0; i < limit; i++ {
		topSum += timings[sorted[i]].runMs
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

// lockWaits extracts lockMs values for percentile computation.
func lockWaits(timings []fileTiming) []int64 {
	out := make([]int64, len(timings))
	for i, t := range timings {
		out[i] = t.lockMs
	}
	return out
}

// aggHolds extracts aggMs values for percentile computation.
func aggHolds(timings []fileTiming) []int64 {
	out := make([]int64, len(timings))
	for i, t := range timings {
		out[i] = t.aggMs
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

func androidProviderWorkerCount(maxWorkers int) int {
	if maxWorkers < 1 {
		return 1
	}
	workers := maxWorkers / 4
	if workers < 2 && maxWorkers >= 8 {
		workers = 2
	}
	if workers < 1 {
		workers = 1
	}
	if workers > 4 {
		workers = 4
	}
	return workers
}

func androidIconProviderWorkerCount(maxWorkers int) int {
	if maxWorkers >= 16 {
		return 2
	}
	return 1
}

func androidProviderStartConcurrency(maxWorkers int) int {
	if maxWorkers >= 8 {
		return 2
	}
	return 1
}

func androidIconProviderStartConcurrency(maxWorkers int) int {
	return 1
}

func androidValuesScanKinds(deps rules.AndroidDataDependency) android.ValuesScanKind {
	return android.ValuesScanAll
}

func convertManifestForRules(m *android.ConvertedManifest) *rules.Manifest {
	rm := &rules.Manifest{
		Path:            m.Path,
		Package:         m.Package,
		MinSDK:          m.MinSDK,
		TargetSDK:       m.TargetSDK,
		UsesPermissions: append([]string(nil), m.UsesPermissions...),
		Permissions:     append([]string(nil), m.Permissions...),
	}
	for _, f := range m.UsesFeatures {
		rm.UsesFeatures = append(rm.UsesFeatures, rules.ManifestUsesFeature{
			Name:     f.Name,
			Required: f.Required,
			Line:     f.Line,
		})
	}

	for _, elem := range m.Elements {
		rm.Elements = append(rm.Elements, rules.ManifestElement{
			Tag:       elem.Tag,
			Line:      elem.Line,
			ParentTag: elem.ParentTag,
		})
	}
	if m.HasUsesSdk {
		rm.UsesSdk = &rules.ManifestElement{
			Tag:       "uses-sdk",
			Line:      m.UsesSdkLine,
			ParentTag: "manifest",
		}
	}

	if m.HasApplication {
		app := &rules.ManifestApplication{
			Line:                  m.AppLine,
			AllowBackup:           m.AllowBackup,
			Debuggable:            m.Debuggable,
			LocaleConfig:          m.LocaleConfig,
			SupportsRtl:           m.SupportsRtl,
			ExtractNativeLibs:     m.ExtractNativeLibs,
			Icon:                  m.Icon,
			UsesCleartextTraffic:  m.UsesCleartextTraffic,
			FullBackupContent:     m.FullBackupContent,
			DataExtractionRules:   m.DataExtractionRules,
			NetworkSecurityConfig: m.NetworkSecurityConfig,
		}
		for _, activity := range m.Activities {
			app.Activities = append(app.Activities, rules.ManifestComponent{
				Tag:                     activity.Tag,
				Name:                    activity.Name,
				Line:                    activity.Line,
				Exported:                activity.Exported,
				Permission:              activity.Permission,
				HasIntentFilter:         activity.HasIntentFilter,
				ParentTag:               activity.ParentTag,
				IntentFilterActions:     activity.IntentFilterActions,
				IntentFilterCategories:  activity.IntentFilterCategories,
				IntentFilterDataSchemes: activity.IntentFilterDataSchemes,
			})
		}
		for _, service := range m.Services {
			app.Services = append(app.Services, rules.ManifestComponent{
				Tag:                     service.Tag,
				Name:                    service.Name,
				Line:                    service.Line,
				Exported:                service.Exported,
				Permission:              service.Permission,
				HasIntentFilter:         service.HasIntentFilter,
				ParentTag:               service.ParentTag,
				IntentFilterActions:     service.IntentFilterActions,
				IntentFilterCategories:  service.IntentFilterCategories,
				IntentFilterDataSchemes: service.IntentFilterDataSchemes,
			})
		}
		for _, receiver := range m.Receivers {
			var metaEntries []rules.ManifestMetaData
			for _, md := range receiver.MetaDataEntries {
				metaEntries = append(metaEntries, rules.ManifestMetaData{
					Name:     md.Name,
					Value:    md.Value,
					Resource: md.Resource,
				})
			}
			app.Receivers = append(app.Receivers, rules.ManifestComponent{
				Tag:                     receiver.Tag,
				Name:                    receiver.Name,
				Line:                    receiver.Line,
				Exported:                receiver.Exported,
				Permission:              receiver.Permission,
				HasIntentFilter:         receiver.HasIntentFilter,
				ParentTag:               receiver.ParentTag,
				IntentFilterActions:     receiver.IntentFilterActions,
				IntentFilterCategories:  receiver.IntentFilterCategories,
				IntentFilterDataSchemes: receiver.IntentFilterDataSchemes,
				MetaDataEntries:         metaEntries,
			})
		}
		for _, provider := range m.Providers {
			app.Providers = append(app.Providers, rules.ManifestComponent{
				Tag:        provider.Tag,
				Name:       provider.Name,
				Line:       provider.Line,
				Exported:   provider.Exported,
				Permission: provider.Permission,
				ParentTag:  provider.ParentTag,
			})
		}
		rm.Application = app
	}

	return rm
}

func runActiveIconChecks(idx *android.IconIndex, activeNames map[string]bool) []scanner.Finding {
	columns := runActiveIconChecksColumns(idx, activeNames)
	return columns.Findings()
}

func runActiveIconChecksColumns(idx *android.IconIndex, activeNames map[string]bool) scanner.FindingColumns {
	collector := scanner.NewFindingCollector(8)
	if activeNames["IconDensities"] {
		collector.AppendAll(rules.CheckIconDensities(idx))
	}
	if activeNames["IconDipSize"] {
		collector.AppendAll(rules.CheckIconDipSize(idx))
	}
	if activeNames["IconDuplicates"] {
		collector.AppendAll(rules.CheckIconDuplicates(idx))
	}
	if activeNames["GifUsage"] {
		collector.AppendAll(rules.CheckGifUsage(idx))
	}
	if activeNames["ConvertToWebp"] {
		collector.AppendAll(rules.CheckConvertToWebp(idx))
	}
	if activeNames["IconMissingDensityFolder"] {
		collector.AppendAll(rules.CheckIconMissingDensityFolder(idx))
	}
	if activeNames["IconExpectedSize"] {
		collector.AppendAll(rules.CheckIconExpectedSize(idx))
	}
	return *collector.Columns()
}

func countActive(registry []rules.Rule) int {
	inactive := rules.DefaultInactive
	count := 0
	for _, r := range registry {
		if !inactive[r.Name()] {
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

// loadFilesForOracleFilter reads the raw bytes of each .kt path into a
// lightweight *scanner.File so oracle.CollectOracleFiles can run its
// substring-based filter before krit-types is invoked. The returned
// files are NOT fully parsed (no FlatTree) — the filter is byte-only,
// so the cgo tree-sitter pass is deliberately skipped to keep the
// pre-scan cheap. Files that fail to read are dropped silently; they
// will show up later in the real parse loop as parse errors and be
// surfaced there.
func loadFilesForOracleFilter(paths []string) []*scanner.File {
	out := make([]*scanner.File, 0, len(paths))
	for _, p := range paths {
		content, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		out = append(out, &scanner.File{Path: p, Content: content})
	}
	return out
}

// maxInt avoids a division-by-zero in the filter's verbose reporting
// when the file set is empty.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
