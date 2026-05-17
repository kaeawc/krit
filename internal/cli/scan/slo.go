package scan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	sloRuleName = "SloRules"
	sloRuleSet  = "metrics"
)

func (r *runner) applySLOs() {
	if r.cfg == nil {
		return
	}
	slos := r.cfg.SLOs()
	if len(slos) == 0 {
		return
	}
	graph := r.moduleGraph
	if graph == nil || len(graph.Modules) == 0 {
		graph = discoverSLOGraph(r.paths)
		if graph == nil || len(graph.Modules) == 0 {
			return
		}
		r.moduleGraph = graph
	}
	pmi := r.pmi
	if pmi == nil {
		pmi = module.BuildPerModuleIndex(graph, r.sourceFiles, runtime.NumCPU())
		r.pmi = pmi
	}
	r.allFindings = append(r.allFindings, evaluateSLOs(slos, graph, pmi.ModuleFiles, r.allFindings)...)
}

func discoverSLOGraph(paths []string) *module.Graph {
	root := "."
	if len(paths) > 0 && paths[0] != "" {
		root = paths[0]
	}
	info, err := os.Stat(root)
	if err == nil && !info.IsDir() {
		root = filepath.Dir(root)
	}
	graph, err := module.DiscoverModules(context.Background(), root)
	if err != nil {
		return nil
	}
	return graph
}

func evaluateSLOs(slos []config.SLOConfig, graph *module.Graph, moduleFiles map[string][]*scanner.File, findings []scanner.Finding) []scanner.Finding {
	if graph == nil || len(slos) == 0 {
		return nil
	}
	locByModule := moduleLineCounts(moduleFiles)
	countsByModule := moduleFindingCounts(graph, findings)

	var out []scanner.Finding
	for _, slo := range slos {
		mod := graph.Modules[slo.Module]
		if mod == nil {
			continue
		}
		loc := locByModule[slo.Module]
		if loc == 0 {
			continue
		}
		counts := countsByModule[slo.Module]
		kloc := float64(loc) / 1000.0
		anchor := moduleAnchorFile(mod)
		if slo.MaxWarningsPerKLOC != nil {
			density := float64(counts.warnings) / kloc
			if density > *slo.MaxWarningsPerKLOC {
				out = append(out, sloFinding(anchor, slo.Module, "warnings", density, *slo.MaxWarningsPerKLOC))
			}
		}
		if slo.MaxErrorsPerKLOC != nil {
			density := float64(counts.errors) / kloc
			if density > *slo.MaxErrorsPerKLOC {
				out = append(out, sloFinding(anchor, slo.Module, "errors", density, *slo.MaxErrorsPerKLOC))
			}
		}
	}
	return out
}

type sloCounts struct {
	warnings int
	errors   int
}

func moduleLineCounts(moduleFiles map[string][]*scanner.File) map[string]int {
	counts := make(map[string]int, len(moduleFiles))
	for modPath, files := range moduleFiles {
		for _, file := range files {
			if file == nil || scanner.IsTestFile(filepath.ToSlash(file.Path)) {
				continue
			}
			counts[modPath] += len(file.Lines)
		}
	}
	return counts
}

func moduleFindingCounts(graph *module.Graph, findings []scanner.Finding) map[string]sloCounts {
	counts := make(map[string]sloCounts)
	for _, finding := range findings {
		if finding.File == "" || finding.Rule == sloRuleName {
			continue
		}
		modPath := graph.FileToModule(finding.File)
		if modPath == "" {
			continue
		}
		c := counts[modPath]
		switch strings.ToLower(finding.Severity) {
		case "warning":
			c.warnings++
		case "error":
			c.errors++
		}
		counts[modPath] = c
	}
	return counts
}

func moduleAnchorFile(mod *module.Module) string {
	for _, name := range []string{"build.gradle.kts", "build.gradle"} {
		path := filepath.Join(mod.Dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return filepath.Join(mod.Dir, "build.gradle.kts")
}

func sloFinding(file, modulePath, kind string, density, limit float64) scanner.Finding {
	return scanner.Finding{
		File:       file,
		Line:       1,
		Col:        1,
		RuleSet:    sloRuleSet,
		Rule:       sloRuleName,
		Severity:   "warning",
		Confidence: 1,
		Message:    fmt.Sprintf("Module %q has %.1f %s per 1k LOC, above SLO %.1f.", modulePath, density, kind, limit),
	}
}
