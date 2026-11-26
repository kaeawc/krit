package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Port of InvalidSetHasFixedSizeDetector from androidx.recyclerview:recyclerview 1.4.0.

const (
	recyclerViewClass        = "androidx.recyclerview.widget.RecyclerView"
	recyclerFixedSizeMessage = "When using `setHasFixedSize() in an `RecyclerView`, `wrap_content` cannot be used as a value for `size` in the scrolling direction."
)

var recyclerViewCoordinates = []librarymodel.Coordinate{
	{Group: "androidx.recyclerview", Name: "recyclerview"},
}

// InvalidSetHasFixedSizeRule reports setHasFixedSize(true) on a RecyclerView that findViewById looks up by an id
// whose layout element is an androidx RecyclerView with a wrap_content width or height.
type InvalidSetHasFixedSizeRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence is high: the receiver is traced to the findViewById call and the id to the layout element.
func (r *InvalidSetHasFixedSizeRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *InvalidSetHasFixedSizeRule) check(ctx *api.Context) {
	file := ctx.File
	if ctx.ResourceIndex == nil || scanner.IsTestFile(projectPath(ctx, file.Path)) {
		return
	}
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "setHasFixedSize" || call.ReceiverNode == 0 || len(call.Args) != 1 {
		return
	}
	if v, ok := lintEvalBool(file, call.Args[0]); !ok || !v {
		return
	}
	if !librarymodel.EnsureFacts(ctx.LibraryFacts).Profile.MayUseAnyDependency(recyclerViewCoordinates...) {
		return
	}
	id, typed := recyclerLookupID(file, call.ReceiverNode, call.Idx, 0)
	if id == "" || !typed || !recyclerWrapContentIDs(ctx.ResourceIndex)[id] {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, call.Idx, "", recyclerFixedSizeMessage))
}

// recyclerWrapContentIDs lists the `@+id/` names of androidx RecyclerView layout elements with a wrap_content width
// or height, as the detector's visitElement collects them.
func recyclerWrapContentIDs(idx *android.ResourceIndex) map[string]bool {
	out := map[string]bool{}
	var walk func(v *android.View)
	walk = func(v *android.View) {
		if v.Type == recyclerViewClass && strings.HasPrefix(v.ID, "@+id/") &&
			(v.LayoutWidth == "wrap_content" || v.LayoutHeight == "wrap_content") {
			out[strings.TrimPrefix(v.ID, "@+id/")] = true
		}
		for _, c := range v.Children {
			walk(c)
		}
	}
	for _, layout := range idx.LayoutVariants {
		if layout != nil && layout.RootView != nil {
			walk(layout.RootView)
		}
	}
	return out
}

// recyclerLookupID traces a receiver to the findViewById(R.id.x) call it comes from and returns x. typed reports
// that the receiver is proven to be a RecyclerView: by a type argument, a cast, or a declared type.
func recyclerLookupID(file *scanner.File, expr, scope uint32, depth int) (id string, typed bool) {
	if expr == 0 || depth > 6 {
		return "", false
	}
	switch file.FlatType(expr) {
	case "parenthesized_expression":
		return recyclerLookupID(file, flatUnwrapParenExpr(file, expr), scope, depth+1)
	case "as_expression", "cast_expression":
		var inner, typ uint32
		for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			switch file.FlatType(child) {
			case "user_type", "type_identifier", "scoped_type_identifier", "nullable_type":
				typ = child
			default:
				inner = child
			}
		}
		id, _ = recyclerLookupID(file, inner, scope, depth+1)
		return id, typ != 0 && lintTypeRefMatches(file, lintTrimGenerics(strings.TrimSuffix(file.FlatNodeText(typ), "?")), recyclerViewClass)
	case "call_expression", "method_invocation":
		call, ok := lintCallAt(file, expr)
		if !ok || (call.Name != "findViewById" && call.Name != "requireViewById") || len(call.Args) != 1 {
			return "", false
		}
		name, ok := appResourceRef(file, call.Args[0], "id")
		if !ok {
			return "", false
		}
		typeArg := recyclerCallTypeArgument(file, expr)
		return name, typeArg != "" && lintTypeRefMatches(file, typeArg, recyclerViewClass)
	case "simple_identifier", "identifier":
		name := file.FlatNodeText(expr)
		declared := lintDeclaredTypeText(file, scope, name)
		typed = declared != "" && lintTypeRefMatches(file, declared, recyclerViewClass)
		if init := lintInitializerOf(file, scope, name, true); init != 0 {
			id, initTyped := recyclerLookupID(file, init, init, depth+1)
			return id, typed || initTyped
		}
		if value := recyclerAssignedValue(file, name); value != 0 {
			id, valueTyped := recyclerLookupID(file, value, value, depth+1)
			return id, typed || valueTyped
		}
		return "", false
	}
	return "", false
}

// recyclerAssignedValue returns the value of the file's only `name = value` assignment (a lateinit property set in
// onCreate, for example), or 0 when there is none or more than one.
func recyclerAssignedValue(file *scanner.File, name string) uint32 {
	var value uint32
	count := 0
	for _, kind := range []string{"assignment", "assignment_expression"} {
		file.FlatWalkNodes(0, kind, func(idx uint32) {
			var target, rhs uint32
			for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
				if !file.FlatIsNamed(child) {
					continue
				}
				if target == 0 {
					target = child
				} else {
					rhs = child
				}
			}
			text := strings.TrimPrefix(strings.TrimSpace(file.FlatNodeText(target)), "this.")
			if text == name && rhs != 0 {
				value = rhs
				count++
			}
		})
	}
	if count != 1 {
		return 0
	}
	return value
}

// recyclerCallTypeArgument returns the single explicit type argument of a Kotlin call such as findViewById<T>(...).
func recyclerCallTypeArgument(file *scanner.File, call uint32) string {
	suffix, ok := file.FlatFindChild(call, "call_suffix")
	if !ok {
		return ""
	}
	args, ok := file.FlatFindChild(suffix, "type_arguments")
	if !ok {
		return ""
	}
	text := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(file.FlatNodeText(args)), "<"), ">")
	if strings.Contains(text, ",") {
		return ""
	}
	return strings.TrimSpace(text)
}
