package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/sourceheader"
)

// GodClassOrModuleRule reports Kotlin source files that import from an
// unusually broad set of packages. This first slice models the "module"
// side of the concept; class-level ownership can reuse the same thresholding
// approach with AST ownership in a later iteration.
type GodClassOrModuleRule struct {
	FlatDispatchBase
	BaseRule
	AllowedDistinctPackages int
}

// Confidence reports a tier-2 (medium) base confidence. Hotspot rule. Detection uses cross-file fan-in/fan-out metrics whose
// threshold is a project-sensitive heuristic. Classified per roadmap/17.
func (r *GodClassOrModuleRule) Confidence() float64 { return api.ConfidenceMedium }

// Walks `import_header` AST nodes instead of `file.Lines` so the count
// excludes `import ` text inside block comments, KDoc, and raw-string
// literals — the original line-prefix scan miscounted those.
func (r *GodClassOrModuleRule) check(ctx *api.Context) {
	file := ctx.File
	packages := make(map[string]struct{})
	firstImportLine := 0

	file.FlatWalkNodes(ctx.Idx, "import_header", func(node uint32) {
		imp := sourceheader.FirstHeaderLine(file.FlatNodeText(node), "import")
		if imp == "" {
			return
		}
		if firstImportLine == 0 {
			firstImportLine = file.FlatRow(node) + 1
		}
		if idx := strings.Index(imp, " as "); idx >= 0 {
			imp = strings.TrimSpace(imp[:idx])
		}
		if strings.HasSuffix(imp, ".*") {
			imp = strings.TrimSuffix(imp, ".*")
		} else if lastDot := strings.LastIndex(imp, "."); lastDot > 0 {
			imp = imp[:lastDot]
		} else {
			return
		}
		if imp == "" {
			return
		}
		packages[imp] = struct{}{}
	})

	if len(packages) <= r.AllowedDistinctPackages {
		return
	}

	msg := fmt.Sprintf("Module imports from %d distinct packages; consider splitting responsibilities or narrowing dependencies.", len(packages))
	ctx.Emit(r.Finding(file, firstImportLine, 1, msg))
}

// FanInFanOutHotspotRule reports class-like declarations with unusually high
// fan-in across the project. This first slice handles threshold-based fan-in;
// a later pass can layer complexity scoring on the same substrate.
type FanInFanOutHotspotRule struct {
	BaseRule
	AllowedFanIn            int
	IgnoreCommentReferences bool
}

// Confidence reports a tier-2 (medium) base confidence. Hotspot rule. Detection uses cross-file fan-in/fan-out metrics whose
// threshold is a project-sensitive heuristic. Classified per roadmap/17.
func (r *FanInFanOutHotspotRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *FanInFanOutHotspotRule) check(ctx *api.Context) {
	index := ctx.CodeIndex
	stats := index.ClassLikeFanInStats(r.IgnoreCommentReferences)
	for _, stat := range stats {
		if stat.FanIn < r.AllowedFanIn {
			break
		}
		if stat.Symbol.Kind != "class" && stat.Symbol.Kind != "object" {
			continue
		}
		if isLikelyFrameworkEntryTypeName(stat.Symbol.Name) {
			continue
		}

		msg := fmt.Sprintf("%s '%s' has fan-in %d across %d external files",
			titleCaseKind(stat.Symbol.Kind), stat.Symbol.Name, stat.FanIn, stat.FanIn)
		if len(stat.ReferencingFiles) > 0 {
			msg += fmt.Sprintf(" (%s)", formatFanInExamples(stat.ReferencingFiles))
		}
		msg += "; consider splitting responsibilities or narrowing its API surface."

		ctx.Emit(scanner.Finding{
			File:     stat.Symbol.File,
			Line:     stat.Symbol.Line,
			Col:      1,
			RuleSet:  r.RuleSetName,
			Rule:     r.RuleName,
			Severity: r.Sev,
			Message:  msg,
		})
	}
}

func formatFanInExamples(files []string) string {
	const maxExamples = 3
	if len(files) <= maxExamples {
		return "referenced by " + strings.Join(files, ", ")
	}
	return fmt.Sprintf("referenced by %s and %d more", strings.Join(files[:maxExamples], ", "), len(files)-maxExamples)
}

func titleCaseKind(kind string) string {
	if kind == "" {
		return "Declaration"
	}
	return strings.ToUpper(kind[:1]) + kind[1:]
}
