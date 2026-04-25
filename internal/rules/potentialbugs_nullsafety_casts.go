package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ---------------------------------------------------------------------------
// UnsafeCastRule detects casts that cannot ever succeed.
//
// The authoritative path consumes Kotlin compiler diagnostics from the oracle
// (CAST_NEVER_SUCCEEDS). Without compiler diagnostics, the fallback only emits
// when the local resolver proves both source and target are final, disjoint
// types.
// ---------------------------------------------------------------------------
type UnsafeCastRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-1 base confidence. The oracle-backed path mirrors
// the Kotlin compiler's CAST_NEVER_SUCCEEDS diagnostic; the non-oracle fallback
// is intentionally limited to final, disjoint types.
func (r *UnsafeCastRule) Confidence() float64 { return 0.95 }

type unsafeCastExpressionParts struct {
	source uint32
	op     uint32
	target uint32
	safe   bool
}

func unsafeCastExpressionPartsFlat(file *scanner.File, idx uint32) (unsafeCastExpressionParts, bool) {
	if file == nil || file.FlatType(idx) != "as_expression" {
		return unsafeCastExpressionParts{}, false
	}
	var out unsafeCastExpressionParts
	seenOp := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "as":
			out.op = child
			seenOp = true
			continue
		case "as?":
			out.op = child
			out.safe = true
			seenOp = true
			continue
		}
		if !file.FlatIsNamed(child) {
			continue
		}
		if !seenOp {
			if out.source == 0 {
				out.source = child
			}
			continue
		}
		if out.target == 0 && unsafeCastNodeCanBeTypeRefFlat(file, child) {
			out.target = child
		}
	}
	return out, out.source != 0 && out.op != 0 && out.target != 0
}

func unsafeCastNodeCanBeTypeRefFlat(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "user_type", "nullable_type", "type_identifier", "function_type", "parenthesized_type":
		return true
	default:
		return false
	}
}

func unsafeCastUnwrapExpressionFlat(file *scanner.File, idx uint32) uint32 {
	for idx != 0 {
		switch file.FlatType(idx) {
		case "parenthesized_expression", "expression_statement":
			next := uint32(0)
			for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
				if file.FlatIsNamed(child) {
					next = child
					break
				}
			}
			if next == 0 {
				return idx
			}
			idx = next
		default:
			return idx
		}
	}
	return 0
}

func unsafeCastComparableExpressionFlat(file *scanner.File, idx uint32) string {
	idx = unsafeCastUnwrapExpressionFlat(file, idx)
	if idx == 0 {
		return ""
	}
	switch file.FlatType(idx) {
	case "simple_identifier", "navigation_expression", "call_expression":
		return strings.TrimSpace(file.FlatNodeText(idx))
	default:
		return strings.TrimSpace(file.FlatNodeText(idx))
	}
}

func unsafeCastTypeNameFlat(file *scanner.File, idx uint32) string {
	idx = unsafeCastInnermostTypeNodeFlat(file, idx)
	if idx == 0 {
		return ""
	}
	text := strings.TrimSpace(file.FlatNodeText(idx))
	text = strings.TrimSuffix(text, "?")
	if lt := strings.IndexByte(text, '<'); lt >= 0 {
		text = text[:lt]
	}
	return strings.TrimSpace(text)
}

func unsafeCastInnermostTypeNodeFlat(file *scanner.File, idx uint32) uint32 {
	for idx != 0 {
		switch file.FlatType(idx) {
		case "nullable_type", "parenthesized_type":
			next := uint32(0)
			for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
				if file.FlatIsNamed(child) && unsafeCastNodeCanBeTypeRefFlat(file, child) {
					next = child
					break
				}
			}
			if next == 0 {
				return idx
			}
			idx = next
		default:
			return idx
		}
	}
	return 0
}

func unsafeCastTypeNodeIsNullableFlat(file *scanner.File, idx uint32) bool {
	if idx == 0 {
		return false
	}
	if file.FlatType(idx) == "nullable_type" {
		return true
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) && unsafeCastTypeNodeIsNullableFlat(file, child) {
			return true
		}
	}
	return false
}

func unsafeCastIsNullLiteralFlat(file *scanner.File, idx uint32) bool {
	idx = unsafeCastUnwrapExpressionFlat(file, idx)
	return idx != 0 && file.FlatNodeTextEquals(idx, "null")
}

var unsafeCastNeverSucceedsDiagnosticFactories = map[string]bool{
	"CAST_NEVER_SUCCEEDS": true,
	"CastNeverSucceeds":   true,
}

var unsafeCastKnownFinalTypeKeys = map[string]bool{
	"Boolean":        true,
	"Byte":           true,
	"Char":           true,
	"Double":         true,
	"Float":          true,
	"Int":            true,
	"Long":           true,
	"Short":          true,
	"String":         true,
	"Unit":           true,
	"kotlin.Boolean": true,
	"kotlin.Byte":    true,
	"kotlin.Char":    true,
	"kotlin.Double":  true,
	"kotlin.Float":   true,
	"kotlin.Int":     true,
	"kotlin.Long":    true,
	"kotlin.Short":   true,
	"kotlin.String":  true,
	"kotlin.Unit":    true,
}

var unsafeCastKnownSubtypeKeys = map[string][]string{
	"kotlin.Byte":   {"kotlin.Number"},
	"kotlin.Short":  {"kotlin.Number"},
	"kotlin.Int":    {"kotlin.Number"},
	"kotlin.Long":   {"kotlin.Number"},
	"kotlin.Float":  {"kotlin.Number"},
	"kotlin.Double": {"kotlin.Number"},
	"kotlin.String": {"kotlin.CharSequence", "kotlin.Comparable"},
}

func unsafeCastHasNeverSucceedsDiagnosticFlat(ctx *v2.Context, idx uint32) (oracle.OracleDiagnostic, bool) {
	if ctx == nil || ctx.File == nil || ctx.Resolver == nil {
		return oracle.OracleDiagnostic{}, false
	}
	cr, ok := ctx.Resolver.(*oracle.CompositeResolver)
	if !ok {
		return oracle.OracleDiagnostic{}, false
	}
	for _, d := range cr.Oracle().LookupDiagnostics(ctx.File.Path) {
		if !unsafeCastNeverSucceedsDiagnosticFactories[d.FactoryName] {
			continue
		}
		if unsafeCastDiagnosticOverlapsFlat(ctx.File, idx, d) {
			return d, true
		}
	}
	return oracle.OracleDiagnostic{}, false
}

func unsafeCastDiagnosticOverlapsFlat(file *scanner.File, idx uint32, d oracle.OracleDiagnostic) bool {
	if file == nil || idx == 0 || d.Line <= 0 || d.Col <= 0 {
		return false
	}
	off := file.LineOffset(d.Line-1) + d.Col - 1
	return off >= int(file.FlatStartByte(idx)) && off < int(file.FlatEndByte(idx))
}

func unsafeCastLocalNeverSucceedsFlat(ctx *v2.Context, cast unsafeCastExpressionParts) bool {
	if ctx == nil || ctx.File == nil {
		return false
	}
	if unsafeCastIsNullLiteralFlat(ctx.File, cast.source) {
		return !cast.safe
	}
	if !unsafeCastSourceEligibleForLocalNeverSucceedsFlat(ctx.File, cast.source) {
		return false
	}
	if ctx.Resolver == nil {
		return false
	}
	sourceType := ctx.Resolver.ResolveFlatNode(cast.source, ctx.File)
	targetType := ctx.Resolver.ResolveFlatNode(cast.target, ctx.File)
	if !unsafeCastResolvedTypeKnown(sourceType) || !unsafeCastResolvedTypeKnown(targetType) {
		return false
	}
	if unsafeCastTypesRelated(ctx.Resolver, sourceType, targetType) {
		return false
	}
	return unsafeCastTypeKnownFinal(ctx.Resolver, sourceType) &&
		unsafeCastTypeKnownFinal(ctx.Resolver, targetType)
}

func unsafeCastSourceEligibleForLocalNeverSucceedsFlat(file *scanner.File, source uint32) bool {
	source = unsafeCastUnwrapExpressionFlat(file, source)
	if source == 0 {
		return false
	}
	switch file.FlatType(source) {
	case "simple_identifier",
		"integer_literal",
		"real_literal",
		"boolean_literal",
		"character_literal",
		"line_string_literal",
		"multi_line_string_literal",
		"string_literal":
		return true
	default:
		return false
	}
}

func unsafeCastResolvedTypeKnown(t *typeinfer.ResolvedType) bool {
	if t == nil || t.Kind == typeinfer.TypeUnknown {
		return false
	}
	key := unsafeCastTypeKey(t)
	return key != "" && key != "Any" && key != "kotlin.Any" &&
		key != "java.lang.Object" && key != "Nothing" && key != "kotlin.Nothing"
}

func unsafeCastTypesRelated(resolver typeinfer.TypeResolver, a, b *typeinfer.ResolvedType) bool {
	return unsafeCastIsSubtypeOf(resolver, a, b) || unsafeCastIsSubtypeOf(resolver, b, a)
}

func unsafeCastIsSubtypeOf(resolver typeinfer.TypeResolver, sub, sup *typeinfer.ResolvedType) bool {
	if sub == nil || sup == nil {
		return false
	}
	subKey := unsafeCastTypeKey(sub)
	supKey := unsafeCastTypeKey(sup)
	if subKey == "" || supKey == "" {
		return false
	}
	if subKey == supKey || sub.Name == sup.Name || sub.FQN == sup.FQN {
		return true
	}
	if sup.FQN != "" && sub.IsSubtypeOf(sup.FQN) {
		return true
	}
	if sup.Name != "" && sub.IsSubtypeOf(sup.Name) {
		return true
	}
	if typeinfer.IsKnownSubtype(subKey, supKey) {
		return true
	}
	for _, knownSuper := range unsafeCastKnownSubtypeKeys[subKey] {
		if knownSuper == supKey || simpleTypeName(knownSuper) == sup.Name {
			return true
		}
	}
	if info := unsafeCastClassInfoForType(resolver, sub); info != nil {
		for _, st := range info.Supertypes {
			if st == supKey || simpleTypeName(st) == sup.Name || st == sup.Name {
				return true
			}
		}
	}
	return false
}

func unsafeCastTypeKnownFinal(resolver typeinfer.TypeResolver, t *typeinfer.ResolvedType) bool {
	key := unsafeCastTypeKey(t)
	if unsafeCastKnownFinalTypeKeys[key] {
		return true
	}
	info := unsafeCastClassInfoForType(resolver, t)
	if info == nil {
		return false
	}
	kind := strings.ToLower(info.Kind)
	if strings.Contains(kind, "interface") {
		return false
	}
	if strings.Contains(kind, "enum") || kind == "object" {
		return true
	}
	if info.IsOpen || info.IsAbstract || info.IsSealed {
		return false
	}
	// Kotlin source classes are final by default. External classes from the
	// built-in framework tables are not treated as final unless listed above.
	return info.File != ""
}

func unsafeCastClassInfoForType(resolver typeinfer.TypeResolver, t *typeinfer.ResolvedType) *typeinfer.ClassInfo {
	if resolver == nil || t == nil {
		return nil
	}
	for _, name := range []string{t.FQN, t.Name} {
		if name == "" {
			continue
		}
		if info := resolver.ClassHierarchy(name); info != nil {
			return info
		}
	}
	return nil
}

func unsafeCastTypeKey(t *typeinfer.ResolvedType) string {
	if t == nil {
		return ""
	}
	key := strings.TrimSpace(t.FQN)
	if key == "" {
		key = strings.TrimSpace(t.Name)
	}
	if mapped := typeinfer.MapJavaToKotlin(key); mapped != "" {
		key = mapped
	}
	return key
}

func unsafeCastFindingMessage(cast unsafeCastExpressionParts, d oracle.OracleDiagnostic) string {
	if d.Message != "" {
		return "Cast can never succeed: " + d.Message
	}
	if cast.safe {
		return "Safe cast can never succeed and always returns null."
	}
	return "Cast can never succeed."
}

func unsafeCastTypesCompatible(exprType, targetType *typeinfer.ResolvedType) bool {
	if exprType == nil || targetType == nil ||
		exprType.Kind == typeinfer.TypeUnknown || targetType.Kind == typeinfer.TypeUnknown {
		return false
	}
	if exprType.FQN != "" && targetType.FQN != "" && exprType.FQN == targetType.FQN {
		return true
	}
	if exprType.Name != "" && targetType.Name != "" && exprType.Name == targetType.Name {
		return true
	}
	if targetType.FQN != "" && exprType.IsSubtypeOf(targetType.FQN) {
		return true
	}
	if targetType.Name != "" && exprType.IsSubtypeOf(targetType.Name) {
		return true
	}
	return false
}

func unsafeCastKnownPlatformCastFlat(ctx *v2.Context, source, target uint32, targetName string) bool {
	if ctx == nil || ctx.File == nil {
		return false
	}
	file := ctx.File
	source = unsafeCastUnwrapExpressionFlat(file, source)
	if source == 0 || file.FlatType(source) != "call_expression" {
		return false
	}
	callee := flatCallExpressionName(file, source)
	switch callee {
	case "getSystemService":
		return unsafeCastGetSystemServiceMatchesFlat(ctx, source, target, targetName)
	case "findViewById", "requireViewById":
		return unsafeCastPlatformCallConfirmedFlat(ctx, source, callee, unsafeCastReceiverIsViewLookupHost) &&
			unsafeCastTargetIsAndroidViewLikeFlat(ctx.Resolver, file, target, targetName)
	case "findFragmentById", "findFragmentByTag":
		return unsafeCastPlatformCallConfirmedFlat(ctx, source, callee, unsafeCastReceiverIsFragmentManagerLike) &&
			unsafeCastTargetIsFragmentLikeFlat(ctx.Resolver, file, target, targetName)
	default:
		return false
	}
}

func unsafeCastGetSystemServiceMatchesFlat(ctx *v2.Context, call, target uint32, targetName string) bool {
	file := ctx.File
	if !unsafeCastPlatformCallConfirmedFlat(ctx, call, "getSystemService", unsafeCastReceiverIsContextLike) {
		return false
	}
	serviceName := unsafeCastFirstArgumentLastIdentifierFlat(file, call)
	expectedType := serviceCastMap[serviceName]
	if expectedType == "" {
		return false
	}
	targetType := unsafeCastResolveTypeFlat(ctx.Resolver, file, target)
	if unsafeCastTypeMatchesSimpleName(targetType, targetName, expectedType) {
		return true
	}
	return isClipboardServiceType(expectedType) && unsafeCastTargetIsClipboardLike(targetType, targetName)
}

func unsafeCastPlatformCallConfirmedFlat(ctx *v2.Context, call uint32, callee string, receiverOK func(*typeinfer.ResolvedType) bool) bool {
	file := ctx.File
	if target := unsafeCastOracleCallTargetFlat(ctx, call); target != "" {
		if unsafeCastCallTargetMatches(target, callee) && unsafeCastCallTargetIsPlatform(target) {
			return true
		}
		if unsafeCastCallTargetLooksResolved(target) {
			return false
		}
	}
	receiver := unsafeCastCallReceiverFlat(file, call)
	if receiver == 0 || ctx.Resolver == nil {
		return unsafeCastEnclosingOwnerMatchesFlat(ctx, call, receiverOK)
	}
	receiverType := ctx.Resolver.ResolveFlatNode(receiver, file)
	return receiverOK(receiverType) || unsafeCastEnclosingOwnerMatchesFlat(ctx, call, receiverOK)
}

func unsafeCastEnclosingOwnerMatchesFlat(ctx *v2.Context, idx uint32, receiverOK func(*typeinfer.ResolvedType) bool) bool {
	if ctx == nil || ctx.Resolver == nil || ctx.File == nil {
		return false
	}
	for owner, ok := ctx.File.FlatParent(idx); ok; owner, ok = ctx.File.FlatParent(owner) {
		if ctx.File.FlatType(owner) != "class_declaration" {
			continue
		}
		name := extractIdentifierFlat(ctx.File, owner)
		if name == "" {
			return false
		}
		info := ctx.Resolver.ClassHierarchy(name)
		if info == nil {
			info = ctx.Resolver.ClassHierarchy(ctx.Resolver.ResolveImport(name, ctx.File))
		}
		if info == nil {
			return false
		}
		return receiverOK(&typeinfer.ResolvedType{
			Name:       info.Name,
			FQN:        info.FQN,
			Kind:       typeinfer.TypeClass,
			Supertypes: info.Supertypes,
		})
	}
	return false
}

func unsafeCastOracleCallTargetFlat(ctx *v2.Context, idx uint32) string {
	if ctx == nil || ctx.Resolver == nil || ctx.File == nil {
		return ""
	}
	var oracleLookup oracle.Lookup
	if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
		oracleLookup = cr.Oracle()
	}
	if oracleLookup == nil {
		return ""
	}
	return oracleLookup.LookupCallTarget(ctx.File.Path, ctx.File.FlatRow(idx)+1, ctx.File.FlatCol(idx)+1)
}

func unsafeCastCallTargetMatches(callTarget, callee string) bool {
	if callTarget == callee {
		return true
	}
	return strings.HasSuffix(callTarget, "."+callee) ||
		strings.HasSuffix(callTarget, "#"+callee)
}

func unsafeCastCallTargetIsPlatform(callTarget string) bool {
	return strings.HasPrefix(callTarget, "android.") ||
		strings.HasPrefix(callTarget, "androidx.") ||
		strings.HasPrefix(callTarget, "com.google.android.material.")
}

func unsafeCastCallTargetLooksResolved(callTarget string) bool {
	return strings.Contains(callTarget, ".") || strings.Contains(callTarget, "#")
}

func unsafeCastCallReceiverFlat(file *scanner.File, call uint32) uint32 {
	nav, _ := flatCallExpressionParts(file, call)
	if nav == 0 || file.FlatType(nav) != "navigation_expression" {
		return 0
	}
	for child := file.FlatFirstChild(nav); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) && file.FlatType(child) != "navigation_suffix" {
			return child
		}
	}
	return 0
}

func unsafeCastFirstArgumentLastIdentifierFlat(file *scanner.File, call uint32) string {
	args := flatCallKeyArguments(file, call)
	arg := flatPositionalValueArgument(file, args, 0)
	if arg == 0 {
		return ""
	}
	return unsafeCastLastReferenceIdentifierFlat(file, arg)
}

func unsafeCastLastReferenceIdentifierFlat(file *scanner.File, idx uint32) string {
	idx = unsafeCastUnwrapExpressionFlat(file, idx)
	switch file.FlatType(idx) {
	case "simple_identifier", "type_identifier":
		return file.FlatNodeString(idx, nil)
	case "navigation_expression":
		return flatNavigationExpressionLastIdentifier(file, idx)
	case "value_argument":
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				return unsafeCastLastReferenceIdentifierFlat(file, child)
			}
		}
	case "call_expression":
		return flatCallExpressionName(file, idx)
	}
	last := ""
	file.FlatWalkAllNodes(idx, func(n uint32) {
		switch file.FlatType(n) {
		case "simple_identifier", "type_identifier":
			last = file.FlatNodeString(n, nil)
		}
	})
	return last
}

func unsafeCastResolveTypeFlat(resolver typeinfer.TypeResolver, file *scanner.File, idx uint32) *typeinfer.ResolvedType {
	if resolver == nil || idx == 0 {
		return nil
	}
	return resolver.ResolveFlatNode(idx, file)
}

func unsafeCastReceiverIsContextLike(t *typeinfer.ResolvedType) bool {
	return unsafeCastTypeMatchesAny(t,
		"android.content.Context", "Context",
		"android.app.Activity", "Activity",
		"android.app.Application", "Application",
		"android.app.Service", "Service",
	)
}

func unsafeCastReceiverIsViewLookupHost(t *typeinfer.ResolvedType) bool {
	return unsafeCastTypeMatchesAny(t,
		"android.app.Activity", "Activity",
		"android.app.Dialog", "Dialog",
		"android.view.View", "View",
	)
}

func unsafeCastReceiverIsFragmentManagerLike(t *typeinfer.ResolvedType) bool {
	return unsafeCastTypeMatchesAny(t,
		"android.app.FragmentManager", "FragmentManager",
		"androidx.fragment.app.FragmentManager", "FragmentManager",
	)
}

func unsafeCastTypeMatchesAny(t *typeinfer.ResolvedType, names ...string) bool {
	if t == nil || t.Kind == typeinfer.TypeUnknown {
		return false
	}
	for _, name := range names {
		if t.Name == name || t.FQN == name || t.IsSubtypeOf(name) {
			return true
		}
	}
	return false
}

func unsafeCastTargetIsAndroidViewLikeFlat(resolver typeinfer.TypeResolver, file *scanner.File, target uint32, targetName string) bool {
	t := unsafeCastResolveTypeFlat(resolver, file, target)
	if unsafeCastTypeMatchesSimpleName(t, targetName, "View") ||
		unsafeCastTypeHasFQNPrefix(t, "android.view.", "android.widget.", "android.webkit.", "androidx.recyclerview.widget.", "com.google.android.material.") {
		return true
	}
	return unsafeCastHierarchyContainsFlat(resolver, t, targetName, "android.view.View", "View")
}

func unsafeCastTargetIsFragmentLikeFlat(resolver typeinfer.TypeResolver, file *scanner.File, target uint32, targetName string) bool {
	t := unsafeCastResolveTypeFlat(resolver, file, target)
	if unsafeCastTypeMatchesSimpleName(t, targetName, "Fragment") ||
		unsafeCastTypeHasFQNPrefix(t, "android.app.", "androidx.fragment.app.") {
		return true
	}
	return unsafeCastHierarchyContainsFlat(resolver, t, targetName, "android.app.Fragment", "androidx.fragment.app.Fragment", "Fragment")
}

func unsafeCastTypeMatchesSimpleName(t *typeinfer.ResolvedType, targetName, want string) bool {
	if simpleTypeName(targetName) == want {
		return true
	}
	if t == nil || t.Kind == typeinfer.TypeUnknown {
		return false
	}
	return t.Name == want || strings.HasSuffix(t.FQN, "."+want)
}

func unsafeCastTypeHasFQNPrefix(t *typeinfer.ResolvedType, prefixes ...string) bool {
	if t == nil || t.Kind == typeinfer.TypeUnknown || t.FQN == "" {
		return false
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(t.FQN, prefix) {
			return true
		}
	}
	return false
}

func unsafeCastHierarchyContainsFlat(resolver typeinfer.TypeResolver, t *typeinfer.ResolvedType, targetName string, supers ...string) bool {
	if resolver == nil {
		return false
	}
	candidates := []string{targetName}
	if t != nil {
		candidates = append(candidates, t.Name, t.FQN)
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		info := resolver.ClassHierarchy(candidate)
		if info == nil {
			continue
		}
		for _, supertype := range info.Supertypes {
			for _, want := range supers {
				if supertype == want || strings.HasSuffix(supertype, "."+want) {
					return true
				}
			}
		}
	}
	return false
}

func isClipboardServiceType(name string) bool {
	return name == "ClipboardManager" ||
		name == "android.content.ClipboardManager" ||
		name == "android.text.ClipboardManager"
}

func unsafeCastTargetIsClipboardLike(t *typeinfer.ResolvedType, targetName string) bool {
	return isClipboardServiceType(targetName) ||
		isClipboardServiceType(simpleTypeName(targetName)) ||
		(t != nil && (isClipboardServiceType(t.Name) || isClipboardServiceType(t.FQN)))
}

func unsafeCastTargetResolvableFlat(file *scanner.File, target uint32, targetName string, resolver typeinfer.TypeResolver) bool {
	if targetName == "" {
		return false
	}
	if _, ok := typeinfer.PrimitiveTypes[simpleTypeName(targetName)]; ok {
		return true
	}
	if t := unsafeCastResolveTypeFlat(resolver, file, target); t != nil && t.Kind != typeinfer.TypeUnknown {
		return true
	}
	return unsafeCastSameFileDeclaresTypeFlat(file, targetName)
}

func unsafeCastSameFileDeclaresTypeFlat(file *scanner.File, targetName string) bool {
	want := simpleTypeName(targetName)
	if want == "" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if found {
			return
		}
		switch file.FlatType(idx) {
		case "class_declaration", "object_declaration":
			found = extractIdentifierFlat(file, idx) == want
		}
	})
	return found
}

func unsafeCastSourceClearlyLocalFlat(file *scanner.File, source uint32) bool {
	source = unsafeCastUnwrapExpressionFlat(file, source)
	if source == 0 {
		return false
	}
	switch file.FlatType(source) {
	case "simple_identifier":
		return unsafeCastNameDeclaredBeforeFlat(file, source, file.FlatNodeString(source, nil))
	case "call_expression":
		name := flatCallExpressionName(file, source)
		return name != "" && unsafeCastFunctionDeclaredBeforeFlat(file, source, name)
	default:
		return false
	}
}

func unsafeCastNameDeclaredBeforeFlat(file *scanner.File, idx uint32, name string) bool {
	if name == "" {
		return false
	}
	for owner, ok := file.FlatParent(idx); ok; owner, ok = file.FlatParent(owner) {
		switch file.FlatType(owner) {
		case "function_declaration", "class_declaration", "object_declaration", "source_file":
			found := false
			file.FlatWalkAllNodes(owner, func(n uint32) {
				if found || file.FlatStartByte(n) > file.FlatStartByte(idx) {
					return
				}
				switch file.FlatType(n) {
				case "parameter", "class_parameter", "property_declaration", "variable_declaration":
					found = extractIdentifierFlat(file, n) == name
				}
			})
			if found {
				return true
			}
		}
		if file.FlatType(owner) == "source_file" {
			break
		}
	}
	return false
}

func unsafeCastFunctionDeclaredBeforeFlat(file *scanner.File, idx uint32, name string) bool {
	found := false
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		if found || file.FlatStartByte(fn) > file.FlatStartByte(idx) {
			return
		}
		found = extractIdentifierFlat(file, fn) == name
	})
	return found
}

func (r *UnsafeCastRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	// Skip .gradle.kts files — Gradle's extra properties and configuration
	// delegates commonly require `as` casts with no safer alternative.
	if strings.HasSuffix(file.Path, ".gradle.kts") {
		return
	}
	// Skip test files — test code commonly uses `as` to cast framework
	// return values to concrete types; failure = test failure anyway.
	if isTestFile(file.Path) {
		return
	}
	// Skip @Preview / sample / fixture functions — these build hand-crafted
	// test data and casts are part of the scaffolding.
	if isInsidePreviewOrSampleFunctionFlat(file, idx) {
		return
	}
	cast, ok := unsafeCastExpressionPartsFlat(file, idx)
	if !ok {
		return
	}

	castVar := unsafeCastComparableExpressionFlat(file, cast.source)
	castTarget := unsafeCastTypeNameFlat(file, cast.target)
	// Casts to nullable target syntax (`as T?`) stay owned by
	// CastToNullableType. A safe cast (`as? T`) still reaches this rule when
	// the target syntax itself is non-nullable and the cast can never match.
	if unsafeCastTypeNodeIsNullableFlat(file, cast.target) {
		return
	}
	if d, ok := unsafeCastHasNeverSucceedsDiagnosticFlat(ctx, idx); ok {
		f := r.Finding(file, d.Line, d.Col, unsafeCastFindingMessage(cast, d))
		f.Confidence = 0.95
		if !cast.safe {
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(cast.op)),
				EndByte:     int(file.FlatEndByte(cast.op)),
				Replacement: "as?",
			}
		}
		ctx.Emit(f)
		return
	}
	// Skip `other as ThisClass` inside an `equals(other: Any?)` method,
	// which is the IntelliJ-generated pattern safeguarded by a prior
	// class check.
	if castVar == "other" && isInsideEqualsMethodFlat(file, idx) {
		return
	}
	// Skip single-letter type parameter targets (e.g., `as T`) — these are
	// ViewModel.Factory.create, Bundle serialization helpers, and other
	// type-erasure boundary casts.
	if len(castTarget) == 1 && castTarget[0] >= 'A' && castTarget[0] <= 'Z' {
		return
	}
	// Skip `expr as Any` — every Kotlin reference type is a subtype of Any,
	// so the cast can never fail at runtime. Authors usually add this to
	// force a type for reflection/interop call boundaries.
	if castTarget == "Any" {
		return
	}
	if unsafeCastKnownPlatformCastFlat(ctx, cast.source, cast.target, castTarget) {
		return
	}
	// Skip casts inside a `when` branch whose subject is the same variable
	// being cast's discriminator — the branch condition has guaranteed the
	// target shape already. Common idiom for annotation/value deserializers:
	//     when (name) { "x" -> foo = value as String; "y" -> ... }
	if isInsideWhenBranchWithStringSubjectFlat(file, idx) {
		return
	}
	// Skip empty-lambda SAM conversion: `{} as OnEventListener` — the
	// Kotlin compiler generates a valid SAM instance; the `as` is a
	// workaround for default-parameter SAM inference limitations.
	if file.FlatType(unsafeCastUnwrapExpressionFlat(file, cast.source)) == "lambda_literal" {
		return
	}

	// 1. Resolver-based check: the expression type already matches the target
	//    (e.g., via is-check smart cast or declaration type).
	if ctx.Resolver != nil {
		exprType := ctx.Resolver.ResolveFlatNode(cast.source, file)
		targetType := ctx.Resolver.ResolveFlatNode(cast.target, file)
		if unsafeCastTypesCompatible(exprType, targetType) {
			return
		}
	}

	// 2. AST-based fallback: walk up to find an enclosing if/when that has
	//    an is-check for the same variable and target type.
	if unsafeCastGuardedByIsCheckFlat(file, idx, castVar, castTarget) {
		return
	}
	// 3. Predicate-based guard: casts to sealed-hierarchy types like
	//    `record as MmsMessageRecord` are often gated by a boolean helper
	//    (`isMms`, `isMediaMessage()`) rather than an `is`-check. Accept a
	//    small allow-list of type+predicate pairs that are ubiquitous in
	//    Android/Signal code.
	if isCastGuardedByTypePredicateFlat(file, idx, castVar, castTarget) {
		return
	}
	if !unsafeCastLocalNeverSucceedsFlat(ctx, cast) {
		return
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		unsafeCastFindingMessage(cast, oracle.OracleDiagnostic{}))
	f.Confidence = 0.95
	if !cast.safe {
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(cast.op)),
			EndByte:     int(file.FlatEndByte(cast.op)),
			Replacement: "as?",
		}
	}
	ctx.Emit(f)
}

// castTypePredicates maps target types to boolean predicates that, when
// proven true in an enclosing if/&& chain, guarantee the cast is safe.
// These are hierarchy invariants rather than smart-cast semantics.
var castTypePredicates = map[string][]string{
	"MmsMessageRecord": {
		"isMms", "isMediaMessage", "hasSharedContact", "hasLinkPreview",
		"hasSticker", "hasLocation", "hasAudio", "hasThumbnail",
		"isStoryReaction", "isStory", "hasGiftBadge", "isViewOnceMessage",
		"hasAttachment", "hasMediaMessage", "isMediaPendingMessage",
	},
	"MmsSmsDatabase": {"isMms"},
	// Recipient / group hierarchy
	"GroupRecord":               {"isGroup", "isPushGroup"},
	"GroupId.V2":                {"isV2"},
	"GroupId.V1":                {"isV1"},
	"GroupId.Mms":               {"isMms"},
	"V2":                        {"isV2"},
	"V1":                        {"isV1"},
	"GroupReply":                {"isGroupReply"},
	"RotatableGradientDrawable": {"isGradient"},
	"StoryTextPostModel":        {"isTextStory"},
	"TextStory":                 {"isTextStory"},
}

// unsafeCastIsCheckPartsFlat parses a check_expression node into its components.
func unsafeCastIsCheckPartsFlat(file *scanner.File, node uint32) (expr uint32, typeName string, positive bool, ok bool) {
	if file.FlatType(node) != "check_expression" {
		return 0, "", false, false
	}
	positive = true
	seenOp := false
	var typeNode uint32
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "!is":
			positive = false
			seenOp = true
		case "is":
			positive = true
			seenOp = true
		default:
			if !file.FlatIsNamed(child) {
				continue
			}
			if !seenOp && expr == 0 {
				expr = child
			} else if seenOp && typeNode == 0 {
				typeNode = child
			}
		}
	}
	if !seenOp || expr == 0 || typeNode == 0 {
		return 0, "", false, false
	}
	typeName = unsafeCastTypeNameFlat(file, typeNode)
	return expr, typeName, positive, typeName != ""
}

// unsafeCastConditionHasIsCheckFlat walks the condition subtree looking for a
// check_expression that matches castVar is/!is castTarget. Uses AST nodes only,
// so it never matches text inside comments or string literals.
func unsafeCastConditionHasIsCheckFlat(file *scanner.File, cond uint32, castVar, castTarget string, wantPositive bool) bool {
	found := false
	file.FlatWalkAllNodes(cond, func(n uint32) {
		if found {
			return
		}
		expr, typeName, positive, ok := unsafeCastIsCheckPartsFlat(file, n)
		if !ok || positive != wantPositive || typeName != castTarget {
			return
		}
		if strings.TrimSpace(file.FlatNodeText(expr)) == castVar {
			found = true
		}
	})
	return found
}

// unsafeCastConditionHasPredicateCallFlat walks the condition subtree looking
// for a call_expression whose callee name is in predicates. Only matches real
// call nodes, never text inside comments or string literals.
func unsafeCastConditionHasPredicateCallFlat(file *scanner.File, cond uint32, predicates []string) bool {
	found := false
	file.FlatWalkAllNodes(cond, func(n uint32) {
		if found || file.FlatType(n) != "call_expression" {
			return
		}
		callee := flatCallExpressionName(file, n)
		for _, pred := range predicates {
			if callee == pred {
				found = true
				return
			}
		}
	})
	return found
}

// unsafeCastExtractIfPartsFlat returns the condition node, then-body, and
// else-body of an if_expression using the AST structure.
func unsafeCastExtractIfPartsFlat(file *scanner.File, ifNode uint32) (cond, thenBody, elseBody uint32) {
	foundElse := false
	for child := file.FlatFirstChild(ifNode); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "control_structure_body":
			if !foundElse && thenBody == 0 {
				thenBody = child
			} else if foundElse && elseBody == 0 {
				elseBody = child
			}
		case "else":
			foundElse = true
		default:
			if cond == 0 && file.FlatIsNamed(child) {
				cond = child
			}
		}
	}
	return
}

// unsafeCastFindIfExprFlat extracts an if_expression from a statement node,
// handling the expression_statement wrapper.
func unsafeCastFindIfExprFlat(file *scanner.File, node uint32) uint32 {
	switch file.FlatType(node) {
	case "if_expression":
		return node
	case "expression_statement":
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "if_expression" {
				return child
			}
		}
	}
	return 0
}

// unsafeCastStatementAlwaysExitsFlat reports whether a single AST statement
// unconditionally transfers control (return, throw, continue, break, or an
// if/else where both branches do the same).
func unsafeCastStatementAlwaysExitsFlat(file *scanner.File, node uint32) bool {
	if node == 0 {
		return false
	}
	switch file.FlatType(node) {
	case "jump_expression":
		return true
	case "expression_statement":
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				return unsafeCastStatementAlwaysExitsFlat(file, child)
			}
		}
	case "if_expression":
		_, thenBody, elseBody := unsafeCastExtractIfPartsFlat(file, node)
		if elseBody == 0 {
			return false
		}
		return bodyAlwaysExitsFlat(file, thenBody) && bodyAlwaysExitsFlat(file, elseBody)
	}
	return false
}

func isCastGuardedByTypePredicateFlat(file *scanner.File, idx uint32, castVar, castTarget string) bool {
	predicates, ok := castTypePredicates[castTarget]
	if !ok {
		return false
	}
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "lambda_literal":
			return false
		case "conjunction_expression":
			if unsafeCastConditionHasPredicateCallFlat(file, cur, predicates) {
				return true
			}
		case "if_expression":
			cond, thenBody, _ := unsafeCastExtractIfPartsFlat(file, cur)
			if cond != 0 && thenBody != 0 && isDescendantFlat(file, idx, thenBody) {
				if unsafeCastConditionHasPredicateCallFlat(file, cond, predicates) {
					return true
				}
			}
		case "when_entry":
			if unsafeCastConditionHasPredicateCallFlat(file, cur, predicates) {
				return true
			}
		}
	}
	return false
}

// unsafeCastGuardedByIsCheck walks up the AST from an as_expression to check
// whether an enclosing if-expression or when-entry has an is-check that proves
// the cast is safe. This handles the common pattern:
//
//	if (x is Foo) { val f = x as Foo }       — safe, is-check guards the cast
//	when (x) { is Foo -> x as Foo }           — safe, when branch guards the cast
//
// It also handles early-return patterns like:
//
//	if (x !is Foo) return; val f = x as Foo   — safe, negative is-check + early exit
func unsafeCastGuardedByIsCheckFlat(file *scanner.File, idx uint32, varName, targetType string) bool {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "if_expression":
			if unsafeCastIfGuardsFlat(file, current, idx, varName, targetType) {
				return true
			}
		case "when_entry":
			if unsafeCastWhenEntryGuardsFlat(file, current, varName, targetType) {
				return true
			}
		case "statements", "function_body", "class_body":
			if unsafeCastPrecedingNegativeIsCheckFlat(file, current, idx, varName, targetType) {
				return true
			}
		case "function_declaration", "lambda_literal", "anonymous_function":
			return false
		}
	}
	return false
}

func unsafeCastPrecedingNegativeIsCheckFlat(file *scanner.File, statementsNode, asNode uint32, varName, targetType string) bool {
	asStart := file.FlatStartByte(asNode)
	for i := 0; i < file.FlatChildCount(statementsNode); i++ {
		child := file.FlatChild(statementsNode, i)
		if child == 0 {
			continue
		}
		if file.FlatStartByte(child) >= asStart {
			break
		}
		ifNode := unsafeCastFindIfExprFlat(file, child)
		if ifNode == 0 {
			continue
		}
		cond, thenBody, _ := unsafeCastExtractIfPartsFlat(file, ifNode)
		if cond == 0 || thenBody == 0 {
			continue
		}
		if unsafeCastConditionHasIsCheckFlat(file, cond, varName, targetType, false) &&
			bodyAlwaysExitsFlat(file, thenBody) {
			return true
		}
	}
	return false
}

func unsafeCastIfGuardsFlat(file *scanner.File, ifNode, asNode uint32, varName, targetType string) bool {
	cond, thenBody, elseBody := unsafeCastExtractIfPartsFlat(file, ifNode)
	if cond == 0 {
		return false
	}

	// if (x is T) { ... x as T ... }
	if unsafeCastConditionHasIsCheckFlat(file, cond, varName, targetType, true) {
		if thenBody != 0 && isDescendantFlat(file, asNode, thenBody) {
			return true
		}
	}

	// if (x !is T) { return/throw } then x as T (in else or after the if)
	if unsafeCastConditionHasIsCheckFlat(file, cond, varName, targetType, false) {
		if thenBody != 0 && bodyAlwaysExitsFlat(file, thenBody) {
			if elseBody != 0 && isDescendantFlat(file, asNode, elseBody) {
				return true
			}
			if !isDescendantFlat(file, asNode, ifNode) {
				return true
			}
		}
	}

	return false
}

func unsafeCastWhenEntryGuardsFlat(file *scanner.File, whenEntry uint32, varName, targetType string) bool {
	whenExpr, ok := file.FlatParent(whenEntry)
	if !ok || file.FlatType(whenExpr) != "when_expression" {
		return false
	}
	whenSubject := ""
	for i := 0; i < file.FlatChildCount(whenExpr); i++ {
		child := file.FlatChild(whenExpr, i)
		if file.FlatType(child) == "when_subject" {
			subjectText := strings.TrimSpace(file.FlatNodeText(child))
			if strings.HasPrefix(subjectText, "(") && strings.HasSuffix(subjectText, ")") {
				subjectText = strings.TrimSpace(subjectText[1 : len(subjectText)-1])
			}
			if strings.HasPrefix(subjectText, "val ") {
				if eqIdx := strings.Index(subjectText, "="); eqIdx >= 0 {
					subjectText = strings.TrimSpace(subjectText[eqIdx+1:])
				}
			}
			whenSubject = subjectText
			break
		}
	}
	if whenSubject != varName {
		return false
	}
	for i := 0; i < file.FlatChildCount(whenEntry); i++ {
		child := file.FlatChild(whenEntry, i)
		switch file.FlatType(child) {
		case "when_condition", "type_test":
			condText := strings.TrimSpace(file.FlatNodeText(child))
			if condText == "is "+targetType {
				return true
			}
		}
	}
	return false
}

func isDescendantFlat(file *scanner.File, child, parent uint32) bool {
	for c, ok := file.FlatParent(child); ok; c, ok = file.FlatParent(c) {
		if c == parent {
			return true
		}
	}
	return false
}

func bodyAlwaysExitsFlat(file *scanner.File, body uint32) bool {
	if body == 0 {
		return false
	}
	// Find the last statement: look for a statements block first (braced body),
	// then fall back to the last named child (single-expression body).
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "statements" {
			return unsafeCastStatementAlwaysExitsFlat(file, flatLastNamedChild(file, child))
		}
	}
	return unsafeCastStatementAlwaysExitsFlat(file, flatLastNamedChild(file, body))
}

func isInsideEqualsMethodFlat(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "function_declaration" {
			return extractIdentifierFlat(file, p) == "equals"
		}
	}
	return false
}

func isInsideWhenBranchWithStringSubjectFlat(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "when_expression":
			for i := 0; i < file.FlatNamedChildCount(p); i++ {
				c := file.FlatNamedChild(p, i)
				if c == 0 {
					continue
				}
				ct := file.FlatType(c)
				if ct == "when_subject" ||
					(ct != "when_entry" && ct != "line_comment" && ct != "multiline_comment") {
					return true
				}
				if ct == "when_entry" {
					break
				}
			}
			return false
		case "function_declaration", "lambda_literal", "anonymous_function":
			return false
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// CastNullableToNonNullableTypeRule detects `as Type` on nullable expression.
// ---------------------------------------------------------------------------
type CastNullableToNonNullableTypeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. The rule is
// resolver-backed so it only reports when source nullability is known.
func (r *CastNullableToNonNullableTypeRule) Confidence() float64 { return 0.75 }

func (r *CastNullableToNonNullableTypeRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	cast, ok := unsafeCastExpressionPartsFlat(file, idx)
	if !ok || cast.safe {
		return
	}

	if unsafeCastIsNullLiteralFlat(file, cast.source) {
		return
	}
	if unsafeCastTypeNodeIsNullableFlat(file, cast.target) {
		return
	}
	if ctx.Resolver == nil {
		return
	}
	targetType := ctx.Resolver.ResolveFlatNode(cast.target, file)
	if targetType == nil || targetType.Kind == typeinfer.TypeUnknown {
		return
	}
	if targetType.IsNullable() {
		return
	}
	sourceNullable := ctx.Resolver.IsNullableFlat(cast.source, file)
	if sourceNullable == nil || !*sourceNullable {
		return
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Casting a nullable type to a non-nullable type. Use 'as?' instead.")
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(cast.op)),
		EndByte:     int(file.FlatEndByte(cast.op)),
		Replacement: "as?",
	}
	ctx.Emit(f)
}

// ---------------------------------------------------------------------------
// CastToNullableTypeRule detects `as Type?`.
// ---------------------------------------------------------------------------
type CastToNullableTypeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs null safety rule. Detection leans on structural patterns
// around nullable expressions and has a heuristic fallback when the
// resolver is absent. Classified per roadmap/17.
func (r *CastToNullableTypeRule) Confidence() float64 { return 0.75 }

func (r *CastToNullableTypeRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	text := file.FlatNodeText(idx)
	// Skip safe casts
	if strings.Contains(text, "as?") {
		return
	}
	// Check for `as Type?`
	parts := strings.SplitN(text, " as ", 2)
	if len(parts) == 2 && strings.HasSuffix(strings.TrimSpace(parts[1]), "?") {
		f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			"Cast to nullable type. Use 'as?' for a safe cast instead of 'as Type?'.")
		// Replace "as Type?" with "as? Type"
		typeName := strings.TrimSpace(parts[1])
		typeName = strings.TrimSuffix(typeName, "?")
		fixed := parts[0] + " as? " + typeName
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(idx)),
			EndByte:     int(file.FlatEndByte(idx)),
			Replacement: fixed,
		}
		ctx.Emit(f)
		return
	}
}
