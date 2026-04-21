package typeinfer

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

func (r *defaultResolver) indexDeclarationsFlat(idx uint32, file *scanner.File, scope *ScopeTable, it *ImportTable, pkg string) {
	if file == nil || file.FlatTree == nil || int(idx) >= len(file.FlatTree.Nodes) || scope == nil || it == nil {
		return
	}

	switch file.FlatType(idx) {
	case "property_declaration":
		r.indexPropertyFlat(idx, file, scope, it)
	case "function_declaration":
		r.indexFunctionFlat(idx, file, it, pkg)
	case "class_declaration":
		r.indexClassFlat(idx, file, it, pkg)
	case "object_declaration":
		r.indexObjectFlat(idx, file, it, pkg)
	}

	switch file.FlatType(idx) {
	case "source_file", "class_body", "class_member_declarations", "statements",
		"function_declaration", "function_body", "lambda_literal", "control_structure_body",
		"catch_block", "finally_block", "primary_constructor",
		"secondary_constructor", "anonymous_initializer", "object_declaration",
		"class_declaration":
		if idx == 0 {
			for i := 0; i < file.FlatNamedChildCount(0); i++ {
				child := file.FlatNamedChild(0, i)
				if child == 0 {
					continue
				}
				switch file.FlatType(child) {
				case "property_declaration", "function_declaration", "class_declaration",
					"object_declaration", "class_body", "class_member_declarations",
					"statements", "function_body", "lambda_literal", "control_structure_body",
					"catch_block", "finally_block", "primary_constructor", "secondary_constructor",
					"anonymous_initializer":
					r.indexDeclarationsFlat(child, file, scope, it, pkg)
				}
			}
			return
		}
		flatForEachRelevantDeclarationChild(file, idx, func(child uint32) {
			r.indexDeclarationsFlat(child, file, scope, it, pkg)
		})
	}
}

func (r *defaultResolver) indexPropertyFlat(propIdx uint32, file *scanner.File, scope *ScopeTable, it *ImportTable) {
	if file == nil || propIdx == 0 || scope == nil || it == nil {
		return
	}

	if multiVar := flatFindNamedChildOfType(file, propIdx, "multi_variable_declaration"); multiVar != 0 {
		r.indexDestructuringDeclarationFlat(propIdx, multiVar, file, scope, it)
		return
	}

	if varDecl, ok := file.FlatFindChild(propIdx, "variable_declaration"); ok {
		if varName := flatMemberName(file, varDecl); varName != "" {
			scope.Declare(varName, r.resolvePropertyTypeFlat(propIdx, file, it))
			return
		}
	}

	if varDecl := flatFindNamedChildOfType(file, propIdx, "variable_declaration"); varDecl != 0 {
		if varName, typeIdx := flatVariableDeclNameAndType(file, varDecl); varName != "" {
			if typeIdx != 0 {
				scope.Declare(varName, r.resolveTypeNodeFlat(typeIdx, file, it))
				return
			}
			scope.Declare(varName, r.resolvePropertyTypeFlat(propIdx, file, it))
		}
	}
}

func (r *defaultResolver) inferDelegateTypeFlat(delegateIdx uint32, file *scanner.File, it *ImportTable) *ResolvedType {
	var callIdx uint32
	file.FlatWalkAllNodes(delegateIdx, func(idx uint32) {
		if callIdx == 0 && file.FlatType(idx) == "call_expression" {
			callIdx = idx
		}
	})
	if callIdx == 0 {
		return UnknownType()
	}
	return r.inferDelegateCallFlat(callIdx, file, it)
}

func (r *defaultResolver) inferDelegateCallFlat(callIdx uint32, file *scanner.File, it *ImportTable) *ResolvedType {
	funcName := flatLastIdentifierText(file, file.FlatChild(callIdx, 0))

	if funcName == "lazy" || funcName == "remember" {
		var lambdaIdx uint32
		file.FlatWalkAllNodes(callIdx, func(idx uint32) {
			if lambdaIdx == 0 && file.FlatType(idx) == "lambda_literal" {
				lambdaIdx = idx
			}
		})
		if lambdaIdx != 0 {
			return r.inferLambdaLastExpressionFlat(lambdaIdx, file, it)
		}
	}

	if funcName == "mutableStateOf" || funcName == "stateOf" {
		var valueArgs uint32
		file.FlatWalkAllNodes(callIdx, func(idx uint32) {
			if valueArgs == 0 && file.FlatType(idx) == "value_arguments" {
				valueArgs = idx
			}
		})
		if valueArgs != 0 {
			for i := 0; i < file.FlatNamedChildCount(valueArgs); i++ {
				arg := file.FlatNamedChild(valueArgs, i)
				if file.FlatType(arg) != "value_argument" {
					continue
				}
				for j := 0; j < file.FlatChildCount(arg); j++ {
					inner := file.FlatChild(arg, j)
					switch file.FlatType(inner) {
					case "(", ")", ",":
						continue
					default:
						return r.inferExpressionTypeFlat(inner, file, it)
					}
				}
			}
		}
	}

	return UnknownType()
}

func (r *defaultResolver) inferLambdaLastExpressionFlat(lambdaIdx uint32, file *scanner.File, it *ImportTable) *ResolvedType {
	stmts := flatFindNamedChildOfType(file, lambdaIdx, "statements")
	if stmts == 0 {
		return UnknownType()
	}

	var lastExpr uint32
	var priorDecls []uint32
	for i := 0; i < file.FlatChildCount(stmts); i++ {
		child := file.FlatChild(stmts, i)
		switch file.FlatType(child) {
		case "property_declaration":
			priorDecls = append(priorDecls, child)
		case "call_expression", "simple_identifier", "string_literal",
			"line_string_literal", "multi_line_string_literal",
			"integer_literal", "long_literal", "real_literal",
			"boolean_literal", "character_literal", "null_literal",
			"navigation_expression", "if_expression", "when_expression",
			"parenthesized_expression", "as_expression",
			"additive_expression", "multiplicative_expression",
			"comparison_expression", "equality_expression",
			"prefix_expression", "range_expression":
			lastExpr = child
		}
	}
	if lastExpr == 0 {
		return UnknownType()
	}

	restore := r.installLambdaPreludeScope(file, func(scope *ScopeTable) {
		for _, decl := range priorDecls {
			if file.FlatStartByte(decl) >= file.FlatStartByte(lastExpr) {
				break
			}
			if name := flatMemberName(file, decl); name != "" {
				scope.Declare(name, r.resolvePropertyTypeFlat(decl, file, it))
			}
		}
	})
	if restore != nil {
		defer restore()
	}

	return r.inferExpressionTypeFlat(lastExpr, file, it)
}

func (r *defaultResolver) installLambdaPreludeScope(file *scanner.File, populate func(scope *ScopeTable)) func() {
	if file == nil || populate == nil {
		return nil
	}
	prevScope := r.scopes[file.Path]
	tempScope := &ScopeTable{
		Parent:         prevScope,
		Entries:        make(map[string]*ResolvedType),
		SmartCasts:     make(map[string]bool),
		SmartCastTypes: make(map[string]*ResolvedType),
	}
	populate(tempScope)
	if len(tempScope.Entries) == 0 {
		return nil
	}
	r.scopes[file.Path] = tempScope
	return func() {
		if prevScope != nil {
			r.scopes[file.Path] = prevScope
		} else {
			delete(r.scopes, file.Path)
		}
	}
}

func (r *defaultResolver) indexClassFlat(flatIdx uint32, file *scanner.File, it *ImportTable, pkg string) {
	if file == nil || flatIdx == 0 {
		return
	}

	name := flatDeclarationName(file, flatIdx)
	supertypes := flatDeclarationSupertypes(file, flatIdx)
	mods := flatReadModifierFlags(file, flatIdx)
	isInterface := strings.Contains(trimmedNodePrefix(file.FlatNodeText(flatIdx), 256), "interface ")
	kind := r.classKind(mods.enum, mods.sealed, isInterface)

	if name == "" {
		return
	}

	fqn := name
	if pkg != "" {
		fqn = pkg + "." + name
	}

	info := &ClassInfo{
		Name:       name,
		FQN:        fqn,
		Kind:       kind,
		Supertypes: supertypes,
		IsSealed:   mods.sealed,
		IsData:     mods.data,
		IsInner:    mods.inner,
		IsAbstract: mods.abstract,
		IsOpen:     mods.open,
		File:       file.Path,
		Line:       declarationLine(file, flatIdx),
	}

	if bodyIdx := flatFindNamedChildOfType(file, flatIdx, "class_body"); bodyIdx != 0 {
		info.Members = r.extractMembersFlat(bodyIdx, file, it)
		for i := 0; i < file.FlatNamedChildCount(bodyIdx); i++ {
			child := file.FlatNamedChild(bodyIdx, i)
			if child == 0 || file.FlatType(child) != "companion_object" {
				continue
			}
			companionBody := flatFindNamedChildOfType(file, child, "class_body")
			if companionBody == 0 {
				continue
			}
			companionMembers := r.extractMembersFlat(companionBody, file, it)
			for _, m := range companionMembers {
				if m.Type != nil && m.Type.Kind != TypeUnknown {
					r.functions[name+"."+m.Name] = m.Type
				}
			}
		}
	}

	r.classes[name] = info
	r.classFQN[fqn] = info

	if mods.enum {
		entries := flatExtractEnumEntries(file, flatIdx)
		if len(entries) > 0 {
			r.enumEntries[name] = entries
			r.enumEntries[fqn] = entries
		}
	}

	if !mods.sealed {
		for _, st := range supertypes {
			r.sealedVariants[st] = append(r.sealedVariants[st], name)
			parts := strings.Split(st, ".")
			if len(parts) > 1 {
				r.sealedVariants[parts[len(parts)-1]] = append(r.sealedVariants[parts[len(parts)-1]], name)
			}
		}
	}
}

func (r *defaultResolver) indexObjectFlat(flatIdx uint32, file *scanner.File, it *ImportTable, pkg string) {
	if file == nil || flatIdx == 0 {
		return
	}

	name := flatDeclarationName(file, flatIdx)
	if name == "" {
		return
	}

	fqn := name
	if pkg != "" {
		fqn = pkg + "." + name
	}

	supertypes := flatDeclarationSupertypes(file, flatIdx)
	mods := flatReadModifierFlags(file, flatIdx)

	info := &ClassInfo{
		Name:       name,
		FQN:        fqn,
		Kind:       "object",
		Supertypes: supertypes,
		IsSealed:   mods.sealed,
		IsData:     mods.data,
		IsInner:    mods.inner,
		IsAbstract: mods.abstract,
		IsOpen:     mods.open,
		File:       file.Path,
		Line:       declarationLine(file, flatIdx),
	}

	if bodyIdx := flatFindNamedChildOfType(file, flatIdx, "class_body"); bodyIdx != 0 {
		info.Members = r.extractMembersFlat(bodyIdx, file, it)
	}

	r.classes[name] = info
	r.classFQN[fqn] = info
}

func (r *defaultResolver) classKind(isEnum, isSealed, isInterface bool) string {
	if isEnum {
		return "enum"
	}
	if isSealed && isInterface {
		return "sealed interface"
	}
	if isSealed {
		return "sealed class"
	}
	if isInterface {
		return "interface"
	}
	return "class"
}

func flatDeclarationName(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		child := file.FlatNamedChild(idx, i)
		switch file.FlatType(child) {
		case "type_identifier", "simple_identifier":
			return file.FlatNodeText(child)
		}
	}
	return ""
}

func flatDeclarationSupertypes(file *scanner.File, idx uint32) []string {
	var supertypes []string
	if file == nil || idx == 0 {
		return supertypes
	}
	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		child := file.FlatNamedChild(idx, i)
		switch file.FlatType(child) {
		case "delegation_specifier", "delegation_specifiers":
			supertypes = append(supertypes, flatExtractSupertypes(file, child)...)
		}
	}
	return supertypes
}

func flatExtractSupertypes(file *scanner.File, idx uint32) []string {
	if file == nil || idx == 0 {
		return nil
	}
	var supertypes []string
	file.FlatWalkAllNodes(idx, func(node uint32) {
		switch file.FlatType(node) {
		case "user_type", "type_identifier":
			text := file.FlatNodeText(node)
			if cut := strings.Index(text, "<"); cut >= 0 {
				text = text[:cut]
			}
			text = strings.TrimSpace(text)
			if text != "" {
				supertypes = append(supertypes, text)
			}
		}
	})
	return supertypes
}

func flatExtractEnumEntries(file *scanner.File, idx uint32) []string {
	if file == nil || idx == 0 {
		return nil
	}
	var entries []string
	file.FlatWalkAllNodes(idx, func(node uint32) {
		if file.FlatType(node) != "enum_entry" {
			return
		}
		for i := 0; i < file.FlatNamedChildCount(node); i++ {
			child := file.FlatNamedChild(node, i)
			if file.FlatType(child) == "simple_identifier" {
				entries = append(entries, file.FlatNodeText(child))
			}
		}
	})
	return entries
}

func trimmedNodePrefix(text string, limit int) string {
	if limit > 0 && len(text) > limit {
		text = text[:limit]
	}
	return text
}

func declarationLine(file *scanner.File, flatIdx uint32) int {
	if file == nil {
		return 0
	}
	return int(file.FlatRow(flatIdx)) + 1
}

func flatMemberName(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	switch file.FlatType(idx) {
	case "property_declaration":
		if varDecl, ok := file.FlatFindChild(idx, "variable_declaration"); ok {
			return flatMemberName(file, varDecl)
		}
	case "variable_declaration", "function_declaration", "class_declaration", "object_declaration":
		return flatDeclarationName(file, idx)
	}
	return ""
}

func flatFunctionSignature(file *scanner.File, idx uint32) (receiverType, funcName string, foundDot bool) {
	if file == nil || idx == 0 {
		return "", "", false
	}
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case "user_type":
			if funcName == "" {
				receiverType = file.FlatNodeText(child)
			}
		case ".":
			if receiverType != "" && funcName == "" {
				foundDot = true
			}
		case "simple_identifier":
			if funcName == "" {
				funcName = file.FlatNodeText(child)
			}
		}
	}
	return receiverType, funcName, foundDot
}

func (r *defaultResolver) extractMembersFlat(bodyIdx uint32, file *scanner.File, it *ImportTable) []MemberInfo {
	var members []MemberInfo
	appendMember := func(memberIdx uint32) {
		if memberIdx == 0 {
			return
		}
		switch file.FlatType(memberIdx) {
		case "function_declaration":
			name := flatMemberName(file, memberIdx)
			if name == "" {
				name = flatFindIdentifier(file, memberIdx)
			}
			if name == "" {
				return
			}
			mods := flatReadModifierFlags(file, memberIdx)
			retType := r.extractFunctionReturnTypeFlat(memberIdx, file, it)
			members = append(members, MemberInfo{
				Name:       name,
				Kind:       "function",
				Type:       retType,
				Visibility: mods.visibility,
				IsOverride: mods.override,
				IsAbstract: mods.abstract,
			})
		case "property_declaration":
			name := flatMemberName(file, memberIdx)
			if name == "" {
				name = flatFindIdentifier(file, memberIdx)
			}
			if name == "" {
				if varDecl := flatFindNamedChildOfType(file, memberIdx, "variable_declaration"); varDecl != 0 {
					name, _ = flatVariableDeclNameAndType(file, varDecl)
				}
			}
			if name == "" {
				return
			}
			mods := flatReadModifierFlags(file, memberIdx)
			typ := r.resolvePropertyTypeFlat(memberIdx, file, it)
			members = append(members, MemberInfo{
				Name:       name,
				Kind:       "property",
				Type:       typ,
				Visibility: mods.visibility,
				IsOverride: mods.override,
				IsAbstract: mods.abstract,
			})
		}
	}

	for i := 0; i < file.FlatNamedChildCount(bodyIdx); i++ {
		child := file.FlatNamedChild(bodyIdx, i)
		if child == 0 {
			continue
		}
		switch file.FlatType(child) {
		case "class_member_declarations", "statements":
			for j := 0; j < file.FlatNamedChildCount(child); j++ {
				appendMember(file.FlatNamedChild(child, j))
			}
		default:
			appendMember(child)
		}
	}

	return members
}

func (r *defaultResolver) extractFunctionReturnTypeFlat(funcIdx uint32, file *scanner.File, it *ImportTable) *ResolvedType {
	foundParams := false
	for i := 0; i < file.FlatNamedChildCount(funcIdx); i++ {
		child := file.FlatNamedChild(funcIdx, i)
		switch file.FlatType(child) {
		case "function_value_parameters":
			foundParams = true
			continue
		}
		if foundParams {
			switch file.FlatType(child) {
			case "user_type", "nullable_type", "type_identifier":
				return r.resolveTypeNodeFlat(child, file, it)
			case "function_body", "{":
				return nil
			}
		}
	}
	return nil
}

