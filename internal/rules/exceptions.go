package rules

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/experiment"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ExceptionRaisedInUnexpectedLocationRule detects throw inside equals/hashCode/toString/finalize.
type ExceptionRaisedInUnexpectedLocationRule struct {
	FlatDispatchBase
	BaseRule
	// MethodNames is the list of function names where throwing is treated
	// as a finding. Configurable via the `methodNames` YAML option.
	// Default: equals, hashCode, toString, finalize.
	MethodNames []string
}

// defaultExceptionRaisedInUnexpectedLocationMethods enumerates the default
// `methodNames` set. Used as the fallback when the configured list is empty
// (i.e. zero-value or explicitly cleared).
var defaultExceptionRaisedInUnexpectedLocationMethods = []string{
	"equals", "hashCode", "toString", "finalize",
}

// includesMethod reports whether name is one of the rule's configured
// method names, falling back to the default when the configured list is empty.
func (r *ExceptionRaisedInUnexpectedLocationRule) includesMethod(name string) bool {
	names := r.MethodNames
	if len(names) == 0 {
		names = defaultExceptionRaisedInUnexpectedLocationMethods
	}
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ExceptionRaisedInUnexpectedLocationRule) Confidence() float64 { return api.ConfidenceMedium }

// InstanceOfCheckForExceptionRule detects `is SomeException` inside catch blocks.
type InstanceOfCheckForExceptionRule struct {
	FlatDispatchBase
	BaseRule
}

var (
	isExceptionRe             = regexp.MustCompile(`\bis\s+\w*Exception\b`)
	javaInstanceOfExceptionRe = regexp.MustCompile(`\binstanceof\s+[\w.]*Exception\b`)
)

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *InstanceOfCheckForExceptionRule) Confidence() float64 { return api.ConfidenceMedium }

func isInsideWhenDispatchOnCatchVarFlat(file *scanner.File, isNode uint32, caughtVar string) bool {
	for p, ok := file.FlatParent(isNode); ok; p, ok = file.FlatParent(p) {
		t := file.FlatType(p)
		if t == "when_expression" {
			for i := 0; i < file.FlatChildCount(p); i++ {
				c := file.FlatChild(p, i)
				ctype := file.FlatType(c)
				if ctype == "when_subject" || ctype == "parenthesized_expression" ||
					ctype == "value_arguments" {
					text := strings.TrimSpace(file.FlatNodeText(c))
					text = strings.TrimPrefix(text, "(")
					text = strings.TrimSuffix(text, ")")
					text = strings.TrimSpace(text)
					if text == caughtVar {
						return true
					}
				}
			}
			return false
		}
		if t == "function_declaration" || t == "lambda_literal" || t == "source_file" {
			return false
		}
	}
	return false
}

// NotImplementedDeclarationRule detects TODO() calls.
type NotImplementedDeclarationRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *NotImplementedDeclarationRule) Confidence() float64 { return api.ConfidenceMedium }

// isKotlinTODOCall reports whether the call_expression at idx is a call to
// `kotlin.TODO()` — either the unqualified `TODO(...)` (kotlin's TODO is
// imported automatically from the `kotlin` package) or the fully-qualified
// `kotlin.TODO(...)`. Member calls like `foo.TODO()` or `Items.TODO()` on
// non-kotlin receivers are NOT kotlin.TODO and must not be flagged.
func isKotlinTODOCall(file *scanner.File, idx uint32) bool {
	if file == nil || file.FlatType(idx) != "call_expression" {
		return false
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			// Bare `TODO(...)` — kotlin.TODO is auto-imported from the
			// `kotlin` package and is the only realistic referent for an
			// unqualified TODO call at top-level expression position.
			return file.FlatNodeTextEquals(child, "TODO")
		case "navigation_expression":
			// Qualified call. Only `kotlin.TODO(...)` counts as kotlin.TODO.
			segments := flatNavigationChainIdentifiers(file, child)
			return len(segments) == 2 && segments[0] == "kotlin" && segments[1] == "TODO"
		}
	}
	return false
}

// RethrowCaughtExceptionRule detects catch { throw e } where e is the caught variable.
type RethrowCaughtExceptionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *RethrowCaughtExceptionRule) Confidence() float64 { return api.ConfidenceMedium }

// ReturnFromFinallyRule detects return statements inside finally blocks.
type ReturnFromFinallyRule struct {
	FlatDispatchBase
	BaseRule
	IgnoreLabeled bool
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ReturnFromFinallyRule) Confidence() float64 { return api.ConfidenceMedium }

// SwallowedExceptionRule detects catch blocks that either never use the exception
// variable or that throw a new exception without passing the original as the cause.
// Referencing only e.message or e.toString() (directly or via a variable) in a
// throw counts as swallowed.
type SwallowedExceptionRule struct {
	FlatDispatchBase
	BaseRule
	AllowedExceptionNameRegex *regexp.Regexp // exception names matching this are allowed to be swallowed
	IgnoredExceptionTypes     []string       // exception types that are allowed to be swallowed
	LoggingCountsAsHandling   bool
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *SwallowedExceptionRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *SwallowedExceptionRule) makeUnusedFindingFlat(ctx *api.Context, caughtVar string) {
	idx, file := ctx.Idx, ctx.File
	f := r.Finding(file, file.FlatRow(idx)+1, 1,
		fmt.Sprintf("Exception '%s' is caught but never used. Either log/handle it or rethrow.", caughtVar))
	endByte := int(file.FlatEndByte(idx))
	if endByte > 0 && file.Content[endByte-1] == '}' {
		catchLine := file.Lines[file.FlatRow(idx)]
		indent := ""
		for _, ch := range catchLine {
			if ch == ' ' || ch == '\t' {
				indent += string(ch)
			} else {
				break
			}
		}
		insertion := indent + "    throw " + caughtVar + "\n" + indent
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   endByte - 1,
			EndByte:     endByte,
			Replacement: insertion + "}",
		}
	}
	ctx.Emit(f)
}

func (r *SwallowedExceptionRule) checkSwallowedException(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if scanner.IsTestFile(file.Path) {
		return
	}
	if swallowedCatchSuppressesRule(file, idx) {
		return
	}
	caughtVar := extractCaughtVarNameFlat(file, idx)
	if caughtVar == "" || caughtVar == "_" {
		return
	}
	if r.AllowedExceptionNameRegex != nil && r.AllowedExceptionNameRegex.MatchString(caughtVar) {
		return
	}
	caughtType := extractCaughtTypeNameFlat(file, idx)
	if len(r.IgnoredExceptionTypes) > 0 && caughtType != "" {
		lowerType := strings.ToLower(caughtType)
		for _, ignored := range r.IgnoredExceptionTypes {
			if strings.Contains(lowerType, strings.ToLower(ignored)) {
				return
			}
		}
	}
	if isCatchPartOfTryExpressionFlat(file, idx) {
		return
	}
	if caughtType == "EOFException" {
		return
	}

	result := r.analyzeSwallowedCatchBody(ctx, idx, caughtVar)
	if result.handled {
		return
	}
	if result.indirectOnly || result.anyCaughtReference {
		ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Exception '%s' is caught but not meaningfully handled. Pass it directly to logging, handling, or rethrowing code.", caughtVar)))
		return
	}
	r.makeUnusedFindingFlat(ctx, caughtVar)
}

func swallowedCatchSuppressesRule(file *scanner.File, catchNode uint32) bool {
	if file == nil || catchNode == 0 {
		return false
	}
	text := file.FlatNodeText(catchNode)
	if !strings.Contains(text, "@Suppress") {
		return false
	}
	return strings.Contains(text, "\"SwallowedException\"") ||
		strings.Contains(text, "\"exceptions:SwallowedException\"") ||
		strings.Contains(text, "\"all\"")
}

type swallowedCatchResult struct {
	handled            bool
	indirectOnly       bool
	anyCaughtReference bool
}

func (r *SwallowedExceptionRule) analyzeSwallowedCatchBody(ctx *api.Context, catchNode uint32, caughtVar string) swallowedCatchResult {
	file := ctx.File
	body := swallowedCatchStatements(file, catchNode)
	if body == 0 || !swallowedHasNamedStatement(file, body) {
		return swallowedCatchResult{}
	}

	directAliases := map[string]bool{caughtVar: true}
	derivedAliases := map[string]bool{}
	swallowedWalkCatchBody(file, body, func(node uint32) bool {
		if file.FlatType(node) != "property_declaration" {
			return true
		}
		name := swallowedPropertyDeclarationName(file, node)
		if name == "" {
			return true
		}
		init := swallowedPropertyInitializer(file, node)
		switch swallowedExpressionAliasKind(file, init, directAliases, derivedAliases) {
		case swallowedAliasDirect:
			directAliases[name] = true
		case swallowedAliasDerived:
			derivedAliases[name] = true
		}
		return true
	})

	result := swallowedCatchResult{}
	swallowedWalkCatchBody(file, body, func(node uint32) bool {
		if result.handled {
			return false
		}
		if file.FlatType(node) == "simple_identifier" && swallowedIdentifierReferencesCaught(file, node, directAliases, derivedAliases) {
			result.anyCaughtReference = true
		}
		switch file.FlatType(node) {
		case "jump_expression":
			jumpResult := swallowedAnalyzeJump(file, node, directAliases, derivedAliases)
			if jumpResult.handled {
				result.handled = true
				return false
			}
			result.indirectOnly = result.indirectOnly || jumpResult.indirectOnly
		case "call_expression":
			callResult := r.swallowedAnalyzeCall(ctx, catchNode, node, directAliases, derivedAliases)
			if callResult.handled {
				result.handled = true
				return false
			}
			result.indirectOnly = result.indirectOnly || callResult.indirectOnly
		case "assignment", "augmented_assignment":
			if swallowedSubtreeReferencesCaught(file, node, directAliases, derivedAliases) {
				result.handled = true
				return false
			}
		case "return_expression":
			result.handled = true
			return false
		}
		return true
	})
	return result
}

type swallowedEvidence struct {
	handled      bool
	indirectOnly bool
}

type swallowedAliasKind uint8

const (
	swallowedAliasNone swallowedAliasKind = iota
	swallowedAliasDirect
	swallowedAliasDerived
)

func swallowedCatchStatements(file *scanner.File, catchNode uint32) uint32 {
	for child := file.FlatFirstChild(catchNode); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "statements" {
			return child
		}
	}
	return 0
}

func swallowedHasNamedStatement(file *scanner.File, statements uint32) bool {
	for child := file.FlatFirstChild(statements); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			return true
		}
	}
	return false
}

func swallowedWalkCatchBody(file *scanner.File, root uint32, fn func(uint32) bool) {
	var walk func(uint32) bool
	walk = func(node uint32) bool {
		if node != root {
			switch file.FlatType(node) {
			case "lambda_literal", "function_declaration", "class_declaration", "object_declaration":
				return true
			}
		}
		if !fn(node) {
			return false
		}
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if !walk(child) {
				return false
			}
		}
		return true
	}
	walk(root)
}

func swallowedPropertyDeclarationName(file *scanner.File, prop uint32) string {
	if decl, ok := file.FlatFindChild(prop, "variable_declaration"); ok {
		if ident, ok := file.FlatFindChild(decl, "simple_identifier"); ok {
			return file.FlatNodeString(ident, nil)
		}
	}
	return ""
}

func swallowedPropertyInitializer(file *scanner.File, prop uint32) uint32 {
	afterEquals := false
	for child := file.FlatFirstChild(prop); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			afterEquals = true
			continue
		}
		if afterEquals && file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

func swallowedExpressionAliasKind(file *scanner.File, expr uint32, directAliases, derivedAliases map[string]bool) swallowedAliasKind {
	expr = swallowedUnwrapExpression(file, expr)
	if expr == 0 {
		return swallowedAliasNone
	}
	if file.FlatType(expr) == "simple_identifier" {
		name := file.FlatNodeString(expr, nil)
		if directAliases[name] {
			return swallowedAliasDirect
		}
		if derivedAliases[name] {
			return swallowedAliasDerived
		}
	}
	if swallowedSubtreeReferencesCaught(file, expr, directAliases, derivedAliases) {
		return swallowedAliasDerived
	}
	return swallowedAliasNone
}

func swallowedUnwrapExpression(file *scanner.File, expr uint32) uint32 {
	for expr != 0 {
		switch file.FlatType(expr) {
		case "parenthesized_expression":
			if file.FlatNamedChildCount(expr) != 1 {
				return expr
			}
			expr = file.FlatNamedChild(expr, 0)
		default:
			return expr
		}
	}
	return 0
}

func swallowedAnalyzeJump(file *scanner.File, node uint32, directAliases, derivedAliases map[string]bool) swallowedEvidence {
	switch {
	case swallowedIsThrowExpression(file, node):
		return swallowedAnalyzeThrow(file, node, directAliases, derivedAliases)
	case swallowedIsReturnExpression(file, node):
		return swallowedAnalyzeReturn(file, node, directAliases, derivedAliases)
	default:
		return swallowedEvidence{}
	}
}

func swallowedAnalyzeThrow(file *scanner.File, node uint32, directAliases, derivedAliases map[string]bool) swallowedEvidence {
	expr := swallowedJumpExpressionValue(file, node)
	if swallowedExpressionIsDirectAlias(file, expr, directAliases) {
		return swallowedEvidence{handled: true}
	}
	if file.FlatType(expr) == "call_expression" {
		args := flatCallKeyArguments(file, expr)
		argUse := swallowedAnalyzeArguments(file, args, directAliases, derivedAliases)
		if argUse.handled {
			return swallowedEvidence{handled: true}
		}
		if argUse.indirectOnly {
			return swallowedEvidence{indirectOnly: true}
		}
	}
	if swallowedExpressionAliasKind(file, expr, directAliases, derivedAliases) == swallowedAliasDerived {
		return swallowedEvidence{indirectOnly: true}
	}
	return swallowedEvidence{}
}

func swallowedAnalyzeReturn(file *scanner.File, node uint32, directAliases, derivedAliases map[string]bool) swallowedEvidence {
	expr := swallowedJumpExpressionValue(file, node)
	if swallowedExpressionIsDirectAlias(file, expr, directAliases) {
		return swallowedEvidence{handled: true}
	}
	if file.FlatType(expr) == "call_expression" {
		args := flatCallKeyArguments(file, expr)
		argUse := swallowedAnalyzeArguments(file, args, directAliases, derivedAliases)
		if argUse.handled || argUse.indirectOnly {
			return swallowedEvidence{handled: true}
		}
	}
	if swallowedExpressionAliasKind(file, expr, directAliases, derivedAliases) != swallowedAliasNone {
		return swallowedEvidence{handled: true}
	}
	return swallowedEvidence{}
}

func swallowedIsThrowExpression(file *scanner.File, node uint32) bool {
	if file.FlatType(node) != "jump_expression" {
		return false
	}
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		return file.FlatType(child) == "throw"
	}
	return false
}

func swallowedIsReturnExpression(file *scanner.File, node uint32) bool {
	if file.FlatType(node) != "jump_expression" {
		return false
	}
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		return file.FlatType(child) == "return"
	}
	return false
}

func swallowedJumpExpressionValue(file *scanner.File, node uint32) uint32 {
	seenKeyword := false
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		if !seenKeyword {
			seenKeyword = file.FlatType(child) == "throw" || file.FlatType(child) == "return"
			continue
		}
		if file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

func (r *SwallowedExceptionRule) swallowedAnalyzeCall(ctx *api.Context, catchNode, node uint32, directAliases, derivedAliases map[string]bool) swallowedEvidence {
	file := ctx.File
	kind := swallowedClassifyCall(ctx, catchNode, node)
	if kind == swallowedCallUnknown {
		return swallowedEvidence{}
	}
	args := flatCallKeyArguments(file, node)
	argUse := swallowedAnalyzeArguments(file, args, directAliases, derivedAliases)
	if kind == swallowedCallUI {
		if argUse.handled || argUse.indirectOnly {
			return swallowedEvidence{handled: true}
		}
		return swallowedEvidence{}
	}
	if kind == swallowedCallLogging {
		if !r.LoggingCountsAsHandling {
			return swallowedEvidence{}
		}
		if argUse.handled || argUse.indirectOnly || swallowedLoggingCallMentionsCaughtText(file, node, directAliases, derivedAliases) {
			return swallowedEvidence{handled: true}
		}
		return swallowedEvidence{}
	}
	if kind == swallowedCallLocalHandler {
		if argUse.handled {
			return swallowedEvidence{handled: true}
		}
		return swallowedEvidence{}
	}
	return argUse
}

type swallowedCallKind uint8

const (
	swallowedCallUnknown swallowedCallKind = iota
	swallowedCallLogging
	swallowedCallUI
	swallowedCallLocalHandler
)

func swallowedClassifyCall(ctx *api.Context, catchNode, node uint32) swallowedCallKind {
	file := ctx.File
	callee, receivers := swallowedCallTarget(file, node)
	if callee == "" {
		return swallowedCallUnknown
	}
	if swallowedIsQualifiedLoggingCall(file, callee, receivers) || swallowedReceiverTypeIsLogging(file, node) {
		return swallowedCallLogging
	}
	if swallowedIsQualifiedUICall(file, callee, receivers) || swallowedReceiverTypeIsUI(file, node) {
		return swallowedCallUI
	}
	if swallowedIsHandlerName(callee) &&
		(len(receivers) > 0 ||
			swallowedSameOwnerLocalFunctionExists(file, catchNode, callee) ||
			swallowedSameOwnerCallablePropertyExists(file, catchNode, callee)) {
		return swallowedCallLocalHandler
	}
	return swallowedCallUnknown
}

func swallowedCallTarget(file *scanner.File, node uint32) (string, []string) {
	core := node
	if flatCallExpressionName(file, core) == "" {
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "call_expression" {
				core = child
				break
			}
		}
	}
	callee := flatCallExpressionName(file, core)
	if callee == "" {
		return "", nil
	}
	navExpr, _ := flatCallExpressionParts(file, core)
	if navExpr == 0 {
		return callee, nil
	}
	parts := swallowedNavigationIdentifiers(file, navExpr)
	if len(parts) <= 1 {
		return callee, nil
	}
	return callee, parts[:len(parts)-1]
}

func swallowedNavigationIdentifiers(file *scanner.File, navExpr uint32) []string {
	var parts []string
	for child := file.FlatFirstChild(navExpr); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			parts = append(parts, file.FlatNodeString(child, nil))
		case "navigation_suffix":
			for gc := file.FlatFirstChild(child); gc != 0; gc = file.FlatNextSib(gc) {
				if file.FlatType(gc) == "simple_identifier" {
					parts = append(parts, file.FlatNodeString(gc, nil))
				}
			}
		case "navigation_expression":
			parts = append(parts, swallowedNavigationIdentifiers(file, child)...)
		}
	}
	return parts
}

func swallowedIsQualifiedLoggingCall(file *scanner.File, callee string, receivers []string) bool {
	if !swallowedLoggingCallee(callee) {
		return false
	}
	timberImports := timberImportsForFile(file)
	if len(receivers) == 0 {
		return swallowedUnqualifiedLoggingCallee(callee) ||
			(swallowedTimberLoggingCallee(callee) && (timberImports.members[callee] || timberImports.wildcard))
	}
	path := strings.Join(receivers, ".")
	for _, receiver := range receivers {
		if swallowedGenericLogReceiver(receiver) && swallowedCompactLoggingCallee(callee) {
			return true
		}
		if swallowedLoggingReceiverName(receiver) {
			return true
		}
		if receiver == "Log" && swallowedFileImportsKnownLogReceiver(file) {
			return true
		}
		if receiver == "Timber" && fileImportsFQN(file, "timber.log.Timber") {
			return true
		}
		if timberImports.receivers[receiver] {
			return true
		}
	}
	switch path {
	case "ZLog":
		return true
	case "android.util.Log", "timber.log.Timber":
		return true
	case "Log":
		if fileImportsFQN(file, "android.util.Log") {
			return true
		}
		_, aliases := buildLoggerImportsFromAST(file)
		_, ok := aliases[path]
		return ok
	case "Timber":
		return fileImportsFQN(file, "timber.log.Timber")
	default:
		_, aliases := buildLoggerImportsFromAST(file)
		_, ok := aliases[path]
		return ok
	}
}

func swallowedUnqualifiedLoggingCallee(callee string) bool {
	switch callee {
	case "println", "debug", "trace", "info", "warn", "warning", "severe",
		"log", "logError", "logWarning", "logWarn", "logInfo", "logDebug",
		"logTrace", "logException", "logThrowable", "logDatadogException",
		"recordException", "trackError":
		return true
	default:
		return false
	}
}

func swallowedTimberLoggingCallee(callee string) bool {
	switch callee {
	case "v", "d", "i", "w", "e", "wtf", "log":
		return true
	default:
		return false
	}
}

func swallowedFileImportsKnownLogReceiver(file *scanner.File) bool {
	if fileImportsFQN(file, "android.util.Log") {
		return true
	}
	_, aliases := buildLoggerImportsFromAST(file)
	_, ok := aliases["Log"]
	return ok
}

func swallowedReceiverTypeIsLogging(file *scanner.File, call uint32) bool {
	receiver := flatReceiverNameFromCall(file, call)
	return swallowedReceiverHasKnownType(file, call, receiver, swallowedKnownLoggerType)
}

func swallowedLoggingCallee(callee string) bool {
	switch callee {
	case "v", "d", "i", "w", "e", "wtf", "println",
		"trace", "debug", "info", "warn", "warning", "severe",
		"error", "log", "logError", "logWarning",
		"logWarn", "logInfo", "logDebug", "logTrace",
		"logException", "logThrowable", "logDatadogException",
		"recordException", "trackError":
		return true
	default:
		return false
	}
}

func swallowedCompactLoggingCallee(callee string) bool {
	switch callee {
	case "w", "e", "i", "d", "v", "wtf":
		return true
	default:
		return false
	}
}

func swallowedGenericLogReceiver(receiver string) bool {
	return receiver == "Timber" || strings.HasSuffix(receiver, "Log")
}

func swallowedLoggingReceiverName(receiver string) bool {
	switch receiver {
	case "Log":
		return false
	case "ZLog":
		return true
	}
	lower := strings.ToLower(receiver)
	return strings.Contains(lower, "logger") ||
		strings.Contains(lower, "telemetry") ||
		strings.Contains(lower, "errorreporter") ||
		strings.Contains(lower, "crash") ||
		strings.Contains(lower, "datadog")
}

func swallowedIsQualifiedUICall(file *scanner.File, callee string, receivers []string) bool {
	uiReceivers := map[string]bool{
		"Toast": true, "Snackbar": true, "AlertDialog": true,
		"MaterialAlertDialog": true, "MaterialAlertDialogBuilder": true,
	}
	if len(receivers) > 0 {
		for _, receiver := range receivers {
			if uiReceivers[receiver] && swallowedUIReceiverHasImportEvidence(file, receiver) {
				return true
			}
		}
	}
	switch callee {
	case "showError", "showDialog", "showErrorDialog":
		return len(receivers) > 0
	default:
		return false
	}
}

func swallowedUIReceiverHasImportEvidence(file *scanner.File, receiver string) bool {
	switch receiver {
	case "Toast":
		return fileImportsFQN(file, "android.widget.Toast")
	case "Snackbar":
		return fileImportsFQN(file, "com.google.android.material.snackbar.Snackbar")
	case "AlertDialog":
		return fileImportsFQN(file, "android.app.AlertDialog")
	case "MaterialAlertDialog", "MaterialAlertDialogBuilder":
		return fileImportsFQN(file, "com.google.android.material.dialog.MaterialAlertDialogBuilder")
	default:
		return strings.Contains(receiver, ".")
	}
}

func swallowedReceiverTypeIsUI(file *scanner.File, call uint32) bool {
	receiver := flatReceiverNameFromCall(file, call)
	return swallowedReceiverHasKnownType(file, call, receiver, swallowedKnownUIType)
}

func swallowedReceiverHasKnownType(file *scanner.File, call uint32, receiver string, match func(*scanner.File, string) bool) bool {
	if file == nil || call == 0 || receiver == "" {
		return false
	}
	for parent, ok := file.FlatParent(call); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "function_declaration":
			if swallowedReceiverHasKnownTypeInParameters(file, parent, receiver, match) {
				return true
			}
		case "statements", "class_body", "source_file":
			if swallowedReceiverHasKnownTypeInProperties(file, parent, receiver, match) {
				return true
			}
		case "class_declaration":
			if swallowedReceiverHasKnownTypeInClassParameters(file, parent, receiver, match) {
				return true
			}
		}
	}
	return false
}

func swallowedReceiverHasKnownTypeInParameters(file *scanner.File, function uint32, receiver string, match func(*scanner.File, string) bool) bool {
	params, _ := file.FlatFindChild(function, "function_value_parameters")
	if params == 0 {
		return false
	}
	for param := file.FlatFirstChild(params); param != 0; param = file.FlatNextSib(param) {
		if file.FlatType(param) == "parameter" && extractIdentifierFlat(file, param) == receiver && match(file, explicitTypeTextFlat(file, param)) {
			return true
		}
	}
	return false
}

func swallowedReceiverHasKnownTypeInProperties(file *scanner.File, container uint32, receiver string, match func(*scanner.File, string) bool) bool {
	for child := file.FlatFirstChild(container); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "property_declaration" && extractIdentifierFlat(file, child) == receiver && match(file, explicitTypeTextFlat(file, child)) {
			return true
		}
	}
	return false
}

func swallowedReceiverHasKnownTypeInClassParameters(file *scanner.File, classDecl uint32, receiver string, match func(*scanner.File, string) bool) bool {
	ctor, _ := file.FlatFindChild(classDecl, "primary_constructor")
	if ctor == 0 {
		return false
	}
	for i := 0; i < file.FlatNamedChildCount(ctor); i++ {
		param := file.FlatNamedChild(ctor, i)
		if param == 0 || file.FlatType(param) != "class_parameter" {
			continue
		}
		if extractIdentifierFlat(file, param) == receiver && classParameterDefinesPropertyFlat(file, param) && match(file, explicitTypeTextFlat(file, param)) {
			return true
		}
	}
	return false
}

func swallowedKnownLoggerType(file *scanner.File, text string) bool {
	return swallowedTypeTextMatches(file, text,
		"android.util.Log",
		"timber.log.Timber",
		"org.slf4j.Logger",
		"java.util.logging.Logger",
		"ch.qos.logback.classic.Logger",
		"org.apache.logging.log4j.Logger",
		"mu.KLogger",
		"io.github.oshai.kotlinlogging.KLogger",
	)
}

func swallowedKnownUIType(file *scanner.File, text string) bool {
	return swallowedTypeTextMatches(file, text,
		"android.widget.Toast",
		"com.google.android.material.snackbar.Snackbar",
		"android.app.AlertDialog",
		"com.google.android.material.dialog.MaterialAlertDialogBuilder",
	)
}

func swallowedTypeTextMatches(file *scanner.File, text string, fqns ...string) bool {
	text = compactConditionText(strings.TrimSuffix(strings.TrimSpace(text), "?"))
	if text == "" {
		return false
	}
	for _, fqn := range fqns {
		if text == fqn {
			return true
		}
		if simple := fqn[strings.LastIndex(fqn, ".")+1:]; text == simple && fileImportsFQN(file, fqn) {
			return true
		}
	}
	_, aliases := buildLoggerImportsFromAST(file)
	if fqn, ok := aliases[text]; ok {
		for _, want := range fqns {
			if fqn == want {
				return true
			}
		}
	}
	return false
}

func swallowedIsHandlerName(name string) bool {
	switch name {
	case "toastOn", "showError", "showErrorDialog", "handleError",
		"reportError", "recoverFrom", "onError", "fallback", "notifyError",
		"logError", "logWarning", "logWarn", "onLoadFailed", "onFailure", "onException":
		return true
	default:
		return false
	}
}

func swallowedSameOwnerLocalFunctionExists(file *scanner.File, catchNode uint32, name string) bool {
	callFunc, _ := flatEnclosingAncestor(file, catchNode, "function_declaration")
	callClass, _ := flatEnclosingAncestor(file, catchNode, "class_declaration", "object_declaration")
	found := false
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		if found || fn == callFunc {
			return
		}
		if swallowedFunctionName(file, fn) != name {
			return
		}
		if callFunc != 0 {
			if ownerFunc, ok := flatEnclosingAncestor(file, fn, "function_declaration"); ok && ownerFunc == callFunc {
				found = true
				return
			}
		}
		if callClass != 0 {
			if ownerClass, ok := flatEnclosingAncestor(file, fn, "class_declaration", "object_declaration"); ok && ownerClass == callClass {
				found = true
				return
			}
		}
		if callClass == 0 {
			if _, ok := flatEnclosingAncestor(file, fn, "class_declaration", "object_declaration"); !ok {
				found = true
			}
		}
	})
	return found
}

func swallowedSameOwnerCallablePropertyExists(file *scanner.File, catchNode uint32, name string) bool {
	for parent, ok := file.FlatParent(catchNode); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "function_declaration":
			if swallowedCallableParameterExists(file, parent, name) {
				return true
			}
		case "class_declaration", "object_declaration":
			if swallowedCallableClassParameterExists(file, parent, name) ||
				swallowedCallablePropertyExistsInClass(file, parent, name) {
				return true
			}
		case "source_file":
			return false
		}
	}
	return false
}

func swallowedCallableParameterExists(file *scanner.File, function uint32, name string) bool {
	params, _ := file.FlatFindChild(function, "function_value_parameters")
	if params == 0 {
		return false
	}
	for param := file.FlatFirstChild(params); param != 0; param = file.FlatNextSib(param) {
		if file.FlatType(param) == "parameter" &&
			extractIdentifierFlat(file, param) == name &&
			swallowedTypeTextLooksCallable(explicitTypeTextFlat(file, param)) {
			return true
		}
	}
	return false
}

func swallowedCallableClassParameterExists(file *scanner.File, classDecl uint32, name string) bool {
	ctor, _ := file.FlatFindChild(classDecl, "primary_constructor")
	if ctor == 0 {
		return false
	}
	for i := 0; i < file.FlatNamedChildCount(ctor); i++ {
		param := file.FlatNamedChild(ctor, i)
		if param == 0 || file.FlatType(param) != "class_parameter" {
			continue
		}
		if extractIdentifierFlat(file, param) == name &&
			classParameterDefinesPropertyFlat(file, param) &&
			swallowedTypeTextLooksCallable(explicitTypeTextFlat(file, param)) {
			return true
		}
	}
	return false
}

func swallowedCallablePropertyExistsInClass(file *scanner.File, classDecl uint32, name string) bool {
	body, _ := file.FlatFindChild(classDecl, "class_body")
	if body == 0 {
		return false
	}
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "property_declaration" &&
			extractIdentifierFlat(file, child) == name &&
			swallowedTypeTextLooksCallable(explicitTypeTextFlat(file, child)) {
			return true
		}
	}
	return false
}

func swallowedTypeTextLooksCallable(text string) bool {
	text = strings.TrimSpace(text)
	return strings.Contains(text, "->") || strings.HasPrefix(text, "Function")
}

func swallowedFunctionName(file *scanner.File, fn uint32) string {
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "simple_identifier" {
			return file.FlatNodeString(child, nil)
		}
	}
	return ""
}

func swallowedAnalyzeArguments(file *scanner.File, args uint32, directAliases, derivedAliases map[string]bool) swallowedEvidence {
	if args == 0 {
		return swallowedEvidence{}
	}
	indirect := false
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		expr := flatValueArgumentExpression(file, arg)
		if swallowedExpressionIsDirectAlias(file, expr, directAliases) {
			return swallowedEvidence{handled: true}
		}
		if swallowedExpressionAliasKind(file, expr, directAliases, derivedAliases) == swallowedAliasDerived {
			indirect = true
		}
	}
	return swallowedEvidence{indirectOnly: indirect}
}

func swallowedLoggingCallMentionsCaughtText(file *scanner.File, call uint32, directAliases, derivedAliases map[string]bool) bool {
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return false
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		text := file.FlatNodeText(arg)
		for name := range directAliases {
			if swallowedInterpolatedNameText(text, name) {
				return true
			}
		}
		for name := range derivedAliases {
			if swallowedInterpolatedNameText(text, name) {
				return true
			}
		}
	}
	return false
}

func swallowedInterpolatedNameText(text, name string) bool {
	if name == "" {
		return false
	}
	return strings.Contains(text, "$"+name) || strings.Contains(text, "${"+name)
}

func swallowedExpressionIsDirectAlias(file *scanner.File, expr uint32, directAliases map[string]bool) bool {
	expr = swallowedUnwrapExpression(file, expr)
	if expr == 0 || file.FlatType(expr) != "simple_identifier" {
		return false
	}
	return directAliases[file.FlatNodeString(expr, nil)] && swallowedIsReferenceIdentifier(file, expr)
}

func swallowedSubtreeReferencesCaught(file *scanner.File, root uint32, directAliases, derivedAliases map[string]bool) bool {
	found := false
	swallowedWalkCatchBody(file, root, func(node uint32) bool {
		if file.FlatType(node) == "simple_identifier" && swallowedIdentifierReferencesCaught(file, node, directAliases, derivedAliases) {
			found = true
			return false
		}
		return true
	})
	return found
}

func swallowedIdentifierReferencesCaught(file *scanner.File, ident uint32, directAliases, derivedAliases map[string]bool) bool {
	if !swallowedIsReferenceIdentifier(file, ident) {
		return false
	}
	name := file.FlatNodeString(ident, nil)
	return directAliases[name] || derivedAliases[name]
}

func swallowedIsReferenceIdentifier(file *scanner.File, ident uint32) bool {
	parent, ok := file.FlatParent(ident)
	if !ok {
		return true
	}
	switch file.FlatType(parent) {
	case "variable_declaration", "function_declaration", "type_identifier",
		"user_type", "value_argument_label", "navigation_suffix":
		return false
	case "value_argument":
		if next, ok := file.FlatNextSibling(ident); ok && file.FlatType(next) == "=" {
			return false
		}
	case "call_expression":
		if first := file.FlatFirstChild(parent); first == ident {
			return false
		}
	}
	return true
}

// ThrowingExceptionFromFinallyRule detects throw inside finally blocks.
type ThrowingExceptionFromFinallyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ThrowingExceptionFromFinallyRule) Confidence() float64 { return api.ConfidenceMedium }

// ThrowingExceptionsWithoutMessageOrCauseRule detects throw Exception() with no args.
// Uses tree-sitter dispatch to find call_expression nodes, then checks if the parent
// is a throw (jump_expression) and the exception type is in the configured allowlist.
type ThrowingExceptionsWithoutMessageOrCauseRule struct {
	FlatDispatchBase
	BaseRule
	Exceptions    []string
	allowlistOnce sync.Once
	allowlist     map[string]bool
}

// Default exception types that should have a message.
var defaultExceptionsRequiringMessage = map[string]bool{
	"ArrayIndexOutOfBoundsException": true,
	"Exception":                      true,
	"IllegalArgumentException":       true,
	"IllegalMonitorStateException":   true,
	"IllegalStateException":          true,
	"IndexOutOfBoundsException":      true,
	"NullPointerException":           true,
	"RuntimeException":               true,
	"Throwable":                      true,
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ThrowingExceptionsWithoutMessageOrCauseRule) Confidence() float64 {
	return api.ConfidenceMedium
}

func (r *ThrowingExceptionsWithoutMessageOrCauseRule) exceptionAllowlist() map[string]bool {
	if !experiment.Enabled("exceptions-allowlist-cache") || len(r.Exceptions) == 0 {
		if len(r.Exceptions) == 0 {
			return defaultExceptionsRequiringMessage
		}
		set := make(map[string]bool, len(r.Exceptions))
		for _, e := range r.Exceptions {
			set[e] = true
		}
		return set
	}
	r.allowlistOnce.Do(func() {
		r.allowlist = make(map[string]bool, len(r.Exceptions))
		for _, e := range r.Exceptions {
			r.allowlist[e] = true
		}
	})
	return r.allowlist
}

func throwingExceptionArgCountFlat(file *scanner.File, idx uint32) int {
	if !experiment.Enabled("exceptions-throw-fastpath") {
		callSuffix, _ := file.FlatFindChild(idx, "call_suffix")
		if callSuffix == 0 {
			return -1
		}
		valueArgs, _ := file.FlatFindChild(callSuffix, "value_arguments")
		if valueArgs == 0 {
			return -1
		}
		argCount := 0
		for i := 0; i < file.FlatChildCount(valueArgs); i++ {
			if file.FlatType(file.FlatChild(valueArgs, i)) == "value_argument" {
				argCount++
			}
		}
		return argCount
	}
	callSuffix, _ := file.FlatFindChild(idx, "call_suffix")
	if callSuffix == 0 {
		return -1
	}
	argList, _ := file.FlatFindChild(callSuffix, "value_arguments")
	if argList == 0 {
		return -1
	}
	return file.FlatNamedChildCount(argList)
}

// ThrowingNewInstanceOfSameExceptionRule detects catch(e: X) { throw X(e) }.
type ThrowingNewInstanceOfSameExceptionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ThrowingNewInstanceOfSameExceptionRule) Confidence() float64 { return api.ConfidenceMedium }

// ThrowingExceptionInMainRule detects throw in main function.
type ThrowingExceptionInMainRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ThrowingExceptionInMainRule) Confidence() float64 { return api.ConfidenceMedium }

// ErrorUsageWithThrowableRule detects error(throwable) calls.
type ErrorUsageWithThrowableRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ErrorUsageWithThrowableRule) Confidence() float64 { return api.ConfidenceMedium }

func errorUsageWithThrowableArgument(ctx *api.Context, idx uint32) (string, bool) {
	if ctx.File == nil || !errorUsageWithThrowableDirectErrorCall(ctx.File, idx) {
		return "", false
	}
	args := errorUsageWithThrowableValueArguments(ctx.File, idx)
	if args == 0 {
		return "", false
	}
	firstArg := uint32(0)
	for arg := ctx.File.FlatFirstChild(args); arg != 0; arg = ctx.File.FlatNextSib(arg) {
		if ctx.File.FlatType(arg) == "value_argument" {
			firstArg = arg
			break
		}
	}
	if firstArg == 0 {
		return "", false
	}
	expr := flatValueArgumentExpression(ctx.File, firstArg)
	if expr == 0 {
		return "", false
	}
	argText := strings.TrimSpace(ctx.File.FlatNodeText(expr))
	if argText == "" || strings.HasPrefix(argText, "\"") {
		return "", false
	}
	if ctx.Resolver != nil {
		typ := ctx.Resolver.ResolveFlatNode(expr, ctx.File)
		if typ != nil && typ.Kind != typeinfer.TypeUnknown {
			return argText, errorUsageWithThrowableTypeMatches(typ)
		}
	}
	if typ, _, ok := flatNullOrEmptyExplicitReceiverType(ctx.File, expr); ok {
		return argText, errorUsageWithThrowableTypeTextMatches(typ)
	}
	return argText, errorUsageWithThrowableTextHeuristic(argText)
}

func errorUsageWithThrowableDirectErrorCall(file *scanner.File, idx uint32) bool {
	if file == nil || file.FlatType(idx) != "call_expression" {
		return false
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		return file.FlatType(child) == "simple_identifier" && file.FlatNodeTextEquals(child, "error")
	}
	return false
}

func errorUsageWithThrowableValueArguments(file *scanner.File, idx uint32) uint32 {
	_, args := flatCallExpressionParts(file, idx)
	return args
}

func errorUsageWithThrowableTypeMatches(typ *typeinfer.ResolvedType) bool {
	if typ == nil {
		return false
	}
	for _, name := range []string{typ.Name, typ.FQN} {
		if name == "Throwable" || name == "kotlin.Throwable" || name == "java.lang.Throwable" ||
			typeinfer.IsSubtypeOfException(name, "Throwable") ||
			typeinfer.IsSubtypeOfException(name, "kotlin.Throwable") ||
			typeinfer.IsSubtypeOfException(name, "java.lang.Throwable") {
			return true
		}
	}
	return typ.IsSubtypeOf("Throwable") || typ.IsSubtypeOf("kotlin.Throwable") || typ.IsSubtypeOf("java.lang.Throwable")
}

func errorUsageWithThrowableTypeTextMatches(typ string) bool {
	typ = strings.TrimSpace(typ)
	typ = strings.TrimSuffix(typ, "?")
	if idx := strings.Index(typ, "<"); idx >= 0 {
		typ = typ[:idx]
	}
	if idx := strings.LastIndex(typ, "."); idx >= 0 {
		typ = typ[idx+1:]
	}
	return typ == "Throwable" || typeinfer.IsSubtypeOfException(typ, "Throwable")
}

func errorUsageWithThrowableTextHeuristic(argText string) bool {
	lower := strings.ToLower(strings.TrimSpace(argText))
	return strings.Contains(lower, "exception") || strings.Contains(lower, "throwable") ||
		strings.Contains(lower, "error") || lower == "e" || lower == "ex" || lower == "err" || lower == "t"
}

// ObjectExtendsThrowableRule detects object : Exception/Throwable/Error.
type ObjectExtendsThrowableRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — relies on
// resolver to determine supertypes; falls back to name-based heuristics on
// the `Throwable` identifier. Classified per roadmap/17.
func (r *ObjectExtendsThrowableRule) Confidence() float64 { return api.ConfidenceMedium }

var throwableBaseTypes = []string{"Throwable", "Exception", "Error", "RuntimeException",
	"IllegalStateException", "IllegalArgumentException", "IOException",
	"UnsupportedOperationException"}

func walkThrowExpressionsFlat(file *scanner.File, idx uint32, fn func(throwNode uint32)) {
	file.FlatWalkNodes(idx, "jump_expression", func(n uint32) {
		text := file.FlatNodeText(n)
		if strings.HasPrefix(text, "throw ") || text == "throw" {
			fn(n)
		}
	})
	file.FlatWalkNodes(idx, "throw_statement", fn)
}

// throwExpressionTargetFlat returns the call_expression (Kotlin) or
// object_creation_expression (Java) used as the thrown instance, or 0
// if the throw is bare (`throw` with no operand) or rethrows a simple
// identifier (`throw e`).
func throwExpressionTargetFlat(file *scanner.File, throwNode uint32) uint32 {
	for child := file.FlatFirstChild(throwNode); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "call_expression", "object_creation_expression":
			return child
		}
	}
	return 0
}

// throwTargetTypeNameFlat returns the last identifier of the type
// constructed by a throw target — `IOException` for both
// `IOException(msg)` and `java.io.IOException(msg)`.
func throwTargetTypeNameFlat(file *scanner.File, target uint32) string {
	switch file.FlatType(target) {
	case "call_expression":
		return flatCallExpressionName(file, target)
	case "object_creation_expression":
		for child := file.FlatFirstChild(target); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "type_identifier":
				return file.FlatNodeText(child)
			case "scoped_type_identifier", "scoped_identifier":
				if name := flatTypeLastIdentifier(file, child); name != "" {
					return name
				}
			}
		}
	}
	return ""
}

// throwTargetArgUsageFlat reports the number of positional arguments
// in the throw target and whether any of them references varName as
// an identifier. String literals and comments are skipped because
// they are not identifier AST nodes.
func throwTargetArgUsageFlat(file *scanner.File, target uint32, varName string) (argCount int, hasVar bool) {
	args := throwTargetArgsFlat(file, target)
	if args == 0 {
		return 0, false
	}
	for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		argCount++
		if hasVar {
			continue
		}
		file.FlatWalkAllNodes(child, func(id uint32) {
			if hasVar {
				return
			}
			switch file.FlatType(id) {
			case "simple_identifier", "identifier":
				if file.FlatNodeTextEquals(id, varName) {
					hasVar = true
				}
			}
		})
	}
	return argCount, hasVar
}

func throwTargetArgsFlat(file *scanner.File, target uint32) uint32 {
	switch file.FlatType(target) {
	case "call_expression":
		return flatCallKeyArguments(file, target)
	case "object_creation_expression":
		for child := file.FlatFirstChild(target); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "argument_list" {
				return child
			}
		}
	}
	return 0
}

func javaCatchOnlyRethrowsVar(file *scanner.File, catchNode uint32, varName string) bool {
	block, ok := file.FlatFindChild(catchNode, "block")
	if !ok || block == 0 {
		return false
	}
	statementCount := 0
	rethrows := false
	for child := file.FlatFirstChild(block); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "{", "}", "line_comment", "block_comment", "comment":
			continue
		case "throw_statement":
			statementCount++
			rethrows = strings.TrimSpace(file.FlatNodeText(child)) == "throw "+varName+";"
		default:
			if file.FlatIsNamed(child) {
				statementCount++
			}
		}
	}
	return statementCount == 1 && rethrows
}
