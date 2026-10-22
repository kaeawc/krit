package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/analyzers/nullflow"
	"github.com/kaeawc/krit/internal/experiment"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ---------------------------------------------------------------------------
// MapGetWithNotNullAssertionRule detects map[key]!!.
// ---------------------------------------------------------------------------
type MapGetWithNotNullAssertionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — tree-sitter
// structural check backed by resolver/source type confirmation that the
// receiver is Map-like. Classified per roadmap/17.
func (r *MapGetWithNotNullAssertionRule) Confidence() float64 { return api.ConfidenceMedium }

type mapGetBangAccess struct {
	access   uint32
	receiver uint32
	key      uint32
	call     bool
	safeCall bool
}

func (r *MapGetWithNotNullAssertionRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	// Skip test files — fail-fast `map[key]!!` is idiomatic in tests.
	if scanner.IsTestFile(file.Path) {
		return
	}
	access, ok := flatMapGetBangAccess(file, idx)
	if !ok {
		return
	}
	receiverType, ok := mapGetReceiverMapType(ctx, access.receiver)
	if !ok || !mapGetAccessMatchesMapGet(ctx, access, receiverType) {
		return
	}
	// Skip when the access is guarded by `map.containsKey(key)` in an
	// enclosing if or earlier statement, or by a preceding filter.
	if nullflow.IsMapContainsKeyGuarded(file, idx, access.receiver, access.key) ||
		nullflow.IsEarlyReturnMapContainsKeyGuarded(file, idx, access.receiver, access.key, bodyAlwaysExitsFlat) {
		return
	}
	if experiment.Enabled("map-get-bang-skip-contains-key-filter") &&
		nullflow.IsInsideContainsKeyFilterChain(file, idx, access.receiver) {
		return
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Map access with not-null assertion operator (!!). Use getValue() or getOrDefault() instead.")
	if !access.safeCall {
		f.Fix = &scanner.Fix{
			ByteMode:  true,
			StartByte: int(file.FlatStartByte(idx)),
			EndByte:   int(file.FlatEndByte(idx)),
			Replacement: strings.TrimSpace(file.FlatNodeText(access.receiver)) +
				".getValue(" + strings.TrimSpace(file.FlatNodeText(access.key)) + ")",
		}
	}
	ctx.Emit(f)
}

func flatMapGetBangAccess(file *scanner.File, idx uint32) (mapGetBangAccess, bool) {
	if file == nil || file.FlatType(idx) != "postfix_expression" || !flatPostfixHasBangBang(file, idx) {
		return mapGetBangAccess{}, false
	}
	expr := flatFirstNamedChild(file, idx)
	if expr == 0 {
		return mapGetBangAccess{}, false
	}
	access := flatUnwrapParenExpr(file, expr)
	switch file.FlatType(access) {
	case "indexing_expression":
		receiver, key, ok := flatIndexingExpressionParts(file, access)
		if !ok {
			return mapGetBangAccess{}, false
		}
		return mapGetBangAccess{access: access, receiver: receiver, key: key}, true
	case "call_expression":
		receiver, key, safeCall, ok := flatGetCallExpressionParts(file, access)
		if !ok {
			return mapGetBangAccess{}, false
		}
		return mapGetBangAccess{access: access, receiver: receiver, key: key, call: true, safeCall: safeCall}, true
	default:
		return mapGetBangAccess{}, false
	}
}

func flatPostfixHasBangBang(file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) && file.FlatType(child) == "!!" {
			return true
		}
	}
	return false
}

func flatIndexingExpressionParts(file *scanner.File, idx uint32) (receiver, key uint32, ok bool) {
	if file.FlatType(idx) != "indexing_expression" {
		return 0, 0, false
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if file.FlatType(child) == "indexing_suffix" {
			if key != 0 || file.FlatNamedChildCount(child) != 1 {
				return 0, 0, false
			}
			key = file.FlatNamedChild(child, 0)
			continue
		}
		if receiver == 0 {
			receiver = child
		}
	}
	return receiver, key, receiver != 0 && key != 0
}

func flatGetCallExpressionParts(file *scanner.File, idx uint32) (receiver, key uint32, safeCall bool, ok bool) {
	nav, args := flatCallExpressionParts(file, idx)
	if nav == 0 || args == 0 || flatNavigationExpressionLastIdentifier(file, nav) != "get" {
		return 0, 0, false, false
	}
	receiver = flatNavigationExpressionReceiver(file, nav)
	if receiver == 0 {
		return 0, 0, false, false
	}
	key, ok = flatSingleValueArgumentExpression(file, args)
	if !ok {
		return 0, 0, false, false
	}
	return receiver, key, flatNavigationLastSuffixHasSafeAccess(file, nav), true
}

func flatNavigationExpressionReceiver(file *scanner.File, nav uint32) uint32 {
	if file == nil || nav == 0 || file.FlatType(nav) != "navigation_expression" || file.FlatNamedChildCount(nav) < 2 {
		return 0
	}
	return file.FlatNamedChild(nav, 0)
}

func flatNavigationLastSuffixHasSafeAccess(file *scanner.File, nav uint32) bool {
	var suffix uint32
	for child := file.FlatFirstChild(nav); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "navigation_suffix" {
			suffix = child
		}
	}
	if suffix == 0 {
		return false
	}
	for child := file.FlatFirstChild(suffix); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) && file.FlatType(child) == "?." {
			return true
		}
	}
	return false
}

func flatSingleValueArgumentExpression(file *scanner.File, args uint32) (uint32, bool) {
	var arg uint32
	for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "value_argument" {
			continue
		}
		if arg != 0 {
			return 0, false
		}
		arg = child
	}
	if arg == 0 {
		return 0, false
	}
	if flatHasValueArgumentLabel(file, arg) {
		expr := flatLastNamedChild(file, arg)
		return expr, expr != 0
	}
	expr := flatValueArgumentExpression(file, arg)
	return expr, expr != 0
}

func mapGetAccessMatchesMapGet(ctx *api.Context, access mapGetBangAccess, receiverType *typeinfer.ResolvedType) bool {
	if access.call {
		target, ok := semantics.ResolveCallTarget(ctx, access.access)
		if !ok || target.CalleeName != "get" {
			return false
		}
		if target.Resolved && !mapGetResolvedTargetIsMapGet(target.QualifiedName) {
			return false
		}
	}
	return mapGetKeyCompatible(ctx, receiverType, access.key)
}

func mapGetResolvedTargetIsMapGet(target string) bool {
	return target == "kotlin.collections.Map.get" ||
		target == "kotlin.collections.MutableMap.get" ||
		target == "java.util.Map.get" ||
		strings.HasSuffix(target, ".Map.get") ||
		strings.HasSuffix(target, ".MutableMap.get")
}

func mapGetReceiverMapType(ctx *api.Context, receiver uint32) (*typeinfer.ResolvedType, bool) {
	if ctx.File == nil || ctx.Resolver == nil || receiver == 0 {
		return nil, false
	}
	receiver = flatUnwrapParenExpr(ctx.File, receiver)
	resolved := mapResolveExpressionType(ctx, receiver, nil)
	if mapResolvedTypeIsMap(ctx.Resolver, resolved, nil) {
		return resolved, true
	}
	return nil, false
}

func mapResolveExpressionType(ctx *api.Context, expr uint32, seen map[uint32]bool) *typeinfer.ResolvedType {
	if ctx.File == nil || ctx.Resolver == nil || expr == 0 {
		return nil
	}
	expr = flatUnwrapParenExpr(ctx.File, expr)
	if seen == nil {
		seen = make(map[uint32]bool)
	}
	if seen[expr] {
		return nil
	}
	seen[expr] = true

	switch ctx.File.FlatType(expr) {
	case "simple_identifier":
		if resolved := ctx.Resolver.ResolveFlatNode(expr, ctx.File); resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
			return resolved
		}
		return ctx.Resolver.ResolveByNameFlat(ctx.File.FlatNodeString(expr, nil), expr, ctx.File)
	case "navigation_expression":
		name := flatNavigationExpressionLastIdentifier(ctx.File, expr)
		if name != "" {
			if resolved := ctx.Resolver.ResolveByNameFlat(name, expr, ctx.File); resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
				return resolved
			}
			base := flatNavigationExpressionReceiver(ctx.File, expr)
			if baseType := mapResolveExpressionType(ctx, base, seen); baseType != nil {
				return mapMemberType(ctx, baseType, name)
			}
		}
	}
	resolved := ctx.Resolver.ResolveFlatNode(expr, ctx.File)
	if resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
		return resolved
	}
	return nil
}

func mapResolvedTypeIsMap(resolver typeinfer.TypeResolver, resolved *typeinfer.ResolvedType, seen map[string]bool) bool {
	if resolved == nil || resolved.Kind == typeinfer.TypeUnknown {
		return false
	}
	if mapTypeNameIsKnown(resolved.Name) || mapTypeNameIsKnown(resolved.FQN) {
		return true
	}
	for _, super := range resolved.Supertypes {
		if mapTypeNameIsKnown(super) {
			return true
		}
	}
	if resolver == nil {
		return false
	}
	if seen == nil {
		seen = make(map[string]bool)
	}
	for _, name := range []string{resolved.FQN, resolved.Name} {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if info := resolver.ClassHierarchy(name); info != nil {
			for _, super := range info.Supertypes {
				if mapTypeNameIsKnown(super) {
					return true
				}
				if mapResolvedTypeIsMap(resolver, &typeinfer.ResolvedType{Name: simpleTypeName(super), FQN: super, Kind: typeinfer.TypeClass}, seen) {
					return true
				}
			}
		}
	}
	return false
}

func mapTypeNameIsKnown(name string) bool {
	switch name {
	case "Map", "MutableMap", "HashMap", "LinkedHashMap", "TreeMap",
		"kotlin.collections.Map", "kotlin.collections.MutableMap",
		"kotlin.collections.HashMap", "kotlin.collections.LinkedHashMap", "kotlin.collections.TreeMap",
		"java.util.Map", "java.util.HashMap", "java.util.LinkedHashMap", "java.util.TreeMap":
		return true
	default:
		return false
	}
}

func mapGetKeyCompatible(ctx *api.Context, receiverType *typeinfer.ResolvedType, key uint32) bool {
	if receiverType == nil || len(receiverType.TypeArgs) == 0 {
		return true
	}
	want := receiverType.TypeArgs[0]
	if mapTypeArgIsWildcard(want) {
		return true
	}
	got := mapResolveExpressionType(ctx, key, nil)
	if got == nil || got.Kind == typeinfer.TypeUnknown {
		return false
	}
	return mapTypesCompatible(want, *got)
}

func mapTypeArgIsWildcard(t typeinfer.ResolvedType) bool {
	name := simpleTypeName(t.Name)
	return name == "" || name == "*" || name == "Any"
}

func mapTypesCompatible(want, got typeinfer.ResolvedType) bool {
	if mapTypeArgIsWildcard(want) {
		return true
	}
	wantName := simpleTypeName(firstNonEmpty(want.FQN, want.Name))
	gotName := simpleTypeName(firstNonEmpty(got.FQN, got.Name))
	if wantName == "" || gotName == "" {
		return false
	}
	if wantName == gotName {
		return true
	}
	return typeinfer.MapJavaToKotlin(firstNonEmpty(got.FQN, got.Name)) == firstNonEmpty(want.FQN, want.Name)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func mapMemberType(ctx *api.Context, owner *typeinfer.ResolvedType, member string) *typeinfer.ResolvedType {
	if ctx.File == nil || ctx.Resolver == nil || owner == nil || member == "" {
		return nil
	}
	for _, ownerName := range []string{owner.FQN, owner.Name} {
		if ownerName == "" {
			continue
		}
		if info := ctx.Resolver.ClassHierarchy(ownerName); info != nil {
			for _, m := range info.Members {
				if m.Name == member && m.Type != nil && m.Type.Kind != typeinfer.TypeUnknown {
					return m.Type
				}
			}
		}
		if typ := mapMemberTypeFromSameFileDeclaration(ctx, simpleTypeName(ownerName), member); typ != nil {
			return typ
		}
	}
	return nil
}

func mapMemberTypeFromSameFileDeclaration(ctx *api.Context, ownerName, member string) *typeinfer.ResolvedType {
	if ctx.File == nil || ctx.Resolver == nil || ownerName == "" || member == "" {
		return nil
	}
	file := ctx.File
	var classDecl uint32
	file.FlatWalkAllNodes(0, func(candidate uint32) {
		if classDecl != 0 || file.FlatType(candidate) != "class_declaration" {
			return
		}
		if extractIdentifierFlat(file, candidate) == ownerName {
			classDecl = candidate
		}
	})
	if classDecl == 0 {
		return nil
	}
	var out *typeinfer.ResolvedType
	file.FlatWalkAllNodes(classDecl, func(candidate uint32) {
		if out != nil || candidate == classDecl {
			return
		}
		if !mapClassMemberCandidate(file, classDecl, candidate) || extractIdentifierFlat(file, candidate) != member {
			return
		}
		if typeNode := mapExplicitTypeNode(file, candidate); typeNode != 0 {
			out = ctx.Resolver.ResolveFlatNode(typeNode, file)
			if out == nil || out.Kind == typeinfer.TypeUnknown {
				out = &typeinfer.ResolvedType{Name: simpleTypeName(file.FlatNodeText(typeNode)), Kind: typeinfer.TypeClass}
			}
		}
	})
	return out
}

func mapClassMemberCandidate(file *scanner.File, classDecl, candidate uint32) bool {
	switch file.FlatType(candidate) {
	case "class_parameter", "property_declaration":
	default:
		return false
	}
	for p, ok := file.FlatParent(candidate); ok; p, ok = file.FlatParent(p) {
		if p == classDecl {
			return true
		}
		switch file.FlatType(p) {
		case "function_declaration", "lambda_literal", "object_declaration":
			return false
		case "class_declaration":
			return p == classDecl
		}
	}
	return false
}

func mapExplicitTypeNode(file *scanner.File, idx uint32) uint32 {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type", "nullable_type", "type_identifier":
			return child
		case "variable_declaration":
			if inner := mapExplicitTypeNode(file, child); inner != 0 {
				return inner
			}
		}
	}
	return 0
}
