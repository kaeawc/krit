package rules

// Source check of Lint's ClickableViewAccessibility: custom views and touch
// listeners must call performClick when they detect a click.

import (
	"fmt"
	"regexp"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ClickableViewAccessibilitySourceRule reports onTouchEvent and OnTouchListener code that never calls performClick.
type ClickableViewAccessibilitySourceRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: view subclasses are recognized by name.
func (r *ClickableViewAccessibilitySourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ClickableViewAccessibilitySourceRule) check(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	switch file.FlatType(idx) {
	case "function_declaration":
		r.checkKotlinTouchEvent(ctx, idx)
		r.checkKotlinPerformClickSuper(ctx, idx)
	case "method_declaration":
		r.checkJavaTouchEvent(ctx, idx)
		r.checkJavaPerformClickSuper(ctx, idx)
	case "call_expression":
		r.checkKotlinListener(ctx, idx)
	case "method_invocation":
		r.checkJavaListener(ctx, idx)
	}
}

const clickableTouchMessage = " when a click is detected"

func (r *ClickableViewAccessibilitySourceRule) reportMissingPerformClick(ctx *api.Context, at uint32, className string, hasPerformClick bool) {
	file := ctx.File
	row, col := file.FlatRow(at)+1, file.FlatCol(at)+1
	if hasPerformClick {
		ctx.EmitAt(row, col, fmt.Sprintf("`%s#onTouchEvent` should call `%s#performClick`%s", className, className, clickableTouchMessage))
		return
	}
	ctx.EmitAt(row, col, fmt.Sprintf("Custom view `%s` overrides `onTouchEvent` but not `performClick`", className))
}

// ---- Kotlin ----

func (r *ClickableViewAccessibilitySourceRule) checkKotlinTouchEvent(ctx *api.Context, fn uint32) {
	file := ctx.File
	if flatFunctionName(file, fn) != "onTouchEvent" || !file.FlatHasModifier(fn, "override") {
		return
	}
	classIdx, ok := flatEnclosingAncestor(file, fn, "class_declaration", "object_declaration")
	if !ok || !androidEnclosingClassExtendsView(file, fn, ctx.Resolver) {
		return
	}
	var hasPerformClick bool
	if body, ok := file.FlatFindChild(classIdx, "class_body"); ok {
		for m := file.FlatFirstChild(body); m != 0; m = file.FlatNextSib(m) {
			if file.FlatType(m) == "function_declaration" && flatFunctionName(file, m) == "performClick" {
				hasPerformClick = true
			}
		}
	}
	if hasPerformClick && kotlinBodyCalls(file, fn, "performClick") {
		return
	}
	var name uint32
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "simple_identifier" {
			name = child
			break
		}
	}
	if name == 0 {
		name = fn
	}
	r.reportMissingPerformClick(ctx, name, extractIdentifierFlat(file, classIdx), hasPerformClick)
}

// callsSuperPerformClick matches `super.performClick(`, `super<T>.performClick(` and `super@Label.performClick(`.
var callsSuperPerformClick = regexp.MustCompile(`\bsuper\s*(?:<[^>]*>)?\s*(?:@\w+)?\s*\.\s*performClick\s*\(`)

// checkKotlinPerformClickSuper reports a View's own performClick() that never calls super.performClick().
func (r *ClickableViewAccessibilitySourceRule) checkKotlinPerformClickSuper(ctx *api.Context, fn uint32) {
	file := ctx.File
	if flatFunctionName(file, fn) != "performClick" || !file.FlatHasModifier(fn, "override") {
		return
	}
	classIdx, ok := flatEnclosingAncestor(file, fn, "class_declaration", "object_declaration")
	if !ok || !androidEnclosingClassExtendsView(file, fn, ctx.Resolver) {
		return
	}
	if callsSuperPerformClick.MatchString(file.FlatNodeText(fn)) {
		return
	}
	at := fn
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "simple_identifier" {
			at = child
			break
		}
	}
	className := extractIdentifierFlat(file, classIdx)
	ctx.EmitAt(file.FlatRow(at)+1, file.FlatCol(at)+1, fmt.Sprintf("`%s#performClick` should call `super#performClick`", className))
}

func (r *ClickableViewAccessibilitySourceRule) checkJavaPerformClickSuper(ctx *api.Context, m uint32) {
	file := ctx.File
	name, ok := file.FlatFindChild(m, "identifier")
	if !ok || file.FlatNodeText(name) != "performClick" {
		return
	}
	classIdx, ok := flatEnclosingAncestor(file, m, "class_declaration")
	if !ok || !javaViewSuperclass(file, classIdx, 0) || callsSuperPerformClick.MatchString(file.FlatNodeText(m)) {
		return
	}
	ctx.EmitAt(file.FlatRow(name)+1, file.FlatCol(name)+1, fmt.Sprintf("`%s#performClick` should call `super#performClick`", extractIdentifierFlat(file, classIdx)))
}

func kotlinBodyCalls(file *scanner.File, root uint32, method string) bool {
	found := false
	file.FlatWalkNodes(root, "call_expression", func(call uint32) {
		if flatCallExpressionName(file, call) == method {
			found = true
		}
	})
	return found
}

func (r *ClickableViewAccessibilitySourceRule) checkKotlinListener(ctx *api.Context, call uint32) {
	file := ctx.File
	if flatCallExpressionName(file, call) != "setOnTouchListener" {
		return
	}
	if lambda := flatCallTrailingLambda(file, call); lambda != 0 {
		if !kotlinBodyCalls(file, lambda, "performClick") {
			ctx.EmitAt(file.FlatRow(lambda)+1, file.FlatCol(lambda)+1, "`onTouch` lambda should call `View#performClick`"+clickableTouchMessage)
		}
		return
	}
	file.FlatWalkNodes(call, "object_literal", func(obj uint32) {
		file.FlatWalkNodes(obj, "function_declaration", func(fn uint32) {
			if flatFunctionName(file, fn) != "onTouch" || kotlinBodyCalls(file, fn, "performClick") {
				return
			}
			for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) == "simple_identifier" {
					ctx.EmitAt(file.FlatRow(child)+1, file.FlatCol(child)+1, "`onTouch` should call `View#performClick`"+clickableTouchMessage)
					return
				}
			}
		})
	})
}

// ---- Java ----

// javaViewSuperclass reports whether a Java class extends a View, by name or through a class declared in the file.
func javaViewSuperclass(file *scanner.File, classIdx uint32, depth int) bool {
	if depth > 4 {
		return false
	}
	sc, ok := file.FlatFindChild(classIdx, "superclass")
	if !ok {
		return false
	}
	var name string
	for child := file.FlatFirstChild(sc); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "scoped_type_identifier", "generic_type":
			name = file.FlatNodeText(child)
		}
	}
	simple := androidSimpleName(name)
	if name == "android.view.View" || simple == "View" {
		return true
	}
	if _, ok := androidKnownViewSimpleNames[simple]; ok {
		return true
	}
	// A class declared in this file that is itself a View.
	var parent uint32
	file.FlatWalkNodes(0, "class_declaration", func(candidate uint32) {
		if parent == 0 && candidate != classIdx && extractIdentifierFlat(file, candidate) == simple {
			parent = candidate
		}
	})
	return parent != 0 && javaViewSuperclass(file, parent, depth+1)
}

func javaBodyCalls(file *scanner.File, root uint32, method string) bool {
	found := false
	file.FlatWalkNodes(root, "method_invocation", func(call uint32) {
		if javaMethodInvocationName(file, call) == method {
			found = true
		}
	})
	return found
}

func (r *ClickableViewAccessibilitySourceRule) checkJavaTouchEvent(ctx *api.Context, m uint32) {
	file := ctx.File
	name, ok := file.FlatFindChild(m, "identifier")
	if !ok || file.FlatNodeText(name) != "onTouchEvent" {
		return
	}
	classIdx, ok := flatEnclosingAncestor(file, m, "class_declaration")
	if !ok || !javaViewSuperclass(file, classIdx, 0) {
		return
	}
	var hasPerformClick bool
	if body, ok := file.FlatFindChild(classIdx, "class_body"); ok {
		for sibling := file.FlatFirstChild(body); sibling != 0; sibling = file.FlatNextSib(sibling) {
			if file.FlatType(sibling) != "method_declaration" {
				continue
			}
			if id, ok := file.FlatFindChild(sibling, "identifier"); ok && file.FlatNodeText(id) == "performClick" {
				hasPerformClick = true
			}
		}
	}
	if hasPerformClick && javaBodyCalls(file, m, "performClick") {
		return
	}
	r.reportMissingPerformClick(ctx, name, extractIdentifierFlat(file, classIdx), hasPerformClick)
}

func (r *ClickableViewAccessibilitySourceRule) checkJavaListener(ctx *api.Context, call uint32) {
	file := ctx.File
	if javaMethodInvocationName(file, call) != "setOnTouchListener" {
		return
	}
	args, ok := file.FlatFindChild(call, "argument_list")
	if !ok {
		return
	}
	file.FlatWalkNodes(args, "lambda_expression", func(lambda uint32) {
		if !javaBodyCalls(file, lambda, "performClick") {
			ctx.EmitAt(file.FlatRow(lambda)+1, file.FlatCol(lambda)+1, "`onTouch` lambda should call `View#performClick`"+clickableTouchMessage)
		}
	})
	file.FlatWalkNodes(args, "method_declaration", func(m uint32) {
		name, ok := file.FlatFindChild(m, "identifier")
		if !ok || file.FlatNodeText(name) != "onTouch" || javaBodyCalls(file, m, "performClick") {
			return
		}
		ctx.EmitAt(file.FlatRow(name)+1, file.FlatCol(name)+1, "`onTouch` should call `View#performClick`"+clickableTouchMessage)
	})
}
