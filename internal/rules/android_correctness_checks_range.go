package rules

import (
	"math"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

type RangeRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *RangeRule) NodeTypes() []string { return []string{"call_expression"} }

// Confidence reports a tier-2 (medium) base confidence. The rule is
// structural and requires an explicit API/range anchor, but framework
// calls still depend on type-oracle call-target availability.
// Classified per roadmap/17.
func (r *RangeRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *RangeRule) check(ctx *api.Context) {
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

func rangeSummaryForFile(file *scanner.File) *rangeFileSummary {
	if file == nil {
		return &rangeFileSummary{constants: map[string][]rangeConstantSpec{}, functions: map[string][]rangeFunctionSpec{}}
	}
	return filefacts.FileFact(fileFactsCache(), file, slotRangeSummary, func() *rangeFileSummary {
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
		return summary
	})
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

func rangeSpecsForCall(ctx *api.Context, call uint32, callName string, args uint32, summary *rangeFileSummary) []rangeArgSpec {
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
