package deadcode

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/fixer"
	"github.com/kaeawc/krit/internal/scanner"
)

var deadCodeMessagePattern = regexp.MustCompile(`^[A-Z][a-z]+ ([a-z]+) '([^']+)'`)

// Candidate is a directly removable dead-code finding.
type Candidate struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Rule    string `json:"rule"`
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Finding scanner.Finding
}

// BlockedCandidate is a dead-code finding that is intentionally not removed.
type BlockedCandidate struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Rule   string `json:"rule"`
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// KindCount is a deterministic count bucket for candidate kinds.
type KindCount struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

// ReasonCount is a deterministic count bucket for blocked reasons.
type ReasonCount struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// Summary is a compact overview of a dead-code removal plan.
type Summary struct {
	Declarations int           `json:"declarations"`
	Files        int           `json:"files"`
	Kinds        []KindCount   `json:"kinds"`
	Blocked      int           `json:"blocked"`
	Reasons      []ReasonCount `json:"reasons"`
}

// ApplyResult captures the outcome of applying a plan.
type ApplyResult struct {
	Declarations int
	Files        int
	Errors       []error
}

// Plan separates directly removable dead code from findings that still need
// additional safety work before bulk deletion can use them.
type Plan struct {
	Candidates []Candidate        `json:"candidates"`
	Blocked    []BlockedCandidate `json:"blockedCandidates"`

	candidateColumns scanner.FindingColumns
}

// BuildPlanColumns classifies columnar dead-code findings into immediately
// removable entries and blocked entries without reconstructing all rows.
func BuildPlanColumns(columns *scanner.FindingColumns) Plan {
	if columns == nil || columns.Len() == 0 {
		return Plan{}
	}

	var candidates []Candidate
	candidateCollector := scanner.NewFindingCollector(columns.Len())
	removableKeys := make(map[string]bool)

	for row := 0; row < columns.Len(); row++ {
		rule := columns.RuleAt(row)
		if rule != "DeadCode" || !columns.HasFix(row) {
			continue
		}
		file := columns.FileAt(row)
		line := columns.LineAt(row)
		kind, name := classifyFindingMessage(columns.MessageAt(row))
		key := planKey(file, line, kind, name)
		if removableKeys[key] {
			continue
		}
		removableKeys[key] = true
		candidateCollector.AppendRow(columns, row)
		finding := columns.Finding(row)
		candidates = append(candidates, Candidate{
			File:    file,
			Line:    line,
			Rule:    rule,
			Kind:    kind,
			Name:    name,
			Finding: finding,
		})
	}

	var blocked []BlockedCandidate
	seenBlocked := make(map[string]bool)
	for row := 0; row < columns.Len(); row++ {
		rule := columns.RuleAt(row)
		if rule != "DeadCode" && rule != "ModuleDeadCode" {
			continue
		}
		file := columns.FileAt(row)
		line := columns.LineAt(row)
		message := columns.MessageAt(row)
		kind, name := classifyFindingMessage(message)
		key := planKey(file, line, kind, name)
		if removableKeys[key] || seenBlocked[key] {
			continue
		}
		reason, ok := blockedReasonFor(rule, columns.HasFix(row), message)
		if !ok {
			continue
		}
		seenBlocked[key] = true
		blocked = append(blocked, BlockedCandidate{
			File:   file,
			Line:   line,
			Rule:   rule,
			Kind:   kind,
			Name:   name,
			Reason: reason,
		})
	}

	sortCandidates(candidates)
	sortBlocked(blocked)

	return Plan{
		Candidates:       candidates,
		Blocked:          blocked,
		candidateColumns: *candidateCollector.Columns(),
	}
}

// Summary returns stable aggregate counts for the plan.
func (p Plan) Summary() Summary {
	files := make(map[string]bool)
	kindCounts := make(map[string]int)
	for _, candidate := range p.Candidates {
		files[candidate.File] = true
		kind := candidate.Kind
		if kind == "" {
			kind = "declaration"
		}
		kindCounts[kind]++
	}

	reasonCounts := make(map[string]int)
	for _, blocked := range p.Blocked {
		reasonCounts[blocked.Reason]++
	}

	return Summary{
		Declarations: len(p.Candidates),
		Files:        len(files),
		Kinds:        sortKindCounts(kindCounts),
		Blocked:      len(p.Blocked),
		Reasons:      sortReasonCounts(reasonCounts),
	}
}

// Apply runs the plan through the shared fix engine.
func (p Plan) Apply(suffix string) ApplyResult {
	if p.candidateColumns.Len() == 0 {
		return ApplyResult{}
	}

	declarations, files, errors := fixer.ApplyAllFixesColumns(&p.candidateColumns, suffix)
	return ApplyResult{
		Declarations: declarations,
		Files:        files,
		Errors:       errors,
	}
}

func classifyFinding(finding scanner.Finding) (kind, name string) {
	return classifyFindingMessage(finding.Message)
}

func classifyFindingMessage(message string) (kind, name string) {
	matches := deadCodeMessagePattern.FindStringSubmatch(message)
	if len(matches) == 3 {
		return matches[1], matches[2]
	}
	return "declaration", ""
}

func blockedReason(finding scanner.Finding) (string, bool) {
	return blockedReasonFor(finding.Rule, finding.Fix != nil, finding.Message)
}

func blockedReasonFor(rule string, hasFix bool, message string) (string, bool) {
	switch rule {
	case "DeadCode":
		if !hasFix {
			return "finding has no safe delete fix", true
		}
		return "", false
	case "ModuleDeadCode":
		if strings.Contains(message, "Consider making it internal.") {
			return "visibility narrowing is not deletion", true
		}
		return "module-aware dead code is not safely removable yet", true
	default:
		return "", false
	}
}

func planKey(file string, line int, kind, name string) string {
	return fmt.Sprintf("%s:%d:%s:%s", file, line, kind, name)
}

func sortCandidates(candidates []Candidate) {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].File != candidates[j].File {
			return candidates[i].File < candidates[j].File
		}
		if candidates[i].Line != candidates[j].Line {
			return candidates[i].Line < candidates[j].Line
		}
		if candidates[i].Kind != candidates[j].Kind {
			return candidates[i].Kind < candidates[j].Kind
		}
		return candidates[i].Name < candidates[j].Name
	})
}

func sortBlocked(blocked []BlockedCandidate) {
	sort.Slice(blocked, func(i, j int) bool {
		if blocked[i].Reason != blocked[j].Reason {
			return blocked[i].Reason < blocked[j].Reason
		}
		if blocked[i].File != blocked[j].File {
			return blocked[i].File < blocked[j].File
		}
		if blocked[i].Line != blocked[j].Line {
			return blocked[i].Line < blocked[j].Line
		}
		return blocked[i].Name < blocked[j].Name
	})
}

func sortKindCounts(counts map[string]int) []KindCount {
	if len(counts) == 0 {
		return nil
	}
	order := map[string]int{
		"function":    0,
		"class":       1,
		"object":      2,
		"interface":   3,
		"property":    4,
		"declaration": 5,
	}
	out := make([]KindCount, 0, len(counts))
	for kind, count := range counts {
		out = append(out, KindCount{Kind: kind, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		oi, iok := order[out[i].Kind]
		oj, jok := order[out[j].Kind]
		if iok && jok && oi != oj {
			return oi < oj
		}
		if iok != jok {
			return iok
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func sortReasonCounts(counts map[string]int) []ReasonCount {
	if len(counts) == 0 {
		return nil
	}
	out := make([]ReasonCount, 0, len(counts))
	for reason, count := range counts {
		out = append(out, ReasonCount{Reason: reason, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Reason < out[j].Reason
	})
	return out
}
