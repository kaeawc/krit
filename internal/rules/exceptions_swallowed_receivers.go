package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

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
