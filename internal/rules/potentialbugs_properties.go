package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// ---------------------------------------------------------------------------
// PropertyUsedBeforeDeclarationRule detects using property before declared.
// Uses DispatchBase on class_body to correctly identify class-level properties
// and avoid false positives from function bodies, lambdas, and init blocks.
// ---------------------------------------------------------------------------
type PropertyUsedBeforeDeclarationRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs properties rule. Detection is structural with heuristic
// fallbacks for flow-dependent cases. Classified per roadmap/17.
func (r *PropertyUsedBeforeDeclarationRule) Confidence() float64 { return 0.75 }

func (r *PropertyUsedBeforeDeclarationRule) NodeTypes() []string { return []string{"class_body"} }

func (r *PropertyUsedBeforeDeclarationRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	type propInfo struct {
		name  string
		node  uint32
		index int // order among direct children
	}

	// First pass: collect class-level property declarations (direct children of class_body).
	var props []propInfo
	propByName := map[string]int{} // property name -> index in class_body children
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) != "property_declaration" {
			continue
		}
		name := propertyDeclarationNameFlat(file, child)
		if name == "" {
			continue
		}
		props = append(props, propInfo{name, child, i})
		propByName[name] = i
	}
	if len(props) == 0 {
		return nil
	}

	var findings []scanner.Finding

	// collectIdentifiers gathers all simple_identifier text from a subtree,
	// but does NOT descend into function_declaration or lambda_literal nodes
	// (those execute lazily, so references there are fine).
	var collectIdentifiers func(n uint32) []string
	collectIdentifiers = func(n uint32) []string {
		var ids []string
		switch file.FlatType(n) {
		case "function_declaration", "lambda_literal":
			return nil
		}
		if file.FlatType(n) == "simple_identifier" {
			ids = append(ids, file.FlatNodeText(n))
		}
		for i := 0; i < file.FlatChildCount(n); i++ {
			ids = append(ids, collectIdentifiers(file.FlatChild(n, i))...)
		}
		return ids
	}

	// Second pass: for each class-level property initializer AND each init block,
	// check if it references a property declared later.
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case "property_declaration":
			propName := propertyDeclarationNameFlat(file, child)
			if propName == "" {
				continue
			}
			// Collect identifiers from the entire property_declaration, minus the
			// property name itself and skipping lambdas/functions.
			refs := collectIdentifiers(child)
			for _, ref := range refs {
				if ref == propName {
					continue
				}
				declIdx, ok := propByName[ref]
				if ok && declIdx > i {
					findings = append(findings, r.Finding(file, file.FlatRow(child)+1, 1,
						fmt.Sprintf("Property '%s' uses '%s' which is declared later.", propName, ref)))
					break // one finding per property
				}
			}
		case "anonymous_initializer":
			// init {} blocks execute eagerly in declaration order.
			refs := collectIdentifiers(child)
			for _, ref := range refs {
				declIdx, ok := propByName[ref]
				if ok && declIdx > i {
					findings = append(findings, r.Finding(file, file.FlatRow(child)+1, 1,
						fmt.Sprintf("Init block uses '%s' which is declared later.", ref)))
					break
				}
			}
		}
	}
	return findings
}

// ---------------------------------------------------------------------------
// UnconditionalJumpStatementInLoopRule detects loop with unconditional return/break.
// ---------------------------------------------------------------------------
type UnconditionalJumpStatementInLoopRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs properties rule. Detection is structural with heuristic
// fallbacks for flow-dependent cases. Classified per roadmap/17.
func (r *UnconditionalJumpStatementInLoopRule) Confidence() float64 { return 0.75 }

func (r *UnconditionalJumpStatementInLoopRule) NodeTypes() []string {
	return []string{"for_statement", "while_statement", "do_while_statement"}
}

func (r *UnconditionalJumpStatementInLoopRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Get the loop body
	body := file.FlatFindChild(idx, "statements")
	if body == 0 {
		// Try to find the body block
		for i := 0; i < file.FlatChildCount(idx); i++ {
			child := file.FlatChild(idx, i)
			if file.FlatType(child) == "control_structure_body" {
				body = child
				break
			}
		}
	}
	if body == 0 {
		return nil
	}
	text := file.FlatNodeText(body)
	trimmed := strings.TrimSpace(text)
	// Remove outer braces
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		trimmed = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	}
	// Check if the first statement is an unconditional jump
	lines := strings.Split(trimmed, "\n")
	for _, l := range lines {
		lt := strings.TrimSpace(l)
		if lt == "" {
			continue
		}
		if strings.HasPrefix(lt, "return") || strings.HasPrefix(lt, "break") || strings.HasPrefix(lt, "continue") || strings.HasPrefix(lt, "throw") {
			// Check it's not inside an if/when
			if !strings.Contains(trimmed, "if ") && !strings.Contains(trimmed, "if(") && !strings.Contains(trimmed, "when") {
				return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
					"Unconditional jump statement in loop. The loop will only execute once.")}
			}
		}
		break // Only check the first non-empty statement
	}
	return nil
}

func (r *UnconditionalJumpStatementInLoopRule) Check(file *scanner.File) []scanner.Finding {
	return nil
}

// ---------------------------------------------------------------------------
// UnnamedParameterUseRule detects function calls with many unnamed params.
// ---------------------------------------------------------------------------
type UnnamedParameterUseRule struct {
	FlatDispatchBase
	BaseRule
	AllowSingleParamUse bool
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs properties rule. Detection is structural with heuristic
// fallbacks for flow-dependent cases. Classified per roadmap/17.
func (r *UnnamedParameterUseRule) Confidence() float64 { return 0.75 }

func (r *UnnamedParameterUseRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *UnnamedParameterUseRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return nil
	}
	// Count value_argument children
	count := 0
	hasNamed := false
	for i := 0; i < file.FlatChildCount(args); i++ {
		child := file.FlatChild(args, i)
		if file.FlatType(child) == "value_argument" {
			count++
			hasNamed = hasNamed || flatHasValueArgumentLabel(file, child)
		}
	}
	if count < 5 || hasNamed {
		return nil
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Function call with many unnamed parameters. Consider using named parameters for clarity.")}
}

// ---------------------------------------------------------------------------
// UnusedUnaryOperatorRule detects standalone +x or -x as expression statements
// whose result is never used. This catches the common bug where a line like
// `+ 3` appears to continue a previous expression but is actually parsed as
// a separate unary prefix expression with no effect.
// ---------------------------------------------------------------------------
type UnusedUnaryOperatorRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs properties rule. Detection is structural with heuristic
// fallbacks for flow-dependent cases. Classified per roadmap/17.
func (r *UnusedUnaryOperatorRule) Confidence() float64 { return 0.75 }

func (r *UnusedUnaryOperatorRule) NodeTypes() []string { return []string{"prefix_expression"} }

func (r *UnusedUnaryOperatorRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Only care about unary + and -
	if file.FlatChildCount(idx) < 2 {
		return nil
	}
	op := file.FlatNodeText(file.FlatChild(idx, 0))
	if op != "+" && op != "-" {
		return nil
	}

	// Walk up through binary_expression parents to find the top-level
	// expression this prefix_expression belongs to (e.g. + 3 + 4 + 5).
	topExpr := idx
	for {
		parent, ok := file.FlatParent(topExpr)
		if !ok || file.FlatType(parent) != "binary_expression" {
			break
		}
		topExpr = parent
	}

	stmts, ok := file.FlatParent(topExpr)
	if !ok {
		return nil
	}

	// The expression is unused if its parent is a "statements" block
	// that is NOT acting as an expression body (if/when/lambda return value).
	if file.FlatType(stmts) != "statements" {
		return nil
	}

	// Check if this is the last expression in a block used as a value
	// (if_expression, when_entry, lambda_literal, try/catch). In those
	// contexts the last statement is the implicit return value, so it IS used.
	if flatIsLastNamedChildOf(file, topExpr, stmts) && flatIsExpressionBlock(file, stmts) {
		return nil
	}

	text := file.FlatNodeText(idx)
	// If the node is inside a parent binary expression, report the whole thing
	if file.FlatType(topExpr) == "binary_expression" {
		text = file.FlatNodeText(topExpr)
	}

	row := file.FlatRow(idx) + 1
	col := file.FlatCol(idx) + 1
	return []scanner.Finding{r.Finding(file, row, col,
		fmt.Sprintf("Unused unary operator. The result of '%s' is not used.", text))}
}

// flatIsLastNamedChildOf checks if child is the last named child of parent.
func flatIsLastNamedChildOf(file *scanner.File, child, parent uint32) bool {
	count := file.FlatNamedChildCount(parent)
	if count == 0 {
		return false
	}
	return file.FlatNamedChild(parent, count-1) == child
}

// flatIsExpressionBlock checks if a "statements" node is part of a construct
// that uses its last expression as a value (if/when/lambda/try-catch bodies).
func flatIsExpressionBlock(file *scanner.File, stmts uint32) bool {
	parent, ok := file.FlatParent(stmts)
	if !ok {
		return false
	}
	// statements -> control_structure_body -> if_expression / when_entry
	// statements -> lambda_literal
	// statements -> catch_block -> try_expression
	// statements -> finally_block -> try_expression
	pt := file.FlatType(parent)
	if pt == "lambda_literal" {
		return true
	}
	if pt == "control_structure_body" {
		gp, ok := file.FlatParent(parent)
		if ok {
			gpt := file.FlatType(gp)
			if gpt == "if_expression" || gpt == "when_entry" || gpt == "when_expression" {
				return true
			}
		}
	}
	// try/catch/finally are expressions in Kotlin — last statement in the
	// try block, catch block, or finally block is the implicit return value.
	if pt == "catch_block" || pt == "finally_block" {
		gp, ok := file.FlatParent(parent)
		if ok && file.FlatType(gp) == "try_expression" {
			return true
		}
	}
	// statements directly inside try_expression (the try block body)
	if pt == "try_expression" {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// UselessPostfixExpressionRule detects `return x++` or `return x--`.
// ---------------------------------------------------------------------------
type UselessPostfixExpressionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs properties rule. Detection is structural with heuristic
// fallbacks for flow-dependent cases. Classified per roadmap/17.
func (r *UselessPostfixExpressionRule) Confidence() float64 { return 0.75 }

func (r *UselessPostfixExpressionRule) NodeTypes() []string { return []string{"jump_expression"} }

var uselessPostfixRe = regexp.MustCompile(`\breturn\s+\w+(\+\+|--)`)
var uselessPostfixFixRe = regexp.MustCompile(`(\s*)return\s+(\w+)(\+\+|--)`)

func (r *UselessPostfixExpressionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if !uselessPostfixRe.MatchString(text) {
		return nil
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Useless postfix expression in return statement. The increment/decrement has no effect.")
	if m := uselessPostfixFixRe.FindStringSubmatch(text); m != nil {
		indent := m[1]
		varName := m[2]
		op := m[3]
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(idx)),
			EndByte:     int(file.FlatEndByte(idx)),
			Replacement: indent + varName + op + "\n" + indent + "return " + varName,
		}
	}
	return []scanner.Finding{f}
}

func propertyDeclarationNameFlat(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	varDecl := file.FlatFindChild(idx, "variable_declaration")
	if varDecl == 0 {
		return ""
	}
	ident := file.FlatFindChild(varDecl, "simple_identifier")
	if ident == 0 {
		return ""
	}
	return file.FlatNodeText(ident)
}
