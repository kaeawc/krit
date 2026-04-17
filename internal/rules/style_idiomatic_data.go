package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

type UseArrayLiteralsInAnnotationsRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseArrayLiteralsInAnnotationsRule) Confidence() float64 { return 0.75 }

func (r *UseArrayLiteralsInAnnotationsRule) NodeTypes() []string { return []string{"annotation"} }
func (r *UseArrayLiteralsInAnnotationsRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if !strings.Contains(text, "arrayOf(") {
		return nil
	}
	f := r.Finding(file, file.FlatRow(idx)+1, 1, "Use array literal '[]' syntax in annotations instead of 'arrayOf()'.")
	nodeStart := int(file.FlatStartByte(idx))
	loc := strings.Index(text, "arrayOf(")
	if loc >= 0 {
		depth := 1
		start := loc + len("arrayOf(")
		end := -1
		for k := start; k < len(text); k++ {
			if text[k] == '(' {
				depth++
			} else if text[k] == ')' {
				depth--
				if depth == 0 {
					end = k
					break
				}
			}
		}
		if end >= 0 {
			inner := text[start:end]
			f.Fix = &scanner.Fix{ByteMode: true, StartByte: nodeStart, EndByte: int(file.FlatEndByte(idx)), Replacement: text[:loc] + "[" + inner + "]" + text[end+1:]}
		}
	}
	return []scanner.Finding{f}
}

type UseSumOfInsteadOfFlatMapSizeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseSumOfInsteadOfFlatMapSizeRule) Confidence() float64 { return 0.75 }

func (r *UseSumOfInsteadOfFlatMapSizeRule) NodeTypes() []string { return []string{"call_expression"} }

var sumOfSourceCalls = map[string]bool{"flatMap": true, "flatten": true, "map": true}

func (r *UseSumOfInsteadOfFlatMapSizeRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if !sumOfSourceCalls[name] {
		return nil
	}
	parent, ok := file.FlatParent(idx)
	if !ok {
		return nil
	}
	var selectorName string
	var chainEnd uint32
	if file.FlatType(parent) == "navigation_expression" {
		suffix := sumOfNavSelectorFlat(file, parent)
		gp, ok := file.FlatParent(parent)
		if ok && file.FlatType(gp) == "call_expression" {
			if outerName := flatCallExpressionName(file, gp); outerName != "" {
				selectorName = outerName
				chainEnd = gp
			}
		}
		if selectorName == "" && suffix != "" {
			selectorName = suffix
			chainEnd = parent
		}
	}
	if selectorName == "" {
		return nil
	}
	var msg string
	switch selectorName {
	case "size":
		msg = fmt.Sprintf("Use 'sumOf' instead of '%s' and 'size'.", name)
	case "count":
		msg = fmt.Sprintf("Use 'sumOf' instead of '%s' and 'count'.", name)
	case "sum":
		if name != "map" {
			return nil
		}
		msg = "Use 'sumOf' instead of 'map' and 'sum'."
	default:
		return nil
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
	if name == "flatMap" && selectorName == "size" && chainEnd != 0 {
		if lambdaSuffix := file.FlatFindChild(idx, "call_suffix"); lambdaSuffix != 0 {
			if lambdaNode := file.FlatFindChild(lambdaSuffix, "annotated_lambda"); lambdaNode != 0 {
				body := strings.TrimSpace(file.FlatNodeText(lambdaNode))
				if len(body) >= 2 && body[0] == '{' && body[len(body)-1] == '}' {
					body = strings.TrimSpace(body[1 : len(body)-1])
				}
				var receiverText string
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					if file.FlatType(child) == "navigation_expression" {
						if file.FlatChildCount(child) > 0 {
							receiverText = file.FlatNodeText(file.FlatChild(child, 0))
						}
						break
					} else if file.FlatType(child) == "simple_identifier" && file.FlatNodeTextEquals(child, "flatMap") {
						break
					}
				}
				if receiverText != "" {
					f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(chainEnd)), EndByte: int(file.FlatEndByte(chainEnd)), Replacement: receiverText + ".sumOf { " + body + ".size }"}
				} else {
					f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(chainEnd)), EndByte: int(file.FlatEndByte(chainEnd)), Replacement: "sumOf { " + body + ".size }"}
				}
			}
		}
	}
	return []scanner.Finding{f}
}

func sumOfNavSelectorFlat(file *scanner.File, nav uint32) string {
	for i := file.FlatChildCount(nav) - 1; i >= 0; i-- {
		child := file.FlatChild(nav, i)
		if file.FlatType(child) == "simple_identifier" {
			return file.FlatNodeText(child)
		}
		if file.FlatType(child) == "navigation_suffix" {
			for j := 0; j < file.FlatChildCount(child); j++ {
				gc := file.FlatChild(child, j)
				if file.FlatType(gc) == "simple_identifier" {
					return file.FlatNodeText(gc)
				}
			}
		}
	}
	return ""
}

type UseLetRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseLetRule) Confidence() float64 { return 0.75 }

func (r *UseLetRule) NodeTypes() []string { return []string{"if_expression"} }
func (r *UseLetRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if strings.Contains(text, "!= null") && !strings.Contains(text, "else") {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1, "Null check could be replaced with ?.let { }.")}
	}
	return nil
}

type UseDataClassRule struct {
	FlatDispatchBase
	BaseRule
	AllowVars bool
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseDataClassRule) Confidence() float64 { return 0.75 }

func (r *UseDataClassRule) NodeTypes() []string { return []string{"class_declaration"} }
func (r *UseDataClassRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if file.FlatHasModifier(idx, "data") || file.FlatHasModifier(idx, "abstract") || file.FlatHasModifier(idx, "open") || file.FlatHasModifier(idx, "sealed") || file.FlatHasModifier(idx, "enum") || file.FlatHasModifier(idx, "annotation") {
		return nil
	}
	ctor := file.FlatFindChild(idx, "primary_constructor")
	if ctor == 0 {
		return nil
	}
	paramCount := 0
	file.FlatWalkNodes(ctor, "class_parameter", func(p uint32) {
		trimmed := strings.TrimSpace(file.FlatNodeText(p))
		if strings.HasPrefix(trimmed, "val ") || strings.HasPrefix(trimmed, "var ") {
			paramCount++
		}
	})
	if paramCount == 0 {
		return nil
	}
	body := file.FlatFindChild(idx, "class_body")
	if body != 0 {
		for i := 0; i < file.FlatChildCount(body); i++ {
			if file.FlatType(file.FlatChild(body, i)) == "function_declaration" {
				return nil
			}
		}
	}
	name := extractIdentifierFlat(file, idx)
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1, fmt.Sprintf("Class '%s' could be a data class.", name))}
}

type UseIfInsteadOfWhenRule struct {
	FlatDispatchBase
	BaseRule
	IgnoreWhenContainingVariableDeclaration bool
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseIfInsteadOfWhenRule) Confidence() float64 { return 0.75 }

func (r *UseIfInsteadOfWhenRule) NodeTypes() []string { return []string{"when_expression"} }
func (r *UseIfInsteadOfWhenRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	entryCount := 0
	for i := 0; i < file.FlatChildCount(idx); i++ {
		if file.FlatType(file.FlatChild(idx, i)) == "when_entry" {
			entryCount++
		}
	}
	if entryCount <= 2 {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1, "When expression with two or fewer branches could be replaced with if.")}
	}
	return nil
}

type UseIfEmptyOrIfBlankRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseIfEmptyOrIfBlankRule) Confidence() float64 { return 0.75 }

func (r *UseIfEmptyOrIfBlankRule) NodeTypes() []string { return []string{"if_expression"} }

var ifEmptyOrBlankMethods = map[string]struct {
	replacement string
	negated     bool
}{"isEmpty": {"ifEmpty", false}, "isBlank": {"ifBlank", false}, "isNotEmpty": {"ifEmpty", true}, "isNotBlank": {"ifBlank", true}}

func (r *UseIfEmptyOrIfBlankRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	var condNode, thenNode, elseNode uint32
	sawElse := false
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case "if", "(", ")", "{", "}":
			continue
		case "else":
			sawElse = true
			continue
		default:
			if condNode == 0 {
				condNode = child
			} else if !sawElse {
				thenNode = child
			} else if elseNode == 0 {
				elseNode = child
			}
		}
	}
	if condNode == 0 || thenNode == 0 || elseNode == 0 {
		return nil
	}
	if file.FlatType(elseNode) == "if_expression" {
		return nil
	}
	condText := strings.TrimSpace(file.FlatNodeText(condNode))
	isNegatedPrefix := false
	innerCondText := condText
	if file.FlatType(condNode) == "prefix_expression" && file.FlatChildCount(condNode) >= 2 {
		if opNode := file.FlatChild(condNode, 0); opNode != 0 && file.FlatNodeTextEquals(opNode, "!") {
			isNegatedPrefix = true
			if argNode := file.FlatChild(condNode, 1); argNode != 0 {
				innerCondText = strings.TrimSpace(file.FlatNodeText(argNode))
			}
		}
	}
	parenIdx := strings.LastIndex(innerCondText, "()")
	if parenIdx < 0 {
		return nil
	}
	beforeParen := innerCondText[:parenIdx]
	dotIdx := strings.LastIndex(beforeParen, ".")
	if dotIdx < 0 {
		return nil
	}
	receiver := beforeParen[:dotIdx]
	methodName := beforeParen[dotIdx+1:]
	info, ok := ifEmptyOrBlankMethods[methodName]
	if !ok {
		return nil
	}
	negated := info.negated != isNegatedPrefix
	var selfBranch, defaultBranch uint32
	if negated {
		selfBranch = thenNode
		defaultBranch = elseNode
	} else {
		selfBranch = elseNode
		defaultBranch = thenNode
	}
	selfText := strings.TrimSpace(file.FlatNodeText(selfBranch))
	if strings.HasPrefix(selfText, "{") && strings.HasSuffix(selfText, "}") {
		selfText = strings.TrimSpace(selfText[1 : len(selfText)-1])
	}
	if selfText != receiver {
		return nil
	}
	defaultText := strings.TrimSpace(file.FlatNodeText(defaultBranch))
	if strings.HasPrefix(defaultText, "{") && strings.HasSuffix(defaultText, "}") {
		defaultText = strings.TrimSpace(defaultText[1 : len(defaultText)-1])
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, fmt.Sprintf("Use '.%s {}' instead of manual %s() check.", info.replacement, methodName))
	f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(idx)), EndByte: int(file.FlatEndByte(idx)), Replacement: receiver + "." + info.replacement + " { " + defaultText + " }"}
	return []scanner.Finding{f}
}

type ExplicitCollectionElementAccessMethodRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *ExplicitCollectionElementAccessMethodRule) Confidence() float64 { return 0.75 }

func (r *ExplicitCollectionElementAccessMethodRule) NodeTypes() []string {
	return []string{"call_expression"}
}
func (r *ExplicitCollectionElementAccessMethodRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	var navNode uint32
	var methodName string
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "navigation_expression" {
			navText := file.FlatNodeText(child)
			if strings.HasSuffix(navText, ".get") {
				navNode = child
				methodName = "get"
				break
			}
			if strings.HasSuffix(navText, ".set") {
				navNode = child
				methodName = "set"
				break
			}
		}
	}
	if navNode == 0 {
		return nil
	}
	argsNode := uint32(0)
	if callSuffix := file.FlatFindChild(idx, "call_suffix"); callSuffix != 0 {
		argsNode = file.FlatFindChild(callSuffix, "value_arguments")
	}
	if argsNode == 0 {
		return nil
	}
	argCount := 0
	for i := 0; i < file.FlatChildCount(argsNode); i++ {
		if file.FlatType(file.FlatChild(argsNode, i)) == "value_argument" {
			argCount++
		}
	}
	if methodName == "get" && argCount < 1 {
		return nil
	}
	if methodName == "set" && argCount < 2 {
		return nil
	}
	row := file.FlatRow(idx) + 1
	col := file.FlatCol(idx) + 1
	var argTexts []string
	for i := 0; i < file.FlatChildCount(argsNode); i++ {
		child := file.FlatChild(argsNode, i)
		if file.FlatType(child) == "value_argument" {
			argTexts = append(argTexts, file.FlatNodeText(child))
		}
	}
	receiver := file.FlatChild(navNode, 0)
	if receiver == 0 {
		return nil
	}
	receiverText := file.FlatNodeText(receiver)
	if methodName == "get" {
		f := r.Finding(file, row, col, "Use index operator instead of explicit 'get' call.")
		f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(idx)), EndByte: int(file.FlatEndByte(idx)), Replacement: receiverText + "[" + strings.Join(argTexts, ", ") + "]"}
		return []scanner.Finding{f}
	}
	keys := strings.Join(argTexts[:len(argTexts)-1], ", ")
	value := argTexts[len(argTexts)-1]
	f := r.Finding(file, row, col, "Use index operator instead of explicit 'set' call.")
	f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(idx)), EndByte: int(file.FlatEndByte(idx)), Replacement: receiverText + "[" + keys + "] = " + value}
	return []scanner.Finding{f}
}

type AlsoCouldBeApplyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *AlsoCouldBeApplyRule) Confidence() float64 { return 0.75 }

func (r *AlsoCouldBeApplyRule) NodeTypes() []string { return []string{"call_expression"} }
func (r *AlsoCouldBeApplyRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if !strings.Contains(text, ".also") {
		return nil
	}
	lambdaStart := strings.Index(text, "{")
	if lambdaStart < 0 {
		return nil
	}
	if strings.Count(text[lambdaStart:], "it.") >= 2 {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1, "'also' with multiple 'it.' references could be replaced with 'apply'.")}
	}
	return nil
}

type EqualsNullCallRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *EqualsNullCallRule) Confidence() float64 { return 0.75 }

func (r *EqualsNullCallRule) NodeTypes() []string { return []string{"call_expression"} }
func (r *EqualsNullCallRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	var navNode uint32
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "navigation_expression" && strings.HasSuffix(file.FlatNodeText(child), ".equals") {
			navNode = child
			break
		}
	}
	if navNode == 0 {
		return nil
	}
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if (file.FlatType(child) == "call_suffix" || file.FlatType(child) == "value_arguments") && strings.Contains(file.FlatNodeText(child), "null") {
			f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Use '== null' instead of '.equals(null)'.")
			navText := file.FlatNodeText(navNode)
			if dotIdx := strings.LastIndex(navText, ".equals"); dotIdx >= 0 {
				f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(navNode)) + dotIdx, EndByte: int(file.FlatEndByte(idx)), Replacement: " == null"}
			}
			return []scanner.Finding{f}
		}
	}
	return nil
}
