package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/experiment"
	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

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
func (r *UnnecessarySafeCallRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UnnecessarySafeCallRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if !flatNavigationHasSafeCall(file, idx) {
		return
	}

	// Trust the compiler's UNNECESSARY_SAFE_CALL verdict when available: it is
	// emitted only for a genuinely non-null receiver, so the safe-cast /
	// indexing / name-collision guards below are unnecessary on this path, and
	// it covers dotted receivers the name-based heuristic skips.
	if projectDiagnostic(ctx, DiagnosticProjection{
		RuleID:       "UnnecessarySafeCall",
		FactoryNames: []string{"UNNECESSARY_SAFE_CALL"},
		Message: func(ctx *api.Context, _ oracle.Diagnostic) string {
			recv := ""
			if r0 := ctx.File.FlatChild(ctx.Idx, 0); r0 != 0 {
				recv = strings.TrimSpace(ctx.File.FlatNodeText(r0))
			}
			return fmt.Sprintf("Unnecessary safe call (?.) on non-nullable '%s'.", recv)
		},
		Fix: func(ctx *api.Context, _ oracle.Diagnostic) *scanner.Fix {
			op := flatNavigationSafeCallOperator(ctx.File, ctx.Idx)
			if op == 0 {
				return nil
			}
			return &scanner.Fix{ByteMode: true, StartByte: int(ctx.File.FlatStartByte(op)), EndByte: int(ctx.File.FlatEndByte(op)), Replacement: "."}
		},
		Confidence: api.ConfidenceVeryHigh,
	}) {
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

	// A safe-cast receiver `(x as? T)?.foo` is unconditionally nullable, so the
	// `?.` is required — never flag it, whatever the resolver reports for the
	// cast target type.
	if flatAsExpressionIsSafeCast(file, receiver) {
		return
	}

	// An indexed receiver `m[k]?.foo` is nullable for a Map; skip it (see
	// flatExpressionIsIndexing).
	if flatExpressionIsIndexing(file, receiver) {
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
	// emit directly only after the resolver confirms that its type is genuine;
	// the val-only heuristic below cannot see function parameters.
	if unnecessarySafeCallNonNullFunctionParamFlat(file, idx, name) {
		if ctx.Resolver == nil {
			return
		}
		resolved := ctx.Resolver.ResolveByNameFlat(name, idx, file)
		if resolved == nil || !resolved.Resolved || resolved.Kind == typeinfer.TypeGeneric || resolved.IsNullable() {
			return
		}
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
			if !resolved.Resolved {
				return
			}
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
// check() consider the parameter-specific path before the val-only heuristic,
// while the resolver still confirms that the annotation names a genuine type.
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
