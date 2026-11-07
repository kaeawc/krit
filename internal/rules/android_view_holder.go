package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const viewHolderMessage = "Unconditional layout inflation from view adapter: Should use View Holder pattern (use recycled view passed into this method as the second parameter) for smoother scrolling"

// viewHolderParameterTypes returns the simple type names of a function's parameters, without nullability.
func viewHolderParameterTypes(file *scanner.File, fn uint32) []string {
	params, _ := file.FlatFindChild(fn, "function_value_parameters")
	if params == 0 {
		return nil
	}
	var types []string
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "parameter" {
			continue
		}
		text := file.FlatNodeText(child)
		_, typ, _ := strings.Cut(text, ":")
		typ = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(typ), "?"))
		typ = typ[strings.LastIndex(typ, ".")+1:]
		types = append(types, typ)
	}
	return types
}

// checkViewHolder follows Lint's ViewHolderDetector: in any method getView(int, View, ViewGroup), an inflate call
// with a receiver reports unless some inflate call in the method sits inside an if, when or elvis expression.
func checkViewHolder(ctx *api.Context) {
	file, fn := ctx.File, ctx.Idx
	if extractIdentifierFlat(file, fn) != "getView" {
		return
	}
	types := viewHolderParameterTypes(file, fn)
	if len(types) != 3 || types[0] != "Int" || types[1] != "View" || types[2] != "ViewGroup" {
		return
	}
	body, _ := file.FlatFindChild(fn, "function_body")
	if body == 0 {
		return
	}
	var unconditional []uint32
	haveConditional := false
	file.FlatWalkNodes(body, "call_expression", func(call uint32) {
		if flatCallExpressionName(file, call) != "inflate" {
			return
		}
		// A bare inflate(...) has no receiver, and Lint only looks at calls with at least one argument.
		if navExpr, args := flatCallExpressionParts(file, call); navExpr == 0 || args == 0 || file.FlatNamedChildCount(args) < 1 {
			return
		}
		if viewHolderInsideConditional(file, body, call) {
			haveConditional = true
			return
		}
		unconditional = append(unconditional, call)
	})
	if haveConditional {
		return
	}
	for _, call := range unconditional {
		ctx.EmitAt(callLine(file, call), callCol(file, call), viewHolderMessage)
	}
}

// viewHolderInsideConditional reports whether call has an if, when or elvis expression between it and the body.
func viewHolderInsideConditional(file *scanner.File, body, call uint32) bool {
	for node, ok := file.FlatParent(call); ok && node != body; node, ok = file.FlatParent(node) {
		switch file.FlatType(node) {
		case "if_expression", "when_expression", "elvis_expression":
			return true
		}
	}
	return false
}
