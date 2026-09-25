package firchecks

// verdict.go — FIR-authoritative merge of checker findings into Go's.
//
// For every requested file the compiler analyzed cleanly (not crashed, not in
// ErrorFiles) and every rule the jar has a checker for (Result.Rules), FIR's
// findings are the verdict: Go findings for that (file, rule) are kept only
// where FIR confirms them, and FIR findings Go missed are added. Everywhere
// else Go's findings stand and FIR's are discarded. That includes the
// (file, rule) pairs in Result.RuleErrors, where the rule's checker threw:
// krit-fir isolates the exception to that rule and file, so only that pair
// falls back to Go.

import (
	"path/filepath"
	"sort"

	"github.com/kaeawc/krit/internal/scanner"
)

// VerdictInput is everything ApplyVerdict needs.
type VerdictInput struct {
	// Go is every finding produced before the FIR pass.
	Go []scanner.Finding
	// FIR is the checker result for Requested. nil leaves Go unchanged.
	FIR *Result
	// Requested is the absolute paths sent to the checker, spelled as the
	// checker reports them.
	Requested []string
	// Excluded lists files withheld from the checker (not in a JVM source
	// set). They are never authoritative; only counted.
	Excluded []string
	// DisplayPath maps a requested path to the spelling Go findings use for
	// that file. FIR-only findings are reported under it. Missing entries
	// keep the requested spelling.
	DisplayPath map[string]string
	// Suppressed reports whether a FIR-only finding (File in display
	// spelling) is silenced by the filters Go findings already passed:
	// @Suppress / @SuppressWarnings / `detekt:` / "all" annotations, inline
	// krit:ignore comments, and per-rule excludes. nil suppresses nothing.
	Suppressed func(scanner.Finding) bool
}

// RuleVerdict counts what the merge did for one rule.
type RuleVerdict struct {
	// Confirmed: a Go finding matched by a FIR finding (kept, Go's form).
	Confirmed int
	// EnrichedWithFix: confirmed findings that carry Go's autofix.
	EnrichedWithFix int
	// GoDropped: Go findings FIR's verdict says are clean.
	GoDropped int
	// FirAdded: FIR findings with no Go counterpart, added.
	FirAdded int
	// Suppressed: FIR-only findings dropped by suppression or excludes.
	Suppressed int
	// RuleErrorFiles: authoritative files where this rule's checker threw;
	// Go's findings for the rule stand there.
	RuleErrorFiles int
}

// RuleError is one (file, rule) pair whose checker threw.
type RuleError struct {
	File, Rule, Message string
}

// VerdictStats summarizes an ApplyVerdict call.
type VerdictStats struct {
	// Rules is keyed by every authoritative rule, even when all counts are 0.
	Rules map[string]*RuleVerdict
	// AuthoritativeFiles is the number of requested files FIR owns.
	AuthoritativeFiles int
	// GatedFiles maps each requested file whose verdict was not trusted to
	// the reason (crash or first compiler error).
	GatedFiles map[string]string
	// ExcludedFiles is len(VerdictInput.Excluded).
	ExcludedFiles int
	// RuleErrors lists, sorted by file then rule, the (file, rule) pairs on
	// authoritative files and rules that fell back to Go because the rule's
	// checker threw there.
	RuleErrors []RuleError
}

type fileRule struct{ file, rule string }

// ApplyVerdict returns the merged findings: Go findings outside the
// authoritative (file, rule) pairs in their original order, followed by the
// verdict for the authoritative pairs sorted by file, position, and rule.
//
// Matching is one-to-one. A FIR finding first claims an unclaimed Go finding
// for the same (file, rule) whose byte range overlaps its own; a FIR finding
// left over then claims an unclaimed Go finding on the same line (the
// compiler often anchors on a callee or argument token while the tree-sitter
// rule anchors on the enclosing expression). A matched pair is emitted as the
// Go finding unchanged, so its message, range, confidence, and autofix
// survive; an unmatched FIR finding is emitted in its own form after the
// suppression check; an unmatched Go finding is dropped.
func ApplyVerdict(in VerdictInput) ([]scanner.Finding, VerdictStats) {
	stats := VerdictStats{
		Rules:         map[string]*RuleVerdict{},
		GatedFiles:    map[string]string{},
		ExcludedFiles: len(in.Excluded),
	}
	if in.FIR == nil {
		return in.Go, stats
	}
	authoritativeFile := gateFiles(in.Requested, in.FIR, &stats)
	authoritativeRule := make(map[string]bool, len(in.FIR.Rules))
	for _, rule := range in.FIR.Rules {
		authoritativeRule[rule] = true
		stats.Rules[rule] = &RuleVerdict{}
	}
	if len(authoritativeFile) == 0 || len(authoritativeRule) == 0 {
		return in.Go, stats
	}
	threw := collectRuleErrors(in.FIR.RuleErrors, authoritativeFile, authoritativeRule, &stats)
	owned := func(file, rule string) bool {
		return authoritativeFile[file] && authoritativeRule[rule] && !threw[fileRule{file, rule}]
	}

	out, goByKey := partitionGoFindings(in.Go, owned)
	firByKey := groupFIRFindings(in.FIR.Findings, owned)
	keys := make([]fileRule, 0, len(goByKey)+len(firByKey))
	for k := range goByKey {
		keys = append(keys, k)
	}
	for k := range firByKey {
		if _, ok := goByKey[k]; !ok {
			keys = append(keys, k)
		}
	}
	var verdict []scanner.Finding
	for _, k := range keys {
		verdict = append(verdict, decide(goByKey[k], firByKey[k], in, stats.Rules[k.rule])...)
	}
	sortByPosition(verdict)
	return append(out, verdict...), stats
}

// gateFiles returns the requested files whose verdict FIR owns and records
// the others (crashed, or with a compiler error) in stats.GatedFiles.
func gateFiles(requested []string, fir *Result, stats *VerdictStats) map[string]bool {
	authoritative := make(map[string]bool, len(requested))
	for _, path := range requested {
		if msg, ok := fir.Crashed[path]; ok {
			stats.GatedFiles[path] = "crashed: " + msg
			continue
		}
		if msg, ok := fir.ErrorFiles[path]; ok {
			stats.GatedFiles[path] = msg
			continue
		}
		authoritative[path] = true
	}
	stats.AuthoritativeFiles = len(authoritative)
	return authoritative
}

// collectRuleErrors returns the authoritative (file, rule) pairs whose
// checker threw and records them in stats. Pairs on gated files or
// unadvertised rules are already Go's and are not counted.
func collectRuleErrors(ruleErrors map[string]map[string]string, files, rules map[string]bool, stats *VerdictStats) map[fileRule]bool {
	threw := map[fileRule]bool{}
	for rule, byPath := range ruleErrors {
		if !rules[rule] {
			continue
		}
		for path, msg := range byPath {
			if !files[path] {
				continue
			}
			threw[fileRule{path, rule}] = true
			stats.Rules[rule].RuleErrorFiles++
			stats.RuleErrors = append(stats.RuleErrors, RuleError{File: path, Rule: rule, Message: msg})
		}
	}
	sort.Slice(stats.RuleErrors, func(i, j int) bool {
		a, b := stats.RuleErrors[i], stats.RuleErrors[j]
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Rule < b.Rule
	})
	return threw
}

// partitionGoFindings splits Go findings into the ones FIR does not own
// (kept in order) and the owned ones grouped by absolute file and rule. Go
// findings carry the scan's spelling of the path (often relative); the
// checker was sent absolute paths.
func partitionGoFindings(goFindings []scanner.Finding, owned func(file, rule string) bool) ([]scanner.Finding, map[fileRule][]scanner.Finding) {
	absByFile := map[string]string{}
	kept := make([]scanner.Finding, 0, len(goFindings))
	byKey := map[fileRule][]scanner.Finding{}
	for _, g := range goFindings {
		abs, ok := absByFile[g.File]
		if !ok {
			var err error
			if abs, err = filepath.Abs(g.File); err != nil {
				abs = g.File
			}
			absByFile[g.File] = abs
		}
		if owned(abs, g.Rule) {
			k := fileRule{abs, g.Rule}
			byKey[k] = append(byKey[k], g)
			continue
		}
		kept = append(kept, g)
	}
	return kept, byKey
}

// groupFIRFindings groups the owned FIR findings by file and rule, dropping
// exact repeats (the compiler can report one diagnostic twice).
func groupFIRFindings(findings []scanner.Finding, owned func(file, rule string) bool) map[fileRule][]scanner.Finding {
	type firKey struct {
		file, rule, message string
		line, col           int
	}
	seen := map[firKey]bool{}
	byKey := map[fileRule][]scanner.Finding{}
	for _, f := range findings {
		fk := firKey{f.File, f.Rule, f.Message, f.Line, f.Col}
		if !owned(f.File, f.Rule) || seen[fk] {
			continue
		}
		seen[fk] = true
		k := fileRule{f.File, f.Rule}
		byKey[k] = append(byKey[k], f)
	}
	return byKey
}

// decide applies the verdict to one owned (file, rule) pair.
func decide(goFindings, firFindings []scanner.Finding, in VerdictInput, rs *RuleVerdict) []scanner.Finding {
	sortByPosition(goFindings)
	sortByPosition(firFindings)
	goForFIR := matchFindings(goFindings, firFindings)
	matchedGo := make([]bool, len(goFindings))
	var out []scanner.Finding
	for i, f := range firFindings {
		if j := goForFIR[i]; j >= 0 {
			g := goFindings[j]
			matchedGo[j] = true
			out = append(out, g)
			rs.Confirmed++
			if g.Fix != nil || g.BinaryFix != nil {
				rs.EnrichedWithFix++
			}
			continue
		}
		if display, ok := in.DisplayPath[f.File]; ok {
			f.File = display
		}
		if in.Suppressed != nil && in.Suppressed(f) {
			rs.Suppressed++
			continue
		}
		out = append(out, f)
		rs.FirAdded++
	}
	for _, matched := range matchedGo {
		if !matched {
			rs.GoDropped++
		}
	}
	return out
}

// matchFindings pairs each FIR finding with at most one Go finding and
// returns, per FIR finding, the Go index or -1. Byte-range overlaps are
// claimed first across all FIR findings, then same-line pairs.
func matchFindings(goFindings, firFindings []scanner.Finding) []int {
	goForFIR := make([]int, len(firFindings))
	for i := range goForFIR {
		goForFIR[i] = -1
	}
	claimed := make([]bool, len(goFindings))
	claim := func(match func(f, g scanner.Finding) bool) {
		for i, f := range firFindings {
			if goForFIR[i] >= 0 {
				continue
			}
			for j, g := range goFindings {
				if !claimed[j] && match(f, g) {
					claimed[j] = true
					goForFIR[i] = j
					break
				}
			}
		}
	}
	claim(func(f, g scanner.Finding) bool { return hasRange(f) && hasRange(g) && rangesOverlap(f, g) })
	claim(func(f, g scanner.Finding) bool { return f.Line == g.Line })
	return goForFIR
}

func hasRange(f scanner.Finding) bool { return f.EndByte > f.StartByte }

func rangesOverlap(a, b scanner.Finding) bool {
	return a.StartByte < b.EndByte && b.StartByte < a.EndByte
}

func sortByPosition(findings []scanner.Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Col != b.Col {
			return a.Col < b.Col
		}
		if a.StartByte != b.StartByte {
			return a.StartByte < b.StartByte
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		return a.Message < b.Message
	})
}
