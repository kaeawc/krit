package rules

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of Android Lint 32.4.1's TrimDetector (TrimLambda), TextConcatDetector (TextConcatSpace) and
// WrongCommentTypeDetector (WrongCommentType).

// TrimLambdaRule mirrors Lint's TrimLambda: `trim { it <= ' ' }` or `trim { it.isWhitespace() }` on a string, the
// shape the Java-to-Kotlin converter produces for Java's trim().
type TrimLambdaRule struct {
	AndroidRule
}

// TextConcatSpaceRule mirrors Lint's TextConcatSpace: string literals concatenated across lines where the separating
// space looks missing.
type TextConcatSpaceRule struct {
	AndroidRule
}

// WrongCommentTypeRule mirrors Lint's WrongCommentType: a block comment on a declaration that holds KDoc or Javadoc
// tags.
type WrongCommentTypeRule struct {
	AndroidRule
}

// Confidence reports a high base confidence: the receiver must be proven a string and the lambda matched exactly.
func (r *TrimLambdaRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a medium base confidence: like Lint, the check is a wording heuristic.
func (r *TextConcatSpaceRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a high base confidence: the comment text is checked as Lint parses it.
func (r *WrongCommentTypeRule) Confidence() float64 { return api.ConfidenceHigh }

// ---------------------------------------------------------------------------
// TrimLambda
// ---------------------------------------------------------------------------

func (r *TrimLambdaRule) check(ctx *api.Context) {
	file := ctx.File
	call := ctx.Idx
	// The callee of a call written `s.trim() { ... }` is itself a call node; it is checked from the outer call.
	if parent, ok := file.FlatParent(call); ok && file.FlatType(parent) == "call_expression" && file.FlatFirstChild(parent) == call {
		return
	}
	// Cheap callee text check first: almost every call is something else.
	callee := file.FlatFirstChild(call)
	if file.FlatType(callee) == "call_expression" {
		callee = file.FlatFirstChild(callee)
	}
	if !strings.HasSuffix(file.FlatNodeText(callee), "trim") {
		return
	}
	nav, lambda, ok := trimLambdaCall(file, call)
	if !ok || !trimIsDefaultLambda(file, lambda) || trimShadowed(file) {
		return
	}
	receiver := file.FlatFirstChild(nav)
	if !trimReceiverIsString(ctx, receiver) {
		return
	}
	text := file.FlatNodeText(lambda)
	f := lintFindingAtNode(ctx, lambda, "", "The lambda argument (`"+text+"`) is unnecessary")
	f.Fix = trimLambdaFix(file, nav, lambda)
	ctx.Emit(f)
}

// trimLambdaCall matches `recv.trim { }`, `recv.trim() { }` and `recv.trim({ })` with no other argument, returning
// the `recv.trim` navigation and the lambda literal.
func trimLambdaCall(file *scanner.File, call uint32) (nav, lambda uint32, ok bool) {
	callee := file.FlatFirstChild(call)
	suffix := textLastChild(file, call)
	if callee == 0 || suffix == 0 || file.FlatType(suffix) != "call_suffix" {
		return 0, 0, false
	}
	args := 0
	for child := file.FlatFirstChild(suffix); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "annotated_lambda":
			lambda, _ = file.FlatFindChild(child, "lambda_literal")
			args++
		case "value_arguments":
			for arg := file.FlatFirstChild(child); arg != 0; arg = file.FlatNextSib(arg) {
				if file.FlatType(arg) != "value_argument" {
					continue
				}
				args++
				if expr := flatValueArgumentExpression(file, arg); expr != 0 && file.FlatType(expr) == "lambda_literal" &&
					flatValueArgumentLabel(file, arg) == "" {
					lambda = expr
				}
			}
		case "type_arguments":
			return 0, 0, false
		}
	}
	// `s.trim() { }`: the callee is the empty-argument call `s.trim()`.
	if file.FlatType(callee) == "call_expression" {
		inner := textLastChild(file, callee)
		if inner == 0 || file.FlatType(inner) != "call_suffix" || strings.TrimSpace(file.FlatNodeText(inner)) != "()" {
			return 0, 0, false
		}
		callee = file.FlatFirstChild(callee)
	}
	if args != 1 || lambda == 0 || file.FlatType(callee) != "navigation_expression" {
		return 0, 0, false
	}
	last := textLastChild(file, callee)
	if last == 0 || file.FlatType(last) != "navigation_suffix" || file.FlatChildTextOrEmpty(last, "simple_identifier") != "trim" {
		return 0, 0, false
	}
	return callee, lambda, true
}

// trimIsDefaultLambda reports whether the lambda is `{ p <= ' ' }` or `{ p.isWhitespace() }` on its implicit or only
// parameter, with parentheses allowed around the statement and the receiver.
func trimIsDefaultLambda(file *scanner.File, lambda uint32) bool {
	param := "it"
	if params, ok := file.FlatFindChild(lambda, "lambda_parameters"); ok {
		var names []string
		for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "variable_declaration" {
				names = append(names, file.FlatChildTextOrEmpty(child, "simple_identifier"))
			}
		}
		if len(names) != 1 || names[0] == "" {
			return false
		}
		param = names[0]
	}
	stmts, ok := file.FlatFindChild(lambda, "statements")
	if !ok || file.FlatNamedChildCount(stmts) != 1 {
		return false
	}
	stmt := trimSkipParens(file, file.FlatNamedChild(stmts, 0))
	switch file.FlatType(stmt) {
	case "comparison_expression":
		if file.FlatNamedChildCount(stmt) != 2 || !strings.Contains(file.FlatNodeText(stmt), "<=") {
			return false
		}
		left, right := file.FlatNamedChild(stmt, 0), file.FlatNamedChild(stmt, 1)
		op := strings.TrimSpace(string(file.Content[file.FlatEndByte(left):file.FlatStartByte(right)]))
		return op == "<=" && file.FlatType(left) == "simple_identifier" && file.FlatNodeText(left) == param &&
			file.FlatType(right) == "character_literal" && file.FlatNodeText(right) == "' '"
	case "call_expression":
		callee := file.FlatFirstChild(stmt)
		suffix := textLastChild(file, stmt)
		if file.FlatType(callee) != "navigation_expression" || strings.TrimSpace(file.FlatNodeText(suffix)) != "()" {
			return false
		}
		recv := trimSkipParens(file, file.FlatFirstChild(callee))
		last := textLastChild(file, callee)
		return file.FlatType(recv) == "simple_identifier" && file.FlatNodeText(recv) == param &&
			strings.TrimSpace(file.FlatNodeText(last)) == ".isWhitespace"
	}
	return false
}

// textLastChild returns the last child of idx, or 0.
func textLastChild(file *scanner.File, idx uint32) uint32 {
	if n := file.FlatChildCount(idx); n > 0 {
		return file.FlatChild(idx, n-1)
	}
	return 0
}

func trimSkipParens(file *scanner.File, idx uint32) uint32 {
	for idx != 0 && file.FlatType(idx) == "parenthesized_expression" {
		idx = file.FlatNamedChild(idx, 0)
	}
	return idx
}

// trimShadowed reports whether the file declares its own trim or isWhitespace function, which would take the call
// away from the standard library.
func trimShadowed(file *scanner.File) bool {
	shadowed := false
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		switch file.FlatChildTextOrEmpty(fn, "simple_identifier") {
		case "trim", "isWhitespace":
			shadowed = true
		}
	})
	return shadowed
}

// trimReceiverIsString reports whether the receiver is proven a String or CharSequence (nullable through `?.`).
func trimReceiverIsString(ctx *api.Context, recv uint32) bool {
	file := ctx.File
	recv = trimSkipParens(file, recv)
	if typ, ok := semantics.ExpressionType(ctx, recv); ok && typ.FQN != "" {
		return trimIsStringType(file, typ.FQN)
	}
	switch file.FlatType(recv) {
	case "string_literal":
		return true
	case "simple_identifier":
		return trimIsStringType(file, strings.TrimSuffix(lintDeclaredTypeText(file, recv, file.FlatNodeText(recv)), "?"))
	}
	return false
}

// trimIsStringType reports whether a type name is String or CharSequence, qualified or not; an unqualified name
// declared as a class in the file is that class.
func trimIsStringType(file *scanner.File, typ string) bool {
	switch typ {
	case "kotlin.String", "kotlin.CharSequence", "java.lang.String", "java.lang.CharSequence":
		return true
	case "String", "CharSequence":
		return lintSameFileClass(file, typ) == 0
	}
	return false
}

// trimLambdaFix removes the lambda and the whitespace before it, adding `()` when the call has no parentheses, as
// Lint's quick fix does.
func trimLambdaFix(file *scanner.File, nav, lambda uint32) *scanner.Fix {
	start := int(file.FlatStartByte(lambda))
	for start > int(file.FlatEndByte(nav)) && (file.Content[start-1] == ' ' || file.Content[start-1] == '\t' || file.Content[start-1] == '\n') {
		start--
	}
	end := int(file.FlatEndByte(lambda))
	replacement := "()"
	between := string(file.Content[file.FlatEndByte(nav):file.FlatStartByte(lambda)])
	if strings.Contains(between, "(") {
		replacement = ""
		// `trim({ ... })`: keep the parentheses, drop only the lambda.
		if strings.TrimSpace(between) == "(" {
			start = int(file.FlatStartByte(lambda))
		}
	}
	return &scanner.Fix{ByteMode: true, StartByte: start, EndByte: end, Replacement: replacement}
}

// ---------------------------------------------------------------------------
// TextConcatSpace
// ---------------------------------------------------------------------------

// textConcatPiece is the string part Lint's findString picks: a literal run with its position.
type textConcatPiece struct {
	text      string
	line, col int
	start     int
	endLine   int
}

func (r *TextConcatSpaceRule) check(ctx *api.Context) {
	file := ctx.File
	node := ctx.Idx
	lhs, rhs, ok := textConcatOperands(file, node)
	if !ok {
		return
	}
	left, ok := textConcatFind(file, lhs, false)
	if !ok {
		return
	}
	ls := left.text
	last, _ := utf8.DecodeLastRuneInString(ls)
	if ls == "" || !unicode.IsLetter(last) {
		return
	}
	// The previous string must end with a word after a space, so "The end.\n\n" does not match.
	i := len(ls)
	for i > 0 {
		r, size := utf8.DecodeLastRuneInString(ls[:i])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			break
		}
		i -= size
	}
	if i == 0 || ls[i-1] != ' ' {
		return
	}
	right, ok := textConcatFind(file, rhs, true)
	if !ok {
		return
	}
	rs := right.text
	first, _ := utf8.DecodeRuneInString(rs)
	if rs == "" || !unicode.IsLetter(first) {
		return
	}
	leftWords := strings.Split(ls, " ")
	if len(leftWords) < 2 || right.line <= left.endLine {
		return
	}
	lastWord, firstWord := leftWords[len(leftWords)-1], strings.Split(rs, " ")[0]
	ctx.Emit(scanner.Finding{File: file.Path, Line: right.line, Col: right.col, StartByte: right.start, EndByte: right.start + len(rs),
		Message: fmt.Sprintf("Missing space between \"%s\" on the previous line and \"%s\" here? Resulting string is \"%s%s\".",
			lastWord, firstWord, lastWord, firstWord)})
}

// textConcatOperands returns the operands of a `+` expression, skipping comments between them.
func textConcatOperands(file *scanner.File, node uint32) (lhs, rhs uint32, ok bool) {
	plus := false
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		switch t := file.FlatType(child); {
		case t == "+":
			plus = true
		case t == "line_comment" || t == "multiline_comment" || t == "block_comment":
		case file.FlatIsNamed(child):
			if lhs == 0 {
				lhs = child
			} else if rhs == 0 {
				rhs = child
			} else {
				return 0, 0, false
			}
		default:
			return 0, 0, false
		}
	}
	return lhs, rhs, plus && lhs != 0 && rhs != 0
}

// textConcatFind follows Lint's findString: through parentheses and `+` operands (the right one, or the left one when
// biasLeft) to a string literal, whose last (or first) literal entry it returns. In Kotlin an entry is a run of
// characters between templates and escapes; in Java it is the whole literal without its quotes.
func textConcatFind(file *scanner.File, expr uint32, biasLeft bool) (textConcatPiece, bool) {
	for depth := 0; expr != 0 && depth < 64; depth++ {
		switch file.FlatType(expr) {
		case "parenthesized_expression":
			expr = file.FlatNamedChild(expr, 0)
		case "additive_expression", "binary_expression":
			l, r, ok := textConcatOperands(file, expr)
			if !ok {
				return textConcatPiece{}, false
			}
			if biasLeft {
				expr = l
			} else {
				expr = r
			}
		case "string_literal":
			if file.Language == scanner.LangJava {
				return textConcatJavaLiteral(file, expr)
			}
			return textConcatKotlinEntry(file, expr, biasLeft)
		default:
			return textConcatPiece{}, false
		}
	}
	return textConcatPiece{}, false
}

func textConcatJavaLiteral(file *scanner.File, lit uint32) (textConcatPiece, bool) {
	text := file.FlatNodeText(lit)
	if strings.HasPrefix(text, `"""`) || len(text) < 2 || text[0] != '"' || text[len(text)-1] != '"' {
		return textConcatPiece{}, false
	}
	return textConcatPiece{text: text[1 : len(text)-1], line: file.FlatRow(lit) + 1, col: file.FlatCol(lit) + 1,
		start: int(file.FlatStartByte(lit)), endLine: textConcatEndLine(file, lit)}, true
}

// textConcatKotlinEntry returns the first or last template entry of a Kotlin string when it is literal text.
func textConcatKotlinEntry(file *scanner.File, lit uint32, first bool) (textConcatPiece, bool) {
	var parts []uint32
	for child := file.FlatFirstChild(lit); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) || file.FlatType(child) == "${" {
			parts = append(parts, child)
		}
	}
	if len(parts) == 0 {
		return textConcatPiece{}, false
	}
	part := parts[len(parts)-1]
	if first {
		part = parts[0]
	}
	if file.FlatType(part) != "string_content" {
		return textConcatPiece{}, false
	}
	content := file.FlatNodeText(part)
	start := int(file.FlatStartByte(part))
	runs := kotlinLiteralRuns(content)
	if len(runs) == 0 {
		return textConcatPiece{}, false
	}
	run := runs[len(runs)-1]
	if first {
		run = runs[0]
	}
	// An escape at the edge is its own entry, which is not literal text.
	if (first && run[0] != 0) || (!first && run[1] != len(content)) {
		return textConcatPiece{}, false
	}
	pos := start + run[0]
	line, col := file.FlatRow(part)+1, file.FlatCol(part)+1+run[0]
	if nl := strings.LastIndexByte(content[:run[0]], '\n'); nl >= 0 {
		line += strings.Count(content[:run[0]], "\n")
		col = run[0] - nl
	}
	return textConcatPiece{text: content[run[0]:run[1]], line: line, col: col, start: pos, endLine: textConcatEndLine(file, lit)}, true
}

// kotlinLiteralRuns splits string content into the [start, end) ranges between escape sequences.
func kotlinLiteralRuns(content string) [][2]int {
	var runs [][2]int
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] != '\\' || i+1 >= len(content) {
			continue
		}
		if i > start {
			runs = append(runs, [2]int{start, i})
		}
		n := 2
		if content[i+1] == 'u' {
			n = 6
		}
		i += n - 1
		start = i + 1
	}
	if start < len(content) {
		runs = append(runs, [2]int{start, len(content)})
	}
	return runs
}

func textConcatEndLine(file *scanner.File, node uint32) int {
	return file.FlatRow(node) + 1 + strings.Count(file.FlatNodeText(node), "\n")
}

// ---------------------------------------------------------------------------
// WrongCommentType
// ---------------------------------------------------------------------------

func (r *WrongCommentTypeRule) check(ctx *api.Context) {
	file := ctx.File
	decl := ctx.Idx
	comment := wctPrecedingComment(file, decl)
	if comment == 0 || !wctIsMember(file, decl) || wctIsPrivate(file, decl) {
		return
	}
	switch file.FlatType(comment) {
	case "multiline_comment", "block_comment":
	default:
		return
	}
	text := file.FlatNodeText(comment)
	if !strings.HasPrefix(text, "/*") || strings.HasPrefix(text, "/**") || strings.Contains(text, "(non-Javadoc)") {
		return
	}
	if !strings.ContainsAny(text, "@[") {
		return
	}
	var tag string
	if file.Language == scanner.LangJava {
		tag = wctJavadocTag(text)
	} else {
		tag = wctKDocTag(text)
	}
	if tag == "" {
		return
	}
	if file.Language != scanner.LangJava && wctSuppressedAt(file, decl, r.RuleName, r.RuleSetName) {
		return
	}
	kind := "KDoc"
	if file.Language == scanner.LangJava {
		kind = "javadoc"
	}
	start := int(file.FlatStartByte(comment))
	line, col := file.FlatRow(comment)+1, file.FlatCol(comment)+1
	length := int(file.FlatEndByte(comment)) - start
	if delta := strings.Index(text, tag); delta >= 0 {
		before := text[:delta]
		if nl := strings.LastIndexByte(before, '\n'); nl >= 0 {
			line += strings.Count(before, "\n")
			col = delta - nl
		} else {
			col += delta
		}
		start += delta
		length = len(tag)
	}
	cs := int(file.FlatStartByte(comment))
	ctx.Emit(scanner.Finding{File: file.Path, Line: line, Col: col, StartByte: start, EndByte: start + length,
		Message: "This block comment looks like it was intended to be a " + kind + " comment",
		Fix:     &scanner.Fix{ByteMode: true, StartByte: cs, EndByte: cs + 2, Replacement: "/**"}})
}

// wctIsMember reports the declarations Lint visits as UClass, UMethod and UField: classes, objects, functions,
// properties and `val`/`var` constructor parameters that are not local to a function body (local classes count).
func wctIsMember(file *scanner.File, decl uint32) bool {
	switch file.FlatType(decl) {
	case "class_parameter":
		_, ok := file.FlatFindChild(decl, "binding_pattern_kind")
		return ok
	case "class_declaration", "object_declaration", "companion_object", "interface_declaration", "enum_declaration",
		"record_declaration", "annotation_type_declaration", "enum_constant", "enum_entry":
		return true
	case "primary_constructor", "getter", "setter":
		return true
	case "property_declaration":
		if !wctPropertyIsField(file, decl) {
			return false
		}
		parent, ok := file.FlatParent(decl)
		return ok && (file.FlatType(parent) == "source_file" || file.FlatType(parent) == "class_body")
	case "function_declaration", "method_declaration", "constructor_declaration",
		"field_declaration", "constant_declaration", "secondary_constructor":
		parent, ok := file.FlatParent(decl)
		if !ok {
			return false
		}
		switch file.FlatType(parent) {
		case "source_file", "class_body", "enum_class_body", "enum_body_declarations", "interface_body":
			return true
		}
		return false
	}
	return false
}

// wctPrecedingComment returns the comment Lint inspects for the declaration. Kotlin binds the run of comments
// right before a declaration (line comments too, blank lines allowed) into it and Lint takes the first; Java's
// backwards walk takes the nearest. tree-sitter attaches such comments to the end of whatever precedes them (a
// package header, the import list, the previous declaration), so trailing descendants count too.
func wctPrecedingComment(file *scanner.File, decl uint32) uint32 {
	var run []uint32
	nextStart := file.FlatStartByte(decl)
	cur := wctPrev(file, decl)
	for depth := 0; cur != 0 && depth < 256; depth++ {
		switch file.FlatType(cur) {
		case "multiline_comment", "block_comment", "line_comment":
			if strings.TrimSpace(string(file.Content[file.FlatEndByte(cur):nextStart])) != "" {
				cur = 0
				continue
			}
			run = append(run, cur)
			nextStart = file.FlatStartByte(cur)
			cur = wctPrev(file, cur)
			continue
		}
		cur = textLastChild(file, cur)
	}
	if len(run) == 0 {
		return 0
	}
	comment := run[0]
	if file.Language != scanner.LangJava {
		comment = run[len(run)-1]
	}
	switch file.FlatType(comment) {
	case "multiline_comment", "block_comment":
		return comment
	}
	return 0
}

// wctPrev is PSI's prev(): the previous sibling, or the parent's when there is none.
func wctPrev(file *scanner.File, n uint32) uint32 {
	for depth := 0; depth < 64; depth++ {
		if prev, ok := file.FlatPrevSibling(n); ok {
			return prev
		}
		parent, ok := file.FlatParent(n)
		if !ok {
			return 0
		}
		n = parent
	}
	return 0
}

// wctPropertyIsField reports whether Lint sees a Kotlin property through its own source element: it has a backing
// field (an initializer, a delegate, lateinit, or an accessor using `field`) or no accessor at all. A property with
// accessors and no field is only reached through the accessors.
func wctPropertyIsField(file *scanner.File, prop uint32) bool {
	hasAccessor, usesField := false, false
	// An accessor on the next line parses as a sibling of the declaration rather than a child.
	parts := []uint32{}
	for child := file.FlatFirstChild(prop); child != 0; child = file.FlatNextSib(child) {
		parts = append(parts, child)
	}
	for next := file.FlatNextSib(prop); next != 0 && (file.FlatType(next) == "getter" || file.FlatType(next) == "setter"); next = file.FlatNextSib(next) {
		parts = append(parts, next)
	}
	for _, child := range parts {
		switch file.FlatType(child) {
		case "=", "property_delegate":
			return true
		case "getter", "setter":
			hasAccessor = true
			file.FlatWalkNodes(child, "simple_identifier", func(id uint32) {
				if file.FlatNodeText(id) == "field" {
					usesField = true
				}
			})
		case "modifiers":
			for _, word := range strings.Fields(file.FlatNodeText(child)) {
				if word == "lateinit" {
					return true
				}
			}
		}
	}
	return !hasAccessor || usesField
}

// wctSuppressedAt reports a @Suppress, @SuppressLint or @SuppressWarnings on the declaration or around it. Kotlin
// binds the leading comment into the declaration, so its suppression covers a finding in the comment.
func wctSuppressedAt(file *scanner.File, decl uint32, ruleID, ruleSet string) bool {
	filter := file.Suppression
	if filter == nil {
		filter = scanner.BuildSuppressionFilter(file, nil, allRuleExcludes(), "").WithRuleAliases(AllSuppressionAliases())
		file.Suppression = filter
		file.SuppressionIdx = filter.Annotations()
	}
	return filter.IsSuppressedAtColumn(ruleID, ruleSet, file.FlatRow(decl)+1, file.FlatCol(decl)+1)
}

func wctIsPrivate(file *scanner.File, decl uint32) bool {
	mods, ok := file.FlatFindChild(decl, "modifiers")
	if !ok {
		return false
	}
	for _, word := range strings.Fields(file.FlatNodeText(mods)) {
		if word == "private" {
			return true
		}
	}
	return false
}

// wctDocLines rebuilds the comment as Lint does before parsing it as a doc comment: each line trimmed, without its
// leading `*`.
func wctDocLines(text string) []string {
	body := strings.TrimSuffix(strings.TrimPrefix(text, "/*"), "*/")
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "*"))
	}
	return lines
}

// wctTagName returns the name of the block tag a doc line starts with, or "".
func wctTagName(line string) string {
	if !strings.HasPrefix(line, "@") {
		return ""
	}
	end := 1
	for end < len(line) && line[end] != ' ' && line[end] != '\t' {
		end++
	}
	return line[1:end]
}

func wctValidTagName(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return name != "" && unicode.IsLower(r)
}

// wctKDocTag returns the text Lint reports for a Kotlin block comment: the first inline `[text](url)` link of the
// description, or the first line of the first lowercase block tag. A tag whose text (up to the next tag) holds a
// period makes Lint give up on the comment.
func wctKDocTag(text string) string {
	lines := wctDocLines(text)
	for i := 0; i < len(lines); i++ {
		name := wctTagName(lines[i])
		if name == "" {
			if link := wctMarkdownLink(lines[i]); link != "" {
				return link
			}
			continue
		}
		if !wctValidTagName(name) {
			continue
		}
		full := lines[i]
		for j := i + 1; j < len(lines) && wctTagName(lines[j]) == ""; j++ {
			full += "\n" + lines[j]
		}
		if strings.Contains(full, ".") {
			return ""
		}
		return lines[i]
	}
	return ""
}

// wctMarkdownLink returns the first `[text](destination)` inline link of a KDoc description line, or "".
func wctMarkdownLink(line string) string {
	for open := strings.IndexByte(line, '['); open >= 0; {
		closeIdx := strings.IndexByte(line[open:], ']')
		if closeIdx < 0 {
			return ""
		}
		closeIdx += open
		if closeIdx+1 < len(line) && line[closeIdx+1] == '(' {
			if end := strings.IndexByte(line[closeIdx+1:], ')'); end > 0 {
				return line[open : closeIdx+2+end]
			}
		}
		next := strings.IndexByte(line[closeIdx:], '[')
		if next < 0 {
			return ""
		}
		open = closeIdx + next
	}
	return ""
}

// wctJavadocTag returns the text Lint reports for a Java block comment: the first `{@link}`, `{@linkplain}` or
// `{@inheritDoc}` of the description, else the first line of the first lowercase block tag.
func wctJavadocTag(text string) string {
	lines := wctDocLines(text)
	for _, line := range lines {
		if wctTagName(line) != "" {
			break
		}
		for rest := line; ; {
			at := strings.Index(rest, "{@")
			if at < 0 {
				break
			}
			end := strings.IndexByte(rest[at:], '}')
			if end < 0 {
				break
			}
			inline := rest[at : at+end+1]
			name := strings.Fields(strings.TrimSuffix(strings.TrimPrefix(inline, "{@"), "}"))
			if len(name) > 0 && (name[0] == "link" || name[0] == "linkplain" || name[0] == "inheritDoc") {
				return inline
			}
			rest = rest[at+end+1:]
		}
	}
	for _, line := range lines {
		if name := wctTagName(line); name != "" && wctValidTagName(name) {
			return line
		}
	}
	return ""
}
