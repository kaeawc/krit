package rules

import (
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
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
	text := file.FlatNodeText(idx)
	if strings.Contains(text, "as?") {
		return
	}

	// Extract the variable name and target type from "expr as TargetType"
	parts := strings.SplitN(text, " as ", 2)
	if len(parts) != 2 {
		return
	}
	castVar := strings.TrimSpace(parts[0])
	castTarget := strings.TrimSpace(parts[1])

	// Skip idiomatic Android patterns where the cast is guaranteed by context:
	// - findViewById/requireViewById casts to a View subtype (layout XML guarantees type)
	// - DialogCompat.requireViewById similar
	// - getSystemService(...) casts to a service type
	// - getSerializable/getParcelable — Android Bundle access requires cast
	// - .layoutParams — View.getLayoutParams returns abstract base type
	// - .applicationContext as Application — always safe per Android contract
	// - TransitionValues.values[...] — Map<String, Any> requires cast
	castVarLower := castVar
	if strings.Contains(castVarLower, "findViewById") ||
		strings.Contains(castVarLower, "requireViewById") ||
		strings.Contains(castVarLower, "findFragmentById") ||
		strings.Contains(castVarLower, "findFragmentByTag") ||
		strings.Contains(castVarLower, "getSerializable(") ||
		strings.Contains(castVarLower, "requireNonNull(") ||
		strings.Contains(castVarLower, "requireNotNull(") ||
		strings.Contains(castVarLower, "checkNotNull(") ||
		strings.Contains(castVarLower, "requireView(") ||
		strings.Contains(castVarLower, "requireActivity(") ||
		strings.Contains(castVarLower, "requireContext(") ||
		strings.Contains(castVarLower, "requireDialog(") ||
		// Android framework return-type casts — the caller's context
		// determines the concrete type.
		strings.Contains(castVarLower, ".inflate(") ||
		strings.Contains(castVarLower, "onCreateDialog(") ||
		strings.Contains(castVarLower, "onCreateView(") ||
		strings.Contains(castVarLower, "getSystemService(") ||
		strings.Contains(castVarLower, ".fromBundle(") ||
		// RecyclerView.ViewHolder / PopupWindow / DialogFragment layout-root
		// downcasts — `itemView`/`contentView` are layout-guaranteed.
		castVarLower == "itemView" || castVarLower == "contentView" ||
		strings.HasSuffix(castVarLower, ".itemView") ||
		strings.HasSuffix(castVarLower, ".contentView") ||
		// RecyclerView LayoutManager downcasts — the LayoutManager was
		// set programmatically by the caller that owns the cast.
		strings.HasSuffix(castVarLower, ".layoutManager") ||
		// ViewGroup.getChildAt(i) returns View — caller knows the child
		// type from the layout structure.
		strings.Contains(castVarLower, ".getChildAt(") ||
		strings.Contains(castVarLower, "getChildAt(") ||
		strings.HasSuffix(castVarLower, ".layoutParams") ||
		castVarLower == "layoutParams" ||
		strings.HasSuffix(castVarLower, ".applicationContext") ||
		strings.HasSuffix(castVarLower, ".parent") ||
		strings.HasSuffix(castVarLower, ".rootView") ||
		strings.Contains(castVarLower, ".values[") ||
		strings.Contains(castVarLower, "startValues.values") ||
		strings.Contains(castVarLower, "endValues.values") {
		return
	}
	// Skip `null as Type?` — cast of null literal for overload resolution.
	if castVar == "null" {
		return
	}
	// Skip casts to nullable types `as T?` — they can never throw
	// ClassCastException on null input and are often used for generic
	// variance widening.
	if strings.HasSuffix(castTarget, "?") {
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
	// Skip casts inside a `when` branch whose subject is the same variable
	// being cast's discriminator — the branch condition has guaranteed the
	// target shape already. Common idiom for annotation/value deserializers:
	//     when (name) { "x" -> foo = value as String; "y" -> ... }
	if isInsideWhenBranchWithStringSubjectFlat(file, idx) {
		return
	}
	// Skip ValueAnimator.animatedValue casts — the animator constructor
	// determines the return type safely.
	if strings.HasSuffix(castVar, "animatedValue") ||
		strings.HasSuffix(castVar, ".animatedValue") {
		return
	}
	// Skip rootProject.extra["..."] / extra[...] Gradle patterns.
	if strings.Contains(castVar, ".extra[") || strings.Contains(castVar, "rootProject.extra") {
		return
	}
	// Skip empty-lambda SAM conversion: `{} as OnEventListener` — the
	// Kotlin compiler generates a valid SAM instance; the `as` is a
	// workaround for default-parameter SAM inference limitations.
	if strings.HasPrefix(castVar, "{") && strings.HasSuffix(castVar, "}") {
		return
	}

	// 1. Resolver-based check: the expression type already matches the target
	//    (e.g., via is-check smart cast or declaration type).
	if ctx.Resolver != nil && file.FlatChildCount(idx) >= 2 {
		exprType := ctx.Resolver.ResolveFlatNode(file.FlatChild(idx, 0), file)
		targetType := ctx.Resolver.ResolveFlatNode(file.FlatChild(idx, file.FlatChildCount(idx)-1), file)
		if exprType != nil && targetType != nil &&
			exprType.Kind != typeinfer.TypeUnknown && targetType.Kind != typeinfer.TypeUnknown {
			if exprType.FQN == targetType.FQN || exprType.IsSubtypeOf(targetType.FQN) ||
				exprType.Name == targetType.Name {
				return
			}
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

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Unsafe cast. Use 'as?' for a safe cast instead.")
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


// Confidence reports a tier-2 (medium) base confidence — needs the
// resolver to decide whether the source expression is nullable; without it,
// reverts to a heuristic. Classified per roadmap/17.
func (r *CastNullableToNonNullableTypeRule) Confidence() float64 { return 0.75 }

var castNullableRe = regexp.MustCompile(`\?\s+as\s+[A-Z]\w+\s*$|[?!]\s+as\s+[A-Z]`)

func (r *CastNullableToNonNullableTypeRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	text := file.FlatNodeText(idx)
	// Skip safe casts
	if strings.Contains(text, "as?") {
		return
	}
	// Check if the left-hand side is nullable (ends with ? or !!)
	parts := strings.SplitN(text, " as ", 2)
	if len(parts) != 2 {
		return
	}
	lhs := strings.TrimSpace(parts[0])
	rhs := strings.TrimSpace(parts[1])
	if (strings.HasSuffix(lhs, "?") || strings.HasSuffix(lhs, "!!") || strings.HasSuffix(lhs, ")")) && !strings.HasSuffix(rhs, "?") {
		// If resolver is available, check if source expression is actually nullable
		if ctx.Resolver != nil && file.FlatChildCount(idx) >= 2 {
			isNull := ctx.Resolver.IsNullableFlat(file.FlatChild(idx, 0), file)
			if isNull != nil && !*isNull {
				return // source is known non-null, cast is safe
			}
		}
		f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			"Casting a nullable type to a non-nullable type. Use 'as?' instead.")
		// Fix: replace "as Type" with "as? Type"
		fixed := lhs + " as? " + rhs
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

