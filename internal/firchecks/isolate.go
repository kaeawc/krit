package firchecks

// isolate.go — recheck the projects whose merged compile reports a duplicate
// declaration with each scanned source set compiled on its own (#972).

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/scanner"
)

// SourceSetIsolator rechecks, with each scanned source set compiled on its
// own, the projects that own any of the gated files. request is a check
// request in the daemon's shape for every file the pass checks. It returns
// the check response for the files it rechecked and those files; both are
// empty when no project qualified, and the caller keeps the merged verdicts.
type SourceSetIsolator func(ctx context.Context, jarPath string, gated []string, request []byte) (response []byte, rechecked []string, err error)

// isolateRequestID is the request ID of a source-set isolation recheck.
const isolateRequestID = 2

// duplicateDeclaration reports whether a gating reason is a duplicate
// declaration, the error a merged compile of sibling source sets produces.
func duplicateDeclaration(reason string) bool {
	line := firstLine(reason)
	return strings.HasPrefix(line, "Redeclaration:") || strings.HasPrefix(line, "Conflicting overloads:")
}

// duplicateDeclarationGated returns the files result gates on a duplicate
// declaration, sorted.
func duplicateDeclarationGated(result *Result) []string {
	var out []string
	for path, reason := range result.ErrorFiles {
		if duplicateDeclaration(reason) {
			out = append(out, path)
		}
	}
	sort.Strings(out)
	return out
}

// isolationOutcome summarizes a recheck for the verbose log.
type isolationOutcome struct {
	gated, rechecked, stillGated int
	duration                     time.Duration
	err                          error
	// still lists the gated files the recheck left gated on a duplicate
	// declaration, with that reason.
	still []string
}

// isolateSourceSets replaces result's verdicts for the projects the isolator
// rechecks. Any failure keeps the merged verdicts, which only gate files.
func isolateSourceSets(ctx context.Context, opts PassOptions, requested, sourceDirs, classpath, rules []string, configs RuleConfigs, facts FileFacts, result *Result) (out isolationOutcome) {
	gated := duplicateDeclarationGated(result)
	out.gated = len(gated)
	if len(gated) == 0 {
		return out
	}
	began := time.Now()
	defer func() { out.duration = time.Since(began) }()
	jarPath, target := "", CompileTarget{}
	if checker, ok := opts.Checker.(*ProductionFirChecker); ok {
		jarPath, target = checker.JarPath, checker.CompileTarget()
	}
	refs := make([]fileRef, len(requested))
	for i, path := range requested {
		refs[i] = fileRef{Path: path}
	}
	request, spelling, err := encodeCheckRequest(isolateRequestID, refs, sourceDirs, classpath, rules, configs, facts, target)
	if err != nil {
		out.err = err
		return out
	}
	data, rechecked, err := opts.Isolator(ctx, jarPath, oracle.AbsolutePaths(gated), request)
	if err != nil || len(rechecked) == 0 {
		out.err = err
		return out
	}
	resp, err := decodeCheckResponse(data, isolateRequestID, spelling)
	if err != nil {
		out.err = err
		return out
	}
	if missingKnownFIRRules(rules, resp) {
		out.err = fmt.Errorf("the isolated check registered no checkers")
		return out
	}
	files := make([]string, len(rechecked))
	for i, path := range rechecked {
		files[i] = spelling.Caller(path)
	}
	result.replaceFiles(files, resp)
	out.rechecked = len(files)
	for _, path := range gated {
		if reason := result.ErrorFiles[path]; duplicateDeclaration(reason) {
			out.stillGated++
			out.still = append(out.still, path+": "+strings.Join(strings.Fields(reason), " "))
		}
	}
	return out
}

// replaceFiles swaps r's verdicts for files with resp's. The incremental
// account stays the merged compile's.
func (r *Result) replaceFiles(files []string, resp *CheckResponse) {
	drop := make(map[string]bool, len(files))
	for _, path := range files {
		drop[path] = true
		delete(r.Crashed, path)
		delete(r.ErrorFiles, path)
	}
	r.Findings = slices.DeleteFunc(r.Findings, func(f scanner.Finding) bool { return drop[f.File] })
	for rule, byPath := range r.RuleErrors {
		for path := range byPath {
			if drop[path] {
				delete(byPath, path)
			}
		}
		if len(byPath) == 0 {
			delete(r.RuleErrors, rule)
		}
	}
	contentCache := map[string][]byte{}
	for _, f := range resp.Findings {
		if drop[f.Path] {
			r.Findings = append(r.Findings, toScannerFindingWithRange(f, contentCache))
		}
	}
	for path, msg := range resp.Crashed {
		if drop[path] {
			r.Crashed[path] = msg
		}
	}
	for path := range resp.ErrorFiles {
		if drop[path] {
			r.ErrorFiles[path] = errorFileMessage(resp.ErrorFiles, path)
		}
	}
	for rule, byPath := range resp.RuleErrors {
		for path, msg := range byPath {
			if drop[path] {
				r.addRuleError(rule, path, msg)
			}
		}
	}
	r.addRules(resp.Rules)
}

// writeIsolationOutcome writes the recheck summary and a sample of the files it left gated.
func writeIsolationOutcome(w io.Writer, o isolationOutcome) {
	fmt.Fprintf(w, "verbose: FIR source sets: %s\n", o)
	for i, line := range o.still {
		if i == maxGatedFilesListed {
			fmt.Fprintf(w, "verbose: FIR source sets: still gated: ... and %d more\n", len(o.still)-maxGatedFilesListed)
			break
		}
		fmt.Fprintf(w, "verbose: FIR source sets: still gated: %s\n", line)
	}
}

func (o isolationOutcome) String() string {
	if o.err != nil {
		return fmt.Sprintf("%d files gated by duplicate declarations; isolated recheck not applied (%v)", o.gated, o.err)
	}
	if o.rechecked == 0 {
		return fmt.Sprintf("%d files gated by duplicate declarations; no project qualified for an isolated recheck", o.gated)
	}
	return fmt.Sprintf("%d files gated by duplicate declarations; rechecked %d files with each source set compiled on its own in %v, %d still gated by one",
		o.gated, o.rechecked, o.duration.Round(time.Millisecond), o.stillGated)
}
