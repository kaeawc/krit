package rules

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/oracle"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

type OverrideAbstractRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *OverrideAbstractRule) Confidence() float64 { return 0.75 }

var abstractClassRequirements = map[string][]string{"Service": {"onBind"}, "BroadcastReceiver": {"onReceive"}, "ContentProvider": {"onCreate", "query", "insert", "update", "delete", "getType"}}

type ParcelCreatorRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ParcelCreatorRule) Confidence() float64 { return 0.75 }

type SwitchIntDefRule struct {
	LineBase
	AndroidRule
}

var whenVisibilityRe = regexp.MustCompile(`when\s*\([^)]*visibility[^)]*\)\s*\{`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *SwitchIntDefRule) Confidence() float64 { return 0.75 }

func (r *SwitchIntDefRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if !whenVisibilityRe.MatchString(line) {
			continue
		}
		hasVisible, hasInvisible, hasGone, hasElse := false, false, false, false
		depth := 0
		for j := i; j < len(file.Lines); j++ {
			l := file.Lines[j]
			depth += strings.Count(l, "{") - strings.Count(l, "}")
			if strings.Contains(l, "VISIBLE") && !strings.Contains(l, "INVISIBLE") {
				hasVisible = true
			}
			if strings.Contains(l, "INVISIBLE") {
				hasInvisible = true
			}
			if strings.Contains(l, "GONE") {
				hasGone = true
			}
			if strings.Contains(l, "else") {
				hasElse = true
			}
			if depth <= 0 && j > i {
				break
			}
		}
		if hasElse {
			continue
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
			ctx.Emit(r.Finding(file, i+1, 1, "when on visibility missing constants: "+strings.Join(missing, ", ")+". Add them or an else branch."))
		}
	}
}

type TextViewEditsRule struct {
	LineBase
	AndroidRule
}

var textViewEditsRe = regexp.MustCompile(`\beditText\w*\.setText\b`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *TextViewEditsRule) Confidence() float64 { return 0.75 }

func (r *TextViewEditsRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if textViewEditsRe.MatchString(line) {
			ctx.Emit(r.Finding(file, i+1, 1, "Using setText on an EditText. Consider using Editable or getText()."))
		}
	}
}

type WrongViewCastRule struct {
	LineBase
	AndroidRule
}

var viewIdPrefixMap = map[string][]string{"btn_": {"Button", "MaterialButton", "AppCompatButton", "ImageButton", "ToggleButton", "RadioButton", "CompoundButton"}, "button_": {"Button", "MaterialButton", "AppCompatButton", "ImageButton", "ToggleButton", "RadioButton", "CompoundButton"}, "tv_": {"TextView", "AppCompatTextView", "MaterialTextView"}, "text_": {"TextView", "AppCompatTextView", "MaterialTextView"}, "iv_": {"ImageView", "AppCompatImageView", "ShapeableImageView"}, "img_": {"ImageView", "AppCompatImageView", "ShapeableImageView"}, "image_": {"ImageView", "AppCompatImageView", "ShapeableImageView"}, "rv_": {"RecyclerView"}, "recycler_": {"RecyclerView"}, "et_": {"EditText", "AppCompatEditText", "TextInputEditText"}, "edit_": {"EditText", "AppCompatEditText", "TextInputEditText"}, "input_": {"EditText", "AppCompatEditText", "TextInputEditText"}}

var (
	findViewByIdGenericRe = regexp.MustCompile(`findViewById<(\w+)>\s*\(\s*R\.id\.(\w+)\)`)
	findViewByIdCastRe    = regexp.MustCompile(`findViewById\s*\(\s*R\.id\.(\w+)\)\s+as\s+(\w+)`)
)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *WrongViewCastRule) Confidence() float64 { return 0.75 }

func (r *WrongViewCastRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if scanner.IsCommentLine(line) {
			continue
		}
		var castType, idName string
		if m := findViewByIdGenericRe.FindStringSubmatch(line); m != nil {
			castType = m[1]
			idName = m[2]
		} else if m := findViewByIdCastRe.FindStringSubmatch(line); m != nil {
			idName = m[1]
			castType = m[2]
		}
		if castType == "" || idName == "" {
			continue
		}
		idLower := strings.ToLower(idName)
		for prefix, expectedTypes := range viewIdPrefixMap {
			if strings.HasPrefix(idLower, prefix) {
				compatible := false
				for _, et := range expectedTypes {
					if castType == et {
						compatible = true
						break
					}
				}
				if !compatible {
					ctx.Emit(r.Finding(file, i+1, 1, "Suspicious cast: id '"+idName+"' (prefix '"+prefix+"') suggests "+expectedTypes[0]+", but cast to "+castType+"."))
				}
				break
			}
		}
	}
}

type DeprecatedRule struct {
	LineBase
	AndroidRule
}

type deprecatedApiEntry struct {
	Pattern string
	Message string
}

var deprecatedApis = []deprecatedApiEntry{{"AsyncTask", "AsyncTask is deprecated as of API 30. Use java.util.concurrent or Kotlin coroutines instead."}, {"IntentService", "IntentService is deprecated as of API 30. Use WorkManager or JobIntentService instead."}, {"PreferenceActivity", "PreferenceActivity is deprecated as of API 29. Use PreferenceFragmentCompat instead."}, {"CursorLoader", "CursorLoader is deprecated as of API 28. Use Room with LiveData or Flow instead."}, {"LocalBroadcastManager", "LocalBroadcastManager is deprecated. Use LiveData, Flow, or other observable patterns instead."}, {"TabActivity", "TabActivity is deprecated as of API 13. Use tabs with Fragment/ViewPager instead."}, {"ActivityGroup", "ActivityGroup is deprecated as of API 13. Use Fragment-based navigation instead."}, {"getRunningTasks", "getRunningTasks is deprecated as of API 21. It returns only the caller's own tasks for privacy."}, {"DefaultHttpClient", "DefaultHttpClient is deprecated as of API 22. Use HttpURLConnection or OkHttp instead."}, {"AndroidHttpClient", "AndroidHttpClient is deprecated as of API 22. Use HttpURLConnection or OkHttp instead."}}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *DeprecatedRule) Confidence() float64 { return 0.75 }

func (r *DeprecatedRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || scanner.IsCommentLine(line) || strings.HasPrefix(trimmed, "import ") {
			continue
		}
		for _, entry := range deprecatedApis {
			if strings.Contains(line, entry.Pattern) {
				ctx.Emit(r.Finding(file, i+1, 1, entry.Message))
				break
			}
		}
	}
}

type RangeRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *RangeRule) NodeTypes() []string { return []string{"call_expression"} }

// Confidence reports a tier-2 (medium) base confidence. The rule is
// structural and requires an explicit API/range anchor, but framework
// calls still depend on type-oracle call-target availability.
// Classified per roadmap/17.
func (r *RangeRule) Confidence() float64 { return 0.75 }

func (r *RangeRule) check(ctx *v2.Context) {
	if ctx == nil || ctx.File == nil || ctx.Idx == 0 {
		return
	}
	file := ctx.File
	call := ctx.Idx
	if file.FlatType(call) != "call_expression" {
		return
	}
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return
	}
	callName := flatCallExpressionName(file, call)
	if callName == "" {
		return
	}

	summary := rangeSummaryForFile(file)
	specs := rangeSpecsForCall(ctx, call, callName, args, summary)
	if len(specs) == 0 {
		return
	}
	for _, spec := range specs {
		arg := rangeArgumentForSpec(file, args, spec)
		if arg == 0 {
			continue
		}
		expr := rangeValueArgumentExpression(file, arg)
		value, valueNode, ok := rangeEvaluateNumericExpr(file, expr, summary.constants, nil)
		if !ok || !spec.bounds.outside(value.value) {
			continue
		}
		if valueNode == 0 {
			valueNode = expr
		}
		ctx.EmitAt(file.FlatRow(valueNode)+1, file.FlatCol(valueNode)+1, rangeFindingMessage(spec, rangeNumberText(file, valueNode)))
	}
}

type rangeBounds struct {
	from          float64
	to            float64
	fromInclusive bool
	toInclusive   bool
}

type rangeArgSpec struct {
	index     int
	name      string
	callLabel string
	argLabel  string
	bounds    rangeBounds
}

type rangeNumber struct {
	value float64
	isInt bool
	node  uint32
}

type rangeFileSummary struct {
	constants map[string]rangeNumber
	functions map[string][]rangeFunctionSpec
}

type rangeFunctionSpec struct {
	name   string
	params []rangeArgSpec
}

var rangeSummaryCache sync.Map

func rangeSummaryForFile(file *scanner.File) *rangeFileSummary {
	if file == nil {
		return &rangeFileSummary{constants: map[string]rangeNumber{}, functions: map[string][]rangeFunctionSpec{}}
	}
	key := file.Path + "\x00" + strconv.Itoa(len(file.Content))
	if cached, ok := rangeSummaryCache.Load(key); ok {
		return cached.(*rangeFileSummary)
	}
	summary := &rangeFileSummary{
		constants: map[string]rangeNumber{},
		functions: map[string][]rangeFunctionSpec{},
	}
	file.FlatWalkNodes(0, "property_declaration", func(idx uint32) {
		name, value, ok := rangeConstantProperty(file, idx, summary.constants)
		if ok && name != "" {
			summary.constants[name] = value
		}
	})
	file.FlatWalkNodes(0, "function_declaration", func(idx uint32) {
		fn := rangeFunctionSpec{name: extractIdentifierFlat(file, idx)}
		if fn.name == "" {
			return
		}
		params, ok := file.FlatFindChild(idx, "function_value_parameters")
		if !ok {
			return
		}
		paramIndex := 0
		for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) != "parameter" {
				continue
			}
			if spec, ok := rangeAnnotatedParameterSpec(file, child, paramIndex, fn.name); ok {
				fn.params = append(fn.params, spec)
			}
			paramIndex++
		}
		if len(fn.params) > 0 {
			summary.functions[fn.name] = append(summary.functions[fn.name], fn)
		}
	})
	actual, _ := rangeSummaryCache.LoadOrStore(key, summary)
	return actual.(*rangeFileSummary)
}

func rangeConstantProperty(file *scanner.File, idx uint32, constants map[string]rangeNumber) (string, rangeNumber, bool) {
	if file == nil || file.FlatType(idx) != "property_declaration" || rangePropertyIsVar(file, idx) {
		return "", rangeNumber{}, false
	}
	name := ""
	if vd, ok := file.FlatFindChild(idx, "variable_declaration"); ok {
		name = extractIdentifierFlat(file, vd)
	}
	if name == "" {
		name = extractIdentifierFlat(file, idx)
	}
	init := rangePropertyInitializer(file, idx)
	value, _, ok := rangeEvaluateNumericExpr(file, init, constants, nil)
	return name, value, ok
}

func rangePropertyIsVar(file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "var":
			return true
		case "val":
			return false
		}
	}
	return false
}

func rangePropertyInitializer(file *scanner.File, idx uint32) uint32 {
	seenEquals := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			seenEquals = true
			continue
		}
		if seenEquals && file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

func rangeAnnotatedParameterSpec(file *scanner.File, param uint32, index int, callLabel string) (rangeArgSpec, bool) {
	paramName := extractIdentifierFlat(file, param)
	for _, ann := range rangeParameterAnnotations(file, param) {
		annName := androidAnnotationSimpleName(file.FlatNodeText(ann))
		if annName != "IntRange" && annName != "FloatRange" {
			continue
		}
		bounds, ok := rangeBoundsFromAnnotation(file, ann)
		if !ok {
			continue
		}
		return rangeArgSpec{
			index:     index,
			name:      paramName,
			callLabel: callLabel,
			argLabel:  "argument",
			bounds:    bounds,
		}, true
	}
	return rangeArgSpec{}, false
}

func rangeParameterAnnotations(file *scanner.File, param uint32) []uint32 {
	var out []uint32
	if mods, ok := file.FlatFindChild(param, "modifiers"); ok {
		for ann := file.FlatFirstChild(mods); ann != 0; ann = file.FlatNextSib(ann) {
			if file.FlatType(ann) == "annotation" {
				out = append(out, ann)
			}
		}
	}
	for prev, ok := file.FlatPrevSibling(param); ok; prev, ok = file.FlatPrevSibling(prev) {
		switch file.FlatType(prev) {
		case "annotation":
			out = append(out, prev)
		case "modifiers", "parameter_modifiers":
			for ann := file.FlatFirstChild(prev); ann != 0; ann = file.FlatNextSib(ann) {
				if file.FlatType(ann) == "annotation" {
					out = append(out, ann)
				}
			}
		default:
			if file.FlatIsNamed(prev) {
				return out
			}
		}
	}
	return out
}

func rangeBoundsFromAnnotation(file *scanner.File, ann uint32) (rangeBounds, bool) {
	bounds := rangeBounds{
		from:          math.Inf(-1),
		to:            math.Inf(1),
		fromInclusive: true,
		toInclusive:   true,
	}
	args := rangeAnnotationValueArguments(file, ann)
	if args == 0 {
		return bounds, false
	}
	found := false
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		label := flatValueArgumentLabel(file, arg)
		expr := rangeValueArgumentExpression(file, arg)
		switch label {
		case "from":
			if value, _, ok := rangeEvaluateNumericExpr(file, expr, nil, nil); ok {
				bounds.from = value.value
				found = true
			}
		case "to":
			if value, _, ok := rangeEvaluateNumericExpr(file, expr, nil, nil); ok {
				bounds.to = value.value
				found = true
			}
		case "fromInclusive":
			if value, ok := rangeBoolLiteral(file, expr); ok {
				bounds.fromInclusive = value
			}
		case "toInclusive":
			if value, ok := rangeBoolLiteral(file, expr); ok {
				bounds.toInclusive = value
			}
		}
	}
	return bounds, found
}

func rangeAnnotationValueArguments(file *scanner.File, ann uint32) uint32 {
	var args uint32
	file.FlatWalkNodes(ann, "value_arguments", func(idx uint32) {
		if args == 0 {
			args = idx
		}
	})
	return args
}

func rangeSpecsForCall(ctx *v2.Context, call uint32, callName string, args uint32, summary *rangeFileSummary) []rangeArgSpec {
	if target := rangeOracleCallTarget(ctx, call); target != "" {
		if specs := rangeFrameworkSpecsForTarget(target, callName); len(specs) > 0 {
			return specs
		}
		return nil
	}
	if summary == nil {
		return nil
	}
	candidates := summary.functions[callName]
	if len(candidates) == 0 {
		return nil
	}
	argCount := rangeValueArgumentCount(ctx.File, args)
	var out []rangeArgSpec
	for _, candidate := range candidates {
		if rangeMaxSpecIndex(candidate.params) >= argCount {
			continue
		}
		out = append(out, candidate.params...)
	}
	return out
}

func rangeOracleCallTarget(ctx *v2.Context, call uint32) string {
	if ctx == nil || ctx.Resolver == nil || ctx.File == nil {
		return ""
	}
	cr, ok := ctx.Resolver.(*oracle.CompositeResolver)
	if !ok {
		return ""
	}
	lookup := cr.Oracle()
	if lookup == nil {
		return ""
	}
	return lookup.LookupCallTarget(ctx.File.Path, ctx.File.FlatRow(call)+1, ctx.File.FlatCol(call)+1)
}

func rangeFrameworkSpecsForTarget(target, callName string) []rangeArgSpec {
	target = strings.TrimSpace(target)
	switch {
	case rangeTargetMatches(target, callName, "android.graphics.Color.argb"):
		return []rangeArgSpec{
			rangeFrameworkSpec(0, "alpha", "Color.argb", "argument", 0, 255),
			rangeFrameworkSpec(1, "red", "Color.argb", "argument", 0, 255),
			rangeFrameworkSpec(2, "green", "Color.argb", "argument", 0, 255),
			rangeFrameworkSpec(3, "blue", "Color.argb", "argument", 0, 255),
		}
	case rangeTargetMatches(target, callName, "android.graphics.Color.rgb"):
		return []rangeArgSpec{
			rangeFrameworkSpec(0, "red", "Color.rgb", "argument", 0, 255),
			rangeFrameworkSpec(1, "green", "Color.rgb", "argument", 0, 255),
			rangeFrameworkSpec(2, "blue", "Color.rgb", "argument", 0, 255),
		}
	case callName == "setAlpha" && rangeTargetAny(target,
		"android.graphics.Paint.setAlpha",
		"android.graphics.drawable.Drawable.setAlpha",
		"android.view.View.setAlpha",
		"android.widget.ImageView.setAlpha"):
		return []rangeArgSpec{rangeFrameworkSpec(0, "alpha", "setAlpha", "value", 0, 255)}
	case callName == "setProgress" && rangeTargetAny(target,
		"android.widget.ProgressBar.setProgress",
		"android.widget.AbsSeekBar.setProgress"):
		return []rangeArgSpec{rangeFrameworkSpec(0, "progress", "setProgress", "value", 0, 100)}
	case callName == "setRotation" && rangeTargetAny(target,
		"android.view.View.setRotation"):
		return []rangeArgSpec{rangeFrameworkSpec(0, "rotation", "setRotation", "value", -360, 360)}
	default:
		return nil
	}
}

func rangeFrameworkSpec(index int, name, callLabel, argLabel string, from, to float64) rangeArgSpec {
	return rangeArgSpec{
		index:     index,
		name:      name,
		callLabel: callLabel,
		argLabel:  argLabel,
		bounds: rangeBounds{
			from:          from,
			to:            to,
			fromInclusive: true,
			toInclusive:   true,
		},
	}
}

func rangeTargetMatches(target, callName, want string) bool {
	return callName == simpleQualifiedName(want) && rangeTargetAny(target, want)
}

func rangeTargetAny(target string, wants ...string) bool {
	for _, want := range wants {
		if target == want || strings.HasSuffix(target, "."+want) || strings.Contains(target, want+"(") {
			return true
		}
	}
	return false
}

func rangeArgumentForSpec(file *scanner.File, args uint32, spec rangeArgSpec) uint32 {
	if spec.name != "" {
		if arg := flatNamedValueArgument(file, args, spec.name); arg != 0 {
			return arg
		}
	}
	return flatPositionalValueArgument(file, args, spec.index)
}

func rangeValueArgumentCount(file *scanner.File, args uint32) int {
	count := 0
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) == "value_argument" {
			count++
		}
	}
	return count
}

func rangeMaxSpecIndex(specs []rangeArgSpec) int {
	max := -1
	for _, spec := range specs {
		if spec.index > max {
			max = spec.index
		}
	}
	return max
}

func rangeValueArgumentExpression(file *scanner.File, arg uint32) uint32 {
	if file == nil || arg == 0 {
		return 0
	}
	if !flatHasValueArgumentLabel(file, arg) {
		return flatValueArgumentExpression(file, arg)
	}
	seenEquals := false
	for child := file.FlatFirstChild(arg); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			seenEquals = true
			continue
		}
		if seenEquals && file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

func rangeEvaluateNumericExpr(file *scanner.File, idx uint32, constants map[string]rangeNumber, seen map[string]bool) (rangeNumber, uint32, bool) {
	if file == nil || idx == 0 {
		return rangeNumber{}, 0, false
	}
	idx = flatUnwrapParenExpr(file, idx)
	switch file.FlatType(idx) {
	case "integer_literal":
		if value, ok := rangeParseIntegerLiteral(file.FlatNodeText(idx)); ok {
			return rangeNumber{value: float64(value), isInt: true, node: idx}, idx, true
		}
	case "real_literal":
		if value, ok := rangeParseFloatLiteral(file.FlatNodeText(idx)); ok {
			return rangeNumber{value: value, node: idx}, idx, true
		}
	case "simple_identifier":
		name := file.FlatNodeString(idx, nil)
		if constants == nil || constants[name].node == 0 {
			return rangeNumber{}, 0, false
		}
		if seen == nil {
			seen = map[string]bool{}
		}
		if seen[name] {
			return rangeNumber{}, 0, false
		}
		seen[name] = true
		value := constants[name]
		return value, idx, true
	case "unary_expression", "prefix_expression":
		if !rangeHasMinusPrefix(file, idx) {
			return rangeNumber{}, 0, false
		}
		operand := rangeFirstNamedChildAfterPrefix(file, idx)
		value, _, ok := rangeEvaluateNumericExpr(file, operand, constants, seen)
		if !ok {
			return rangeNumber{}, 0, false
		}
		value.value = -value.value
		value.node = idx
		return value, idx, true
	}
	return rangeNumber{}, 0, false
}

func rangeHasMinusPrefix(file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "-" {
			return true
		}
		if file.FlatIsNamed(child) {
			return false
		}
	}
	return false
}

func rangeFirstNamedChildAfterPrefix(file *scanner.File, idx uint32) uint32 {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

func rangeParseIntegerLiteral(text string) (int64, bool) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(text), "_", "")
	cleaned = strings.TrimSuffix(strings.TrimSuffix(cleaned, "L"), "l")
	if cleaned == "" {
		return 0, false
	}
	value, err := strconv.ParseInt(cleaned, 0, 64)
	return value, err == nil
}

func rangeParseFloatLiteral(text string) (float64, bool) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(text), "_", "")
	cleaned = strings.TrimRight(cleaned, "fFdD")
	if cleaned == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(cleaned, 64)
	return value, err == nil
}

func rangeBoolLiteral(file *scanner.File, idx uint32) (bool, bool) {
	idx = flatUnwrapParenExpr(file, idx)
	if file == nil || idx == 0 || file.FlatType(idx) != "boolean_literal" {
		return false, false
	}
	switch file.FlatNodeString(idx, nil) {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return false, false
	}
}

func (b rangeBounds) outside(value float64) bool {
	if b.fromInclusive {
		if value < b.from {
			return true
		}
	} else if value <= b.from {
		return true
	}
	if b.toInclusive {
		return value > b.to
	}
	return value >= b.to
}

func rangeFindingMessage(spec rangeArgSpec, valueText string) string {
	label := spec.argLabel
	if label == "" {
		label = "argument"
	}
	return fmt.Sprintf("%s() %s %s is outside the valid range %s.", spec.callLabel, label, valueText, spec.bounds.String())
}

func (b rangeBounds) String() string {
	left, right := "[", "]"
	if !b.fromInclusive {
		left = "("
	}
	if !b.toInclusive {
		right = ")"
	}
	return fmt.Sprintf("%s%s, %s%s", left, rangeBoundString(b.from), rangeBoundString(b.to), right)
}

func rangeBoundString(value float64) string {
	switch {
	case math.IsInf(value, -1):
		return "-inf"
	case math.IsInf(value, 1):
		return "inf"
	case math.Trunc(value) == value:
		return strconv.FormatInt(int64(value), 10)
	default:
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
}

func rangeNumberText(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	return strings.TrimSpace(file.FlatNodeText(idx))
}

type ResourceTypeRule struct {
	LineBase
	AndroidRule
}

var resourceMethodExpected = map[string]string{"getString": "string", "getText": "string", "getQuantityString": "plurals", "getStringArray": "array", "getIntArray": "array", "getDrawable": "drawable", "setImageResource": "drawable", "setImageDrawable": "drawable", "setContentView": "layout", "inflate": "layout", "getColor": "color", "getColorStateList": "color", "getDimension": "dimen", "getDimensionPixelSize": "dimen", "getDimensionPixelOffset": "dimen", "getBoolean": "bool", "getInteger": "integer", "getAnimation": "anim", "getLayout": "layout"}
var resourceCallRe = regexp.MustCompile(`\b(\w+)\s*\(\s*R\.(\w+)\.(\w+)`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ResourceTypeRule) Confidence() float64 { return 0.75 }

func (r *ResourceTypeRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if scanner.IsCommentLine(line) {
			continue
		}
		for _, m := range resourceCallRe.FindAllStringSubmatch(line, -1) {
			if expected, ok := resourceMethodExpected[m[1]]; ok && m[2] != expected {
				ctx.Emit(r.Finding(file, i+1, 1, fmt.Sprintf("%s(R.%s.%s): expected R.%s resource, not R.%s.", m[1], m[2], m[3], expected, m[2])))
			}
		}
	}
}

type ResourceAsColorRule struct {
	LineBase
	AndroidRule
}

var resAsColorRe = regexp.MustCompile(`\.(setBackgroundColor|setTextColor|setColor)\s*\(\s*R\.`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ResourceAsColorRule) Confidence() float64 { return 0.75 }

func (r *ResourceAsColorRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if resAsColorRe.MatchString(line) {
			ctx.Emit(r.Finding(file, i+1, 1, "Passing a resource ID where a color value is expected. Use ContextCompat.getColor() instead."))
		}
	}
}

type SupportAnnotationUsageRule struct {
	LineBase
	AndroidRule
}

var mainThreadAnnotRe = regexp.MustCompile(`@MainThread`)
var ioMethodPatterns = []string{"HttpURLConnection", "OkHttpClient", "FileInputStream", "FileOutputStream", "BufferedReader", "BufferedWriter", "URLConnection", "HttpClient", "Retrofit", "Socket(", "ServerSocket(", "DatagramSocket(", "FileReader", "FileWriter", "RandomAccessFile", "openConnection("}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *SupportAnnotationUsageRule) Confidence() float64 { return 0.75 }

func (r *SupportAnnotationUsageRule) check(ctx *v2.Context) {
	file := ctx.File
	inMainThreadFun := false
	mainThreadLine := 0
	braceDepth := 0
	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if mainThreadAnnotRe.MatchString(trimmed) {
			inMainThreadFun = true
			mainThreadLine = i + 1
			continue
		}
		if inMainThreadFun && braceDepth == 0 {
			if strings.Contains(line, "{") {
				braceDepth = 1
			}
			if !strings.Contains(line, "fun ") && !strings.Contains(line, "{") && !strings.HasPrefix(trimmed, "@") && trimmed != "" {
				inMainThreadFun = false
			}
			continue
		}
		if inMainThreadFun && braceDepth > 0 {
			braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
			if braceDepth <= 0 {
				inMainThreadFun = false
				braceDepth = 0
				continue
			}
			for _, pat := range ioMethodPatterns {
				if strings.Contains(line, pat) {
					ctx.Emit(r.Finding(file, i+1, 1, fmt.Sprintf("@MainThread function (line %d) performs IO/network operation (%s). This may block the UI thread.", mainThreadLine, pat)))
					break
				}
			}
		}
	}
}

type AccidentalOctalRule struct {
	LineBase
	AndroidRule
}

var accidentalOctalRe = regexp.MustCompile(`\b0\d{2,}\b`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *AccidentalOctalRule) Confidence() float64 { return 0.75 }

func (r *AccidentalOctalRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if !scanner.IsCommentLine(line) && accidentalOctalRe.MatchString(line) && !strings.Contains(line, "0x") && !strings.Contains(line, "0b") {
			ctx.Emit(r.Finding(file, i+1, 1, "Suspicious leading zero \u2014 this may be an accidental octal literal."))
		}
	}
}

type AppCompatMethodRule struct {
	LineBase
	AndroidRule
}

var appCompatMethodRe = regexp.MustCompile(`\b(getActionBar|setProgressBarVisibility|setProgressBarIndeterminateVisibility)\s*\(`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *AppCompatMethodRule) Confidence() float64 { return 0.75 }

func (r *AppCompatMethodRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if appCompatMethodRe.MatchString(line) && !strings.Contains(line, "getSupportActionBar") {
			ctx.Emit(r.Finding(file, i+1, 1, "Use AppCompat equivalent methods for backward compatibility."))
		}
	}
}

type CustomViewStyleableRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *CustomViewStyleableRule) Confidence() float64 { return 0.75 }

var obtainStyledAttrsRe = regexp.MustCompile(`obtainStyledAttributes\s*\(\s*\w+\s*,\s*R\.styleable\.(\w+)`)

type DalvikOverrideRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *DalvikOverrideRule) Confidence() float64 { return 0.75 }

type InnerclassSeparatorRule struct {
	LineBase
	AndroidRule
}

var innerclassSepRe = regexp.MustCompile(`"[^"]*\w/\w+\w"`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *InnerclassSeparatorRule) Confidence() float64 { return 0.75 }

func (r *InnerclassSeparatorRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if strings.Contains(line, "Class.forName") && innerclassSepRe.MatchString(line) && !strings.Contains(line, "$") {
			ctx.Emit(r.Finding(file, i+1, 1, "Use '$' instead of '/' as inner class separator in class names."))
		}
	}
}

type ObjectAnimatorBindingRule struct {
	LineBase
	AndroidRule
}

var objAnimatorRe = regexp.MustCompile(`ObjectAnimator\.of(?:Float|Int|Object)\s*\([^,]+,\s*"([^"]+)"`)
var knownAnimatorProperties = map[string]bool{"alpha": true, "translationX": true, "translationY": true, "translationZ": true, "rotation": true, "rotationX": true, "rotationY": true, "scaleX": true, "scaleY": true, "x": true, "y": true, "z": true, "elevation": true}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ObjectAnimatorBindingRule) Confidence() float64 { return 0.75 }

func (r *ObjectAnimatorBindingRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if !scanner.IsCommentLine(line) {
			if m := objAnimatorRe.FindStringSubmatch(line); m != nil && !knownAnimatorProperties[m[1]] {
				ctx.Emit(r.Finding(file, i+1, 1, "ObjectAnimator property \""+m[1]+"\" is not a standard View property. Verify the target has a setter for this property."))
			}
		}
	}
}

type OnClickRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *OnClickRule) Confidence() float64 { return 0.75 }

type PropertyEscapeRule struct {
	LineBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *PropertyEscapeRule) Confidence() float64 { return 0.75 }

func (r *PropertyEscapeRule) check(ctx *v2.Context) {
	file := ctx.File
	inMultilineString := false
	for i, line := range file.Lines {
		if scanner.IsCommentLine(line) {
			continue
		}
		if strings.Count(line, `"""`)%2 != 0 {
			inMultilineString = !inMultilineString
		}
		if inMultilineString {
			continue
		}
		inString := false
		for j := 0; j < len(line); j++ {
			ch := line[j]
			if ch == '"' && (j == 0 || line[j-1] != '\\') {
				inString = !inString
				continue
			}
			if inString && ch == '\\' && j+1 < len(line) {
				next := line[j+1]
				switch next {
				case 'n', 't', 'r', '\\', '"', '\'', '$', 'b', 'u', 'f':
					j++
				default:
					if next >= '0' && next <= '9' {
						j++
						continue
					}
					ctx.Emit(r.Finding(file, i+1, 1, "Invalid escape sequence '\\"+string(next)+"' in string literal."))
					j++
				}
			}
		}
	}
}

type ShortAlarmRule struct {
	LineBase
	AndroidRule
}

var shortAlarmRe = regexp.MustCompile(`\b(setRepeating|setInexactRepeating)\s*\(`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ShortAlarmRule) Confidence() float64 { return 0.75 }

func (r *ShortAlarmRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if shortAlarmRe.MatchString(line) && (strings.Contains(line, "1000") || strings.Contains(line, "5000") || strings.Contains(line, "10000") || strings.Contains(line, "30000")) {
			ctx.Emit(r.Finding(file, i+1, 1, "Short alarm interval. Consider using a minimum of 60 seconds for repeating alarms."))
		}
	}
}

type LocalSuppressRule struct {
	LineBase
	AndroidRule
}

var suppressLintRe = regexp.MustCompile(`@SuppressLint\s*\(\s*"([^"]+)"`)
var knownLintIssueIDs = map[string]bool{"NewApi": true, "InlinedApi": true, "Override": true, "UnusedResources": true, "HardcodedText": true, "MissingTranslation": true, "ExtraTranslation": true, "MissingPermission": true, "ContentDescription": true, "ObsoleteLayoutParam": true, "ViewHolder": true, "LogConditional": true, "SdCardPath": true, "Wakelock": true, "SetJavaScriptEnabled": true, "ExportedService": true, "PackagedPrivateKey": true, "ValidFragment": true, "ViewConstructor": true, "WrongImport": true, "ServiceCast": true, "LayoutInflation": true, "ShowToast": true, "PackageManagerGetSignatures": true, "UseSparseArrays": true, "UseValueOf": true, "LongLogTag": true, "LogTagMismatch": true, "UnlocalizedSms": true, "ViewTag": true, "ShortAlarm": true, "UniqueConstants": true, "ShiftFlags": true, "AccidentalOctal": true, "AppCompatMethod": true, "CheckResult": true, "CommitPrefEdits": true, "CommitTransaction": true, "CustomViewStyleable": true, "CutPasteId": true, "DalvikOverride": true, "DefaultLocale": true, "Deprecated": true, "DeviceAdmin": true, "DuplicateActivity": true, "DuplicateIds": true, "DuplicateIncludedIds": true, "DuplicateUsesFeature": true, "ExtraText": true, "FullBackupContent": true, "GradleCompatible": true, "GradleDependency": true, "GradleDeprecated": true, "GradleDynamicVersion": true, "GradleGetter": true, "GradleIdeError": true, "GradleOverrides": true, "GradlePath": true, "IllegalResourceRef": true, "InconsistentArrays": true, "InconsistentLayout": true, "InnerclassSeparator": true, "InvalidResourceFolder": true, "ManifestOrder": true, "ManifestTypo": true, "MissingApplicationIcon": true, "MissingId": true, "MissingRegistered": true, "MissingVersion": true, "MockLocation": true, "MultipleUsesSdk": true, "NestedScrolling": true, "NotSibling": true, "ObjectAnimatorBinding": true, "OnClick": true, "OverrideAbstract": true, "ParcelCreator": true, "PluralsCandidate": true, "PropertyEscape": true, "ProtectedPermissions": true, "Range": true, "Registered": true, "RequiredSize": true, "ResAuto": true, "ResourceAsColor": true, "ResourceType": true, "ScrollViewCount": true, "ScrollViewSize": true, "ServiceExported": true, "SetTextI18n": true, "SimpleDateFormat": true, "SpUsage": true, "StopShip": true, "StringFormatCount": true, "StringFormatInvalid": true, "StringFormatMatches": true, "SupportAnnotationUsage": true, "SwitchIntDef": true, "TextFields": true, "TextViewEdits": true, "UnpackedNativeCode": true, "UnusedAttribute": true, "UnusedNamespace": true, "UseCompoundDrawables": true, "UsesMinSdkAttributes": true, "WrongCall": true, "WrongCase": true, "WrongFolder": true, "WrongManifestParent": true, "WrongRegion": true, "WrongThread": true, "WrongViewCast": true, "Assert": true, "SQLiteString": true, "LocalSuppress": true, "AddJavascriptInterface": true, "GetInstance": true, "EasterEgg": true, "ExportedContentProvider": true, "ExportedReceiver": true, "ExportedPreferenceActivity": true, "GrantAllUris": true, "HardcodedDebugMode": true, "InsecureBaseConfiguration": true, "SecureRandom": true, "TrustedServer": true, "UnprotectedSMSBroadcastReceiver": true, "UnsafeProtectedBroadcastReceiver": true, "UseCheckPermission": true, "WorldReadableFiles": true, "WorldWriteableFiles": true, "DisableBaselineAlignment": true, "DrawAllocation": true, "FieldGetter": true, "FloatMath": true, "HandlerLeak": true, "InefficientWeight": true, "MergeRootFrame": true, "NestedWeights": true, "Overdraw": true, "Recycle": true, "TooDeepLayout": true, "UselessLeaf": true, "ClickableViewAccessibility": true, "LabelFor": true, "ByteOrderMark": true, "RelativeOverlap": true, "IconColors": true, "IconDensities": true, "IconDipSize": true, "IconDuplicates": true, "IconDuplicatesConfig": true, "IconExpectedSize": true, "IconExtension": true, "IconLauncherShape": true, "IconLocation": true, "IconMissingDensityFolder": true, "IconMixedNinePatch": true, "IconNoDpi": true, "IconXmlAndPng": true, "ConvertToWebp": true, "GifUsage": true, "AppCompatResource": true, "AppIndexingError": true, "AppIndexingWarning": true, "BackButton": true, "ButtonCase": true, "ButtonOrder": true, "ButtonStyle": true, "GoogleAppIndexingDeepLinkError": true, "GoogleAppIndexingWarning": true, "NegativeMargin": true, "RtlCompat": true, "RtlEnabled": true, "RtlHardcoded": true, "RtlSymmetry": true, "RtlSuperscript": true, "AllowBackup": true, "AlwaysShowAction": true, "InvalidUsesTagAttribute": true, "OldTargetApi": true, "PermissionImpliesUnsupportedHardware": true, "UnsupportedChromeOsHardware": true, "MissingLeanbackLauncher": true, "MissingLeanbackSupport": true, "Typos": true, "TypographyDashes": true, "TypographyEllipsis": true, "TypographyFractions": true, "TypographyOther": true, "TypographyQuotes": true, "UnusedIds": true, "Autofill": true, "RestrictedApi": true, "VisibleForTests": true, "MissingInflatedId": true, "NotificationPermission": true, "ObsoleteSdkInt": true, "all": true}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *LocalSuppressRule) Confidence() float64 { return 0.75 }

func (r *LocalSuppressRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		for _, m := range suppressLintRe.FindAllStringSubmatch(line, -1) {
			if !knownLintIssueIDs[m[1]] {
				ctx.Emit(r.Finding(file, i+1, 1, fmt.Sprintf("@SuppressLint(\"%s\"): '%s' is not a known Android Lint issue ID.", m[1], m[1])))
			}
		}
	}
}

type PluralsCandidateRule struct {
	LineBase
	AndroidRule
}

var (
	pluralsIfRe   = regexp.MustCompile(`if\s*\(\s*\w+\s*==\s*1\s*\)`)
	pluralsWhenRe = regexp.MustCompile(`when\s*\(\s*(count|num|size|quantity|amount|number)\s*\)`)
)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *PluralsCandidateRule) Confidence() float64 { return 0.75 }

func (r *PluralsCandidateRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if scanner.IsCommentLine(line) {
			continue
		}
		if pluralsIfRe.MatchString(line) && containsStringFormatting(gatherWindow(file.Lines, i, 5)) {
			ctx.Emit(r.Finding(file, i+1, 1, "Manual pluralization detected. Use getQuantityString() for proper plural handling."))
		}
		if pluralsWhenRe.MatchString(line) && containsStringFormatting(gatherWindow(file.Lines, i, 10)) {
			ctx.Emit(r.Finding(file, i+1, 1, "Manual pluralization detected. Use getQuantityString() for proper plural handling."))
		}
	}
}

func gatherWindow(lines []string, i, radius int) string {
	start := i - radius
	if start < 0 {
		start = 0
	}
	end := i + radius + 1
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], "\n")
}

func containsStringFormatting(text string) bool {
	for _, ind := range []string{"\"${", "getString(", "string.", "String.format", "resources.", "R.string.", "\"item", "\"file", "\"message", "\"photo", "\"comment", "\"result", "\"day", "\"hour", "\"minute", "\"second", "plural", "Plural"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(ind)) {
			return true
		}
	}
	return false
}
