package rules

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules/semantics"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
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
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *SwitchIntDefRule) Confidence() float64 { return 0.75 }

type TextViewEditsRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *TextViewEditsRule) Confidence() float64 { return 0.75 }

type WrongViewCastRule struct {
	FlatDispatchBase
	AndroidRule
}

var wrongViewCastSupertypes = map[string][]string{
	"AppCompatButton":    {"Button", "TextView", "View"},
	"AppCompatEditText":  {"EditText", "TextView", "View"},
	"AppCompatImageView": {"ImageView", "View"},
	"AppCompatTextView":  {"TextView", "View"},
	"Button":             {"TextView", "View"},
	"CheckBox":           {"CompoundButton", "Button", "TextView", "View"},
	"CompoundButton":     {"Button", "TextView", "View"},
	"EditText":           {"TextView", "View"},
	"FrameLayout":        {"ViewGroup", "View"},
	"ImageButton":        {"ImageView", "View"},
	"ImageView":          {"View"},
	"LinearLayout":       {"ViewGroup", "View"},
	"MaterialButton":     {"AppCompatButton", "Button", "TextView", "View"},
	"MaterialTextView":   {"AppCompatTextView", "TextView", "View"},
	"RadioButton":        {"CompoundButton", "Button", "TextView", "View"},
	"RecyclerView":       {"ViewGroup", "View"},
	"RelativeLayout":     {"ViewGroup", "View"},
	"ShapeableImageView": {"AppCompatImageView", "ImageView", "View"},
	"TextInputEditText":  {"AppCompatEditText", "EditText", "TextView", "View"},
	"TextView":           {"View"},
	"ToggleButton":       {"CompoundButton", "Button", "TextView", "View"},
	"ViewGroup":          {"View"},
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *WrongViewCastRule) Confidence() float64 { return 0.75 }

func (r *WrongViewCastRule) NodeTypes() []string {
	return []string{"call_expression", "as_expression", "cast_expression", "local_variable_declaration"}
}

func (r *WrongViewCastRule) check(ctx *v2.Context) {
	if ctx == nil || ctx.File == nil || ctx.Idx == 0 {
		return
	}
	switch ctx.File.Language {
	case scanner.LangKotlin:
		r.checkKotlin(ctx)
	case scanner.LangJava:
		r.checkJava(ctx)
	}
}

func (r *WrongViewCastRule) checkKotlin(ctx *v2.Context) {
	file := ctx.File
	switch file.FlatType(ctx.Idx) {
	case "call_expression":
		castType := wrongViewCastKotlinGenericType(file, ctx.Idx)
		if castType == "" {
			return
		}
		r.checkLookup(ctx, ctx.Idx, castType)
	case "as_expression":
		parts, ok := unsafeCastExpressionPartsFlat(file, ctx.Idx)
		if !ok || parts.safe {
			return
		}
		call := flatUnwrapParenExpr(file, parts.source)
		if file.FlatType(call) != "call_expression" {
			return
		}
		castType := wrongViewCastTypeName(file, parts.target)
		if castType == "" {
			return
		}
		r.checkLookup(ctx, call, castType)
	}
}

func (r *WrongViewCastRule) checkJava(ctx *v2.Context) {
	file := ctx.File
	switch file.FlatType(ctx.Idx) {
	case "cast_expression":
		castType, call := wrongViewCastJavaCast(file, ctx.Idx)
		if castType == "" || call == 0 {
			return
		}
		r.checkLookup(ctx, call, castType)
	case "local_variable_declaration":
		castType := wrongViewCastJavaDeclarationType(file, ctx.Idx)
		if castType == "" {
			return
		}
		file.FlatWalkNodes(ctx.Idx, "method_invocation", func(call uint32) {
			if wrongViewCastHasAncestorBefore(file, call, ctx.Idx, "cast_expression") {
				return
			}
			r.checkLookup(ctx, call, castType)
		})
	}
}

func (r *WrongViewCastRule) checkLookup(ctx *v2.Context, call uint32, castType string) {
	file := ctx.File
	callConfidence, ok := wrongViewCastAndroidLookupConfidence(ctx, call)
	if !ok {
		return
	}
	idName := wrongViewCastLookupID(file, call)
	if idName == "" {
		return
	}
	if actualType, ok := wrongViewCastResourceViewType(ctx.ResourceIndex, idName); ok {
		if wrongViewCastTypeCompatible(actualType, castType) {
			return
		}
		ctx.Emit(scanner.Finding{
			File:       file.Path,
			Line:       int(file.FlatRow(call)) + 1,
			Col:        int(file.FlatCol(call)) + 1,
			Message:    "Suspicious cast: id '" + idName + "' is " + actualType + " in layout resources, but cast to " + castType + ".",
			Confidence: callConfidence,
		})
	}
}

func wrongViewCastKotlinGenericType(file *scanner.File, call uint32) string {
	if file == nil || file.FlatType(call) != "call_expression" {
		return ""
	}
	for child := file.FlatFirstChild(call); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "call_suffix" {
			continue
		}
		typeArgs, ok := file.FlatFindChild(child, "type_arguments")
		if !ok {
			return ""
		}
		return wrongViewCastTypeName(file, typeArgs)
	}
	return ""
}

func wrongViewCastTypeName(file *scanner.File, idx uint32) string {
	var last string
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		switch file.FlatType(candidate) {
		case "type_identifier", "identifier", "scoped_type_identifier":
			last = file.FlatNodeText(candidate)
		}
	})
	return wrongViewCastSimpleType(last)
}

func wrongViewCastJavaCast(file *scanner.File, idx uint32) (string, uint32) {
	if file == nil || file.FlatType(idx) != "cast_expression" {
		return "", 0
	}
	var castType string
	var call uint32
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if castType == "" {
			castType = wrongViewCastTypeName(file, child)
			if castType != "" {
				continue
			}
		}
		if file.FlatType(child) == "method_invocation" {
			call = child
		}
	}
	return castType, call
}

func wrongViewCastJavaDeclarationType(file *scanner.File, decl uint32) string {
	if file == nil || file.FlatType(decl) != "local_variable_declaration" {
		return ""
	}
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "variable_declarator" {
			return ""
		}
		if !file.FlatIsNamed(child) {
			continue
		}
		if typ := wrongViewCastTypeName(file, child); typ != "" {
			return typ
		}
	}
	return ""
}

func wrongViewCastHasAncestorBefore(file *scanner.File, idx, stop uint32, nodeType string) bool {
	for parent, ok := file.FlatParent(idx); ok && parent != stop; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) == nodeType {
			return true
		}
	}
	return false
}

func wrongViewCastAndroidLookupConfidence(ctx *v2.Context, call uint32) (float64, bool) {
	file := ctx.File
	callee := wrongViewCastCallName(file, call)
	if callee != "findViewById" && callee != "requireViewById" {
		return 0, false
	}
	if file.Language == scanner.LangKotlin {
		if target, ok := semantics.ResolveCallTarget(ctx, call); ok && target.Resolved {
			if wrongViewCastAllowedCallTarget(target.QualifiedName, callee) {
				return 0.95, true
			}
			return 0, false
		}
	}

	receiver := wrongViewCastCallReceiverName(file, call)
	if callee == "requireViewById" && wrongViewCastIsViewCompatReceiver(receiver) {
		return 0.90, true
	}
	if callee == "findViewById" && wrongViewCastIsQualifiedViewLookupReceiver(receiver) {
		return 0.90, true
	}
	return 0, false
}

func wrongViewCastAllowedCallTarget(target, callee string) bool {
	if target == "" {
		return false
	}
	if !strings.HasSuffix(target, "."+callee) && !strings.HasSuffix(target, "#"+callee) && target != callee {
		return false
	}
	return strings.HasPrefix(target, "android.") || strings.HasPrefix(target, "androidx.")
}

func wrongViewCastCallName(file *scanner.File, call uint32) string {
	switch file.FlatType(call) {
	case "call_expression":
		return flatCallExpressionName(file, call)
	case "method_invocation":
		var last string
		for child := file.FlatFirstChild(call); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "argument_list" {
				break
			}
			if file.FlatType(child) == "identifier" {
				last = file.FlatNodeText(child)
			}
		}
		return last
	default:
		return ""
	}
}

func wrongViewCastCallReceiverName(file *scanner.File, call uint32) string {
	switch file.FlatType(call) {
	case "call_expression":
		return flatReceiverNameFromCall(file, call)
	case "method_invocation":
		var identifiers []string
		for child := file.FlatFirstChild(call); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "argument_list" {
				break
			}
			if file.FlatType(child) == "identifier" {
				identifiers = append(identifiers, file.FlatNodeText(child))
			}
		}
		if len(identifiers) > 1 {
			return strings.Join(identifiers[:len(identifiers)-1], ".")
		}
	}
	return ""
}

func wrongViewCastIsViewCompatReceiver(receiver string) bool {
	return receiver == "ViewCompat" || receiver == "androidx.core.view.ViewCompat" || strings.HasSuffix(receiver, ".ViewCompat")
}

func wrongViewCastIsQualifiedViewLookupReceiver(receiver string) bool {
	switch receiver {
	case "android.app.Activity", "android.view.View":
		return true
	default:
		return strings.HasPrefix(receiver, "android.") && (strings.HasSuffix(receiver, ".Activity") || strings.HasSuffix(receiver, ".View"))
	}
}

func wrongViewCastLookupID(file *scanner.File, call uint32) string {
	for _, arg := range wrongViewCastCallArgumentExpressions(file, call) {
		if id := wrongViewCastResourceIDName(file, arg); id != "" {
			return id
		}
	}
	return ""
}

func wrongViewCastCallArgumentExpressions(file *scanner.File, call uint32) []uint32 {
	var argsNode uint32
	switch file.FlatType(call) {
	case "call_expression":
		_, argsNode = flatCallExpressionParts(file, call)
	case "method_invocation":
		if args, ok := file.FlatFindChild(call, "argument_list"); ok {
			argsNode = args
		}
	}
	if argsNode == 0 {
		return nil
	}
	var args []uint32
	for child := file.FlatFirstChild(argsNode); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if file.FlatType(child) == "value_argument" {
			if expr := flatValueArgumentExpression(file, child); expr != 0 {
				args = append(args, expr)
			}
			continue
		}
		args = append(args, child)
	}
	return args
}

func wrongViewCastResourceIDName(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	var identifiers []string
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		switch file.FlatType(candidate) {
		case "simple_identifier", "identifier":
			identifiers = append(identifiers, file.FlatNodeText(candidate))
		}
	})
	if len(identifiers) < 3 {
		return ""
	}
	for i := 0; i+2 < len(identifiers); i++ {
		if identifiers[i] == "R" && identifiers[i+1] == "id" {
			return identifiers[i+2]
		}
	}
	return ""
}

func wrongViewCastResourceViewType(idx *android.ResourceIndex, idName string) (string, bool) {
	if idx == nil || idName == "" {
		return "", false
	}
	var actual string
	var saw bool
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v == nil || wrongViewCastNormalizeID(v.ID) != idName {
				return
			}
			viewType := wrongViewCastSimpleType(v.Type)
			if viewType == "" {
				return
			}
			if !saw {
				actual = viewType
				saw = true
				return
			}
			if !wrongViewCastTypeCompatible(viewType, actual) || !wrongViewCastTypeCompatible(actual, viewType) {
				actual = ""
			}
		})
		if saw && actual == "" {
			return "", false
		}
	}
	if !saw || actual == "" {
		return "", false
	}
	return actual, true
}

func wrongViewCastNormalizeID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "@+id/")
	id = strings.TrimPrefix(id, "@id/")
	return id
}

func wrongViewCastTypeCompatible(actualType, castType string) bool {
	actual := wrongViewCastSimpleType(actualType)
	cast := wrongViewCastSimpleType(castType)
	if actual == "" || cast == "" {
		return false
	}
	if actual == cast || cast == "View" {
		return true
	}
	for _, super := range wrongViewCastSupertypes[actual] {
		if super == cast {
			return true
		}
	}
	return false
}

func wrongViewCastSimpleType(typ string) string {
	typ = strings.TrimSpace(typ)
	typ = strings.TrimSuffix(typ, "?")
	if i := strings.IndexByte(typ, '<'); i >= 0 {
		typ = typ[:i]
	}
	if i := strings.LastIndexAny(typ, ".$"); i >= 0 {
		typ = typ[i+1:]
	}
	return strings.TrimSpace(typ)
}

type DeprecatedRule struct {
	FlatDispatchBase
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
	node  uint32
}

type rangeFileSummary struct {
	constants map[string][]rangeConstantSpec
	functions map[string][]rangeFunctionSpec
}

type rangeFunctionSpec struct {
	name   string
	decl   uint32
	owner  uint32
	params []rangeArgSpec
}

type rangeConstantSpec struct {
	name     string
	decl     uint32
	owner    uint32
	function uint32
	value    rangeNumber
}

var rangeSummaryCache sync.Map

func rangeSummaryForFile(file *scanner.File) *rangeFileSummary {
	if file == nil {
		return &rangeFileSummary{constants: map[string][]rangeConstantSpec{}, functions: map[string][]rangeFunctionSpec{}}
	}
	key := file.Path + "\x00" + strconv.Itoa(len(file.Content))
	if cached, ok := rangeSummaryCache.Load(key); ok {
		return cached.(*rangeFileSummary)
	}
	summary := &rangeFileSummary{
		constants: map[string][]rangeConstantSpec{},
		functions: map[string][]rangeFunctionSpec{},
	}
	file.FlatWalkNodes(0, "property_declaration", func(idx uint32) {
		spec, ok := rangeConstantProperty(file, idx, summary.constants)
		if ok && spec.name != "" {
			summary.constants[spec.name] = append(summary.constants[spec.name], spec)
		}
	})
	file.FlatWalkNodes(0, "function_declaration", func(idx uint32) {
		fn := rangeFunctionSpec{
			name:  extractIdentifierFlat(file, idx),
			decl:  idx,
			owner: androidEnclosingOwner(file, idx),
		}
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

func rangeConstantProperty(file *scanner.File, idx uint32, constants map[string][]rangeConstantSpec) (rangeConstantSpec, bool) {
	if file == nil || file.FlatType(idx) != "property_declaration" || rangePropertyIsVar(file, idx) {
		return rangeConstantSpec{}, false
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
	if !ok {
		return rangeConstantSpec{}, false
	}
	fn, _ := flatEnclosingFunction(file, idx)
	return rangeConstantSpec{
		name:     name,
		decl:     idx,
		owner:    androidEnclosingOwner(file, idx),
		function: fn,
		value:    value,
	}, true
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
	bestScore := 0
	var best *rangeFunctionSpec
	ambiguous := false
	for _, candidate := range candidates {
		if rangeMaxSpecIndex(candidate.params) >= argCount {
			continue
		}
		score := rangeLocalFunctionMatchScore(ctx.File, call, candidate)
		if score == 0 {
			continue
		}
		if score > bestScore {
			c := candidate
			best = &c
			bestScore = score
			ambiguous = false
			continue
		}
		if score == bestScore {
			ambiguous = true
		}
	}
	if best == nil || ambiguous {
		return nil
	}
	return best.params
}

func rangeLocalFunctionMatchScore(file *scanner.File, call uint32, candidate rangeFunctionSpec) int {
	if file == nil || candidate.decl == 0 || call == 0 {
		return 0
	}
	callOwner := androidEnclosingOwner(file, call)
	receiver := rangeCallReceiverNode(file, call)
	if receiver == 0 {
		if candidate.owner != 0 && candidate.owner == callOwner {
			return 3
		}
		if candidate.owner == 0 {
			return 2
		}
		return 0
	}
	receiverText := strings.TrimSpace(file.FlatNodeText(receiver))
	if receiverText == "this" && candidate.owner != 0 && candidate.owner == callOwner {
		return 4
	}
	if candidate.owner != 0 && receiverText == rangeOwnerName(file, candidate.owner) {
		return 4
	}
	return 0
}

func rangeCallReceiverNode(file *scanner.File, call uint32) uint32 {
	nav, _ := flatCallExpressionParts(file, call)
	if nav == 0 || file.FlatNamedChildCount(nav) < 2 {
		return 0
	}
	return file.FlatNamedChild(nav, 0)
}

func rangeOwnerName(file *scanner.File, owner uint32) string {
	if file == nil || owner == 0 {
		return ""
	}
	return extractIdentifierFlat(file, owner)
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
	return oracleLookupCallTargetFlat(lookup, ctx.File, call)
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

func rangeEvaluateNumericExpr(file *scanner.File, idx uint32, constants map[string][]rangeConstantSpec, seen map[string]bool) (rangeNumber, uint32, bool) {
	if file == nil || idx == 0 {
		return rangeNumber{}, 0, false
	}
	idx = flatUnwrapParenExpr(file, idx)
	switch file.FlatType(idx) {
	case "integer_literal":
		if value, ok := rangeParseIntegerLiteral(file.FlatNodeText(idx)); ok {
			return rangeNumber{value: float64(value), node: idx}, idx, true
		}
	case "real_literal":
		if value, ok := rangeParseFloatLiteral(file.FlatNodeText(idx)); ok {
			return rangeNumber{value: value, node: idx}, idx, true
		}
	case "simple_identifier":
		name := file.FlatNodeString(idx, nil)
		if constants == nil {
			return rangeNumber{}, 0, false
		}
		if seen == nil {
			seen = map[string]bool{}
		}
		if seen[name] {
			return rangeNumber{}, 0, false
		}
		seen[name] = true
		if value, ok := rangeResolveConstant(file, name, idx, constants); ok {
			return value, idx, true
		}
		return rangeNumber{}, 0, false
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

func rangeResolveConstant(file *scanner.File, name string, use uint32, constants map[string][]rangeConstantSpec) (rangeNumber, bool) {
	candidates := constants[name]
	if len(candidates) == 0 {
		return rangeNumber{}, false
	}
	useFn, _ := flatEnclosingFunction(file, use)
	useOwner := androidEnclosingOwner(file, use)
	useStart := file.FlatTree.Nodes[use].StartByte
	bestScore := 0
	var best rangeConstantSpec
	for _, candidate := range candidates {
		if candidate.decl == 0 || file.FlatTree.Nodes[candidate.decl].StartByte >= useStart {
			continue
		}
		score := 0
		switch {
		case candidate.function != 0:
			if candidate.function == useFn {
				score = 4
			}
		case candidate.owner != 0:
			if candidate.owner == useOwner {
				score = 3
			}
		default:
			score = 1
		}
		if score == 0 {
			continue
		}
		if score > bestScore || (score == bestScore && file.FlatTree.Nodes[candidate.decl].StartByte > file.FlatTree.Nodes[best.decl].StartByte) {
			best = candidate
			bestScore = score
		}
	}
	if bestScore == 0 {
		return rangeNumber{}, false
	}
	return best.value, true
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
	FlatDispatchBase
	AndroidRule
}

var resourceMethodExpected = map[string]string{"getString": "string", "getText": "string", "getQuantityString": "plurals", "getStringArray": "array", "getIntArray": "array", "getDrawable": "drawable", "setImageResource": "drawable", "setImageDrawable": "drawable", "setContentView": "layout", "inflate": "layout", "getColor": "color", "getColorStateList": "color", "getDimension": "dimen", "getDimensionPixelSize": "dimen", "getDimensionPixelOffset": "dimen", "getBoolean": "bool", "getInteger": "integer", "getAnimation": "anim", "getLayout": "layout"}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ResourceTypeRule) Confidence() float64 { return 0.75 }

type ResourceAsColorRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ResourceAsColorRule) Confidence() float64 { return 0.75 }

// ioCallNames is the set of known IO/network type and method names whose
// presence in a @MainThread function indicates a likely thread violation.
var ioCallNames = map[string]bool{
	"HttpURLConnection": true, "OkHttpClient": true,
	"FileInputStream": true, "FileOutputStream": true,
	"BufferedReader": true, "BufferedWriter": true,
	"URLConnection": true, "HttpClient": true,
	"Retrofit": true, "FileReader": true, "FileWriter": true,
	"RandomAccessFile": true, "openConnection": true,
	"Socket": true, "ServerSocket": true, "DatagramSocket": true,
}

type SupportAnnotationUsageRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *SupportAnnotationUsageRule) Confidence() float64 { return 0.75 }

type AccidentalOctalRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *AccidentalOctalRule) Confidence() float64 { return 0.75 }

type AppCompatMethodRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *AppCompatMethodRule) Confidence() float64 { return 0.75 }

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
func (r *CustomViewStyleableRule) Confidence() float64 { return 0.9 }

type InnerclassSeparatorRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *InnerclassSeparatorRule) Confidence() float64 { return 0.75 }

type ObjectAnimatorBindingRule struct {
	FlatDispatchBase
	AndroidRule
}

var knownAnimatorProperties = map[string]bool{
	"alpha": true, "translationX": true, "translationY": true, "translationZ": true,
	"rotation": true, "rotationX": true, "rotationY": true,
	"scaleX": true, "scaleY": true, "x": true, "y": true, "z": true,
	"elevation": true, "pivotX": true, "pivotY": true,
}

func (r *ObjectAnimatorBindingRule) Confidence() float64 { return 0.75 }

func (r *ObjectAnimatorBindingRule) check(ctx *v2.Context) {
	if ctx == nil || ctx.File == nil || ctx.Idx == 0 {
		return
	}
	file := ctx.File
	targetExpr, propertyArg, propertyName, ok := objectAnimatorBindingCall(ctx, ctx.Idx)
	if !ok {
		return
	}
	targetType := objectAnimatorResolveTargetType(ctx.Resolver, file, targetExpr)
	if targetType == nil || targetType.Kind == typeinfer.TypeUnknown || (targetType.Name == "" && targetType.FQN == "") {
		return
	}
	classInfo := objectAnimatorClassInfo(ctx.Resolver, targetType)
	if objectAnimatorClassHasProperty(classInfo, propertyName) {
		return
	}
	if objectAnimatorTypeIsView(ctx.Resolver, targetType) {
		if knownAnimatorProperties[propertyName] {
			return
		}
		ctx.Emit(r.Finding(file, file.FlatRow(propertyArg)+1, file.FlatCol(propertyArg)+1,
			"ObjectAnimator property \""+propertyName+"\" is not a standard View property. Verify the target has a setter for this property."))
		return
	}
	if classInfo != nil {
		ctx.Emit(r.Finding(file, file.FlatRow(propertyArg)+1, file.FlatCol(propertyArg)+1,
			"ObjectAnimator property \""+propertyName+"\" does not match a setter or property on target type "+classInfo.Name+"."))
	}
}

func objectAnimatorBindingCall(ctx *v2.Context, call uint32) (targetExpr, propertyArg uint32, propertyName string, ok bool) {
	file := ctx.File
	if file == nil || file.FlatType(call) != "call_expression" {
		return 0, 0, "", false
	}
	if !objectAnimatorCallIsObjectAnimator(ctx, call) {
		return 0, 0, "", false
	}
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return 0, 0, "", false
	}
	targetArg := flatNamedValueArgument(file, args, "target")
	if targetArg == 0 {
		targetArg = flatPositionalValueArgument(file, args, 0)
	}
	propertyArg = flatNamedValueArgument(file, args, "propertyName")
	if propertyArg == 0 {
		propertyArg = flatNamedValueArgument(file, args, "property")
	}
	if propertyArg == 0 {
		propertyArg = flatPositionalValueArgument(file, args, 1)
	}
	if targetArg == 0 || propertyArg == 0 {
		return 0, 0, "", false
	}
	targetExpr = objectAnimatorValueArgumentExpression(file, targetArg)
	propertyExpr := objectAnimatorValueArgumentExpression(file, propertyArg)
	propertyName, ok = objectAnimatorStringLiteralValue(file, propertyExpr)
	if targetExpr == 0 || !ok || propertyName == "" {
		return 0, 0, "", false
	}
	return targetExpr, propertyArg, propertyName, true
}

func objectAnimatorCallIsObjectAnimator(ctx *v2.Context, call uint32) bool {
	file := ctx.File
	name := flatCallExpressionName(file, call)
	if name != "ofFloat" && name != "ofInt" && name != "ofObject" {
		return false
	}
	if target := objectAnimatorOracleCallTarget(ctx, call); target != "" {
		return objectAnimatorCallTargetMatches(target, name)
	}
	navExpr, _ := flatCallExpressionParts(file, call)
	ids := flatNavigationIdentifierParts(file, navExpr)
	if len(ids) == 1 {
		if ctx.Resolver == nil {
			return false
		}
		return ctx.Resolver.ResolveImport(name, file) == "android.animation.ObjectAnimator."+name
	}
	if len(ids) < 2 {
		return false
	}
	receiver := ids[len(ids)-2]
	receiverFQN := strings.Join(ids[:len(ids)-1], ".")
	switch receiverFQN {
	case "android.animation.ObjectAnimator":
		return true
	}
	if ctx.Resolver != nil {
		if imported := ctx.Resolver.ResolveImport(receiver, file); imported != "" {
			return imported == "android.animation.ObjectAnimator"
		}
		if resolved := ctx.Resolver.ResolveByNameFlat(receiver, call, file); resolved != nil &&
			resolved.Kind != typeinfer.TypeUnknown && (resolved.Name != "" || resolved.FQN != "") {
			return resolved.FQN == "android.animation.ObjectAnimator"
		}
	}
	return false
}

func objectAnimatorOracleCallTarget(ctx *v2.Context, call uint32) string {
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
	return oracleLookupCallTargetFlat(lookup, ctx.File, call)
}

func objectAnimatorCallTargetMatches(target, methodName string) bool {
	want := "android.animation.ObjectAnimator." + methodName
	return target == want || strings.Contains(target, want+"(") ||
		target == "android.animation.ObjectAnimator#"+methodName ||
		strings.Contains(target, "android.animation.ObjectAnimator#"+methodName+"(")
}

func flatNavigationIdentifierParts(file *scanner.File, idx uint32) []string {
	if file == nil || idx == 0 {
		return nil
	}
	var ids []string
	file.FlatWalkAllNodes(idx, func(node uint32) {
		switch file.FlatType(node) {
		case "simple_identifier", "type_identifier":
			ids = append(ids, file.FlatNodeText(node))
		}
	})
	return ids
}

func objectAnimatorValueArgumentExpression(file *scanner.File, arg uint32) uint32 {
	if file == nil || arg == 0 {
		return 0
	}
	for child := file.FlatFirstChild(arg); child != 0; child = file.FlatNextSib(child) {
		typ := file.FlatType(child)
		if typ == "value_argument_label" || typ == "=" || typ == "," || !file.FlatIsNamed(child) {
			continue
		}
		if typ == "simple_identifier" {
			if next, ok := file.FlatNextSibling(child); ok && file.FlatType(next) == "=" {
				continue
			}
		}
		return child
	}
	return 0
}

func objectAnimatorStringLiteralValue(file *scanner.File, idx uint32) (string, bool) {
	if file == nil || idx == 0 {
		return "", false
	}
	idx = flatUnwrapParenExpr(file, idx)
	switch file.FlatType(idx) {
	case "string_literal", "line_string_literal", "multi_line_string_literal":
	default:
		return "", false
	}
	if flatContainsStringInterpolation(file, idx) {
		return "", false
	}
	text := strings.TrimSpace(file.FlatNodeText(idx))
	if strings.HasPrefix(text, "\"\"\"") && strings.HasSuffix(text, "\"\"\"") && len(text) >= 6 {
		return strings.TrimSuffix(strings.TrimPrefix(text, "\"\"\""), "\"\"\""), true
	}
	value, err := strconv.Unquote(text)
	if err != nil {
		return "", false
	}
	return value, true
}

func objectAnimatorResolveTargetType(resolver typeinfer.TypeResolver, file *scanner.File, targetExpr uint32) *typeinfer.ResolvedType {
	if resolver == nil || file == nil || targetExpr == 0 {
		return nil
	}
	targetExpr = flatUnwrapParenExpr(file, targetExpr)
	resolved := resolver.ResolveFlatNode(targetExpr, file)
	if resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
		return resolved
	}
	if file.FlatType(targetExpr) == "simple_identifier" {
		return resolver.ResolveByNameFlat(file.FlatNodeText(targetExpr), targetExpr, file)
	}
	return resolved
}

func objectAnimatorClassInfo(resolver typeinfer.TypeResolver, typ *typeinfer.ResolvedType) *typeinfer.ClassInfo {
	if resolver == nil || typ == nil {
		return nil
	}
	for _, name := range []string{typ.FQN, typ.Name} {
		if name == "" {
			continue
		}
		if info := resolver.ClassHierarchy(name); info != nil {
			return info
		}
	}
	return nil
}

func objectAnimatorClassHasProperty(info *typeinfer.ClassInfo, propertyName string) bool {
	if info == nil || propertyName == "" {
		return false
	}
	setterName := "set" + strings.ToUpper(propertyName[:1]) + propertyName[1:]
	for _, member := range info.Members {
		switch member.Kind {
		case "property":
			if member.Name == propertyName {
				return true
			}
		case "function":
			if member.Name == setterName {
				return true
			}
		}
	}
	return false
}

func objectAnimatorTypeIsView(resolver typeinfer.TypeResolver, typ *typeinfer.ResolvedType) bool {
	if typ == nil {
		return false
	}
	seen := make(map[string]bool)
	var visit func(string) bool
	visit = func(name string) bool {
		if name == "" || seen[name] {
			return false
		}
		seen[name] = true
		if name == "View" || name == "android.view.View" {
			return true
		}
		if resolver == nil {
			return false
		}
		info := resolver.ClassHierarchy(name)
		if info == nil {
			return false
		}
		if info.Name == "View" || info.FQN == "android.view.View" {
			return true
		}
		for _, supertype := range info.Supertypes {
			if visit(supertype) {
				return true
			}
		}
		return false
	}
	return visit(typ.FQN) || visit(typ.Name)
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
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *PropertyEscapeRule) Confidence() float64 { return 0.75 }

type ShortAlarmRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ShortAlarmRule) Confidence() float64 { return 0.75 }

type LocalSuppressRule struct {
	FlatDispatchBase
	AndroidRule
}

var knownLintIssueIDs = map[string]bool{"NewApi": true, "InlinedApi": true, "Override": true, "UnusedResources": true, "HardcodedText": true, "MissingTranslation": true, "ExtraTranslation": true, "MissingPermission": true, "ContentDescription": true, "ObsoleteLayoutParam": true, "ViewHolder": true, "LogConditional": true, "SdCardPath": true, "Wakelock": true, "SetJavaScriptEnabled": true, "ExportedService": true, "PackagedPrivateKey": true, "ValidFragment": true, "ViewConstructor": true, "WrongImport": true, "ServiceCast": true, "LayoutInflation": true, "ShowToast": true, "PackageManagerGetSignatures": true, "UseSparseArrays": true, "UseValueOf": true, "LongLogTag": true, "LogTagMismatch": true, "UnlocalizedSms": true, "ViewTag": true, "ShortAlarm": true, "UniqueConstants": true, "ShiftFlags": true, "AccidentalOctal": true, "AppCompatMethod": true, "CheckResult": true, "CommitPrefEdits": true, "CommitTransaction": true, "CustomViewStyleable": true, "CutPasteId": true, "DalvikOverride": true, "DefaultLocale": true, "Deprecated": true, "DeviceAdmin": true, "DuplicateActivity": true, "DuplicateIds": true, "DuplicateIncludedIds": true, "DuplicateUsesFeature": true, "ExtraText": true, "FullBackupContent": true, "GradleCompatible": true, "GradleDependency": true, "GradleDeprecated": true, "GradleDynamicVersion": true, "GradleGetter": true, "GradleIdeError": true, "GradleOverrides": true, "GradlePath": true, "IllegalResourceRef": true, "InconsistentArrays": true, "InconsistentLayout": true, "InnerclassSeparator": true, "InvalidResourceFolder": true, "ManifestOrder": true, "ManifestTypo": true, "MissingApplicationIcon": true, "MissingId": true, "MissingRegistered": true, "MissingVersion": true, "MockLocation": true, "MultipleUsesSdk": true, "NestedScrolling": true, "NotSibling": true, "ObjectAnimatorBinding": true, "OnClick": true, "OverrideAbstract": true, "ParcelCreator": true, "PluralsCandidate": true, "PropertyEscape": true, "ProtectedPermissions": true, "Range": true, "Registered": true, "RequiredSize": true, "ResAuto": true, "ResourceAsColor": true, "ResourceType": true, "ScrollViewCount": true, "ScrollViewSize": true, "ServiceExported": true, "SetTextI18n": true, "SimpleDateFormat": true, "SpUsage": true, "StopShip": true, "StringFormatCount": true, "StringFormatInvalid": true, "StringFormatMatches": true, "SupportAnnotationUsage": true, "SwitchIntDef": true, "TextFields": true, "TextViewEdits": true, "UnpackedNativeCode": true, "UnusedAttribute": true, "UnusedNamespace": true, "UseCompoundDrawables": true, "UsesMinSdkAttributes": true, "WrongCall": true, "WrongCase": true, "WrongFolder": true, "WrongManifestParent": true, "WrongRegion": true, "WrongThread": true, "WrongViewCast": true, "Assert": true, "SQLiteString": true, "LocalSuppress": true, "AddJavascriptInterface": true, "GetInstance": true, "EasterEgg": true, "ExportedContentProvider": true, "ExportedReceiver": true, "ExportedPreferenceActivity": true, "GrantAllUris": true, "HardcodedDebugMode": true, "InsecureBaseConfiguration": true, "SecureRandom": true, "TrustedServer": true, "UnprotectedSMSBroadcastReceiver": true, "UnsafeProtectedBroadcastReceiver": true, "UseCheckPermission": true, "WorldReadableFiles": true, "WorldWriteableFiles": true, "DisableBaselineAlignment": true, "DrawAllocation": true, "FieldGetter": true, "FloatMath": true, "HandlerLeak": true, "InefficientWeight": true, "MergeRootFrame": true, "NestedWeights": true, "Overdraw": true, "Recycle": true, "TooDeepLayout": true, "UselessLeaf": true, "ClickableViewAccessibility": true, "LabelFor": true, "ByteOrderMark": true, "RelativeOverlap": true, "IconColors": true, "IconDensities": true, "IconDipSize": true, "IconDuplicates": true, "IconDuplicatesConfig": true, "IconExpectedSize": true, "IconExtension": true, "IconLauncherShape": true, "IconLocation": true, "IconMissingDensityFolder": true, "IconMixedNinePatch": true, "IconNoDpi": true, "IconXmlAndPng": true, "ConvertToWebp": true, "GifUsage": true, "AppCompatResource": true, "AppIndexingError": true, "AppIndexingWarning": true, "BackButton": true, "ButtonCase": true, "ButtonOrder": true, "ButtonStyle": true, "GoogleAppIndexingDeepLinkError": true, "GoogleAppIndexingWarning": true, "NegativeMargin": true, "RtlCompat": true, "RtlEnabled": true, "RtlHardcoded": true, "RtlSymmetry": true, "RtlSuperscript": true, "AllowBackup": true, "AlwaysShowAction": true, "InvalidUsesTagAttribute": true, "OldTargetApi": true, "PermissionImpliesUnsupportedHardware": true, "UnsupportedChromeOsHardware": true, "MissingLeanbackLauncher": true, "MissingLeanbackSupport": true, "Typos": true, "TypographyDashes": true, "TypographyEllipsis": true, "TypographyFractions": true, "TypographyOther": true, "TypographyQuotes": true, "UnusedIds": true, "Autofill": true, "RestrictedApi": true, "VisibleForTests": true, "MissingInflatedId": true, "NotificationPermission": true, "ObsoleteSdkInt": true, "all": true}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *LocalSuppressRule) Confidence() float64 { return 0.75 }

// pluralsCountNames is the set of identifiers whose presence as a when-subject
// suggests manual pluralization.
var pluralsCountNames = map[string]bool{
	"count": true, "num": true, "size": true,
	"quantity": true, "amount": true, "number": true,
}

// pluralsStringCalls is the set of call names that suggest string-resource usage.
var pluralsStringCalls = map[string]bool{
	"getString": true, "getQuantityString": true, "format": true,
	"getText": true, "getResources": true,
}

type PluralsCandidateRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *PluralsCandidateRule) Confidence() float64 { return 0.75 }
