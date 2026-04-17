package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// GodClassOrModuleRule reports Kotlin source files that import from an
// unusually broad set of packages. This first slice models the "module"
// side of the concept; class-level ownership can reuse the same thresholding
// approach with AST ownership in a later iteration.
type GodClassOrModuleRule struct {
	LineBase
	BaseRule
	AllowedDistinctPackages int
}

// Confidence reports a tier-2 (medium) base confidence. Hotspot rule. Detection uses cross-file fan-in/fan-out metrics whose
// threshold is a project-sensitive heuristic. Classified per roadmap/17.
func (r *GodClassOrModuleRule) Confidence() float64 { return 0.75 }

func (r *GodClassOrModuleRule) CheckLines(file *scanner.File) []scanner.Finding {
	packages := make(map[string]struct{})
	firstImportLine := 0

	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "import ") {
			continue
		}
		if firstImportLine == 0 {
			firstImportLine = i + 1
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
		if imp == "" {
			continue
		}
		packages[imp] = struct{}{}
	}

	if len(packages) <= r.AllowedDistinctPackages {
		return nil
	}

	msg := fmt.Sprintf("Module imports from %d distinct packages; consider splitting responsibilities or narrowing dependencies.", len(packages))
	return []scanner.Finding{r.Finding(file, firstImportLine, 1, msg)}
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
func (r *FanInFanOutHotspotRule) Confidence() float64 { return 0.75 }

func (r *FanInFanOutHotspotRule) Check(file *scanner.File) []scanner.Finding {
	return nil
}

func (r *FanInFanOutHotspotRule) CheckCrossFile(index *scanner.CodeIndex) []scanner.Finding {
	stats := index.ClassLikeFanInStats(r.IgnoreCommentReferences)
	findings := make([]scanner.Finding, 0, len(stats))
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

		findings = append(findings, scanner.Finding{
			File:     stat.Symbol.File,
			Line:     stat.Symbol.Line,
			Col:      1,
			RuleSet:  r.RuleSetName,
			Rule:     r.RuleName,
			Severity: r.Sev,
			Message:  msg,
		})
	}
	return findings
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
