package rules

import (
	"fmt"
	"path/filepath"

	"github.com/kaeawc/krit/internal/arch"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

// LayerDependencyViolationRule enforces a configurable layer dependency
// matrix (e.g. ui may depend on domain, domain may depend on data).
// Inactive by default: requires project-specific layer config.
type LayerDependencyViolationRule struct {
	BaseRule
	LayerConfig *arch.LayerConfig
	pmi         *module.PerModuleIndex
}

func (r *LayerDependencyViolationRule) Check(_ *scanner.File) []scanner.Finding { return nil }

// Confidence is 0.95 — given a well-defined layer matrix, the check is
// a deterministic graph walk. False positives only occur from misconfigured
// layer definitions, not from algorithm imprecision.
func (r *LayerDependencyViolationRule) Confidence() float64 { return 0.95 }

func (r *LayerDependencyViolationRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{NeedsDependencies: true}
}

func (r *LayerDependencyViolationRule) SetModuleIndex(pmi *module.PerModuleIndex) {
	r.pmi = pmi
}

func (r *LayerDependencyViolationRule) CheckModuleAware() []scanner.Finding {
	if r.pmi == nil || r.pmi.Graph == nil || r.LayerConfig == nil {
		return nil
	}

	violations := arch.ValidateLayers(r.LayerConfig, r.pmi.Graph)
	if len(violations) == 0 {
		return nil
	}

	var findings []scanner.Finding
	for _, v := range violations {
		mod, ok := r.pmi.Graph.Modules[v.SourceModule]
		if !ok {
			continue
		}
		findings = append(findings, scanner.Finding{
			File:     filepath.Join(mod.Dir, "build.gradle.kts"),
			Line:     1,
			Col:      1,
			RuleSet:  r.RuleSetName,
			Rule:     r.RuleName,
			Severity: r.Sev,
			Message: fmt.Sprintf("Module %s (layer %q) must not depend on %s (layer %q): %q → %q is not in the allowed matrix.",
				v.SourceModule, v.SourceLayer, v.TargetModule, v.TargetLayer, v.SourceLayer, v.TargetLayer),
		})
	}
	return findings
}
