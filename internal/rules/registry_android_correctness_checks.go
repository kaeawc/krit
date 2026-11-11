package rules

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerAndroidCorrectnessChecksRules() {
	registerAndroidCorrectnessChecksOverrideAbstract()
	registerAndroidCorrectnessChecksParcelCreator()
	registerAndroidCorrectnessChecksSwitchIntDef()
	registerAndroidCorrectnessChecksTextViewEdits()
	registerAndroidCorrectnessChecksWrongViewCast()
	registerAndroidCorrectnessChecksDeprecated()
	registerAndroidCorrectnessChecksRange()
	registerAndroidCorrectnessChecksResourceType()
	registerAndroidCorrectnessChecksResourceAsColor()
	registerAndroidCorrectnessChecksSupportAnnotationUsage()
	registerAndroidCorrectnessChecksAccidentalOctal()
	registerAndroidCorrectnessChecksAppCompatMethod()
	registerAndroidCorrectnessChecksCustomViewStyleable()
	registerAndroidCorrectnessChecksInnerclassSeparator()
	registerAndroidCorrectnessChecksObjectAnimatorBinding()
	registerAndroidCorrectnessChecksOnClick()
	registerAndroidCorrectnessChecksPropertyEscape()
	registerAndroidCorrectnessChecksShortAlarm()
	registerAndroidCorrectnessChecksLocalSuppress()
	registerAndroidCorrectnessChecksPluralsCandidate()
}

func registerAndroidCorrectnessChecksOverrideAbstract() {
	r := &OverrideAbstractRule{AndroidRule: alcRule("OverrideAbstract", "Missing abstract method overrides", ALSError, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceHigher, Implementation: r,
		Check: func(ctx *api.Context) {
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
				if classExtendsAndroidClass(file, idx, cls) {
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
				ctx.EmitAt(declLine(file, idx), declCol(file, idx), baseClass+" subclass must override: "+strings.Join(missing, ", ")+".")
			}
		},
	})
}

func registerAndroidCorrectnessChecksParcelCreator() {
	r := &ParcelCreatorRule{AndroidRule: alcRule("ParcelCreator", "Missing Parcelable CREATOR field", ALSError, 3)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceHigher, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !classHasSupertypeNamed(file, idx, "Parcelable") {
				return
			}
			// Only concrete classes need a CREATOR; subclasses of an abstract parent declare their own.
			if abstractRuleIsInterface(file, idx) || file.FlatHasModifier(idx, "abstract") || file.FlatHasModifier(idx, "sealed") {
				return
			}
			if hasAnnotationNamed(file, idx, "Parcelize") {
				return
			}
			if prop, found := parcelCreatorMember(file, idx); found {
				if prop != 0 && !hasAnnotationNamed(file, prop, "JvmField") {
					ctx.EmitAt(declLine(file, prop), declCol(file, prop), "Field should be annotated with `@JvmField`")
				}
				return
			}
			ctx.EmitAt(declLine(file, idx), declCol(file, idx), "This class implements `Parcelable` but does not provide a `CREATOR` field")
		},
	})
}

func registerAndroidCorrectnessChecksSwitchIntDef() {
	r := &SwitchIntDefRule{AndroidRule: alcRule("SwitchIntDef", "Missing @IntDef constants in switch", ALSWarning, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"when_expression"}, Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
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
				// Only the conditions before `->` count, not the branch body.
				entryText := file.FlatNodeText(entry)
				if arrow := strings.Index(entryText, "->"); arrow >= 0 {
					entryText = entryText[:arrow]
				}
				if strings.TrimSpace(entryText) == "else" {
					hasElse = true
				}
				for _, cond := range strings.Split(entryText, ",") {
					switch name := lastDottedSegment(strings.TrimSpace(cond)); name {
					case "VISIBLE":
						hasVisible = true
					case "INVISIBLE":
						hasInvisible = true
					case "GONE":
						hasGone = true
					}
				}
			})
			if _, ok := file.FlatFindChild(idx, "else_entry"); ok {
				hasElse = true
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
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "when on visibility missing constants: "+strings.Join(missing, ", ")+". Add them or an else branch.")
			}
		},
	})
}

func registerAndroidCorrectnessChecksTextViewEdits() {
	r := &TextViewEditsRule{AndroidRule: alcRule("TextViewEdits", "Editing the text of a TextView that is not an EditText", ALSWarning, 5)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"as_expression"}, Needs: api.NeedsTypeInfo, Confidence: r.Confidence(), Implementation: r,
		Check: checkTextViewEdits,
	})
}

func registerAndroidCorrectnessChecksWrongViewCast() {
	r := &WrongViewCastRule{AndroidRule: alcRule("WrongViewCast", "Mismatched view type", ALSError, 9)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression", "as_expression", "cast_expression", "local_variable_declaration"},
		Needs: api.NeedsTypeInfo |
			api.NeedsOracleCallTargets |
			api.NeedsOracleSupertypes |
			api.NeedsResources,
		AndroidDeps: uint32(AndroidDepLayout),
		Languages:   []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Confidence:  r.Confidence(), Implementation: r,
		Check: r.check,
	})
}

func registerAndroidCorrectnessChecksDeprecated() {
	r := &DeprecatedRule{AndroidRule: alcRule("Deprecated", "Using deprecated API", ALSWarning, 2)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"type_identifier", "simple_identifier"}, Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if _, inImport := flatEnclosingAncestor(file, idx, "import_header"); inImport {
				return
			}
			name := file.FlatNodeText(idx)
			for _, entry := range deprecatedApis {
				if entry.Pattern != name || !deprecatedEntryApplies(file, idx, entry) {
					continue
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, entry.Message)
				return
			}
		},
	})
}

// deprecatedEntryApplies reports whether the identifier at idx is a use of the deprecated API.
// Class entries need an import (or fully qualified mention) of the platform class, and the
// identifier must not be a declaration name. Method entries must be member calls.
func deprecatedEntryApplies(file *scanner.File, idx uint32, entry deprecatedAPIEntry) bool {
	parent, _ := file.FlatParent(idx)
	if len(entry.FQNs) == 0 {
		return file.FlatType(parent) == "navigation_suffix"
	}
	switch file.FlatType(parent) {
	case "class_declaration", "object_declaration", "type_alias", "function_declaration", "property_declaration", "variable_declaration":
		return false
	}
	for _, fqn := range entry.FQNs {
		if sourceImportsOrMentions(file, fqn) {
			return true
		}
	}
	return false
}

func registerAndroidCorrectnessChecksRange() {
	r := &RangeRule{AndroidRule: alcRule("Range", "Outside allowed range", ALSError, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes:  []string{"call_expression"},
		Needs:      api.NeedsTypeInfo | api.NeedsOracleCallTargets,
		Confidence: r.Confidence(), Implementation: r,
		Oracle: &api.OracleFilter{Identifiers: []string{"IntRange", "FloatRange", "Color", "setAlpha", "setProgress", "setRotation"}},
		Check:  r.check,
	})
}

func registerAndroidCorrectnessChecksResourceType() {
	r := &ResourceTypeRule{AndroidRule: alcRule("ResourceType", "Wrong resource type", ALSError, 7)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets, Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			name := flatCallExpressionName(file, idx)
			expected, ok := resourceMethodExpected[name]
			if !ok {
				return
			}
			// With an oracle, an app-defined method that happens to share a framework name is not checked.
			if cr, isComposite := ctx.Resolver.(*oracle.CompositeResolver); isComposite && cr.Oracle() != nil {
				if !strings.HasPrefix(oracleLookupCallTargetFlat(cr.Oracle(), file, idx), "android.") {
					return
				}
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
					// TypedArray accessors take R.styleable indices, not resource ids of the result type.
					if actualType == "styleable" {
						return
					}
					if actualType != expected && !resourceTypeAlsoAccepted(name, actualType) {
						ctx.EmitAt(callLine(file, idx), callCol(file, idx), fmt.Sprintf("%s(R.%s.%s): expected R.%s resource, not R.%s.", name, actualType, resourceName, expected, actualType))
					}
					return
				}
				break // only inspect first positional argument
			}
		},
	})
}

func registerAndroidCorrectnessChecksResourceAsColor() {
	r := &ResourceAsColorRule{AndroidRule: alcRule("ResourceAsColor", "Should pass resolved color instead of resource id", ALSError, 7)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets, Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			name := flatCallExpressionName(file, idx)
			if name != "setBackgroundColor" && name != "setTextColor" && name != "setColor" {
				return
			}
			// With an oracle, an app-defined setter that takes a @ColorRes id is not a framework call.
			if cr, isComposite := ctx.Resolver.(*oracle.CompositeResolver); isComposite && cr.Oracle() != nil {
				if !strings.HasPrefix(oracleLookupCallTargetFlat(cr.Oracle(), file, idx), "android.") {
					return
				}
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
							ctx.EmitAt(callLine(file, idx), callCol(file, idx), "Passing a resource ID where a color value is expected. Use ContextCompat.getColor() instead.")
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

func registerAndroidCorrectnessChecksSupportAnnotationUsage() {
	r := &SupportAnnotationUsageRule{AndroidRule: alcRule("SupportAnnotationUsage", "Incorrect support annotation usage", ALSError, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration", "property_declaration", "parameter", "class_parameter"}, Confidence: r.Confidence(), Implementation: r,
		Check: checkSupportAnnotationUsage,
	})
}

func registerAndroidCorrectnessChecksAccidentalOctal() {
	r := &AccidentalOctalRule{AndroidRule: alcRule("AccidentalOctal", "Accidental octal interpretation", ALSError, 5)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
		Check: r.check,
	})
}

func registerAndroidCorrectnessChecksAppCompatMethod() {
	r := &AppCompatMethodRule{AndroidRule: alcRule("AppCompatMethod", "Using Wrong AppCompat Method", ALSWarning, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			name := flatCallExpressionName(file, idx)
			replacement, ok := appCompatMethodReplacements[name]
			if !ok {
				return
			}
			// Only the activity's own methods have AppCompat equivalents, not `other.getActionBar()`.
			if navExpr, _ := flatCallExpressionParts(file, idx); navExpr != 0 {
				recv := strings.TrimSpace(file.FlatNodeText(file.FlatNamedChild(navExpr, 0)))
				if recv != "this" && recv != "super" {
					return
				}
			}
			cls, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_declaration")
			if !ok || !classExtendsAnyFlat(file, cls, "AppCompatActivity", "ActionBarActivity") {
				return
			}
			ctx.EmitAt(callLine(file, idx), callCol(file, idx), "Should use `"+replacement+"` instead of `"+name+"` name")
		},
	})
}

// appCompatMethodReplacements maps framework Activity methods to their AppCompatActivity equivalents.
var appCompatMethodReplacements = map[string]string{
	"getActionBar":                          "getSupportActionBar",
	"startActionMode":                       "startSupportActionMode",
	"setProgressBarVisibility":              "setSupportProgressBarVisibility",
	"setProgressBarIndeterminate":           "setSupportProgressBarIndeterminate",
	"setProgressBarIndeterminateVisibility": "setSupportProgressBarIndeterminateVisibility",
}

func registerAndroidCorrectnessChecksCustomViewStyleable() {
	r := &CustomViewStyleableRule{AndroidRule: alcRule("CustomViewStyleable", "Mismatched Styleable/Custom View Name", ALSWarning, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Needs: api.NeedsTypeInfo, Confidence: api.ConfidenceHigher, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallExpressionName(file, idx) != "obtainStyledAttributes" {
				return
			}
			// The attrs array is the second argument, or the only one in obtainStyledAttributes(attrs).
			args := flatCallKeyArguments(file, idx)
			attrsArg := flatPositionalValueArgument(file, args, 1)
			if attrsArg == 0 && file.FlatNamedChildCount(args) == 1 {
				attrsArg = flatPositionalValueArgument(file, args, 0)
			}
			expr := flatValueArgumentExpression(file, attrsArg)
			if expr == 0 || file.FlatType(expr) != "navigation_expression" {
				return
			}
			styleableName := flatNavigationExpressionLastIdentifier(file, expr)
			if styleableName == "" || !strings.HasSuffix(strings.TrimSpace(file.FlatNodeText(expr)), "styleable."+styleableName) {
				return
			}
			// The styleable only has to match a custom view, and only code that runs per instance.
			classIdx, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_declaration", "companion_object")
			if !ok || file.FlatType(classIdx) != "class_declaration" {
				return
			}
			if !androidEnclosingClassExtendsView(file, idx, ctx.Resolver) {
				return
			}
			className := extractIdentifierFlat(file, classIdx)
			if className == "" || styleableName == className {
				return
			}
			// A subclass may reuse the styleable of the view it extends.
			for _, super := range androidDirectSupertypesFlat(file, classIdx) {
				if super.simple == styleableName {
					return
				}
			}
			ctx.EmitAt(file.FlatRow(expr)+1, file.FlatCol(expr)+1,
				fmt.Sprintf("By convention, the custom view (`%s`) and the declare-styleable (`%s`) should have the same name (various editor features rely on this convention)", className, styleableName))
		},
	})
}

// dottedInnerClassName reports a dotted name where two consecutive segments start with an uppercase letter,
// such as com.example.Outer.Inner, which Class.forName cannot load.
func dottedInnerClassName(name string) bool {
	segments := strings.Split(name, ".")
	for i := 0; i+1 < len(segments); i++ {
		if startsWithUpper(segments[i]) && startsWithUpper(segments[i+1]) {
			return true
		}
	}
	return false
}

func startsWithUpper(s string) bool {
	return s != "" && s[0] >= 'A' && s[0] <= 'Z'
}

func registerAndroidCorrectnessChecksInnerclassSeparator() {
	r := &InnerclassSeparatorRule{AndroidRule: alcRule("InnerclassSeparator", "Inner classes should use '$' not '/'", ALSWarning, 3)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
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
			if strings.Contains(content, "$") {
				return
			}
			if strings.Contains(content, "/") || dottedInnerClassName(content) {
				ctx.EmitAt(callLine(file, idx), callCol(file, idx), "Use '$' instead of '.' or '/' as the inner class separator in class names.")
			}
		},
	})
}

func registerAndroidCorrectnessChecksObjectAnimatorBinding() {
	r := &ObjectAnimatorBindingRule{AndroidRule: alcRule("ObjectAnimatorBinding", "Incorrect ObjectAnimator Property", ALSError, 4)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"},
		Needs: api.NeedsTypeInfo |
			api.NeedsOracleCallTargets |
			api.NeedsOracleSupertypes |
			api.NeedsOracleMembers,
		Confidence: r.Confidence(), Implementation: r,
		Oracle: &api.OracleFilter{Identifiers: []string{"ObjectAnimator", "ofFloat", "ofInt", "ofObject"}},
		Check:  r.check,
	})
}

func registerAndroidCorrectnessChecksOnClick() {
	r := &OnClickRule{AndroidRule: alcRule("OnClick", "android:onClick handler missing or has wrong signature", ALSError, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		Needs:       api.NeedsResources | api.NeedsParsedFiles,
		AndroidDeps: uint32(AndroidDepLayout),
		Confidence:  r.Confidence(), Implementation: r,
		Check: r.check,
	})
}

func registerAndroidCorrectnessChecksPropertyEscape() {
	r := &PropertyEscapeRule{AndroidRule: alcRule("PropertyEscape", "Invalid property file escapes", ALSError, 5)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"string_literal", "line_string_literal"}, Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
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
						ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Invalid escape sequence '\\"+string(next)+"' in string literal.")
						i++
					}
				}
			}
		},
	})
}

func registerAndroidCorrectnessChecksShortAlarm() {
	r := &ShortAlarmRule{AndroidRule: alcRule("ShortAlarm", "Short or frequent alarm", ALSWarning, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallExpressionName(file, idx) != "setRepeating" {
				return
			}
			// setRepeating(type, triggerAtMillis, intervalMillis, operation); the receiver is
			// not typed here, so require the file to mention AlarmManager.
			if !bytes.Contains(file.Content, []byte("AlarmManager")) {
				return
			}
			_, args := flatCallExpressionParts(file, idx)
			if args == 0 {
				return
			}
			// Android Lint only checks the four-parameter overload.
			params := shortAlarmParameters(file, args)
			if params == nil {
				return
			}
			for _, check := range shortAlarmChecks {
				expr := flatValueArgumentExpression(file, params[check.index])
				// Lint reports any known value below the minimum, zero and negatives included.
				if val, ok := shortAlarmConstantMillis(file, expr); ok && val < check.min {
					ctx.EmitAtNode(expr, fmt.Sprintf("Value will be forced up to %d as of Android 5.1; don't rely on this to be exact", check.min))
				}
			}
		},
	})
}

// shortAlarmChecks are the triggerAtMillis and intervalMillis minimums Lint enforces.
var shortAlarmChecks = []struct {
	index int
	min   int64
}{{1, 5000}, {2, 60000}}

// shortAlarmParamNames are the parameter names of setRepeating(int, long, long, PendingIntent).
var shortAlarmParamNames = [4]string{"type", "triggerAtMillis", "intervalMillis", "operation"}

// shortAlarmParameters maps the four setRepeating parameters to their arguments, honoring
// named arguments. It returns nil unless the call has exactly four arguments.
func shortAlarmParameters(file *scanner.File, args uint32) *[4]uint32 {
	var out [4]uint32
	var positional []uint32
	count := 0
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		count++
		label := flatValueArgumentLabel(file, arg)
		if label == "" {
			positional = append(positional, arg)
			continue
		}
		slot := -1
		for i, name := range shortAlarmParamNames {
			if name == label {
				slot = i
			}
		}
		if slot < 0 || out[slot] != 0 {
			return nil
		}
		out[slot] = arg
	}
	if count != len(out) {
		return nil
	}
	for i := range out {
		if out[i] == 0 && len(positional) > 0 {
			out[i], positional = positional[0], positional[1:]
		}
		if out[i] == 0 {
			return nil
		}
	}
	return &out
}

// shortAlarmConstantMillis folds integer literals combined with * and +.
func shortAlarmConstantMillis(file *scanner.File, idx uint32) (int64, bool) {
	idx = flatUnwrapParenExpr(file, idx)
	switch file.FlatType(idx) {
	case "integer_literal", "long_literal", "hex_literal", "bin_literal":
		return parseKotlinIntegerLiteral(file.FlatNodeText(idx))
	case "prefix_expression":
		text := strings.TrimSpace(file.FlatNodeText(idx))
		if !strings.HasPrefix(text, "-") {
			return 0, false
		}
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) && file.FlatType(child) != "unary_prefix_operator" {
				v, ok := shortAlarmConstantMillis(file, child)
				return -v, ok
			}
		}
		return 0, false
	case "multiplicative_expression", "additive_expression":
		var values []int64
		op := ""
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				op = file.FlatNodeText(child)
				continue
			}
			v, ok := shortAlarmConstantMillis(file, child)
			if !ok {
				return 0, false
			}
			values = append(values, v)
		}
		if len(values) != 2 {
			return 0, false
		}
		switch op {
		case "*":
			return values[0] * values[1], true
		case "+":
			return values[0] + values[1], true
		}
	}
	return 0, false
}

func registerAndroidCorrectnessChecksLocalSuppress() {
	r := &LocalSuppressRule{AndroidRule: alcRule("LocalSuppress", "@SuppressLint misuse", ALSError, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"annotation"}, Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
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
					// The known list is partial and Lint accepts third-party IDs, so report only near-misses of a known ID.
					if isKnownLintIssueID(issueID) {
						continue
					}
					if near := nearestKnownLintIssueID(issueID); near != "" {
						ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, fmt.Sprintf("@SuppressLint(\"%s\"): '%s' is not a known Android Lint issue ID. Did you mean '%s'?", issueID, issueID, near))
					}
				}
			}
		},
	})
}

// pluralsEqualsOneRe matches `== 1` or `== 1L` but not `== 10`, `== 1.5` or `== 1x`.
var pluralsEqualsOneRe = regexp.MustCompile(`==\s*1[lL]?($|[^0-9.A-Za-z_])`)

func registerAndroidCorrectnessChecksPluralsCandidate() {
	r := &PluralsCandidateRule{AndroidRule: alcRule("PluralsCandidate", "Potential plurals", ALSWarning, 5)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"if_expression", "when_expression"}, Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
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
				if !pluralsEqualsOneRe.MatchString(condText) {
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
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Manual pluralization detected. Use getQuantityString() for proper plural handling.")
			}
		},
	})
}

// lastDottedSegment returns the text after the final dot, so `View.GONE` yields `GONE`.
func lastDottedSegment(text string) string {
	if dot := strings.LastIndex(text, "."); dot >= 0 {
		return text[dot+1:]
	}
	return text
}
