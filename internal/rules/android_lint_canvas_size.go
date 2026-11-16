package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	androidCanvasClass   = "android.graphics.Canvas"
	androidDrawableClass = "android.graphics.drawable.Drawable"
)

var (
	canvasKnownNames   = map[string]struct{}{"Canvas": {}}
	drawableKnownNames = map[string]struct{}{
		"Drawable": {}, "BitmapDrawable": {}, "ColorDrawable": {}, "GradientDrawable": {}, "ShapeDrawable": {},
		"LayerDrawable": {}, "StateListDrawable": {}, "InsetDrawable": {}, "RippleDrawable": {}, "VectorDrawable": {},
		"DrawableWrapper": {}, "ClipDrawable": {}, "RotateDrawable": {}, "ScaleDrawable": {}, "PaintDrawable": {},
		"NinePatchDrawable": {}, "TransitionDrawable": {}, "LevelListDrawable": {}, "AnimationDrawable": {},
	}
)

// CanvasSizeRule mirrors Lint's CanvasSize: Canvas.getWidth or getHeight used inside a custom View or Drawable's
// onDraw or draw(Canvas), where the view's own size is usually what is meant.
type CanvasSizeRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the receiver is proved from declarations when no resolved target exists.
func (r *CanvasSizeRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *CanvasSizeRule) check(ctx *api.Context) {
	file := ctx.File
	call, member, isProperty, ok := canvasSizeAccess(file, ctx.Idx)
	if !ok || !canvasSizeReceiver(ctx, call, isProperty) || !lintInViewDrawMethod(ctx, ctx.Idx) {
		return
	}
	size := strings.ToLower(strings.TrimPrefix(member, "get"))
	if isProperty {
		ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", "Referencing `Canvas."+size+"` is usually wrong; you should be referencing `"+size+"` instead"))
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", "Calling `Canvas."+member+"()` is usually wrong; you should be calling `"+member+"()` instead"))
}

// canvasSizeAccess recognizes `x.getWidth()`, `x.getHeight()` and the Kotlin properties `x.width` and `x.height`. The
// returned member is the getter name; isProperty marks the Kotlin property form.
func canvasSizeAccess(file *scanner.File, idx uint32) (call lintCall, member string, isProperty, ok bool) {
	if file.FlatType(idx) != "navigation_expression" {
		call, ok = lintCallAt(file, idx)
		if !ok || call.ReceiverNode == 0 || len(call.Args) != 0 || (call.Name != "getWidth" && call.Name != "getHeight") {
			return call, "", false, false
		}
		return call, call.Name, false, true
	}
	if parent, has := file.FlatParent(idx); has && file.FlatType(parent) == "call_expression" {
		return call, "", false, false
	}
	recv := file.FlatNamedChild(idx, 0)
	suffix, found := file.FlatFindChild(idx, "navigation_suffix")
	if !found || recv == 0 {
		return call, "", false, false
	}
	name := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(file.FlatNodeText(suffix), "?"), "."))
	if name != "width" && name != "height" {
		return call, "", false, false
	}
	call = lintCall{File: file, Idx: idx, Name: name, ReceiverNode: recv, ReceiverText: strings.TrimSpace(file.FlatNodeText(recv))}
	return call, "get" + strings.ToUpper(name[:1]) + name[1:], true, true
}

// canvasSizeReceiver reports whether the access goes to an android.graphics.Canvas.
func canvasSizeReceiver(ctx *api.Context, call lintCall, isProperty bool) bool {
	if !isProperty {
		if verdict, resolved := lintResolvedOwnerExtends(ctx, call, androidCanvasClass); resolved {
			return verdict
		}
	}
	file := ctx.File
	recv := lintUnwrapReceiver(file, call.ReceiverNode)
	typ := lintReceiverClassText(file, lintWithReceiver(file, call, recv))
	// An untyped local copy of a canvas (`val c = canvas`) has the canvas's type.
	for depth := 0; typ == "" && depth < 3 && file.FlatType(recv) == "simple_identifier"; depth++ {
		init := lintUnwrapReceiver(file, lintInitializerOf(file, recv, file.FlatNodeText(recv), false))
		if init == 0 || file.FlatType(init) != "simple_identifier" {
			break
		}
		recv = init
		typ = lintReceiverClassText(file, lintWithReceiver(file, call, recv))
	}
	return typ != "" && lintTypeNameReaches(ctx, typ, androidCanvasClass, canvasKnownNames, 0)
}

// lintInViewDrawMethod reports whether idx sits, at any nesting depth, in an onDraw method or a draw(Canvas) method
// of a View or Drawable subclass.
func lintInViewDrawMethod(ctx *api.Context, idx uint32) bool {
	file := ctx.File
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "method_declaration":
		default:
			continue
		}
		if !lintIsDrawMethod(ctx, cur) {
			continue
		}
		cls, found := lintDirectClassOf(file, cur)
		if found && (lintClassExtends(ctx, cls, androidViewClass, lintKnownViewSimpleNames) ||
			lintClassExtends(ctx, cls, androidDrawableClass, drawableKnownNames)) {
			return true
		}
	}
	return false
}

// lintIsDrawMethod matches a method named onDraw, or draw with a single Canvas parameter.
func lintIsDrawMethod(ctx *api.Context, fn uint32) bool {
	file := ctx.File
	name := lintDeclaredMethodName(file, fn)
	if name == 0 {
		return false
	}
	switch file.FlatNodeText(name) {
	case "onDraw":
		return true
	case "draw":
		if lintDeclaredParamCount(file, fn) != 1 {
			return false
		}
		return lintTypeRefMatches(file, lintFirstParamTypeText(file, fn), androidCanvasClass)
	}
	return false
}

// lintFirstParamTypeText is the declared type of the first parameter of a Kotlin function or Java method.
func lintFirstParamTypeText(file *scanner.File, fn uint32) string {
	kind, param := "function_value_parameters", "parameter"
	if file.FlatType(fn) == "method_declaration" {
		kind, param = "formal_parameters", "formal_parameter"
	}
	params, ok := file.FlatFindChild(fn, kind)
	if !ok {
		return ""
	}
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == param {
			return lintChildTypeText(file, child)
		}
	}
	return ""
}
