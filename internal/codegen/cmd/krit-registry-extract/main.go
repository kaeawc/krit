// Command krit-registry-extract parses the rule source files in
// internal/rules/*.go, collects every statement inside a `func init()`
// body that calls v2.Register(...) (directly or via a nested block),
// and emits two outputs:
//
//  1. internal/rules/zz_registry_gen.go — a single generated file that
//     contains ONE init() function, consolidating every registration
//     call verbatim from the original source.
//
//  2. With -rewrite, it rewrites each original rule file in place to
//     delete the collected registration statements. Any init() body
//     left empty after rewriting is dropped entirely. Init() bodies
//     that still hold non-registration code (e.g. expectedParent map
//     setup) keep the non-registration statements intact.
//
// The extractor operates on original source bytes, so struct literals,
// adapter options, and comments inside individual registration blocks
// are preserved exactly as written.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "krit-registry-extract:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("krit-registry-extract", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		rulesDir = fs.String("rules", "internal/rules", "directory holding rule source files")
		outFile  = fs.String("out", "internal/rules/zz_registry_gen.go", "path for generated registry file")
		pkgName  = fs.String("package", "rules", "Go package name for generated file")
		rewrite  = fs.Bool("rewrite", false, "rewrite rule source files to delete extracted init() statements")
		verify   = fs.Bool("verify", false, "check that generated registry file on disk matches what would be generated")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Files to skip entirely during scanning/rewriting: they contain
	// init() functions that do NOT register rules, and their init()
	// MUST be preserved verbatim.
	skipFiles := map[string]bool{
		"zzz_v2bridge.go":       true, // bridges v2.Registry into v1
		"zzzz_defaults_init.go": true, // (historical: derive DefaultInactive)
		"zzz_registry_gen.go":   true, // guard against renames
	}
	// scanOnlyFiles are read for their registerAllRules() body (so re-
	// runs remain idempotent) but are never rewritten.
	scanOnlyFiles := map[string]bool{
		"zz_registry_gen.go": true,
	}

	entries, err := os.ReadDir(*rulesDir)
	if err != nil {
		return fmt.Errorf("read rules dir: %w", err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if strings.HasPrefix(name, "zz_meta_") && strings.HasSuffix(name, "_gen.go") {
			continue
		}
		if skipFiles[name] {
			continue
		}
		files = append(files, filepath.Join(*rulesDir, name))
	}
	sort.Strings(files)

	type fileExtract struct {
		path     string
		original []byte
		// Each []regStmt is one init() body's list of registration stmts.
		// initRanges[i] is the position of the "func init()" funcDecl.
		initFuncs []*initFuncInfo
		fset      *token.FileSet
		fileAST   *ast.File
	}

	type extraction struct {
		file     string // short filename only, for logging
		stmt     []byte // verbatim source bytes (trimmed)
		sortKey  string // stable sort key: filename + offset
		ruleName string // extracted rule ID (for logging / alias aware)
	}

	var files2 []*fileExtract
	var all []extraction

	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		fset := token.NewFileSet()
		astFile, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		fe := &fileExtract{
			path:     path,
			original: src,
			fset:     fset,
			fileAST:  astFile,
		}

		shortName := filepath.Base(path)
		scanOnly := scanOnlyFiles[shortName]
		// Track which banner (//--- from X.go ---) we last saw when
		// walking registerAllRules() so extractions carry their true
		// origin filename for ordering.
		curOrigin := shortName

		for _, decl := range astFile.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fd.Recv != nil {
				continue
			}
			if fd.Name == nil {
				continue
			}
			// For the generated registry file, scan registerAllRules();
			// for everything else, scan init().
			want := "init"
			if scanOnly {
				want = "registerAllRules"
			}
			if fd.Name.Name != want {
				continue
			}
			if fd.Body == nil {
				continue
			}

			info := &initFuncInfo{funcDecl: fd}
			for _, stmt := range fd.Body.List {
				if stmtContainsRegister(stmt) {
					// Capture original bytes.
					startOff := fset.Position(stmt.Pos()).Offset
					endOff := fset.Position(stmt.End()).Offset
					chunk := src[startOff:endOff]
					info.regStmts = append(info.regStmts, stmt)
					ruleName := extractRuleName(string(chunk))
					origin := shortName
					var sortKey string
					if scanOnly {
						// Recover origin filename from the nearest
						// preceding "// --- from X.go ---" banner comment.
						curOrigin = findBannerBefore(src, startOff, curOrigin)
						origin = curOrigin
						// Keep stable order: origin filename first, then
						// the offset within this registry file so order
						// round-trips across re-runs.
						sortKey = fmt.Sprintf("%s:%08d", origin, startOff)
					} else {
						sortKey = fmt.Sprintf("%s:%08d", origin, startOff)
					}
					all = append(all, extraction{
						file:     origin,
						stmt:     append([]byte(nil), chunk...),
						sortKey:  sortKey,
						ruleName: ruleName,
					})
				} else {
					info.keepStmts = append(info.keepStmts, stmt)
				}
			}
			fe.initFuncs = append(fe.initFuncs, info)
		}
		files2 = append(files2, fe)
	}

	if len(all) == 0 {
		return fmt.Errorf("no registration statements extracted — did the input files change?")
	}

	// Sort by stable key.
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].sortKey < all[j].sortKey
	})

	// Detect whether any collected statement references packages that
	// need imports beyond v2.
	needsRegexp := false
	needsStrings := false
	needsFmt := false
	needsStrconv := false
	needsBytes := false
	needsFilepath := false
	needsUnicode := false
	needsScanner := false
	needsTypeinfer := false
	needsOracle := false
	needsExperiment := false
	for _, e := range all {
		s := string(e.stmt)
		if strings.Contains(s, "regexp.") {
			needsRegexp = true
		}
		if strings.Contains(s, "strings.") {
			needsStrings = true
		}
		if strings.Contains(s, "fmt.") {
			needsFmt = true
		}
		if strings.Contains(s, "strconv.") {
			needsStrconv = true
		}
		if strings.Contains(s, "bytes.") {
			needsBytes = true
		}
		if strings.Contains(s, "filepath.") {
			needsFilepath = true
		}
		if strings.Contains(s, "unicode.") {
			needsUnicode = true
		}
		if strings.Contains(s, "scanner.") {
			needsScanner = true
		}
		if strings.Contains(s, "typeinfer.") {
			needsTypeinfer = true
		}
		if strings.Contains(s, "oracle.") {
			needsOracle = true
		}
		if strings.Contains(s, "experiment.") {
			needsExperiment = true
		}
	}

	// Render zz_registry_gen.go.
	var b bytes.Buffer
	b.WriteString("// Code generated by krit-registry-extract. DO NOT EDIT.\n")
	b.WriteString("// generator: internal/codegen/cmd/krit-registry-extract\n")
	b.WriteString("//\n")
	b.WriteString("// This file consolidates every rule-registration call that used to\n")
	b.WriteString("// live in scattered init() bodies across internal/rules/*.go. The\n")
	b.WriteString("// blocks are copied verbatim from the original sources so embedded\n")
	b.WriteString("// struct literals, adapter options, and comments are preserved.\n")
	b.WriteString("//\n")
	b.WriteString("// Ordering: by (source filename, byte offset) — stable.\n\n")
	fmt.Fprintf(&b, "package %s\n\n", *pkgName)
	b.WriteString("import (\n")
	if needsBytes {
		b.WriteString("\t\"bytes\"\n")
	}
	if needsFmt {
		b.WriteString("\t\"fmt\"\n")
	}
	if needsFilepath {
		b.WriteString("\t\"path/filepath\"\n")
	}
	if needsRegexp {
		b.WriteString("\t\"regexp\"\n")
	}
	if needsStrconv {
		b.WriteString("\t\"strconv\"\n")
	}
	if needsStrings {
		b.WriteString("\t\"strings\"\n")
	}
	if needsUnicode {
		b.WriteString("\t\"unicode\"\n")
	}
	if needsBytes || needsFmt || needsFilepath || needsRegexp || needsStrconv || needsStrings || needsUnicode {
		b.WriteString("\n")
	}
	if needsExperiment {
		b.WriteString("\t\"github.com/kaeawc/krit/internal/experiment\"\n")
	}
	if needsOracle {
		b.WriteString("\t\"github.com/kaeawc/krit/internal/oracle\"\n")
	}
	b.WriteString("\tv2 \"github.com/kaeawc/krit/internal/rules/v2\"\n")
	if needsScanner {
		b.WriteString("\t\"github.com/kaeawc/krit/internal/scanner\"\n")
	}
	if needsTypeinfer {
		b.WriteString("\t\"github.com/kaeawc/krit/internal/typeinfer\"\n")
	}
	b.WriteString(")\n\n")
	b.WriteString("// _ pacifies goimports if v2 is only referenced inside the init body.\n")
	b.WriteString("var _ = v2.Register\n\n")
	if needsRegexp {
		b.WriteString("var _ = regexp.MustCompile\n\n")
	}
	if needsStrings {
		b.WriteString("var _ = strings.Contains\n\n")
	}
	b.WriteString("func init() {\n")
	b.WriteString("\tregisterAllRules()\n")
	b.WriteString("}\n\n")
	b.WriteString("func registerAllRules() {\n")
	var prevFile string
	for _, e := range all {
		if e.file != prevFile {
			fmt.Fprintf(&b, "\n\t// --- from %s ---\n", e.file)
			prevFile = e.file
		}
		// Indent the statement one level.
		indented := indentBlock(string(e.stmt), "\t")
		b.WriteString(indented)
		if !strings.HasSuffix(indented, "\n") {
			b.WriteString("\n")
		}
	}
	b.WriteString("}\n")

	formatted, err := format.Source(b.Bytes())
	if err != nil {
		// Emit unformatted bytes to aid debugging but fail.
		return fmt.Errorf("gofmt generated registry: %w\n---\n%s", err, b.String())
	}

	if *verify {
		existing, readErr := os.ReadFile(*outFile)
		if readErr != nil {
			return fmt.Errorf("verify: read %s: %w", *outFile, readErr)
		}
		if !bytes.Equal(existing, formatted) {
			return fmt.Errorf("verify: %s is out of date (re-run krit-registry-extract)", *outFile)
		}
	} else {
		if err := os.WriteFile(*outFile, formatted, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", *outFile, err)
		}
		fmt.Fprintf(stdout, "krit-registry-extract: wrote %s (%d registrations from %d files)\n",
			*outFile, len(all), len(files))
	}

	if *rewrite {
		for _, fe := range files2 {
			if len(fe.initFuncs) == 0 {
				continue
			}
			anyChange := false
			for _, inf := range fe.initFuncs {
				if len(inf.regStmts) > 0 {
					anyChange = true
					break
				}
			}
			if !anyChange {
				continue
			}
			newSrc, err := rewriteFile(fe.original, fe.fset, fe.fileAST, fe.initFuncs)
			if err != nil {
				return fmt.Errorf("rewrite %s: %w", fe.path, err)
			}
			if err := os.WriteFile(fe.path, newSrc, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", fe.path, err)
			}
		}
	}

	return nil
}

type initFuncInfo struct {
	funcDecl  *ast.FuncDecl
	regStmts  []ast.Stmt // statements to remove (contain v2.Register(...))
	keepStmts []ast.Stmt // statements to keep (e.g. map inits)
}

// stmtContainsRegister reports whether stmt (recursively) contains a
// call expression of the form v2.Register(...) — the only registration
// shape actually used in rule init() blocks after the pre-3E audit.
func stmtContainsRegister(stmt ast.Stmt) bool {
	found := false
	ast.Inspect(stmt, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if ident.Name == "v2" && sel.Sel != nil && sel.Sel.Name == "Register" {
			found = true
			return false
		}
		return true
	})
	return found
}

// findBannerBefore scans backwards from offset for the most recent
// "// --- from X.go ---" comment and returns the extracted filename.
// If none is found, fallback is returned.
func findBannerBefore(src []byte, offset int, fallback string) string {
	// Walk back line by line.
	s := src[:offset]
	// Find last "// --- from " occurrence.
	marker := []byte("// --- from ")
	idx := bytes.LastIndex(s, marker)
	if idx < 0 {
		return fallback
	}
	// Read until " ---" or end of line.
	tail := src[idx+len(marker):]
	end := bytes.Index(tail, []byte(" ---"))
	if end < 0 {
		nl := bytes.IndexByte(tail, '\n')
		if nl < 0 {
			return fallback
		}
		end = nl
	}
	return string(tail[:end])
}

// extractRuleName pulls the first RuleName: "X" literal out of the
// registration source for a debug-only sort key. Never used for correctness.
func extractRuleName(src string) string {
	const marker = `RuleName: "`
	idx := strings.Index(src, marker)
	if idx < 0 {
		idx = strings.Index(src, `RuleName:"`)
		if idx < 0 {
			return ""
		}
		idx += len(`RuleName:"`)
	} else {
		idx += len(marker)
	}
	end := strings.IndexByte(src[idx:], '"')
	if end < 0 {
		return ""
	}
	return src[idx : idx+end]
}

// indentBlock prefixes every line of src with prefix. Existing
// indentation inside the block is preserved so nested braces line up
// with their opening.
func indentBlock(src, prefix string) string {
	lines := strings.Split(src, "\n")
	var b strings.Builder
	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			// Preserve trailing newline without adding a prefix-only line.
			b.WriteString("\n")
			continue
		}
		b.WriteString(prefix)
		b.WriteString(line)
		if i < len(lines)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// rewriteFile removes the registration statements from each init() in
// the file. If an init()'s keepStmts is empty after removal, the whole
// funcDecl is dropped.
//
// Strategy: operate on the source bytes directly, using position info
// from the AST. For each init() FuncDecl:
//   - If it has no keepStmts remaining, delete the whole FuncDecl
//     (including its leading doc comment, if any).
//   - Otherwise, replace only the body's List with keepStmts.
func rewriteFile(src []byte, fset *token.FileSet, _ *ast.File, initFuncs []*initFuncInfo) ([]byte, error) {
	// Build a list of byte-range edits (delete or replace).
	type edit struct {
		start, end int
		replace    []byte
	}
	var edits []edit

	for _, inf := range initFuncs {
		fd := inf.funcDecl
		if len(inf.regStmts) == 0 {
			continue // nothing to remove
		}
		funcStart := fset.Position(fd.Pos()).Offset
		funcEnd := fset.Position(fd.End()).Offset

		// Expand funcStart backwards to include any leading doc comment
		// attached to the FuncDecl (fd.Doc).
		if fd.Doc != nil && len(fd.Doc.List) > 0 {
			docStart := fset.Position(fd.Doc.Pos()).Offset
			if docStart < funcStart {
				funcStart = docStart
			}
		}

		if len(inf.keepStmts) == 0 {
			// Delete entire func decl — include the trailing newline if
			// present and a leading blank line before it (to avoid
			// doubled blanks).
			delStart := funcStart
			delEnd := funcEnd
			// Consume one trailing newline.
			if delEnd < len(src) && src[delEnd] == '\n' {
				delEnd++
			}
			// If preceded by \n\n, swallow one of the preceding newlines
			// so we don't leave a triple blank.
			if delStart >= 2 && src[delStart-1] == '\n' && src[delStart-2] == '\n' {
				delStart--
			}
			edits = append(edits, edit{start: delStart, end: delEnd, replace: nil})
			continue
		}

		// Partial: replace the body content with only the keep statements.
		// Find body range (the opening brace's offset and its closing).
		bodyOpen := fset.Position(fd.Body.Lbrace).Offset // points at '{'
		bodyClose := fset.Position(fd.Body.Rbrace).Offset // points at '}'

		// Reconstruct the body from kept statements' original bytes.
		var newBody bytes.Buffer
		newBody.WriteString("{\n")
		for _, st := range inf.keepStmts {
			sOff := fset.Position(st.Pos()).Offset
			eOff := fset.Position(st.End()).Offset
			// Take the original bytes AS IS, but re-indent: strip any
			// leading-tab indentation beyond one tab so we preserve
			// internal structure. Actually, simpler: if the original
			// bytes begin with the same column as the original body
			// indentation, reuse them verbatim with a single-tab prefix.
			chunk := src[sOff:eOff]
			newBody.WriteString("\t")
			newBody.Write(chunk)
			newBody.WriteByte('\n')
		}
		newBody.WriteString("}")

		edits = append(edits, edit{
			start:   bodyOpen,
			end:     bodyClose + 1,
			replace: newBody.Bytes(),
		})
	}

	// Apply edits in reverse order to keep offsets valid.
	sort.SliceStable(edits, func(i, j int) bool {
		return edits[i].start > edits[j].start
	})
	out := append([]byte(nil), src...)
	for _, e := range edits {
		var buf bytes.Buffer
		buf.Write(out[:e.start])
		buf.Write(e.replace)
		buf.Write(out[e.end:])
		out = buf.Bytes()
	}

	// Remove v2 imports that are no longer referenced after statement
	// removal. Re-parse; if the file no longer has any v2.* selector
	// outside the import spec, drop the import.
	out = maybeRemoveUnusedImports(out)

	// gofmt to normalize.
	formatted, err := format.Source(out)
	if err != nil {
		// Fall back to unformatted bytes — rare but surfaces real issues.
		return nil, fmt.Errorf("gofmt rewritten file: %w", err)
	}
	return formatted, nil
}

// maybeRemoveUnusedImports drops imports that are no longer referenced
// in the file's non-import declarations. We re-parse the rewritten
// source and check each import's local name (default or explicit)
// against a selector-ident count. If count==0, the import is removed.
func maybeRemoveUnusedImports(src []byte) []byte {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "rewrite.go", src, parser.ParseComments)
	if err != nil {
		return src // leave as-is; gofmt will surface the error
	}

	// Collect selector-idents used as X.Y (including inside type decls).
	used := map[string]int{}
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		used[id.Name]++
		return true
	})

	// Also count standalone ident refs (e.g. `v2.Register` as bare
	// identifier in `var _ = v2.Register` is a selector, so it's caught
	// above).

	// Walk imports and determine local names.
	type importLoc struct {
		start, end int
		localName  string
	}
	var toRemove []importLoc
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		for _, spec := range gd.Specs {
			is := spec.(*ast.ImportSpec)
			local := ""
			if is.Name != nil {
				local = is.Name.Name
			} else {
				// default name = last path segment, stripped of quotes
				path := strings.Trim(is.Path.Value, `"`)
				slash := strings.LastIndex(path, "/")
				local = path[slash+1:]
			}
			if local == "_" || local == "." {
				continue
			}
			if used[local] > 0 {
				continue
			}
			// Only remove imports we introduced concerns about — the
			// registration migration. Be conservative: only strip
			// package paths known to be rule-registration related.
			path := strings.Trim(is.Path.Value, `"`)
			if path != "github.com/kaeawc/krit/internal/rules/v2" {
				continue
			}
			toRemove = append(toRemove, importLoc{
				start:     fset.Position(is.Pos()).Offset,
				end:       fset.Position(is.End()).Offset,
				localName: local,
			})
		}
	}

	if len(toRemove) == 0 {
		return src
	}
	sort.SliceStable(toRemove, func(i, j int) bool {
		return toRemove[i].start > toRemove[j].start
	})
	out := append([]byte(nil), src...)
	for _, r := range toRemove {
		// Expand end to include trailing newline + any leading whitespace
		// on the same line.
		s, e := r.start, r.end
		// Consume trailing newline.
		if e < len(out) && out[e] == '\n' {
			e++
		}
		// Consume leading tabs/spaces on the same line.
		for s > 0 && (out[s-1] == '\t' || out[s-1] == ' ') {
			s--
		}
		var buf bytes.Buffer
		buf.Write(out[:s])
		buf.Write(out[e:])
		out = buf.Bytes()
	}
	return out
}
