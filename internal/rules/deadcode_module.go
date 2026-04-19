package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/module"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// Module-aware rules are identified structurally by the presence of
// SetModuleIndex + CheckModuleAware methods. See v2.Rule.Needs
// (NeedsModuleIndex) for the canonical form.

// ModuleAwareNeeds describes which module-analysis inputs a rule requires.
// Graph-only rules can avoid the heavier dependency parsing and per-module
// symbol index build that dead-code style rules need.
type ModuleAwareNeeds struct {
	NeedsFiles        bool
	NeedsDependencies bool
	NeedsIndex        bool
}

// ModuleAwareRuleTuning is an optional interface that lets module-aware rules
// declare whether they actually need module files, dependency metadata, or
// per-module symbol indexes. Rules that do not implement it default to the
// most conservative behavior.
type ModuleAwareRuleTuning interface {
	ModuleAwareNeeds() ModuleAwareNeeds
}

// CollectModuleAwareNeeds collapses the requirements for the currently-active
// module-aware rules so callers can avoid paying for unused analysis stages.
func CollectModuleAwareNeeds(activeRules []Rule) ModuleAwareNeeds {
	var needs ModuleAwareNeeds
	for _, rule := range activeRules {
		if _, ok := rule.(interface {
			SetModuleIndex(pmi *module.PerModuleIndex)
			CheckModuleAware() []scanner.Finding
		}); !ok {
			continue
		}
		current := ModuleAwareNeeds{
			NeedsFiles:        true,
			NeedsDependencies: true,
			NeedsIndex:        true,
		}
		if tuned, ok := rule.(interface {
			ModuleAwareNeeds() ModuleAwareNeeds
		}); ok {
			current = tuned.ModuleAwareNeeds()
		}
		if current.NeedsIndex {
			current.NeedsFiles = true
		}
		needs.NeedsFiles = needs.NeedsFiles || current.NeedsFiles
		needs.NeedsDependencies = needs.NeedsDependencies || current.NeedsDependencies
		needs.NeedsIndex = needs.NeedsIndex || current.NeedsIndex
	}
	return needs
}

// CollectModuleAwareNeedsV2 collapses the requirements for v2 module-aware
// rules so callers can avoid paying for unused analysis stages.
func CollectModuleAwareNeedsV2(activeRules []*v2.Rule) ModuleAwareNeeds {
	var needs ModuleAwareNeeds
	for _, r := range activeRules {
		if r == nil || !r.Needs.Has(v2.NeedsModuleIndex) {
			continue
		}
		// Wrap as a v1-compat value to reuse ModuleAwareRuleTuning detection.
		v1r, ok := v2.ToV1(r).(Rule)
		if !ok {
			// Not a v1 Rule — default to most conservative (needs everything).
			needs.NeedsFiles = true
			needs.NeedsDependencies = true
			needs.NeedsIndex = true
			continue
		}
		current := ModuleAwareNeeds{
			NeedsFiles:        true,
			NeedsDependencies: true,
			NeedsIndex:        true,
		}
		if tuned, ok := v1r.(ModuleAwareRuleTuning); ok {
			current = tuned.ModuleAwareNeeds()
		}
		if current.NeedsIndex {
			current.NeedsFiles = true
		}
		needs.NeedsFiles = needs.NeedsFiles || current.NeedsFiles
		needs.NeedsDependencies = needs.NeedsDependencies || current.NeedsDependencies
		needs.NeedsIndex = needs.NeedsIndex || current.NeedsIndex
	}
	return needs
}

// ModuleDeadCodeRule detects dead code with module-boundary awareness.
// It categorises symbols as truly-dead, could-be-internal, or dead-internal.
type ModuleDeadCodeRule struct {
	BaseRule
	pmi *module.PerModuleIndex
}

func (r *ModuleDeadCodeRule) IsFixable() bool { return false }

// Confidence reports a tier-2 (medium) base confidence for the same
// reason as DeadCode: the module-aware analyzer has no DI annotation
// awareness, so Dagger/Hilt/Anvil bindings that are wired at compile
// time by the DI framework look unreferenced to the per-module
// index. Classified per roadmap/17.
func (r *ModuleDeadCodeRule) Confidence() float64 { return 0.75 }

func (r *ModuleDeadCodeRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{
		NeedsFiles:        true,
		NeedsDependencies: true,
		NeedsIndex:        true,
	}
}

// Check is a no-op; analysis happens in CheckModuleAware.
func (r *ModuleDeadCodeRule) Check(_ *scanner.File) []scanner.Finding {
	return nil
}

// SetModuleIndex injects the per-module index for later analysis.
func (r *ModuleDeadCodeRule) SetModuleIndex(pmi *module.PerModuleIndex) {
	r.pmi = pmi
}

// CheckModuleAware runs module-aware dead code detection and returns findings.
func (r *ModuleDeadCodeRule) CheckModuleAware() []scanner.Finding {
	if r.pmi == nil || r.pmi.Graph == nil {
		return nil
	}

	var findings []scanner.Finding

	for modPath, idx := range r.pmi.ModuleIndex {
		mod := r.pmi.Graph.Modules[modPath]
		// Skip published modules: their public API is intended for external consumers
		if mod != nil && mod.IsPublished {
			continue
		}

		consumers := r.pmi.Graph.Consumers[modPath]

		for _, sym := range idx.Symbols {
			if shouldSkipSymbol(sym) {
				continue
			}
			if sym.Visibility == "private" {
				continue // handled by single-file rules
			}

			category := classifySymbol(sym, modPath, idx, consumers, r.pmi)
			if category == "" {
				continue // symbol is used, not dead
			}

			msg := formatModuleDeadCodeMsg(sym, modPath, category)
			findings = append(findings, scanner.Finding{
				File:     sym.File,
				Line:     sym.Line,
				Col:      1,
				RuleSet:  r.RuleSetName,
				Rule:     r.RuleName,
				Severity: r.Sev,
				Message:  msg,
			})
		}
	}

	return findings
}

// classifySymbol determines the dead-code category for a symbol.
// Returns "" if the symbol is alive (used by a consumer or used within its module).
func classifySymbol(
	sym scanner.Symbol,
	modPath string,
	modIndex *scanner.CodeIndex,
	consumers []string,
	pmi *module.PerModuleIndex,
) string {
	// Check if any consumer module references the symbol
	usedByConsumer := false
	for _, consumerPath := range consumers {
		consumerIdx := pmi.ModuleIndex[consumerPath]
		if consumerIdx == nil {
			continue
		}
		if consumerIdx.ReferenceCount(sym.Name) > 0 {
			usedByConsumer = true
			break
		}
	}

	// Check if the symbol is used within its own module (in a different file)
	usedInOwnModule := modIndex.IsReferencedOutsideFile(sym.Name, sym.File)

	switch {
	case usedByConsumer:
		// Alive: used by at least one consumer
		return ""
	case !usedInOwnModule && len(consumers) == 0:
		// No consumers and not used within the module
		return "truly-dead"
	case !usedInOwnModule && len(consumers) > 0:
		// Has consumers but none use it, and own module doesn't either
		return "truly-dead"
	case usedInOwnModule && sym.Visibility != "internal" && !usedByConsumer:
		// Public symbol used only within its own module; could be narrowed to internal
		return "could-be-internal"
	case sym.Visibility == "internal" && !usedInOwnModule:
		// Internal symbol not used within its own module
		return "dead-internal"
	default:
		// Used within own module and visibility is already internal — alive
		return ""
	}
}

func formatModuleDeadCodeMsg(sym scanner.Symbol, modPath, category string) string {
	vis := strings.Title(sym.Visibility)
	switch category {
	case "truly-dead":
		return fmt.Sprintf("%s %s '%s' in module %s is not used by any module (including itself).",
			vis, sym.Kind, sym.Name, modPath)
	case "could-be-internal":
		return fmt.Sprintf("%s %s '%s' in module %s is used only within the module. Consider making it internal.",
			vis, sym.Kind, sym.Name, modPath)
	case "dead-internal":
		return fmt.Sprintf("Internal %s '%s' in module %s is not used within the module.",
			sym.Kind, sym.Name, modPath)
	default:
		return fmt.Sprintf("%s %s '%s' in module %s appears unused.", vis, sym.Kind, sym.Name, modPath)
	}
}
