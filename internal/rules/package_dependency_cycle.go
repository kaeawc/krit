package rules

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/graph"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

// PackageDependencyCycleRule reports cycles in the package-level import graph
// within a single Gradle module.
type PackageDependencyCycleRule struct {
	BaseRule
	pmi *module.PerModuleIndex
}

func (r *PackageDependencyCycleRule) Check(_ *scanner.File) []scanner.Finding { return nil }

// Confidence holds the 0.95 dispatch default — cycle detection on
// the package-level import graph is a precise Tarjan/DFS result; a
// reported cycle is a real cycle. No heuristic path.
func (r *PackageDependencyCycleRule) Confidence() float64 { return 0.95 }

func (r *PackageDependencyCycleRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{NeedsFiles: true}
}

func (r *PackageDependencyCycleRule) SetModuleIndex(pmi *module.PerModuleIndex) {
	r.pmi = pmi
}

func (r *PackageDependencyCycleRule) CheckModuleAware() []scanner.Finding {
	if r.pmi == nil || r.pmi.Graph == nil {
		return nil
	}

	var findings []scanner.Finding
	for modPath, files := range r.pmi.ModuleFiles {
		if modPath == "root" {
			continue
		}

		moduleFindings := r.checkModule(modPath, files)
		findings = append(findings, moduleFindings...)
	}

	return findings
}

type packageCycleFile struct {
	pkg     string
	file    string
	line    int
	imports []string
}

func (r *PackageDependencyCycleRule) checkModule(modPath string, files []*scanner.File) []scanner.Finding {
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
		return nil
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

	var findings []scanner.Finding
	for _, cycle := range graph.FindSCCs(g) {
		sort.Strings(cycle)
		anchor := packages[cycle[0]]
		findings = append(findings, scanner.Finding{
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

	return findings
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
