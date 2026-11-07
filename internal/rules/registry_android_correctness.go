package rules

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func registerAndroidCorrectnessRules() {
	registerAndroidCorrectnessDefaultLocale()
	registerAndroidCorrectnessCommitPrefEdits()
	registerAndroidCorrectnessCommitTransaction()
	registerAndroidCorrectnessAssert()
	registerAndroidCorrectnessCheckResult()
	registerAndroidCorrectnessShiftFlags()
	registerAndroidCorrectnessUniqueConstants()
	registerAndroidCorrectnessWrongThread()
	registerAndroidCorrectnessSQLiteString()
	registerAndroidCorrectnessRegistered()
	registerAndroidCorrectnessScrollViewCount()
	registerAndroidCorrectnessSimpleDateFormat()
	registerAndroidCorrectnessSetTextI18n()
	registerAndroidCorrectnessStopShip()
	registerAndroidCorrectnessWrongCall()
}

func registerAndroidCorrectnessDefaultLocale() {
	r := &DefaultLocaleRule{AndroidRule: alcRule("DefaultLocale", "Implied default locale in case conversion", ALSWarning, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes:  []string{"call_expression", "method_invocation"},
		Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Needs:      api.NeedsTypeInfo,
		Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if file.FlatType(idx) == "method_invocation" {
				checkDefaultLocaleJava(ctx, idx)
				return
			}
			switch flatCallExpressionName(file, idx) {
			case "toLowerCase", "toUpperCase":
				// Only the zero-argument String form uses the default locale; Char.toLowerCase() does not.
				navExpr, args := flatCallExpressionParts(file, idx)
				if args != 0 && file.FlatNamedChildCount(args) > 0 {
					return
				}
				if navExpr != 0 && defaultLocaleReceiverIsChar(ctx, navExpr) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, defaultLocaleCaseMessage(flatCallExpressionName(file, idx)))
			case "format":
				// String.format(pattern, ...) without an explicit Locale first arg.
				navExpr, args := flatCallExpressionParts(file, idx)
				if navExpr == 0 || args == 0 {
					return
				}
				if recv := file.FlatNodeText(file.FlatFirstChild(navExpr)); recv != "String" && recv != "java.lang.String" {
					return
				}
				// Only the first positional argument, or one labeled `locale`, can be the Locale; a Locale
				// later in the list is a format argument.
				first := true
				for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
					if file.FlatType(arg) != "value_argument" {
						continue
					}
					label := flatValueArgumentLabel(file, arg)
					positionalFirst := first && label == ""
					first = false
					if label != "locale" && !positionalFirst {
						continue
					}
					expr := flatValueArgumentExpression(file, arg)
					if label == "locale" || kotlinExprIsLocaleReference(file, expr) ||
						strings.TrimSpace(file.FlatNodeText(arg)) == "null" || defaultLocaleArgIsLocale(ctx, expr) {
						return
					}
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, defaultLocaleFormatMessage)
			}
		},
	})
}

const (
	defaultLocaleFormatMessage = "Implicitly using the default locale is a common source of bugs: Use `String.format(Locale, ...)` instead"
)

// defaultLocaleCaseMessage names the case-conversion method the way Lint does.
func defaultLocaleCaseMessage(method string) string {
	return "Implicitly using the default locale is a common source of bugs: Use `" + method + "(Locale)` instead. " +
		"For strings meant to be internal use `Locale.ROOT`, otherwise `Locale.getDefault()`."
}

// javaCallNameNode returns the identifier node naming a Java method invocation.
func javaCallNameNode(file *scanner.File, call uint32) uint32 {
	var last uint32
	for child := file.FlatFirstChild(call); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "argument_list" {
			break
		}
		if file.FlatType(child) == "identifier" {
			last = child
		}
	}
	return last
}

// defaultLocaleArgIsLocale reports an argument the resolver types as a Locale.
func defaultLocaleArgIsLocale(ctx *api.Context, expr uint32) bool {
	file := ctx.File
	if expr == 0 || ctx.Resolver == nil {
		return false
	}
	typ := ctx.Resolver.ResolveFlatNode(expr, file)
	if (typ == nil || typ.Kind == typeinfer.TypeUnknown) && file.FlatType(expr) == "simple_identifier" {
		typ = ctx.Resolver.ResolveByNameFlat(file.FlatNodeText(expr), expr, file)
	}
	return typ != nil && (typ.Name == "Locale" || typ.FQN == "java.util.Locale" || typ.Name == "ULocale")
}

// defaultLocaleReceiverIsChar reports whether the receiver of a navigation call is known to be a Char.
func defaultLocaleReceiverIsChar(ctx *api.Context, navExpr uint32) bool {
	if ctx.Resolver == nil || ctx.File.FlatNamedChildCount(navExpr) == 0 {
		return false
	}
	recv := ctx.File.FlatNamedChild(navExpr, 0)
	typ := ctx.Resolver.ResolveFlatNode(recv, ctx.File)
	if (typ == nil || typ.Kind == typeinfer.TypeUnknown) && ctx.File.FlatType(recv) == "simple_identifier" {
		typ = ctx.Resolver.ResolveByNameFlat(ctx.File.FlatNodeText(recv), recv, ctx.File)
	}
	return typ != nil && (typ.Name == "Char" || typ.FQN == "kotlin.Char" || typ.Name == "Character")
}

func registerAndroidCorrectnessCommitPrefEdits() {
	r := &CommitPrefEditsRule{AndroidRule: alcRule("CommitPrefEdits", "Missing commit() on SharedPreferences editor", ALSWarning, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression", "method_invocation"},
		Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: api.ConfidenceMediumHigh, Implementation: r,
		Needs:             api.NeedsTypeInfo | api.NeedsOracleCallTargets,
		JavaFacts:         &api.JavaFactProfile{ReceiverTypesForCallees: []string{"edit"}},
		NeedsLibraryFacts: true,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if file.FlatType(idx) == "method_invocation" {
				checkCommitPrefEditsJava(ctx, idx)
				return
			}
			if flatCallExpressionName(file, idx) != "edit" {
				return
			}
			if args := flatCallKeyArguments(file, idx); args != 0 && file.FlatNamedChildCount(args) > 0 {
				return // `edit(n)` on a Collection/Array — not SharedPreferences.edit().
			}
			if ancestorFinalizesEditor(file, idx) {
				return
			}
			// The KTX `edit { ... }` overload applies the changes itself.
			if call := flatCallTrailingLambda(file, idx); call != 0 {
				return
			}
			if editorChainEscapes(file, idx) {
				return
			}
			fn, ok := flatEnclosingAncestor(file, idx, "function_declaration", "function_body")
			if !ok {
				return
			}
			if editorVar := initializerAssignedName(file, idx); editorVar != "" &&
				(functionHasReceiverCallAfter(file, fn, idx, editorVar, commitOrApplyNames, editorFinalizeCallShape) ||
					variableEscapes(file, fn, editorVar)) {
				return
			}
			// Gate on resolved call-target FQN; falls back to AST evidence only without an oracle.
			var oracleLookup oracle.Lookup
			if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
				oracleLookup = cr.Oracle()
			}
			if !commitPrefEditsOracleConfirmed(oracleLookup, file, idx) {
				return
			}
			ctx.EmitAt(calleeLine(file, idx), calleeCol(file, idx), "`SharedPreferences.edit()` without a corresponding `commit()` or `apply()` call")
		},
	})
}

// commitPrefEditsOracleConfirmed accepts when no oracle is available
// or it resolves the `.edit()` call to `SharedPreferences.edit` or
// one of the androidx.core KTX extensions. A `.edit(n)` on a
// Collection or a project-local `edit` extension resolves to a
// different FQN and is filtered out. When the oracle is available but
// cannot resolve the call, stay silent (do not treat as confirmed).
func commitPrefEditsOracleConfirmed(lookup oracle.Lookup, file *scanner.File, idx uint32) bool {
	if lookup == nil {
		return true
	}
	target := oracleLookupCallTargetFlat(lookup, file, idx)
	if target == "" {
		return false
	}
	return target == "android.content.SharedPreferences.edit" ||
		strings.HasPrefix(target, "androidx.core.content.SharedPreferencesKt.edit") ||
		strings.HasPrefix(target, "androidx.core.content.ContextKt.edit")
}

func registerAndroidCorrectnessCommitTransaction() {
	r := &CommitTransactionRule{AndroidRule: alcRule("CommitTransaction", "Missing commit() on FragmentTransaction", ALSWarning, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression", "method_invocation"},
		Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: api.ConfidenceMediumHigh, Implementation: r,
		Needs:             api.NeedsTypeInfo | api.NeedsOracleCallTargets,
		JavaFacts:         &api.JavaFactProfile{ReceiverTypesForCallees: []string{"beginTransaction"}},
		NeedsLibraryFacts: true,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if file.FlatType(idx) == "method_invocation" {
				checkCommitTransactionJava(ctx, idx)
				return
			}
			if flatCallExpressionName(file, idx) != "beginTransaction" {
				return
			}
			if ancestorCallNameIn(file, idx, commitTransactionNames) {
				return
			}
			if editorChainEscapes(file, idx) {
				return
			}
			fn, ok := flatEnclosingAncestor(file, idx, "function_declaration", "function_body")
			if !ok {
				return
			}
			if txVar := initializerAssignedName(file, idx); txVar != "" &&
				(functionHasReceiverCallAfter(file, fn, idx, txVar, commitTransactionNames, nil) ||
					variableEscapes(file, fn, txVar)) {
				return
			}
			// Gate on resolved call-target FQN; falls back to AST evidence only without an oracle.
			var oracleLookup oracle.Lookup
			if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
				oracleLookup = cr.Oracle()
			}
			if !commitTransactionOracleConfirmed(oracleLookup, file, idx) {
				return
			}
			ctx.EmitAt(calleeLine(file, idx), calleeCol(file, idx), "This transaction should be completed with a `commit()` call")
		},
	})
}

// commitTransactionOracleConfirmed accepts when no oracle is available
// or it resolves `.beginTransaction()` to one of the FragmentManager
// implementations. A project-local `beginTransaction()` (e.g. on a
// database wrapper or domain transactional API) resolves to a
// different FQN and is filtered out. When the oracle is available but
// cannot resolve the call, stay silent (do not treat as confirmed).
func commitTransactionOracleConfirmed(lookup oracle.Lookup, file *scanner.File, idx uint32) bool {
	if lookup == nil {
		return true
	}
	target := oracleLookupCallTargetFlat(lookup, file, idx)
	if target == "" {
		return false
	}
	return target == "androidx.fragment.app.FragmentManager.beginTransaction" ||
		target == "android.app.FragmentManager.beginTransaction" ||
		target == "android.support.v4.app.FragmentManager.beginTransaction"
}

func registerAndroidCorrectnessAssert() {
	r := &AssertRule{AndroidRule: alcRule("Assert", "Assertions are unreliable on Android", ALSWarning, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallExpressionName(file, idx) != "assert" {
				return
			}
			// `kotlin.assert(...)` is the stdlib call; any other receiver is a different method.
			if navExpr, _ := flatCallExpressionParts(file, idx); navExpr != 0 && strings.TrimSpace(file.FlatNodeText(file.FlatNamedChild(navExpr, 0))) != "kotlin" {
				return
			}
			if navExpr, _ := flatCallExpressionParts(file, idx); navExpr == 0 && fileDeclaresFunctionNamed(file, "assert") {
				return
			}
			ctx.EmitAt(callLine(file, idx), callCol(file, idx), "assert is not reliable on Android. Use a proper assertion library or throw explicitly.")
		},
	})
}

func registerAndroidCorrectnessCheckResult() {
	r := &CheckResultRule{AndroidRule: alcRule("CheckResult", "Ignoring results", ALSWarning, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression", "method_invocation"},
		Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: api.ConfidenceMediumHigh, Implementation: r,
		Needs:             api.NeedsTypeInfo | api.NeedsOracleCallTargets,
		JavaFacts:         &api.JavaFactProfile{ReturnTypesForCallees: []string{"animate", "buildUpon", "edit", "format", "trim", "replace"}},
		NeedsLibraryFacts: true,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if file.FlatType(idx) == "method_invocation" {
				checkResultJava(ctx, idx)
				return
			}
			if flatIsUsedAsExpression(file, idx) {
				return
			}
			name := flatCallExpressionName(file, idx)
			if !checkResultCalleeNames[name] {
				return
			}
			// `String.format(...)` is the only one with a required
			// receiver. For the rest, we flag any receiver or none.
			if name == "format" && !isReceiverNamed(file, idx, "String") {
				return
			}
			// The KTX `edit { ... }` overload applies the changes itself and returns Unit.
			if name == "edit" && flatCallTrailingLambda(file, idx) != 0 {
				return
			}
			// Gate on resolved call-target FQN; falls back to AST evidence only without an oracle.
			var oracleLookup oracle.Lookup
			if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
				oracleLookup = cr.Oracle()
			}
			if !checkResultOracleConfirmed(oracleLookup, file, idx, name) {
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "The result of `"+name+"` is not used")
		},
	})
}

// checkResultOracleConfirmed accepts when no oracle is available or
// the resolved call-target FQN matches a known API whose return
// value should not be discarded. Project-local methods with the
// same simple names (e.g. a builder DSL's `replace()` that mutates
// in place) resolve to a different FQN and are filtered out. When the
// oracle is available but cannot resolve the call, stay silent (do not
// treat as confirmed).
func checkResultOracleConfirmed(lookup oracle.Lookup, file *scanner.File, idx uint32, name string) bool {
	if lookup == nil {
		return true
	}
	target := oracleLookupCallTargetFlat(lookup, file, idx)
	if target == "" {
		return false
	}
	expected, ok := checkResultExpectedFQNs[name]
	if !ok {
		return true
	}
	for _, fqn := range expected {
		if target == fqn {
			return true
		}
	}
	return false
}

// checkResultExpectedFQNs maps each watched callee name to the
// concrete library FQN(s) whose ignored return values are bugs.
// Order is the most likely real call first; the list short-circuits
// on first match.
var checkResultExpectedFQNs = map[string][]string{
	"format": {
		"kotlin.text.StringsKt.format",
		"java.lang.String.format",
	},
	"trim": {
		"kotlin.text.StringsKt.trim",
		"kotlin.CharSequence.trim",
		"kotlin.String.trim",
		"java.lang.String.trim",
	},
	"replace": {
		"kotlin.text.StringsKt.replace",
		"kotlin.text.replace",
		"java.lang.String.replace",
	},
	"animate": {
		"android.view.View.animate",
	},
	"buildUpon": {
		"android.net.Uri.buildUpon",
	},
	"edit": {
		"android.content.SharedPreferences.edit",
		"androidx.core.content.SharedPreferencesKt.edit",
		"androidx.core.content.ContextKt.edit",
	},
}

func registerAndroidCorrectnessShiftFlags() {
	r := &ShiftFlagsRule{AndroidRule: alcRule("ShiftFlags", "Suspicious flag constant declarations", ALSWarning, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"property_declaration"}, Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			// Require const modifier.
			if !file.FlatHasModifier(idx, "const") {
				return
			}
			// Identifier must contain "FLAG" (case-insensitive check via uppercase).
			name := extractIdentifierFlat(file, idx)
			upper := strings.ToUpper(name)
			if !strings.Contains(upper, "FLAG") {
				return
			}
			// Only a bare single-bit literal (2, 4, 8, 0x10, ...) is better written as a shift. Zero, one,
			// masks and combinations such as FLAG_ALL = 7 or `A or B` are normal flag definitions.
			var initExpr uint32
			for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) == "=" {
					initExpr = file.FlatNextSib(child)
					break
				}
			}
			if initExpr == 0 {
				return
			}
			switch file.FlatType(initExpr) {
			case "integer_literal", "long_literal", "hex_literal", "bin_literal":
			default:
				return
			}
			value, ok := parseKotlinIntegerLiteral(file.FlatNodeText(initExpr))
			if !ok || value < 2 || value&(value-1) != 0 {
				return
			}
			ctx.EmitAt(declLine(file, idx), declCol(file, idx), "Consider using shift operators (1 shl N) for flag constants for clarity.")
		},
	})
}

func registerAndroidCorrectnessUniqueConstants() {
	r := &UniqueConstantsRule{AndroidRule: alcRule("UniqueConstants", "Overlapping enumeration constants", ALSError, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"annotation"}, Confidence: api.ConfidenceHigher, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			ctor, _ := file.FlatFindChild(idx, "constructor_invocation")
			if ctor == 0 {
				return
			}
			name := annotationConstructorName(file, ctor)
			if name != "IntDef" && name != "StringDef" {
				return
			}
			args, _ := file.FlatFindChild(ctor, "value_arguments")
			if args == 0 {
				return
			}
			seen := make(map[string]bool)
			consts := wrongConstantFileConstants(file)
			// check reports the first constant whose value repeats an earlier one.
			check := func(expr uint32) bool {
				key, display := annotationConstantKey(file, expr)
				if value, ok := wrongConstantEvalIntExpr(file, expr, consts); ok {
					text := strings.TrimSpace(file.FlatNodeText(expr))
					key, display = "v:"+strconv.FormatInt(value, 10), text
					if text != strconv.FormatInt(value, 10) {
						display = text + " (" + strconv.FormatInt(value, 10) + ")"
					}
				}
				if key == "" {
					return false
				}
				if seen[key] {
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Duplicate constant value "+display+" in annotation definition.")
					return true
				}
				seen[key] = true
				return false
			}
			for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
				if file.FlatType(arg) != "value_argument" {
					continue
				}
				expr := flatValueArgumentExpression(file, arg)
				if expr == 0 {
					continue
				}
				// `value = [A, B]` and `[A, B]` list their constants in an array literal.
				if file.FlatType(expr) == "collection_literal" || file.FlatType(expr) == "array_literal" {
					for elem := file.FlatFirstChild(expr); elem != 0; elem = file.FlatNextSib(elem) {
						if file.FlatIsNamed(elem) && check(elem) {
							return
						}
					}
					continue
				}
				if check(expr) {
					return
				}
			}
		},
	})
}

func registerAndroidCorrectnessWrongThread() {
	r := &WrongThreadRule{AndroidRule: alcRule("WrongThread", "Wrong thread", ALSError, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration"}, Needs: api.NeedsTypeInfo, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			caller := threadContextOfFunction(file, idx)
			if caller == threadNone {
				return
			}
			annotated := sameFileThreadAnnotatedFunctions(file)
			file.FlatWalkNodes(idx, "call_expression", func(callIdx uint32) {
				if wrongThreadCallIsDispatched(file, callIdx, idx) {
					return
				}
				name := flatCallExpressionName(file, callIdx)
				// A bare call resolves against the same file; a qualified call is only trusted for view methods.
				bare := flatReceiverNameFromCall(file, callIdx) == "" && !callHasNavigationReceiver(file, callIdx)
				callee := annotated[name]
				switch {
				case caller == threadWorker && bare && callee == threadMain:
					ctx.EmitAt(file.FlatRow(callIdx)+1, file.FlatCol(callIdx)+1,
						"Method "+name+" must be called from the main thread, currently inferred thread is worker (@WorkerThread).")
				case caller == threadMain && bare && callee == threadWorker:
					ctx.EmitAt(file.FlatRow(callIdx)+1, file.FlatCol(callIdx)+1,
						"Method "+name+" must be called from a worker thread, currently inferred thread is main (@MainThread or @UiThread).")
				case caller == threadWorker && wrongThreadUIMethods[name] && !bare && wrongThreadReceiverMayBeView(ctx, callIdx, name):
					ctx.EmitAt(file.FlatRow(callIdx)+1, file.FlatCol(callIdx)+1, "UI operation in @WorkerThread context. Use runOnUiThread or Handler.post().")
				}
			})
		},
	})
}

type threadContext int

const (
	threadNone threadContext = iota
	threadMain
	threadWorker
)

// wrongThreadUIMethods are View methods that must run on the main thread.
var wrongThreadUIMethods = map[string]bool{
	"setText": true, "setImageResource": true, "setVisibility": true,
	"addView": true, "removeView": true, "invalidate": true,
}

// threadContextOfFunction maps @MainThread/@UiThread to main and @WorkerThread to worker.
func threadContextOfFunction(file *scanner.File, fn uint32) threadContext {
	switch {
	case hasAnnotationNamed(file, fn, "WorkerThread"):
		return threadWorker
	case hasAnnotationNamed(file, fn, "MainThread"), hasAnnotationNamed(file, fn, "UiThread"):
		return threadMain
	}
	return threadNone
}

// sameFileThreadAnnotatedFunctions maps function names declared in this file to their thread annotation.
// A name declared with conflicting annotations is dropped because overloads cannot be told apart.
func sameFileThreadAnnotatedFunctions(file *scanner.File) map[string]threadContext {
	out := map[string]threadContext{}
	conflicts := map[string]bool{}
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		name := extractIdentifierFlat(file, fn)
		if name == "" {
			return
		}
		ctxt := threadContextOfFunction(file, fn)
		if prev, seen := out[name]; seen && prev != ctxt {
			conflicts[name] = true
		}
		out[name] = ctxt
	})
	for name := range conflicts {
		delete(out, name)
	}
	return out
}

// wrongThreadCallIsDispatched reports whether the call sits inside a lambda handed to the main thread.
func wrongThreadCallIsDispatched(file *scanner.File, call, fn uint32) bool {
	for p, ok := file.FlatParent(call); ok && p != fn; p, ok = file.FlatParent(p) {
		if file.FlatType(p) != "call_expression" {
			continue
		}
		switch flatCallExpressionName(file, p) {
		case "runOnUiThread", "post", "postDelayed", "postOnAnimation":
			return true
		}
		if strings.Contains(file.FlatNodeText(p), "Dispatchers.Main") && flatCallTrailingLambda(file, p) != 0 {
			return true
		}
	}
	return false
}

func callHasNavigationReceiver(file *scanner.File, call uint32) bool {
	navExpr, _ := flatCallExpressionParts(file, call)
	return navExpr != 0
}

// wrongThreadReceiverMayBeView accepts a call whose receiver resolves to an android.view or android.widget
// type. An unresolved receiver is accepted for the distinctly view-like methods, but not for `invalidate`,
// which caches and similar classes also declare.
func wrongThreadReceiverMayBeView(ctx *api.Context, call uint32, method string) bool {
	if target, ok := semantics.ResolveCallTarget(ctx, call); ok && target.Receiver.Valid() {
		if typ, ok := semantics.ExpressionType(ctx, target.Receiver.Node); ok && typ.Type != nil && typ.Type.Kind != typeinfer.TypeUnknown {
			return strings.HasPrefix(typ.Type.FQN, "android.view.") || strings.HasPrefix(typ.Type.FQN, "android.widget.") ||
				typeinfer.IsKnownSubtype(typ.Type.FQN, "android.view.View")
		}
	}
	return method != "invalidate"
}

// sqliteStringTypeRe is SQLiteDetector's `.*\bSTRING\b.*`: a case-sensitive whole word.
var sqliteStringTypeRe = regexp.MustCompile(`\bSTRING\b`)

const sqliteStringMessage = "Using column type STRING; did you mean to use TEXT? (STRING is a numeric type and its value can be adjusted; for example, strings that look like integers can drop leading zeroes. See issue explanation for details.)"

// sqliteSQLText resolves the SQL text of an execSQL argument the way Lint's constant evaluation does for
// this check: a literal, a literal followed by concatenated parts, or a same-file property initialized with one.
func sqliteSQLText(file *scanner.File, expr uint32, depth int) (string, bool) {
	if expr == 0 || depth > 3 {
		return "", false
	}
	switch file.FlatType(expr) {
	case "string_literal":
		if flatContainsStringInterpolation(file, expr) {
			return "", false
		}
		return stringLiteralContent(file, expr), true
	case "additive_expression":
		return sqliteSQLText(file, file.FlatFirstChild(expr), depth+1)
	case "simple_identifier":
		name := file.FlatNodeText(expr)
		var text string
		found := false
		file.FlatWalkNodes(0, "property_declaration", func(decl uint32) {
			if found || composePropertyNameFlat(file, decl) != name {
				return
			}
			text, found = sqliteSQLText(file, propertyInitializerExpression(file, decl), depth+1)
		})
		return text, found
	}
	return "", false
}

func registerAndroidCorrectnessSQLiteString() {
	r := &SQLiteStringRule{AndroidRule: alcRule("SQLiteString", "Using STRING instead of TEXT", ALSWarning, 5)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallExpressionName(file, idx) != "execSQL" {
				return
			}
			arg := flatPositionalValueArgument(file, flatCallKeyArguments(file, idx), 0)
			expr := flatValueArgumentExpression(file, arg)
			text, ok := sqliteSQLText(file, expr, 0)
			if !ok || !(strings.HasPrefix(text, "CREATE TABLE") || strings.HasPrefix(text, "ALTER TABLE")) {
				return
			}
			if sqliteStringTypeRe.MatchString(text) {
				ctx.EmitAt(file.FlatRow(expr)+1, file.FlatCol(expr)+1, sqliteStringMessage)
			}
		},
	})
}

func registerAndroidCorrectnessRegistered() {
	r := &RegisteredRule{AndroidRule: alcRule("Registered", "Class is not registered in the manifest", ALSWarning, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration"}, Needs: api.NeedsResolver, TypeInfo: api.TypeInfoHint{PreferBackend: api.PreferResolver, Required: true}, Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			// Test-only components are registered by test manifests.
			if scanner.IsTestFile(file.Path) {
				return
			}
			componentType, confidence := androidComponentType(file, idx, ctx.Resolver)
			if componentType == "" {
				return
			}
			className := extractIdentifierFlat(file, idx)
			if className == "" {
				className = "This class"
			}
			ctx.Emit(scanner.Finding{
				Line:       file.FlatRow(idx) + 1,
				Col:        1,
				Message:    formatRegisteredMsg(className, componentType),
				Confidence: confidence,
			})
		},
	})
}

func registerAndroidCorrectnessScrollViewCount() {
	r := &ScrollViewCountRule{AndroidRule: alcRule("ScrollViewCount", "ScrollViews can have only one child", ALSWarning, 7)}
	scrollViewCtorNames := map[string]bool{"ScrollView": true, "HorizontalScrollView": true}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallExpressionName(file, idx) != "apply" {
				return
			}
			navExpr, _ := flatCallExpressionParts(file, idx)
			if navExpr == 0 {
				return
			}
			receiver := file.FlatNamedChild(navExpr, 0)
			if receiver == 0 || file.FlatType(receiver) != "call_expression" {
				return
			}
			ctorName := flatCallExpressionName(file, receiver)
			if !scrollViewCtorNames[ctorName] {
				return
			}
			lambda := flatCallTrailingLambda(file, idx)
			if lambda == 0 {
				return
			}
			statements, _ := file.FlatFindChild(lambda, "statements")
			if statements == 0 {
				return
			}
			addViewCalls := 0
			for stmt := file.FlatFirstChild(statements); stmt != 0; stmt = file.FlatNextSib(stmt) {
				if file.FlatType(stmt) != "call_expression" {
					continue
				}
				if flatCallExpressionName(file, stmt) == "addView" {
					addViewCalls++
				}
			}
			if addViewCalls <= 1 {
				return
			}
			ctx.EmitAt(file.FlatRow(receiver)+1, file.FlatCol(receiver)+1,
				ctorName+" can host only one direct child, but "+
					"this apply block adds "+strconv.Itoa(addViewCalls)+" views.")
		},
	})
}

func registerAndroidCorrectnessSimpleDateFormat() {
	r := &SimpleDateFormatRule{AndroidRule: alcRule("SimpleDateFormat", "Using SimpleDateFormat directly without Locale", ALSWarning, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes:  []string{"call_expression", "object_creation_expression"},
		Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if file.FlatType(idx) == "object_creation_expression" {
				checkSimpleDateFormatJava(ctx, idx)
				return
			}
			if flatCallExpressionName(file, idx) != "SimpleDateFormat" || !simpleDateFormatIsJavaText(file, idx) {
				return
			}
			// Require at least two args (pattern + Locale). Single-arg form uses default locale.
			_, args := flatCallExpressionParts(file, idx)
			if args == 0 {
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "To get local formatting use `getDateInstance()`, `getDateTimeInstance()`, or `getTimeInstance()`, or use `new SimpleDateFormat(String template, Locale locale)` with for example `Locale.US` for ASCII dates.")
				return
			}
			count := 0
			for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
				if file.FlatType(arg) == "value_argument" {
					count++
				}
			}
			if count < 2 {
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "To get local formatting use `getDateInstance()`, `getDateTimeInstance()`, or `getTimeInstance()`, or use `new SimpleDateFormat(String template, Locale locale)` with for example `Locale.US` for ASCII dates.")
			}
		},
	})
}

// simpleDateFormatIsJavaText rejects a qualified call outside java.text and a bare call whose
// SimpleDateFormat is imported from another package.
func simpleDateFormatIsJavaText(file *scanner.File, call uint32) bool {
	if navExpr, _ := flatCallExpressionParts(file, call); navExpr != 0 {
		return strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(navExpr)), "java.text.")
	}
	if imp, ok := fileFactsCache().Imports(file).Aliases["SimpleDateFormat"]; ok {
		return imp == "java.text.SimpleDateFormat"
	}
	return true
}

func registerAndroidCorrectnessSetTextI18n() {
	r := &SetTextI18nRule{AndroidRule: alcRule("SetTextI18n", "TextView with internationalization issues", ALSWarning, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes:  []string{"call_expression"},
		Needs:      api.NeedsTypeInfo,
		Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallExpressionName(file, idx) != "setText" {
				return
			}
			// Lint only flags literal text with a run of at least two ASCII word
			// characters; interpolation expressions are excluded from the check.
			_, args := flatCallExpressionParts(file, idx)
			if args == 0 {
				return
			}
			firstArg := flatPositionalValueArgument(file, args, 0)
			if firstArg == 0 {
				return
			}
			hasLiteralText := false
			for expr := file.FlatFirstChild(firstArg); expr != 0; expr = file.FlatNextSib(expr) {
				if !file.FlatIsNamed(expr) {
					continue
				}
				hasLiteralText = setTextI18nLiteralHasWordRun(file, expr)
				break
			}
			if !hasLiteralText {
				return
			}
			if !setTextI18nReceiverIsTextView(ctx, idx) {
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Do not pass hardcoded text to setText. Use resource strings with placeholders.")
		},
	})
}

// setTextI18nReceiverIsTextView reports whether the receiver of a setText
// call_expression is a TextView (or a likely TextView subtype). It refuses
// to fire when the receiver chain starts with a known non-View symbol
// (NotificationCompat, RemoteViews, ...) and skips when no receiver
// evidence is available.
func setTextI18nReceiverIsTextView(ctx *api.Context, call uint32) bool {
	file := ctx.File
	navExpr, _ := flatCallExpressionParts(file, call)
	if navExpr == 0 {
		// Bare `setText("...")` — only flag when the enclosing class is
		// a TextView subtype. Otherwise we have no receiver evidence.
		classIdx, ok := flatEnclosingAncestor(file, call, "class_declaration", "object_declaration")
		if !ok || classIdx == 0 {
			return false
		}
		for _, super := range androidDirectSupertypesFlat(file, classIdx) {
			if super.simple == "TextView" ||
				super.simple == "Button" ||
				super.simple == "EditText" ||
				super.simple == "AppCompatTextView" ||
				super.simple == "AppCompatEditText" ||
				super.simple == "AppCompatButton" ||
				super.simple == "MaterialButton" ||
				super.simple == "TextInputEditText" ||
				super.name == "android.widget.TextView" {
				return true
			}
			if ctx.Resolver != nil {
				fqn := super.name
				if !super.qualified {
					if resolved := ctx.Resolver.ResolveImport(super.simple, file); resolved != "" {
						fqn = resolved
					}
				}
				typ := &typeinfer.ResolvedType{Name: super.simple, FQN: fqn, Kind: typeinfer.TypeClass}
				if androidTypeIsTextViewSubtype(ctx.Resolver, typ) {
					return true
				}
			}
		}
		return false
	}
	// Navigation receiver: walk down to the leftmost identifier.
	recv := file.FlatNamedChild(navExpr, 0)
	if recv == 0 {
		return false
	}
	// Hard skip: receiver chain rooted at a known non-View symbol.
	if androidReceiverIsKnownNonView(file, recv) {
		return false
	}
	// Receiver is `this`/`super`: check enclosing class.
	switch file.FlatType(recv) {
	case "this_expression", "super_expression":
		classIdx, ok := flatEnclosingAncestor(file, call, "class_declaration", "object_declaration")
		if !ok || classIdx == 0 {
			return false
		}
		for _, super := range androidDirectSupertypesFlat(file, classIdx) {
			if super.simple == "TextView" || super.simple == "Button" ||
				super.simple == "EditText" || super.simple == "AppCompatTextView" ||
				super.simple == "AppCompatEditText" || super.simple == "AppCompatButton" ||
				super.simple == "MaterialButton" || super.simple == "TextInputEditText" ||
				super.name == "android.widget.TextView" {
				return true
			}
		}
		return false
	}
	if ctx.Resolver != nil {
		typ := ctx.Resolver.ResolveFlatNode(recv, file)
		if typ != nil && typ.Name != "" {
			if androidTypeIsTextViewSubtype(ctx.Resolver, typ) {
				return true
			}
			// Receiver resolved to a non-TextView type — refuse to fire.
			return false
		}
	}
	// Heuristic: if the trailing identifier of the chain matches a
	// TextView simple name, accept (covers `binding.textView.setText("...")`).
	// The receiver expression IS the navigation chain whose tail names
	// the field/variable holding the call target.
	field := setTextI18nReceiverFieldName(file, recv)
	if field != "" {
		lower := strings.ToLower(field)
		if strings.Contains(lower, "textview") || strings.Contains(lower, "edittext") ||
			strings.Contains(lower, "button") || strings.HasSuffix(lower, "text") ||
			lower == "tv" || lower == "et" || lower == "btn" {
			return true
		}
	}
	// Truly unknown — skip rather than false-positive.
	return false
}

// setTextI18nReceiverFieldName returns the trailing identifier of the
// receiver subtree (the field/variable holding the call target). For
// `binding.helloTextView` the receiver expression is the navigation
// expression itself and the trailing identifier is "helloTextView".
func setTextI18nReceiverFieldName(file *scanner.File, recv uint32) string {
	switch file.FlatType(recv) {
	case "simple_identifier":
		return file.FlatNodeText(recv)
	case "navigation_expression":
		return flatNavigationExpressionLastIdentifier(file, recv)
	}
	return ""
}

func registerAndroidCorrectnessStopShip() {
	r := &StopShipRule{AndroidRule: alcRule("StopShip", "STOPSHIP comment found", ALSFatal, 10)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		Needs: api.NeedsLinePass, Confidence: r.Confidence(), Implementation: r,
		Check: r.check,
	})
}

func registerAndroidCorrectnessWrongCall() {
	r := &WrongCallRule{AndroidRule: alcRule("WrongCall", "Using wrong draw/layout method", ALSError, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes:  []string{"call_expression"},
		Needs:      api.NeedsTypeInfo,
		Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			name := flatCallExpressionName(file, idx)
			if name != "onDraw" && name != "onMeasure" && name != "onLayout" {
				return
			}
			// Must have a navigation receiver (i.e. obj.onDraw()), not a bare call or override.
			navExpr, _ := flatCallExpressionParts(file, idx)
			if navExpr == 0 {
				return
			}
			// Skip if the receiver is "super".
			recv := file.FlatFirstChild(navExpr)
			if recv != 0 && file.FlatNodeText(recv) == "super" {
				return
			}
			// A same-named override may legitimately forward to another receiver.
			if fn, inFn := flatEnclosingAncestor(file, idx, "function_declaration"); inFn &&
				file.FlatHasModifier(fn, "override") && extractIdentifierFlat(file, fn) == name {
				return
			}
			// Receiver-type proof: require positive evidence the call is
			// a View method. Either the receiver resolves to a View
			// subtype, or the enclosing class extends View (so the
			// `obj.onDraw()` call really is dispatching to a View method
			// implementation). Without proof, refuse to fire — a method
			// named `onDraw` on a non-View class is an unrelated callback.
			recvNamed := file.FlatNamedChild(navExpr, 0)
			if recvNamed != 0 {
				// Hard skip: receiver chain rooted at a known non-View
				// symbol (NotificationCompat, RemoteViews, ...).
				if androidReceiverIsKnownNonView(file, recvNamed) {
					return
				}
				if ctx.Resolver != nil {
					typ := ctx.Resolver.ResolveFlatNode(recvNamed, file)
					if typ != nil && typ.Name != "" {
						if !androidTypeIsViewSubtype(ctx.Resolver, typ) {
							return
						}
						ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Suspicious method call; should probably call draw/measure/layout instead of "+name+".")
						return
					}
				}
			}
			// Fallback: require the enclosing class extends View.
			if !androidEnclosingClassExtendsView(file, idx, ctx.Resolver) {
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Suspicious method call; should probably call draw/measure/layout instead of "+name+".")
		},
	})
}

func checkDefaultLocaleJava(ctx *api.Context, idx uint32) {
	file := ctx.File
	name := wrongViewCastCallName(file, idx)
	args := wrongViewCastCallArgumentExpressions(file, idx)
	switch name {
	case "toLowerCase", "toUpperCase":
		// String.toLowerCase(Locale) is the only overload with an argument; Character.toLowerCase(c) is not locale-based.
		if len(args) == 0 {
			at := javaCallNameNode(file, idx)
			if at == 0 {
				at = idx
			}
			ctx.EmitAt(file.FlatRow(at)+1, file.FlatCol(at)+1, defaultLocaleCaseMessage(name))
		}
	case "format":
		receiver := wrongViewCastCallReceiverName(file, idx)
		if receiver != "String" && receiver != "java.lang.String" {
			receiver = javaMethodReceiverText(file, idx)
		}
		if receiver != "String" && receiver != "java.lang.String" {
			return
		}
		if len(args) == 0 {
			return
		}
		if javaExprMentionsLocale(file, args[0]) {
			return
		}
		ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, defaultLocaleFormatMessage)
	}
}

func checkSimpleDateFormatJava(ctx *api.Context, idx uint32) {
	file := ctx.File
	typeName := javaObjectCreationTypeName(file, idx)
	if typeName != "SimpleDateFormat" && typeName != "java.text.SimpleDateFormat" {
		return
	}
	if typeName == "SimpleDateFormat" && !sourceImportsOrMentions(file, "java.text.SimpleDateFormat") {
		return
	}
	args := javaArgumentExpressions(file, idx)
	if len(args) < 2 {
		ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "To get local formatting use `getDateInstance()`, `getDateTimeInstance()`, or `getTimeInstance()`, or use `new SimpleDateFormat(String template, Locale locale)` with for example `Locale.US` for ASCII dates.")
	}
}

func javaObjectCreationTypeName(file *scanner.File, idx uint32) string {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "scoped_type_identifier", "scoped_identifier", "generic_type":
			text := strings.TrimSpace(file.FlatNodeText(child))
			if i := strings.IndexByte(text, '<'); i >= 0 {
				text = text[:i]
			}
			return text
		case "argument_list", "class_body":
			return ""
		}
	}
	return ""
}

func javaArgumentExpressions(file *scanner.File, idx uint32) []uint32 {
	argsNode, ok := file.FlatFindChild(idx, "argument_list")
	if !ok {
		return nil
	}
	var args []uint32
	for child := file.FlatFirstChild(argsNode); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			args = append(args, child)
		}
	}
	return args
}

func javaExprMentionsLocale(file *scanner.File, idx uint32) bool {
	return exprIdentifierPartsContainLocale(file, idx)
}

func kotlinExprIsLocaleReference(file *scanner.File, idx uint32) bool {
	return exprIdentifierPartsContainLocale(file, idx)
}

func exprIdentifierPartsContainLocale(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		if found {
			return
		}
		switch file.FlatType(candidate) {
		case "simple_identifier", "identifier", "scoped_identifier":
			if file.FlatNodeText(candidate) == "Locale" {
				found = true
			}
		}
	})
	return found
}

func checkCommitPrefEditsJava(ctx *api.Context, idx uint32) {
	file := ctx.File
	if javaMethodInvocationName(file, idx) != "edit" {
		return
	}
	if len(javaArgumentExpressions(file, idx)) != 0 {
		return
	}
	if fact, ok := javaSemanticCallFact(ctx, idx); ok && fact.ReceiverType != "" {
		if !javaProfileTypeMatches(ctx, fact.ReceiverType, "android.content.SharedPreferences") {
			return
		}
	} else if !sourceImportsOrMentions(file, "android.content.SharedPreferences") {
		return
	}
	if javaAncestorCallNameMatches(file, idx, commitOrApplyNames) {
		return
	}
	fn, ok := flatEnclosingAncestor(file, idx, "method_declaration", "function_declaration")
	if !ok {
		return
	}
	if editorVar := javaLocalInitializerAssignedName(file, idx); editorVar != "" &&
		javaFunctionHasReceiverCallAfter(file, fn, idx, editorVar, commitOrApplyNames) {
		return
	}
	ctx.EmitAt(calleeLine(file, idx), calleeCol(file, idx), "`SharedPreferences.edit()` without a corresponding `commit()` or `apply()` call")
}

func checkCommitTransactionJava(ctx *api.Context, idx uint32) {
	file := ctx.File
	if javaMethodInvocationName(file, idx) != "beginTransaction" {
		return
	}
	if fact, ok := javaSemanticCallFact(ctx, idx); ok && fact.ReceiverType != "" {
		if !javaProfileTypeMatches(ctx, fact.ReceiverType,
			"android.app.FragmentManager",
			"androidx.fragment.app.FragmentManager") {
			return
		}
	} else if !sourceImportsOrMentions(file, "android.app.FragmentTransaction") &&
		!sourceImportsOrMentions(file, "androidx.fragment.app.FragmentTransaction") &&
		!sourceImportsOrMentions(file, "android.app.FragmentManager") &&
		!sourceImportsOrMentions(file, "androidx.fragment.app.FragmentManager") {
		return
	}
	if javaAncestorCallNameMatches(file, idx, commitTransactionNames) {
		return
	}
	fn, ok := flatEnclosingAncestor(file, idx, "method_declaration", "function_declaration")
	if !ok {
		return
	}
	if txVar := javaLocalInitializerAssignedName(file, idx); txVar != "" {
		if javaFunctionHasReceiverCallAfter(file, fn, idx, txVar, commitTransactionNames) || javaVariableEscapes(file, fn, txVar) {
			return
		}
	} else if javaChainEscapes(file, idx) {
		return
	}
	ctx.EmitAt(calleeLine(file, idx), calleeCol(file, idx), "This transaction should be completed with a `commit()` call")
}

func checkResultJava(ctx *api.Context, idx uint32) {
	file := ctx.File
	if !javaMethodInvocationResultIsIgnored(file, idx) {
		return
	}
	name := javaMethodInvocationName(file, idx)
	if !checkResultCalleeNames[name] {
		return
	}
	if ok, known := checkResultJavaSemanticConfirmed(ctx, idx, name); known && !ok {
		return
	}
	if name == "format" {
		receiver := wrongViewCastCallReceiverName(file, idx)
		if receiver != "String" && receiver != "java.lang.String" {
			return
		}
	}
	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "The result of `"+name+"` is not used")
}

func checkResultJavaSemanticConfirmed(ctx *api.Context, idx uint32, name string) (bool, bool) {
	fact, ok := javaSemanticCallFact(ctx, idx)
	if !ok {
		return false, false
	}
	if fact.ReturnType == "" {
		return false, false
	}
	if expected := javaProfileMethodReturn(ctx, fact.MethodOwner, fact.ReceiverType, name, len(javaArgumentExpressions(ctx.File, idx))); expected != "" {
		return javaProfileTypeMatches(ctx, fact.ReturnType, expected), true
	}
	switch name {
	case "edit":
		return javaProfileTypeMatches(ctx, fact.ReturnType, "android.content.SharedPreferences.Editor"), true
	case "format", "trim", "replace":
		return javaProfileTypeMatches(ctx, fact.ReturnType, "java.lang.String"), true
	case "buildUpon":
		return strings.HasSuffix(strings.ReplaceAll(fact.ReturnType, "$", "."), ".Builder") ||
			javaProfileTypeMatches(ctx, fact.ReturnType, "Builder"), true
	case "animate":
		return javaProfileTypeMatches(ctx, fact.ReturnType, "android.view.ViewPropertyAnimator"), true
	default:
		return false, false
	}
}

func javaMethodInvocationResultIsIgnored(file *scanner.File, idx uint32) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "expression_statement":
			return true
		case "parenthesized_expression":
			continue
		default:
			return false
		}
	}
	return false
}

func isJavaBooleanTrue(file *scanner.File, idx uint32) bool {
	idx = flatUnwrapParenExpr(file, idx)
	text := strings.TrimSpace(file.FlatNodeText(idx))
	return text == "true"
}

func javaMethodReceiverText(file *scanner.File, call uint32) string {
	if file == nil || file.FlatType(call) != "method_invocation" {
		return ""
	}
	var parts []string
	for child := file.FlatFirstChild(call); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "argument_list" {
			break
		}
		if !file.FlatIsNamed(child) || file.FlatType(child) == "identifier" {
			continue
		}
		parts = append(parts, strings.TrimSpace(file.FlatNodeText(child)))
	}
	if len(parts) > 0 {
		return strings.Join(parts, ".")
	}
	text := strings.TrimSpace(file.FlatNodeText(call))
	open := strings.LastIndex(text, "(")
	if open < 0 {
		return ""
	}
	beforeCall := strings.TrimSpace(text[:open])
	dot := strings.LastIndex(beforeCall, ".")
	if dot < 0 {
		return ""
	}
	return strings.TrimSpace(beforeCall[:dot])
}

func javaAncestorCallNameMatches(file *scanner.File, idx uint32, names map[string]bool) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "call_expression", "method_invocation":
			if names[javaAwareCallName(file, parent)] {
				return true
			}
		case "method_declaration", "function_declaration", "source_file":
			return false
		}
	}
	return false
}

func javaAwareCallName(file *scanner.File, idx uint32) string {
	switch file.FlatType(idx) {
	case "call_expression":
		return flatCallExpressionName(file, idx)
	case "method_invocation":
		return javaMethodInvocationName(file, idx)
	default:
		return ""
	}
}

func javaMethodInvocationName(file *scanner.File, call uint32) string {
	if file == nil || file.FlatType(call) != "method_invocation" {
		return ""
	}
	if name := wrongViewCastCallName(file, call); name != "" {
		return name
	}
	text := strings.TrimSpace(file.FlatNodeText(call))
	open := strings.LastIndex(text, "(")
	if open >= 0 {
		beforeCall := strings.TrimSpace(text[:open])
		if dot := strings.LastIndex(beforeCall, "."); dot >= 0 {
			return strings.TrimSpace(beforeCall[dot+1:])
		}
		fields := strings.Fields(beforeCall)
		if len(fields) > 0 {
			return fields[len(fields)-1]
		}
	}
	return ""
}

func javaLocalInitializerAssignedName(file *scanner.File, idx uint32) string {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "variable_declarator":
			name, ok := file.FlatFindChild(current, "identifier")
			if !ok {
				return ""
			}
			return file.FlatNodeText(name)
		case "local_variable_declaration", "method_declaration", "class_declaration", "source_file":
			return ""
		}
	}
	return ""
}

func javaFunctionHasReceiverCallAfter(file *scanner.File, fn, target uint32, receiverName string, names map[string]bool) bool {
	if file == nil || fn == 0 || target == 0 || receiverName == "" {
		return false
	}
	targetStart := file.FlatStartByte(target)
	found := false
	file.FlatWalkNodes(fn, "method_invocation", func(call uint32) {
		if found || call == target || file.FlatStartByte(call) < targetStart {
			return
		}
		if !names[javaMethodInvocationName(file, call)] {
			return
		}
		if wrongViewCastCallReceiverName(file, call) == receiverName {
			found = true
		}
	})
	return found
}

func javaEnclosingFunctionHasCallNamed(file *scanner.File, fn, except uint32, names map[string]bool) bool {
	if file == nil || fn == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(fn, "method_invocation", func(call uint32) {
		if found || call == except {
			return
		}
		if names[javaMethodInvocationName(file, call)] {
			found = true
		}
	})
	return found
}

// javaVariableEscapes reports whether a local variable is returned, passed as an argument or stored,
// in which case another scope may complete it.
func javaVariableEscapes(file *scanner.File, fn uint32, name string) bool {
	escapes := false
	file.FlatWalkNodes(fn, "identifier", func(id uint32) {
		if escapes || !file.FlatNodeTextEquals(id, name) {
			return
		}
		parent, ok := file.FlatParent(id)
		if !ok {
			return
		}
		switch file.FlatType(parent) {
		case "return_statement", "argument_list":
			escapes = true
		case "variable_declarator", "assignment_expression":
			escapes = file.FlatFirstChild(parent) != id
		}
	})
	return escapes
}

// javaChainEscapes reports whether the receiver chain starting at a call is returned, passed on or stored.
func javaChainEscapes(file *scanner.File, idx uint32) bool {
	top := idx
	for {
		parent, ok := file.FlatParent(top)
		if !ok {
			return false
		}
		switch file.FlatType(parent) {
		case "method_invocation":
			if file.FlatFirstChild(parent) != top && !javaIsChainReceiver(file, parent, top) {
				return true // an argument of another call
			}
			top = parent
		case "return_statement", "argument_list", "assignment_expression", "variable_declarator", "field_declaration":
			return true
		default:
			return false
		}
	}
}

// javaIsChainReceiver reports whether child is the receiver (object) part of a method invocation.
func javaIsChainReceiver(file *scanner.File, call, child uint32) bool {
	for c := file.FlatFirstChild(call); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "argument_list" {
			return false
		}
		if c == child {
			return true
		}
	}
	return false
}
