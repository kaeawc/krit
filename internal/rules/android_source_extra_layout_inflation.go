package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

type LayoutInflationRule struct {
	FlatDispatchBase
	LayoutResourceBase
	AndroidRule
}

// layoutInflationDialogContexts are class suffixes / API markers where
// passing null to inflate() is the correct idiomatic pattern because no
// parent ViewGroup is available at inflation time.
var layoutInflationDialogContexts = []string{
	": Dialog", ": DialogFragment", ": BottomSheetDialogFragment",
	": AppCompatDialog", ": AppCompatDialogFragment", ": AlertDialog",
	": PopupWindow", ": ListPopupWindow",
	"MaterialAlertDialogBuilder", "AlertDialog.Builder",
	"onCreateDialog", "setView(", ".setContentView(",
	"PopupWindow(",
	// Compose interop via AndroidView -- Compose manages attachment.
	"AndroidView(", "AndroidView {",
	// Offscreen rendering to a Bitmap/Canvas -- no parent exists by design.
	"Bitmap.createBitmap", "createBitmap(",
	"Canvas(", "drawToBitmap",
}

func (r *LayoutInflationRule) NodeTypes() []string { return []string{"call_expression"} }

// Confidence reports a high-confidence AST/resource-backed check. The
// remaining uncertainty is limited to intentional no-parent contexts that do
// not have a precise type signal in tree-sitter-only analysis.
func (r *LayoutInflationRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *LayoutInflationRule) check(ctx *api.Context) {
	file := ctx.File
	if flatCallExpressionName(file, ctx.Idx) != "inflate" {
		return
	}
	layoutName, ok := layoutInflationLayoutName(file, ctx.Idx)
	if !ok || !layoutInflationSecondArgumentIsNull(file, ctx.Idx) {
		return
	}
	if layoutInflationHasIntentionalNullParentContext(file, ctx.Idx) {
		return
	}
	if !layoutInflationRootHasLayoutParams(ctx.ResourceIndex, layoutName) &&
		!layoutInflationCallerHasNonNullViewGroup(file, ctx.Idx) {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"Avoid passing null as the parent ViewGroup. Inflate with the parent to get correct LayoutParams."))
}

// layoutInflationCallerHasNonNullViewGroup reports whether the enclosing
// function (or any outer function for nested declarations) declares a
// non-nullable ViewGroup parameter, indicating a parent is in scope and
// should have been passed to inflate().
func layoutInflationCallerHasNonNullViewGroup(file *scanner.File, call uint32) bool {
	if file == nil || call == 0 {
		return false
	}
	for fn, ok := flatEnclosingFunction(file, call); ok; fn, ok = flatEnclosingFunction(file, fn) {
		params, _ := file.FlatFindChild(fn, "function_value_parameters")
		if params == 0 {
			continue
		}
		for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) != "parameter" {
				continue
			}
			if layoutInflationParameterIsNonNullViewGroup(file, child) {
				return true
			}
		}
	}
	return false
}

func layoutInflationParameterIsNonNullViewGroup(file *scanner.File, param uint32) bool {
	for child := file.FlatFirstChild(param); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type":
			ident := flatLastChildOfType(file, child, "type_identifier")
			if ident == 0 {
				return false
			}
			return layoutInflationTypeNameIsViewGroup(file.FlatNodeText(ident))
		case "nullable_type":
			return false
		}
	}
	return false
}

func layoutInflationTypeNameIsViewGroup(name string) bool {
	name = strings.TrimSpace(name)
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	if idx := strings.Index(name, "<"); idx >= 0 {
		name = name[:idx]
	}
	return name == "ViewGroup"
}

func layoutInflationLayoutName(file *scanner.File, call uint32) (string, bool) {
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return "", false
	}
	arg := flatNamedValueArgument(file, args, "resource")
	if arg == 0 {
		arg = flatPositionalValueArgument(file, args, 0)
	}
	expr := flatValueArgumentExpression(file, arg)
	if expr == 0 {
		return "", false
	}
	ids := layoutInflationIdentifierChain(file, flatUnwrapParenExpr(file, expr))
	if len(ids) != 3 || ids[0] != "R" || ids[1] != "layout" || ids[2] == "" {
		return "", false
	}
	return ids[2], true
}

func layoutInflationSecondArgumentIsNull(file *scanner.File, call uint32) bool {
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return false
	}
	arg := flatNamedValueArgument(file, args, "root")
	if arg == 0 {
		arg = flatNamedValueArgument(file, args, "parent")
	}
	if arg == 0 {
		arg = flatPositionalValueArgument(file, args, 1)
	}
	expr := flatValueArgumentExpression(file, arg)
	if expr == 0 {
		return strings.TrimSpace(file.FlatNodeText(arg)) == "null"
	}
	expr = flatUnwrapParenExpr(file, expr)
	return expr != 0 && strings.TrimSpace(file.FlatNodeText(expr)) == "null"
}

func layoutInflationIdentifierChain(file *scanner.File, idx uint32) []string {
	if file == nil || idx == 0 {
		return nil
	}
	switch file.FlatType(idx) {
	case "simple_identifier":
		return []string{file.FlatNodeText(idx)}
	case "navigation_expression":
		var ids []string
		var walk func(uint32)
		walk = func(node uint32) {
			for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
				if !file.FlatIsNamed(child) {
					continue
				}
				switch file.FlatType(child) {
				case "simple_identifier":
					ids = append(ids, file.FlatNodeText(child))
				case "navigation_expression", "navigation_suffix":
					walk(child)
				}
			}
		}
		walk(idx)
		return ids
	default:
		return nil
	}
}

func layoutInflationRootHasLayoutParams(idx *android.ResourceIndex, layoutName string) bool {
	if idx == nil || layoutName == "" {
		return false
	}
	if configs := idx.LayoutConfigs[layoutName]; len(configs) > 0 {
		for _, layout := range configs {
			if layoutInflationViewHasLayoutParams(layoutRootView(layout)) {
				return true
			}
		}
		return false
	}
	return layoutInflationViewHasLayoutParams(layoutRootView(idx.Layouts[layoutName]))
}

func layoutRootView(layout *android.Layout) *android.View {
	if layout == nil {
		return nil
	}
	return layout.RootView
}

func layoutInflationViewHasLayoutParams(root *android.View) bool {
	if root == nil {
		return false
	}
	for attr := range root.Attributes {
		prefix, local := "", attr
		if idx := strings.LastIndex(attr, ":"); idx >= 0 {
			prefix = attr[:idx]
			local = attr[idx+1:]
		}
		if prefix == "tools" {
			continue
		}
		if strings.HasPrefix(local, "layout_") {
			return true
		}
	}
	return false
}

func layoutInflationHasIntentionalNullParentContext(file *scanner.File, idx uint32) bool {
	for cur, ok := idx, idx != 0; ok; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) != "call_expression" {
			continue
		}
		switch flatCallNameAny(file, cur) {
		case "AndroidView", "PopupWindow", "setView", "setContentView", "createBitmap", "drawToBitmap":
			return true
		}
	}
	for _, scopeType := range []string{"function_declaration", "class_declaration", "object_declaration"} {
		if scope := enclosingAncestor(file, idx, scopeType); scope != 0 && containsAny(declarationHeaderText(file, scope), layoutInflationDialogContexts) {
			return true
		}
	}
	line := file.FlatRow(idx) + 1
	if prefix, ok := prefixTextBeforeLine(file, line, "function_declaration", "secondary_constructor", "anonymous_initializer", "lambda_literal"); ok && containsAny(prefix, layoutInflationDialogContexts) {
		return true
	}
	if fn := enclosingAncestor(file, idx, "function_declaration", "secondary_constructor", "anonymous_initializer", "lambda_literal"); fn != 0 && containsAny(file.FlatNodeText(fn), layoutInflationDialogContexts) {
		return true
	}
	return false
}
