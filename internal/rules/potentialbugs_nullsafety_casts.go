package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
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
	CustomPreviewWildcard bool
	CustomPreviewPrefixes []string
}

// Confidence reports a tier-1 base confidence. The oracle-backed path mirrors
// the Kotlin compiler's CAST_NEVER_SUCCEEDS diagnostic; the non-oracle fallback
// is intentionally limited to final, disjoint types.
func (r *UnsafeCastRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func (r *UnsafeCastRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	// Skip .gradle.kts files — Gradle's extra properties and configuration
	// delegates commonly require `as` casts with no safer alternative.
	if strings.HasSuffix(file.Path, ".gradle.kts") {
		return
	}
	// Skip test files — test code commonly uses `as` to cast framework
	// return values to concrete types; failure = test failure anyway.
	if scanner.IsTestFile(file.Path) {
		return
	}
	// Skip @Preview / sample / fixture functions — these build hand-crafted
	// test data and casts are part of the scaffolding.
	if isInsidePreviewOrSampleFunctionFlatWithConfig(file, idx, customPreviewConfigFromFields(r.CustomPreviewWildcard, r.CustomPreviewPrefixes)) {
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
	if isTypeParameter(castTarget) {
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
	//    Android message-model code.
	if isCastGuardedByTypePredicateFlat(file, idx, castVar, castTarget) {
		return
	}
	if !unsafeCastLocalNeverSucceedsFlat(ctx, cast) {
		return
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		unsafeCastFindingMessage(cast, oracle.Diagnostic{}))
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
func (r *CastNullableToNonNullableTypeRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *CastNullableToNonNullableTypeRule) check(ctx *api.Context) {
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
	if targetType == nil || !targetType.Resolved || targetType.Kind == typeinfer.TypeUnknown {
		return
	}
	if targetType.IsNullable() {
		return
	}
	if !castNullableToNonNullableSourceIsNullable(ctx, cast.source) {
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

// castNullableToNonNullableSourceIsNullable consults the oracle's
// expression-type fact at source, falling back to the resolver.
//
// It resolves the operand by BYTE RANGE (ResolveFlatNode → oracle
// lookupRangeFact), not line/column: the FIR backend reports columns that
// don't line up with tree-sitter's, so a line/col probe usually misses and
// silently falls back to source inference. The byte-range path also reads
// the SMART-CAST-refined fact (emitted by krit-fir's OracleSmartCastChecker),
// so `o as T` after `if (o == null) return` correctly sees `o` as non-null
// and the cast is not flagged.
func castNullableToNonNullableSourceIsNullable(ctx *api.Context, source uint32) bool {
	if resolved := ctx.Resolver.ResolveFlatNode(source, ctx.File); resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
		return resolved.IsNullable()
	}
	nullable := ctx.Resolver.IsNullableFlat(source, ctx.File)
	return nullable != nil && *nullable
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
func (r *CastToNullableTypeRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *CastToNullableTypeRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	cast, ok := unsafeCastExpressionPartsFlat(file, idx)
	if !ok || cast.safe || file.FlatType(cast.target) != "nullable_type" {
		return
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Cast to nullable type. Use 'as?' for a safe cast instead of 'as Type?'.")
	typeName := strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(cast.target)), "?")
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(idx)),
		EndByte:     int(file.FlatEndByte(idx)),
		Replacement: strings.TrimSpace(file.FlatNodeText(cast.source)) + " as? " + typeName,
	}
	ctx.Emit(f)
}
