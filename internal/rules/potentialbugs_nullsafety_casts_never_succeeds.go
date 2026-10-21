package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

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

func unsafeCastHasNeverSucceedsDiagnosticFlat(ctx *api.Context, idx uint32) (oracle.Diagnostic, bool) {
	if ctx.File == nil || ctx.Resolver == nil {
		return oracle.Diagnostic{}, false
	}
	cr, ok := ctx.Resolver.(*oracle.CompositeResolver)
	if !ok {
		return oracle.Diagnostic{}, false
	}
	for _, d := range oracleLookupDiagnosticsForFlatRange(cr.Oracle(), ctx.File, idx) {
		if !unsafeCastNeverSucceedsDiagnosticFactories[d.FactoryName] {
			continue
		}
		// Anchor to the tightest as_expression rather than any overlapping
		// one: for a nested cast like `(x as A) as B`, a CAST_NEVER_SUCCEEDS
		// the compiler emitted on the inner cast overlaps the outer node's
		// range, and a plain overlap check would wrongly attribute it (and its
		// deleting fix) to the outer cast.
		if diagnosticAnchoredAtNode(ctx.File, idx, d) {
			return d, true
		}
	}
	return oracle.Diagnostic{}, false
}

func unsafeCastLocalNeverSucceedsFlat(ctx *api.Context, cast unsafeCastExpressionParts) bool {
	if ctx.File == nil {
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

func unsafeCastFindingMessage(cast unsafeCastExpressionParts, d oracle.Diagnostic) string {
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
