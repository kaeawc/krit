package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/module"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

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

// CollectModuleAwareNeedsV2 collapses the requirements for v2 module-aware
// rules so callers can avoid paying for unused analysis stages.
func CollectModuleAwareNeedsV2(activeRules []*v2.Rule) ModuleAwareNeeds {
	var needs ModuleAwareNeeds
	for _, r := range activeRules {
		if r == nil || !r.Needs.Has(v2.NeedsModuleIndex) {
			continue
		}
		current := ModuleAwareNeeds{
			NeedsFiles:        true,
			NeedsDependencies: true,
			NeedsIndex:        true,
		}
		// Check if the concrete rule declares tuning preferences.
		if r.Implementation != nil {
			if tuned, ok := r.Implementation.(ModuleAwareRuleTuning); ok {
				current = tuned.ModuleAwareNeeds()
			}
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
}

func (r *ModuleDeadCodeRule) IsFixable() bool { return false }

// Confidence reports a tier-2 (medium) base confidence for the same
// reason as DeadCode: the module-aware analyzer relies on index evidence
// and local generated-use filters rather than a full compiler model.
func (r *ModuleDeadCodeRule) Confidence() float64 { return 0.75 }

func (r *ModuleDeadCodeRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{
		NeedsFiles:        true,
		NeedsDependencies: true,
		NeedsIndex:        true,
	}
}

// check is the v2 dispatch entry point.
func (r *ModuleDeadCodeRule) check(ctx *v2.Context) {
	pmi := ctx.ModuleIndex
	if pmi == nil || pmi.Graph == nil {
		return
	}
	filesByPath := deadCodeModuleFilesByPath(pmi.ModuleFiles)

	for modPath, idx := range pmi.ModuleIndex {
		mod := pmi.Graph.Modules[modPath]
		// Skip published modules: their public API is intended for external consumers
		if mod != nil && mod.IsPublished {
			continue
		}

		consumers := pmi.Graph.Consumers[modPath]

		for _, sym := range idx.Symbols {
			if isGradleBuildScript(sym.File) {
				continue
			}
			if shouldSkipSymbolWithFile(sym, filesByPath[sym.File]) {
				continue
			}
			if sym.Visibility == "private" {
				continue // handled by single-file rules
			}

			category := classifySymbol(sym, modPath, idx, consumers, pmi)
			if category == "" {
				continue // symbol is used, not dead
			}

			msg := formatModuleDeadCodeMsg(sym, modPath, category)
			ctx.Emit(scanner.Finding{
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
}

func deadCodeModuleFilesByPath(moduleFiles map[string][]*scanner.File) map[string]*scanner.File {
	total := 0
	for _, files := range moduleFiles {
		total += len(files)
	}
	filesByPath := make(map[string]*scanner.File, total)
	for _, files := range moduleFiles {
		for _, file := range files {
			if file == nil {
				continue
			}
			filesByPath[file.Path] = file
		}
	}
	return filesByPath
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
		if consumerIdx.SymbolReferenceCount(sym) > 0 {
			usedByConsumer = true
			break
		}
	}

	// Check if the symbol is used within its own module (in a different file)
	usedInOwnModule := modIndex.IsSymbolReferencedOutsideFile(sym, false) || modIndex.SymbolReferenceCount(sym) > 1

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
