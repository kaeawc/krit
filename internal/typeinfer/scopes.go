package typeinfer

import (
	"strings"

	"github.com/kaeawc/krit/internal/analyzers/astflat"
	"github.com/kaeawc/krit/internal/scanner"
)

type flatScopeSpan struct {
	start uint32
	end   uint32
}

func (s flatScopeSpan) StartByte() uint32 { return s.start }
func (s flatScopeSpan) EndByte() uint32   { return s.end }

func (r *defaultResolver) buildScopesFlat(idx uint32, file *scanner.File, scope *ScopeTable, it *ImportTable) {
	if file == nil || file.FlatTree == nil || int(idx) >= file.FlatTree.Len() || scope == nil {
		return
	}
	if idx == 0 && file.FlatType(0) == "source_file" {
		r.buildSourceFileScopesFlat(file, scope, it)
		return
	}

	switch file.FlatType(idx) {
	case "function_declaration":
		r.buildFunctionDeclScopeFlat(idx, file, scope, it)
		return
	case "getter", "setter", "anonymous_initializer":
		r.buildExecutableScopeFlat(idx, file, scope, it)
		return
	case "lambda_literal":
		r.buildLambdaScopeFlat(idx, file, scope, it)
		return
	case "class_body":
		r.buildClassBodyScopeFlat(idx, file, scope, it)
		return
	case "if_expression":
		r.buildIfExpressionScopesFlat(idx, file, scope, it)
		return
	case "when_expression":
		r.buildWhenExpressionScopesFlat(idx, file, scope, it)
		return
	case "for_statement":
		r.buildForStatementScopeFlat(idx, file, scope, it)
		return
	case "property_declaration":
		r.declarePropertyFlat(idx, file, scope, it)
	}

	flatForEachRelevantScopeChild(file, idx, func(child uint32) {
		r.buildScopesFlat(child, file, scope, it)
	})

	// Smart casts are collected once for the whole tree. Running them after a top-level child
	// marks calls in later, not yet scoped declarations on the root scope, where they leak everywhere.
	if idx == 0 {
		r.buildFlatSmartCastScopes(file, scope)
	}
}

func (r *defaultResolver) buildExecutableScopeFlat(idx uint32, file *scanner.File, scope *ScopeTable, it *ImportTable) {
	childScope := scope.NewScopeForNode(flatScopeSpan{start: file.FlatStartByte(idx), end: file.FlatEndByte(idx)})
	if t := file.FlatType(idx); t == "getter" || t == "setter" {
		if prop := flatAccessorProperty(file, idx); prop != 0 {
			r.declareExtensionReceiverFlat(prop, file, childScope, it)
		}
	}
	flatForEachRelevantScopeChild(file, idx, func(child uint32) {
		r.buildScopesFlat(child, file, childScope, it)
	})
}

func (r *defaultResolver) buildSourceFileScopesFlat(file *scanner.File, scope *ScopeTable, it *ImportTable) {
	for i := 0; i < file.FlatNamedChildCount(0); i++ {
		child := file.FlatNamedChild(0, i)
		if child == 0 {
			continue
		}
		switch file.FlatType(child) {
		case "function_declaration", "lambda_literal", "class_declaration", "object_declaration", "class_body",
			"companion_object", "object_literal", "enum_class_body", "enum_entry",
			"getter",
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
}

func (r *defaultResolver) buildFunctionDeclScopeFlat(idx uint32, file *scanner.File, scope *ScopeTable, it *ImportTable) {
	span := flatScopeSpan{start: file.FlatStartByte(idx), end: file.FlatEndByte(idx)}
	// Context parameters live in an outer scope so a local of the same name
	// shadows them.
	if ctxParams := flatFindNamedChildOfType(file, idx, scanner.ContextParametersNodeType); ctxParams != 0 {
		scope = scope.NewScopeForNode(span)
		r.declareParamsFlat(ctxParams, file, scope, it)
	}
	childScope := scope.NewScopeForNode(span)
	r.declareExtensionReceiverFlat(idx, file, childScope, it)
	r.indexFunctionParamsFlat(idx, file, childScope, it)
	if bodyIdx := flatFindNamedChildOfType(file, idx, "function_body"); bodyIdx != 0 {
		flatForEachRelevantScopeChild(file, bodyIdx, func(child uint32) {
			r.buildScopesFlat(child, file, childScope, it)
		})
	}
}

// declareExtensionReceiverFlat declares `this` as the extension receiver of a function or property
// declaration, so inside `fun Config?.f()` it is a `Config?` rather than the enclosing class.
func (r *defaultResolver) declareExtensionReceiverFlat(decl uint32, file *scanner.File, scope *ScopeTable, it *ImportTable) {
	recv := flatExtensionReceiverTypeNode(file, decl)
	if recv == 0 {
		return
	}
	typ := r.resolveTypeNodeFlat(recv, file, it)
	if typ == nil {
		typ = UnknownType()
	}
	if file.FlatType(recv) == "nullable_type" {
		typ.Nullable = true
	}
	scope.Declare("this", typ)
	scope.Declare(implicitReceiverEntry, typ)
}

// flatAccessorProperty returns the property a getter or setter belongs to, or 0. An accessor on its own
// line parses as a sibling that follows the property_declaration rather than as its child.
func flatAccessorProperty(file *scanner.File, accessor uint32) uint32 {
	if parent, ok := file.FlatParent(accessor); ok && file.FlatType(parent) == "property_declaration" {
		return parent
	}
	for prev, ok := file.FlatPrevSibling(accessor); ok; prev, ok = file.FlatPrevSibling(prev) {
		switch file.FlatType(prev) {
		case "property_declaration":
			return prev
		case "getter", "setter":
			continue
		}
		if file.FlatIsNamed(prev) {
			return 0
		}
	}
	return 0
}

// flatExtensionReceiverTypeNode returns the receiver type node of an extension function or property, or 0.
// The grammar has no receiver node: the receiver is the type child directly followed by `.`, before the name.
func flatExtensionReceiverTypeNode(file *scanner.File, decl uint32) uint32 {
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier", "variable_declaration", "multi_variable_declaration", "function_value_parameters":
			return 0
		case "user_type", "nullable_type", "function_type", "parenthesized_type", "type_identifier":
			if next := file.FlatNextSib(child); next != 0 && file.FlatType(next) == "." {
				return child
			}
		}
	}
	return 0
}

func (r *defaultResolver) buildLambdaScopeFlat(idx uint32, file *scanner.File, scope *ScopeTable, it *ImportTable) {
	childScope := scope.NewScopeForNode(flatScopeSpan{start: file.FlatStartByte(idx), end: file.FlatEndByte(idx)})
	// Only `this` changes: unlike a declared extension, the lambda keeps resolving bare names outward,
	// because the receiver's members are rarely known and shadowing every outer name loses real findings.
	if recv := r.scopeFunctionReceiverFlat(idx, file, scope); recv != nil {
		childScope.Declare("this", recv)
	}
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
}

func (r *defaultResolver) buildClassBodyScopeFlat(idx uint32, file *scanner.File, scope *ScopeTable, it *ImportTable) {
	childScope := scope.NewScopeForNode(flatScopeSpan{start: file.FlatStartByte(idx), end: file.FlatEndByte(idx)})
	var ctorParams map[string]*plainCtorParam
	if parent, ok := file.FlatParent(idx); ok {
		if pt := file.FlatType(parent); pt == "class_declaration" || pt == "object_declaration" {
			className := flatFirstTypeOrSimpleName(file, parent)
			if className != "" {
				childScope.Declare("this", r.makeResolvedType(className, it, false))
			}
			r.indexPrimaryConstructorPropsFlat(parent, file, childScope, it)
			ctorParams = plainPrimaryConstructorParamsFlat(file, parent)
		}
	}
	if len(ctorParams) == 0 {
		flatForEachRelevantScopeChild(file, idx, func(child uint32) {
			r.buildScopesFlat(child, file, childScope, it)
		})
		return
	}
	// Plain primary-constructor parameters are in scope in property
	// initializers, property delegates and init blocks, where they shadow
	// same-named members (#1673); accessors and member functions see only the
	// members. val/var parameters are already class-scope members with the
	// same type. Each initializer gets a scope only for the parameters it
	// mentions, so classes whose initializers read none pay nothing.
	initScope := func(span uint32) *ScopeTable {
		var s *ScopeTable
		file.FlatWalkNodes(span, "simple_identifier", func(id uint32) {
			param, ok := ctorParams[string(file.FlatNodeBytes(id))]
			if !ok {
				return
			}
			if s == nil {
				s = childScope.NewScopeForNode(flatScopeSpan{start: file.FlatStartByte(span), end: file.FlatEndByte(span)})
			} else if _, declared := s.Entries[param.name]; declared {
				return
			}
			if param.typ == nil {
				param.typ = r.resolveParamTypeFlat(param.node, file, it)
			}
			typ := *param.typ
			s.Declare(param.name, &typ)
		})
		if s == nil {
			return childScope
		}
		return s
	}
	flatForEachRelevantScopeChild(file, idx, func(child uint32) {
		switch file.FlatType(child) {
		case "anonymous_initializer":
			r.buildExecutableScopeFlat(child, file, initScope(child), it)
		case "property_declaration":
			r.declarePropertyFlat(child, file, childScope, it)
			init := flatPropertyInitializerOrDelegate(file, child)
			flatForEachRelevantScopeChild(file, child, func(part uint32) {
				if part != init {
					r.buildScopesFlat(part, file, childScope, it)
				}
			})
			if init != 0 {
				r.buildScopesFlat(init, file, initScope(init), it)
			}
		default:
			r.buildScopesFlat(child, file, childScope, it)
		}
	})
}

// plainCtorParam is a primary-constructor parameter without a val/var
// binding; typ is resolved on first use.
type plainCtorParam struct {
	name string
	node uint32
	typ  *ResolvedType
}

// plainPrimaryConstructorParamsFlat returns a class's primary-constructor
// parameters that have no val/var binding, by name.
func plainPrimaryConstructorParamsFlat(file *scanner.File, classDecl uint32) map[string]*plainCtorParam {
	pc, ok := file.FlatFindChild(classDecl, "primary_constructor")
	if !ok {
		return nil
	}
	var params map[string]*plainCtorParam
	for i := 0; i < file.FlatNamedChildCount(pc); i++ {
		param := file.FlatNamedChild(pc, i)
		if param == 0 || file.FlatType(param) != "class_parameter" {
			continue
		}
		if _, binding := file.FlatFindChild(param, "binding_pattern_kind"); binding {
			continue
		}
		name := flatFirstIdentifierText(file, param)
		if name == "" {
			continue
		}
		if params == nil {
			params = make(map[string]*plainCtorParam)
		}
		params[name] = &plainCtorParam{name: name, node: param}
	}
	return params
}

// flatPropertyInitializerOrDelegate returns the initializer expression after
// `=`, or the property_delegate node, of a property declaration; 0 if it has
// neither.
func flatPropertyInitializerOrDelegate(file *scanner.File, prop uint32) uint32 {
	seenEquals := false
	for child := file.FlatFirstChild(prop); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			if file.FlatNodeTextEquals(child, "=") {
				seenEquals = true
			}
			continue
		}
		if file.FlatType(child) == "property_delegate" || seenEquals {
			return child
		}
	}
	return 0
}

// indexPrimaryConstructorPropsFlat declares each `val`/`var` class_parameter
// from the parent class's primary_constructor as an entry in the class body
// scope. Plain constructor parameters (no val/var binding modifier) are
// constructor-local and intentionally skipped.
func (r *defaultResolver) indexPrimaryConstructorPropsFlat(classDecl uint32, file *scanner.File, scope *ScopeTable, it *ImportTable) {
	pc, ok := file.FlatFindChild(classDecl, "primary_constructor")
	if !ok {
		return
	}
	for i := 0; i < file.FlatNamedChildCount(pc); i++ {
		param := file.FlatNamedChild(pc, i)
		if param == 0 || file.FlatType(param) != "class_parameter" {
			continue
		}
		hasBinding := false
		var name string
		for j := 0; j < file.FlatNamedChildCount(param); j++ {
			part := file.FlatNamedChild(param, j)
			switch file.FlatType(part) {
			case "binding_pattern_kind":
				hasBinding = true
			case "simple_identifier":
				if name == "" {
					name = file.FlatNodeText(part)
				}
			}
		}
		if !hasBinding || name == "" {
			continue
		}
		if _, exists := scope.Entries[name]; exists {
			continue
		}
		scope.Declare(name, r.resolveParamTypeFlat(param, file, it))
	}
}

func (r *defaultResolver) buildForStatementScopeFlat(idx uint32, file *scanner.File, scope *ScopeTable, it *ImportTable) {
	childScope := scope.NewScopeForNode(flatScopeSpan{start: file.FlatStartByte(idx), end: file.FlatEndByte(idx)})
	r.indexForVariableFlat(idx, file, childScope)
	flatForEachRelevantScopeChild(file, idx, func(child uint32) {
		r.buildScopesFlat(child, file, childScope, it)
	})
}

func (r *defaultResolver) declarePropertyFlat(idx uint32, file *scanner.File, scope *ScopeTable, it *ImportTable) {
	if multiVar := flatFindNamedChildOfType(file, idx, "multi_variable_declaration"); multiVar != 0 {
		r.indexDestructuringDeclarationFlat(idx, multiVar, file, scope, it)
		return
	}
	name := ""
	if varDecl := flatFindNamedChildOfType(file, idx, "variable_declaration"); varDecl != 0 {
		name = flatFirstIdentifierText(file, varDecl)
	}
	if name == "" {
		name = flatFirstIdentifierText(file, idx)
	}
	if _, exists := scope.Entries[name]; name != "" && !exists {
		typ := r.resolvePropertyTypeFlat(idx, file, it)
		if flatPropertyDeclarationIsLocal(file, idx) {
			scope.DeclareAt(name, typ, file.FlatEndByte(idx))
		} else {
			scope.Declare(name, typ)
		}
	}
}

func flatPropertyDeclarationIsLocal(file *scanner.File, idx uint32) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "function_body", "lambda_literal", "getter", "setter", "control_structure_body",
			"catch_block", "finally_block", "anonymous_initializer":
			return true
		case "class_body", "source_file":
			return false
		}
	}
	return false
}

func (r *defaultResolver) buildIfExpressionScopesFlat(idx uint32, file *scanner.File, scope *ScopeTable, it *ImportTable) {
	thenBody, elseBody := flatIfBranchBodies(file, idx)

	nullVars, isChecks := r.extractConjunctionChecksFlat(idx, file, it)
	if len(nullVars) > 0 || len(isChecks) > 0 {
		// The conjunction holds only in the then-branch; the else-branch learns nothing about any one operand.
		r.buildIfBranchScopesFlat(idx, thenBody, file, scope, it, func(branch *ScopeTable) {
			for _, v := range nullVars {
				branch.SetSmartCast(v)
			}
			for varName, typ := range isChecks {
				branch.SetSmartCastType(varName, typ)
			}
		})
		return
	}

	varName, isNotNullCheck, isNullCheck, _ := r.extractNullCheckFromIfFlatByIdx(idx, file)
	if varName != "" && isNotNullCheck {
		r.buildIfBranchScopesFlat(idx, thenBody, file, scope, it, func(branch *ScopeTable) {
			branch.SetSmartCast(varName)
		})
		return
	}

	if varName != "" && isNullCheck && r.ifBodyIsEarlyReturnFlat(idx, file) {
		scope.MarkSmartCastAfter(varName, file.FlatEndByte(idx))
	}

	isVarName, targetType, isPositive, _ := r.extractIsCheckFromIfFlatByIdx(idx, file, it)
	if isVarName != "" && targetType != nil && isPositive {
		// `x is T` holds in the then-branch only: in the else-branch x is "not T".
		r.buildIfBranchScopesFlat(idx, thenBody, file, scope, it, func(branch *ScopeTable) {
			branch.SetSmartCastType(isVarName, targetType)
		})
		return
	}

	// `x !is T`, alone or as an operand of `||`, is false whenever the condition is false: after an early
	// exit and in the else-branch, x is a T.
	if negated, fromPrefix := r.extractNegatedIsChecksFromIfFlat(idx, file, it); len(negated) > 0 {
		if r.ifBodyIsEarlyReturnFlat(idx, file) {
			// A `!(x is T)` guard only narrows after a then-body that always exits, not one that merely
			// holds a return somewhere inside it.
			alwaysExits := flatBodyAlwaysExits(file, thenBody)
			for name, typ := range negated {
				if fromPrefix[name] && !alwaysExits {
					continue
				}
				// The guard narrows x only after the if-statement, not for earlier uses in the scope.
				scope.SetSmartCastTypeAfter(name, typ, file.FlatEndByte(idx))
			}
		}
		if elseBody != 0 {
			r.buildIfBranchScopesFlat(idx, elseBody, file, scope, it, func(branch *ScopeTable) {
				for name, typ := range negated {
					branch.SetSmartCastType(name, typ)
				}
			})
			return
		}
	}

	// No narrowing, but each branch body still gets its own scope so its locals and facts stay inside it.
	r.buildIfBranchScopesFlat(idx, 0, file, scope, it, nil)
}

// flatBodyAlwaysExits reports whether a branch body ends in return/throw on every path through its own
// statements: a single exit expression, or a block whose last statement is one. Calls such as TODO() or
// error() are not accepted by name, because a local function with that name can return normally.
func flatBodyAlwaysExits(file *scanner.File, body uint32) bool {
	if file == nil || body == 0 {
		return false
	}
	last := body
	for {
		switch file.FlatType(last) {
		case "control_structure_body", "statements", "block", "parenthesized_expression":
			next := flatLastNamedChildFlat(file, last)
			if next == 0 {
				return false
			}
			last = next
		case "jump_expression":
			text := file.FlatNodeText(last)
			return strings.HasPrefix(text, "return") || strings.HasPrefix(text, "throw")
		default:
			return false
		}
	}
}

// buildIfBranchScopesFlat builds the scopes of an if-expression's children. Every branch body gets its own
// child scope, so its locals and post-statement facts stay inside it; only narrowed (one branch body) also
// gets the facts narrow records. Other children are built in scope.
func (r *defaultResolver) buildIfBranchScopesFlat(ifIdx, narrowed uint32, file *scanner.File, scope *ScopeTable, it *ImportTable, narrow func(branch *ScopeTable)) {
	flatForEachRelevantScopeChild(file, ifIdx, func(child uint32) {
		if file.FlatType(child) != "control_structure_body" {
			r.buildScopesFlat(child, file, scope, it)
			return
		}
		branch := scope.NewScopeForNode(flatScopeSpan{start: file.FlatStartByte(child), end: file.FlatEndByte(child)})
		if child == narrowed && narrow != nil {
			narrow(branch)
		}
		flatForEachRelevantScopeChild(file, child, func(gc uint32) {
			r.buildScopesFlat(gc, file, branch, it)
		})
	})
}

// flatUnwrapParenthesizedExpression strips redundant parentheses around an expression.
func flatUnwrapParenthesizedExpression(file *scanner.File, idx uint32) uint32 {
	for idx != 0 && file.FlatType(idx) == "parenthesized_expression" {
		inner := uint32(0)
		for i := 0; i < file.FlatNamedChildCount(idx); i++ {
			if c := file.FlatNamedChild(idx, i); c != 0 {
				inner = c
				break
			}
		}
		idx = inner
	}
	return idx
}

// flatIfBranchBodies returns the then-body and the else-body (0 when absent) of an if-expression.
func flatIfBranchBodies(file *scanner.File, ifIdx uint32) (thenBody, elseBody uint32) {
	if file == nil || file.FlatTree == nil || ifIdx == 0 {
		return 0, 0
	}
	afterElse := false
	for child := file.FlatFirstChild(ifIdx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "else":
			afterElse = true
		case "control_structure_body":
			if afterElse {
				if elseBody == 0 {
					elseBody = child
				}
			} else if thenBody == 0 {
				thenBody = child
			}
		}
	}
	return thenBody, elseBody
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
					childScope.SetSmartCastType(subjectName, targetType)
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
	cond := uint32(0)
	for i := 0; i < file.FlatNamedChildCount(entryIdx); i++ {
		c := file.FlatNamedChild(entryIdx, i)
		if c == 0 || file.FlatType(c) != "when_condition" {
			continue
		}
		if cond != 0 {
			// `is A, is B ->` proves only "A or B", so no single type narrows the entry.
			return nil
		}
		cond = c
	}
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

	// `x != null` narrows the then-branch only when it is the whole condition: under `||`, `!` or a call
	// argument it proves nothing. Each `x == null` operand of a top-level `||` is false when the condition
	// is, so it still narrows x after an early exit.
	condNode := astflat.UnwrapParenExpr(file, condRoot)
	if file.FlatType(condNode) == "disjunction_expression" {
		return flatDisjunctionEqualsNullOperand(file, condNode), false, true, true
	}
	return flatNullCheckOperand(file, condNode)
}

// flatDisjunctionEqualsNullOperand returns the first `x == null` operand of a `||` chain, or "".
func flatDisjunctionEqualsNullOperand(file *scanner.File, disj uint32) string {
	for i := 0; i < file.FlatNamedChildCount(disj); i++ {
		operand := astflat.UnwrapParenExpr(file, file.FlatNamedChild(disj, i))
		if file.FlatType(operand) == "disjunction_expression" {
			if name := flatDisjunctionEqualsNullOperand(file, operand); name != "" {
				return name
			}
			continue
		}
		if name, _, isNull, ok := flatNullCheckOperand(file, operand); ok && isNull {
			return name
		}
	}
	return ""
}

// flatNullCheckOperand reads `x != null` / `x == null` (either operand order) from an equality_expression.
func flatNullCheckOperand(file *scanner.File, condNode uint32) (string, bool, bool, bool) {
	if condNode == 0 || file.FlatType(condNode) != "equality_expression" || file.FlatChildCount(condNode) < 3 {
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

	// Only a check that is the whole condition narrows a branch: under `||`, `!` or a call argument it does not hold.
	return r.checkExpressionFactFlat(flatUnwrapParenthesizedExpression(file, condRoot), file, it)
}

// extractNegatedIsChecksFromIfFlat returns the `x !is T` checks that all hold when the if-condition is false:
// the condition itself, or any operand of a top-level `||` chain (nested and parenthesized `||` included).
// fromPrefix marks the names whose check came from a `!(x is T)` operand.
func (r *defaultResolver) extractNegatedIsChecksFromIfFlat(ifIdx uint32, file *scanner.File, it *ImportTable) (checks map[string]*ResolvedType, fromPrefix map[string]bool) {
	condRoot := flatIfConditionNode(file, ifIdx)
	if condRoot == 0 {
		return nil, nil
	}
	checks = make(map[string]*ResolvedType)
	fromPrefix = make(map[string]bool)
	var collect func(node uint32)
	collect = func(node uint32) {
		node = flatUnwrapParenthesizedExpression(file, node)
		switch file.FlatType(node) {
		case "disjunction_expression":
			for i := 0; i < file.FlatChildCount(node); i++ {
				if child := file.FlatChild(node, i); child != 0 && file.FlatIsNamed(child) {
					collect(child)
				}
			}
		case "check_expression":
			if name, typ, positive, ok := r.checkExpressionFactFlat(node, file, it); ok && !positive {
				checks[name] = typ
			}
		case "prefix_expression":
			// `!(x is T)` is false exactly when `x is T` holds, like `x !is T`.
			if !astflat.PrefixExpressionIsNot(file, node) {
				return
			}
			inner := flatUnwrapParenthesizedExpression(file, flatLastNamedChildFlat(file, node))
			if name, typ, positive, ok := r.checkExpressionFactFlat(inner, file, it); ok && positive {
				checks[name] = typ
				fromPrefix[name] = true
			}
		}
	}
	collect(condRoot)
	return checks, fromPrefix
}

// flatLastNamedChildFlat returns idx's last named child, or 0.
func flatLastNamedChildFlat(file *scanner.File, idx uint32) uint32 {
	n := file.FlatNamedChildCount(idx)
	if n == 0 {
		return 0
	}
	return file.FlatNamedChild(idx, n-1)
}

// checkExpressionFactFlat reads `x is T` / `x !is T` from a check_expression node.
func (r *defaultResolver) checkExpressionFactFlat(isNode uint32, file *scanner.File, it *ImportTable) (string, *ResolvedType, bool, bool) {
	if isNode == 0 || file.FlatType(isNode) != "check_expression" || file.FlatChildCount(isNode) < 3 {
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

	// Only a conjunction that is the whole condition guarantees each operand in the then-branch.
	conjIdx := flatUnwrapParenthesizedExpression(file, condRoot)
	if conjIdx == 0 || file.FlatType(conjIdx) != "conjunction_expression" {
		return nil, nil
	}

	var nullVars []string
	isChecks := make(map[string]*ResolvedType)
	r.collectConjunctionChecksFlat(conjIdx, file, it, &nullVars, isChecks)
	return nullVars, isChecks
}

// collectConjunctionChecksFlat records the `!= null` and `is` operands of a conjunction. `&&` parses
// left-nested (`a && b && c` is `(a && b) && c`), so nested and parenthesized conjunctions are walked too.
func (r *defaultResolver) collectConjunctionChecksFlat(conjIdx uint32, file *scanner.File, it *ImportTable, nullVars *[]string, isChecks map[string]*ResolvedType) {
	for i := 0; i < file.FlatChildCount(conjIdx); i++ {
		child := flatUnwrapParenthesizedExpression(file, file.FlatChild(conjIdx, i))
		if child == 0 {
			continue
		}
		switch file.FlatType(child) {
		case "conjunction_expression":
			r.collectConjunctionChecksFlat(child, file, it, nullVars, isChecks)
		case "equality_expression":
			if v := r.extractConjunctionNullCheckFlat(child, file); v != "" {
				*nullVars = append(*nullVars, v)
			}
		case "check_expression":
			r.extractConjunctionIsCheckFlat(child, file, it, isChecks)
		}
	}
}

func (r *defaultResolver) extractConjunctionNullCheckFlat(child uint32, file *scanner.File) string {
	if file.FlatChildCount(child) < 3 {
		return ""
	}
	left := file.FlatChild(child, 0)
	op := file.FlatChild(child, 1)
	right := file.FlatChild(child, 2)
	if left == 0 || op == 0 || right == 0 {
		return ""
	}
	if file.FlatNodeText(op) != "!=" {
		return ""
	}
	leftText := file.FlatNodeText(left)
	rightText := file.FlatNodeText(right)
	if rightText == "null" && file.FlatType(left) == "simple_identifier" {
		return leftText
	}
	if leftText == "null" && file.FlatType(right) == "simple_identifier" {
		return rightText
	}
	return ""
}

func (r *defaultResolver) extractConjunctionIsCheckFlat(child uint32, file *scanner.File, it *ImportTable, isChecks map[string]*ResolvedType) {
	if file.FlatChildCount(child) < 3 {
		return
	}
	left := file.FlatChild(child, 0)
	op := file.FlatChild(child, 1)
	right := file.FlatChild(child, 2)
	if left == 0 || op == 0 || right == 0 {
		return
	}
	if file.FlatType(left) != "simple_identifier" || file.FlatType(op) != "is" {
		return
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
					callScope.MarkSmartCastAfter(varName, file.FlatEndByte(idx))
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
	exprScope.MarkSmartCastAfter(varName, file.FlatEndByte(idx))
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
	r.declareParamsFlat(params, file, scope, it)
	return true
}

func (r *defaultResolver) declareParamsFlat(params uint32, file *scanner.File, scope *ScopeTable, it *ImportTable) {
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
		case "multi_variable_declaration":
			// `{ (items, count) -> }`: each component shadows outer names; its type is not tracked.
			for j := 0; j < file.FlatNamedChildCount(param); j++ {
				component := file.FlatNamedChild(param, j)
				if file.FlatType(component) != "variable_declaration" {
					continue
				}
				if name := flatFirstIdentifierText(file, component); name != "" && name != "_" {
					scope.Declare(name, r.resolveParamTypeFlat(component, file, it))
					handled = true
				}
			}
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
			"companion_object", "object_literal", "enum_class_body", "enum_entry",
			"getter", "setter", "anonymous_initializer",
			"if_expression", "when_expression", "for_statement",
			"property_declaration", "call_expression", "elvis_expression",
			"control_structure_body", "statements", "function_body",
			"catch_block", "finally_block", "binary_expression",
			"conjunction_expression", "disjunction_expression",
			"call_suffix", "lambda_argument", "annotated_lambda",
			// Lambdas also sit in returned values, call chains, parenthesized arguments and assignments.
			"jump_expression", "navigation_expression", "value_arguments", "value_argument",
			"parenthesized_expression", "assignment", "infix_expression":
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

func (r *defaultResolver) inferImplicitItTypeFlat(lambdaIdx uint32, file *scanner.File, scope *ScopeTable, _ *ImportTable) {
	if lambdaIdx == 0 {
		return
	}
	callIdx := uint32(0)
	for current, ok := file.FlatParent(lambdaIdx); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "call_expression":
			callIdx = current
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
			Name:     receiverType.TypeArgs[0].Name,
			FQN:      receiverType.TypeArgs[0].FQN,
			Kind:     receiverType.TypeArgs[0].Kind,
			Nullable: receiverType.TypeArgs[0].Nullable,
			Resolved: receiverType.TypeArgs[0].Resolved,
		})
	}
}

// flatLambdaCall returns the call a trailing lambda belongs to (`x.run { }`, `with(x) { }`), or 0.
func flatLambdaCall(file *scanner.File, lambda uint32) uint32 {
	for current, ok := file.FlatParent(lambda); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "annotated_lambda", "call_suffix":
			continue
		case "call_expression":
			return current
		}
		return 0
	}
	return 0
}

// scopeFunctionReceiverFlat returns the type `this` has inside a `with(x) { }`, `x.run { }` or
// `x.apply { }` lambda: x's type, non-null after `x?.run`, and unknown when x cannot be resolved yet. It
// returns nil for any other lambda.
func (r *defaultResolver) scopeFunctionReceiverFlat(lambda uint32, file *scanner.File, scope *ScopeTable) *ResolvedType {
	call := flatLambdaCall(file, lambda)
	if call == 0 {
		return nil
	}
	callee := file.FlatChild(call, 0)
	var recv uint32
	safeCall := false
	switch file.FlatType(callee) {
	case "call_expression":
		// `with(x) { }` parses as a call of the call `with(x)`.
		if name := file.FlatChild(callee, 0); file.FlatType(name) != "simple_identifier" || file.FlatNodeText(name) != "with" {
			return nil
		}
		suffix := flatFindNamedChildOfType(file, callee, "call_suffix")
		args := flatFindNamedChildOfType(file, suffix, "value_arguments")
		if suffix == 0 || args == 0 || file.FlatNamedChildCount(args) != 1 {
			return UnknownType()
		}
		arg := file.FlatNamedChild(args, 0)
		if file.FlatNamedChildCount(arg) != 1 {
			return UnknownType()
		}
		recv = file.FlatNamedChild(arg, 0)
	case "navigation_expression":
		switch flatLastIdentifierText(file, callee) {
		case "run", "apply":
		default:
			return nil
		}
		recv = flatNavigationReceiver(file, callee)
		suffix := astflat.LastNamedChild(file, callee)
		safeCall = file.FlatType(suffix) == "navigation_suffix" && file.FlatHasChildOfType(suffix, "?.")
	default:
		return nil
	}
	typ := r.resolveDuringScopeBuildFlat(recv, file, scope)
	if typ == nil || typ.Kind == TypeUnknown {
		return UnknownType()
	}
	result := *typ
	if safeCall && result.Nullable {
		result.Nullable = false
		if result.Kind == TypeNullable {
			result.Kind = TypeClass
		}
	}
	return &result
}

// resolveDuringScopeBuildFlat resolves expr while its file's scopes are still being built: a bare name
// is looked up in the partial scope chain, anything else through the resolver.
func (r *defaultResolver) resolveDuringScopeBuildFlat(expr uint32, file *scanner.File, scope *ScopeTable) *ResolvedType {
	if expr == 0 {
		return nil
	}
	if file.FlatType(expr) == "simple_identifier" {
		if typ := scope.LookupAt(file.FlatNodeText(expr), file.FlatStartByte(expr)); typ != nil {
			return typ
		}
	}
	return r.ResolveFlatNode(expr, file)
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
