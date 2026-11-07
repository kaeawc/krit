package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ---------------------------------------------------------------------------
// UnnecessaryNotNullCheckRule detects != null on non-nullable.
// ---------------------------------------------------------------------------
type UnnecessaryNotNullCheckRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs null safety rule. Detection leans on structural patterns
// around nullable expressions and has a heuristic fallback when the
// resolver is absent. Classified per roadmap/17.
func (r *UnnecessaryNotNullCheckRule) Confidence() float64 { return api.ConfidenceMedium }

// isMutableVarProperty returns true if the given name is declared as `var`
// in the file's AST. Kotlin cannot smart-cast mutable properties because their
// value could change between the null check and access.
//
// Backed by the per-file nullSafetyFileSummary so repeat queries on the same
// file are O(1). The first call builds a single-pass scan of all four
// helpers' results; subsequent calls (including from sibling rules) reuse it.
func isMutableVarProperty(file *scanner.File, name string) bool {
	return nullSafetySummaryFor(file).mutableVar[name]
}

// isFrameworkNullableProperty returns true if `name` is a bare identifier that
// in typical Android framework usage resolves to an inherited property whose
// getter is `@Nullable`. A conservative name-based resolver cannot see the
// Java `@Nullable` annotation and may widen these to non-null, producing
// false positives for UnnecessarySafeCall / UnnecessaryNotNullOperator.
//
// Only listed here are names whose framework property is nullable AND which
// are rarely the subject of a legitimately-unnecessary !! / ?. in Android
// code. The FP risk from these framework properties outweighs any missed
// findings on identically-named local vals.
func isFrameworkNullableProperty(name string) bool {
	switch name {
	// RecyclerView.adapter, RecyclerView.layoutManager, RecyclerView.itemAnimator
	case "adapter", "layoutManager", "itemAnimator":
		return true
	// DialogFragment.dialog, Fragment.view, Fragment.parentFragment,
	// Fragment.targetFragment, Fragment.host
	case "activity", "dialog", "parentFragment", "targetFragment", "host":
		return true
	// View.parent, View.tag, View.rootView, View.background, View.contentDescription
	case "parent", "tag", "background", "contentDescription":
		return true
	}
	return false
}

// hasMemberAccessInitializer returns true if `name` is declared as a local val
// whose initializer is a member access expression like `something.field` that a
// bare-name resolver cannot prove non-null.
func hasMemberAccessInitializer(file *scanner.File, name string) bool {
	return nullSafetySummaryFor(file).memberAccessInitializer[name]
}

func hasUncertainCallInitializer(file *scanner.File, name string) bool {
	return nullSafetySummaryFor(file).uncertainCallInitializer[name]
}

// isExplicitNullableDeclaration returns true if the given name is declared
// with an explicit nullable type annotation (`: T?`) anywhere in the file.
func isExplicitNullableDeclaration(file *scanner.File, name string) bool {
	return nullSafetySummaryFor(file).explicitNullable[name]
}

func (r *UnnecessaryNotNullCheckRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if file.FlatType(idx) == "call_expression" {
		r.checkPreconditionOnNonNull(ctx)
		return
	}
	if ctx.Resolver == nil {
		return
	}

	operand, op, ok := flatNullComparisonOperand(file, idx)
	if !ok {
		return
	}

	// Trust the compiler's SENSELESS_COMPARISON verdict for the non-null
	// direction of a null comparison — `x != null` always true, `x == null`
	// always false. Accept gates out the always-null direction (the factory
	// covers every always-true/false condition; this rule owns only the
	// non-null null check), and covers operands the heuristic below skips.
	if projectDiagnostic(ctx, DiagnosticProjection{
		RuleID:       "UnnecessaryNotNullCheck",
		FactoryNames: []string{"SENSELESS_COMPARISON"},
		Accept: func(_ *api.Context, d oracle.Diagnostic) bool {
			val := senselessComparisonAlwaysValue(d.Message)
			return (op == "!=" && val == "true") || (op == "==" && val == "false")
		},
		Message: func(ctx *api.Context, _ oracle.Diagnostic) string {
			operandText := strings.TrimSpace(ctx.File.FlatNodeText(flatUnwrapParenExpr(ctx.File, operand)))
			return fmt.Sprintf("Unnecessary null check on non-nullable '%s'.", operandText)
		},
		Fix: func(ctx *api.Context, _ oracle.Diagnostic) *scanner.Fix {
			replacement := "true"
			if op == "==" {
				replacement = "false"
			}
			return &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(ctx.File.FlatStartByte(ctx.Idx)),
				EndByte:     int(ctx.File.FlatEndByte(ctx.Idx)),
				Replacement: replacement,
			}
		},
		Confidence: api.ConfidenceVeryHigh,
	}) {
		return
	}

	// The compiler analyzed this file and reported nothing, so the name-based guess would only add false positives.
	if compilerAnalyzedFile(ctx) {
		return
	}
	operand = flatUnwrapParenExpr(file, operand)
	resolved, ok := flatResolvedNullCheckOperandType(file, ctx.Resolver, operand)
	if !ok || resolved.IsNullable() {
		return
	}

	replacement := "true"
	if op == "==" {
		replacement = "false"
	}
	operandText := strings.TrimSpace(file.FlatNodeText(operand))
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("Unnecessary null check on non-nullable '%s'.", operandText))
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(idx)),
		EndByte:     int(file.FlatEndByte(idx)),
		Replacement: replacement,
	}
	ctx.Emit(f)
}

func flatNullComparisonOperand(file *scanner.File, idx uint32) (operand uint32, op string, ok bool) {
	if file == nil || idx == 0 || file.FlatType(idx) != "equality_expression" || file.FlatChildCount(idx) < 3 {
		return 0, "", false
	}
	left := file.FlatChild(idx, 0)
	operator := file.FlatChild(idx, 1)
	right := file.FlatChild(idx, file.FlatChildCount(idx)-1)
	op = file.FlatNodeText(operator)
	if op != "==" && op != "!=" {
		return 0, "", false
	}
	left = flatUnwrapParenExpr(file, left)
	right = flatUnwrapParenExpr(file, right)
	switch {
	case flatIsNullLiteral(file, left):
		return right, op, true
	case flatIsNullLiteral(file, right):
		return left, op, true
	default:
		return 0, "", false
	}
}

// senselessComparisonAlwaysValue extracts the always-value ("true"/"false")
// from the compiler's SENSELESS_COMPARISON message, whose template is
// `Condition is always ”{0}”.` and renders as `Condition is always 'true'.`.
// Returns "" when the message doesn't carry a recognizable value.
func senselessComparisonAlwaysValue(message string) string {
	const marker = "always '"
	i := strings.Index(message, marker)
	if i < 0 {
		return ""
	}
	rest := message[i+len(marker):]
	j := strings.IndexByte(rest, '\'')
	if j < 0 {
		return ""
	}
	return rest[:j]
}

func flatResolvedNullCheckOperandType(file *scanner.File, resolver typeinfer.TypeResolver, operand uint32) (*typeinfer.ResolvedType, bool) {
	if file == nil || resolver == nil || operand == 0 {
		return nil, false
	}
	switch file.FlatType(operand) {
	case "simple_identifier", "this_expression":
		if !flatReferenceHasSameFileTarget(file, operand) {
			return nil, false
		}
		return flatKnownResolvedType(file, resolver, operand)
	case "call_expression":
		resolved, ok := flatKnownResolvedType(file, resolver, operand)
		if !ok || !flatCallHasResolvedTarget(file, resolver, operand) {
			return nil, false
		}
		return resolved, true
	case "navigation_expression":
		return flatNavigationResolvedMemberType(file, resolver, operand)
	default:
		return nil, false
	}
}

func flatKnownResolvedType(file *scanner.File, resolver typeinfer.TypeResolver, idx uint32) (*typeinfer.ResolvedType, bool) {
	resolved := resolver.ResolveFlatNode(idx, file)
	if resolved == nil || !resolved.Resolved || resolved.Kind == typeinfer.TypeUnknown || resolved.Kind == typeinfer.TypeGeneric {
		return nil, false
	}
	if nullable := resolver.IsNullableFlat(idx, file); nullable != nil {
		copyVal := *resolved
		// IsNullableFlat may DEMOTE a nullable type to non-null only through a
		// smart cast, which the source resolver tracks for simple identifiers.
		// For other shapes (call results, qualified accesses) source inference
		// defaults unresolved nullability to non-null, so it must not override
		// a nullable type the oracle already proved (e.g. a fun returning T?
		// resolved across files / overloads). Honour a non-null verdict only
		// when it can come from a real smart cast.
		demotes := !*nullable && resolved.IsNullable()
		if demotes && file.FlatType(idx) != "simple_identifier" {
			return &copyVal, true
		}
		copyVal.Nullable = *nullable
		if !copyVal.Nullable && copyVal.Kind == typeinfer.TypeNullable {
			copyVal.Kind = typeinfer.TypeClass
		}
		return &copyVal, true
	}
	return resolved, true
}

func flatReferenceHasSameFileTarget(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	name := "this"
	if file.FlatType(idx) != "this_expression" {
		name = strings.TrimSpace(file.FlatNodeText(idx))
	}
	if name == "" {
		return false
	}
	if name == "this" {
		_, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_declaration", "function_declaration")
		return ok
	}
	return flatFindSameFileValueDeclaration(file, idx, name) != 0
}

func flatFindSameFileValueDeclaration(file *scanner.File, ref uint32, name string) uint32 {
	if file == nil || name == "" {
		return 0
	}
	var sameOwner uint32
	var sameFile uint32
	refOwner := flatSemanticOwner(file, ref)
	refStart := file.FlatStartByte(ref)
	file.FlatWalkAllNodes(0, func(candidate uint32) {
		if sameOwner != 0 {
			return
		}
		if !flatValueDeclarationMatches(file, candidate, name) {
			return
		}
		if file.FlatType(candidate) == "property_declaration" && file.FlatStartByte(candidate) > refStart && flatSemanticOwner(file, candidate) == refOwner {
			return
		}
		if sameFile == 0 {
			sameFile = candidate
		}
		if flatSemanticOwner(file, candidate) == refOwner {
			sameOwner = candidate
		}
	})
	if sameOwner != 0 {
		return sameOwner
	}
	return sameFile
}

func flatValueDeclarationMatches(file *scanner.File, idx uint32, name string) bool {
	switch file.FlatType(idx) {
	case "property_declaration", "parameter":
		return extractIdentifierFlat(file, idx) == name
	default:
		return false
	}
}

func flatCallHasResolvedTarget(file *scanner.File, resolver typeinfer.TypeResolver, call uint32) bool {
	name := flatCallExpressionName(file, call)
	if name == "" {
		return false
	}
	first := file.FlatChild(call, 0)
	if file.FlatType(first) == "navigation_expression" {
		return flatQualifiedCallHasResolvedTarget(file, resolver, first, name)
	}
	if flatFindSameFileFunctionDeclaration(file, call, name) != 0 {
		return true
	}
	if flatLooksLikeConstructorCallName(name) {
		if flatFindSameFileClassLikeDeclaration(file, name) != 0 {
			return true
		}
		resolved := resolver.ResolveByNameFlat(name, call, file)
		return resolved != nil && resolved.Kind != typeinfer.TypeUnknown
	}
	if _, ok := flatKnownResolvedType(file, resolver, call); ok {
		return true
	}
	return false
}

func flatQualifiedCallHasResolvedTarget(file *scanner.File, resolver typeinfer.TypeResolver, nav uint32, name string) bool {
	receiver := flatNullCheckNavigationReceiver(file, nav)
	receiverType, ok := flatKnownResolvedType(file, resolver, receiver)
	if !ok || receiverType.Name == "" {
		return false
	}
	if typeinfer.LookupStdlibMethod(receiverType.Name, name) != nil {
		return true
	}
	if flatClassLikeMemberFunctionHasReturn(file, resolver, receiverType.Name, name) {
		return true
	}
	return flatSameFileExtensionFunctionMatches(file, receiverType.Name, name)
}

func flatLooksLikeConstructorCallName(name string) bool {
	return name != "" && name[0] >= 'A' && name[0] <= 'Z'
}

func flatFindSameFileFunctionDeclaration(file *scanner.File, ref uint32, name string) uint32 {
	var sameOwner uint32
	var sameFile uint32
	refOwner := flatSemanticOwner(file, ref)
	file.FlatWalkAllNodes(0, func(candidate uint32) {
		if sameOwner != 0 || file.FlatType(candidate) != "function_declaration" || flatFunctionName(file, candidate) != name {
			return
		}
		if sameFile == 0 {
			sameFile = candidate
		}
		if flatSemanticOwner(file, candidate) == refOwner {
			sameOwner = candidate
		}
	})
	if sameOwner != 0 {
		return sameOwner
	}
	return sameFile
}

func flatFindSameFileClassLikeDeclaration(file *scanner.File, name string) uint32 {
	var classNode uint32
	file.FlatWalkAllNodes(0, func(candidate uint32) {
		if classNode != 0 {
			return
		}
		switch file.FlatType(candidate) {
		case "class_declaration", "object_declaration":
			if extractIdentifierFlat(file, candidate) == name {
				classNode = candidate
			}
		}
	})
	return classNode
}

func flatClassLikeMemberFunctionHasReturn(file *scanner.File, resolver typeinfer.TypeResolver, className string, memberName string) bool {
	info := resolver.ClassHierarchy(className)
	if info == nil || info.File != file.Path {
		return false
	}
	for _, member := range info.Members {
		if member.Kind == "function" && member.Name == memberName && member.Type != nil && member.Type.Kind != typeinfer.TypeUnknown {
			return true
		}
	}
	return false
}

func flatSameFileExtensionFunctionMatches(file *scanner.File, receiverType string, funcName string) bool {
	if receiverType == "" || funcName == "" {
		return false
	}
	var matched bool
	file.FlatWalkAllNodes(0, func(candidate uint32) {
		if matched || file.FlatType(candidate) != "function_declaration" || flatFunctionName(file, candidate) != funcName {
			return
		}
		matched = flatFunctionReceiverTypeName(file, candidate) == receiverType
	})
	return matched
}

func flatFunctionReceiverTypeName(file *scanner.File, fn uint32) string {
	if file == nil || fn == 0 {
		return ""
	}
	var receiverType string
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type":
			if receiverType == "" {
				receiverType = flatLastIdentifierInNode(file, child)
			}
		case ".":
			return receiverType
		case "function_value_parameters", "function_body":
			return ""
		}
	}
	return ""
}

func flatLastIdentifierInNode(file *scanner.File, idx uint32) string {
	last := ""
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		switch file.FlatType(candidate) {
		case "simple_identifier", "type_identifier":
			last = file.FlatNodeText(candidate)
		}
	})
	return last
}

func flatNavigationResolvedMemberType(file *scanner.File, resolver typeinfer.TypeResolver, nav uint32) (*typeinfer.ResolvedType, bool) {
	if flatNavigationHasSafeCall(file, nav) {
		return nil, false
	}
	memberName := flatNavigationExpressionLastIdentifier(file, nav)
	if memberName == "" {
		return nil, false
	}
	receiver := flatNullCheckNavigationReceiver(file, nav)
	if receiver == 0 {
		return nil, false
	}
	if file.FlatType(receiver) == "this_expression" || file.FlatNodeTextEquals(receiver, "this") {
		owner, ok := flatEnclosingAncestor(file, nav, "class_declaration", "object_declaration")
		if !ok {
			return nil, false
		}
		return flatClassLikeMemberPropertyType(file, resolver, owner, memberName)
	}
	receiverType, ok := flatKnownResolvedType(file, resolver, receiver)
	if !ok || receiverType.Name == "" {
		return nil, false
	}
	return flatSameFileClassMemberPropertyType(file, resolver, receiverType.Name, memberName)
}

func flatNavigationHasSafeCall(file *scanner.File, nav uint32) bool {
	found := false
	file.FlatWalkAllNodes(nav, func(candidate uint32) {
		if file.FlatType(candidate) == "?." {
			found = true
		}
	})
	return found
}

// flatNavigationSafeCallOperator returns the AST node index of the top-level
// `?.` token for a navigation_expression — i.e. the operator that joins this
// expression's receiver to its suffix. Returns 0 if no `?.` token is present
// directly on the expression (chained `?.` inside the receiver is ignored).
//
// Kotlin tree-sitter shapes the operator as either a direct `?.` child of
// `navigation_expression` or as a `?.` child inside its `navigation_suffix`.
// Using the AST node's byte range avoids the text-scan footgun of locating
// `?.` inside a receiver-side string literal (e.g. `x.contains("?.")?.bar`).
func flatNavigationSafeCallOperator(file *scanner.File, nav uint32) uint32 {
	if file == nil || nav == 0 {
		return 0
	}
	for child := file.FlatFirstChild(nav); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "?.":
			return child
		case "navigation_suffix":
			for gc := file.FlatFirstChild(child); gc != 0; gc = file.FlatNextSib(gc) {
				if file.FlatType(gc) == "?." {
					return gc
				}
			}
		}
	}
	return 0
}

func flatNullCheckNavigationReceiver(file *scanner.File, idx uint32) uint32 {
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

func flatSameFileClassMemberPropertyType(file *scanner.File, resolver typeinfer.TypeResolver, className string, memberName string) (*typeinfer.ResolvedType, bool) {
	classNode := flatFindSameFileClassLikeDeclaration(file, className)
	if classNode == 0 {
		return nil, false
	}
	return flatClassLikeMemberPropertyType(file, resolver, classNode, memberName)
}

func flatClassLikeMemberPropertyType(file *scanner.File, resolver typeinfer.TypeResolver, classNode uint32, memberName string) (*typeinfer.ResolvedType, bool) {
	var property uint32
	file.FlatWalkAllNodes(classNode, func(candidate uint32) {
		if property != 0 || file.FlatType(candidate) != "property_declaration" {
			return
		}
		if flatEnclosingClassLike(file, candidate) == classNode && extractIdentifierFlat(file, candidate) == memberName {
			property = candidate
		}
	})
	if property == 0 {
		return nil, false
	}
	return flatKnownResolvedType(file, resolver, property)
}

func flatEnclosingClassLike(file *scanner.File, idx uint32) uint32 {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "class_declaration", "object_declaration":
			return p
		}
	}
	return 0
}

func flatSemanticOwner(file *scanner.File, idx uint32) uint32 {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration", "class_declaration", "object_declaration":
			return p
		}
	}
	return 0
}

// sameFileNameDeclaredNullable reports whether name resolves to a same-file declaration with an explicit nullable type.
func sameFileNameDeclaredNullable(file *scanner.File, ref uint32, name string) bool {
	if file == nil || ref == 0 || name == "" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(0, func(decl uint32) {
		if found || file.FlatStartByte(decl) > file.FlatStartByte(ref) {
			return
		}
		switch file.FlatType(decl) {
		case "property_declaration", "parameter", "class_parameter":
		default:
			return
		}
		if extractIdentifierFlat(file, decl) != name || !declarationVisibleFromReference(file, decl, ref) {
			return
		}
		found = declarationHasNullableTypeFlat(file, decl)
	})
	return found
}

// provenNonNullMessage words a redundant null operator finding the compiler proved. A receiver whose declaration
// is nullable, or a member access whose declaration is not visible here, may be non-null only through a smart
// cast, so "non-nullable" would contradict the declaration.
func provenNonNullMessage(file *scanner.File, ref uint32, op, recv, kind string) string {
	if !isSimpleIdentifierText(recv) {
		return fmt.Sprintf("Unnecessary %s (%s): '%s' is already non-null here (non-null type or smart cast).", kind, op, recv)
	}
	if sameFileNameDeclaredNullable(file, ref, recv) {
		return fmt.Sprintf("Unnecessary %s (%s): '%s' is smart-cast to non-null here.", kind, op, recv)
	}
	return fmt.Sprintf("Unnecessary %s (%s) on non-nullable '%s'.", kind, op, recv)
}

// unnecessaryNotNullCheckPreconditions are the stdlib preconditions detekt's UnnecessaryNotNullCheck reports on a
// non-null argument.
var unnecessaryNotNullCheckPreconditions = map[string]bool{"requireNotNull": true, "checkNotNull": true}

// checkPreconditionOnNonNull reports `requireNotNull(x)` and `checkNotNull(x)` where x is a parameter of the
// enclosing function declared with a non-null type. detekt's rule of this name reads the argument's type from the
// compiler; the declared type of a parameter is the one case the syntax proves, so a platform type or a smart cast
// never produces a finding here.
func (r *UnnecessaryNotNullCheckRule) checkPreconditionOnNonNull(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	name := flatCallExpressionName(file, idx)
	if !unnecessaryNotNullCheckPreconditions[name] {
		return
	}
	if first := file.FlatFirstChild(idx); first == 0 || file.FlatType(first) != "simple_identifier" {
		return
	}
	args := directCallArgumentsFlat(file, idx)
	if args == 0 || file.FlatNamedChildCount(args) < 1 {
		return
	}
	arg := file.FlatNamedChild(args, 0)
	if arg == 0 || file.FlatType(arg) != "value_argument" || file.FlatNamedChildCount(arg) != 1 {
		return
	}
	operand := flatUnwrapParenExpr(file, file.FlatNamedChild(arg, 0))
	if file.FlatType(operand) != "simple_identifier" {
		return
	}
	operandName := file.FlatNodeText(operand)
	if !parameterDeclaredNonNull(file, idx, operandName) || hasNullableGenericParamBoundFlat(file, idx, operandName) {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("Using `%s` on non-null `%s` is unnecessary", name, operandName)))
}

// parameterDeclaredNonNull reports whether the nearest enclosing function declares a parameter called name with an
// explicit non-null type, and nothing else in that function reuses the name.
func parameterDeclaredNonNull(file *scanner.File, idx uint32, name string) bool {
	fn, ok := flatEnclosingAncestor(file, idx, "function_declaration", "secondary_constructor")
	if !ok {
		return false
	}
	nonNullParam := false
	reused := false
	file.FlatWalkAllNodes(fn, func(n uint32) {
		switch file.FlatType(n) {
		case "parameter":
			if parameterIdentifier(file, n) != name {
				return
			}
			typed := false
			nullable := false
			for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
				switch file.FlatType(c) {
				case "nullable_type":
					nullable = true
				case "user_type", "function_type", "parenthesized_type":
					typed = true
				}
			}
			if typed && !nullable {
				nonNullParam = true
			} else {
				reused = true
			}
		case "property_declaration", "variable_declaration":
			if extractIdentifierFlat(file, n) == name || propertyDeclarationNameFlat(file, n) == name {
				reused = true
			}
		}
	})
	return nonNullParam && !reused
}

func parameterIdentifier(file *scanner.File, param uint32) string {
	for c := file.FlatFirstChild(param); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "simple_identifier" {
			return file.FlatNodeText(c)
		}
	}
	return ""
}
