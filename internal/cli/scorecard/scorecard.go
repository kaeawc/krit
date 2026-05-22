package scorecard

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

type Row struct {
	Module          string
	FindingsPerKLOC float64
	AvgComplexity   float64
	TestRatio       float64
}

func Run(args []string) int {
	return run(args, os.Stdout, os.Stderr)
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("scorecard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "markdown", "Output format: markdown")
	configPath := fs.String("config", "", "Path to krit.yml")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *format != "markdown" {
		fmt.Fprintf(stderr, "scorecard: unsupported format %q; use markdown\n", *format)
		return 1
	}
	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	rows, err := Build(paths, *configPath)
	if err != nil {
		fmt.Fprintf(stderr, "scorecard: %v\n", err)
		return 1
	}
	WriteMarkdown(stdout, rows)
	return 0
}

func Build(paths []string, configPath string) ([]Row, error) {
	root, err := scanRoot(paths)
	if err != nil {
		return nil, err
	}
	graph, err := module.DiscoverModules(context.Background(), root)
	if err != nil {
		return nil, fmt.Errorf("discovering modules: %w", err)
	}
	if graph == nil {
		return nil, fmt.Errorf("no settings.gradle(.kts) found at %s", root)
	}
	if err := module.ParseAllDependencies(graph); err != nil {
		return nil, fmt.Errorf("parsing module dependencies: %w", err)
	}
	kotlinPaths, err := scanner.CollectKotlinFiles(paths, nil)
	if err != nil {
		return nil, fmt.Errorf("collecting Kotlin files: %w", err)
	}
	files, parseErrs := scanner.ScanFiles(context.Background(), kotlinPaths, runtime.NumCPU())
	if len(parseErrs) > 0 {
		return nil, fmt.Errorf("parsing Kotlin files: %w", parseErrs[0])
	}

	cfg, err := config.LoadAndMerge(configPath, config.FindDefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	if cfg == nil {
		cfg = config.NewConfig()
	}
	rules.ApplyConfig(cfg)
	active := rules.ActiveRulesV2(nil, nil, false, false, false)
	findings := runRules(files, active)

	pmi := module.BuildPerModuleIndex(graph, files, runtime.NumCPU())
	return rowsFromData(graph, pmi.ModuleFiles, findings), nil
}

func runRules(files []*scanner.File, active []*api.Rule) []scanner.Finding {
	dispatcher := rules.NewDispatcher(active, nil)
	collector := scanner.NewFindingCollector(len(files) * 8)
	for _, file := range files {
		cols, _ := dispatcher.RunColumnsWithStats(file)
		collector.AppendColumns(&cols)
	}
	return collector.Columns().Findings()
}

func rowsFromData(graph *module.Graph, moduleFiles map[string][]*scanner.File, findings []scanner.Finding) []Row {
	findingCounts := make(map[string]int)
	for _, finding := range findings {
		mod := graph.FileToModule(finding.File)
		if mod != "" {
			findingCounts[mod]++
		}
	}
	modules := make([]string, 0, len(graph.Modules))
	for mod := range graph.Modules {
		modules = append(modules, mod)
	}
	sort.Strings(modules)

	rows := make([]Row, 0, len(modules))
	for _, mod := range modules {
		files := moduleFiles[mod]
		mainLOC, testLOC := moduleLOC(files)
		avgComplexity := averageComplexity(files)
		var findingsPerKLOC float64
		if mainLOC > 0 {
			findingsPerKLOC = float64(findingCounts[mod]) / (float64(mainLOC) / 1000.0)
		}
		var testRatio float64
		if mainLOC > 0 {
			testRatio = float64(testLOC) / float64(mainLOC)
		}
		rows = append(rows, Row{
			Module:          mod,
			FindingsPerKLOC: findingsPerKLOC,
			AvgComplexity:   avgComplexity,
			TestRatio:       testRatio,
		})
	}
	return rows
}

func moduleLOC(files []*scanner.File) (mainLOC, testLOC int) {
	for _, file := range files {
		if file == nil {
			continue
		}
		if isTestSource(file.Path) {
			testLOC += len(file.Lines)
		} else {
			mainLOC += len(file.Lines)
		}
	}
	return mainLOC, testLOC
}

func averageComplexity(files []*scanner.File) float64 {
	total := 0
	count := 0
	for _, file := range files {
		if file == nil || isTestSource(file.Path) {
			continue
		}
		for _, c := range rules.FunctionComplexities(file) {
			total += c.Cognitive
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return float64(total) / float64(count)
}

func WriteMarkdown(w io.Writer, rows []Row) {
	fmt.Fprintln(w, "| Module | Findings/1kLOC | Avg Complexity | Test Ratio |")
	fmt.Fprintln(w, "|--------|----------------|----------------|------------|")
	for _, row := range rows {
		fmt.Fprintf(w, "| %s | %.1f | %.1f | %.1f |\n", row.Module, row.FindingsPerKLOC, row.AvgComplexity, row.TestRatio)
	}
}

func scanRoot(paths []string) (string, error) {
	root := "."
	if len(paths) > 0 && paths[0] != "" {
		root = paths[0]
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		root = filepath.Dir(root)
	}
	return filepath.Abs(root)
}

func isTestSource(path string) bool {
	slash := filepath.ToSlash(path)
	for _, marker := range []string{"/src/test/", "/src/androidTest/", "/src/commonTest/", "/src/testFixtures/"} {
		if strings.Contains(slash, marker) {
			return true
		}
	}
	return false
}
