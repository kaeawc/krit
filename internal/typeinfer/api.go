package typeinfer

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// ExtensionFuncInfo describes an extension function from source.
type ExtensionFuncInfo struct {
	ReceiverType string        // Simple name of the receiver type (e.g., "String")
	Name         string        // Function name
	ReturnType   *ResolvedType // Return type
}

// defaultResolver is the concrete implementation of TypeResolver.
// It is built via IndexFilesParallel() which populates all maps,
// then used read-only during parallel rule execution. No locking needed
// because indexing completes before any reads begin.
type defaultResolver struct {
	// Per-file import tables
	imports map[string]*ImportTable // file path -> import table
	// Per-file scope tables (top-level declarations)
	scopes map[string]*ScopeTable // file path -> root scope
	// Class info by simple name and FQN
	classes  map[string]*ClassInfo // simple name -> info
	classFQN map[string]*ClassInfo // FQN -> info
	// Sealed class variants
	sealedVariants map[string][]string // sealed type name -> variant names
	// Enum entries
	enumEntries map[string][]string // enum type name -> entry names
	// Top-level and class-level function return types
	functions map[string]*ResolvedType // function name -> return type
	// Extension functions from source
	extensions []*ExtensionFuncInfo
}

// NewResolver creates a new resolver backed by source-level analysis.
func NewResolver() *defaultResolver {
	return &defaultResolver{
		imports:        make(map[string]*ImportTable),
		scopes:         make(map[string]*ScopeTable),
		classes:        make(map[string]*ClassInfo),
		classFQN:       make(map[string]*ClassInfo),
		sealedVariants: make(map[string][]string),
		enumEntries:    make(map[string][]string),
		functions:      make(map[string]*ResolvedType),
	}
}

// ---------------------------------------------------------------------------
// TypeResolver interface implementation
// ---------------------------------------------------------------------------

func (r *defaultResolver) ResolveFlatNode(idx uint32, file *scanner.File) *ResolvedType {
	if file == nil || idx == 0 {
		return UnknownType()
	}

	it := r.imports[file.Path]

	switch file.FlatType(idx) {
	case "string_literal", "line_string_literal", "multi_line_string_literal":
		return r.makeResolvedType("String", it, false)
	case "integer_literal":
		return r.makeResolvedType("Int", it, false)
	case "long_literal":
		return r.makeResolvedType("Long", it, false)
	case "real_literal":
		text := file.FlatNodeText(idx)
		if strings.HasSuffix(strings.ToLower(text), "f") {
			return r.makeResolvedType("Float", it, false)
		}
		return r.makeResolvedType("Double", it, false)
	case "boolean_literal":
		return r.makeResolvedType("Boolean", it, false)
	case "character_literal":
		return r.makeResolvedType("Char", it, false)
	case "null_literal":
		t := UnknownType()
		t.Nullable = true
		return t
	case "simple_identifier":
		name := file.FlatNodeText(idx)
		if resolved := r.resolveFlatName(name, file.FlatStartByte(idx), file); resolved != nil {
			return resolved
		}
		return UnknownType()
	case "this_expression":
		if resolved := r.resolveFlatName("this", file.FlatStartByte(idx), file); resolved != nil {
			return resolved
		}
		return UnknownType()
	case "property_declaration":
		if typ := r.resolvePropertyTypeFlat(idx, file, it); typ != nil {
			return typ
		}
		return UnknownType()
	case "call_expression":
		return r.inferCallExpressionTypeFlat(idx, file, it)
	case "navigation_expression":
		return r.inferNavigationExpressionTypeFlat(idx, file, it)
	case "user_type", "nullable_type", "type_identifier":
		return r.resolveTypeNodeFlat(idx, file, it)
	}

	return r.inferExpressionTypeFlat(idx, file, it)
}

func (r *defaultResolver) ResolveByNameFlat(name string, idx uint32, file *scanner.File) *ResolvedType {
	if file == nil {
		return nil
	}
	return r.resolveByNameAtOffset(name, file.FlatStartByte(idx), file)
}

func (r *defaultResolver) resolveByNameAtOffset(name string, offset uint32, file *scanner.File) *ResolvedType {
	if file == nil {
		return nil
	}

	rootScope := r.scopes[file.Path]
	it := r.imports[file.Path]

	if rootScope != nil {
		if scope := rootScope.FindScopeAtOffset(offset); scope != nil {
			if t := scope.Lookup(name); t != nil {
				return t
			}
		}
		if t := rootScope.Lookup(name); t != nil {
			return t
		}
	}

	if it != nil {
		if fqn := it.Resolve(name); fqn != "" {
			if info, ok := r.classFQN[fqn]; ok {
				return &ResolvedType{Name: info.Name, FQN: info.FQN, Kind: TypeClass}
			}
			return r.makeResolvedType(name, it, false)
		}
	}

	if info, ok := r.classes[name]; ok {
		return &ResolvedType{Name: info.Name, FQN: info.FQN, Kind: TypeClass}
	}

	return nil
}

func (r *defaultResolver) ResolveImport(simpleName string, file *scanner.File) string {
	it := r.imports[file.Path]
	if it != nil {
		return it.Resolve(simpleName)
	}
	return ""
}

func (r *defaultResolver) IsNullableFlat(idx uint32, file *scanner.File) *bool {
	if file == nil || idx == 0 {
		return nil
	}
	resolved := r.ResolveFlatNode(idx, file)
	if resolved.Kind == TypeUnknown {
		return nil
	}
	if file.FlatType(idx) == "simple_identifier" {
		if rootScope := r.scopes[file.Path]; rootScope != nil {
			if scope := rootScope.FindScopeAtOffset(file.FlatStartByte(idx)); scope != nil && scope.IsSmartCastNonNull(file.FlatNodeText(idx)) {
				nonNull := false
				return &nonNull
			}
		}
	}
	result := resolved.IsNullable()
	return &result
}

func (r *defaultResolver) resolveFlatName(name string, offset uint32, file *scanner.File) *ResolvedType {
	rootScope := r.scopes[file.Path]
	it := r.imports[file.Path]

	if rootScope != nil {
		if scope := rootScope.FindScopeAtOffset(offset); scope != nil {
			if t := scope.Lookup(name); t != nil {
				return t
			}
		}
		if t := rootScope.Lookup(name); t != nil {
			return t
		}
	}

	if it != nil {
		fqn := it.Resolve(name)
		if fqn != "" {
			if info, ok := r.classFQN[fqn]; ok {
				return &ResolvedType{Name: info.Name, FQN: info.FQN, Kind: TypeClass}
			}
			return r.makeResolvedType(name, it, false)
		}
	}

	if info, ok := r.classes[name]; ok {
		return &ResolvedType{Name: info.Name, FQN: info.FQN, Kind: TypeClass}
	}
	return nil
}

func (r *defaultResolver) resolveTypeNodeFlat(idx uint32, file *scanner.File, it *ImportTable) *ResolvedType {
	if file == nil || idx == 0 {
		return UnknownType()
	}
	if file.FlatType(idx) == "nullable_type" {
		innerIdx := flatFirstResolvableTypeChild(file, idx)
		inner := UnknownType()
		if innerIdx != 0 {
			inner = r.resolveTypeNodeFlat(innerIdx, file, it)
		}
		inner.Nullable = true
		inner.Kind = TypeNullable
		return inner
	}

	simpleName, hasTypeArgs := resolvedTypeSimpleNameFlat(file, idx)
	if simpleName == "" {
		simpleName = strings.TrimSpace(file.FlatNodeText(idx))
		if cut := strings.Index(simpleName, "<"); cut >= 0 {
			simpleName = simpleName[:cut]
		}
	}

	if fqn, ok := PrimitiveTypes[simpleName]; ok {
		result := &ResolvedType{Name: simpleName, FQN: fqn, Kind: TypePrimitive}
		if hasTypeArgs {
			r.attachFlatTypeArgs(result, idx, file, it)
		}
		return result
	}

	fqn := ""
	if it != nil {
		fqn = it.Resolve(simpleName)
	}
	if fqn == "" {
		fqn = simpleName
	}

	kind := TypeClass
	if simpleName == "Unit" {
		kind = TypeUnit
	} else if simpleName == "Nothing" {
		kind = TypeNothing
	}

	result := &ResolvedType{Name: simpleName, FQN: fqn, Kind: kind}
	if hasTypeArgs {
		r.attachFlatTypeArgs(result, idx, file, it)
	}
	return result
}

func (r *defaultResolver) attachFlatTypeArgs(result *ResolvedType, idx uint32, file *scanner.File, it *ImportTable) {
	typeArgs := flatFindNamedChildOfType(file, idx, "type_arguments")
	if typeArgs == 0 {
		return
	}
	for i := 0; i < file.FlatNamedChildCount(typeArgs); i++ {
		proj := file.FlatNamedChild(typeArgs, i)
		if proj == 0 || file.FlatType(proj) != "type_projection" {
			continue
		}
		inner := flatFirstResolvableTypeChild(file, proj)
		if inner == 0 {
			continue
		}
		t := r.resolveTypeNodeFlat(inner, file, it)
		if t != nil {
			result.TypeArgs = append(result.TypeArgs, *t)
		}
	}
}

func (r *defaultResolver) resolvePropertyTypeFlat(idx uint32, file *scanner.File, it *ImportTable) *ResolvedType {
	if typeNode := flatExplicitTypeNode(file, idx); typeNode != 0 {
		return r.resolveTypeNodeFlat(typeNode, file, it)
	}
	if delegate := flatFindNamedChildOfType(file, idx, "property_delegate"); delegate != 0 {
		return r.inferDelegateTypeFlat(delegate, file, it)
	}
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if child == 0 {
			continue
		}
		switch file.FlatType(child) {
		case "expression", "call_expression", "navigation_expression", "integer_literal", "string_literal", "boolean_literal",
			"long_literal", "real_literal", "null_literal", "parenthesized_expression", "additive_expression",
			"multiplicative_expression", "comparison_expression", "equality_expression", "conjunction_expression",
			"disjunction_expression", "prefix_expression", "range_expression", "jump_expression", "this_expression",
			"simple_identifier":
			return r.ResolveFlatNode(child, file)
		}
	}
	return UnknownType()
}

func (r *defaultResolver) inferExpressionTypeFlat(idx uint32, file *scanner.File, it *ImportTable) *ResolvedType {
	if file == nil || idx == 0 {
		return UnknownType()
	}
	switch file.FlatType(idx) {
	case "string_literal", "line_string_literal", "multi_line_string_literal":
		return r.makeResolvedType("String", it, false)
	case "integer_literal":
		return r.makeResolvedType("Int", it, false)
	case "long_literal":
		return r.makeResolvedType("Long", it, false)
	case "real_literal":
		text := file.FlatNodeText(idx)
		if strings.HasSuffix(strings.ToLower(text), "f") {
			return r.makeResolvedType("Float", it, false)
		}
		return r.makeResolvedType("Double", it, false)
	case "boolean_literal":
		return r.makeResolvedType("Boolean", it, false)
	case "character_literal":
		return r.makeResolvedType("Char", it, false)
	case "null_literal":
		t := UnknownType()
		t.Nullable = true
		return t
	case "call_expression":
		return r.inferCallExpressionTypeFlat(idx, file, it)
	case "this_expression":
		if typ := r.resolveFlatName("this", file.FlatStartByte(idx), file); typ != nil {
			return typ
		}
		return UnknownType()
	case "simple_identifier":
		if typ := r.resolveFlatName(file.FlatNodeText(idx), file.FlatStartByte(idx), file); typ != nil {
			return typ
		}
		return UnknownType()
	case "navigation_expression":
		return r.inferNavigationExpressionTypeFlat(idx, file, it)
	case "parenthesized_expression":
		for i := 0; i < file.FlatChildCount(idx); i++ {
			child := file.FlatChild(idx, i)
			switch file.FlatType(child) {
			case "(", ")":
				continue
			default:
				return r.inferExpressionTypeFlat(child, file, it)
			}
		}
	case "additive_expression":
		left := r.inferExpressionTypeFlat(file.FlatChild(idx, 0), file, it)
		if left != nil && left.Name == "String" {
			return r.makeResolvedType("String", it, false)
		}
		if left != nil && (left.Kind == TypePrimitive || left.Name == "Int" || left.Name == "Long" || left.Name == "Double" || left.Name == "Float") {
			return left
		}
		return UnknownType()
	case "multiplicative_expression":
		left := r.inferExpressionTypeFlat(file.FlatChild(idx, 0), file, it)
		if left != nil && left.Kind == TypePrimitive {
			return left
		}
		return UnknownType()
	case "comparison_expression", "equality_expression", "conjunction_expression", "disjunction_expression":
		return r.makeResolvedType("Boolean", it, false)
	case "prefix_expression":
		if file.FlatChildCount(idx) > 0 {
			first := file.FlatChild(idx, 0)
			text := file.FlatNodeText(first)
			if text == "!" {
				return r.makeResolvedType("Boolean", it, false)
			}
			if text == "-" && file.FlatChildCount(idx) > 1 {
				return r.inferExpressionTypeFlat(file.FlatChild(idx, 1), file, it)
			}
		}
		return UnknownType()
	case "range_expression":
		return r.makeResolvedType("IntRange", it, false)
	case "jump_expression":
		return r.makeResolvedType("Nothing", it, false)
	case "if_expression":
		for i := 0; i < file.FlatChildCount(idx); i++ {
			child := file.FlatChild(idx, i)
			if file.FlatType(child) == "control_structure_body" {
				typ := r.inferExpressionTypeFlat(child, file, it)
				if typ != nil && typ.Kind != TypeUnknown {
					return typ
				}
			}
		}
	case "as_expression":
		foundAs := false
		isSafe := false
		for i := 0; i < file.FlatChildCount(idx); i++ {
			child := file.FlatChild(idx, i)
			childType := file.FlatType(child)
			if childType == "as" {
				foundAs = true
				continue
			}
			if childType == "as?" {
				foundAs = true
				isSafe = true
				continue
			}
			if foundAs && (childType == "user_type" || childType == "nullable_type" || childType == "type_identifier") {
				typ := r.resolveTypeNodeFlat(child, file, it)
				if isSafe {
					typ.Nullable = true
					typ.Kind = TypeNullable
				}
				return typ
			}
		}
	}
	return UnknownType()
}

func (r *defaultResolver) inferCallExpressionTypeFlat(idx uint32, file *scanner.File, it *ImportTable) *ResolvedType {
	collectionFactories := map[string]string{
		"listOf": "List", "mutableListOf": "MutableList", "arrayListOf": "ArrayList",
		"setOf": "Set", "mutableSetOf": "MutableSet", "hashSetOf": "HashSet",
		"linkedSetOf": "LinkedHashSet",
		"mapOf":       "Map", "mutableMapOf": "MutableMap", "hashMapOf": "HashMap",
		"linkedMapOf": "LinkedHashMap",
		"sequenceOf":  "Sequence",
		"emptyList":   "List", "emptySet": "Set", "emptyMap": "Map",
		"arrayOf":    "Array",
		"intArrayOf": "IntArray", "longArrayOf": "LongArray",
		"byteArrayOf": "ByteArray", "charArrayOf": "CharArray",
		"booleanArrayOf": "BooleanArray", "floatArrayOf": "FloatArray",
		"doubleArrayOf": "DoubleArray", "shortArrayOf": "ShortArray",
	}

	firstChild := file.FlatChild(idx, 0)
	funcName := flatLastIdentifierText(file, firstChild)
	if resultType, ok := collectionFactories[funcName]; ok {
		result := r.makeResolvedType(resultType, it, false)
		r.attachFlatCallTypeArgs(result, idx, file, it)
		return result
	}
	if funcName != "" && len(funcName) > 0 && funcName[0] >= 'A' && funcName[0] <= 'Z' {
		return r.makeResolvedType(funcName, it, false)
	}
	switch funcName {
	case "lazy":
		return r.makeResolvedType("Lazy", it, false)
	case "to":
		return r.makeResolvedType("Pair", it, false)
	}

	if file.FlatType(firstChild) == "navigation_expression" && funcName != "" {
		receiverIdx := flatNavigationReceiver(file, firstChild)
		if funcName == "apply" || funcName == "also" {
			receiverType := r.ResolveFlatNode(receiverIdx, file)
			if receiverType != nil && receiverType.Kind != TypeUnknown {
				return receiverType
			}
		}
		receiverType := r.ResolveFlatNode(receiverIdx, file)
		if receiverType != nil && receiverType.Kind != TypeUnknown {
			if m := LookupStdlibMethod(receiverType.Name, funcName); m != nil {
				return r.applyStdlibReturnType(m, receiverType)
			}
			for _, ext := range r.extensions {
				if ext.Name == funcName && ext.ReceiverType == receiverType.Name {
					return ext.ReturnType
				}
			}
		}
		if receiverIdx != 0 && file.FlatType(receiverIdx) == "simple_identifier" {
			key := file.FlatNodeText(receiverIdx) + "." + funcName
			if retType, ok := r.functions[key]; ok {
				return retType
			}
		}
	}

	if funcName != "" {
		if m := LookupStdlibMethod("_", funcName); m != nil {
			return &ResolvedType{Name: m.ReturnType.Name, FQN: m.ReturnType.FQN, Kind: m.ReturnType.Kind, Nullable: m.Nullable}
		}
	}
	if funcName != "" {
		if retType, ok := r.functions[funcName]; ok {
			return retType
		}
	}
	return UnknownType()
}

func (r *defaultResolver) attachFlatCallTypeArgs(result *ResolvedType, idx uint32, file *scanner.File, it *ImportTable) {
	typeArgs := flatFindNamedChildOfType(file, idx, "type_arguments")
	if typeArgs == 0 {
		if callSuffix := flatFindNamedChildOfType(file, idx, "call_suffix"); callSuffix != 0 {
			typeArgs = flatFindNamedChildOfType(file, callSuffix, "type_arguments")
		}
	}
	if typeArgs == 0 {
		return
	}
	for i := 0; i < file.FlatNamedChildCount(typeArgs); i++ {
		proj := file.FlatNamedChild(typeArgs, i)
		if proj == 0 || file.FlatType(proj) != "type_projection" {
			continue
		}
		inner := flatFirstResolvableTypeChild(file, proj)
		if inner == 0 {
			continue
		}
		t := r.resolveTypeNodeFlat(inner, file, it)
		if t != nil {
			result.TypeArgs = append(result.TypeArgs, *t)
		}
	}
}

func (r *defaultResolver) inferNavigationExpressionTypeFlat(idx uint32, file *scanner.File, it *ImportTable) *ResolvedType {
	text := strings.TrimSpace(file.FlatNodeText(idx))
	if info, ok := r.classFQN[text]; ok {
		return &ResolvedType{Name: info.Name, FQN: info.FQN, Kind: TypeClass}
	}

	memberName := flatLastIdentifierText(file, idx)
	receiverIdx := flatNavigationReceiver(file, idx)
	if memberName != "" && receiverIdx != 0 {
		if memberName == "apply" || memberName == "also" {
			receiverType := r.ResolveFlatNode(receiverIdx, file)
			if receiverType != nil && receiverType.Kind != TypeUnknown {
				return receiverType
			}
		}
		receiverType := r.ResolveFlatNode(receiverIdx, file)
		if receiverType != nil && receiverType.Kind != TypeUnknown {
			if m := LookupStdlibMethod(receiverType.Name, memberName); m != nil {
				return r.applyStdlibReturnType(m, receiverType)
			}
			for _, ext := range r.extensions {
				if ext.Name == memberName && ext.ReceiverType == receiverType.Name {
					return ext.ReturnType
				}
			}
		}
		if file.FlatType(receiverIdx) == "simple_identifier" {
			key := file.FlatNodeText(receiverIdx) + "." + memberName
			if retType, ok := r.functions[key]; ok {
				return retType
			}
		}
		return r.ResolveFlatNode(receiverIdx, file)
	}
	return UnknownType()
}

func flatFindNamedChildOfType(file *scanner.File, idx uint32, childType string) uint32 {
	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		child := file.FlatNamedChild(idx, i)
		if child != 0 && file.FlatType(child) == childType {
			return child
		}
	}
	return 0
}

func flatFirstResolvableTypeChild(file *scanner.File, idx uint32) uint32 {
	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		child := file.FlatNamedChild(idx, i)
		switch file.FlatType(child) {
		case "user_type", "type_identifier", "nullable_type":
			return child
		}
	}
	return 0
}

func resolvedTypeSimpleNameFlat(file *scanner.File, idx uint32) (name string, hasTypeArgs bool) {
	switch file.FlatType(idx) {
	case "type_identifier", "simple_identifier":
		return file.FlatNodeText(idx), false
	case "user_type":
		for i := 0; i < file.FlatNamedChildCount(idx); i++ {
			child := file.FlatNamedChild(idx, i)
			switch file.FlatType(child) {
			case "type_arguments":
				hasTypeArgs = true
			case "type_identifier", "simple_identifier":
				if name == "" {
					name = file.FlatNodeText(child)
				}
			}
		}
	}
	return name, hasTypeArgs
}

func flatExplicitTypeNode(file *scanner.File, idx uint32) uint32 {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if child == 0 {
			continue
		}
		t := file.FlatType(child)
		if t == "user_type" || t == "nullable_type" || t == "type_identifier" {
			return child
		}
		if t == "variable_declaration" {
			if inner := flatExplicitTypeNode(file, child); inner != 0 {
				return inner
			}
		}
	}
	return 0
}

func flatNavigationReceiver(file *scanner.File, idx uint32) uint32 {
	if file == nil || idx == 0 {
		return 0
	}
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case ".", "?.", "?:", "navigation_suffix", "simple_identifier", "type_identifier":
			if i == 0 && file.FlatType(child) != "." && file.FlatType(child) != "?." && file.FlatType(child) != "?:" {
				return child
			}
			continue
		default:
			if i == 0 {
				return child
			}
		}
	}
	return file.FlatChild(idx, 0)
}

func flatLastIdentifierText(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	if t := file.FlatType(idx); t == "simple_identifier" || t == "type_identifier" {
		return file.FlatNodeText(idx)
	}
	var last string
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		switch file.FlatType(candidate) {
		case "simple_identifier", "type_identifier":
			last = file.FlatNodeText(candidate)
		}
	})
	return last
}

func (r *defaultResolver) ClassHierarchy(typeName string) *ClassInfo {
	if info, ok := r.classFQN[typeName]; ok {
		return info
	}
	if info, ok := r.classes[typeName]; ok {
		return info
	}
	// Fall back to known framework hierarchy (by FQN)
	if supertypes, ok := KnownClassHierarchy[typeName]; ok {
		return &ClassInfo{Name: simpleNameOf(typeName), FQN: typeName, Supertypes: supertypes}
	}
	// Try simple name lookup against known hierarchy
	for fqn, supertypes := range KnownClassHierarchy {
		if simpleNameOf(fqn) == typeName {
			return &ClassInfo{Name: typeName, FQN: fqn, Supertypes: supertypes}
		}
	}
	// Try known interfaces
	for ifaceFQN := range KnownInterfaces {
		if simpleNameOf(ifaceFQN) == typeName {
			return &ClassInfo{Name: typeName, FQN: ifaceFQN, Kind: "interface"}
		}
	}
	return nil
}

func (r *defaultResolver) SealedVariants(sealedTypeName string) []string {
	return r.sealedVariants[sealedTypeName]
}

func (r *defaultResolver) EnumEntries(enumTypeName string) []string {
	return r.enumEntries[enumTypeName]
}

func (r *defaultResolver) IsExceptionSubtype(a, b string) bool {
	return IsSubtypeOfException(a, b)
}

func (r *defaultResolver) AnnotationValueFlat(idx uint32, file *scanner.File, annotationName, argName string) string {
	if file == nil || idx == 0 {
		return ""
	}

	current := idx
	for {
		switch file.FlatType(current) {
		case "class_declaration", "function_declaration", "property_declaration":
			if mods := file.FlatFindChild(current, "modifiers"); mods != 0 {
				if val := r.findAnnotationValueFlat(mods, file, annotationName, argName); val != "" {
					return val
				}
			}
		}
		parent, ok := file.FlatParent(current)
		if !ok {
			break
		}
		current = parent
	}
	return ""
}

func (r *defaultResolver) findAnnotationValueFlat(modifiers uint32, file *scanner.File, annotationName, argName string) string {
	for i := 0; i < file.FlatChildCount(modifiers); i++ {
		child := file.FlatChild(modifiers, i)
		if child == 0 || file.FlatType(child) != "annotation" {
			continue
		}
		text := file.FlatNodeText(child)
		if !strings.Contains(text, annotationName) {
			continue
		}

		if argsNode := file.FlatFindChild(child, "value_arguments"); argsNode != 0 {
			for j := 0; j < file.FlatChildCount(argsNode); j++ {
				arg := file.FlatChild(argsNode, j)
				if file.FlatType(arg) != "value_argument" {
					continue
				}
				argText := strings.TrimSpace(file.FlatNodeText(arg))
				if strings.Contains(argText, "=") {
					parts := strings.SplitN(argText, "=", 2)
					name := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					if name == argName {
						return cleanAnnotationValue(value)
					}
				} else if argName == "value" || argName == "" {
					return cleanAnnotationValue(argText)
				}
			}
			continue
		}

		if start := strings.Index(text, "("); start >= 0 {
			if end := strings.LastIndex(text, ")"); end > start {
				arg := strings.TrimSpace(text[start+1 : end])
				if eqIdx := strings.Index(arg, "="); eqIdx >= 0 {
					name := strings.TrimSpace(arg[:eqIdx])
					val := strings.TrimSpace(arg[eqIdx+1:])
					if name == argName || (argName == "value" && name == "") {
						return cleanAnnotationValue(val)
					}
				} else if argName == "value" || argName == "" {
					return cleanAnnotationValue(arg)
				}
			}
		}
	}
	return ""
}

// cleanAnnotationValue strips quotes and whitespace from annotation values.
func cleanAnnotationValue(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") && len(value) >= 2 {
		value = value[1 : len(value)-1]
	}
	return value
}

