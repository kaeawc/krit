package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

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
func (r *WrongViewCastRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *WrongViewCastRule) NodeTypes() []string {
	return []string{"call_expression", "as_expression", "cast_expression", "local_variable_declaration"}
}

func (r *WrongViewCastRule) check(ctx *api.Context) {
	switch ctx.File.Language {
	case scanner.LangKotlin:
		r.checkKotlin(ctx)
	case scanner.LangJava:
		r.checkJava(ctx)
	}
}

func (r *WrongViewCastRule) checkKotlin(ctx *api.Context) {
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

func (r *WrongViewCastRule) checkJava(ctx *api.Context) {
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

func (r *WrongViewCastRule) checkLookup(ctx *api.Context, call uint32, castType string) {
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
			Line:       file.FlatRow(call) + 1,
			Col:        file.FlatCol(call) + 1,
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

func wrongViewCastAndroidLookupConfidence(ctx *api.Context, call uint32) (float64, bool) {
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
