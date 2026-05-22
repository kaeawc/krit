package rules

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/graph"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// PackageDependencyCycleRule reports cycles in the package-level import graph
// within a single Gradle module.
type PackageDependencyCycleRule struct {
	BaseRule
}

func (r *PackageDependencyCycleRule) check(ctx *api.Context) {
	pmi := ctx.ModuleIndex
	if pmi == nil || pmi.Graph == nil {
		return
	}

	for modPath, files := range pmi.ModuleFiles {
		if modPath == "root" {
			continue
		}
		r.checkModule(ctx, modPath, files)
	}
}

// Confidence holds the 0.95 dispatch default — cycle detection on
// the package-level import graph is a precise Tarjan/DFS result; a
// reported cycle is a real cycle. No heuristic path.
func (r *PackageDependencyCycleRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func (r *PackageDependencyCycleRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{NeedsFiles: true}
}

type packageCycleFile struct {
	pkg     string
	file    string
	line    int
	imports []string
}

type PackageDependencyEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type PackageDependencyGraphData struct {
	Packages []string                `json:"packages"`
	Edges    []PackageDependencyEdge `json:"edges"`
}

func PackageDependencyGraph(files []*scanner.File) PackageDependencyGraphData {
	entries := make([]packageCycleFile, 0, len(files))
	packages := make(map[string]bool)
	for _, file := range files {
		if file == nil || shouldSkipPackageDependencyCycleFile(file.Path) {
			continue
		}
		pkg, line, imports := packageDependencyCycleData(file)
		if pkg == "" {
			continue
		}
		entries = append(entries, packageCycleFile{pkg: pkg, file: file.Path, line: line, imports: imports})
		packages[pkg] = true
	}
	seen := make(map[string]bool)
	var edges []PackageDependencyEdge
	for _, entry := range entries {
		for _, importedPkg := range entry.imports {
			if importedPkg == "" || importedPkg == entry.pkg || !packages[importedPkg] {
				continue
			}
			key := entry.pkg + "\x00" + importedPkg
			if seen[key] {
				continue
			}
			seen[key] = true
			edges = append(edges, PackageDependencyEdge{From: entry.pkg, To: importedPkg})
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})
	names := make([]string, 0, len(packages))
	for pkg := range packages {
		names = append(names, pkg)
	}
	sort.Strings(names)
	return PackageDependencyGraphData{Packages: names, Edges: edges}
}

func (r *PackageDependencyCycleRule) checkModule(ctx *api.Context, modPath string, files []*scanner.File) {
	entries := make([]packageCycleFile, 0, len(files))
	packages := make(map[string]packageCycleFile)

	for _, file := range files {
		if shouldSkipPackageDependencyCycleFile(file.Path) {
			continue
		}

		pkg, line, imports := packageDependencyCycleData(file)
		if pkg == "" {
			continue
		}

		entry := packageCycleFile{
			pkg:     pkg,
			file:    file.Path,
			line:    line,
			imports: imports,
		}
		entries = append(entries, entry)
		if _, ok := packages[pkg]; !ok {
			packages[pkg] = entry
		}
	}

	if len(packages) < 2 {
		return
	}

	g := graph.NewGraph()
	for pkg := range packages {
		g.AddNode(pkg)
	}
	for _, entry := range entries {
		for _, importedPkg := range entry.imports {
			if importedPkg == "" || importedPkg == entry.pkg {
				continue
			}
			if _, ok := packages[importedPkg]; !ok {
				continue
			}
			g.AddEdge(entry.pkg, importedPkg)
		}
	}

	for _, cycle := range graph.FindSCCs(g) {
		sort.Strings(cycle)
		anchor := packages[cycle[0]]
		ctx.Emit(scanner.Finding{
			File:     anchor.file,
			Line:     anchor.line,
			Col:      1,
			RuleSet:  r.RuleSetName,
			Rule:     r.RuleName,
			Severity: r.Sev,
			Message: fmt.Sprintf("Packages %s form an import cycle within module %s.",
				strings.Join(cycle, ", "), modPath),
		})
	}
}

func shouldSkipPackageDependencyCycleFile(path string) bool {
	path = filepath.ToSlash(path)
	return strings.Contains(path, "/src/test/") ||
		strings.Contains(path, "/src/androidTest/") ||
		strings.Contains(path, "/src/testFixtures/") ||
		strings.Contains(path, "/src/benchmark/")
}

func packageDependencyCycleData(file *scanner.File) (string, int, []string) {
	pkg := ""
	pkgLine := 1
	imports := make(map[string]struct{})

	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		if pkg == "" && strings.HasPrefix(trimmed, "package ") {
			pkg = strings.TrimSpace(strings.TrimPrefix(trimmed, "package "))
			pkgLine = i + 1
			continue
		}

		if !strings.HasPrefix(trimmed, "import ") {
			continue
		}

		imp := strings.TrimSpace(strings.TrimPrefix(trimmed, "import "))
		if idx := strings.Index(imp, " as "); idx >= 0 {
			imp = strings.TrimSpace(imp[:idx])
		}
		if strings.HasSuffix(imp, ".*") {
			imp = strings.TrimSuffix(imp, ".*")
		} else if lastDot := strings.LastIndex(imp, "."); lastDot > 0 {
			imp = imp[:lastDot]
		} else {
			continue
		}

		if imp != "" {
			imports[imp] = struct{}{}
		}
	}

	importedPackages := make([]string, 0, len(imports))
	for imp := range imports {
		importedPackages = append(importedPackages, imp)
	}
	sort.Strings(importedPackages)

	return pkg, pkgLine, importedPackages
}
