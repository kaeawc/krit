package scan

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/experiment"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/schema"
)

//go:embed completions
var completionsFS embed.FS

// completionsFilename maps a shell name to the embedded completion script
// path. Returns ok=false for unsupported shells.
func completionsFilename(shell string) (filename string, ok bool) {
	switch shell {
	case "bash":
		return "completions/krit.bash", true
	case "zsh":
		return "completions/krit.zsh", true
	case "fish":
		return "completions/krit.fish", true
	}
	return "", false
}

func runVersionFlag(versionFlag bool, version string) {
	if !versionFlag {
		return
	}
	fmt.Println("krit", version)
	os.Exit(0)
}

func runClearMatrixCacheFlag(clearMatrixCacheFlag bool) {
	if !clearMatrixCacheFlag {
		return
	}
	if err := ClearMatrixCache(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	fmt.Fprintln(os.Stderr, "info: Matrix baseline cache cleared.")
	os.Exit(0)
}

func runPromoteExperimentFlag(name string) {
	if name == "" {
		return
	}
	os.Exit(PromoteExperiment(name, experiment.StatusPromoted))
}

func runDeprecateExperimentFlag(name string) {
	if name == "" {
		return
	}
	os.Exit(PromoteExperiment(name, experiment.StatusDeprecated))
}

func runListExperimentsFlag(listExperimentsFlag bool, effectiveFormat, version string) {
	if !listExperimentsFlag {
		return
	}
	if effectiveFormat == "plain" {
		fmt.Print(ListExperimentsLifecyclePlain())
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

func runCompletionsFlag(shell string) {
	if shell == "" {
		return
	}
	filename, ok := completionsFilename(shell)
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown shell %q; supported: bash, zsh, fish\n", shell)
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

const initStarterConfig = `# Krit configuration — https://kaeawc.github.io/krit/configuration/
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

func runInitFlag(initFlag bool) {
	if !initFlag {
		return
	}
	for _, name := range config.Filenames {
		if _, err := os.Stat(name); err == nil {
			fmt.Fprintf(os.Stderr, "Config already exists: %s\n", name)
			os.Exit(0)
		}
	}
	if err := os.WriteFile("krit.yml", []byte(initStarterConfig), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing krit.yml: %v\n", err)
		os.Exit(2)
	}
	fmt.Println("Created krit.yml with recommended defaults.")
	fmt.Println("Run 'krit .' to analyze your project.")
	os.Exit(0)
}

func runDoctorFlag(doctorFlag bool, version string) {
	if !doctorFlag {
		return
	}
	fmt.Println("krit doctor")
	fmt.Println()
	fmt.Printf("  krit version: %s\n", version)
	fmt.Printf("  rules: %d registered (%d active by default)\n", len(api.Registry), countActiveV2(api.Registry))
	configFound := false
	for _, name := range config.Filenames {
		if _, err := os.Stat(name); err == nil {
			fmt.Printf("  config: %s (found)\n", name)
			configFound = true
			break
		}
	}
	if !configFound {
		fmt.Println("  config: none (run --init to create)")
	}
	if javaPath, err := exec.LookPath("java"); err == nil {
		fmt.Printf("  java: %s\n", javaPath)
	} else {
		fmt.Println("  java: not found (optional — needed for type oracle)")
	}
	if cwebpPath, err := exec.LookPath("cwebp"); err == nil {
		fmt.Printf("  cwebp: %s (WebP conversion available)\n", cwebpPath)
	} else {
		fmt.Println("  cwebp: not found (optional — needed for --fix-binary WebP)")
	}
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

func runGenerateSchemaFlag(generateSchemaFlag bool) {
	if !generateSchemaFlag {
		return
	}
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

func runNewExperimentScaffoldFlag(opts NewExperimentOpts) {
	if opts.Name == "" {
		return
	}
	os.Exit(RunNewExperimentScaffold(opts))
}

func runValidateConfigFlag(validateConfigFlag bool, cfg *config.Config) {
	if !validateConfigFlag {
		return
	}
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

// listRulesSummary holds the aggregate counts emitted at the bottom of
// --list-rules output. Pulled out so the totals line can be unit-tested
// without driving stdout.
type listRulesSummary struct {
	Total   int
	Active  int
	Fixable int
}

// computeListRulesSummary tallies registered, default-active, and fixable rules.
func computeListRulesSummary(registry []*api.Rule) listRulesSummary {
	s := listRulesSummary{Total: len(registry)}
	for _, r := range registry {
		if rules.IsDefaultActive(r.ID) {
			s.Active++
		}
		if _, isFixable := rules.GetV2FixLevel(r); isFixable {
			s.Fixable++
		}
	}
	return s
}

func runListRulesFlag(listFlag, verboseFlag bool, maturityFilter, taxonomyID string) {
	if !listFlag {
		return
	}
	printListRules(os.Stdout, verboseFlag, maturityFilter, taxonomyID)
	os.Exit(0)
}

// printListRules writes the --list-rules output. Split from
// runListRulesFlag so tests can drive it without the os.Exit.
func printListRules(w io.Writer, verboseFlag bool, maturityFilter, taxonomyID string) {
	var maturity api.Maturity
	maturityFilterSet := false
	if maturityFilter != "" {
		m, ok := api.ParseMaturity(maturityFilter)
		if !ok {
			fmt.Fprintf(os.Stderr, "error: unknown --maturity value %q; valid: stable, experimental, deprecated\n", maturityFilter)
			os.Exit(2)
		}
		maturity = m
		maturityFilterSet = true
	}

	registry := api.Registry
	if maturityFilterSet {
		registry = api.MaturityFilter(registry, maturity)
	}

	var matcher api.TaxonomyMatcher
	taxonomyID = strings.TrimSpace(taxonomyID)
	if taxonomyID != "" {
		matcher = api.TaxonomyMatcher{IDs: []string{taxonomyID}}
		fmt.Fprintf(w, "Available rules (filtered by taxonomy ID %q):\n", taxonomyID)
	} else {
		fmt.Fprintln(w, "Available rules:")
	}
	fixable := 0
	active := 0
	for _, r := range registry {
		if len(matcher.IDs) > 0 && !matcher.Matches(r.Security) {
			continue
		}
		markers := ""
		if rules.IsDefaultActive(r.ID) {
			markers += "A"
			active++
		} else {
			markers += " "
		}
		if fixLvl, isFixable := rules.GetV2FixLevel(r); isFixable {
			markers += "F"
			fixable++
			if verboseFlag {
				fmt.Fprintf(w, "  %s %-40s [%-15s] %s (fix: %s, precision: %s, maturity: %s)\n", markers, r.ID, r.Category, string(r.Sev), fixLvl, rules.V2RulePrecision(r), r.Maturity)
				if r.Description != "" {
					fmt.Fprintf(w, "    %s\n", r.Description)
				}
			} else {
				fmt.Fprintf(w, "  %s %-40s [%-15s] %s\n", markers, r.ID, r.Category, string(r.Sev))
			}
		} else {
			markers += " "
			if verboseFlag {
				fmt.Fprintf(w, "  %s %-40s [%-15s] %s (precision: %s, maturity: %s)\n", markers, r.ID, r.Category, string(r.Sev), rules.V2RulePrecision(r), r.Maturity)
				if r.Description != "" {
					fmt.Fprintf(w, "    %s\n", r.Description)
				}
			} else {
				fmt.Fprintf(w, "  %s %-40s [%-15s] %s\n", markers, r.ID, r.Category, string(r.Sev))
			}
		}
	}
	fmt.Fprintf(w, "\nTotal: %d rules (%d active by default, %d fixable)\n", len(registry), active, fixable)
	fmt.Fprintln(w, "A=active by default, F=fixable. Use -v for fix levels, --all-rules to enable all, --maturity to filter by lifecycle.")
}

// outputTypesOpts bundles the flag values runOutputTypesFlag needs.
type outputTypesOpts struct {
	OutputPath    string
	NoCacheOracle bool
	Verbose       bool
	StoreDir      *string
	Paths         []string
}

// runOutputTypesFlag handles --output-types: a standalone krit-types dump
// that bypasses rules entirely. Locates the krit-types jar, finds Kotlin
// source directories, and writes the oracle JSON to the requested path.
// No-op when opts.OutputPath is empty; otherwise terminates the process.
func runOutputTypesFlag(opts outputTypesOpts) {
	if opts.OutputPath == "" {
		return
	}
	jarPath := oracle.FindJar(opts.Paths)
	if jarPath == "" {
		fmt.Fprintf(os.Stderr, "error: krit-types.jar not found. Build it with: cd tools/krit-types && ./gradlew shadowJar\n")
		os.Exit(2)
	}
	sourceDirs := oracle.FindSourceDirs(opts.Paths)
	if len(sourceDirs) == 0 {
		fmt.Fprintf(os.Stderr, "error: no Kotlin source directories found\n")
		os.Exit(2)
	}
	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "verbose: Found %d source directories\n", len(sourceDirs))
	}
	var err error
	// --output-types is a standalone oracle dump: no rules are loaded so
	// there's no rule-classification filter to apply. Pass "" for the
	// filter list path; both call paths handle that as "no filter".
	if opts.NoCacheOracle {
		_, err = oracle.Invoke(jarPath, sourceDirs, opts.OutputPath, opts.Verbose)
	} else {
		_, err = oracle.InvokeCached(jarPath, sourceDirs, oracle.FindRepoDir(opts.Paths), opts.OutputPath, "", opts.Verbose, resolvedStore(opts.StoreDir))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runClearCacheFlag(clearCacheFlag bool, cacheDirFlag, cacheFilePath string, paths []string) {
	if !clearCacheFlag {
		return
	}
	ctx := cacheutil.ClearContext{RepoDir: oracle.FindRepoDir(paths)}
	if err := cacheutil.ClearAll(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	if cacheDirFlag != "" {
		if err := cache.ClearSharedCache(cacheDirFlag); err != nil {
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
