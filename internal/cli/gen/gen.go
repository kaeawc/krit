package gen

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

	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

// Run implements `krit gen <artifact>`.
func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: krit gen module-readme <:module> [path]")
		return 1
	}
	switch args[0] {
	case "module-readme":
		return runModuleReadme(args[1:], os.Stdout, os.Stderr)
	case "walkthrough":
		return runWalkthrough(args[1:], os.Stdout, os.Stderr)
	default:
		fmt.Fprintf(os.Stderr, "unknown gen artifact %q; use module-readme or walkthrough\n", args[0])
		return 1
	}
}

func runModuleReadme(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("module-readme", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	pos := fs.Args()
	if len(pos) == 0 {
		fmt.Fprintln(stderr, "usage: krit gen module-readme <:module> [path]")
		return 1
	}
	modulePath := pos[0]
	rootPath := "."
	if len(pos) > 1 {
		rootPath = pos[1]
	}
	root, err := filepath.Abs(rootPath)
	if err != nil {
		fmt.Fprintf(stderr, "module-readme: resolving %s: %v\n", rootPath, err)
		return 1
	}
	summary, err := BuildModuleReadmeSummary(root, modulePath)
	if err != nil {
		fmt.Fprintf(stderr, "module-readme: %v\n", err)
		return 1
	}
	if _, err := io.WriteString(stdout, RenderModuleReadme(summary)); err != nil {
		fmt.Fprintf(stderr, "module-readme: writing markdown: %v\n", err)
		return 1
	}
	return 0
}

type ModuleReadmeSummary struct {
	Module       string
	DependsOn    []string
	DependedOnBy []string
	PublicAPI    []PublicAPISymbol
	Tests        []TestFileSummary
}

type PublicAPISymbol struct {
	Kind string
	Name string
	File string
	Line int
}

type TestFileSummary struct {
	Path  string
	Tests int
}

func BuildModuleReadmeSummary(root, modulePath string) (ModuleReadmeSummary, error) {
	graph, err := module.DiscoverModules(root)
	if err != nil {
		return ModuleReadmeSummary{}, fmt.Errorf("discovering modules: %w", err)
	}
	if graph == nil {
		return ModuleReadmeSummary{}, fmt.Errorf("no settings.gradle(.kts) found at %s", root)
	}
	mod, ok := graph.Modules[modulePath]
	if !ok {
		return ModuleReadmeSummary{}, fmt.Errorf("unknown module %q", modulePath)
	}
	if err := module.ParseAllDependencies(graph); err != nil {
		return ModuleReadmeSummary{}, fmt.Errorf("parsing module dependencies: %w", err)
	}
	kotlinPaths, err := scanner.CollectKotlinFiles([]string{root}, nil)
	if err != nil {
		return ModuleReadmeSummary{}, fmt.Errorf("collecting Kotlin files: %w", err)
	}
	parsed, parseErrs := scanner.ScanFiles(context.Background(), kotlinPaths, runtime.NumCPU())
	if len(parseErrs) > 0 {
		return ModuleReadmeSummary{}, fmt.Errorf("parsing Kotlin files: %w", parseErrs[0])
	}
	pmi := module.BuildPerModuleIndex(graph, parsed, runtime.NumCPU())
	summary := ModuleReadmeSummary{
		Module:       modulePath,
		DependsOn:    uniqueDependencyPaths(mod.Dependencies),
		DependedOnBy: sortedStrings(graph.Consumers[modulePath]),
		PublicAPI:    publicAPISymbols(pmi.ModuleIndex[modulePath], root, mod.SourceRoots),
		Tests:        testFileSummaries(pmi.ModuleFiles[modulePath], root, mod.SourceRoots),
	}
	return summary, nil
}

func uniqueDependencyPaths(deps []module.Dependency) []string {
	seen := make(map[string]bool, len(deps))
	var out []string
	for _, dep := range deps {
		if dep.ModulePath == "" || seen[dep.ModulePath] {
			continue
		}
		seen[dep.ModulePath] = true
		out = append(out, dep.ModulePath)
	}
	sort.Strings(out)
	return out
}

func publicAPISymbols(index *scanner.CodeIndex, root string, sourceRoots []string) []PublicAPISymbol {
	if index == nil {
		return nil
	}
	api := make([]PublicAPISymbol, 0, len(index.Symbols))
	for _, sym := range index.Symbols {
		if sym.Visibility != "public" {
			continue
		}
		if !pathUnderAnyRoot(sym.File, sourceRoots) {
			continue
		}
		api = append(api, PublicAPISymbol{
			Kind: symbolKindLabel(sym.Kind),
			Name: publicAPIName(sym),
			File: relativePath(root, sym.File),
			Line: sym.Line,
		})
	}
	sort.Slice(api, func(i, j int) bool {
		if api[i].File != api[j].File {
			return api[i].File < api[j].File
		}
		if api[i].Line != api[j].Line {
			return api[i].Line < api[j].Line
		}
		if api[i].Kind != api[j].Kind {
			return api[i].Kind < api[j].Kind
		}
		return api[i].Name < api[j].Name
	})
	return api
}

func publicAPIName(sym scanner.Symbol) string {
	return sym.Name
}

func symbolKindLabel(kind string) string {
	switch kind {
	case "method":
		return "fun"
	case "function":
		return "fun"
	default:
		if kind == "" {
			return "symbol"
		}
		return kind
	}
}

func testFileSummaries(files []*scanner.File, root string, sourceRoots []string) []TestFileSummary {
	var tests []TestFileSummary
	for _, file := range files {
		if file == nil || !pathUnderAnyRoot(file.Path, sourceRoots) || !scanner.IsTestFile(filepath.ToSlash(file.Path)) {
			continue
		}
		tests = append(tests, TestFileSummary{
			Path:  relativePath(root, file.Path),
			Tests: countTestFunctions(file),
		})
	}
	sort.Slice(tests, func(i, j int) bool { return tests[i].Path < tests[j].Path })
	return tests
}

func countTestFunctions(file *scanner.File) int {
	count := 0
	file.FlatWalkNodes(0, "function_declaration", func(uint32) {
		count++
	})
	return count
}

func RenderModuleReadme(summary ModuleReadmeSummary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", summary.Module)
	fmt.Fprintf(&b, "**Depends on:** %s\n", moduleList(summary.DependsOn))
	fmt.Fprintf(&b, "**Depended on by:** %s\n\n", moduleListWithCount(summary.DependedOnBy))
	b.WriteString("## Public API\n\n")
	if len(summary.PublicAPI) == 0 {
		b.WriteString("- None\n")
	} else {
		for _, sym := range summary.PublicAPI {
			fmt.Fprintf(&b, "- %s `%s`\n", sym.Kind, sym.Name)
		}
	}
	b.WriteString("\n## Tests\n\n")
	if len(summary.Tests) == 0 {
		b.WriteString("- None\n")
	} else {
		for _, test := range summary.Tests {
			fmt.Fprintf(&b, "- `%s` (%d tests)\n", test.Path, test.Tests)
		}
	}
	return b.String()
}

func moduleList(modules []string) string {
	if len(modules) == 0 {
		return "None"
	}
	return strings.Join(modules, ", ")
}

func moduleListWithCount(modules []string) string {
	if len(modules) == 0 {
		return "None"
	}
	if len(modules) == 1 {
		return modules[0]
	}
	return fmt.Sprintf("%s (%d modules)", strings.Join(modules, ", "), len(modules))
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func relativePath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func pathUnderAnyRoot(path string, roots []string) bool {
	if len(roots) == 0 {
		return true
	}
	clean := filepath.Clean(path)
	for _, root := range roots {
		root = filepath.Clean(root)
		if clean == root || strings.HasPrefix(clean, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
