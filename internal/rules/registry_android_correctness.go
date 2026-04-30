package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"strconv"
	"strings"
)

func registerAndroidCorrectnessRules() {

	// --- from android_correctness.go ---
	{
		r := &DefaultLocaleRule{AndroidRule: alcRule("DefaultLocale", "Implied default locale in case conversion", ALSWarning, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
					// Has value_arguments — check if any arg is a Locale reference.
					for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
						if file.FlatType(arg) != "value_argument" {
							continue
						}
						for expr := file.FlatFirstChild(arg); expr != 0; expr = file.FlatNextSib(expr) {
							if !file.FlatIsNamed(expr) {
								continue
							}
							if file.FlatType(expr) == "navigation_expression" {
								first := file.FlatFirstChild(expr)
								if first != 0 && file.FlatNodeText(first) == "Locale" {
									return
								}
							}
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
					for expr := file.FlatFirstChild(firstArg); expr != 0; expr = file.FlatNextSib(expr) {
						if !file.FlatIsNamed(expr) {
							continue
						}
						if file.FlatType(expr) == "navigation_expression" {
							first := file.FlatFirstChild(expr)
							if first != 0 && file.FlatNodeText(first) == "Locale" {
								return
							}
						}
					}
					ctx.EmitAt(file.FlatRow(idx)+1, 1, "Implicitly using the default locale. Use String.format(Locale, ...) instead.")
				}
			},
		})
	}
	{
		r := &CommitPrefEditsRule{AndroidRule: alcRule("CommitPrefEdits", "Missing commit() on SharedPreferences editor", ALSWarning, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.8, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
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
	{
		r := &CommitTransactionRule{AndroidRule: alcRule("CommitTransaction", "Missing commit() on FragmentTransaction", ALSWarning, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.8, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "beginTransaction" {
					return
				}
				fn, ok := flatEnclosingAncestor(file, idx, "function_declaration", "function_body")
				if !ok {
					return
				}
				if enclosingFunctionHasCallNamed(file, fn, idx, commitTransactionNames) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, "FragmentTransaction without commit(). Call commit() or commitAllowingStateLoss().")
			},
		})
	}
	{
		r := &AssertRule{AndroidRule: alcRule("Assert", "Assertions are unreliable on Android", ALSWarning, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "assert" {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, "assert is not reliable on Android. Use a proper assertion library or throw explicitly.")
			},
		})
	}
	{
		r := &CheckResultRule{AndroidRule: alcRule("CheckResult", "Ignoring results", ALSWarning, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.8, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if strings.HasSuffix(file.Path, ".gradle.kts") {
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
	{
		r := &ShiftFlagsRule{AndroidRule: alcRule("ShiftFlags", "Suspicious flag constant declarations", ALSWarning, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
	{
		r := &UniqueConstantsRule{AndroidRule: alcRule("UniqueConstants", "Overlapping enumeration constants", ALSError, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"annotation"}, Confidence: 0.9, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
	{
		r := &WrongThreadRule{AndroidRule: alcRule("WrongThread", "Wrong thread", ALSError, 6)}
		// UI method names forbidden in @WorkerThread contexts.
		wrongThreadUIMethods := map[string]bool{
			"setText": true, "setImageResource": true, "setVisibility": true,
			"addView": true, "removeView": true, "invalidate": true,
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
	{
		r := &SQLiteStringRule{AndroidRule: alcRule("SQLiteString", "Using STRING instead of TEXT in SQLite", ALSWarning, 5)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"string_literal", "line_string_literal", "multi_line_string_literal"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
	{
		r := &RegisteredRule{AndroidRule: alcRule("Registered", "Class is not registered in the manifest", ALSWarning, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Needs: v2.NeedsResolver, TypeInfo: v2.TypeInfoHint{PreferBackend: v2.PreferResolver, Required: true}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
	{
		r := &NestedScrollingRule{AndroidRule: alcRule("NestedScrolling", "Nested scrolling widgets", ALSWarning, 7)}
		nestedScrollNames := map[string]bool{
			"ScrollView": true, "LazyColumn": true, "LazyRow": true,
			"HorizontalPager": true, "VerticalPager": true,
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
	{
		r := &ScrollViewCountRule{AndroidRule: alcRule("ScrollViewCount", "ScrollViews can have only one child", ALSWarning, 7)}
		scrollViewCtorNames := map[string]bool{"ScrollView": true, "HorizontalScrollView": true}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
	{
		r := &SimpleDateFormatRule{AndroidRule: alcRule("SimpleDateFormat", "Using SimpleDateFormat directly without Locale", ALSWarning, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "object_creation_expression"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
	{
		r := &SetTextI18nRule{AndroidRule: alcRule("SetTextI18n", "TextView with internationalization issues", ALSWarning, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
				for expr := file.FlatFirstChild(firstArg); expr != 0; expr = file.FlatNextSib(expr) {
					if !file.FlatIsNamed(expr) {
						continue
					}
					t := file.FlatType(expr)
					if t == "line_string_literal" || t == "string_literal" {
						ctx.EmitAt(file.FlatRow(idx)+1, 1, "Do not pass hardcoded text to setText. Use resource strings with placeholders.")
					}
					return
				}
			},
		})
	}
	{
		r := &StopShipRule{AndroidRule: alcRule("StopShip", "STOPSHIP comment found", ALSFatal, 10)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsLinePass, Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &WrongCallRule{AndroidRule: alcRule("WrongCall", "Using wrong draw/layout method", ALSError, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
				ctx.EmitAt(file.FlatRow(idx)+1, 1, "Suspicious method call; should probably call draw/measure/layout instead of "+name+".")
			},
		})
	}
}

func checkDefaultLocaleJava(ctx *v2.Context, idx uint32) {
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

func checkSimpleDateFormatJava(ctx *v2.Context, idx uint32) {
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
	if idx == 0 {
		return false
	}
	text := strings.TrimSpace(file.FlatNodeText(idx))
	return text == "Locale" || strings.HasPrefix(text, "Locale.") ||
		text == "java.util.Locale" || strings.HasPrefix(text, "java.util.Locale.")
}
