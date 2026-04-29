package rules

import (
	"fmt"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"strings"
)

func registerAndroidCorrectnessChecksRules() {

	// --- from android_correctness_checks.go ---
	{
		r := &OverrideAbstractRule{AndroidRule: alcRule("OverrideAbstract", "Missing abstract method overrides", ALSError, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.9, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				// The class must itself be concrete. `abstract class Foo : Service`
				// is by definition allowed to defer its abstract parent's
				// members. file.FlatHasModifier walks the class's modifiers
				// child directly — no risk of matching "abstract class" inside
				// a nested declaration or doc comment.
				if file.FlatHasModifier(idx, "abstract") {
					return
				}
				var baseClass string
				var required []string
				for cls, reqs := range abstractClassRequirements {
					if classHasSupertypeNamed(file, idx, cls) {
						baseClass = cls
						required = reqs
						break
					}
				}
				if baseClass == "" {
					return
				}
				overridden := classOverriddenFunctions(file, idx)
				var missing []string
				for _, method := range required {
					if !overridden[method] {
						missing = append(missing, method)
					}
				}
				if len(missing) > 0 {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, baseClass+" subclass must override: "+strings.Join(missing, ", ")+".")
				}
			},
		})
	}
	{
		r := &ParcelCreatorRule{AndroidRule: alcRule("ParcelCreator", "Missing Parcelable CREATOR field", ALSError, 3)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.9, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !classHasSupertypeNamed(file, idx, "Parcelable") {
					return
				}
				if hasAnnotationNamed(file, idx, "Parcelize") {
					return
				}
				if classDeclaresStaticProperty(file, idx, "CREATOR") {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, "Parcelable class missing CREATOR field. Use @Parcelize or add a CREATOR companion.")
			},
		})
	}
	{
		r := &SwitchIntDefRule{AndroidRule: alcRule("SwitchIntDef", "Missing @IntDef constants in switch", ALSWarning, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"when_expression"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				// Check if the when subject contains a "visibility" identifier.
				subject, hasSubject := file.FlatFindChild(idx, "when_subject")
				if !hasSubject {
					return
				}
				subjectText := strings.ToLower(file.FlatNodeText(subject))
				if !strings.Contains(subjectText, "visibility") {
					return
				}
				// Walk when entries for coverage of VISIBLE/INVISIBLE/GONE and else.
				hasVisible, hasInvisible, hasGone, hasElse := false, false, false, false
				file.FlatWalkNodes(idx, "when_entry", func(entry uint32) {
					entryText := file.FlatNodeText(entry)
					if strings.Contains(entryText, "VISIBLE") && !strings.Contains(entryText, "INVISIBLE") {
						hasVisible = true
					}
					if strings.Contains(entryText, "INVISIBLE") {
						hasInvisible = true
					}
					if strings.Contains(entryText, "GONE") {
						hasGone = true
					}
				})
				// Check for else_entry directly.
				if _, ok := file.FlatFindChild(idx, "else_entry"); ok {
					hasElse = true
				}
				// Also check when_entry children for "else" keyword.
				if !hasElse {
					file.FlatWalkNodes(idx, "when_entry", func(entry uint32) {
						if strings.Contains(file.FlatNodeText(entry), "else") {
							hasElse = true
						}
					})
				}
				if hasElse {
					return
				}
				var missing []string
				if !hasVisible {
					missing = append(missing, "VISIBLE")
				}
				if !hasInvisible {
					missing = append(missing, "INVISIBLE")
				}
				if !hasGone {
					missing = append(missing, "GONE")
				}
				if len(missing) > 0 && len(missing) < 3 {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, "when on visibility missing constants: "+strings.Join(missing, ", ")+". Add them or an else branch.")
				}
			},
		})
	}
	{
		r := &TextViewEditsRule{AndroidRule: alcRule("TextViewEdits", "Calling setText on an EditText", ALSWarning, 5)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "setText" {
					return
				}
				// Require a navigation receiver whose identifier ends with "editText" or "EditText".
				navExpr, _ := flatCallExpressionParts(file, idx)
				if navExpr == 0 {
					return
				}
				recvText := strings.ToLower(file.FlatNodeText(file.FlatFirstChild(navExpr)))
				if !strings.Contains(recvText, "edittext") {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, "Using setText on an EditText. Consider using Editable or getText().")
			},
		})
	}
	{
		r := &WrongViewCastRule{AndroidRule: alcRule("WrongViewCast", "Mismatched view type", ALSError, 9)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "as_expression", "cast_expression", "local_variable_declaration"},
			Needs:     v2.NeedsTypeInfo,
			Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			OracleCallTargets: &v2.OracleCallTargetFilter{
				CalleeNames: []string{"findViewById", "requireViewById"},
			},
			// Checks whether the cast target is assignable to the receiver view type;
			// needs the class hierarchy but not member signatures.
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{ClassShell: true, Supertypes: true},
			Confidence:             r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &DeprecatedRule{AndroidRule: alcRule("Deprecated", "Using deprecated API", ALSWarning, 2)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"type_identifier", "simple_identifier"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if _, inImport := flatEnclosingAncestor(file, idx, "import_header"); inImport {
					return
				}
				name := file.FlatNodeText(idx)
				for _, entry := range deprecatedApis {
					if entry.Pattern == name {
						ctx.EmitAt(file.FlatRow(idx)+1, 1, entry.Message)
						return
					}
				}
			},
		})
	}
	{
		r := &RangeRule{AndroidRule: alcRule("Range", "Outside allowed range", ALSError, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsTypeInfo, Confidence: r.Confidence(), OriginalV1: r,
			Oracle:            &v2.OracleFilter{Identifiers: []string{"IntRange", "FloatRange", "Color", "setAlpha", "setProgress", "setRotation"}},
			OracleCallTargets: &v2.OracleCallTargetFilter{CalleeNames: []string{"argb", "rgb", "setAlpha", "setProgress", "setRotation"}},
			// Uses LookupCallTarget to verify the method is a framework target;
			// allowed ranges come from hardcoded tables, not oracle annotations.
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check:                  r.check,
		})
	}
	{
		r := &ResourceTypeRule{AndroidRule: alcRule("ResourceType", "Wrong resource type", ALSError, 7)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				expected, ok := resourceMethodExpected[name]
				if !ok {
					return
				}
				// Find first value_argument that is a navigation_expression starting with "R".
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
					if file.FlatType(arg) != "value_argument" {
						continue
					}
					for expr := file.FlatFirstChild(arg); expr != 0; expr = file.FlatNextSib(expr) {
						if !file.FlatIsNamed(expr) || file.FlatType(expr) != "navigation_expression" {
							continue
						}
						// R.category.name: first child is "R", second is the category identifier.
						parts := flatNavigationIdentifierParts(file, expr)
						if len(parts) < 3 || parts[0] != "R" {
							break
						}
						actualType := parts[1]
						resourceName := parts[2]
						if actualType != expected {
							ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("%s(R.%s.%s): expected R.%s resource, not R.%s.", name, actualType, resourceName, expected, actualType))
						}
						return
					}
					break // only inspect first positional argument
				}
			},
		})
	}
	{
		r := &ResourceAsColorRule{AndroidRule: alcRule("ResourceAsColor", "Should pass resolved color instead of resource id", ALSError, 7)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "setBackgroundColor" && name != "setTextColor" && name != "setColor" {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				firstArg := flatPositionalValueArgument(file, args, 0)
				if firstArg == 0 {
					return
				}
				for expr := file.FlatFirstChild(firstArg); expr != 0; expr = file.FlatNextSib(expr) {
					if !file.FlatIsNamed(expr) || file.FlatType(expr) != "navigation_expression" {
						continue
					}
					// Drill to the leftmost identifier of a chained navigation_expression.
					root := expr
					for {
						first := file.FlatFirstChild(root)
						if first == 0 || file.FlatType(first) != "navigation_expression" {
							if first != 0 && file.FlatNodeText(first) == "R" {
								ctx.EmitAt(file.FlatRow(idx)+1, 1, "Passing a resource ID where a color value is expected. Use ContextCompat.getColor() instead.")
							}
							break
						}
						root = first
					}
					return
				}
			},
		})
	}
	{
		r := &SupportAnnotationUsageRule{AndroidRule: alcRule("SupportAnnotationUsage", "Incorrect support annotation usage", ALSError, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				// Require @MainThread annotation in the preceding sibling (modifiers block).
				if !hasAnnotationFlat(file, idx, "MainThread") {
					return
				}
				// Walk call_expressions in the function body, check for IO type names.
				startLine := file.FlatRow(idx) + 1
				file.FlatWalkNodes(idx, "call_expression", func(callIdx uint32) {
					name := flatCallNameAny(file, callIdx)
					if ioCallNames[name] {
						ctx.EmitAt(file.FlatRow(callIdx)+1, 1, fmt.Sprintf("@MainThread function (line %d) performs IO/network operation (%s). This may block the UI thread.", startLine, name))
					}
				})
			},
		})
	}
	{
		r := &AccidentalOctalRule{AndroidRule: alcRule("AccidentalOctal", "Accidental octal interpretation", ALSWarning, 5)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"integer_literal"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				// Must start with "0" followed by at least two more digits (not 0x, 0b).
				if len(text) < 3 || text[0] != '0' {
					return
				}
				if text[1] == 'x' || text[1] == 'X' || text[1] == 'b' || text[1] == 'B' {
					return
				}
				if text[1] < '0' || text[1] > '9' {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, "Suspicious leading zero — this may be an accidental octal literal.")
			},
		})
	}
	{
		r := &AppCompatMethodRule{AndroidRule: alcRule("AppCompatMethod", "Using Wrong AppCompat Method", ALSWarning, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "getActionBar" && name != "setProgressBarVisibility" && name != "setProgressBarIndeterminateVisibility" {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, "Use AppCompat equivalent methods for backward compatibility.")
			},
		})
	}
	{
		r := &CustomViewStyleableRule{AndroidRule: alcRule("CustomViewStyleable", "Mismatched Styleable/Custom View Name", ALSWarning, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.9, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "obtainStyledAttributes" {
					return
				}
				// Second positional arg is R.styleable.<Name> — extract
				// <Name> via AST navigation rather than regex-matching the
				// whole call's text.
				args := flatCallKeyArguments(file, idx)
				secondArg := flatPositionalValueArgument(file, args, 1)
				expr := flatValueArgumentExpression(file, secondArg)
				if expr == 0 || file.FlatType(expr) != "navigation_expression" {
					return
				}
				styleableName := flatNavigationExpressionLastIdentifier(file, expr)
				if styleableName == "" {
					return
				}
				// Enclosing class name via AST, not classNameRe text scan.
				classIdx, ok := flatEnclosingAncestor(file, idx, "class_declaration")
				if !ok {
					return
				}
				className := extractIdentifierFlat(file, classIdx)
				if className == "" || styleableName == className {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1,
					fmt.Sprintf("Custom view '%s' uses R.styleable.%s \u2014 expected R.styleable.%s to match the class name.", className, styleableName, className))
			},
		})
	}
	{
		r := &InnerclassSeparatorRule{AndroidRule: alcRule("InnerclassSeparator", "Inner classes should use '$' not '/'", ALSWarning, 3)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "forName" {
					return
				}
				// Require receiver to be "Class".
				navExpr, args := flatCallExpressionParts(file, idx)
				if navExpr == 0 || args == 0 {
					return
				}
				recv := file.FlatFirstChild(navExpr)
				if recv == 0 || file.FlatNodeText(recv) != "Class" {
					return
				}
				// Inspect the first string argument for "word/word" without "$".
				firstArg := flatPositionalValueArgument(file, args, 0)
				if firstArg == 0 {
					return
				}
				expr := flatValueArgumentExpression(file, firstArg)
				if expr == 0 || file.FlatType(expr) != "string_literal" {
					return
				}
				if flatContainsStringInterpolation(file, expr) {
					return
				}
				content := stringLiteralContent(file, expr)
				if strings.Contains(content, "/") && !strings.Contains(content, "$") {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, "Use '$' instead of '/' as inner class separator in class names.")
				}
			},
		})
	}
	{
		r := &ObjectAnimatorBindingRule{AndroidRule: alcRule("ObjectAnimatorBinding", "Incorrect ObjectAnimator Property", ALSError, 4)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsTypeInfo, Confidence: r.Confidence(), OriginalV1: r,
			Oracle: &v2.OracleFilter{Identifiers: []string{"ObjectAnimator", "ofFloat", "ofInt", "ofObject"}},
			OracleCallTargets: &v2.OracleCallTargetFilter{TargetFQNs: []string{
				"android.animation.ObjectAnimator.ofFloat",
				"android.animation.ObjectAnimator.ofInt",
				"android.animation.ObjectAnimator.ofObject",
			}},
			// Checks member names for the property/setter (Members), traverses
			// the class hierarchy to confirm the target is a View (ClassShell+Supertypes).
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{ClassShell: true, Supertypes: true, Members: true},
			Check:                  r.check,
		})
	}
	{
		r := &OnClickRule{AndroidRule: alcRule("OnClick", "android:onClick handler missing or has wrong signature", ALSWarning, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs:       v2.NeedsResources | v2.NeedsParsedFiles,
			AndroidDeps: uint32(AndroidDepLayout),
			Confidence:  r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &PropertyEscapeRule{AndroidRule: alcRule("PropertyEscape", "Invalid property file escapes", ALSError, 5)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"string_literal", "line_string_literal"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatType(idx) != "string_literal" {
					return
				}
				// Raw strings (""" """) don't process escapes — structural
				// check via stringLiteralIsRaw, not a raw-text prefix match.
				if stringLiteralIsRaw(file, idx) {
					return
				}
				// Walk string_content children for escape sequences. Content
				// nodes give us the exact unescaped source span; stitching
				// them together avoids parsing around the surrounding quotes.
				for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
					if file.FlatType(child) != "string_content" {
						continue
					}
					seg := file.FlatNodeText(child)
					for i := 0; i < len(seg)-1; i++ {
						if seg[i] != '\\' {
							continue
						}
						next := seg[i+1]
						switch next {
						case 'n', 't', 'r', '\\', '"', '\'', '$', 'b', 'u', 'f':
							i++
						default:
							if next >= '0' && next <= '9' {
								i++
								continue
							}
							ctx.EmitAt(file.FlatRow(idx)+1, 1, "Invalid escape sequence '\\"+string(next)+"' in string literal.")
							i++
						}
					}
				}
			},
		})
	}
	{
		r := &ShortAlarmRule{AndroidRule: alcRule("ShortAlarm", "Short or frequent alarm", ALSWarning, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "setRepeating" && name != "setInexactRepeating" {
					return
				}
				// setRepeating(type, triggerAtMillis, intervalMillis, operation) —
				// the interval is the third positional argument.
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				intervalArg := flatPositionalValueArgument(file, args, 2)
				if intervalArg == 0 {
					return
				}
				for expr := file.FlatFirstChild(intervalArg); expr != 0; expr = file.FlatNextSib(expr) {
					if !file.FlatIsNamed(expr) {
						continue
					}
					t := file.FlatType(expr)
					if t != "integer_literal" && t != "long_literal" {
						return
					}
					text := strings.TrimRight(file.FlatNodeText(expr), "lLuU")
					val := parseInt(text)
					if val > 0 && val < 60000 {
						ctx.EmitAt(file.FlatRow(idx)+1, 1, "Short alarm interval. Consider using a minimum of 60 seconds for repeating alarms.")
					}
					return
				}
			},
		})
	}
	{
		r := &LocalSuppressRule{AndroidRule: alcRule("LocalSuppress", "@SuppressLint misuse", ALSWarning, 6)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"annotation"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				// Tree-sitter-kotlin nests the annotation name + args under
				// a constructor_invocation child of the annotation node.
				var ctor uint32
				for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
					if file.FlatIsNamed(child) && file.FlatType(child) == "constructor_invocation" {
						ctor = child
						break
					}
				}
				nameHost := idx
				if ctor != 0 {
					nameHost = ctor
				}
				var annotName string
				for child := file.FlatFirstChild(nameHost); child != 0; child = file.FlatNextSib(child) {
					if !file.FlatIsNamed(child) {
						continue
					}
					t := file.FlatType(child)
					if t == "simple_identifier" || t == "type_identifier" {
						annotName = file.FlatNodeText(child)
						break
					}
					if t == "user_type" {
						annotName = extractIdentifierFlat(file, child)
						break
					}
				}
				if annotName != "SuppressLint" {
					return
				}
				// Extract string argument values from value_arguments (under
				// constructor_invocation when present, else directly on annotation).
				args, hasArgs := file.FlatFindChild(nameHost, "value_arguments")
				if !hasArgs {
					return
				}
				for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
					if file.FlatType(arg) != "value_argument" {
						continue
					}
					for expr := file.FlatFirstChild(arg); expr != 0; expr = file.FlatNextSib(expr) {
						if !file.FlatIsNamed(expr) {
							continue
						}
						t := file.FlatType(expr)
						if t != "line_string_literal" && t != "string_literal" {
							continue
						}
						raw := file.FlatNodeText(expr)
						issueID := strings.Trim(raw, `"`)
						if !knownLintIssueIDs[issueID] {
							ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("@SuppressLint(\"%s\"): '%s' is not a known Android Lint issue ID.", issueID, issueID))
						}
					}
				}
			},
		})
	}
	{
		r := &PluralsCandidateRule{AndroidRule: alcRule("PluralsCandidate", "Potential plurals", ALSWarning, 5)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"if_expression", "when_expression"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				nodeType := file.FlatType(idx)
				if nodeType == "if_expression" {
					// Look for `X == 1` in the condition. Tree-sitter-kotlin exposes
					// the condition as a field (not a node type), so inspect the first
					// named child, which is the condition expression.
					var cond uint32
					for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
						if file.FlatIsNamed(child) && file.FlatType(child) != "control_structure_body" {
							cond = child
							break
						}
					}
					if cond == 0 {
						return
					}
					condText := file.FlatNodeText(cond)
					if !strings.Contains(condText, "== 1") && !strings.Contains(condText, "== 1L") {
						return
					}
				} else {
					// when_expression: subject must be a count-like identifier.
					subject, hasSubject := file.FlatFindChild(idx, "when_subject")
					if !hasSubject {
						return
					}
					// when_subject wraps `(expr)`; drill into the first named
					// simple_identifier child.
					var subjectName string
					for child := file.FlatFirstChild(subject); child != 0; child = file.FlatNextSib(child) {
						if file.FlatIsNamed(child) && file.FlatType(child) == "simple_identifier" {
							subjectName = file.FlatNodeText(child)
							break
						}
					}
					if !pluralsCountNames[subjectName] {
						return
					}
				}
				// Walk body call_expressions for string resource usage.
				found := false
				file.FlatWalkNodes(idx, "call_expression", func(callIdx uint32) {
					if found {
						return
					}
					name := flatCallExpressionName(file, callIdx)
					if pluralsStringCalls[name] {
						found = true
					}
				})
				if found {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, "Manual pluralization detected. Use getQuantityString() for proper plural handling.")
				}
			},
		})
	}
}
