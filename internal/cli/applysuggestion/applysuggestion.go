// Package applysuggestion implements `krit apply-suggestion`, a CLI verb
// that inspects and applies a single rule-emitted suggested fix from a
// JSON findings report.
//
// Suggested fixes are intentionally separate from the autofix slot driven
// by `krit --fix`: autofixes apply automatically while suggestions require
// an explicit user/tool selection. This command provides that selection
// surface — both for direct CLI use (`krit --format=json . > findings.json
// && krit apply-suggestion --finding <id> --suggestion <id> findings.json`)
// and for IDE/LSP wrappers that need a deterministic way to act on one
// suggestion at a time.
//
// The command reuses the autofix application path (fixer.ApplyAllFixesColumns)
// so cross-file edits, byte/line mode handling, overlap deduplication, and
// ktfmt-shaped output stay consistent with `--fix`.
package applysuggestion

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/fixer"
	"github.com/kaeawc/krit/internal/output"
	"github.com/kaeawc/krit/internal/scanner"
)

func Run(args []string) int { return run(args, os.Stdin, os.Stdout, os.Stderr) }

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("apply-suggestion", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: krit apply-suggestion [--list] [--finding ID --suggestion ID] [--dry-run] [--base DIR] [report.json|-]")
		fs.PrintDefaults()
	}
	list := fs.Bool("list", false, "List findings with suggestions and their ids.")
	dryRun := fs.Bool("dry-run", false, "Print the edits that would be applied without modifying any files.")
	findingID := fs.String("finding", "", "Finding id (rule:file:line:column) to target. Required unless --list is set.")
	suggestion := fs.String("suggestion", "", "Suggestion id to apply. Required unless --list is set.")
	base := fs.String("base", "", "Base directory for resolving relative paths in the report (default: current directory).")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	rest := fs.Args()
	if len(rest) > 1 {
		fmt.Fprintf(stderr, "apply-suggestion: expected at most one report path, got %d\n", len(rest))
		return 1
	}
	source := ""
	if len(rest) == 1 {
		source = rest[0]
	}

	if !*list && (*findingID == "" || *suggestion == "") {
		fmt.Fprintln(stderr, "apply-suggestion: --finding and --suggestion are required unless --list is given")
		fs.Usage()
		return 1
	}

	report, err := decodeReport(source, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "apply-suggestion: %v\n", err)
		return 1
	}

	if *list {
		printSuggestionsList(stdout, report)
		return 0
	}

	finding, ok := findFindingByID(report, *findingID)
	if !ok {
		writeStaleError(stderr, "finding", *findingID, listFindingIDsWithSuggestions(report))
		return 1
	}
	sug, ok := findSuggestion(finding, *suggestion)
	if !ok {
		writeStaleError(stderr, "suggestion", *suggestion, listSuggestionIDs(finding))
		return 1
	}
	if len(sug.Edits) == 0 {
		fmt.Fprintf(stderr, "apply-suggestion: suggestion %q for finding %q is not machine-applicable (no edits)\n",
			*suggestion, *findingID)
		if sug.ApplicationToken != "" {
			fmt.Fprintf(stderr, "  applicationToken: %s\n", sug.ApplicationToken)
		}
		return 1
	}

	baseDir, err := resolveBaseDir(*base)
	if err != nil {
		fmt.Fprintf(stderr, "apply-suggestion: %v\n", err)
		return 1
	}

	edits := resolveSuggestionEdits(finding.File, sug, baseDir)
	if *dryRun {
		printDryRun(stdout, finding, sug, edits)
		return 0
	}

	cols := buildFindingColumns(finding, sug, edits)
	applied, _, dropped, errs := fixer.ApplyAllFixesColumnsDetailed(context.Background(), &cols, "")
	for _, e := range errs {
		fmt.Fprintf(stderr, "apply-suggestion: %v\n", e)
	}
	if len(errs) > 0 {
		return 1
	}
	for _, d := range dropped {
		reason := d.Reason
		if reason == "" {
			reason = "overlapping conflict"
		}
		fmt.Fprintf(stderr, "apply-suggestion: warning: dropped edit from %s at %s:%d because %s\n",
			d.Rule, d.File, d.Line, reason)
	}
	appliedTargets := summarizeAppliedTargets(edits, dropped)
	fmt.Fprintf(stdout, "applied suggestion %q for finding %q (%d edit(s) across %s)\n",
		sug.ID, *findingID, applied, appliedTargets)
	if len(dropped) > 0 {
		fmt.Fprintf(stdout, "  %d edit(s) dropped due to overlap; see warnings above\n", len(dropped))
	}
	return 0
}

func resolveBaseDir(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	return os.Getwd()
}

func decodeReport(source string, stdin io.Reader) (output.JSONReport, error) {
	var rdr io.ReadCloser
	if source == "" || source == "-" {
		rdr = io.NopCloser(stdin)
	} else {
		f, err := os.Open(source)
		if err != nil {
			return output.JSONReport{}, fmt.Errorf("opening %s: %w", source, err)
		}
		rdr = f
	}
	defer rdr.Close()
	var report output.JSONReport
	if err := json.NewDecoder(rdr).Decode(&report); err != nil {
		return output.JSONReport{}, fmt.Errorf("decoding findings: %w", err)
	}
	return report, nil
}

func findFindingByID(report output.JSONReport, id string) (output.JSONFinding, bool) {
	for _, f := range report.Findings {
		if output.FindingID(f) == id {
			return f, true
		}
	}
	return output.JSONFinding{}, false
}

func findSuggestion(f output.JSONFinding, id string) (output.JSONSuggestedFix, bool) {
	for _, s := range f.SuggestedFixes {
		if s.ID == id {
			return s, true
		}
	}
	return output.JSONSuggestedFix{}, false
}

func listFindingIDsWithSuggestions(report output.JSONReport) []string {
	out := make([]string, 0, len(report.Findings))
	for _, f := range report.Findings {
		if len(f.SuggestedFixes) == 0 {
			continue
		}
		out = append(out, output.FindingID(f))
	}
	sort.Strings(out)
	return out
}

func listSuggestionIDs(f output.JSONFinding) []string {
	out := make([]string, 0, len(f.SuggestedFixes))
	for _, s := range f.SuggestedFixes {
		out = append(out, s.ID)
	}
	return out
}

func writeStaleError(w io.Writer, kind, id string, available []string) {
	fmt.Fprintf(w, "apply-suggestion: %s id %q not found in report\n", kind, id)
	if len(available) == 0 {
		return
	}
	fmt.Fprintln(w, "available:")
	for _, a := range available {
		fmt.Fprintf(w, "  %s\n", a)
	}
}

// resolveSuggestionEdits returns a copy of the suggestion's edits with
// TargetFile resolved against base when the rule emitted a relative
// path. The finding's own file is used as a fallback when an edit has
// no target file set (mirrors fixer.collectTextFixRows).
func resolveSuggestionEdits(findingFile string, sug output.JSONSuggestedFix, base string) []output.JSONSuggestedEdit {
	out := make([]output.JSONSuggestedEdit, len(sug.Edits))
	for i, e := range sug.Edits {
		target := e.TargetFile
		if target == "" {
			target = findingFile
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(base, target)
		}
		e.TargetFile = target
		out[i] = e
	}
	return out
}

// buildFindingColumns materializes a one-finding-per-edit FindingColumns
// where each row carries the suggestion edit as its autofix Fix. This
// lets fixer.ApplyAllFixesColumns dedupe overlaps, group by file, and
// emit ktfmt-shaped output through the same path `--fix` uses.
func buildFindingColumns(finding output.JSONFinding, sug output.JSONSuggestedFix, edits []output.JSONSuggestedEdit) scanner.FindingColumns {
	syn := make([]scanner.Finding, 0, len(edits))
	for _, e := range edits {
		syn = append(syn, scanner.Finding{
			File:     e.TargetFile,
			Line:     finding.Line,
			Col:      finding.Column,
			RuleSet:  finding.RuleSet,
			Rule:     finding.Rule,
			Severity: finding.Severity,
			Message:  fmt.Sprintf("suggestion %s: %s", sug.ID, sug.Title),
			Fix: &scanner.Fix{
				TargetFile:  e.TargetFile,
				StartLine:   e.StartLine,
				EndLine:     e.EndLine,
				StartByte:   e.StartByte,
				EndByte:     e.EndByte,
				ByteMode:    e.ByteMode,
				Replacement: e.Replacement,
			},
		})
	}
	return scanner.CollectFindings(syn)
}

func printSuggestionsList(w io.Writer, report output.JSONReport) {
	// Wrap in bufio so a large report doesn't generate a write syscall
	// per Fprintf call (~tens of thousands on a kotlin-corpus report).
	bw := bufio.NewWriter(w)
	defer bw.Flush()
	printed := false
	for _, f := range report.Findings {
		if len(f.SuggestedFixes) == 0 {
			continue
		}
		printed = true
		fmt.Fprintf(bw, "%s  %s\n", output.FindingID(f), f.Message)
		for _, s := range f.SuggestedFixes {
			machine := "informational"
			if len(s.Edits) > 0 {
				machine = fmt.Sprintf("%d edit(s)", len(s.Edits))
			}
			fmt.Fprintf(bw, "  %s  %s  [%s]\n", s.ID, s.Title, machine)
		}
	}
	if !printed {
		fmt.Fprintln(bw, "no findings carry suggested fixes")
	}
}

func printDryRun(w io.Writer, finding output.JSONFinding, sug output.JSONSuggestedFix, edits []output.JSONSuggestedEdit) {
	fmt.Fprintf(w, "dry-run: finding %s\n", output.FindingID(finding))
	fmt.Fprintf(w, "  suggestion %s (%s)\n", sug.ID, sug.Title)
	for i, e := range edits {
		mode := "lines"
		span := fmt.Sprintf("%d-%d", e.StartLine, e.EndLine)
		if e.ByteMode {
			mode = "bytes"
			span = fmt.Sprintf("%d-%d", e.StartByte, e.EndByte)
		}
		fmt.Fprintf(w, "  edit %d: %s [%s %s]\n", i+1, e.TargetFile, mode, span)
		fmt.Fprintf(w, "    replacement: %s\n", quoteReplacement(e.Replacement))
	}
}

// quoteReplacement renders a multi-line replacement as a single-line
// JSON string so dry-run output stays scannable. json.Marshal on a
// string cannot fail.
func quoteReplacement(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func summarizeTargets(edits []output.JSONSuggestedEdit) string {
	seen := make(map[string]struct{}, len(edits))
	out := make([]string, 0, len(edits))
	for _, e := range edits {
		if _, ok := seen[e.TargetFile]; ok {
			continue
		}
		seen[e.TargetFile] = struct{}{}
		out = append(out, e.TargetFile)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// summarizeAppliedTargets is summarizeTargets minus any file whose only
// edits were all dropped. This keeps the "applied … across X" string in
// sync with the post-dedup count: if a file's sole edit was dropped, it
// did not actually get touched, so reporting it as a target would be
// just as misleading as the inflated edit count we're fixing.
func summarizeAppliedTargets(edits []output.JSONSuggestedEdit, dropped []fixer.DroppedFix) string {
	if len(dropped) == 0 {
		return summarizeTargets(edits)
	}
	droppedPerFile := make(map[string]int, len(dropped))
	for _, d := range dropped {
		droppedPerFile[d.File]++
	}
	editsPerFile := make(map[string]int, len(edits))
	for _, e := range edits {
		editsPerFile[e.TargetFile]++
	}
	kept := make([]output.JSONSuggestedEdit, 0, len(edits))
	for _, e := range edits {
		if droppedPerFile[e.TargetFile] >= editsPerFile[e.TargetFile] {
			continue
		}
		kept = append(kept, e)
	}
	if len(kept) == 0 {
		return "no files"
	}
	return summarizeTargets(kept)
}
