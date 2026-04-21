package typeinfer

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

type flatScopeSpan struct {
	start uint32
	end   uint32
}

func (s flatScopeSpan) StartByte() uint32 { return s.start }
func (s flatScopeSpan) EndByte() uint32   { return s.end }

func (r *defaultResolver) buildScopesFlat(idx uint32, file *scanner.File, scope *ScopeTable, it *ImportTable) {
	if file == nil || file.FlatTree == nil || int(idx) >= len(file.FlatTree.Nodes) || scope == nil {
		return
	}
	if idx == 0 && file.FlatType(0) == "source_file" {
		for i := 0; i < file.FlatNamedChildCount(0); i++ {
			child := file.FlatNamedChild(0, i)
			if child == 0 {
				continue
			}
			switch file.FlatType(child) {
			case "function_declaration", "lambda_literal", "class_declaration", "object_declaration", "class_body",
				"if_expression", "when_expression", "for_statement",
				"property_declaration", "call_expression", "elvis_expression",
				"control_structure_body", "statements", "function_body",
				"catch_block", "finally_block", "binary_expression",
				"conjunction_expression", "disjunction_expression",
				"call_suffix", "lambda_argument", "annotated_lambda":
				r.buildScopesFlat(child, file, scope, it)
			}
		}
		r.buildFlatSmartCastScopes(file, scope)
		return
	}

	switch file.FlatType(idx) {
	case "function_declaration":
		childScope := scope.NewScopeForNode(flatScopeSpan{start: file.FlatStartByte(idx), end: file.FlatEndByte(idx)})
		r.indexFunctionParamsFlat(idx, file, childScope, it)
		if bodyIdx := flatFindNamedChildOfType(file, idx, "function_body"); bodyIdx != 0 {
			flatForEachRelevantScopeChild(file, bodyIdx, func(child uint32) {
				r.buildScopesFlat(child, file, childScope, it)
			})
		}
		return

	case "lambda_literal":
		childScope := scope.NewScopeForNode(flatScopeSpan{start: file.FlatStartByte(idx), end: file.FlatEndByte(idx)})
		if params := flatFindNamedChildOfType(file, idx, "lambda_parameters"); params != 0 {
			r.indexLambdaParamsFlat(params, file, childScope, it)
		} else {
			r.inferImplicitItTypeFlat(idx, file, childScope, it)
		}
		flatForEachRelevantScopeChild(file, idx, func(child uint32) {
			if file.FlatType(child) != "lambda_parameters" {
				r.buildScopesFlat(child, file, childScope, it)
			}
		})
		return

	case "class_body":
		childScope := scope.NewScopeForNode(flatScopeSpan{start: file.FlatStartByte(idx), end: file.FlatEndByte(idx)})
		if parent, ok := file.FlatParent(idx); ok {
			if pt := file.FlatType(parent); pt == "class_declaration" || pt == "object_declaration" {
				className := flatFirstTypeOrSimpleName(file, parent)
				if className != "" {
					childScope.Declare("this", r.makeResolvedType(className, it, false))
				}
			}
		}
		flatForEachRelevantScopeChild(file, idx, func(child uint32) {
			r.buildScopesFlat(child, file, childScope, it)
		})
		return

	case "if_expression":
		r.buildIfExpressionScopesFlat(idx, file, scope, it)
		return

	case "when_expression":
		r.buildWhenExpressionScopesFlat(idx, file, scope, it)
		return

	case "for_statement":
		childScope := scope.NewScopeForNode(flatScopeSpan{start: file.FlatStartByte(idx), end: file.FlatEndByte(idx)})
		r.indexForVariableFlat(idx, file, childScope)
		flatForEachRelevantScopeChild(file, idx, func(child uint32) {
			r.buildScopesFlat(child, file, childScope, it)
		})
		return

	case "property_declaration":
		if multiVar := flatFindNamedChildOfType(file, idx, "multi_variable_declaration"); multiVar != 0 {
			r.indexDestructuringDeclarationFlat(idx, multiVar, file, scope, it)
		} else {
			name := ""
			if varDecl := flatFindNamedChildOfType(file, idx, "variable_declaration"); varDecl != 0 {
				name = flatFirstIdentifierText(file, varDecl)
			}
			if name == "" {
				name = flatFirstIdentifierText(file, idx)
			}
			if name != "" && scope.Lookup(name) == nil {
				scope.Declare(name, r.resolvePropertyTypeFlat(idx, file, it))
			}
		}
	}

	flatForEachRelevantScopeChild(file, idx, func(child uint32) {
		r.buildScopesFlat(child, file, scope, it)
	})

	parent, ok := file.FlatParent(idx)
	if !ok || parent == 0 {
		r.buildFlatSmartCastScopes(file, scope)
	}
}

func (r *defaultResolver) buildIfExpressionScopesFlat(idx uint32, file *scanner.File, scope *ScopeTable, it *ImportTable) {
	nullVars, isChecks := r.extractConjunctionChecksFlat(idx, file, it)
	if len(nullVars) > 0 || len(isChecks) > 0 {
		flatForEachRelevantScopeChild(file, idx, func(child uint32) {
			if file.FlatType(child) == "control_structure_body" {
				childScope := scope.NewScopeForNode(flatScopeSpan{start: file.FlatStartByte(child), end: file.FlatEndByte(child)})
				for _, v := range nullVars {
					childScope.SmartCasts[v] = true
				}
				for varName, typ := range isChecks {
					childScope.SmartCastTypes[varName] = typ
				}
				flatForEachRelevantScopeChild(file, child, func(gc uint32) {
					r.buildScopesFlat(gc, file, childScope, it)
				})
			} else {
				r.buildScopesFlat(child, file, scope, it)
			}
		})
		return
	}

	varName, isNotNullCheck, isNullCheck, _ := r.extractNullCheckFromIfFlatByIdx(idx, file)
	if varName != "" && isNotNullCheck {
		flatForEachRelevantScopeChild(file, idx, func(child uint32) {
			if file.FlatType(child) == "control_structure_body" {
				childScope := scope.NewScopeForNode(flatScopeSpan{start: file.FlatStartByte(child), end: file.FlatEndByte(child)})
				childScope.SmartCasts[varName] = true
				flatForEachRelevantScopeChild(file, child, func(gc uint32) {
					r.buildScopesFlat(gc, file, childScope, it)
				})
			} else {
				r.buildScopesFlat(child, file, scope, it)
			}
		})
		return
	}

	if varName != "" && isNullCheck && r.ifBodyIsEarlyReturnFlat(idx, file) {
		scope.SmartCasts[varName] = true
	}

	isVarName, targetType, isPositive, _ := r.extractIsCheckFromIfFlatByIdx(idx, file, it)
	if isVarName != "" && targetType != nil {
		if isPositive {
			flatForEachRelevantScopeChild(file, idx, func(child uint32) {
				if file.FlatType(child) == "control_structure_body" {
					childScope := scope.NewScopeForNode(flatScopeSpan{start: file.FlatStartByte(child), end: file.FlatEndByte(child)})
					childScope.SmartCastTypes[isVarName] = targetType
					flatForEachRelevantScopeChild(file, child, func(gc uint32) {
						r.buildScopesFlat(gc, file, childScope, it)
					})
				} else {
					r.buildScopesFlat(child, file, scope, it)
				}
			})
			return
		}
		if r.ifBodyIsEarlyReturnFlat(idx, file) {
			scope.SmartCastTypes[isVarName] = targetType
		}
	}

	flatForEachRelevantScopeChild(file, idx, func(child uint32) {
		r.buildScopesFlat(child, file, scope, it)
	})
}

func (r *defaultResolver) buildWhenExpressionScopesFlat(idx uint32, file *scanner.File, scope *ScopeTable, it *ImportTable) {
	subjectName := flatWhenSubjectNameByIdx(file, idx)
	if subjectName == "" {
		flatForEachRelevantScopeChild(file, idx, func(child uint32) {
			r.buildScopesFlat(child, file, scope, it)
		})
		return
	}

	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		child := file.FlatNamedChild(idx, i)
		if child == 0 || file.FlatType(child) != "when_entry" {
			continue
		}
		targetType := flatWhenEntryTargetType(file, child, it)
		flatForEachRelevantScopeChild(file, child, func(body uint32) {
			if file.FlatType(body) == "control_structure_body" || file.FlatType(body) == "statements" {
				childScope := scope.NewScopeForNode(flatScopeSpan{start: file.FlatStartByte(body), end: file.FlatEndByte(body)})
				if targetType != nil && targetType.Kind != TypeUnknown {
					childScope.SmartCastTypes[subjectName] = targetType
				}
				flatForEachRelevantScopeChild(file, body, func(inner uint32) {
					r.buildScopesFlat(inner, file, childScope, it)
				})
			}
		})
	}
}

func flatWhenSubjectNameByIdx(file *scanner.File, whenIdx uint32) string {
	if file == nil || file.FlatTree == nil || whenIdx == 0 {
		return ""
	}
	if whenIdx == 0 {
		return ""
	}
	if file.FlatType(whenIdx) != "when_expression" {
		for current, ok := file.FlatParent(whenIdx); ok; current, ok = file.FlatParent(current) {
			if file.FlatType(current) == "when_expression" {
				whenIdx = current
				break
			}
		}
		if file.FlatType(whenIdx) != "when_expression" {
			return ""
		}
	}
	subject, _ := file.FlatFindChild(whenIdx, "when_subject")
	if subject == 0 {
		return ""
	}
	ident := flatFirstDescendantOfType(file, subject, "simple_identifier")
	if ident == 0 {
		return ""
	}
	return file.FlatNodeText(ident)
}

func flatWhenEntryTargetType(file *scanner.File, entryIdx uint32, it *ImportTable) *ResolvedType {
	if file == nil || file.FlatTree == nil || entryIdx == 0 {
		return nil
	}
	cond, _ := file.FlatFindChild(entryIdx, "when_condition")
	if cond == 0 {
		return nil
	}
	typeTest := flatFirstDescendantOfType(file, cond, "type_test")
	if typeTest == 0 || file.FlatChildCount(typeTest) < 2 {
		return nil
	}
	op := file.FlatChild(typeTest, 0)
	if op == 0 || file.FlatType(op) != "is" {
		return nil
	}
	typeNode := file.FlatChild(typeTest, 1)
	if typeNode == 0 {
		return nil
	}
	typeName := strings.TrimSpace(file.FlatNodeText(typeNode))
	if typeName == "" {
		return nil
	}
	if idx := strings.Index(typeName, "<"); idx >= 0 {
		typeName = strings.TrimSpace(typeName[:idx])
	}
	nullable := false
	if strings.HasSuffix(typeName, "?") {
		nullable = true
		typeName = strings.TrimSpace(strings.TrimSuffix(typeName, "?"))
	}
	if typeName == "" {
		return nil
	}
	return (&defaultResolver{}).makeResolvedType(typeName, it, nullable)
}

func flatFirstDescendantOfType(file *scanner.File, root uint32, typeName string) uint32 {
	if file == nil || file.FlatTree == nil || root == 0 {
		return 0
	}
	if file.FlatType(root) == typeName {
		return root
	}
	var found uint32
	file.FlatForEachChild(root, func(child uint32) {
		if found != 0 {
			return
		}
		found = flatFirstDescendantOfType(file, child, typeName)
	})
	return found
}

func (r *defaultResolver) extractNullCheckFromIfFlatByIdx(ifIdx uint32, file *scanner.File) (string, bool, bool, bool) {
	if ifIdx == 0 {
		return "", false, false, false
	}
	condRoot := flatIfConditionNode(file, ifIdx)
	if condRoot == 0 {
		return "", false, false, false
	}

	condNode := flatFirstDescendantOfType(file, condRoot, "equality_expression")
	if condNode == 0 || file.FlatChildCount(condNode) < 3 {
		return "", false, false, false
	}

	left := file.FlatChild(condNode, 0)
	op := file.FlatChild(condNode, 1)
	right := file.FlatChild(condNode, 2)
	if left == 0 || op == 0 || right == 0 {
		return "", false, false, false
	}

	opText := strings.TrimSpace(file.FlatNodeText(op))
	leftText := strings.TrimSpace(file.FlatNodeText(left))
	rightText := strings.TrimSpace(file.FlatNodeText(right))

	var varName string
	if rightText == "null" && file.FlatType(left) == "simple_identifier" {
		varName = leftText
	} else if leftText == "null" && file.FlatType(right) == "simple_identifier" {
		varName = rightText
	}
	if varName == "" {
		return "", false, false, false
	}

	return varName, opText == "!=", opText == "==", true
}

func (r *defaultResolver) extractIsCheckFromIfFlatByIdx(ifIdx uint32, file *scanner.File, it *ImportTable) (string, *ResolvedType, bool, bool) {
	if ifIdx == 0 {
		return "", nil, false, false
	}
	condRoot := flatIfConditionNode(file, ifIdx)
	if condRoot == 0 {
		return "", nil, false, false
	}

	isNode := flatFirstDescendantOfType(file, condRoot, "check_expression")
	if isNode == 0 || file.FlatChildCount(isNode) < 3 {
		return "", nil, false, false
	}

	left := file.FlatChild(isNode, 0)
	op := file.FlatChild(isNode, 1)
	right := file.FlatChild(isNode, 2)
	if left == 0 || op == 0 || right == 0 || file.FlatType(left) != "simple_identifier" {
		return "", nil, false, false
	}

	varName := strings.TrimSpace(file.FlatNodeText(left))
	if varName == "" {
		return "", nil, false, false
	}

	opType := file.FlatType(op)
	isPositive := opType == "is"
	if opType != "is" && opType != "!is" {
		return "", nil, false, false
	}

	targetType := r.resolveTypeNodeFlat(right, file, it)
	if targetType == nil || targetType.Kind == TypeUnknown {
		typeName := strings.TrimSpace(file.FlatNodeText(right))
		if typeName != "" {
			targetType = r.makeResolvedType(typeName, it, false)
		}
	}
	if targetType == nil || targetType.Kind == TypeUnknown {
		return "", nil, false, false
	}

	return varName, targetType, isPositive, true
}

func (r *defaultResolver) extractConjunctionChecksFlat(ifIdx uint32, file *scanner.File, it *ImportTable) ([]string, map[string]*ResolvedType) {
	if file == nil || file.FlatTree == nil || ifIdx == 0 {
		return nil, nil
	}

	condRoot := flatIfConditionNode(file, ifIdx)
	if condRoot == 0 {
		return nil, nil
	}

	conjIdx := flatFirstDescendantOfType(file, condRoot, "conjunction_expression")
	if conjIdx == 0 {
		return nil, nil
	}

	var nullVars []string
	isChecks := make(map[string]*ResolvedType)

	for i := 0; i < file.FlatChildCount(conjIdx); i++ {
		child := file.FlatChild(conjIdx, i)
		if child == 0 {
			continue
		}
		switch file.FlatType(child) {
		case "equality_expression":
			if file.FlatChildCount(child) < 3 {
				continue
			}
			left := file.FlatChild(child, 0)
			op := file.FlatChild(child, 1)
			right := file.FlatChild(child, 2)
			if left == 0 || op == 0 || right == 0 {
				continue
			}
			if file.FlatNodeText(op) != "!=" {
				continue
			}
			leftText := file.FlatNodeText(left)
			rightText := file.FlatNodeText(right)
			if rightText == "null" && file.FlatType(left) == "simple_identifier" {
				nullVars = append(nullVars, leftText)
			} else if leftText == "null" && file.FlatType(right) == "simple_identifier" {
				nullVars = append(nullVars, rightText)
			}
		case "check_expression":
			if file.FlatChildCount(child) < 3 {
				continue
			}
			left := file.FlatChild(child, 0)
			op := file.FlatChild(child, 1)
			right := file.FlatChild(child, 2)
			if left == 0 || op == 0 || right == 0 {
				continue
			}
			if file.FlatType(left) != "simple_identifier" || file.FlatType(op) != "is" {
				continue
			}
			varName := file.FlatNodeText(left)
			targetType := r.resolveTypeNodeFlat(right, file, it)
			if targetType == nil || targetType.Kind == TypeUnknown {
				typeName := strings.TrimSpace(file.FlatNodeText(right))
				if typeName != "" {
					targetType = r.makeResolvedType(typeName, it, false)
				}
			}
			if targetType != nil && targetType.Kind != TypeUnknown {
				isChecks[varName] = targetType
			}
		}
	}

	return nullVars, isChecks
}

func flatIfConditionNode(file *scanner.File, ifIdx uint32) uint32 {
	if file == nil || file.FlatTree == nil || ifIdx == 0 {
		return 0
	}
	for i := 0; i < file.FlatNamedChildCount(ifIdx); i++ {
		child := file.FlatNamedChild(ifIdx, i)
		if child != 0 && file.FlatType(child) != "control_structure_body" {
			return child
		}
	}
	return 0
}

func flatFirstControlStructureBody(file *scanner.File, idx uint32) uint32 {
	if file == nil || file.FlatTree == nil || idx == 0 {
		return 0
	}
	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		child := file.FlatNamedChild(idx, i)
		if child != 0 && file.FlatType(child) == "control_structure_body" {
			return child
		}
	}
	return 0
}

func (r *defaultResolver) ifBodyIsEarlyReturnFlat(ifIdx uint32, file *scanner.File) bool {
	if bodyIdx := flatFirstControlStructureBody(file, ifIdx); bodyIdx != 0 {
		return flatNodeIsEarlyExit(file, bodyIdx)
	}
	return false
}

func (r *defaultResolver) buildFlatSmartCastScopes(file *scanner.File, scope *ScopeTable) {
	if file == nil || file.FlatTree == nil || scope == nil {
		return
	}

	file.FlatWalkAllNodes(0, func(idx uint32) {
		switch file.FlatType(idx) {
		case "call_expression":
			r.handleRequireNotNullFlat(idx, file, scope)
		case "elvis_expression":
			r.handleElvisEarlyExitFlat(idx, file, scope)
		}
	})
}

// handleRequireNotNullFlat checks for requireNotNull(x)/checkNotNull(x) calls
// and marks the argument as non-null in the most specific scope covering the call.
func (r *defaultResolver) handleRequireNotNullFlat(idx uint32, file *scanner.File, scope *ScopeTable) {
	if file == nil || file.FlatTree == nil || idx == 0 || scope == nil {
		return
	}

	firstChild := file.FlatChild(idx, 0)
	if firstChild == 0 || file.FlatType(firstChild) != "simple_identifier" {
		return
	}
	funcName := file.FlatNodeText(firstChild)
	if funcName != "requireNotNull" && funcName != "checkNotNull" {
		return
	}

	callScope := scope.FindScopeAtOffset(file.FlatStartByte(idx))
	if callScope == nil {
		callScope = scope
	}

	callSuffix := flatFindNamedChildOfType(file, idx, "call_suffix")
	if callSuffix == 0 {
		return
	}
	valueArgs := flatFindNamedChildOfType(file, callSuffix, "value_arguments")
	if valueArgs == 0 {
		return
	}
	for i := 0; i < file.FlatNamedChildCount(valueArgs); i++ {
		arg := file.FlatNamedChild(valueArgs, i)
		if arg == 0 || file.FlatType(arg) != "value_argument" {
			continue
		}
		for j := 0; j < file.FlatNamedChildCount(arg); j++ {
			inner := file.FlatNamedChild(arg, j)
			if inner != 0 && file.FlatType(inner) == "simple_identifier" {
				varName := file.FlatNodeText(inner)
				if varName != "" {
					callScope.SmartCasts[varName] = true
				}
				return
			}
		}
	}
}

// handleElvisEarlyExitFlat checks for `x ?: return` / `x ?: throw` patterns
// and marks x as non-null in the most specific scope covering the expression.
func (r *defaultResolver) handleElvisEarlyExitFlat(idx uint32, file *scanner.File, scope *ScopeTable) {
	if file == nil || file.FlatTree == nil || idx == 0 || scope == nil {
		return
	}
	if file.FlatType(idx) != "elvis_expression" || file.FlatChildCount(idx) < 3 {
		return
	}

	left := file.FlatChild(idx, 0)
	right := file.FlatChild(idx, 2)
	if left == 0 || right == 0 {
		return
	}

	isExit := flatNodeIsEarlyExit(file, right)
	if !isExit || file.FlatType(left) != "simple_identifier" {
		return
	}

	varName := file.FlatNodeText(left)
	if varName == "" {
		return
	}

	exprScope := scope.FindScopeAtOffset(file.FlatStartByte(idx))
	if exprScope == nil {
		exprScope = scope
	}
	exprScope.SmartCasts[varName] = true
}

func flatNodeIsEarlyExit(file *scanner.File, idx uint32) bool {
	if file == nil || file.FlatTree == nil || idx == 0 {
		return false
	}

	rightText := strings.TrimSpace(file.FlatNodeText(idx))
	if strings.HasPrefix(rightText, "return") || strings.HasPrefix(rightText, "throw") {
		return true
	}

	isExit := false
	file.FlatWalkAllNodes(idx, func(n uint32) {
		if isExit {
			return
		}
		switch file.FlatType(n) {
		case "jump_expression":
			text := file.FlatNodeText(n)
			if strings.HasPrefix(text, "return") || strings.HasPrefix(text, "throw") {
				isExit = true
			}
		case "call_expression":
			if file.FlatChildCount(n) > 0 {
				first := file.FlatChild(n, 0)
				if first != 0 && file.FlatType(first) == "simple_identifier" {
					fname := file.FlatNodeText(first)
					if fname == "TODO" || fname == "error" {
						isExit = true
					}
				}
			}
		}
	})
	return isExit
}

func (r *defaultResolver) indexFunctionParamsFlat(funcIdx uint32, file *scanner.File, scope *ScopeTable, it *ImportTable) bool {
	if file == nil || file.FlatTree == nil || funcIdx == 0 || scope == nil {
		return false
	}
	params := flatFindNamedChildOfType(file, funcIdx, "function_value_parameters")
	if params == 0 {
		return false
	}
	for i := 0; i < file.FlatNamedChildCount(params); i++ {
		param := file.FlatNamedChild(params, i)
		if param == 0 || file.FlatType(param) != "parameter" {
			continue
		}
		nameIdx := flatFirstDescendantOfType(file, param, "simple_identifier")
		if nameIdx == 0 {
			continue
		}
		name := file.FlatNodeText(nameIdx)
		if name == "" {
			continue
		}
		scope.Declare(name, r.resolveParamTypeFlat(param, file, it))
	}
	return true
}

func (r *defaultResolver) indexLambdaParamsFlat(paramsIdx uint32, file *scanner.File, scope *ScopeTable, it *ImportTable) bool {
	if file == nil || file.FlatTree == nil || paramsIdx == 0 || scope == nil {
		return false
	}
	handled := false
	for i := 0; i < file.FlatNamedChildCount(paramsIdx); i++ {
		param := file.FlatNamedChild(paramsIdx, i)
		if param == 0 {
			continue
		}
		switch file.FlatType(param) {
		case "variable_declaration", "simple_identifier":
			nameIdx := flatFirstDescendantOfType(file, param, "simple_identifier")
			var name string
			if nameIdx != 0 {
				name = file.FlatNodeText(nameIdx)
			}
			if name == "" {
				name = strings.TrimSpace(file.FlatNodeText(param))
			}
			if name == "" {
				continue
			}
			scope.Declare(name, r.resolveParamTypeFlat(param, file, it))
			handled = true
		}
	}
	return handled
}

func (r *defaultResolver) resolveParamTypeFlat(paramIdx uint32, file *scanner.File, it *ImportTable) *ResolvedType {
	if typeNode := flatFirstResolvableTypeChild(file, paramIdx); typeNode != 0 {
		return r.resolveTypeNodeFlat(typeNode, file, it)
	}
	return UnknownType()
}

func (r *defaultResolver) indexForVariableFlat(forIdx uint32, file *scanner.File, scope *ScopeTable) bool {
	if file == nil || file.FlatTree == nil || forIdx == 0 || scope == nil {
		return false
	}
	for i := 0; i < file.FlatChildCount(forIdx); i++ {
		child := file.FlatChild(forIdx, i)
		switch file.FlatType(child) {
		case "multi_variable_declaration":
			handled := false
			for j := 0; j < file.FlatChildCount(child); j++ {
				varDecl := file.FlatChild(child, j)
				if file.FlatType(varDecl) != "variable_declaration" {
					continue
				}
				name := flatFirstIdentifierText(file, varDecl)
				if name != "" && name != "_" {
					scope.Declare(name, UnknownType())
					handled = true
				}
			}
			return handled
		case "variable_declaration", "simple_identifier":
			name := flatFirstIdentifierText(file, child)
			if name == "" {
				name = strings.TrimSpace(file.FlatNodeText(child))
			}
			if name != "" {
				scope.Declare(name, UnknownType())
				return true
			}
		}
	}
	return false
}

func flatFirstIdentifierText(file *scanner.File, idx uint32) string {
	if file == nil || file.FlatTree == nil || idx == 0 {
		return ""
	}
	if file.FlatType(idx) == "simple_identifier" {
		return file.FlatNodeText(idx)
	}
	if ident := flatFirstDescendantOfType(file, idx, "simple_identifier"); ident != 0 {
		return file.FlatNodeText(ident)
	}
	return ""
}

func flatFirstTypeOrSimpleName(file *scanner.File, idx uint32) string {
	if file == nil || file.FlatTree == nil || idx == 0 {
		return ""
	}
	if ident := flatFirstDescendantOfType(file, idx, "type_identifier"); ident != 0 {
		return file.FlatNodeText(ident)
	}
	return flatFirstIdentifierText(file, idx)
}

func flatForEachRelevantScopeChild(file *scanner.File, idx uint32, fn func(child uint32)) {
	if file == nil || file.FlatTree == nil || idx == 0 || fn == nil {
		return
	}
	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		child := file.FlatNamedChild(idx, i)
		if child == 0 {
			continue
		}
		switch file.FlatType(child) {
		case "function_declaration", "lambda_literal", "class_declaration", "object_declaration", "class_body",
			"if_expression", "when_expression", "for_statement",
			"property_declaration", "call_expression", "elvis_expression",
			"control_structure_body", "statements", "function_body",
			"catch_block", "finally_block", "binary_expression",
			"conjunction_expression", "disjunction_expression",
			"call_suffix", "lambda_argument", "annotated_lambda":
			fn(child)
		}
	}
}

func (r *defaultResolver) indexDestructuringDeclarationFlat(propIdx, multiIdx uint32, file *scanner.File, scope *ScopeTable, it *ImportTable) {
	type varInfo struct {
		name    string
		typeIdx uint32
		index   int
	}
	var vars []varInfo
	varIndex := 0
	for i := 0; i < file.FlatChildCount(multiIdx); i++ {
		child := file.FlatChild(multiIdx, i)
		if file.FlatType(child) != "variable_declaration" {
			continue
		}
		name := flatFirstIdentifierText(file, child)
		if name == "" || name == "_" {
			varIndex++
			continue
		}
		vars = append(vars, varInfo{name: name, typeIdx: flatExplicitTypeNode(file, child), index: varIndex})
		varIndex++
	}

	componentTypes := r.inferDestructuringComponentTypesFlat(propIdx, file, it)
	for _, v := range vars {
		if v.typeIdx != 0 {
			scope.Declare(v.name, r.resolveTypeNodeFlat(v.typeIdx, file, it))
		} else if v.index < len(componentTypes) && componentTypes[v.index] != nil && componentTypes[v.index].Kind != TypeUnknown {
			scope.Declare(v.name, componentTypes[v.index])
		} else {
			scope.Declare(v.name, UnknownType())
		}
	}
}

func (r *defaultResolver) inferDestructuringComponentTypesFlat(propIdx uint32, file *scanner.File, it *ImportTable) []*ResolvedType {
	foundEq := false
	for i := 0; i < file.FlatChildCount(propIdx); i++ {
		child := file.FlatChild(propIdx, i)
		if child == 0 {
			continue
		}
		if file.FlatType(child) == "=" {
			foundEq = true
			continue
		}
		if !foundEq {
			continue
		}
		if file.FlatType(child) == "call_expression" {
			return r.inferPairTripleArgsFlat(child, file, it)
		}
		break
	}
	return nil
}

func (r *defaultResolver) inferImplicitItTypeFlat(lambdaIdx uint32, file *scanner.File, scope *ScopeTable, it *ImportTable) {
	if lambdaIdx == 0 {
		return
	}
	callIdx := uint32(0)
	for current, ok := file.FlatParent(lambdaIdx); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "call_expression":
			callIdx = current
			break
		case "call_suffix", "annotated_lambda", "lambda_argument":
			continue
		default:
			current = 0
		}
		if callIdx != 0 || current == 0 {
			break
		}
	}
	if callIdx == 0 {
		return
	}
	firstChild := file.FlatChild(callIdx, 0)
	if file.FlatType(firstChild) != "navigation_expression" {
		return
	}
	receiverIdx := flatNavigationReceiver(file, firstChild)
	if receiverIdx == 0 {
		return
	}

	var receiverType *ResolvedType
	if file.FlatType(receiverIdx) == "simple_identifier" {
		name := file.FlatNodeText(receiverIdx)
		if scope.Parent != nil {
			receiverType = scope.Parent.Lookup(name)
		}
		if receiverType == nil {
			receiverType = scope.Lookup(name)
		}
	}
	if receiverType == nil || receiverType.Kind == TypeUnknown {
		receiverType = r.ResolveFlatNode(receiverIdx, file)
	}
	if receiverType != nil && len(receiverType.TypeArgs) > 0 {
		scope.Declare("it", &ResolvedType{
			Name: receiverType.TypeArgs[0].Name,
			FQN:  receiverType.TypeArgs[0].FQN,
			Kind: receiverType.TypeArgs[0].Kind,
		})
	}
}

func (r *defaultResolver) inferPairTripleArgsFlat(callIdx uint32, file *scanner.File, it *ImportTable) []*ResolvedType {
	if callIdx == 0 {
		return nil
	}
	funcName := flatLastIdentifierText(file, file.FlatChild(callIdx, 0))
	if funcName != "Pair" && funcName != "Triple" {
		return nil
	}

	valueArgs := flatFindNamedChildOfType(file, callIdx, "value_arguments")
	if valueArgs == 0 {
		if callSuffix := flatFindNamedChildOfType(file, callIdx, "call_suffix"); callSuffix != 0 {
			valueArgs = flatFindNamedChildOfType(file, callSuffix, "value_arguments")
		}
	}
	if valueArgs == 0 {
		return nil
	}

	var types []*ResolvedType
	for i := 0; i < file.FlatChildCount(valueArgs); i++ {
		arg := file.FlatChild(valueArgs, i)
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		expr := uint32(0)
		for j := 0; j < file.FlatChildCount(arg); j++ {
			inner := file.FlatChild(arg, j)
			switch file.FlatType(inner) {
			case "(", ")", ",":
				continue
			default:
				expr = inner
			}
			if expr != 0 {
				break
			}
		}
		if expr == 0 {
			types = append(types, UnknownType())
			continue
		}
		types = append(types, r.inferExpressionTypeFlat(expr, file, it))
	}
	return types
}
