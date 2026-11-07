package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// computeBlockCommentLines returns indices (0-based) that are inside a
// `/* ... */` or `/** ... */` block comment (including opening/closing lines).
func computeBlockCommentLines(lines []string) map[int]bool {
	result := make(map[int]bool)
	inBlock := false
	for i, line := range lines {
		if inBlock {
			result[i] = true
			if strings.Contains(line, "*/") {
				inBlock = false
			}
			continue
		}
		// Check for block comment start
		if idx := strings.Index(line, "/*"); idx >= 0 {
			// Check if it closes on the same line
			if closeIdx := strings.Index(line[idx+2:], "*/"); closeIdx < 0 {
				inBlock = true
				result[i] = true
			}
		}
	}
	return result
}

// computeRawStringLines returns a map of line indices (0-based) that are
// inside a triple-quoted raw string literal. Does not mark the opening and
// closing lines — only the middle lines where the overflow happens.
func computeRawStringLines(lines []string) map[int]bool {
	result := make(map[int]bool)
	inRaw := false
	for i, line := range lines {
		// Count occurrences of """ on this line
		count := strings.Count(line, `"""`)
		if inRaw {
			result[i] = true
		}
		if count%2 == 1 {
			inRaw = !inRaw
		}
	}
	return result
}

// NewLineAtEndOfFileRule detects files not ending with a newline.
type NewLineAtEndOfFileRule struct {
	LineBase
	BaseRule
}

// Confidence bumps this line rule from the 0.75 line-rule default to
// 0.95 — presence/absence of a trailing newline is a single byte check
// with no room for interpretation.
func (r *NewLineAtEndOfFileRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func (r *NewLineAtEndOfFileRule) check(ctx *api.Context) {
	file := ctx.File
	if len(file.Content) > 0 && file.Content[len(file.Content)-1] != '\n' {
		f := r.Finding(file, len(file.Lines), len(file.Lines[len(file.Lines)-1])+1, "File does not end with a newline.")
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   len(file.Content),
			EndByte:     len(file.Content),
			Replacement: "\n",
		}
		ctx.Emit(f)
	}
}

// SpacingAfterPackageAndImportsRule checks for blank line after package/imports.
type SpacingAfterPackageAndImportsRule struct {
	FlatDispatchBase
	BaseRule
}

// rowCol converts a byte offset to a 1-based row and column.
func spacingRowCol(file *scanner.File, offset int) (int, int) {
	row := file.RowForByte(offset)
	return row + 1, offset - file.LineOffset(row) + 1
}

// exactlyOneBlankLine reports whether gap is a line break, one empty line and a line break, with only blanks around.
func exactlyOneBlankLine(gap string) bool {
	return strings.Count(gap, "\n") == 2 && strings.TrimSpace(gap) == ""
}

// checkKotlin follows detekt's SpacingBetweenPackageAndImports: with imports present, one empty line must separate the
// package declaration (or a comment before the imports) from them and the imports from a following class or object.
func (r *SpacingAfterPackageAndImportsRule) checkKotlin(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	if file.FlatType(idx) != "import_header" {
		return
	}
	// Run once, from the last import.
	if next, ok := nextPreludeSibling(file, idx); ok && (file.FlatType(next) == "import_header" || file.FlatType(next) == "import_list") {
		return
	}
	list, ok := file.FlatParent(idx)
	if !ok || file.FlatType(list) != "import_list" {
		return
	}
	first := file.FlatFirstChild(list)
	for first != 0 && file.FlatType(first) != "import_header" {
		first = file.FlatNextSib(first)
	}
	if first == 0 {
		return
	}
	content := string(file.Content)
	fix := func(start, end int, rep string) *scanner.Fix {
		return &scanner.Fix{ByteMode: true, StartByte: start, EndByte: end, Replacement: rep}
	}

	// detekt only checks files that declare a package.
	prev, hasPrev := file.FlatPrevSibling(list)
	if !hasPrev {
		return
	}
	hasPackage := false
	for n, ok := prev, true; ok; n, ok = file.FlatPrevSibling(n) {
		if file.FlatType(n) == "package_header" {
			hasPackage = true
			break
		}
	}
	if !hasPackage {
		return
	}

	// Package (or comment) to the first import.
	if prev, ok := file.FlatPrevSibling(list); ok {
		if t := file.FlatType(prev); t == "package_header" || t == "line_comment" || t == "multiline_comment" {
			prevEnd, firstStart := int(preludeCodeEndByte(file, prev)), int(file.FlatStartByte(first))
			if prevEnd < len(content) && content[prevEnd] == ';' {
				prevEnd++
			}
			// A comment between the two is left alone.
			if prevEnd <= firstStart && strings.TrimSpace(content[prevEnd:firstStart]) == "" && !exactlyOneBlankLine(content[prevEnd:firstStart]) {
				row, col := spacingRowCol(file, prevEnd)
				f := r.Finding(file, row, col, "There should be exactly one empty line in between the package declaration and the list of imports.")
				f.Fix = fix(prevEnd, firstStart, "\n\n")
				ctx.Emit(f)
			}
		}
	}

	// The last import to a following class or object.
	var decl uint32
	for n, ok := file.FlatNextSibling(list); ok; n, ok = file.FlatNextSibling(n) {
		if isFlatCommentNode(file, n) {
			continue
		}
		decl = n
		break
	}
	if decl == 0 || (file.FlatType(decl) != "class_declaration" && file.FlatType(decl) != "object_declaration") {
		return
	}
	importEnd := int(preludeCodeEndByte(file, idx))
	// A trailing `;` belongs to the import.
	if importEnd < len(content) && content[importEnd] == ';' {
		importEnd++
	}
	rest := importEnd
	for rest < len(content) && strings.ContainsRune(" \t\r\n", rune(content[rest])) {
		rest++
	}
	if exactlyOneBlankLine(content[importEnd:rest]) {
		return
	}
	name := extractIdentifierFlat(file, decl)
	if name == "" {
		name = "the class or object"
	}
	row, col := spacingRowCol(file, importEnd)
	f := r.Finding(file, row, col, "There should be exactly one empty line in between the list of imports and the declaration of "+name+".")
	f.Fix = fix(importEnd, rest, "\n\n")
	ctx.Emit(f)
}

func (r *SpacingAfterPackageAndImportsRule) check(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	if file.Language != scanner.LangJava {
		r.checkKotlin(ctx)
		return
	}

	// Kotlin's import_list span includes trailing blank lines, so fire on
	// the trailing import_header / package_header and climb out of the list
	// to see its successor — each file emits at most once.
	next, hasNext := nextPreludeSibling(file, idx)
	if hasNext {
		switch file.FlatType(next) {
		case "package_header", "import_list", "import_header",
			"package_declaration", "import_declaration":
			return
		}
	}

	endRow := file.RowForByte(int(preludeCodeEndByte(file, idx)))
	if endRow+1 >= len(file.Lines) {
		return
	}
	if strings.TrimSpace(file.Lines[endRow+1]) == "" {
		return
	}

	lineStart := file.LineOffset(endRow + 1)
	f := r.Finding(file, endRow+2, 1,
		"Missing blank line after package/import declarations.")
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   lineStart,
		EndByte:     lineStart,
		Replacement: "\n",
	}
	ctx.Emit(f)
}

// nextPreludeSibling returns the next file-scope sibling of a package/import
// node. When idx is the last import_header inside a Kotlin import_list, it
// climbs out and returns the list's next sibling instead — the list itself
// is never dispatched.
func nextPreludeSibling(file *scanner.File, idx uint32) (uint32, bool) {
	if next, ok := file.FlatNextSibling(idx); ok {
		return next, true
	}
	parent, ok := file.FlatParent(idx)
	if !ok || file.FlatType(parent) != "import_list" {
		return 0, false
	}
	return file.FlatNextSibling(parent)
}

// preludeCodeEndByte returns the end byte of the code portion of a
// package/import declaration, ignoring trailing comment children. Kotlin
// tree-sitter attaches trailing block/line comments to the preceding
// import_header, so its raw EndByte sweeps past the real declaration.
func preludeCodeEndByte(file *scanner.File, idx uint32) uint32 {
	var lastCode uint32
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "line_comment", "block_comment", "multiline_comment", "comment":
			continue
		}
		lastCode = c
	}
	if lastCode != 0 {
		return file.FlatEndByte(lastCode)
	}
	return file.FlatEndByte(idx)
}

// MaxChainedCallsOnSameLineRule limits chained method calls on a single line.
type MaxChainedCallsOnSameLineRule struct {
	FlatDispatchBase
	BaseRule
	MaxCalls int
}

// Confidence is 0.95 — chain depth is computed structurally from the
// flat AST, so string-literal, decimal, import, and trailing-comment
// dots no longer count.
func (r *MaxChainedCallsOnSameLineRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func (r *MaxChainedCallsOnSameLineRule) check(ctx *api.Context) {
	file := ctx.File
	if scanner.IsTestFile(file.Path) || isGradleBuildScript(file.Path) {
		return
	}
	idx := ctx.Idx
	nodeType := file.FlatType(idx)
	// Only the chain head fires — inner chain nodes are skipped so a
	// chain is reported once.
	if parent, ok := file.FlatParent(idx); ok {
		switch nodeType {
		case "call_expression", "navigation_expression":
			pt := file.FlatType(parent)
			if pt == "call_expression" || pt == "navigation_expression" {
				return
			}
		case "method_invocation", "field_access":
			pt := file.FlatType(parent)
			if pt == "method_invocation" || pt == "field_access" {
				return
			}
		}
	}
	if nodeType == "call_expression" || nodeType == "navigation_expression" {
		for _, count := range kotlinChainRuns(file, idx) {
			if count > r.MaxCalls {
				ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, maxChainedCallsMessage(count, r.MaxCalls)))
			}
		}
		return
	}
	count, minRow, maxRow := chainStepCount(file, idx)
	if count <= r.MaxCalls || minRow != maxRow {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, maxChainedCallsMessage(count, r.MaxCalls)))
}

func maxChainedCallsMessage(count, max int) string {
	return fmt.Sprintf("%d chained calls on a single line; more than %d calls should be wrapped to a new line.", count, max)
}

// kotlinChainRuns splits a call chain at every selector that starts a new line and returns the call count of each
// run. Like detekt, a run counts its receiver when that sits on the line of the run's first selector.
func kotlinChainRuns(file *scanner.File, head uint32) []int {
	links := cascadeLinks(file, head)
	var runs []int
	run := 0
	for i, l := range links {
		if l.wrapped && run > 0 {
			runs = append(runs, run)
			run = 0
		}
		if i == 0 && !l.wrapped {
			run++ // the receiver
		}
		run++
	}
	if run > 0 {
		runs = append(runs, run)
	}
	return runs
}

// chainStepCount returns the number of `.foo` (or `?.foo`) steps in the
// chain rooted at head, plus the min/max source row spanned by chain
// steps. For Kotlin it counts navigation_suffix descendants reachable
// through navigation_expression and call_expression skeleton nodes. For
// Java it counts dots implied by the method_invocation / field_access
// shape (1 dot per nested step, plus identifier_count-1 for simple
// receiver chains on a method_invocation).
func chainStepCount(file *scanner.File, head uint32) (count, minRow, maxRow int) {
	if file == nil || head == 0 {
		return 0, 0, 0
	}
	minRow = file.FlatRow(head)
	maxRow = minRow
	switch file.FlatType(head) {
	case "call_expression", "navigation_expression":
		count = walkKotlinChain(file, head, &minRow, &maxRow)
	case "method_invocation", "field_access":
		count = walkJavaChain(file, head, &minRow, &maxRow)
	}
	return count, minRow, maxRow
}

func trackChainRow(minRow, maxRow *int, r int) {
	if r < *minRow {
		*minRow = r
	}
	if r > *maxRow {
		*maxRow = r
	}
}

func walkKotlinChain(file *scanner.File, head uint32, minRow, maxRow *int) int {
	count := 0
	var walk func(uint32)
	walk = func(n uint32) {
		t := file.FlatType(n)
		if t == "navigation_suffix" {
			count++
			trackChainRow(minRow, maxRow, file.FlatRow(n))
			return
		}
		if t != "call_expression" && t != "navigation_expression" {
			return
		}
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			if !file.FlatIsNamed(c) {
				continue
			}
			switch file.FlatType(c) {
			case "navigation_expression", "call_expression", "navigation_suffix":
				walk(c)
			}
		}
	}
	walk(head)
	return count
}

func walkJavaChain(file *scanner.File, head uint32, minRow, maxRow *int) int {
	count := 0
	var walk func(uint32)
	walk = func(n uint32) {
		switch file.FlatType(n) {
		case "method_invocation":
			count += javaInvocationStep(file, n, minRow, maxRow, walk)
		case "field_access":
			count += javaFieldAccessStep(file, n, minRow, maxRow, walk)
		}
	}
	walk(head)
	return count
}

func javaInvocationStep(file *scanner.File, n uint32, minRow, maxRow *int, walk func(uint32)) int {
	ids := 0
	var nestedObj uint32
	lastIdentRow := -1
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "argument_list" {
			break
		}
		if !file.FlatIsNamed(c) {
			continue
		}
		switch file.FlatType(c) {
		case "identifier":
			ids++
			lastIdentRow = file.FlatRow(c)
		case "method_invocation", "field_access":
			if nestedObj == 0 {
				nestedObj = c
			}
		}
	}
	if nestedObj != 0 {
		if lastIdentRow >= 0 {
			trackChainRow(minRow, maxRow, lastIdentRow)
		}
		walk(nestedObj)
		return ids
	}
	if ids > 1 {
		if lastIdentRow >= 0 {
			trackChainRow(minRow, maxRow, lastIdentRow)
		}
		return ids - 1
	}
	return 0
}

func javaFieldAccessStep(file *scanner.File, n uint32, minRow, maxRow *int, walk func(uint32)) int {
	var nestedObj uint32
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		if !file.FlatIsNamed(c) {
			continue
		}
		t := file.FlatType(c)
		if t == "identifier" {
			trackChainRow(minRow, maxRow, file.FlatRow(c))
			continue
		}
		if t == "method_invocation" || t == "field_access" {
			if nestedObj == 0 {
				nestedObj = c
			}
		}
	}
	if nestedObj != 0 {
		walk(nestedObj)
	}
	return 1
}

// CascadingCallWrappingRule checks that chained calls are wrapped consistently.
type CascadingCallWrappingRule struct {
	LineBase
	BaseRule
	IncludeElvis bool
}

// Confidence reports a tier-2 (medium) base confidence. Style/formatting rule. Detection is pattern or regex based on line text;
// deterministic byte checks have been promoted to tier-1 separately.
// Classified per roadmap/17.
func (r *CascadingCallWrappingRule) Confidence() float64 { return api.ConfidenceMedium }

// cascadeLink is one `.`/`?.` selector of a call chain.
type cascadeLink struct {
	op      uint32
	wrapped bool
}

func cascadeEndRow(file *scanner.File, n uint32) int {
	return file.FlatRow(n) + strings.Count(file.FlatNodeText(n), "\n")
}

// cascadeContinuesChain reports whether node is the receiver of its parent chain element.
func cascadeContinuesChain(file *scanner.File, node uint32) bool {
	p, ok := file.FlatParent(node)
	if !ok {
		return false
	}
	switch file.FlatType(p) {
	case "navigation_expression", "call_expression", "postfix_expression":
		return file.FlatFirstChild(p) == node
	}
	return false
}

// cascadeLinks returns the selectors of the chain rooted at root in source order.
func cascadeLinks(file *scanner.File, root uint32) []cascadeLink {
	var links []cascadeLink
	for cur := root; cur != 0; {
		switch file.FlatType(cur) {
		case "call_expression", "postfix_expression":
			cur = file.FlatFirstChild(cur)
		case "navigation_expression":
			recv := file.FlatFirstChild(cur)
			suffix, ok := file.FlatFindChild(cur, "navigation_suffix")
			if recv == 0 || !ok || suffix == 0 {
				return nil
			}
			op := file.FlatFirstChild(suffix)
			if op == 0 {
				return nil
			}
			links = append(links, cascadeLink{op: op, wrapped: file.FlatRow(op) > cascadeEndRow(file, recv)})
			cur = recv
		default:
			cur = 0
		}
	}
	for i, j := 0, len(links)-1; i < j; i, j = i+1, j-1 {
		links[i], links[j] = links[j], links[i]
	}
	return links
}

func (r *CascadingCallWrappingRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	switch file.FlatType(idx) {
	case "elvis_expression":
		if !r.IncludeElvis || file.FlatChildCount(idx) < 3 {
			return
		}
		left, op := file.FlatChild(idx, 0), file.FlatChild(idx, 1)
		wrapped := false
		for _, l := range cascadeLinks(file, left) {
			wrapped = wrapped || l.wrapped
		}
		if wrapped && file.FlatRow(op) == cascadeEndRow(file, left) {
			ctx.Emit(r.Finding(file, file.FlatRow(op)+1, file.FlatCol(op)+1,
				"Elvis operator should be wrapped to a new line because a preceding call in the chain is wrapped."))
		}
	default:
		if cascadeContinuesChain(file, idx) {
			return
		}
		sawWrapped := false
		for _, l := range cascadeLinks(file, idx) {
			if l.wrapped {
				sawWrapped = true
			} else if sawWrapped {
				ctx.Emit(r.Finding(file, file.FlatRow(l.op)+1, file.FlatCol(l.op)+1,
					"Chained call should be wrapped to a new line because a preceding call in the chain is wrapped."))
			}
		}
	}
}

// UnderscoresInNumericLiteralsRule detects large numbers without underscores.
type UnderscoresInNumericLiteralsRule struct {
	FlatDispatchBase
	BaseRule
	AcceptableLength         int // maximum consecutive digits allowed without underscores
	AllowNonStandardGrouping bool
}

// Confidence reports a tier-2 (medium) base confidence. Style/formatting rule. Detection is pattern or regex based on line text;
// deterministic byte checks have been promoted to tier-1 separately.
// Classified per roadmap/17.
func (r *UnderscoresInNumericLiteralsRule) Confidence() float64 { return api.ConfidenceMedium }

// formatWithUnderscores inserts underscores every 3 digits from the right for readability.
func formatWithUnderscores(digits string) string {
	n := len(digits)
	if n <= 3 {
		return digits
	}
	var result []byte
	for i, c := range digits {
		if i > 0 && (n-i)%3 == 0 {
			result = append(result, '_')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

func maxConsecutiveDigits(text string) int {
	maxRun := 0
	current := 0
	for _, c := range text {
		if c >= '0' && c <= '9' {
			current++
			if current > maxRun {
				maxRun = current
			}
			continue
		}
		current = 0
	}
	return maxRun
}

func stripNumericLiteralUnderscores(text string) string {
	if !strings.Contains(text, "_") {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	for _, c := range text {
		if c != '_' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func hasStandardNumericGrouping(text string) bool {
	groups := strings.Split(text, "_")
	if len(groups) == 1 {
		return true
	}
	if len(groups[0]) == 0 || len(groups[0]) > 3 {
		return false
	}
	for _, group := range groups[1:] {
		if len(group) != 3 {
			return false
		}
	}
	return true
}

// EqualsOnSignatureLineRule detects `=` on the next line of a function signature.
type EqualsOnSignatureLineRule struct {
	LineBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/formatting rule. Detection is pattern or regex based on line text;
// deterministic byte checks have been promoted to tier-1 separately.
// Classified per roadmap/17.
func (r *EqualsOnSignatureLineRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *EqualsOnSignatureLineRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	body, ok := file.FlatFindChild(idx, "function_body")
	if !ok || body == 0 {
		return
	}
	eq := file.FlatFirstChild(body)
	if eq == 0 || !file.FlatNodeTextEquals(eq, "=") {
		return
	}
	prev := uint32(0)
	for c := file.FlatFirstChild(idx); c != 0 && c != body; c = file.FlatNextSib(c) {
		prev = c
	}
	// A comment before the `=` would swallow it once joined onto the signature line.
	if prev == 0 || isFlatCommentNode(file, prev) {
		return
	}
	gapStart, gapEnd := int(file.FlatEndByte(prev)), int(file.FlatStartByte(eq))
	if gapStart > gapEnd || gapEnd > len(file.Content) {
		return
	}
	gap := string(file.Content[gapStart:gapEnd])
	// Only a pure whitespace gap with a line break is safe to join; comments stay put.
	if !strings.Contains(gap, "\n") || strings.TrimSpace(gap) != "" {
		return
	}
	f := r.Finding(file, file.FlatRow(eq)+1, file.FlatCol(eq)+1,
		"'=' should be on the same line as the function signature.")
	f.Fix = &scanner.Fix{ByteMode: true, StartByte: gapStart, EndByte: gapEnd, Replacement: " "}
	ctx.Emit(f)
}
