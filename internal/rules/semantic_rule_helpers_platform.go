package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

func logCallIsAndroidLog(ctx *api.Context, call uint32) bool {
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok || target.CalleeName == "" || !logLevelNames[target.CalleeName] {
		return false
	}
	receiver := flatReceiverNameFromCall(ctx.File, call)
	if (receiver == "Log" || receiver == "android.util.Log") && !logFileHasOnlyAndroidLogReferences(ctx.File) {
		return false
	}
	if target.Resolved {
		return semanticTargetOwnerMatches(target.QualifiedName, "android.util.Log")
	}
	switch receiver {
	case "Log":
		// Require an import of `android.util.Log` so Timber.Log,
		// slf4j Logger aliases, kotlin-logging KLogger, and same-file
		// declarations named `Log` are not misclassified as
		// android.util.Log. The resolver already shadows imports with
		// same-file declarations of the simple name; the filefacts
		// fallback still rejects when the file imports anything other
		// than android.util.Log under the simple name `Log`.
		if ctx.Resolver != nil {
			return ctx.Resolver.ResolveImport("Log", ctx.File) == "android.util.Log"
		}
		return fileImportsFQN(ctx.File, "android.util.Log") &&
			!fileDeclaresSimpleTypeNameLocal(ctx.File, "Log")
	case "android.util.Log":
		return true
	default:
		return false
	}
}

func logFileHasOnlyAndroidLogReferences(file *scanner.File) bool {
	if file == nil {
		return false
	}
	valid := true
	file.FlatWalkAllNodes(0, func(node uint32) {
		if !valid {
			return
		}
		nodeType := file.FlatType(node)
		if (nodeType != "simple_identifier" && nodeType != "type_identifier" && nodeType != "interpolated_identifier") || logLexicalIdentifierName(file.FlatNodeText(node)) != "Log" {
			return
		}
		for parent, ok := file.FlatParent(node); ok; parent, ok = file.FlatParent(parent) {
			parentType := file.FlatType(parent)
			if parentType == "line_comment" || parentType == "multiline_comment" || parentType == "string_content" {
				return
			}
			if parentType == "import_header" || parentType == "import_declaration" {
				if logCanonicalImportHeader(file.FlatNodeText(parent)) {
					return
				}
				valid = false
				return
			}
			if parentType == "navigation_expression" {
				receiver := file.FlatNamedChild(parent, 0)
				if receiver != 0 && (logLexicalIdentifierName(file.FlatNodeText(receiver)) == "Log" || file.FlatNodeText(receiver) == "android.util.Log") &&
					file.FlatStartByte(node) >= file.FlatStartByte(receiver) && file.FlatEndByte(node) <= file.FlatEndByte(receiver) {
					return
				}
			}
		}
		valid = false
	})
	return valid
}

func logLexicalIdentifierName(text string) string {
	if len(text) > 2 && strings.HasPrefix(text, "`") && strings.HasSuffix(text, "`") {
		return text[1 : len(text)-1]
	}
	return text
}

func logCanonicalImportHeader(importHeader string) bool {
	importPath := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(importHeader), "import "))
	importPath = strings.TrimSpace(strings.TrimSuffix(importPath, ";"))
	importPath = strings.TrimSpace(strings.TrimPrefix(importPath, "static "))
	return importPath == "android.util.Log"
}

// logReceiverImportsAndroidLog reports whether the `Log` receiver of a call
// written `Log.x(...)` can only be android.util.Log from the file's own text:
// the file imports android.util.Log (explicitly or through android.util.*),
// imports no other class as `Log`, and declares no `Log` of its own. It is the
// evidence a rule falls back to when the oracle cannot resolve the call, for
// example in a file that does not compile.
func logReceiverImportsAndroidLog(file *scanner.File) bool {
	imports := fileFactsCache().Imports(file)
	if imports == nil {
		return false
	}
	// An explicit import under the name Log wins over a star import; an
	// android.util.Log imported under an alias leaves Log unbound.
	visible := false
	if fqn, ok := imports.Aliases["Log"]; ok {
		visible = fqn == "android.util.Log"
	} else {
		_, visible = imports.Wildcards["android.util.*"]
	}
	return visible && !fileDeclaresSimpleTypeNameLocal(file, "Log")
}

// fileDeclaresSimpleTypeNameLocal reports whether `file` declares a
// top-level class / interface / object / type alias whose simple name is
// `name`. Used to reject same-file lookalikes (e.g. a local `class Log`)
// when the resolver is unavailable.
func fileDeclaresSimpleTypeNameLocal(file *scanner.File, name string) bool {
	if file == nil || name == "" {
		return false
	}
	found := false
	for _, kind := range []string{"class_declaration", "object_declaration", "type_alias", "interface_declaration"} {
		file.FlatWalkNodes(0, kind, func(decl uint32) {
			if found {
				return
			}
			if semantics.DeclarationName(file, decl) == name {
				found = true
			}
		})
		if found {
			return true
		}
	}
	return false
}

func logCallIsConditionalFlat(file *scanner.File, call uint32) bool {
	for parent, ok := file.FlatParent(call); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "if_expression":
			condition, _ := ifConditionAndThenBodyFlat(file, parent)
			return file.FlatNodeText(flatUnwrapParenExpr(file, condition)) != "true"
		case "when_entry":
			for child := file.FlatFirstChild(parent); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) == "when_condition" && file.FlatNodeText(flatUnwrapParenExpr(file, file.FlatNamedChild(child, 0))) != "true" {
					return true
				}
			}
			return false
		case "function_declaration":
			if !logDeclarationIsLocalFlat(file, parent) {
				return false
			}
		case "property_declaration":
			if !logDeclarationIsLocalFlat(file, parent) {
				return false
			}
		case "call_expression":
			callee, _ := flatCallExpressionParts(file, parent)
			if callee == 0 || file.FlatStartByte(callee) > file.FlatStartByte(call) || file.FlatEndByte(call) > file.FlatEndByte(callee) {
				return false
			}
		case "class_declaration", "object_declaration", "object_literal",
			"primary_constructor", "secondary_constructor", "getter", "setter", "anonymous_initializer", "source_file":
			return false
		}
	}
	return false
}

func logDeclarationIsLocalFlat(file *scanner.File, declaration uint32) bool {
	for parent, ok := file.FlatParent(declaration); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "function_body", "lambda_literal", "anonymous_function", "anonymous_initializer", "secondary_constructor":
			return true
		case "class_body", "source_file":
			return false
		}
	}
	return false
}

func logMessageDoesWorkFlat(ctx *api.Context, call uint32) bool {
	_, arguments := flatCallExpressionParts(ctx.File, call)
	position := 1
	if flatCallExpressionName(ctx.File, call) == "println" {
		position = 2
	}
	for argument := ctx.File.FlatFirstChild(arguments); argument != 0; argument = ctx.File.FlatNextSib(argument) {
		if ctx.File.FlatType(argument) != "value_argument" {
			continue
		}
		if position == 0 {
			return logExpressionDoesWorkFlat(ctx, flatValueArgumentExpression(ctx.File, argument))
		}
		position--
	}
	return false
}

func logExpressionDoesWorkFlat(ctx *api.Context, expression uint32) bool {
	expression = flatUnwrapParenExpr(ctx.File, expression)
	if expression == 0 {
		return false
	}
	switch ctx.File.FlatType(expression) {
	case "call_expression":
		return true
	case "string_literal":
		for entry := ctx.File.FlatFirstChild(expression); entry != 0; entry = ctx.File.FlatNextSib(entry) {
			if ctx.File.FlatType(entry) != "interpolated_expression" {
				continue
			}
			for value := ctx.File.FlatFirstChild(entry); value != 0; value = ctx.File.FlatNextSib(value) {
				if ctx.File.FlatIsNamed(value) && ctx.File.FlatType(flatUnwrapParenExpr(ctx.File, value)) == "call_expression" {
					return true
				}
			}
		}
	case "additive_expression", "binary_expression":
		var operator string
		var operands []uint32
		for child := ctx.File.FlatFirstChild(expression); child != 0; child = ctx.File.FlatNextSib(child) {
			if ctx.File.FlatIsNamed(child) {
				operands = append(operands, child)
			} else if operator == "" {
				operator = ctx.File.FlatNodeText(child)
			}
		}
		if operator != "+" {
			return false
		}
		for _, operand := range operands {
			if logExpressionDoesWorkFlat(ctx, operand) {
				return true
			}
		}
	case "elvis_expression":
		left := flatUnwrapParenExpr(ctx.File, ctx.File.FlatNamedChild(expression, 0))
		return logElvisLeftIsCall(ctx.File, left)
	}
	return false
}

func logElvisLeftIsCall(file *scanner.File, expression uint32) bool {
	switch file.FlatType(expression) {
	case "call_expression":
		return true
	case "navigation_expression":
		selector := flatLastChildOfType(file, expression, "navigation_suffix")
		if selector == 0 {
			return false
		}
		var containsCall bool
		var walk func(uint32)
		walk = func(node uint32) {
			if containsCall || node == 0 {
				return
			}
			if file.FlatType(node) == "call_expression" || file.FlatType(node) == "call_suffix" {
				containsCall = true
				return
			}
			for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
				walk(child)
			}
		}
		walk(selector)
		return containsCall
	default:
		return false
	}
}

func wakeLockAcquireCall(ctx *api.Context, call uint32) bool {
	if flatCallExpressionName(ctx.File, call) != "acquire" {
		return false
	}
	if semanticCallTargetOrReceiverType(ctx, call,
		[]string{"android.os.PowerManager.WakeLock"},
		[]string{"android.os.PowerManager.WakeLock", "WakeLock"},
	) {
		return true
	}
	return receiverDeclaredFromCall(ctx.File, call, "newWakeLock")
}

// lintActivityKnownNames are framework Activity subclasses a class may extend
// without the resolver seeing the hierarchy.
var lintActivityKnownNames = map[string]struct{}{
	"Activity": {}, "ComponentActivity": {}, "FragmentActivity": {}, "AppCompatActivity": {},
	"ListActivity": {}, "PreferenceActivity": {}, "TabActivity": {}, "ActivityGroup": {},
}

// wakeLockReleaseInActivityOnDestroy mirrors Lint's onDestroy check: a
// WakeLock release() whose nearest enclosing method (lambdas and local
// functions sit inside it) is onDestroy of an android.app.Activity subclass.
func wakeLockReleaseInActivityOnDestroy(ctx *api.Context, call uint32) bool {
	file := ctx.File
	if flatCallExpressionName(file, call) != "release" {
		return false
	}
	method := uint32(0)
	for current, ok := file.FlatParent(call); ok && method == 0; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "function_declaration":
			if parent, ok := file.FlatParent(current); ok && flatIsClassBody(file, parent) {
				method = current
			}
		case "getter", "setter", "secondary_constructor", "anonymous_initializer", "source_file":
			return false
		}
	}
	if method == 0 || extractIdentifierFlat(file, method) != "onDestroy" {
		return false
	}
	cls, ok := flatEnclosingAncestor(file, method, "class_declaration", "object_declaration", "object_literal")
	if !ok || file.FlatType(cls) != "class_declaration" || !lintClassExtends(ctx, cls, "android.app.Activity", lintActivityKnownNames) {
		return false
	}
	return semanticCallTargetOrReceiverType(ctx, call,
		[]string{"android.os.PowerManager.WakeLock"},
		[]string{"android.os.PowerManager.WakeLock", "WakeLock"},
	) || receiverDeclaredFromCall(file, call, "newWakeLock")
}

// wakeLockReleaseScope is where a release of the acquire's receiver may sit.
// Lint never asks for the release in the acquiring function (acquire in
// onResume, release in onPause), so it is the acquire's top-level class, or
// the file for a top-level function. A local declared in the function lives
// only there, and Go matches a release by name, so a local of another
// function with the same name must not count.
func wakeLockReleaseScope(file *scanner.File, call, fn uint32) uint32 {
	receiver := astReceiverNodeFromCall(file, call)
	if root := androidReceiverChainRoot(file, receiver); root != "" && flatDeclaresLocalBefore(file, fn, root, call) {
		return fn
	}
	scope := uint32(0)
	for current, ok := file.FlatParent(call); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "class_declaration", "object_declaration":
			scope = current
		}
	}
	return scope
}

// wakeLockReleaseNamesShadow reports a release whose receiver root is a local or parameter of the release's own
// function, so it cannot be the class-level lock that acquire() took.
func wakeLockReleaseNamesShadow(file *scanner.File, release uint32) bool {
	receiver := astReceiverNodeFromCall(file, release)
	if receiver == 0 {
		return false
	}
	root := androidReceiverChainRoot(file, receiver)
	if root == "" || root == "this" || root == "super" {
		return false
	}
	return flatNameBoundLocallyAt(file, root, release) || wakeLockReceiverIsParameter(file, release)
}

// flatNameBoundLocallyAt reports a local property or lambda parameter named name that is visible at the node at:
// declared earlier in a block that encloses at, inside the same function.
func flatNameBoundLocallyAt(file *scanner.File, name string, at uint32) bool {
	start := file.FlatStartByte(at)
	for cur, ok := file.FlatParent(at); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "class_declaration", "object_declaration", "class_body", "source_file":
			return false
		case "lambda_literal":
			if params, found := file.FlatFindChild(cur, "lambda_parameters"); found {
				for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
					if file.FlatType(p) == "variable_declaration" && lintDeclaredName(file, p) == name {
						return true
					}
				}
			}
		}
		for child := file.FlatFirstChild(cur); child != 0 && file.FlatStartByte(child) < start; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "property_declaration" && extractIdentifierFlat(file, child) == name {
				return true
			}
		}
	}
	return false
}

// flatDeclaresLocalBefore reports whether fn declares a local named name
// before the node at.
func flatDeclaresLocalBefore(file *scanner.File, fn uint32, name string, at uint32) bool {
	found := false
	file.FlatWalkAllNodes(fn, func(n uint32) {
		if found || file.FlatStartByte(n) >= file.FlatStartByte(at) {
			return
		}
		if t := file.FlatType(n); t == "property_declaration" && extractIdentifierFlat(file, n) == name {
			found = true
		}
	})
	return found
}

// wakeLockReceiverIsParameter reports whether the acquire's receiver chain
// starts at a parameter of an enclosing function, or at the receiver of the
// extension function it sits in (a bare or `this` call outside a lambda).
func wakeLockReceiverIsParameter(file *scanner.File, call uint32) bool {
	receiver := astReceiverNodeFromCall(file, call)
	root := ""
	if receiver != 0 {
		root = androidReceiverChainRoot(file, receiver)
	}
	if receiver == 0 || root == "this" {
		scope, ok := flatEnclosingAncestor(file, call, "function_declaration", "lambda_literal", "anonymous_function")
		return ok && file.FlatType(scope) == "function_declaration" && flatIsExtensionFunction(file, scope)
	}
	if root == "" || root == "super" {
		return false
	}
	for fn, ok := flatEnclosingFunction(file, call); ok; fn, ok = flatEnclosingFunction(file, fn) {
		params, found := file.FlatFindChild(fn, "function_value_parameters")
		if !found {
			continue
		}
		for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
			if file.FlatType(p) == "parameter" && extractIdentifierFlat(file, p) == root {
				return true
			}
		}
	}
	return false
}

// flatIsExtensionFunction reports whether a Kotlin function_declaration
// declares a receiver type (`fun Type.name()`).
func flatIsExtensionFunction(file *scanner.File, fn uint32) bool {
	for c := file.FlatFirstChild(fn); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "simple_identifier":
			return false
		case ".":
			return true
		}
	}
	return false
}

func wakeLockReleaseCallOnSameReceiver(ctx *api.Context, acquire, release uint32) bool {
	if flatCallExpressionName(ctx.File, release) != "release" {
		return false
	}
	if !semanticCallTargetOrReceiverType(ctx, release,
		[]string{"android.os.PowerManager.WakeLock"},
		[]string{"android.os.PowerManager.WakeLock", "WakeLock"},
	) {
		if !receiverDeclaredFromCall(ctx.File, release, "newWakeLock") {
			return false
		}
	}
	ar := semantics.ReferenceName(ctx.File, semanticsReceiverNode(ctx, acquire))
	rr := semantics.ReferenceName(ctx.File, semanticsReceiverNode(ctx, release))
	return ar != "" && ar == rr
}

func setJavaScriptEnabledCall(ctx *api.Context, call uint32) bool {
	if flatCallExpressionName(ctx.File, call) != "setJavaScriptEnabled" {
		return false
	}
	if semanticCallTargetOrReceiverType(ctx, call,
		[]string{"android.webkit.WebSettings"},
		[]string{"android.webkit.WebSettings", "WebSettings"},
	) {
		return true
	}
	receiver := semanticsReceiverNode(ctx, call)
	if receiver != 0 && navigationReceiverHasTypedRoot(ctx.File, receiver, "WebView") &&
		navigationChainContainsSegment(ctx.File, receiver, "settings") {
		return true
	}
	// Without types, the shared WebSettings receiver heuristics: a `settings` chain or name in a file that uses the
	// WebView API, or the implicit receiver of `settings.apply { }` / `with(settings) { }`.
	return isWebSettingsReceiverAt(ctx.File, call, kotlinCallReceiverChain(ctx.File, call))
}

func setJavaScriptEnabledJavaCall(ctx *api.Context, call uint32) bool {
	file := ctx.File
	if javaMethodInvocationName(file, call) != "setJavaScriptEnabled" {
		return false
	}
	args := javaArgumentExpressions(file, call)
	if len(args) != 1 {
		return false
	}
	if on, known := lintEvalBool(file, args[0]); !known || !on {
		return false
	}
	if fact, ok := javaSemanticCallFact(ctx, call); ok && fact.ReceiverType != "" {
		return javaProfileTypeMatches(ctx, fact.ReceiverType, "android.webkit.WebSettings")
	}
	if lc, ok := lintCallAt(file, call); ok && lintReceiverIsType(ctx, lc, "android.webkit.WebSettings") {
		return true
	}
	if !sourceImportsOrMentions(file, "android.webkit.WebSettings") && !sourceImportsOrMentions(file, "android.webkit.WebView") {
		return false
	}
	receiver := javaMethodReceiverText(file, call)
	if strings.Contains(receiver, "getSettings") {
		return true
	}
	name := wrongViewCastCallReceiverName(file, call)
	return name == "settings" || name == "webSettings" || strings.HasSuffix(name, ".settings") || strings.HasSuffix(name, ".webSettings")
}

func webSettingsAssignmentTarget(ctx *api.Context, assignment uint32) bool {
	file := ctx.File
	target, _ := file.FlatFindChild(assignment, "directly_assignable_expression")
	if target == 0 || finalSimpleIdentifier(file, target) != "javaScriptEnabled" {
		return false
	}
	if directlyAssignableWebSettingsTarget(file, assignment, target) {
		return true
	}
	if segments := directlyAssignableSegments(file, target); len(segments) >= 1 &&
		isWebSettingsReceiverAt(file, assignment, strings.Join(segments[:len(segments)-1], ".")) {
		return true
	}
	receiver := assignmentReceiverBeforeFinal(file, target)
	if receiver == 0 {
		return false
	}
	if semanticTypeNameMatches(semanticReferenceTypeName(ctx, receiver), "android.webkit.WebSettings", "WebSettings") {
		return true
	}
	if file.FlatType(receiver) == "navigation_expression" {
		return navigationReceiverHasTypedRoot(file, receiver, "WebView") && navigationChainContainsSegment(file, receiver, "settings")
	}
	foundWebSettingsReceiver := false
	file.FlatWalkNodes(target, "navigation_expression", func(nav uint32) {
		if foundWebSettingsReceiver {
			return
		}
		if semanticTypeNameMatches(semanticReferenceTypeName(ctx, nav), "android.webkit.WebSettings", "WebSettings") ||
			(navigationReceiverHasTypedRoot(file, nav, "WebView") && navigationChainContainsSegment(file, nav, "settings")) {
			foundWebSettingsReceiver = true
		}
	})
	if foundWebSettingsReceiver {
		return true
	}
	name := semantics.ReferenceName(file, receiver)
	fn, ok := flatEnclosingFunction(file, assignment)
	return ok && parameterHasTypeFlat(file, fn, name, "WebSettings")
}

func directlyAssignableWebSettingsTarget(file *scanner.File, assignment uint32, target uint32) bool {
	segments := directlyAssignableSegments(file, target)
	if len(segments) < 2 || segments[len(segments)-1] != "javaScriptEnabled" {
		return false
	}
	fn, ok := flatEnclosingFunction(file, assignment)
	if !ok {
		return false
	}
	if len(segments) == 2 {
		return parameterHasTypeFlat(file, fn, segments[0], "WebSettings")
	}
	return navigationSegmentsContain(segments[:len(segments)-1], "settings") && parameterHasTypeFlat(file, fn, segments[0], "WebView")
}

func directlyAssignableSegments(file *scanner.File, target uint32) []string {
	if file.FlatType(target) != "directly_assignable_expression" {
		return nil
	}
	var out []string
	for child := file.FlatFirstChild(target); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			out = append(out, file.FlatNodeText(child))
		case "navigation_suffix":
			if ident, ok := file.FlatFindChild(child, "simple_identifier"); ok {
				out = append(out, file.FlatNodeText(ident))
			}
		case "navigation_expression":
			out = append(out, flatNavigationChainIdentifiers(file, child)...)
		}
	}
	return out
}

func assignmentReceiverBeforeFinal(file *scanner.File, idx uint32) uint32 {
	switch file.FlatType(idx) {
	case "directly_assignable_expression":
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				return assignmentReceiverBeforeFinal(file, child)
			}
		}
	case "navigation_expression":
		var receiver uint32
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			if file.FlatType(child) == "navigation_suffix" {
				break
			}
			receiver = child
		}
		return receiver
	}
	return 0
}

// apiHasPrecedingVersionReturnGuard reports an earlier `if (Build.VERSION.SDK_INT < N) return` (or throw) in the
// block that holds the node, so the code after it only runs on N and above.
func apiHasPrecedingVersionReturnGuard(file *scanner.File, idx uint32) bool {
	scope := enclosingAncestor(file, idx, "function_declaration", "secondary_constructor", "anonymous_initializer", "lambda_literal")
	if scope == 0 {
		return false
	}
	nodeStmt, nodeContainer := missingPermissionContainingStatement(file, idx)
	if nodeContainer == 0 {
		return false
	}
	guarded := false
	file.FlatWalkNodes(scope, "if_expression", func(ifExpr uint32) {
		if guarded || file.FlatStartByte(ifExpr) >= file.FlatStartByte(idx) {
			return
		}
		ifStmt, ifContainer := missingPermissionContainingStatement(file, ifExpr)
		if ifContainer != nodeContainer || file.FlatEndByte(ifStmt) > file.FlatStartByte(nodeStmt) {
			return
		}
		cond, thenBody, _ := missingPermissionIfParts(file, ifExpr)
		if cond == 0 || thenBody == 0 {
			return
		}
		condText := strings.Join(strings.Fields(file.FlatNodeText(cond)), "")
		if strings.Contains(condText, "SDK_INT<") && missingPermissionBodyTerminates(file, thenBody) {
			guarded = true
		}
	})
	return guarded
}

func apiGuardedByVersionCheckFlat(file *scanner.File, idx uint32) bool {
	if apiHasPrecedingVersionReturnGuard(file, idx) {
		return true
	}
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if declarationHasAPIGuardAnnotationFlat(file, p) {
			return true
		}
		switch file.FlatType(p) {
		case "annotation":
			name := annotationFinalName(file, p)
			if name == "RequiresApi" || name == "TargetApi" {
				return true
			}
		case "if_expression", "when_expression":
			found := false
			file.FlatWalkAllNodes(p, func(n uint32) {
				if found || n == idx {
					return
				}
				if file.FlatType(n) == "navigation_expression" {
					segments := flatNavigationChainIdentifiers(file, n)
					if len(segments) >= 3 && segments[0] == "Build" && segments[1] == "VERSION" && segments[2] == "SDK_INT" {
						found = true
					}
				}
			})
			if found {
				return true
			}
		case "source_file":
			return false
		}
	}
	return false
}

func declarationHasAPIGuardAnnotationFlat(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "function_declaration", "class_declaration", "object_declaration", "property_declaration", "variable_declaration":
	default:
		return false
	}
	mods, _ := file.FlatFindChild(idx, "modifiers")
	if mods == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(mods, "annotation", func(ann uint32) {
		if found {
			return
		}
		name := annotationFinalName(file, ann)
		if name == "RequiresApi" || name == "TargetApi" {
			found = true
		}
	})
	return found
}

func nodeHasAncestorTypeFlat(file *scanner.File, idx uint32, types ...string) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		pt := file.FlatType(p)
		for _, typ := range types {
			if pt == typ {
				return true
			}
		}
	}
	return false
}

func apiNodeNameFlat(file *scanner.File, idx uint32) string {
	switch file.FlatType(idx) {
	case "call_expression":
		if name := flatCallExpressionName(file, idx); name != "" {
			return name
		}
		return simpleCallExpressionNameFromText(file.FlatNodeText(idx))
	case "simple_identifier", "type_identifier":
		return file.FlatNodeText(idx)
	case "user_type", "navigation_expression":
		if name := flatNavigationExpressionLastIdentifier(file, idx); name != "" {
			return name
		}
		return strings.TrimSpace(file.FlatNodeText(idx))
	}
	return semantics.ReferenceName(file, idx)
}

func newAPINestedAccessHandledByOuterNode(file *scanner.File, idx uint32) bool {
	name := apiNodeNameFlat(file, idx)
	if name == "" {
		return false
	}
	switch file.FlatType(idx) {
	case "simple_identifier", "type_identifier":
		for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
			switch file.FlatType(parent) {
			case "call_expression", "navigation_expression", "user_type":
				if apiNodeNameFlat(file, parent) == name {
					return true
				}
			case "source_file":
				return false
			}
		}
	case "navigation_expression":
		for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
			switch file.FlatType(parent) {
			case "call_expression", "user_type":
				if apiNodeNameFlat(file, parent) == name {
					return true
				}
			case "source_file":
				return false
			}
		}
	case "user_type":
		for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
			switch file.FlatType(parent) {
			case "user_type":
				if apiNodeNameFlat(file, parent) == name {
					return true
				}
			case "call_expression", "navigation_expression", "source_file":
				return false
			}
		}
	}
	return false
}

func inlinedAPIReferenceNameFlat(file *scanner.File, idx uint32) string {
	segments := flatNavigationChainIdentifiers(file, idx)
	if len(segments) > 0 {
		return strings.Join(segments, ".")
	}
	return apiNodeNameFlat(file, idx)
}

func sharedPrefsEditorCall(ctx *api.Context, call uint32) bool {
	name := flatCallExpressionName(ctx.File, call)
	if !isSharedPrefsPutMethod(name) && !isSharedPrefsGetMethod(name) {
		return false
	}
	return semanticCallTargetOrReceiverType(ctx, call,
		[]string{"android.content.SharedPreferences.Editor", "android.content.SharedPreferences"},
		[]string{"android.content.SharedPreferences.Editor", "android.content.SharedPreferences", "Editor", "SharedPreferences"},
	) || sameFileFunctionDeclaredInOwner(ctx.File, call, "Editor", "SharedPreferences")
}

func encryptedStorageReceiver(ctx *api.Context, call uint32) bool {
	receiver := semanticsReceiverNode(ctx, call)
	if receiver == 0 {
		receiver = astReceiverNodeFromCall(ctx.File, call)
		if receiver == 0 {
			return false
		}
	}
	typ, ok := semantics.ExpressionType(ctx, receiver)
	if !ok {
		return receiverChainHasQualifiedRoot(ctx.File, receiver, "EncryptedSharedPreferences", "EncryptedFile")
	}
	return semanticTypeNameMatches(typ.Name, "EncryptedSharedPreferences", "EncryptedFile") ||
		semanticTypeNameMatches(typ.FQN, "androidx.security.crypto.EncryptedSharedPreferences", "androidx.security.crypto.EncryptedFile")
}

func plainFileWriteCall(ctx *api.Context, call uint32) bool {
	name := flatCallExpressionName(ctx.File, call)
	if name != "writeText" && name != "writeBytes" {
		return false
	}
	return semanticCallTargetOrReceiverType(ctx, call,
		[]string{"java.io.File", "kotlin.io.FilesKt"},
		[]string{"java.io.File", "File"},
	) || receiverDeclaredFromCall(ctx.File, call, "File") || receiverContainsCallName(ctx.File, call, "File")
}

func sensitiveFileReceiverLiteral(ctx *api.Context, call uint32) string {
	receiver := semanticsReceiverNode(ctx, call)
	if receiver == 0 {
		receiver = astReceiverNodeFromCall(ctx.File, call)
	}
	if receiver == 0 {
		return ""
	}
	if ctx.File.FlatType(receiver) != "call_expression" || flatCallExpressionName(ctx.File, receiver) != "File" {
		found := uint32(0)
		ctx.File.FlatWalkNodes(receiver, "call_expression", func(candidate uint32) {
			if found == 0 && flatCallExpressionName(ctx.File, candidate) == "File" {
				found = candidate
			}
		})
		receiver = found
		if receiver == 0 {
			return ""
		}
	}
	_, args := flatCallExpressionParts(ctx.File, receiver)
	if args == 0 {
		return ""
	}
	for arg := ctx.File.FlatFirstChild(args); arg != 0; arg = ctx.File.FlatNextSib(arg) {
		if ctx.File.FlatType(arg) != "value_argument" {
			continue
		}
		expr := flatValueArgumentExpression(ctx.File, arg)
		if expr != 0 && ctx.File.FlatType(expr) == "string_literal" {
			return stringLiteralContent(ctx.File, expr)
		}
	}
	return ""
}

// newAPICompatReceiver reports whether the API reference is written through a compat helper such as
// ContextCompat.getColor, which works on every API level.
func newAPICompatReceiver(file *scanner.File, idx uint32, name string) bool {
	text := file.FlatNodeText(idx)
	i := strings.Index(text, name)
	return i > 0 && strings.Contains(text[:i], "Compat")
}
