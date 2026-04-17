package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

// ModuleDependencyCycleRule reports cross-module cycles in the Gradle
// dependency graph derived from every build.gradle(.kts). Active by default:
// module-level cycles indicate a serious architectural problem that blocks
// clean module boundaries and incremental builds.
type ModuleDependencyCycleRule struct {
	BaseRule
	pmi *module.PerModuleIndex
}

func (r *ModuleDependencyCycleRule) Check(_ *scanner.File) []scanner.Finding { return nil }

// Confidence is 0.95 — Tarjan SCC on the parsed module graph is
// deterministic and precise. A reported cycle is a real cycle.
func (r *ModuleDependencyCycleRule) Confidence() float64 { return 0.95 }

func (r *ModuleDependencyCycleRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{NeedsDependencies: true}
}

func (r *ModuleDependencyCycleRule) SetModuleIndex(pmi *module.PerModuleIndex) {
	r.pmi = pmi
}

func (r *ModuleDependencyCycleRule) CheckModuleAware() []scanner.Finding {
	if r.pmi == nil || r.pmi.Graph == nil {
		return nil
	}

	cycles := r.pmi.Graph.FindCycles()
	if len(cycles) == 0 {
		return nil
	}

	var findings []scanner.Finding
	for _, cycle := range cycles {
		sort.Strings(cycle)
		anchorPath := cycle[0]
		mod, ok := r.pmi.Graph.Modules[anchorPath]
		if !ok {
			continue
		}
		anchorFile := mod.Dir + "/build.gradle.kts"
		findings = append(findings, scanner.Finding{
			File:     anchorFile,
			Line:     1,
			Col:      1,
			RuleSet:  r.RuleSetName,
			Rule:     r.RuleName,
			Severity: r.Sev,
			Message: fmt.Sprintf("Modules %s form a dependency cycle. Break the cycle by extracting shared code into a lower-level module.",
				strings.Join(cycle, " → ")),
		})
	}
	return findings
}
