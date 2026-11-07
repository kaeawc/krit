package rules

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// MultilineLambdaItParameterRule detects 'it' in multiline lambdas.
type MultilineLambdaItParameterRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *MultilineLambdaItParameterRule) Confidence() float64 { return api.ConfidenceMedium }

// MultilineRawStringIndentationRule checks raw string indentation.
type MultilineRawStringIndentationRule struct {
	FlatDispatchBase
	BaseRule
	IndentSize      int
	TrimmingMethods []string
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *MultilineRawStringIndentationRule) Confidence() float64 { return api.ConfidenceMedium }

// rawStringSurroundedByLineBreaks reports whether the raw string starts with a line break after its opening quotes
// and ends with a line break (and only blanks) before its closing quotes.
func rawStringSurroundedByLineBreaks(text string) bool {
	inner := strings.TrimSuffix(strings.TrimPrefix(text, `"""`), `"""`)
	open := strings.TrimLeft(inner, " \t")
	if !strings.HasPrefix(open, "\n") && !strings.HasPrefix(open, "\r\n") {
		return false
	}
	last := inner[strings.LastIndexByte(inner, '\n')+1:]
	return strings.TrimSpace(last) == ""
}

// countLeadingBlanks counts the leading whitespace characters of line.
func countLeadingBlanks(line string) int {
	n := 0
	for n < len(line) && (line[n] == ' ' || line[n] == '\t') {
		n++
	}
	return n
}

func rawStringIndentMessage(expected, actual int) string {
	return fmt.Sprintf("The indentation should be %d but it is %d.", expected, actual)
}

// check follows detekt: the content lines of a trimmed multiline raw string must sit one indent step deeper than the
// line that opens it (lines that are shallower are reported one by one, content that is deeper everywhere is reported
// once at its first line), and the closing quotes must align with that line.
func (r *MultilineRawStringIndentationRule) check(ctx *api.Context) {
	file := ctx.File
	if file == nil || !isRawStringLiteralNode(file, ctx.Idx) || !rawStringHasTrimCall(file, ctx.Idx, r.TrimmingMethods) {
		return
	}
	text := file.FlatNodeText(ctx.Idx)
	lines := strings.Split(text, "\n")
	if len(lines) < 2 {
		return
	}
	row := file.FlatRow(ctx.Idx) + 1
	if !rawStringSurroundedByLineBreaks(text) {
		ctx.EmitAt(row, file.FlatCol(ctx.Idx)+1, "A multiline raw string should start with a break line and should end with another")
		return
	}
	size := r.IndentSize
	if size <= 0 {
		size = 4
	}
	start := int(file.FlatStartByte(ctx.Idx))
	if start < 0 || start > len(file.Content) {
		return
	}
	lineStart := strings.LastIndexByte(string(file.Content[:start]), '\n') + 1
	lineEnd := start + strings.IndexByte(string(file.Content[start:])+"\n", '\n')
	base := countLeadingBlanks(string(file.Content[lineStart:lineEnd]))
	want := base + size

	type lineInfo struct {
		row    int
		text   string
		indent int
	}
	var content []lineInfo
	for i := 1; i < len(lines)-1; i++ {
		line := strings.TrimRight(lines[i], "\r")
		content = append(content, lineInfo{row: row + i, text: line, indent: countLeadingBlanks(line)})
	}
	if len(content) > 0 {
		reported := false
		hasWanted := false
		minIndent := content[0].indent
		for _, l := range content {
			if l.indent == want {
				hasWanted = true
			}
			if l.indent < minIndent {
				minIndent = l.indent
			}
			if l.text != "" && l.indent < want {
				col := l.indent + 1
				if strings.TrimSpace(l.text) == "" {
					col = 1
				}
				ctx.EmitAt(l.row, col, rawStringIndentMessage(want, l.indent))
				reported = true
			}
		}
		if !reported && !hasWanted {
			ctx.EmitAt(content[0].row, want+1, rawStringIndentMessage(want, minIndent))
		}
	}
	closing := strings.TrimRight(lines[len(lines)-1], "\r")
	if actual := countLeadingBlanks(closing); actual != base {
		col := base + 1
		if actual < base {
			col = actual + 1
		}
		ctx.EmitAt(row+len(lines)-1, col, rawStringIndentMessage(base, actual))
	}
}

// TrimMultilineRawStringRule detects raw strings missing trimIndent/trimMargin.
type TrimMultilineRawStringRule struct {
	FlatDispatchBase
	BaseRule
	TrimmingMethods []string
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *TrimMultilineRawStringRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *TrimMultilineRawStringRule) check(ctx *api.Context) {
	if !isUntrimmedMultilineRawString(ctx, r.TrimmingMethods) {
		return
	}
	end := int(ctx.File.FlatEndByte(ctx.Idx))
	f := r.Finding(ctx.File, ctx.File.FlatRow(ctx.Idx)+1, ctx.File.FlatCol(ctx.Idx)+1,
		"Multiline raw string should use trimIndent() or trimMargin().")
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   end,
		EndByte:     end,
		Replacement: ".trimIndent()",
	}
	ctx.Emit(f)
}

func isUntrimmedMultilineRawString(ctx *api.Context, trimmingMethods []string) bool {
	if ctx.File == nil || !isRawStringLiteralNode(ctx.File, ctx.Idx) {
		return false
	}
	text := ctx.File.FlatNodeText(ctx.Idx)
	if !strings.Contains(text, "\n") || rawStringHasTrimCall(ctx.File, ctx.Idx, trimmingMethods) {
		return false
	}
	// Annotation arguments and const values cannot call trimIndent().
	for p, ok := ctx.File.FlatParent(ctx.Idx); ok; p, ok = ctx.File.FlatParent(p) {
		switch ctx.File.FlatType(p) {
		case "annotation":
			return false
		case "property_declaration":
			return !ctx.File.FlatHasModifier(p, "const")
		case "function_declaration", "class_body", "source_file":
			return true
		}
	}
	return true
}

func isRawStringLiteralNode(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "string_literal", "multi_line_string_literal":
		return strings.HasPrefix(file.FlatNodeText(idx), `"""`)
	default:
		return false
	}
}

func rawStringHasTrimCall(file *scanner.File, idx uint32, trimmingMethods []string) bool {
	end := int(file.FlatEndByte(idx))
	if end < 0 || end > len(file.Content) {
		return false
	}
	after := strings.TrimSpace(string(file.Content[end:]))
	for _, method := range rawStringTrimMethods(trimmingMethods) {
		// trimMargin("|") takes an argument, so only the opening parenthesis is checked.
		if strings.HasPrefix(after, "."+method+"(") || strings.HasPrefix(after, "?."+method+"(") {
			return true
		}
	}
	return false
}

func rawStringTrimMethods(configured []string) []string {
	if len(configured) > 0 {
		return configured
	}
	return []string{"trimIndent", "trimMargin"}
}

// StringShouldBeRawStringRule detects strings with many escape characters.
type StringShouldBeRawStringRule struct {
	FlatDispatchBase
	BaseRule
	MaxEscapes int
}

var escapeCountRe = regexp.MustCompile(`\\[nrt"\\]`)

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *StringShouldBeRawStringRule) Confidence() float64 { return api.ConfidenceMedium }

// CanBeNonNullableRule detects nullable types that are never assigned null.
// Handles two cases:
// 1. Properties initialized with non-null values that are never reassigned to null.
// 2. Function parameters declared nullable but only used with !! (non-null assertion).
// Skips override/open/abstract functions, delegated properties, and properties with custom setters.
// Tracks null assignments through if/when branches and lambda bodies.
//
// Known limitations:
//   - Cannot track nullable assignments through function calls (fun setNull(x) { field = null })
//   - Cannot detect properties assigned null via reflection or Java interop
type CanBeNonNullableRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CanBeNonNullableRule) SetResolver(_ typeinfer.TypeResolver) {}

// Confidence reports a tier-2 (medium) base confidence — detecting which
// nullable properties are never assigned null requires flow analysis; the
// fallback is a conservative heuristic. Classified per roadmap/17.
func (r *CanBeNonNullableRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *CanBeNonNullableRule) checkPropertyFlat(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	text := file.FlatNodeText(idx)
	if !strings.Contains(text, "?") {
		return
	}
	if file.FlatHasChildOfType(idx, "property_delegate") {
		return
	}
	// Other files, modules and Java callers can assign null to a non-private member, so only private members and locals qualify.
	if parent, ok := file.FlatParent(idx); ok {
		switch file.FlatType(parent) {
		case "class_body", "enum_class_body", "source_file":
			if !file.FlatHasModifier(idx, "private") || file.FlatHasModifier(idx, "open") || file.FlatHasModifier(idx, "override") {
				return
			}
		}
	}
	if canBeNonNullableHasSetter(file, idx) {
		return
	}
	if !canBeNonNullableHasNullableType(file, idx) {
		return
	}
	if !file.FlatHasChildOfType(idx, "=") {
		return
	}
	initExpr := canBeNonNullableFindInitExpr(file, idx)
	if initExpr != 0 && strings.TrimSpace(file.FlatNodeText(initExpr)) == "null" {
		return
	}
	propName := canBeNonNullableFindPropName(file, idx)
	isVar := canBeNonNullableIsVar(file, idx)
	if isVar && propName != "" {
		scope, ok := file.FlatParent(idx)
		if ok && r.hasNullAssignmentInScopeFlat(file, scope, idx, propName, ctx.Resolver) {
			return
		}
	}
	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Property type can be non-nullable since it is initialized with a non-null value and never assigned null."))
}

func canBeNonNullableHasSetter(file *scanner.File, idx uint32) bool {
	if nextSib, ok := file.FlatNextSibling(idx); ok && file.FlatType(nextSib) == "setter" {
		return true
	}
	if nextSib, ok := file.FlatNextSibling(idx); ok && file.FlatType(nextSib) == "getter" {
		if nextNext, ok := file.FlatNextSibling(nextSib); ok && file.FlatType(nextNext) == "setter" {
			return true
		}
	}
	return false
}

func canBeNonNullableHasNullableType(file *scanner.File, idx uint32) bool {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if child == 0 {
			continue
		}
		if file.FlatType(child) == "nullable_type" {
			return true
		}
		if file.FlatType(child) == "variable_declaration" {
			for j := 0; j < file.FlatChildCount(child); j++ {
				gc := file.FlatChild(child, j)
				if file.FlatType(gc) == "nullable_type" {
					return true
				}
			}
		}
	}
	return false
}

func canBeNonNullableFindInitExpr(file *scanner.File, idx uint32) uint32 {
	foundEq := false
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "=" {
			foundEq = true
			continue
		}
		if foundEq {
			return child
		}
	}
	return 0
}

func canBeNonNullableFindPropName(file *scanner.File, idx uint32) string {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "variable_declaration" {
			return extractIdentifierFlat(file, child)
		}
	}
	return ""
}

func canBeNonNullableIsVar(file *scanner.File, idx uint32) bool {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		if file.FlatNodeText(file.FlatChild(idx, i)) == "var" {
			return true
		}
	}
	return false
}

func (r *CanBeNonNullableRule) hasNullAssignmentInScopeFlat(file *scanner.File, scope, declNode uint32, propName string, resolver typeinfer.TypeResolver) bool {
	assignedNull := false
	file.FlatWalkAllNodes(scope, func(child uint32) {
		if assignedNull || child == declNode {
			return
		}
		if file.FlatType(child) == "assignment" || file.FlatType(child) == "augmented_assignment" {
			if file.FlatChildCount(child) < 2 {
				return
			}
			lhs := file.FlatChild(child, 0)
			lhsText := strings.TrimSpace(file.FlatNodeText(lhs))
			if lhsText != propName && lhsText != "this."+propName {
				return
			}
			rhs := file.FlatChild(child, file.FlatChildCount(child)-1)
			if canBeNonNullableRHSCanBeNullFlat(file, rhs, resolver) {
				assignedNull = true
				return
			}
		}
	})
	return assignedNull
}

func canBeNonNullableRHSCanBeNullFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) bool {
	if file == nil || idx == 0 {
		return false
	}
	idx = flatUnwrapParenExpr(file, idx)
	switch file.FlatType(idx) {
	case "null", "null_literal":
		return true
	case "simple_identifier":
		if resolver != nil {
			if typ := resolver.ResolveFlatNode(idx, file); typ != nil && typ.IsNullable() {
				return true
			}
		}
		return false
	case "elvis_expression":
		right := canBeNonNullableElvisRightFlat(file, idx)
		return canBeNonNullableRHSCanBeNullFlat(file, right, resolver)
	case "if_expression", "when_expression":
		return canBeNonNullableBranchCanBeNullFlat(file, idx, resolver)
	case "navigation_expression":
		return canBeNonNullableNavigationUsesSafeCallFlat(file, idx) || canBeNonNullableResolvedNullableFlat(file, idx, resolver)
	case "call_expression":
		return canBeNonNullableCallCanReturnNullFlat(file, idx, resolver)
	case "as_expression":
		return canBeNonNullableAsExpressionCanBeNullFlat(file, idx) || canBeNonNullableResolvedNullableFlat(file, idx, resolver)
	default:
		return canBeNonNullableResolvedNullableFlat(file, idx, resolver)
	}
}

func canBeNonNullableResolvedNullableFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) bool {
	if resolver == nil || idx == 0 {
		return false
	}
	typ := resolver.ResolveFlatNode(idx, file)
	return typ != nil && typ.IsNullable()
}

func canBeNonNullableElvisRightFlat(file *scanner.File, idx uint32) uint32 {
	if file == nil || idx == 0 || file.FlatType(idx) != "elvis_expression" {
		return 0
	}
	seenElvis := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "?:" {
			seenElvis = true
			continue
		}
		if seenElvis && (file.FlatIsNamed(child) || file.FlatType(child) == "null") {
			return child
		}
	}
	return 0
}

func canBeNonNullableBranchCanBeNullFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "control_structure_body" {
			continue
		}
		if canBeNonNullableRHSCanBeNullFlat(file, child, resolver) {
			return true
		}
	}
	return false
}

func canBeNonNullableNavigationUsesSafeCallFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(idx, func(child uint32) {
		if found {
			return
		}
		found = file.FlatType(child) == "?."
	})
	return found
}

func canBeNonNullableCallCanReturnNullFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) bool {
	if file == nil || idx == 0 {
		return false
	}
	if resolver != nil {
		if typ := resolver.ResolveFlatNode(idx, file); typ != nil && typ.IsNullable() {
			return true
		}
	}
	first := file.FlatFirstChild(idx)
	return first != 0 && file.FlatType(first) == "navigation_expression" && canBeNonNullableNavigationUsesSafeCallFlat(file, first)
}

func canBeNonNullableAsExpressionCanBeNullFlat(file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "as?":
			return true
		case "nullable_type":
			return true
		}
	}
	return false
}

func (r *CanBeNonNullableRule) checkFunctionParamsFlat(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if file.FlatHasModifier(idx, "override") || file.FlatHasModifier(idx, "open") || file.FlatHasModifier(idx, "abstract") ||
		file.FlatHasModifier(idx, "actual") || file.FlatHasModifier(idx, "expect") {
		return
	}

	body, _ := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return
	}

	params, _ := file.FlatFindChild(idx, "function_value_parameters")
	if params == 0 {
		return
	}

	for i := 0; i < file.FlatNamedChildCount(params); i++ {
		param := file.FlatNamedChild(params, i)
		if param == 0 || file.FlatType(param) != "parameter" {
			continue
		}
		if !file.FlatHasChildOfType(param, "nullable_type") {
			continue
		}
		// A `= null` default makes null a valid argument, so the type must stay nullable.
		if flatParameterDefaultIsNull(file, param) {
			continue
		}

		paramName := extractIdentifierFlat(file, param)
		if paramName == "" {
			continue
		}

		allNonNullAsserted := true
		usageCount := 0
		file.FlatWalkNodes(body, "simple_identifier", func(child uint32) {
			if !allNonNullAsserted || !file.FlatNodeTextEquals(child, paramName) {
				return
			}
			if unusedParameterReferenceShadowedFlat(file, body, child, paramName) {
				return
			}
			usageCount++
			parent, ok := file.FlatParent(child)
			if !ok {
				allNonNullAsserted = false
				return
			}
			switch file.FlatType(parent) {
			case "non_null_assertion":
				return
			case "postfix_expression", "postfix_unary_expression":
				if strings.HasSuffix(strings.TrimSpace(file.FlatNodeText(parent)), "!!") {
					return
				}
			}
			allNonNullAsserted = false
		})

		if usageCount > 0 && allNonNullAsserted {
			ctx.Emit(r.Finding(file, file.FlatRow(param)+1, file.FlatCol(param)+1,
				fmt.Sprintf("Parameter '%s' can be non-nullable since every usage applies non-null assertion (!!).", paramName)))
		}
	}
}

// DoubleNegativeExpressionRule detects `!isNotEmpty()` etc.
type DoubleNegativeExpressionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence: tier-1 — the detection is purely syntactic. prefix_expression
// with a `!` operator applied to a no-arg call whose callee starts with
// `isNot`/`isNon` is unambiguously a double negative.
func (r *DoubleNegativeExpressionRule) Confidence() float64 { return api.ConfidenceHigher }

// checkDoubleNegativeExpressionFlat runs on a prefix_expression. It fires
// when the shape is `!<expr>.isNot<Suffix>()` or `!isNon<Suffix>()` — a
// unary-bang applied to a zero-argument callable whose name begins with
// `isNot` or `isNon`. Works for qualified (`xs.isNotEmpty()`) and
// unqualified (`isNonBlank()`) callees via flatCallExpressionName.
func (r *DoubleNegativeExpressionRule) checkDoubleNegativeExpressionFlat(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	first := file.FlatFirstChild(idx)
	if first == 0 || file.FlatType(first) != "!" {
		return
	}
	operand := file.FlatNextSib(first)
	for operand != 0 && !file.FlatIsNamed(operand) {
		operand = file.FlatNextSib(operand)
	}
	operand = flatUnwrapParenExpr(file, operand)
	if operand == 0 || file.FlatType(operand) != "call_expression" {
		return
	}
	args := flatCallKeyArguments(file, operand)
	if args != 0 && file.FlatNamedChildCount(args) > 0 {
		return // has arguments — not the zero-arg form we rewrite.
	}
	callee := flatCallExpressionName(file, operand)
	kind, suffix := splitIsNotPrefix(callee)
	if kind == "" {
		return
	}

	receiverPrefix := ""
	for child := file.FlatFirstChild(operand); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "navigation_expression" {
			// everything before the final navigation_suffix is the receiver
			last := flatLastChildOfType(file, child, "navigation_suffix")
			if last != 0 {
				receiverPrefix = strings.TrimSpace(string(file.Content[file.FlatStartByte(child):file.FlatStartByte(last)])) + "."
			}
			break
		}
	}

	var positive string
	switch suffix {
	case "Empty":
		positive = receiverPrefix + "isEmpty()"
	case "Blank":
		positive = receiverPrefix + "isBlank()"
	case "Null":
		positive = receiverPrefix + "isNull()"
	default:
		positive = receiverPrefix + "is" + suffix + "()"
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Double negative expression. Simplify by using the positive variant.")
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(idx)),
		EndByte:     int(file.FlatEndByte(idx)),
		Replacement: positive,
	}
	ctx.Emit(f)
}

// splitIsNotPrefix returns ("Not"|"Non", suffix) if name starts with
// "isNot<Uppercase...>" or "isNon<Uppercase...>", else ("", "").
func splitIsNotPrefix(name string) (kind, suffix string) {
	for _, prefix := range []string{"isNot", "isNon"} {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		rest := name[len(prefix):]
		if rest == "" {
			return "", ""
		}
		first := rest[0]
		if first < 'A' || first > 'Z' {
			return "", ""
		}
		return prefix[2:], rest
	}
	return "", ""
}

// DoubleNegativeLambdaRule detects a negative function such as `none` or `takeUnless` whose lambda also negates.
type DoubleNegativeLambdaRule struct {
	FlatDispatchBase
	BaseRule
	// NegativeFunctions are the callee names whose lambda must not contain a negation.
	NegativeFunctions []string
	// NegativeFunctionNameParts are the camel-case words that mark a called function as a negation.
	NegativeFunctionNameParts []string
}

// doubleNegativeRecommendations holds detekt's reasons for its default negative functions; krit adds filterNot.
var doubleNegativeRecommendations = map[string]string{
	"none":       "Use `all` instead.",
	"takeUnless": "Use `takeIf` instead.",
	"filterNot":  "Use `filter` instead.",
}

func (r *DoubleNegativeLambdaRule) Confidence() float64 { return api.ConfidenceHigher }

// camelCaseWords splits at each lowercase-to-uppercase boundary, like the regex (?<=[a-z])(?=[A-Z]).
func camelCaseWords(text string) []string {
	var words []string
	start := 0
	for i := 1; i < len(text); i++ {
		if text[i-1] >= 'a' && text[i-1] <= 'z' && text[i] >= 'A' && text[i] <= 'Z' {
			words = append(words, text[start:i])
			start = i
		}
	}
	return append(words, text[start:])
}

// doubleNegativeCallText returns the text of a call without its receiver, such as `isNotEmpty()`.
func doubleNegativeCallText(file *scanner.File, call uint32) string {
	name := flatCallExpressionName(file, call)
	if name == "" {
		return ""
	}
	if suffix, ok := file.FlatFindChild(call, "call_suffix"); ok && suffix != 0 {
		return name + file.FlatNodeText(suffix)
	}
	return name
}

// forbiddenNegations lists, in source order, the negating operators and calls inside a lambda.
func (r *DoubleNegativeLambdaRule) forbiddenNegations(file *scanner.File, lambda uint32) []string {
	type hit struct {
		start int
		text  string
	}
	var hits []hit
	parts := r.NegativeFunctionNameParts
	file.FlatWalkAllNodes(lambda, func(n uint32) {
		switch file.FlatType(n) {
		case "!", "!=", "!==", "!is", "!in":
			hits = append(hits, hit{int(file.FlatStartByte(n)), file.FlatNodeText(n)})
		case "call_expression":
			text := doubleNegativeCallText(file, n)
			if text == "" {
				return
			}
			if text == "not()" {
				hits = append(hits, hit{int(file.FlatStartByte(n)), text})
				return
			}
			for _, word := range camelCaseWords(text) {
				for _, part := range parts {
					if strings.ToLower(word) == part {
						hits = append(hits, hit{int(file.FlatStartByte(n)), text})
						return
					}
				}
			}
		}
	})
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].start < hits[j].start })
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.text
	}
	return out
}

// checkDoubleNegativeLambdaFlat follows detekt: a call to a configured negative function whose trailing lambda holds
// a negating operator or a call named like a negation.
func (r *DoubleNegativeLambdaRule) checkDoubleNegativeLambdaFlat(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	callee := flatCallExpressionName(file, idx)
	if !doubleNegativeLambdaCalleeConfigured(callee, r.NegativeFunctions) {
		return
	}
	lambda := flatCallTrailingLambda(file, idx)
	if lambda == 0 {
		return
	}
	hits := r.forbiddenNegations(file, lambda)
	if len(hits) == 0 {
		return
	}
	recommendation := doubleNegativeRecommendations[callee]
	if recommendation == "" {
		recommendation = "Rewrite in the positive."
	}
	quoted := make([]string, len(hits))
	for i, h := range hits {
		quoted[i] = "`" + h + "`"
	}
	msg := "Double negative through using " + strings.Join(quoted, ", ") + " inside a `" + callee + "` lambda. " + recommendation
	f := r.Finding(file, callLine(file, idx), callCol(file, idx), msg)
	// Only a lone `!pred` body of filterNot or none has a deterministic rewrite.
	if replacement := map[string]string{"filterNot": "filter", "none": "all"}[callee]; replacement != "" {
		if stmts, _ := file.FlatFindChild(lambda, "statements"); stmts != 0 && file.FlatNamedChildCount(stmts) == 1 {
			body := flatUnwrapParenExpr(file, file.FlatNamedChild(stmts, 0))
			if file.FlatType(body) == "prefix_expression" {
				if op := file.FlatFirstChild(body); op != 0 && file.FlatType(op) == "!" {
					if operand := file.FlatNextSib(op); operand == 0 || prefixMutationOperandFlat(file, operand) == operand {
						f.Fix = buildDoubleNegativeLambdaFix(file, idx, op, callee, replacement)
					}
				}
			}
		}
	}
	ctx.Emit(f)
}

// doubleNegativeLambdaCalleeConfigured reports whether the callee name appears in the configured negative functions.
func doubleNegativeLambdaCalleeConfigured(callee string, configured []string) bool {
	if callee == "" {
		return false
	}
	for _, name := range configured {
		if name == callee {
			return true
		}
	}
	return false
}

// buildDoubleNegativeLambdaFix rewrites `recv.filterNot { !pred }` to
// `recv.filter { pred }` (and `none`→`all`) by renaming the callee
// identifier and removing the `!` prefix from the lambda body. Returns
// nil when either edit cannot be located.
func buildDoubleNegativeLambdaFix(file *scanner.File, call uint32, bangOp uint32, oldName, newName string) *scanner.Fix {
	var calleeIdent uint32
	for c := file.FlatFirstChild(call); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "simple_identifier":
			if file.FlatNodeTextEquals(c, oldName) {
				calleeIdent = c
			}
		case "navigation_expression":
			calleeIdent = flatNavigationExpressionLastIdentifierNamed(file, c, oldName)
		}
		if calleeIdent != 0 {
			break
		}
	}
	if calleeIdent == 0 || bangOp == 0 {
		return nil
	}

	callStart := int(file.FlatStartByte(call))
	callEnd := int(file.FlatEndByte(call))
	edits := []byteEdit{
		{int(file.FlatStartByte(calleeIdent)), int(file.FlatEndByte(calleeIdent)), newName},
		{int(file.FlatStartByte(bangOp)), int(file.FlatEndByte(bangOp)), ""},
	}
	repl, ok := applyByteEdits(file.Content, callStart, callEnd, edits)
	if !ok {
		return nil
	}
	return &scanner.Fix{
		ByteMode:    true,
		StartByte:   callStart,
		EndByte:     callEnd,
		Replacement: repl,
	}
}

// NullableBooleanCheckRule detects `x == true` on Boolean?.
type NullableBooleanCheckRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *NullableBooleanCheckRule) Confidence() float64 { return api.ConfidenceMedium }

// RangeUntilInsteadOfRangeToRule detects `until` usage that could use `..<`.
type RangeUntilInsteadOfRangeToRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *RangeUntilInsteadOfRangeToRule) Confidence() float64 { return api.ConfidenceMedium }

// UntilInsteadOfRangeToRule detects `a..b - 1` and `a.rangeTo(b - 1)`, which `a until b` states directly.
type UntilInsteadOfRangeToRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. The check is syntactic: it cannot tell whether the operands
// are integers, for which until applies.
func (r *UntilInsteadOfRangeToRule) Confidence() float64 { return api.ConfidenceMedium }

// isMinusOneExpression reports `x - 1` where the right side is the literal 1.
func isMinusOneExpression(file *scanner.File, n uint32) bool {
	if n == 0 || file.FlatType(n) != "additive_expression" || file.FlatChildCount(n) != 3 {
		return false
	}
	op := file.FlatChild(n, 1)
	right := file.FlatChild(n, 2)
	return op != 0 && right != 0 && file.FlatNodeTextEquals(op, "-") && strings.TrimSpace(file.FlatNodeText(right)) == "1"
}

func (r *UntilInsteadOfRangeToRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	switch file.FlatType(idx) {
	case "range_expression":
		if file.FlatChildCount(idx) != 3 {
			return
		}
		op := file.FlatChild(idx, 1)
		if op == 0 || !file.FlatNodeTextEquals(op, "..") || !isMinusOneExpression(file, file.FlatChild(idx, 2)) {
			return
		}
		ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "'..' call can be replaced with 'until'")
	case "call_expression":
		if flatCallExpressionName(file, idx) != "rangeTo" {
			return
		}
		_, args := flatCallExpressionParts(file, idx)
		if args == 0 {
			return
		}
		var only uint32
		count := 0
		for c := file.FlatFirstChild(args); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "value_argument" {
				only = flatValueArgumentExpression(file, c)
				count++
			}
		}
		if count != 1 || !isMinusOneExpression(file, only) {
			return
		}
		ctx.EmitAt(callLine(file, idx), callCol(file, idx), "'rangeTo' call can be replaced with 'until'")
	}
}

// DestructuringDeclarationWithTooManyEntriesRule limits destructuring entries.
type DestructuringDeclarationWithTooManyEntriesRule struct {
	FlatDispatchBase
	BaseRule
	MaxEntries int
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *DestructuringDeclarationWithTooManyEntriesRule) Confidence() float64 {
	return api.ConfidenceMedium
}

// nullableBooleanOperand reports whether the compared operand can be a nullable Boolean:
// the resolver says so, or the type is unknown and the operand contains a safe call.
func nullableBooleanOperand(ctx *api.Context, operand uint32) bool {
	file := ctx.File
	// An elvis chain is nullable exactly when its last operand is.
	if file.FlatType(operand) == "elvis_expression" && file.FlatChildCount(operand) >= 3 {
		return nullableBooleanOperand(ctx, file.FlatChild(operand, file.FlatChildCount(operand)-1))
	}
	text := file.FlatNodeText(operand)
	if strings.Contains(text, "?.") && !strings.HasSuffix(text, "!!") {
		return true
	}
	if ctx.Resolver != nil {
		if t := ctx.Resolver.ResolveFlatNode(operand, file); t != nil && t.Kind != typeinfer.TypeUnknown {
			return t.IsNullable()
		}
	}
	return false
}

// lambdaUsesImplicitIt reports whether node (inside the lambda root) reads the lambda's implicit `it`.
// Nested lambdas without parameters and nested functions have their own `it`, so they are not entered.
func lambdaUsesImplicitIt(file *scanner.File, root, node uint32) bool {
	for c := file.FlatFirstChild(node); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "simple_identifier":
			if file.FlatNodeTextEquals(c, "it") {
				return true
			}
		case "lambda_literal":
			if !file.FlatHasChildOfType(c, "lambda_parameters") {
				continue
			}
		case "function_declaration", "anonymous_function", "class_declaration", "object_declaration":
			continue
		}
		if lambdaUsesImplicitIt(file, root, c) {
			return true
		}
	}
	return false
}

// flatParameterDefaultIsNull reports whether a parameter declares `= null` as its default value.
// The grammar places `=` and the default expression as siblings after the parameter node.
func flatParameterDefaultIsNull(file *scanner.File, param uint32) bool {
	eq := file.FlatNextSib(param)
	if eq == 0 || file.FlatType(eq) != "=" {
		return false
	}
	value := file.FlatNextSib(eq)
	return value != 0 && file.FlatNodeTextEquals(value, "null")
}
