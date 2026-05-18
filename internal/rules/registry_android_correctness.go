package rules

import (
	"strconv"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
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
	registerAndroidCorrectnessNestedScrolling()
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
		Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if file.FlatType(idx) == "method_invocation" {
				checkDefaultLocaleJava(ctx, idx)
				return
			}
			name := flatCallExpressionName(file, idx)
			switch name {
			case "lowercase", "uppercase":
				// Kotlin's modern lowercase()/uppercase() overloads are
				// locale-invariant when called without a Locale. The
				// deprecated toLowerCase()/toUpperCase() forms are the
				// default-locale APIs this Android lint check targets.
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
			case "toLowerCase", "toUpperCase":
				// Only flag the zero-argument form that uses the default locale.
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, "Implicitly using the default locale. Use lowercase(Locale) or uppercase(Locale) instead.")
					return
				}
				// Has value_arguments; skip only when the argument is a
				// structural Locale reference, not a text lookalike.
				for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
					if file.FlatType(arg) != "value_argument" {
						continue
					}
					if kotlinExprIsLocaleReference(file, flatValueArgumentExpression(file, arg)) {
						return
					}
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, "Implicitly using the default locale. Use lowercase(Locale) or uppercase(Locale) instead.")
			case "format":
				// String.format(pattern, ...) without an explicit Locale first arg.
				navExpr, args := flatCallExpressionParts(file, idx)
				if navExpr == 0 || args == 0 {
					return
				}
				// Require receiver to be "String".
				recv := file.FlatFirstChild(navExpr)
				if recv == 0 || file.FlatNodeText(recv) != "String" {
					return
				}
				// If first arg is a Locale navigation expression, skip.
				firstArg := flatPositionalValueArgument(file, args, 0)
				if firstArg == 0 {
					return
				}
				if kotlinExprIsLocaleReference(file, flatValueArgumentExpression(file, firstArg)) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, "Implicitly using the default locale. Use String.format(Locale, ...) instead.")
			}
		},
	})
}

func registerAndroidCorrectnessCommitPrefEdits() {
	r := &CommitPrefEditsRule{AndroidRule: alcRule("CommitPrefEdits", "Missing commit() on SharedPreferences editor", ALSWarning, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression", "method_invocation"},
		Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: 0.8, Implementation: r,
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
			fn, ok := flatEnclosingAncestor(file, idx, "function_declaration", "function_body")
			if !ok {
				return
			}
			if editorVar := initializerAssignedName(file, idx); editorVar != "" &&
				functionHasReceiverCallAfter(file, fn, idx, editorVar, commitOrApplyNames, editorFinalizeCallShape) {
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, 1, "SharedPreferences.edit() without commit() or apply().")
		},
	})
}

func registerAndroidCorrectnessCommitTransaction() {
	r := &CommitTransactionRule{AndroidRule: alcRule("CommitTransaction", "Missing commit() on FragmentTransaction", ALSWarning, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression", "method_invocation"},
		Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: 0.8, Implementation: r,
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
			fn, ok := flatEnclosingAncestor(file, idx, "function_declaration", "function_body")
			if !ok {
				return
			}
			if txVar := initializerAssignedName(file, idx); txVar != "" &&
				functionHasReceiverCallAfter(file, fn, idx, txVar, commitTransactionNames, nil) {
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, 1, "FragmentTransaction without commit(). Call commit() or commitAllowingStateLoss().")
		},
	})
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
			ctx.EmitAt(file.FlatRow(idx)+1, 1, "assert is not reliable on Android. Use a proper assertion library or throw explicitly.")
		},
	})
}

func registerAndroidCorrectnessCheckResult() {
	r := &CheckResultRule{AndroidRule: alcRule("CheckResult", "Ignoring results", ALSWarning, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression", "method_invocation"},
		Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: 0.8, Implementation: r,
		JavaFacts:         &api.JavaFactProfile{ReturnTypesForCallees: []string{"animate", "buildUpon", "edit", "format", "trim", "replace"}},
		NeedsLibraryFacts: true,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if strings.HasSuffix(file.Path, ".gradle.kts") {
				return
			}
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
			ctx.EmitAt(file.FlatRow(idx)+1, 1,
				"The result of this call is not used. Check if the return value should be consumed.")
		},
	})
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
			// Walk to the initializer expression — must be an integer literal
			// with no shl operator or << in the subtree.
			var initExpr uint32
			for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
				t := file.FlatType(child)
				if t == "integer_literal" || t == "long_literal" || t == "hex_literal" {
					initExpr = child
					break
				}
				// Skip past "=" sign to find the rhs
				if t == "=" {
					next := file.FlatNextSib(child)
					if next != 0 {
						initExpr = next
					}
					break
				}
			}
			if initExpr == 0 {
				return
			}
			// Check the initializer subtree for shl or << operators.
			hasShl := false
			file.FlatWalkAllNodes(initExpr, func(n uint32) {
				if hasShl {
					return
				}
				t := file.FlatType(n)
				if t == "multiplicative_operator" || t == "infixFunctionCall" || t == "infix_expression" || t == "simple_identifier" {
					text := file.FlatNodeText(n)
					if text == "shl" || strings.Contains(text, "<<") {
						hasShl = true
					}
				}
			})
			if hasShl {
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, 1, "Consider using shift operators (1 shl N) for flag constants for clarity.")
		},
	})
}

func registerAndroidCorrectnessUniqueConstants() {
	r := &UniqueConstantsRule{AndroidRule: alcRule("UniqueConstants", "Overlapping enumeration constants", ALSError, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"annotation"}, Confidence: 0.9, Implementation: r,
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
			for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
				if file.FlatType(arg) != "value_argument" {
					continue
				}
				expr := flatValueArgumentExpression(file, arg)
				key, display := annotationConstantKey(file, expr)
				if key == "" {
					continue
				}
				if seen[key] {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, "Duplicate constant value "+display+" in annotation definition.")
					return
				}
				seen[key] = true
			}
		},
	})
}

func registerAndroidCorrectnessWrongThread() {
	r := &WrongThreadRule{AndroidRule: alcRule("WrongThread", "Wrong thread", ALSError, 6)}
	// UI method names forbidden in @WorkerThread contexts.
	wrongThreadUIMethods := map[string]bool{
		"setText": true, "setImageResource": true, "setVisibility": true,
		"addView": true, "removeView": true, "invalidate": true,
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration"}, Confidence: 0.75, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			// Check if the function has a @WorkerThread annotation. The
			// modifiers live either as a direct `modifiers` child of the
			// function_declaration or — in some tree-sitter grammar
			// versions — as the immediately preceding sibling node.
			if !hasAnnotationNamed(file, idx, "WorkerThread") {
				return
			}
			// Walk all call_expression nodes in the function body.
			file.FlatWalkNodes(idx, "call_expression", func(callIdx uint32) {
				name := flatCallExpressionName(file, callIdx)
				if !wrongThreadUIMethods[name] {
					return
				}
				// Skip if inside a runOnUiThread or post call.
				for p, ok := file.FlatParent(callIdx); ok; p, ok = file.FlatParent(p) {
					if p == idx {
						break
					}
					if file.FlatType(p) == "call_expression" {
						pName := flatCallExpressionName(file, p)
						if pName == "runOnUiThread" || pName == "post" {
							return
						}
					}
				}
				ctx.EmitAt(file.FlatRow(callIdx)+1, 1, "UI operation in @WorkerThread context. Use runOnUiThread or Handler.post().")
			})
		},
	})
}

func registerAndroidCorrectnessSQLiteString() {
	r := &SQLiteStringRule{AndroidRule: alcRule("SQLiteString", "Using STRING instead of TEXT in SQLite", ALSWarning, 5)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"string_literal", "line_string_literal", "multi_line_string_literal"}, Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if file.FlatType(idx) != "string_literal" {
				return
			}
			if flatContainsStringInterpolation(file, idx) {
				return
			}
			upper := strings.ToUpper(stringLiteralContent(file, idx))
			if strings.Contains(upper, "CREATE TABLE") && strings.Contains(upper, " STRING") {
				ctx.EmitAt(file.FlatRow(idx)+1, 1, "SQLite does not support STRING type. Use TEXT instead.")
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

func registerAndroidCorrectnessNestedScrolling() {
	r := &NestedScrollingRule{AndroidRule: alcRule("NestedScrolling", "Nested scrolling widgets", ALSWarning, 7)}
	nestedScrollNames := map[string]bool{
		"ScrollView": true, "LazyColumn": true, "LazyRow": true,
		"HorizontalPager": true, "VerticalPager": true,
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			name := flatCallNameAny(file, idx)
			if !nestedScrollNames[name] {
				return
			}
			// Check for an ancestor call_expression that is also a scroll container.
			for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
				if file.FlatType(p) == "call_expression" && nestedScrollNames[flatCallNameAny(file, p)] {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, "Nested scrolling detected ("+name+" inside another scroll container). This can cause performance issues.")
					return
				}
			}
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
			if flatCallExpressionName(file, idx) != "SimpleDateFormat" {
				return
			}
			// Require at least two args (pattern + Locale). Single-arg form uses default locale.
			_, args := flatCallExpressionParts(file, idx)
			if args == 0 {
				ctx.EmitAt(file.FlatRow(idx)+1, 1, "SimpleDateFormat without explicit Locale. Use SimpleDateFormat(pattern, Locale) to avoid locale bugs.")
				return
			}
			count := 0
			for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
				if file.FlatType(arg) == "value_argument" {
					count++
				}
			}
			if count < 2 {
				ctx.EmitAt(file.FlatRow(idx)+1, 1, "SimpleDateFormat without explicit Locale. Use SimpleDateFormat(pattern, Locale) to avoid locale bugs.")
			}
		},
	})
}

func registerAndroidCorrectnessSetTextI18n() {
	r := &SetTextI18nRule{AndroidRule: alcRule("SetTextI18n", "TextView with internationalization issues", ALSWarning, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes:  []string{"call_expression"},
		Needs:      api.NeedsTypeInfo,
		Confidence: 0.75, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallExpressionName(file, idx) != "setText" {
				return
			}
			// Only flag when the first argument is a raw string literal (hardcoded text).
			_, args := flatCallExpressionParts(file, idx)
			if args == 0 {
				return
			}
			firstArg := flatPositionalValueArgument(file, args, 0)
			if firstArg == 0 {
				return
			}
			hasLiteral := false
			for expr := file.FlatFirstChild(firstArg); expr != 0; expr = file.FlatNextSib(expr) {
				if !file.FlatIsNamed(expr) {
					continue
				}
				t := file.FlatType(expr)
				if t == "line_string_literal" || t == "string_literal" {
					hasLiteral = true
				}
				break
			}
			if !hasLiteral {
				return
			}
			if !setTextI18nReceiverIsTextView(ctx, idx) {
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, 1, "Do not pass hardcoded text to setText. Use resource strings with placeholders.")
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
			// Skip if this is inside an override function declaration.
			if _, inOverride := flatEnclosingAncestor(file, idx, "function_declaration"); inOverride {
				fn, _ := flatEnclosingAncestor(file, idx, "function_declaration")
				if file.FlatHasModifier(fn, "override") {
					return
				}
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
						ctx.EmitAt(file.FlatRow(idx)+1, 1, "Suspicious method call; should probably call draw/measure/layout instead of "+name+".")
						return
					}
				}
			}
			// Fallback: require the enclosing class extends View.
			if !androidEnclosingClassExtendsView(file, idx, ctx.Resolver) {
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, 1, "Suspicious method call; should probably call draw/measure/layout instead of "+name+".")
		},
	})
}

func checkDefaultLocaleJava(ctx *api.Context, idx uint32) {
	file := ctx.File
	name := wrongViewCastCallName(file, idx)
	args := wrongViewCastCallArgumentExpressions(file, idx)
	switch name {
	case "toLowerCase", "toUpperCase":
		if len(args) == 0 {
			ctx.EmitAt(file.FlatRow(idx)+1, 1, "Implicitly using the default locale. Use lowercase(Locale) or uppercase(Locale) instead.")
			return
		}
		if !javaExprMentionsLocale(file, args[0]) {
			ctx.EmitAt(file.FlatRow(idx)+1, 1, "Implicitly using the default locale. Use lowercase(Locale) or uppercase(Locale) instead.")
		}
	case "format":
		receiver := wrongViewCastCallReceiverName(file, idx)
		if receiver != "String" && receiver != "java.lang.String" {
			return
		}
		if len(args) == 0 {
			return
		}
		if javaExprMentionsLocale(file, args[0]) {
			return
		}
		ctx.EmitAt(file.FlatRow(idx)+1, 1, "Implicitly using the default locale. Use String.format(Locale, ...) instead.")
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
		ctx.EmitAt(file.FlatRow(idx)+1, 1, "SimpleDateFormat without explicit Locale. Use SimpleDateFormat(pattern, Locale) to avoid locale bugs.")
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
	if fact, ok := javaSemanticCallFact(ctx, idx); ok {
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
	ctx.EmitAt(file.FlatRow(idx)+1, 1, "SharedPreferences.edit() without commit() or apply().")
}

func checkCommitTransactionJava(ctx *api.Context, idx uint32) {
	file := ctx.File
	if javaMethodInvocationName(file, idx) != "beginTransaction" {
		return
	}
	if fact, ok := javaSemanticCallFact(ctx, idx); ok {
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
	if txVar := javaLocalInitializerAssignedName(file, idx); txVar != "" &&
		javaFunctionHasReceiverCallAfter(file, fn, idx, txVar, commitTransactionNames) {
		return
	}
	if javaEnclosingFunctionHasCallNamed(file, fn, idx, commitTransactionNames) {
		return
	}
	ctx.EmitAt(file.FlatRow(idx)+1, 1, "FragmentTransaction without commit(). Call commit() or commitAllowingStateLoss().")
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
	ctx.EmitAt(file.FlatRow(idx)+1, 1,
		"The result of this call is not used. Check if the return value should be consumed.")
}

func checkResultJavaSemanticConfirmed(ctx *api.Context, idx uint32, name string) (bool, bool) {
	fact, ok := javaSemanticCallFact(ctx, idx)
	if !ok {
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
