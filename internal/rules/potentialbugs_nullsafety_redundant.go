package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/experiment"
	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
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
func (r *UnnecessaryNotNullCheckRule) Confidence() float64 { return 0.75 }

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
	if ctx.Resolver == nil {
		return
	}

	operand, op, ok := flatNullComparisonOperand(file, idx)
	if !ok {
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
	f := r.Finding(file, file.FlatRow(idx)+1, 1,
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
	if resolved == nil || resolved.Kind == typeinfer.TypeUnknown || resolved.Kind == typeinfer.TypeGeneric {
		return nil, false
	}
	if nullable := resolver.IsNullableFlat(idx, file); nullable != nil {
		copyVal := *resolved
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

// ---------------------------------------------------------------------------
// UnnecessaryNotNullOperatorRule detects !! on non-null.
// ---------------------------------------------------------------------------
type UnnecessaryNotNullOperatorRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — flags !! on a
// non-null type; relies on resolver for nullability. Heuristic fallback is
// conservative but noisy. Classified per roadmap/17.
func (r *UnnecessaryNotNullOperatorRule) Confidence() float64 { return 0.75 }

func (r *UnnecessaryNotNullOperatorRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	text := file.FlatNodeText(idx)
	if !flatPostfixHasNotNullAssertion(file, idx) {
		return
	}

	// Extract the receiver name (everything before !!)
	receiver := strings.TrimSuffix(text, "!!")
	receiver = strings.TrimSpace(receiver)

	// Skip idiomatic Android/Java platform API patterns where !! is required.
	if isIdiomaticNullAssertionReceiver(receiver, file, flatFirstNamedChild(file, idx)) {
		return
	}

	// Only resolve simple identifiers — dotted member accesses cannot be
	// reliably resolved by bare name lookup (risk of name collision).
	if !strings.Contains(receiver, ".") {
		name := strings.TrimSpace(receiver)
		if name == "this" && nullableThisFromLambdaReceiverCallFlat(file, idx, ctx.Resolver) {
			return
		}

		// Skip if the name refers to a mutable `var` property/field.
		// Kotlin does NOT smart-cast mutable properties (the value could
		// change between the null check and the access), so !! is required
		// even after a null check.
		if isMutableVarProperty(file, name) {
			return
		}
		// Skip if the declaration's initializer contains an `else null`
		// tail or a `?.let` chain — a conservative resolver may widen
		// these to non-null incorrectly.
		if hasBranchNullInitializer(file, name) {
			return
		}
		// Skip framework-inherited nullable properties (e.g. RecyclerView.adapter,
		// DialogFragment.dialog, View.parent) when not shadowed by a local decl.
		if isFrameworkNullableProperty(name) {
			return
		}
		// Skip local vals initialized from a member access the resolver can't
		// prove non-null (e.g. `val attachment = mediaItem.attachment`).
		if hasMemberAccessInitializer(file, name) {
			return
		}
		if hasNullableGenericParamBoundFlat(file, idx, name) {
			return
		}

		// Use type resolver with position-aware smart cast lookup
		if ctx.Resolver != nil {
			resolved := ctx.Resolver.ResolveByNameFlat(name, idx, file)
			if resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
				if resolved.Kind == typeinfer.TypeGeneric {
					return
				}
				if resolved.IsNullable() {
					return // Actually nullable — !! is needed
				}
				// Known non-null — flag it
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Unnecessary not-null assertion (!!) on non-nullable '%s'.", name))
				bangStart := int(file.FlatEndByte(idx)) - 2
				f.Fix = &scanner.Fix{ByteMode: true, StartByte: bangStart, EndByte: int(file.FlatEndByte(idx)), Replacement: ""}
				ctx.Emit(f)
				return
			}
		}
		if sameFileExplicitNonNullValueDeclaration(file, idx, name) {
			f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				fmt.Sprintf("Unnecessary not-null assertion (!!) on non-nullable val '%s'.", name))
			bangStart := int(file.FlatEndByte(idx)) - 2
			f.Fix = &scanner.Fix{ByteMode: true, StartByte: bangStart, EndByte: int(file.FlatEndByte(idx)), Replacement: ""}
			ctx.Emit(f)
			return
		}

	}
}

// ExpressionPositions enumerates `postfix_expression` nodes whose text
// ends with `!!` and whose operand is a bare identifier. Those are the
// positions the rule's check method passes to ResolveByNameFlat — and
// therefore the only positions where a resolved oracle expression fact
// would change the verdict.
//
// Used by the targeted-resolution pre-pass under --depth=thorough:
// the dispatcher batches the union of these positions across active
// rules, sends one resolveExpressionTypes RPC, and populates the
// oracle's expressions map before dispatch begins. Lambda-param `!!`
// is the pattern that lights up here: source inference can't type the
// param, KAA can.
//
// Mirrors the lexical gates inside check (postfix + `!!` + simple
// identifier) so the resolution set is tight; over-approximating
// would still be correct but waste KAA work.
func (r *UnnecessaryNotNullOperatorRule) ExpressionPositions(file *scanner.File) []uint32 {
	if file == nil {
		return nil
	}
	var out []uint32
	file.FlatWalkNodes(0, "postfix_expression", func(idx uint32) {
		if !flatPostfixHasNotNullAssertion(file, idx) {
			return
		}
		receiver := strings.TrimSpace(strings.TrimSuffix(file.FlatNodeText(idx), "!!"))
		if strings.Contains(receiver, ".") {
			return
		}
		out = append(out, idx)
	})
	return out
}

func flatPostfixHasNotNullAssertion(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "postfix_expression" {
		return false
	}
	found := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "!!" || file.FlatNodeTextEquals(child, "!!") {
			found = true
			break
		}
	}
	return found
}

// hasBranchNullInitializer returns true if the given name is declared in
// the file with an initializer that has an `else null` / `-> null` /
// `?: null` tail, producing a nullable inferred type that a conservative
// resolver may widen incorrectly. Used to suppress false positives on
// UnnecessaryNotNullOperator / UnnecessarySafeCall.
func hasBranchNullInitializer(file *scanner.File, name string) bool {
	return nullSafetySummaryFor(file).branchNullInitializer[name]
}

func hasNullableGenericParamBoundFlat(file *scanner.File, idx uint32, name string) bool {
	var fn uint32
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "function_declaration" {
			fn = p
			break
		}
	}
	if fn == 0 {
		return false
	}
	fnText := file.FlatNodeText(fn)
	funStart := strings.Index(fnText, "fun ")
	if funStart < 0 {
		return false
	}
	sigText := fnText[funStart:]
	typeParamsStart := strings.Index(sigText, "<")
	paramsStart := strings.Index(sigText, "(")
	paramsEnd := strings.Index(sigText, ")")
	if typeParamsStart < 0 || paramsStart < 0 || paramsEnd < 0 || typeParamsStart > paramsStart {
		return false
	}
	typeParamsEnd := -1
	depth := 0
	for i := typeParamsStart; i < len(sigText); i++ {
		switch sigText[i] {
		case '<':
			depth++
		case '>':
			depth--
			if depth == 0 {
				typeParamsEnd = i
				break
			}
		}
	}
	if typeParamsEnd < 0 {
		return false
	}
	typeParams := sigText[typeParamsStart+1 : typeParamsEnd]
	params := sigText[paramsStart+1 : paramsEnd]
	nullableTypeParams := make(map[string]bool)
	for _, entry := range splitTopLevelCommaParts(typeParams) {
		entry = strings.TrimSpace(entry)
		colon := strings.Index(entry, ":")
		if colon < 0 {
			continue
		}
		typeParam := strings.TrimSpace(entry[:colon])
		if typeParam == "" {
			continue
		}
		if strings.Contains(entry[colon+1:], "?") {
			nullableTypeParams[typeParam] = true
		}
	}
	if len(nullableTypeParams) == 0 {
		return false
	}
	paramPattern := regexp.MustCompile(`(^|[,(])\s*` + regexp.QuoteMeta(name) + `\s*:\s*([A-Z][A-Za-z0-9_]*)\b`)
	if match := paramPattern.FindStringSubmatch(params); match != nil && nullableTypeParams[match[2]] {
		return true
	}
	var localMatch bool
	file.FlatWalkNodes(fn, "property_declaration", func(prop uint32) {
		if localMatch || extractIdentifierFlat(file, prop) != name {
			return
		}
		varDecl, _ := file.FlatFindChild(prop, "variable_declaration")
		if varDecl == 0 {
			return
		}
		for i := 0; i < file.FlatNamedChildCount(varDecl); i++ {
			child := file.FlatNamedChild(varDecl, i)
			switch file.FlatType(child) {
			case "user_type", "nullable_type":
				localMatch = nullableTypeParams[strings.TrimSpace(file.FlatNodeText(child))]
				return
			}
		}
	})
	return localMatch
}

func splitTopLevelCommaParts(s string) []string {
	var parts []string
	start := 0
	depthAngle := 0
	depthParen := 0
	depthBrace := 0
	depthBracket := 0
	for i, r := range s {
		switch r {
		case '<':
			depthAngle++
		case '>':
			if depthAngle > 0 {
				depthAngle--
			}
		case '(':
			depthParen++
		case ')':
			if depthParen > 0 {
				depthParen--
			}
		case '{':
			depthBrace++
		case '}':
			if depthBrace > 0 {
				depthBrace--
			}
		case '[':
			depthBracket++
		case ']':
			if depthBracket > 0 {
				depthBracket--
			}
		case ',':
			if depthAngle == 0 && depthParen == 0 && depthBrace == 0 && depthBracket == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// nullSafetyFileSummary collects the per-file facts needed by the four
// name-keyed helper queries used by UnnecessarySafeCall and
// UnnecessaryNotNullOperator: mutable `var` names, explicitly-nullable
// declarations, declarations whose initializer has a branch-null tail,
// and declarations whose initializer is a member access.
//
// Built lazily per file on first use and cached in nullSafetyFileCache
// keyed by file.Path. The single-pass build replaces what used to be
// four independent O(lines) scans per `?.` / `!!` receiver name, which
// was the dominant UnnecessarySafeCall cost on large corpora.
//
// Semantics of every map are preserved from the original helpers. In
// particular `memberAccessInitializer` records only the first matching
// declaration line per name (and drops the entry when that first line's
// initializer ends with `!!`) to match the early-return behavior of
// hasMemberAccessInitializer.
type nullSafetyFileSummary struct {
	mutableVar               map[string]bool
	explicitNullable         map[string]bool
	branchNullInitializer    map[string]bool
	memberAccessInitializer  map[string]bool
	uncertainCallInitializer map[string]bool
}

// nullSafetySummaryFor returns a cached summary for the file, building
// it on first call. Backed by the shared filefacts cache.
func nullSafetySummaryFor(file *scanner.File) *nullSafetyFileSummary {
	if file == nil {
		return emptyNullSafetySummary
	}
	return filefacts.FileFact(fileFactsCache(), file, slotNullSafety, func() *nullSafetyFileSummary {
		return buildNullSafetySummary(file)
	})
}

var emptyNullSafetySummary = &nullSafetyFileSummary{
	mutableVar:               map[string]bool{},
	explicitNullable:         map[string]bool{},
	branchNullInitializer:    map[string]bool{},
	memberAccessInitializer:  map[string]bool{},
	uncertainCallInitializer: map[string]bool{},
}

func buildNullSafetySummary(file *scanner.File) *nullSafetyFileSummary {
	s := &nullSafetyFileSummary{
		mutableVar:               make(map[string]bool),
		explicitNullable:         make(map[string]bool),
		branchNullInitializer:    make(map[string]bool),
		memberAccessInitializer:  make(map[string]bool),
		uncertainCallInitializer: make(map[string]bool),
	}
	file.FlatWalkNodes(0, "property_declaration", func(prop uint32) {
		name := propertyDeclarationName(file, prop)
		if name == "" {
			name = extractIdentifierFlat(file, prop)
		}
		if name == "" {
			return
		}
		if propertyDeclarationIsVar(file, prop) {
			s.mutableVar[name] = true
		}
		if declarationHasNullableTypeFlat(file, prop) {
			s.explicitNullable[name] = true
		}
		init := propertyInitializerExpression(file, prop)
		if init == 0 {
			return
		}
		if !propertyDeclarationIsVar(file, prop) && initializerIsUnresolvedMemberAccessFlat(file, init) {
			s.memberAccessInitializer[name] = true
		}
		if !propertyDeclarationIsVar(file, prop) &&
			!declarationHasExplicitTypeFlat(file, prop) &&
			initializerIsUncertainCallFlat(file, init) {
			s.uncertainCallInitializer[name] = true
		}
		if initializerCanProduceNullFlat(file, init) {
			s.branchNullInitializer[name] = true
		}
	})
	return s
}

func declarationHasNullableTypeFlat(file *scanner.File, decl uint32) bool {
	if file == nil || decl == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(decl, func(candidate uint32) {
		if !found && file.FlatType(candidate) == "nullable_type" {
			found = true
		}
	})
	return found
}

func initializerIsUnresolvedMemberAccessFlat(file *scanner.File, init uint32) bool {
	if file == nil || init == 0 {
		return false
	}
	init = flatUnwrapParenExpr(file, init)
	if file.FlatType(init) == "postfix_expression" && flatPostfixHasNotNullAssertion(file, init) {
		return false
	}
	return file.FlatType(init) == "navigation_expression" && !flatNavigationHasSafeCall(file, init)
}

func initializerIsUncertainCallFlat(file *scanner.File, init uint32) bool {
	if file == nil || init == 0 {
		return false
	}
	init = flatUnwrapParenExpr(file, init)
	if file.FlatType(init) == "postfix_expression" && flatPostfixHasNotNullAssertion(file, init) {
		return false
	}
	text := strings.TrimSpace(file.FlatNodeText(init))
	if strings.Contains(text, ")(") {
		return true
	}
	if file.FlatType(init) == "call_expression" {
		name := flatCallExpressionName(file, init)
		return callNameCanReturnNullByConvention(name)
	}
	if file.FlatType(init) == "navigation_expression" {
		uncertain := false
		file.FlatWalkAllNodes(init, func(candidate uint32) {
			if uncertain || file.FlatType(candidate) != "call_expression" {
				return
			}
			name := flatCallExpressionName(file, candidate)
			if callNameCanReturnNullByConvention(name) {
				uncertain = true
			}
		})
		return uncertain
	}
	return false
}

func callNameCanReturnNullByConvention(name string) bool {
	if name == "" {
		return false
	}
	return strings.HasSuffix(name, "OrNull") || strings.HasPrefix(name, "find")
}

func initializerCanProduceNullFlat(file *scanner.File, init uint32) bool {
	if file == nil || init == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(init, func(candidate uint32) {
		if found {
			return
		}
		switch file.FlatType(candidate) {
		case "null":
			found = true
		case "navigation_expression":
			if flatNavigationHasSafeCall(file, candidate) {
				found = true
			}
		}
	})
	return found
}

// ---------------------------------------------------------------------------
// UnnecessarySafeCallRule detects ?. on non-null.
// ---------------------------------------------------------------------------
type UnnecessarySafeCallRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — flags ?. on a
// non-null receiver; needs resolver for nullability, falls back to
// name-based heuristic. Classified per roadmap/17.
func (r *UnnecessarySafeCallRule) Confidence() float64 { return 0.75 }

func (r *UnnecessarySafeCallRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if !flatNavigationHasSafeCall(file, idx) {
		return
	}

	// The navigation_expression has children: receiver, navigation_suffix
	// The ?. operator is in the navigation_suffix
	if file.FlatChildCount(idx) < 2 {
		return
	}

	receiver := file.FlatChild(idx, 0)
	if receiver == 0 {
		return
	}

	// Extract receiver name
	receiverText := file.FlatNodeText(receiver)
	if flatNavigationHasSafeCall(file, receiver) {
		return
	}

	// Only resolve simple identifiers — dotted member accesses cannot be
	// reliably resolved by bare name lookup (risk of name collision).
	if strings.Contains(receiverText, ".") {
		return
	}

	name := strings.TrimSpace(receiverText)
	if name == "" {
		return
	}
	structural := experiment.Enabled("unnecessary-safe-call-structural")

	if unnecessarySafeCallShouldSkipReceiver(file, idx, name, structural) {
		return
	}

	// Locate the ?. operator via the AST so the fix range is correct even when
	// the receiver text itself contains the literal `?.` substring (e.g. inside
	// a string argument to a chained call).
	safeOp := flatNavigationSafeCallOperator(file, idx)
	makeFix := func() *scanner.Fix {
		if safeOp == 0 {
			return nil
		}
		return &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(safeOp)),
			EndByte:     int(file.FlatEndByte(safeOp)),
			Replacement: ".",
		}
	}

	// If the receiver is a function parameter with an explicit non-null type,
	// the safe call is always unnecessary — emit directly without waiting for
	// the resolver or the val-only heuristic (which won't find params).
	if unnecessarySafeCallNonNullFunctionParamFlat(file, idx, name) {
		f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			fmt.Sprintf("Unnecessary safe call (?.) on non-nullable parameter '%s'.", name))
		if fix := makeFix(); fix != nil {
			f.Fix = fix
		}
		ctx.Emit(f)
		return
	}

	// Use type resolver with position-aware smart cast lookup
	if ctx.Resolver != nil {
		resolved := ctx.Resolver.ResolveByNameFlat(name, idx, file)
		if resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
			if resolved.IsNullable() {
				return // Actually nullable — safe call is needed
			}
			// Known non-null (possibly via smart cast) — flag
			f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				fmt.Sprintf("Unnecessary safe call (?.) on non-nullable '%s'.", name))
			if fix := makeFix(); fix != nil {
				f.Fix = fix
			}
			ctx.Emit(f)
			return
		}
	}

	if sameFileExplicitNonNullValueDeclaration(file, idx, name) {
		f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			fmt.Sprintf("Unnecessary safe call (?.) on non-nullable val '%s'.", name))
		if fix := makeFix(); fix != nil {
			f.Fix = fix
		}
		ctx.Emit(f)
	}
}

func unnecessarySafeCallShouldSkipReceiver(file *scanner.File, idx uint32, name string, structural bool) bool {
	if name == "this" {
		return unnecessarySafeCallNullableReceiverFlat(file, idx, structural) ||
			unnecessarySafeCallThisInExternalReceiverLambdaFlat(file, idx)
	}
	if name == "it" && unnecessarySafeCallNullableImplicitItReceiverFlat(file, idx) {
		return true
	}
	if unnecessarySafeCallNullableFunctionParamFlat(file, idx, name) {
		return true
	}
	if isMutableVarProperty(file, name) || isExplicitNullableDeclaration(file, name) {
		return true
	}
	if hasBranchNullInitializer(file, name) || hasMemberAccessInitializer(file, name) {
		return true
	}
	if hasUncertainCallInitializer(file, name) {
		return true
	}
	return isFrameworkNullableProperty(name) || isImportedMemberReference(file, name)
}

func unnecessarySafeCallNullableReceiverFlat(file *scanner.File, idx uint32, structural bool) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		pt := file.FlatType(p)
		if pt == "lambda_literal" {
			if unnecessarySafeCallLambdaHasNullableThisReceiver(file, p) {
				return true
			}
			continue
		}
		if pt != "function_declaration" && pt != "property_declaration" && pt != "getter" {
			continue
		}
		if pt == "getter" {
			if getterNullableReceiverFlat(file, p, structural) {
				return true
			}
			continue
		}
		if structural {
			for i := 0; i < file.FlatNamedChildCount(p); i++ {
				child := file.FlatNamedChild(p, i)
				if child == 0 {
					continue
				}
				if t := file.FlatType(child); t == "receiver_type" || t == "nullable_type" {
					text := strings.TrimSpace(file.FlatNodeText(child))
					return strings.HasSuffix(text, "?") || file.FlatHasChildOfType(child, "nullable_type")
				}
			}
			continue
		}
		for i := 0; i < file.FlatChildCount(p); i++ {
			child := file.FlatChild(p, i)
			switch file.FlatType(child) {
			case "receiver_type":
				recText := strings.TrimSpace(file.FlatNodeText(child))
				if strings.HasSuffix(recText, "?") {
					return true
				}
				if file.FlatHasChildOfType(child, "nullable_type") {
					return true
				}
			case "nullable_type":
				return true
			}
		}
		fnText := file.FlatNodeText(p)
		sigEnd := len(fnText)
		if pt == "function_declaration" {
			if parenIdx := strings.Index(fnText, "("); parenIdx > 0 {
				sigEnd = parenIdx
			}
		} else if colonIdx := strings.Index(fnText, ":"); colonIdx > 0 {
			sigEnd = colonIdx
		}
		prefix := fnText[:sigEnd]
		if strings.Contains(prefix, "?.") {
			return true
		}
		continue
	}
	return false
}

func unnecessarySafeCallNullableImplicitItReceiverFlat(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal":
			return unnecessarySafeCallLambdaHasNullableImplicitItReceiver(file, p)
		case "function_declaration", "property_declaration", "source_file":
			return false
		}
	}
	return false
}

func unnecessarySafeCallThisInExternalReceiverLambdaFlat(file *scanner.File, idx uint32) bool {
	lambda := uint32(0)
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal":
			lambda = p
			goto foundLambda
		case "function_declaration", "property_declaration", "source_file":
			return false
		}
	}
foundLambda:
	if lambda == 0 {
		return false
	}
	call := unnecessarySafeCallScopeFunctionCallForLambda(file, lambda)
	if call == 0 {
		return false
	}
	switch flatCallNameAny(file, call) {
	case "with", "run", "apply":
		return false
	default:
		return true
	}
}

func unnecessarySafeCallLambdaHasNullableThisReceiver(file *scanner.File, lambda uint32) bool {
	call := unnecessarySafeCallScopeFunctionCallForLambda(file, lambda)
	if call == 0 {
		return false
	}
	switch flatCallNameAny(file, call) {
	case "with", "run", "apply":
		return unnecessarySafeCallScopeFunctionPrefixHasSafeCall(file, call, lambda) ||
			unnecessarySafeCallLambdaHasRepeatedThisSafeCalls(file, lambda)
	default:
		return false
	}
}

func isImportedMemberReference(file *scanner.File, name string) bool {
	if file == nil || name == "" {
		return false
	}
	suffix := "." + name
	for _, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "import ") {
			continue
		}
		importPath := strings.TrimSpace(strings.TrimPrefix(trimmed, "import "))
		if strings.Contains(importPath, " as ") {
			continue
		}
		if strings.HasSuffix(importPath, suffix) {
			return true
		}
	}
	return false
}

func unnecessarySafeCallLambdaHasRepeatedThisSafeCalls(file *scanner.File, lambda uint32) bool {
	if file == nil || lambda == 0 {
		return false
	}
	// Count navigation_expressions whose receiver is a `this_expression` and
	// whose operator is `?.`. Walking the AST avoids the text-scan footgun of
	// matching `this?.` inside a string literal argument.
	count := 0
	file.FlatWalkAllNodes(lambda, func(candidate uint32) {
		if count >= 2 || file.FlatType(candidate) != "navigation_expression" {
			return
		}
		receiver := file.FlatChild(candidate, 0)
		if receiver == 0 || file.FlatType(receiver) != "this_expression" {
			return
		}
		if flatNavigationSafeCallOperator(file, candidate) == 0 {
			return
		}
		count++
	})
	return count >= 2
}

func unnecessarySafeCallLambdaHasNullableImplicitItReceiver(file *scanner.File, lambda uint32) bool {
	call := unnecessarySafeCallScopeFunctionCallForLambda(file, lambda)
	if call == 0 {
		return false
	}
	switch flatCallNameAny(file, call) {
	case "let", "also":
		return unnecessarySafeCallScopeFunctionPrefixHasSafeCall(file, call, lambda)
	default:
		return false
	}
}

func unnecessarySafeCallScopeFunctionCallForLambda(file *scanner.File, lambda uint32) uint32 {
	for p, ok := file.FlatParent(lambda); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "call_expression":
			return p
		case "function_declaration", "property_declaration", "source_file":
			return 0
		}
	}
	return 0
}

func unnecessarySafeCallScopeFunctionPrefixHasSafeCall(file *scanner.File, call uint32, lambda uint32) bool {
	if file == nil || call == 0 || lambda == 0 {
		return false
	}
	text := file.FlatNodeText(call)
	callStart := file.FlatStartByte(call)
	lambdaByte := file.FlatStartByte(lambda)
	if lambdaByte <= callStart {
		return strings.Contains(text, "?.")
	}
	lambdaStart := int(lambdaByte - callStart)
	if lambdaStart <= 0 || lambdaStart > len(text) {
		return strings.Contains(text, "?.")
	}
	return strings.Contains(text[:lambdaStart], "?.")
}

func getterNullableReceiverFlat(file *scanner.File, getter uint32, structural bool) bool {
	if file == nil || getter == 0 {
		return false
	}
	getterStart := file.FlatStartByte(getter)
	for i := int(getter) - 1; i > 0; i-- {
		candidate := uint32(i)
		if file.FlatType(candidate) != "property_declaration" {
			continue
		}
		if file.FlatEndByte(candidate) > getterStart {
			continue
		}
		if structural {
			for j := 0; j < file.FlatNamedChildCount(candidate); j++ {
				child := file.FlatNamedChild(candidate, j)
				if t := file.FlatType(child); t == "receiver_type" || t == "nullable_type" {
					text := strings.TrimSpace(file.FlatNodeText(child))
					return strings.HasSuffix(text, "?") || file.FlatHasChildOfType(child, "nullable_type")
				}
			}
		} else {
			for j := 0; j < file.FlatChildCount(candidate); j++ {
				child := file.FlatChild(candidate, j)
				switch file.FlatType(child) {
				case "receiver_type":
					recText := strings.TrimSpace(file.FlatNodeText(child))
					if strings.HasSuffix(recText, "?") || file.FlatHasChildOfType(child, "nullable_type") {
						return true
					}
				case "nullable_type":
					return true
				}
			}
		}
		// Fallback: scan the receiver-type prefix for `?.`. Anchor the prefix
		// to the start of the `variable_declaration` AST child rather than the
		// first `:` in the property text, so a backtick-quoted property name
		// containing `:` (`val `foo:bar`: Foo = ...`) does not shift the
		// boundary into the name itself.
		varDecl, _ := file.FlatFindChild(candidate, "variable_declaration")
		if varDecl == 0 {
			return false
		}
		propStart := file.FlatStartByte(candidate)
		varStart := file.FlatStartByte(varDecl)
		if varStart <= propStart {
			return false
		}
		prefix := file.FlatNodeText(candidate)
		offset := int(varStart - propStart)
		if offset > len(prefix) {
			offset = len(prefix)
		}
		return strings.Contains(prefix[:offset], "?.")
	}
	return false
}

func nullableThisFromLambdaReceiverCallFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) bool {
	if file == nil || idx == 0 || resolver == nil {
		return false
	}
	var lambda uint32
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal":
			lambda = p
			goto foundLambda
		case "function_declaration", "source_file":
			return false
		}
	}
foundLambda:
	if lambda == 0 {
		return false
	}
	var call uint32
	for p, ok := file.FlatParent(lambda); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "call_expression":
			call = p
			goto foundCall
		case "function_declaration", "source_file":
			return false
		}
	}
foundCall:
	if call == 0 {
		return false
	}
	receiverExpr, _ := flatCallExpressionParts(file, call)
	if receiverExpr == 0 {
		return false
	}
	if file.FlatType(receiverExpr) == "navigation_expression" && file.FlatNamedChildCount(receiverExpr) > 0 {
		receiverExpr = file.FlatNamedChild(receiverExpr, 0)
	}
	receiverName := strings.TrimSpace(file.FlatNodeText(receiverExpr))
	if receiverName == "" || strings.Contains(receiverName, ".") {
		return false
	}
	if receiverName == "this" {
		return unnecessarySafeCallNullableReceiverFlat(file, call, experiment.Enabled("unnecessary-safe-call-structural"))
	}
	if unnecessarySafeCallNullableFunctionParamFlat(file, call, receiverName) {
		return true
	}
	resolved := resolver.ResolveByNameFlat(receiverName, call, file)
	return resolved != nil && resolved.Kind != typeinfer.TypeUnknown && resolved.IsNullable()
}

func unnecessarySafeCallNullableFunctionParamFlat(file *scanner.File, idx uint32, name string) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) != "function_declaration" {
			continue
		}
		params, _ := file.FlatFindChild(p, "function_value_parameters")
		if params == 0 {
			return false
		}
		for i := 0; i < file.FlatChildCount(params); i++ {
			param := file.FlatChild(params, i)
			if file.FlatType(param) != "parameter" {
				continue
			}
			paramName := extractIdentifierFlat(file, param)
			if paramName != name {
				continue
			}
			paramText := file.FlatNodeText(param)
			if colonIdx := strings.Index(paramText, ":"); colonIdx >= 0 {
				typeText := strings.TrimSpace(paramText[colonIdx+1:])
				if eqIdx := strings.Index(typeText, "="); eqIdx >= 0 {
					typeText = strings.TrimSpace(typeText[:eqIdx])
				}
				if strings.HasSuffix(typeText, "?") {
					return true
				}
			}
			return false
		}
		return false
	}
	return false
}

// unnecessarySafeCallNonNullFunctionParamFlat returns true when `name` is found
// as a parameter of the enclosing function with an explicit non-null type
// (i.e., the type annotation exists and does NOT end with "?"). This is the
// positive complement of unnecessarySafeCallNullableFunctionParamFlat: it lets
// check() emit a finding immediately rather than relying on the resolver or the
// val-only heuristic, which cannot see function parameters.
func unnecessarySafeCallNonNullFunctionParamFlat(file *scanner.File, idx uint32, name string) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) != "function_declaration" {
			continue
		}
		params, _ := file.FlatFindChild(p, "function_value_parameters")
		if params == 0 {
			return false
		}
		for i := 0; i < file.FlatChildCount(params); i++ {
			param := file.FlatChild(params, i)
			if file.FlatType(param) != "parameter" {
				continue
			}
			paramName := extractIdentifierFlat(file, param)
			if paramName != name {
				continue
			}
			paramText := file.FlatNodeText(param)
			colonIdx := strings.Index(paramText, ":")
			if colonIdx < 0 {
				// No type annotation — can't confirm non-null; be conservative.
				return false
			}
			typeText := strings.TrimSpace(paramText[colonIdx+1:])
			if eqIdx := strings.Index(typeText, "="); eqIdx >= 0 {
				typeText = strings.TrimSpace(typeText[:eqIdx])
			}
			// Non-null iff type does not end with "?"
			return !strings.HasSuffix(typeText, "?")
		}
		return false
	}
	return false
}

func sameFileExplicitNonNullValueDeclaration(file *scanner.File, ref uint32, name string) bool {
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
		if extractIdentifierFlat(file, decl) != name {
			return
		}
		if !declarationVisibleFromReference(file, decl, ref) {
			return
		}
		if declarationHasExplicitTypeFlat(file, decl) && !declarationHasNullableTypeFlat(file, decl) {
			found = true
		}
	})
	return found
}

func declarationHasExplicitTypeFlat(file *scanner.File, decl uint32) bool {
	if file == nil || decl == 0 {
		return false
	}
	if file.FlatType(decl) == "property_declaration" {
		varDecl, _ := file.FlatFindChild(decl, "variable_declaration")
		return declarationHasExplicitTypeFlat(file, varDecl)
	}
	found := false
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if found {
			break
		}
		if file.FlatType(child) == "=" {
			break
		}
		switch file.FlatType(child) {
		case "user_type", "nullable_type", "type_identifier":
			found = true
		}
	}
	return found
}

func declarationVisibleFromReference(file *scanner.File, decl uint32, ref uint32) bool {
	declOwner := flatSemanticOwner(file, decl)
	refOwner := flatSemanticOwner(file, ref)
	if declOwner == refOwner {
		return true
	}
	refClass := flatEnclosingClassLike(file, ref)
	return refClass != 0 && declOwner == refClass
}

// ---------------------------------------------------------------------------
// NullCheckOnMutablePropertyRule detects null check on var property.
// ---------------------------------------------------------------------------
type NullCheckOnMutablePropertyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — distinguishing
// val vs var requires type resolution of the receiver. Heuristic fallback
// uses declaration patterns. Classified per roadmap/17.
func (r *NullCheckOnMutablePropertyRule) Confidence() float64 { return 0.75 }

func (r *NullCheckOnMutablePropertyRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	operand, _, ok := flatNullComparisonOperand(file, idx)
	if !ok {
		return
	}
	operand = flatUnwrapParenExpr(file, operand)
	if file.FlatType(operand) != "simple_identifier" {
		return
	}
	name := file.FlatNodeString(operand, nil)
	if name == "" || !sameOwnerVarPropertyDeclaration(file, operand, name) {
		return
	}
	if ctx.Resolver != nil {
		resolved := ctx.Resolver.ResolveByNameFlat(name, operand, file)
		if resolved != nil && resolved.Kind != typeinfer.TypeUnknown && !resolved.IsNullable() {
			return
		}
	}
	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("Null check on mutable property '%s'. The value may change between the check and the use.", name)))
}

func sameOwnerVarPropertyDeclaration(file *scanner.File, ref uint32, name string) bool {
	if file == nil || ref == 0 || name == "" {
		return false
	}
	refOwner := flatSemanticOwner(file, ref)
	refClass := flatEnclosingClassLike(file, ref)
	found := false
	file.FlatWalkNodes(0, "property_declaration", func(prop uint32) {
		propOwner := flatSemanticOwner(file, prop)
		if found || (propOwner != refOwner && propOwner != refClass) {
			return
		}
		if propertyDeclarationName(file, prop) == name && propertyDeclarationIsVar(file, prop) {
			found = true
		}
	})
	return found
}

// ---------------------------------------------------------------------------
// NullableToStringCallRule detects .toString() on nullable.
// ---------------------------------------------------------------------------
type NullableToStringCallRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — needs resolver
// to know whether the receiver is nullable; heuristic fallback matches
// common null-returning APIs. Classified per roadmap/17.
func (r *NullableToStringCallRule) Confidence() float64 { return 0.75 }

func (r *NullableToStringCallRule) NodeTypes() []string {
	return []string{"call_expression", "string_literal"}
}

func (r *NullableToStringCallRule) check(ctx *api.Context) {
	if ctx.Resolver == nil {
		return
	}
	switch ctx.File.FlatType(ctx.Idx) {
	case "call_expression":
		r.checkNullableToStringCall(ctx)
	case "string_literal":
		r.checkNullableStringTemplate(ctx)
	}
}

func (r *NullableToStringCallRule) checkNullableToStringCall(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if flatCallExpressionName(file, idx) != "toString" {
		return
	}
	nav, args := flatCallExpressionParts(file, idx)
	if nav == 0 || flatCallHasArguments(file, args) {
		return
	}
	if flatNavigationLastSuffixOperator(file, nav) == "?." {
		return
	}
	receiver := flatUnwrapParenExpr(file, flatNullCheckNavigationReceiver(file, nav))
	receiverType, ok := flatKnownResolvedType(file, ctx.Resolver, receiver)
	if !ok || !receiverType.IsNullable() {
		return
	}
	confidence, ok := flatNullableToStringCallConfidence(ctx, idx, receiver)
	if !ok {
		return
	}
	if flatSameFileNullableToStringExtensionMatches(file, receiverType.Name) {
		return
	}

	finding := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Calling toString() on a nullable receiver may produce the string \"null\". Use a safe call with an explicit fallback instead.")
	finding.Confidence = confidence
	ctx.Emit(finding)
}

func (r *NullableToStringCallRule) checkNullableStringTemplate(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		expr := flatStringTemplateInterpolationExpression(file, child)
		if expr == 0 {
			continue
		}
		if !flatTemplateExpressionIsResolvedNullable(file, ctx.Resolver, expr) {
			continue
		}
		ctx.Emit(r.Finding(file, file.FlatRow(child)+1, file.FlatCol(child)+1,
			"Interpolating a nullable expression may produce the string \"null\". Use an explicit fallback instead."))
	}
}

func flatCallHasArguments(file *scanner.File, args uint32) bool {
	return file != nil && args != 0 && file.FlatNamedChildCount(args) > 0
}

func flatNullableToStringCallConfidence(ctx *api.Context, call uint32, receiver uint32) (float64, bool) {
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok {
		return 0, false
	}
	if target.Resolved {
		if target.QualifiedName == "kotlin.toString" || target.QualifiedName == "kotlin.Any.toString" {
			return 0.95, true
		}
		return 0, false
	}
	if _, hasOracle := ctx.Resolver.(*oracle.CompositeResolver); hasOracle {
		return 0, false
	}
	if !flatNullableToStringHasLocalEvidence(ctx.File, ctx.Resolver, receiver) {
		return 0, false
	}
	return 0.80, true
}

func flatNullableToStringHasLocalEvidence(file *scanner.File, resolver typeinfer.TypeResolver, receiver uint32) bool {
	if file == nil || resolver == nil || receiver == 0 {
		return false
	}
	switch file.FlatType(receiver) {
	case "simple_identifier", "this_expression":
		return flatReferenceHasSameFileTarget(file, receiver)
	case "call_expression":
		return flatCallHasResolvedTarget(file, resolver, receiver)
	case "navigation_expression":
		_, ok := flatNavigationResolvedMemberType(file, resolver, receiver)
		return ok
	default:
		return false
	}
}

func flatNavigationLastSuffixOperator(file *scanner.File, nav uint32) string {
	if file == nil || nav == 0 || file.FlatType(nav) != "navigation_expression" {
		return ""
	}
	var suffix uint32
	for child := file.FlatFirstChild(nav); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "navigation_suffix" {
			suffix = child
		}
	}
	for child := file.FlatFirstChild(suffix); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case ".", "?.":
			return file.FlatType(child)
		}
	}
	return ""
}

func flatSameFileNullableToStringExtensionMatches(file *scanner.File, receiverType string) bool {
	if file == nil || receiverType == "" {
		return false
	}
	matched := false
	file.FlatWalkAllNodes(0, func(candidate uint32) {
		if matched || file.FlatType(candidate) != "function_declaration" || flatFunctionName(file, candidate) != "toString" {
			return
		}
		name, nullable := flatFunctionReceiverTypeInfo(file, candidate)
		matched = nullable && (name == receiverType || name == "Any")
	})
	return matched
}

func flatFunctionReceiverTypeInfo(file *scanner.File, fn uint32) (string, bool) {
	if file == nil || fn == 0 || file.FlatType(fn) != "function_declaration" {
		return "", false
	}
	var name string
	var nullable bool
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "nullable_type":
			nullable = true
			name = flatLastIdentifierInNode(file, child)
		case "user_type", "type_identifier":
			if name == "" {
				name = flatLastIdentifierInNode(file, child)
			}
		case ".":
			return name, nullable
		case "function_value_parameters", "function_body":
			return "", false
		}
	}
	return "", false
}

func flatStringTemplateInterpolationExpression(file *scanner.File, idx uint32) uint32 {
	if file == nil || idx == 0 {
		return 0
	}
	switch file.FlatType(idx) {
	case "interpolated_identifier":
		return idx
	case "interpolated_expression", "line_string_expression", "multi_line_string_expression":
		if file.FlatNamedChildCount(idx) > 0 {
			return file.FlatNamedChild(idx, 0)
		}
	}
	return 0
}

func flatTemplateExpressionIsResolvedNullable(file *scanner.File, resolver typeinfer.TypeResolver, expr uint32) bool {
	if file == nil || resolver == nil || expr == 0 {
		return false
	}
	if file.FlatType(expr) == "interpolated_identifier" {
		resolved := resolver.ResolveByNameFlat(file.FlatNodeString(expr, nil), expr, file)
		return resolved != nil && resolved.Kind != typeinfer.TypeUnknown && resolved.IsNullable()
	}
	resolved, ok := flatKnownResolvedType(file, resolver, flatUnwrapParenExpr(file, expr))
	return ok && resolved.IsNullable()
}

// UselessElvisOnNonNullRule detects `expr ?: fallback` where `expr` is
// already non-null. Resolves the left operand via ctx.Resolver, which under
// the KAA oracle recovers platform types and substituted generics (e.g.
// getOrDefault) that source-only inference cannot.
type UselessElvisOnNonNullRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *UselessElvisOnNonNullRule) Confidence() float64 { return 0.85 }

func (r *UselessElvisOnNonNullRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if ctx.Resolver == nil {
		return
	}
	left, operand, ok := uselessElvisOperand(file, idx)
	if !ok {
		return
	}
	resolved := ctx.Resolver.ResolveFlatNode(operand, file)
	if resolved == nil {
		return
	}
	if resolved.Kind == typeinfer.TypeUnknown || resolved.Kind == typeinfer.TypeGeneric {
		return
	}
	if resolved.IsNullable() {
		return
	}
	leftEnd := int(file.FlatEndByte(left))
	elvisEnd := int(file.FlatEndByte(idx))
	leftText := strings.TrimSpace(file.FlatNodeText(left))
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("Useless elvis (?:) on non-nullable '%s'. The fallback is dead code.", leftText))
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   leftEnd,
		EndByte:     elvisEnd,
		Replacement: "",
	}
	ctx.Emit(f)
}

// ExpressionPositions enumerates left operands the rule wants resolved by
// the KAA pre-pass under --depth=thorough. Selector mirrors check's gates
// so the batched resolveExpressionTypes RPC stays tight.
func (r *UselessElvisOnNonNullRule) ExpressionPositions(file *scanner.File) []uint32 {
	if file == nil {
		return nil
	}
	var out []uint32
	file.FlatWalkNodes(0, "elvis_expression", func(idx uint32) {
		if _, operand, ok := uselessElvisOperand(file, idx); ok {
			out = append(out, operand)
		}
	})
	return out
}

// uselessElvisOperand returns (left child, unwrapped operand) when the
// elvis at idx has a resolver-friendly shape and no obvious source-level
// disqualifier. ok=false means the rule should skip this elvis.
func uselessElvisOperand(file *scanner.File, idx uint32) (left, operand uint32, ok bool) {
	if file == nil || file.FlatType(idx) != "elvis_expression" || file.FlatChildCount(idx) < 3 {
		return 0, 0, false
	}
	left = file.FlatChild(idx, 0)
	if left == 0 {
		return 0, 0, false
	}
	operand = flatUnwrapParenExpr(file, left)
	if !uselessElvisOperandResolvable(file, operand) {
		return 0, 0, false
	}
	if uselessElvisOperandShouldSkip(file, operand) {
		return 0, 0, false
	}
	return left, operand, true
}

func uselessElvisOperandResolvable(file *scanner.File, operand uint32) bool {
	if file == nil || operand == 0 {
		return false
	}
	switch file.FlatType(operand) {
	case "string_literal", "line_string_literal", "multi_line_string_literal",
		"integer_literal", "long_literal", "real_literal",
		"boolean_literal", "character_literal":
		return true
	case "simple_identifier":
		return flatReferenceHasSameFileTarget(file, operand)
	case "this_expression":
		return true
	case "navigation_expression":
		return !flatExpressionChainShortCircuits(file, operand)
	case "call_expression":
		return !flatExpressionChainShortCircuits(file, operand)
	case "postfix_expression":
		return flatPostfixHasNotNullAssertion(file, operand)
	case "as_expression":
		return true
	default:
		return false
	}
}

// flatExpressionChainShortCircuits reports whether the static result of the
// expression at idx may be null because some `?.` in its receiver/callee
// spine can short-circuit the chain. Walking only the leftmost spine —
// rather than every descendant — avoids treating `?.` inside call argument
// lists (which do not propagate null to the call's return type) as a
// short-circuit. Used by UselessElvisOnNonNull to refuse operands like
// `obj?.foo()` and `obj?.foo().bar()` whose final return-type lookup hides
// an upstream null-propagating step.
//
// Each iteration strictly descends to a child node, so the loop terminates
// naturally at any leaf (default case). Self-loop guards on the descent
// step keep a malformed AST from spinning indefinitely.
func flatExpressionChainShortCircuits(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	for idx != 0 {
		switch file.FlatType(idx) {
		case "parenthesized_expression":
			inner := file.FlatNamedChild(idx, 0)
			if inner == 0 || inner == idx {
				return false
			}
			idx = inner
		case "navigation_expression":
			if flatNavigationSafeCallOperator(file, idx) != 0 {
				return true
			}
			recv := file.FlatNamedChild(idx, 0)
			if recv == 0 || recv == idx {
				return false
			}
			idx = recv
		case "call_expression":
			callee := file.FlatNamedChild(idx, 0)
			if callee == 0 || callee == idx {
				return false
			}
			idx = callee
		case "postfix_expression":
			// `!!` asserts non-null at this level and seals the chain.
			// Other postfix forms (++/--) do not appear in elvis operands.
			return false
		default:
			return false
		}
	}
	return false
}

// uselessElvisOperandShouldSkip guards against false positives the
// resolver alone cannot rule out: mutable `var` (no smart-cast across
// statements), framework-nullable inherited properties the source
// resolver would widen to non-null, and initializers that can produce
// null at runtime.
func uselessElvisOperandShouldSkip(file *scanner.File, operand uint32) bool {
	if file == nil || operand == 0 || file.FlatType(operand) != "simple_identifier" {
		return false
	}
	name := strings.TrimSpace(file.FlatNodeText(operand))
	if name == "" {
		return true
	}
	summary := nullSafetySummaryFor(file)
	return summary.mutableVar[name] ||
		summary.memberAccessInitializer[name] ||
		summary.uncertainCallInitializer[name] ||
		summary.branchNullInitializer[name] ||
		isFrameworkNullableProperty(name)
}
