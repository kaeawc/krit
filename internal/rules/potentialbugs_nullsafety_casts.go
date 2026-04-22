package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ---------------------------------------------------------------------------
// UnsafeCastRule detects `as Type` (non-safe casts).
// Suppresses findings when:
// 1. The resolver shows the expression type already matches the target (smart cast).
// 2. The cast is guarded by an enclosing is-check (if/when) for the same variable and type.
// ---------------------------------------------------------------------------
type UnsafeCastRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Without the
// resolver this rule flags every bare `as` cast and then removes the
// ones that match a hardcoded allow-list of idiomatic Android and
// framework patterns (findViewById, getSystemService, itemView, etc.).
// Even with the resolver the fallback path remains heuristic for
// project-specific wrapper APIs. Pair with SafeCast, which targets
// the same locations from a different angle.
func (r *UnsafeCastRule) Confidence() float64 { return 0.75 }

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
	if !ok || cast.safe {
		return
	}

	castVar := unsafeCastComparableExpressionFlat(file, cast.source)
	castTarget := unsafeCastTypeNameFlat(file, cast.target)
	// Skip `null as Type?` — cast of null literal for overload resolution.
	if unsafeCastIsNullLiteralFlat(file, cast.source) {
		return
	}
	// Skip casts to nullable types `as T?` — they can never throw
	// ClassCastException on null input and are often used for generic
	// variance widening.
	if unsafeCastTypeNodeIsNullableFlat(file, cast.target) {
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

	confidence := 0.75
	if !unsafeCastTargetResolvableFlat(file, cast.target, castTarget, ctx.Resolver) {
		if !unsafeCastSourceClearlyLocalFlat(file, cast.source) {
			return
		}
		confidence = 0.6
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Unsafe cast. Use 'as?' for a safe cast instead.")
	f.Confidence = confidence
	// Find the "as" keyword child node in the as_expression AST node
	// and replace it with "as?" using its precise byte range.
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatNodeTextEquals(child, "as") {
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(child)),
				EndByte:     int(file.FlatEndByte(child)),
				Replacement: "as?",
			}
			break
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
			condText := file.FlatNodeText(cur)
			for _, pred := range predicates {
				if strings.Contains(condText, pred) {
					return true
				}
			}
		case "if_expression":
			for i := 0; i < file.FlatChildCount(cur); i++ {
				c := file.FlatChild(cur, i)
				if c == 0 {
					continue
				}
				switch file.FlatType(c) {
				case "parenthesized_expression", "check_expression",
					"conjunction_expression", "disjunction_expression",
					"equality_expression", "comparison_expression":
					condText := file.FlatNodeText(c)
					for _, pred := range predicates {
						if strings.Contains(condText, pred) {
							return true
						}
					}
				}
			}
		case "when_entry":
			condText := file.FlatNodeText(cur)
			for _, pred := range predicates {
				if strings.Contains(condText, pred) {
					return true
				}
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
	negPattern := varName + " !is " + targetType
	for i := 0; i < file.FlatChildCount(statementsNode); i++ {
		child := file.FlatChild(statementsNode, i)
		if child == 0 {
			continue
		}
		if file.FlatStartByte(child) >= asStart {
			break
		}
		childType := file.FlatType(child)
		if childType == "if_expression" || childType == "expression_statement" {
			ifText := file.FlatNodeText(child)
			if strings.Contains(ifText, negPattern) && hasEarlyExitKeyword(ifText) {
				return true
			}
		}
	}
	return false
}

func unsafeCastIfGuardsFlat(file *scanner.File, ifNode, asNode uint32, varName, targetType string) bool {
	condText := ""
	var thenBody uint32
	var elseBody uint32
	for i := 0; i < file.FlatChildCount(ifNode); i++ {
		child := file.FlatChild(ifNode, i)
		switch file.FlatType(child) {
		case "parenthesized_expression":
			condText = file.FlatNodeText(child)
		case "check_expression", "equality_expression", "conjunction_expression":
			if condText == "" {
				condText = file.FlatNodeText(child)
			}
		case "control_structure_body":
			if thenBody == 0 {
				thenBody = child
			} else {
				elseBody = child
			}
		}
	}
	if condText == "" {
		return false
	}

	inner := strings.TrimSpace(condText)
	if strings.HasPrefix(inner, "(") && strings.HasSuffix(inner, ")") {
		inner = inner[1 : len(inner)-1]
	}

	isPattern := varName + " is " + targetType
	if strings.Contains(inner, isPattern) {
		if thenBody != 0 && isDescendantFlat(file, asNode, thenBody) {
			return true
		}
	}

	negPattern := varName + " !is " + targetType
	if strings.Contains(inner, negPattern) {
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
	text := strings.TrimSpace(file.FlatNodeText(body))
	if strings.HasPrefix(text, "{") && strings.HasSuffix(text, "}") {
		text = strings.TrimSpace(text[1 : len(text)-1])
	}
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return false
	}
	last := strings.TrimSpace(lines[len(lines)-1])
	return strings.HasPrefix(last, "return") || strings.HasPrefix(last, "throw ") ||
		last == "continue" || last == "break"
}

func hasEarlyExitKeyword(text string) bool {
	return strings.Contains(text, "return") || strings.Contains(text, "throw ") ||
		strings.Contains(text, "continue") || strings.Contains(text, "break")
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
