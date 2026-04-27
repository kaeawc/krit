package rules

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/experiment"
	"github.com/kaeawc/krit/internal/oracle"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ExceptionRaisedInUnexpectedLocationRule detects throw inside equals/hashCode/toString/finalize.
type ExceptionRaisedInUnexpectedLocationRule struct {
	FlatDispatchBase
	BaseRule
	MethodNames []string
}

var unexpectedThrowFunctions = map[string]bool{
	"equals":   true,
	"hashCode": true,
	"toString": true,
	"finalize": true,
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ExceptionRaisedInUnexpectedLocationRule) Confidence() float64 { return 0.75 }

// InstanceOfCheckForExceptionRule detects `is SomeException` inside catch blocks.
type InstanceOfCheckForExceptionRule struct {
	FlatDispatchBase
	BaseRule
}

var isExceptionRe = regexp.MustCompile(`\bis\s+\w*Exception\w*`)

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *InstanceOfCheckForExceptionRule) Confidence() float64 { return 0.75 }

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
					return false
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
func (r *NotImplementedDeclarationRule) Confidence() float64 { return 0.75 }

// RethrowCaughtExceptionRule detects catch { throw e } where e is the caught variable.
type RethrowCaughtExceptionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *RethrowCaughtExceptionRule) Confidence() float64 { return 0.75 }

// ReturnFromFinallyRule detects return statements inside finally blocks.
type ReturnFromFinallyRule struct {
	FlatDispatchBase
	BaseRule
	IgnoreLabeled bool
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ReturnFromFinallyRule) Confidence() float64 { return 0.75 }

// SwallowedExceptionRule detects catch blocks that either never use the exception
// variable or that throw a new exception without passing the original as the cause.
// Matches detekt's SwallowedException semantics: referencing only e.message or
// e.toString() (directly or via a variable) in a throw counts as swallowed.
type SwallowedExceptionRule struct {
	FlatDispatchBase
	BaseRule
	AllowedExceptionNameRegex *regexp.Regexp // exception names matching this are allowed to be swallowed
	IgnoredExceptionTypes     []string       // exception types that are allowed to be swallowed
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *SwallowedExceptionRule) Confidence() float64 { return 0.75 }

func (r *SwallowedExceptionRule) makeUnusedFindingFlat(ctx *v2.Context, caughtVar string) {
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

func (r *SwallowedExceptionRule) checkSwallowedException(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	if isTestFile(file.Path) {
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

	result := analyzeSwallowedCatchBody(ctx, idx, caughtVar)
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

type swallowedCatchResult struct {
	handled            bool
	indirectOnly       bool
	anyCaughtReference bool
}

func analyzeSwallowedCatchBody(ctx *v2.Context, catchNode uint32, caughtVar string) swallowedCatchResult {
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
			throwResult := swallowedAnalyzeThrow(file, node, directAliases, derivedAliases)
			if throwResult.handled {
				result.handled = true
				return false
			}
			result.indirectOnly = result.indirectOnly || throwResult.indirectOnly
		case "call_expression":
			callResult := swallowedAnalyzeCall(ctx, catchNode, node, directAliases, derivedAliases)
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

func swallowedAnalyzeThrow(file *scanner.File, node uint32, directAliases, derivedAliases map[string]bool) swallowedEvidence {
	if !swallowedIsThrowExpression(file, node) {
		return swallowedEvidence{}
	}
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

func swallowedIsThrowExpression(file *scanner.File, node uint32) bool {
	if file.FlatType(node) != "jump_expression" {
		return false
	}
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		return file.FlatType(child) == "throw"
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

func swallowedAnalyzeCall(ctx *v2.Context, catchNode, node uint32, directAliases, derivedAliases map[string]bool) swallowedEvidence {
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

func swallowedClassifyCall(ctx *v2.Context, catchNode, node uint32) swallowedCallKind {
	file := ctx.File
	callee, receivers := swallowedCallTarget(file, node)
	if callee == "" {
		return swallowedCallUnknown
	}
	if target := swallowedOracleCallTarget(ctx, node); target != "" {
		switch {
		case swallowedCallTargetIsLogging(target, callee):
			return swallowedCallLogging
		case swallowedCallTargetIsUI(target, callee):
			return swallowedCallUI
		case swallowedCallTargetLooksResolved(target):
			return swallowedCallUnknown
		}
	}
	if swallowedIsQualifiedLoggingCall(callee, receivers) || swallowedReceiverTypeIsLogging(ctx, node) {
		return swallowedCallLogging
	}
	if swallowedIsQualifiedUICall(callee, receivers) || swallowedReceiverTypeIsUI(ctx, node) {
		return swallowedCallUI
	}
	if swallowedIsHandlerName(callee) && swallowedSameOwnerLocalFunctionExists(file, catchNode, callee) {
		return swallowedCallLocalHandler
	}
	return swallowedCallUnknown
}

func swallowedOracleCallTarget(ctx *v2.Context, idx uint32) string {
	if ctx == nil || ctx.Resolver == nil || ctx.File == nil {
		return ""
	}
	var oracleLookup oracle.Lookup
	if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
		oracleLookup = cr.Oracle()
	}
	if oracleLookup == nil {
		return ""
	}
	return oracleLookupCallTargetFlat(oracleLookup, ctx.File, idx)
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

func swallowedIsQualifiedLoggingCall(callee string, receivers []string) bool {
	if len(receivers) == 0 {
		return false
	}
	if !swallowedLoggingCallee(callee) {
		return false
	}
	path := strings.Join(receivers, ".")
	switch path {
	case "Log", "android.util.Log", "Timber", "timber.log.Timber":
		return true
	default:
		return false
	}
}

func swallowedReceiverTypeIsLogging(ctx *v2.Context, call uint32) bool {
	receiver := swallowedCallReceiverNode(ctx.File, call)
	if receiver == 0 || ctx.Resolver == nil {
		return false
	}
	return swallowedResolvedTypeIsLogging(ctx.Resolver.ResolveFlatNode(receiver, ctx.File))
}

func swallowedCallReceiverNode(file *scanner.File, node uint32) uint32 {
	core := node
	if flatCallExpressionName(file, core) == "" {
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "call_expression" {
				core = child
				break
			}
		}
	}
	navExpr, _ := flatCallExpressionParts(file, core)
	if navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
		return 0
	}
	return file.FlatNamedChild(navExpr, 0)
}

func swallowedResolvedTypeIsLogging(t *typeinfer.ResolvedType) bool {
	if t == nil || t.FQN == "" {
		return false
	}
	return swallowedFQNMatchesAny(t.FQN,
		"android.util.Log",
		"timber.log.Timber",
		"org.slf4j.Logger",
		"java.util.logging.Logger",
		"kotlin.io.ConsoleKt",
	)
}

func swallowedCallTargetIsLogging(target, callee string) bool {
	if !swallowedLoggingCallee(callee) {
		return false
	}
	return swallowedFQNHasAnyPrefix(target,
		"android.util.Log.",
		"android.util.Log#",
		"timber.log.Timber.",
		"timber.log.Timber#",
		"org.slf4j.Logger.",
		"org.slf4j.Logger#",
		"java.util.logging.Logger.",
		"java.util.logging.Logger#",
	)
}

func swallowedLoggingCallee(callee string) bool {
	switch callee {
	case "v", "d", "i", "w", "e", "wtf", "println",
		"trace", "debug", "info", "warn", "warning", "severe",
		"error", "log", "logError", "logWarning",
		"logWarn", "logInfo", "logDebug", "logTrace":
		return true
	default:
		return false
	}
}

func swallowedCallTargetLooksResolved(target string) bool {
	return strings.Contains(target, ".") || strings.Contains(target, "#")
}

func swallowedFQNHasAnyPrefix(fqn string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(fqn, prefix) {
			return true
		}
	}
	return false
}

func swallowedFQNMatchesAny(fqn string, wants ...string) bool {
	for _, want := range wants {
		if fqn == want || strings.HasSuffix(fqn, "."+want) {
			return true
		}
	}
	return false
}

func swallowedIsQualifiedUICall(callee string, receivers []string) bool {
	uiReceivers := map[string]bool{
		"Toast": true, "Snackbar": true, "AlertDialog": true,
		"MaterialAlertDialog": true, "MaterialAlertDialogBuilder": true,
	}
	if len(receivers) > 0 {
		for _, receiver := range receivers {
			if uiReceivers[receiver] {
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

func swallowedReceiverTypeIsUI(ctx *v2.Context, call uint32) bool {
	receiver := swallowedCallReceiverNode(ctx.File, call)
	if receiver == 0 || ctx.Resolver == nil {
		return false
	}
	return swallowedResolvedTypeIsUI(ctx.Resolver.ResolveFlatNode(receiver, ctx.File))
}

func swallowedResolvedTypeIsUI(t *typeinfer.ResolvedType) bool {
	if t == nil || t.FQN == "" {
		return false
	}
	return swallowedFQNMatchesAny(t.FQN,
		"android.widget.Toast",
		"com.google.android.material.snackbar.Snackbar",
		"android.app.AlertDialog",
		"com.google.android.material.dialog.MaterialAlertDialogBuilder",
	)
}

func swallowedCallTargetIsUI(target, callee string) bool {
	switch callee {
	case "make", "makeText", "show", "showError", "showDialog", "showErrorDialog":
	default:
		return false
	}
	return swallowedFQNHasAnyPrefix(target,
		"android.widget.Toast.",
		"android.widget.Toast#",
		"com.google.android.material.snackbar.Snackbar.",
		"com.google.android.material.snackbar.Snackbar#",
		"android.app.AlertDialog.",
		"android.app.AlertDialog#",
		"com.google.android.material.dialog.MaterialAlertDialogBuilder.",
		"com.google.android.material.dialog.MaterialAlertDialogBuilder#",
	)
}

func swallowedExceptionCallTargetCallees() []string {
	return []string{
		"v", "d", "i", "w", "e", "wtf", "println",
		"trace", "debug", "info", "warn", "warning", "severe",
		"error", "log", "logError", "logWarning", "logWarn",
		"logInfo", "logDebug", "logTrace",
		"make", "makeText", "show", "showError", "showDialog", "showErrorDialog",
		"toastOn", "handleError", "reportError", "recoverFrom", "onError", "fallback", "notifyError",
	}
}

func swallowedExceptionCallTargetLexicalSkips() map[string][]string {
	logReceivers := []string{"Log", "Timber", "Logger"}
	uiReceivers := []string{"Toast", "Snackbar", "AlertDialog", "MaterialAlertDialog", "MaterialAlertDialogBuilder"}
	out := make(map[string][]string)
	for _, callee := range []string{"v", "d", "i", "w", "e", "wtf", "trace", "debug", "info", "warn", "warning", "severe", "error", "log", "logError", "logWarning", "logWarn", "logInfo", "logDebug", "logTrace"} {
		out[callee] = append([]string(nil), logReceivers...)
	}
	out["make"] = []string{"Snackbar", "MaterialAlertDialogBuilder"}
	out["makeText"] = []string{"Toast"}
	for _, callee := range []string{"show", "showError", "showDialog", "showErrorDialog"} {
		out[callee] = append([]string(nil), uiReceivers...)
	}
	return out
}

func swallowedIsHandlerName(name string) bool {
	switch name {
	case "toastOn", "showError", "showErrorDialog", "handleError",
		"reportError", "recoverFrom", "onError", "fallback", "notifyError",
		"logError", "logWarning", "logWarn":
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
func (r *ThrowingExceptionFromFinallyRule) Confidence() float64 { return 0.75 }

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

// Default exception types that should have a message (matches detekt defaults)
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
func (r *ThrowingExceptionsWithoutMessageOrCauseRule) Confidence() float64 { return 0.75 }

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
func (r *ThrowingNewInstanceOfSameExceptionRule) Confidence() float64 { return 0.75 }

// ThrowingExceptionInMainRule detects throw in main function.
type ThrowingExceptionInMainRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ThrowingExceptionInMainRule) Confidence() float64 { return 0.75 }

// ErrorUsageWithThrowableRule detects error(throwable) calls.
type ErrorUsageWithThrowableRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ErrorUsageWithThrowableRule) Confidence() float64 { return 0.75 }

// ObjectExtendsThrowableRule detects object : Exception/Throwable/Error.
type ObjectExtendsThrowableRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — relies on
// resolver to determine supertypes; falls back to name-based heuristics on
// the `Throwable` identifier. Classified per roadmap/17.
func (r *ObjectExtendsThrowableRule) Confidence() float64 { return 0.75 }

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
}
