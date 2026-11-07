package rules

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func flatIsUsedAsExpression(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	if ignoredReturnValueFollowsDetachedReturn(file, idx) {
		return true
	}
	parent, ok := file.FlatParent(idx)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "source_file":
		return false
	case "statements":
		return flatIsUsedAsExpressionInStatements(file, idx, parent)
	case "property_declaration", "variable_declaration", "value_argument", "value_arguments",
		"return_expression", "jump_expression", "assignment", "augmented_assignment",
		"binary_expression", "comparison_expression", "equality_expression",
		"additive_expression", "multiplicative_expression", "conjunction_expression",
		"disjunction_expression", "elvis_expression", "range_expression",
		"infix_expression", "check_expression", "as_expression", "if_expression",
		"when_expression", "call_expression":
		return true
	case "parenthesized_expression", "navigation_expression":
		return flatIsUsedAsExpression(file, parent)
	case "lambda_literal":
		for i := 0; i < file.FlatNamedChildCount(parent); i++ {
			child := file.FlatNamedChild(parent, i)
			if file.FlatType(child) == "statements" {
				return flatLastCodeNamedChild(file, child) == idx
			}
		}
		return false
	case "control_structure_body":
		return flatIsUsedAsExpressionInControlBody(file, parent)
	case "when_entry":
		if gp, ok := file.FlatParent(parent); ok && file.FlatType(gp) == "when_expression" {
			return flatIsUsedAsExpression(file, gp)
		}
		return false
	default:
		return true
	}
}

func flatIsUsedAsExpressionInStatements(file *scanner.File, idx, parent uint32) bool {
	for prev, ok := file.FlatPrevSibling(idx); ok; prev, ok = file.FlatPrevSibling(prev) {
		if !file.FlatTree.NodeIsNamed(prev) {
			continue
		}
		if file.FlatType(prev) == "jump_expression" {
			return true
		}
	}
	gp, ok := file.FlatParent(parent)
	if !ok {
		return false
	}
	gpType := file.FlatType(gp)
	if gpType == "control_structure_body" || gpType == "lambda_literal" {
		if flatLastCodeNamedChild(file, parent) == idx {
			return flatIsUsedAsExpression(file, gp)
		}
	}
	if gpType == "catch_block" || gpType == "finally_block" {
		if flatLastCodeNamedChild(file, parent) == idx {
			if ggp, ok := file.FlatParent(gp); ok && file.FlatType(ggp) == "try_expression" {
				return flatIsUsedAsExpression(file, ggp)
			}
		}
	}
	if gpType == "try_expression" && flatLastCodeNamedChild(file, parent) == idx {
		return flatIsUsedAsExpression(file, gp)
	}
	return false
}

func flatIsUsedAsExpressionInControlBody(file *scanner.File, parent uint32) bool {
	gp, ok := file.FlatParent(parent)
	if !ok {
		return false
	}
	gpType := file.FlatType(gp)
	if gpType == "if_expression" || gpType == "when_expression" {
		return flatIsUsedAsExpression(file, gp)
	}
	if gpType == "when_entry" {
		if ggp, ok := file.FlatParent(gp); ok && file.FlatType(ggp) == "when_expression" {
			return flatIsUsedAsExpression(file, ggp)
		}
	}
	return false
}

// hasCheckReturnAnnotation checks if any annotation in the list matches the
// built-in @CheckReturnValue/@CheckResult patterns or the user-configured patterns.
func hasCheckReturnAnnotation(annotations []string, configPatterns []string) bool {
	for _, ann := range annotations {
		// Check against built-in annotation set
		if checkReturnValueAnnotations[ann] {
			return true
		}
		// Also check the simple name (last segment after '.')
		if idx := strings.LastIndex(ann, "."); idx >= 0 {
			if checkReturnValueAnnotations[ann[idx+1:]] {
				return true
			}
		}
		// Check user-configured patterns (supports wildcards like "*.CheckResult")
		for _, pat := range configPatterns {
			if matchAnnotationPattern(pat, ann) {
				return true
			}
		}
	}
	return false
}

// hasIgnoreReturnAnnotation checks if any annotation overrides @CheckReturnValue
// (e.g., @CanIgnoreReturnValue).
func hasIgnoreReturnAnnotation(annotations []string, configPatterns []string) bool {
	for _, ann := range annotations {
		if canIgnoreReturnValueAnnotations[ann] {
			return true
		}
		if idx := strings.LastIndex(ann, "."); idx >= 0 {
			if canIgnoreReturnValueAnnotations[ann[idx+1:]] {
				return true
			}
		}
		for _, pat := range configPatterns {
			if matchAnnotationPattern(pat, ann) {
				return true
			}
		}
	}
	return false
}

// ignoredReturnValueAnnotatedMessage names the must-use annotation the resolved callee carries. An Rx processor's
// offer returns whether the item was accepted, so the message also points at onNext for callers that do not care.
func ignoredReturnValueAnnotatedMessage(funcName string, annotations, configPatterns []string) string {
	found := ""
	for _, ann := range annotations {
		if hasCheckReturnAnnotation([]string{ann}, configPatterns) {
			found = ann
			break
		}
	}
	simple := found
	if i := strings.LastIndex(found, "."); i >= 0 {
		simple = found[i+1:]
	}
	msg := fmt.Sprintf("Return value of '%s' is ignored. The function is annotated with @%s (%s).", funcName, simple, found)
	if funcName == "offer" && strings.HasPrefix(found, "io.reactivex.") {
		msg += " Check the result, or call onNext() when delivery does not need to be confirmed."
	}
	return msg
}

func ignoredReturnValueAnnotationEvidence(annotations []string, checkPatterns, ignorePatterns []string) (check bool, ignored bool) {
	return hasCheckReturnAnnotation(annotations, checkPatterns), hasIgnoreReturnAnnotation(annotations, ignorePatterns)
}

func ignoredReturnValueOracleAnnotations(lookup oracleAnnotationLookup, filePath string, line, col int) []string {
	if lookup == nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	add := func(values []string) {
		for _, ann := range values {
			if ann == "" || seen[ann] {
				continue
			}
			seen[ann] = true
			out = append(out, ann)
		}
	}
	add(lookup.LookupCallTargetAnnotations(filePath, line, col))
	return out
}

func ignoredReturnValueMergedOracleAnnotations(lookup oracle.Lookup, file *scanner.File, idx uint32, line, col int) []string {
	if lookup == nil || file == nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	add := func(values []string) {
		for _, ann := range values {
			if ann == "" || seen[ann] {
				continue
			}
			seen[ann] = true
			out = append(out, ann)
		}
	}
	add(oracleLookupCallTargetAnnotationsFlat(lookup, file, idx))
	add(ignoredReturnValueOracleAnnotations(lookup, file.Path, line, col))
	return out
}

type oracleAnnotationLookup interface {
	LookupCallTargetAnnotations(filePath string, line, col int) []string
}

// matchAnnotationPattern matches an annotation FQN against a pattern.
// Patterns: exact match, "*.Name" matches any FQN ending with ".Name", "*Name"
// matches a suffix.
func matchAnnotationPattern(pattern, annotation string) bool {
	if pattern == annotation {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".Name"
		return strings.HasSuffix(annotation, suffix)
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(annotation, pattern[1:])
	}
	return false
}

// matchesReturnValueType checks if a resolved FQN matches the configured
// returnValueTypes patterns (e.g., "kotlinx.coroutines.flow.*Flow").
func matchesReturnValueType(fqn string, patterns []string) bool {
	fqn = strings.TrimSpace(fqn)
	if fqn == "" {
		return false
	}
	if idx := strings.IndexByte(fqn, '<'); idx >= 0 {
		fqn = fqn[:idx]
	}
	if returnValueFQNs[fqn] {
		return true
	}
	for _, pat := range patterns {
		pat = strings.TrimSpace(pat)
		if pat == fqn {
			return true
		}
		// "pkg.*Suffix" — prefix + suffix glob
		if idx := strings.Index(pat, "*"); idx >= 0 {
			prefix := pat[:idx]
			suffix := pat[idx+1:]
			if strings.HasPrefix(fqn, prefix) && strings.HasSuffix(fqn, suffix) {
				return true
			}
		}
	}
	return false
}

func ignoredReturnValueTypeMatches(rt *typeinfer.ResolvedType, configured []string, restrictToConfig bool) bool {
	if !ignoredReturnValueTypeKnown(rt) ||
		ignoredReturnValueTypeIsUnitOrNothing(rt) {
		return false
	}
	if rt.Kind == typeinfer.TypeFunction ||
		strings.HasPrefix(rt.FQN, "kotlin.Function") ||
		strings.HasPrefix(rt.Name, "Function") {
		return !restrictToConfig || matchesReturnValueType("kotlin.Function", configured)
	}
	patterns := configured
	if !restrictToConfig {
		patterns = append(append([]string{}, defaultReturnValueTypePatterns...), configured...)
	}
	if matchesReturnValueType(rt.FQN, patterns) {
		return true
	}
	if rt.FQN == "" && rt.Name != "" {
		return matchesReturnValueType(rt.Name, patterns)
	}
	return false
}

func ignoredReturnValueTypeKnown(rt *typeinfer.ResolvedType) bool {
	if rt == nil || rt.Kind == typeinfer.TypeUnknown {
		return false
	}
	return rt.FQN != "" || rt.Name != "" || rt.Kind == typeinfer.TypeFunction ||
		rt.Kind == typeinfer.TypeUnit || rt.Kind == typeinfer.TypeNothing
}

func ignoredReturnValueTypeIsUnitOrNothing(rt *typeinfer.ResolvedType) bool {
	if rt == nil {
		return false
	}
	// `x?.let { return }` has type Nothing?, which carries no value either.
	if strings.TrimSuffix(rt.FQN, "?") == "kotlin.Nothing" || strings.TrimSuffix(rt.Name, "?") == "Nothing" {
		return true
	}
	return rt.Kind == typeinfer.TypeUnit || rt.Kind == typeinfer.TypeNothing ||
		rt.FQN == "kotlin.Unit" || rt.FQN == "kotlin.Nothing" ||
		rt.Name == "Unit" || rt.Name == "Nothing"
}

func ignoredReturnValueTypeName(rt *typeinfer.ResolvedType) string {
	if rt == nil {
		return "a must-use type"
	}
	if rt.FQN != "" {
		return rt.FQN
	}
	if rt.Name != "" {
		return rt.Name
	}
	return "a must-use type"
}

var ignoredReturnValueCollectionSourceCalls = map[string]bool{
	"arrayListOf":   true,
	"asFlow":        true,
	"asSequence":    true,
	"emptyList":     true,
	"emptyMap":      true,
	"emptySet":      true,
	"flow":          true,
	"flowOf":        true,
	"hashMapOf":     true,
	"hashSetOf":     true,
	"linkedMapOf":   true,
	"linkedSetOf":   true,
	"listOf":        true,
	"mapOf":         true,
	"mutableListOf": true,
	"mutableMapOf":  true,
	"mutableSetOf":  true,
	"sequence":      true,
	"sequenceOf":    true,
	"setOf":         true,
	"stream":        true,
	"streamOf":      true,
	"toList":        true,
	"toMap":         true,
	"toSet":         true,
}

var ignoredReturnValueReceiverTypeHints = []string{
	"Array",
	"Collection",
	"Flow",
	"Iterable",
	"List",
	"Map",
	"Sequence",
	"Set",
	"Stream",
}

func ignoredReturnValueFunctionalFallbackHasReceiverEvidence(file *scanner.File, call uint32) bool {
	if file == nil || call == 0 || file.FlatType(call) != "call_expression" {
		return false
	}
	navExpr, _ := flatCallExpressionParts(file, call)
	if navExpr == 0 || file.FlatType(navExpr) != "navigation_expression" || file.FlatNamedChildCount(navExpr) == 0 {
		return false
	}
	return ignoredReturnValueReceiverLooksFunctionalSource(file, flatUnwrapParenExpr(file, file.FlatNamedChild(navExpr, 0)))
}

func ignoredReturnValueReceiverLooksFunctionalSource(file *scanner.File, receiver uint32) bool {
	if file == nil || receiver == 0 {
		return false
	}
	receiver = flatUnwrapParenExpr(file, receiver)
	switch file.FlatType(receiver) {
	case "simple_identifier":
		return ignoredReturnValueIdentifierHasFunctionalType(file, file.FlatNodeText(receiver))
	case "navigation_expression":
		return ignoredReturnValueNavigationLooksFunctionalSource(file, receiver)
	case "call_expression":
		return ignoredReturnValueCallLooksFunctionalSource(file, receiver)
	default:
		return false
	}
}

func ignoredReturnValueNavigationLooksFunctionalSource(file *scanner.File, nav uint32) bool {
	if file == nil || nav == 0 || file.FlatType(nav) != "navigation_expression" {
		return false
	}
	if ignoredReturnValueIdentifierHasFunctionalType(file, flatNavigationExpressionLastIdentifier(file, nav)) {
		return true
	}
	if file.FlatNamedChildCount(nav) == 0 {
		return false
	}
	return ignoredReturnValueReceiverLooksFunctionalSource(file, file.FlatNamedChild(nav, 0))
}

func ignoredReturnValueCallLooksFunctionalSource(file *scanner.File, call uint32) bool {
	name := flatCallExpressionName(file, call)
	if name == "" {
		return false
	}
	if ignoredReturnValueCollectionSourceCalls[name] ||
		ignoredReturnValueFunctionReturnsFunctionalType(file, name) {
		return true
	}
	if !functionalOps[name] {
		return false
	}
	return ignoredReturnValueFunctionalFallbackHasReceiverEvidence(file, call)
}

func ignoredReturnValueIdentifierHasFunctionalType(file *scanner.File, name string) bool {
	if file == nil || name == "" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if found {
			return
		}
		switch file.FlatType(idx) {
		case "function_value_parameter", "parameter", "variable_declaration", "property_declaration":
			declName, typeText := ignoredReturnValueDeclarationNameAndType(file, idx)
			found = declName == name && (ignoredReturnValueTypeTextLooksFunctional(typeText) ||
				ignoredReturnValueDeclarationInitializerLooksFunctional(file, idx))
		}
	})
	return found
}

func ignoredReturnValueFunctionReturnsFunctionalType(file *scanner.File, name string) bool {
	if file == nil || name == "" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if found || file.FlatType(idx) != "function_declaration" || flatFunctionName(file, idx) != name {
			return
		}
		found = ignoredReturnValueTypeTextLooksFunctional(ignoredReturnValueFunctionReturnTypeText(file, idx))
	})
	return found
}

func ignoredReturnValueDeclarationNameAndType(file *scanner.File, idx uint32) (string, string) {
	if file == nil || idx == 0 {
		return "", ""
	}
	return extractIdentifierFlat(file, idx), explicitTypeTextFlat(file, idx)
}

func ignoredReturnValueFunctionReturnTypeText(file *scanner.File, fn uint32) string {
	if file == nil || fn == 0 || file.FlatType(fn) != "function_declaration" {
		return ""
	}
	return explicitTypeTextFlat(file, fn)
}

func ignoredReturnValueDeclarationInitializerLooksFunctional(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		if found || candidate == idx || file.FlatType(candidate) != "call_expression" {
			return
		}
		found = ignoredReturnValueCallLooksFunctionalSource(file, candidate)
	})
	return found
}

func ignoredReturnValueTypeTextLooksFunctional(typeText string) bool {
	typeText = strings.TrimSpace(typeText)
	if typeText == "" {
		return false
	}
	for _, hint := range ignoredReturnValueReceiverTypeHints {
		if typeText == hint ||
			strings.HasPrefix(typeText, hint+"<") ||
			strings.HasPrefix(typeText, hint+"?") ||
			strings.HasSuffix(typeText, "."+hint) ||
			strings.Contains(typeText, "."+hint+"<") {
			return true
		}
	}
	return false
}

func stringListContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// ignoredReturnValueIsFunctionValueInvoke reports a bare call to a function-typed parameter or property, such as
// `eventSink(x)`. The call yields the function's return value, not the function value itself.
func ignoredReturnValueIsFunctionValueInvoke(file *scanner.File, idx uint32) bool {
	first := file.FlatFirstChild(idx)
	if first == 0 || file.FlatType(first) != "simple_identifier" {
		return false
	}
	name := file.FlatNodeText(first)
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration", "class_declaration", "object_declaration":
			// Cheap necessary check on the whole scope before walking its declarations.
			if !textDeclaresFunctionTyped(file.FlatNodeBytes(p), name, false) {
				continue
			}
			found := false
			for _, declType := range []string{"parameter", "class_parameter", "property_declaration"} {
				file.FlatWalkNodes(p, declType, func(decl uint32) {
					if !found && textDeclaresFunctionTyped(file.FlatNodeBytes(decl), name, true) {
						found = true
					}
				})
			}
			if found {
				return true
			}
		}
	}
	return false
}

// textDeclaresFunctionTyped reports whether text holds `name : ... ->` with no `=` before the arrow, the
// shape of a function-typed parameter or property. Equivalent to matching
// `(?:^|[\s(,])name\s*:\s*[^=]*->` without compiling a regexp per call. With boundary false the leading
// `^|[\s(,]` check is skipped, which makes the result a superset usable as a prefilter.
func textDeclaresFunctionTyped(text []byte, name string, boundary bool) bool {
	if name == "" {
		return false
	}
	for off := 0; off < len(text); {
		i := bytes.Index(text[off:], []byte(name))
		if i < 0 {
			return false
		}
		i += off
		off = i + 1
		if boundary && i > 0 && !isFunctionTypeBoundary(text[i-1]) {
			continue
		}
		j := i + len(name)
		for j < len(text) && isRegexpSpace(text[j]) {
			j++
		}
		if j >= len(text) || text[j] != ':' {
			continue
		}
		j++
		arrow := bytes.Index(text[j:], []byte("->"))
		if arrow < 0 {
			return false
		}
		if eq := bytes.IndexByte(text[j:j+arrow], '='); eq < 0 {
			return true
		}
	}
	return false
}

// isRegexpSpace matches RE2's \s class.
func isRegexpSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\f' || c == '\r'
}

func isFunctionTypeBoundary(c byte) bool { return isRegexpSpace(c) || c == '(' || c == ',' }

// ignoredReturnValueLambdaReturnsFromEnclosingFunction reports a call whose lambda contains a non-local `return`,
// as in `x?.let { return it }`. The call then never completes normally, so there is no ignored result.
func ignoredReturnValueLambdaReturnsFromEnclosingFunction(file *scanner.File, idx uint32) bool {
	enclosing := ""
	if fn, ok := flatEnclosingFunction(file, idx); ok {
		enclosing = flatFunctionName(file, fn)
	}
	found := false
	file.FlatWalkNodes(idx, "jump_expression", func(n uint32) {
		if found || !file.FlatHasAncestorOfType(n, "lambda_literal") {
			return
		}
		text := strings.TrimSpace(file.FlatNodeText(n))
		if !strings.HasPrefix(text, "return") {
			return
		}
		rest := strings.TrimPrefix(text, "return")
		if strings.HasPrefix(rest, "@") {
			label := strings.TrimPrefix(rest, "@")
			end := strings.IndexFunc(label, func(r rune) bool {
				return r != '_' && (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z')
			})
			if end >= 0 {
				label = label[:end]
			}
			found = enclosing != "" && label == enclosing
			return
		}
		found = true
	})
	return found
}

var ignoredReturnValueDetachedReturnPattern = regexp.MustCompile(`(?:^|[^\w@.])return(?:@\w+)?[ \t]+$`)

// ignoredReturnValueFollowsDetachedReturn reports a call that is the operand of a `return` or
// `return@label` on the same line. The parser detaches it from the jump inside `if` and `?:`.
func ignoredReturnValueFollowsDetachedReturn(file *scanner.File, idx uint32) bool {
	start := int(file.FlatStartByte(idx))
	if start <= 0 || start > len(file.Content) {
		return false
	}
	lineStart := start
	for lineStart > 0 && file.Content[lineStart-1] != '\n' {
		lineStart--
	}
	return ignoredReturnValueDetachedReturnPattern.Match(file.Content[lineStart:start])
}

// ignoredReturnValueHasPlainArgument reports whether the call passes at least one argument.
func ignoredReturnValueHasPlainArgument(file *scanner.File, call uint32) bool {
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return false
	}
	_, count := flatValueArgumentStats(file, args)
	return count > 0
}

// ignoredReturnValueHelperResultUsedElsewhere reports a function declared in this file whose result
// another call site of the same name uses, so ignoring it at one site is an intentional choice.
func ignoredReturnValueHelperResultUsedElsewhere(file *scanner.File, idx uint32, name string) bool {
	if file == nil || name == "" || !ignoredReturnValueFileDeclaresFunction(file, name) {
		return false
	}
	used := false
	file.FlatWalkNodes(0, "call_expression", func(call uint32) {
		if used || call == idx || flatCallExpressionName(file, call) != name {
			return
		}
		used = flatIsUsedAsExpression(file, call)
	})
	return used
}

func ignoredReturnValueFileDeclaresFunction(file *scanner.File, name string) bool {
	names := filefacts.FileFact(fileFactsCache(), file, slotDeclaredFunctionNames, func() map[string]struct{} {
		out := map[string]struct{}{}
		file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
			out[flatFunctionName(file, fn)] = struct{}{}
		})
		return out
	})
	_, found := names[name]
	return found
}

// ignoredReturnValueTestFileDisposableCall reports Rx/processor calls whose result tests drop on purpose.
func ignoredReturnValueTestFileDisposableCall(file *scanner.File, funcName string) bool {
	if !scanner.IsTestFile(file.Path) {
		return false
	}
	switch funcName {
	case "test", "offer", "subscribe":
		return true
	}
	return false
}

// ignoredReturnValueFunctionTypeFromInvoke reports a function-typed result that comes from invoking a function value.
// The expression type of `callback?.invoke(x)` or `state.eventSink(x)` is the callee's type, not a returned function.
func ignoredReturnValueFunctionTypeFromInvoke(file *scanner.File, call uint32, funcName string, rt *typeinfer.ResolvedType) bool {
	if rt == nil || rt.Kind != typeinfer.TypeFunction && !strings.HasPrefix(rt.FQN, "kotlin.Function") &&
		!strings.HasPrefix(rt.Name, "Function") {
		return false
	}
	if funcName == "invoke" {
		return true
	}
	navExpr, _ := flatCallExpressionParts(file, call)
	return navExpr != 0 && !ignoredReturnValueFunctionReturnsFunctionalType(file, funcName)
}

// flatLastCodeNamedChild returns the last named child that is not a comment, since a trailing
// comment after the final expression does not make that expression discarded.
func flatLastCodeNamedChild(file *scanner.File, idx uint32) uint32 {
	if file == nil || idx == 0 {
		return 0
	}
	for i := file.FlatNamedChildCount(idx) - 1; i >= 0; i-- {
		child := file.FlatNamedChild(idx, i)
		switch file.FlatType(child) {
		case "line_comment", "multiline_comment":
			continue
		}
		return child
	}
	return 0
}

// ignoredReturnValuePureCalls are stdlib and coroutines calls that build a new Flow, Sequence or Stream
// without side effects, so discarding the result is a bug.
var ignoredReturnValuePureCalls = map[string]bool{
	"onEach": true, "onStart": true, "onCompletion": true, "onEmpty": true, "catch": true, "flowOn": true,
	"buffer": true, "conflate": true, "distinctUntilChanged": true, "debounce": true, "sample": true,
	"takeWhile": true, "dropWhile": true, "transform": true, "mapNotNull": true, "mapIndexed": true,
	"flatMapLatest": true, "flatMapConcat": true, "flatMapMerge": true, "combine": true, "merge": true,
	"scan": true, "runningFold": true, "runningReduce": true, "retry": true, "retryWhen": true, "withIndex": true,
	"cancellable": true, "asStateFlow": true, "asSharedFlow": true, "shareIn": true, "stateIn": true,
	"flowOf": true, "emptyFlow": true, "flow": true, "channelFlow": true, "callbackFlow": true,
	"MutableStateFlow": true, "MutableSharedFlow": true,
	"sequenceOf": true, "emptySequence": true, "sequence": true, "generateSequence": true, "windowed": true,
	"chunked": true, "zipWithNext": true, "stream": true, "parallelStream": true,
}

// ignoredReturnValueCollectionMutators are stdlib collection and queue mutators whose result is routinely
// ignored; they are never reported on type evidence alone.
var ignoredReturnValueCollectionMutators = map[string]bool{
	"remove": true, "removeAt": true, "removeFirst": true, "removeLast": true, "removeAll": true, "removeIf": true,
	"retainAll": true, "emit": true, "send": true, "add": true, "addAll": true, "addFirst": true, "addLast": true, "put": true, "putAll": true,
	"putIfAbsent": true, "set": true, "clear": true, "offer": true, "offerFirst": true, "offerLast": true,
	"poll": true, "pollFirst": true, "pollLast": true, "pop": true, "push": true, "getOrPut": true,
	"compute": true, "computeIfAbsent": true, "computeIfPresent": true, "replace": true,
	"replaceAll": true, "trySend": true, "tryEmit": true, "tryReceive": true,
}

// ignoredReturnValueCalleeIsKnownPure reports whether a call whose type matched a must-use type is a
// known side-effect-free builder. A project function that returns a Flow can register state eagerly
// (a cache entry, a collector), so only stdlib targets, or known names when the target is unresolved, count.
// ignoredReturnValueTestLibraryPrefixes are call-target prefixes of assertion libraries.
var ignoredReturnValueTestLibraryPrefixes = []string{
	"kotlin.test.", "org.junit.", "junit.framework.", "com.google.common.truth.", "org.assertj.",
	"org.hamcrest.", "app.cash.turbine.",
}

// ignoredReturnValueIsTestAssertion reports whether the call is an assertion-library function. The resolved
// target decides when known; otherwise the name does.
func ignoredReturnValueIsTestAssertion(oracleLookup oracle.Lookup, file *scanner.File, idx uint32, funcName string) bool {
	if oracleLookup != nil {
		if target := oracleLookupCallTargetFlat(oracleLookup, file, idx); target != "" {
			for _, prefix := range ignoredReturnValueTestLibraryPrefixes {
				if strings.HasPrefix(target, prefix) {
					return true
				}
			}
		}
	}
	return strings.HasPrefix(funcName, "assert") && len(funcName) > len("assert") && funcName[len("assert")] >= 'A' && funcName[len("assert")] <= 'Z'
}

func ignoredReturnValueCalleeIsKnownPure(oracleLookup oracle.Lookup, file *scanner.File, idx uint32, funcName string) bool {
	// A collection mutator returns the previous element or a success flag, and its call is the point of the statement.
	if ignoredReturnValueCollectionMutators[funcName] {
		return false
	}
	target := ""
	if oracleLookup != nil {
		target = oracleLookupCallTargetFlat(oracleLookup, file, idx)
	}
	if target != "" {
		// Invoking a function-typed property is a call on the user's own value, not a stdlib function.
		if strings.HasPrefix(target, "kotlin.Function") || strings.HasPrefix(target, "kotlin.coroutines.SuspendFunction") {
			return false
		}
		for _, prefix := range []string{"kotlin.", "kotlinx.coroutines.", "java.util."} {
			if strings.HasPrefix(target, prefix) {
				return true
			}
		}
		return false
	}
	return functionalOps[funcName] || ignoredReturnValuePureCalls[funcName]
}

// ignoredReturnValueChainJoinsAcrossLines reports a call chain whose receiver is a call followed by a call
// suffix on a later line. Kotlin ends the statement at that newline, so the parser tree does not match the code.
func ignoredReturnValueChainJoinsAcrossLines(file *scanner.File, idx uint32) bool {
	for node := idx; node != 0; node = file.FlatFirstChild(node) {
		switch file.FlatType(node) {
		case "call_expression":
			first := file.FlatFirstChild(node)
			if first == 0 {
				return false
			}
			if file.FlatType(first) == "call_expression" {
				for sib := file.FlatNextSib(first); sib != 0; sib = file.FlatNextSib(sib) {
					if file.FlatType(sib) == "call_suffix" &&
						bytes.IndexByte(file.Content[file.FlatEndByte(first):file.FlatStartByte(sib)], '\n') >= 0 {
						return true
					}
				}
			}
		case "navigation_expression":
		default:
			return false
		}
	}
	return false
}

// ignoredReturnValueTargetIsFunctionInvoke reports an `invoke` call the oracle resolved to a function type.
func ignoredReturnValueTargetIsFunctionInvoke(oracleLookup oracle.Lookup, file *scanner.File, idx uint32) bool {
	if oracleLookup == nil {
		return false
	}
	target := oracleLookupCallTargetFlat(oracleLookup, file, idx)
	return strings.HasPrefix(target, "kotlin.Function") || strings.HasPrefix(target, "kotlin.coroutines.SuspendFunction")
}
