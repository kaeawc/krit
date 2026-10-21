package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/javafacts"
	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

func semanticTargetOwnerMatches(target string, owners ...string) bool {
	target = strings.TrimSpace(strings.ReplaceAll(target, "#", "."))
	if cut := strings.Index(target, "("); cut >= 0 {
		target = target[:cut]
	}
	for _, owner := range owners {
		owner = strings.TrimSpace(strings.ReplaceAll(owner, "#", "."))
		if target == owner || strings.HasPrefix(target, owner+".") {
			return true
		}
	}
	return false
}

func semanticResolvedTargetMatches(ctx *api.Context, call uint32, fqns ...string) bool {
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok || !target.Resolved {
		return false
	}
	got := strings.TrimSpace(strings.ReplaceAll(target.QualifiedName, "#", "."))
	for _, want := range fqns {
		want = strings.TrimSpace(strings.ReplaceAll(want, "#", "."))
		if got == want || strings.HasSuffix(got, "."+want) {
			return true
		}
	}
	return false
}

func semanticCallReceiverTypeMatches(ctx *api.Context, call uint32, types ...string) bool {
	if javaSemanticCallReceiverTypeMatches(ctx, call, types...) {
		return true
	}
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok || !target.Receiver.Valid() {
		return false
	}
	typ, ok := semantics.ExpressionType(ctx, target.Receiver.Node)
	if !ok {
		return false
	}
	return semanticTypeNameMatches(typ.Name, types...) || semanticTypeNameMatches(typ.FQN, types...)
}

func semanticTypeNameMatches(got string, wants ...string) bool {
	got = strings.TrimSpace(strings.ReplaceAll(got, "$", "."))
	if got == "" {
		return false
	}
	for _, want := range wants {
		want = strings.TrimSpace(strings.ReplaceAll(want, "$", "."))
		if got == want || strings.HasSuffix(got, "."+want) {
			return true
		}
	}
	return false
}

func semanticCallTargetOrReceiverType(ctx *api.Context, call uint32, owners []string, receiverTypes []string) bool {
	if javaSemanticCallTargetOrReceiverType(ctx, call, owners, receiverTypes) {
		return true
	}
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok {
		return false
	}
	if target.Resolved {
		return semanticTargetOwnerMatches(target.QualifiedName, owners...)
	}
	return semanticCallReceiverTypeMatches(ctx, call, receiverTypes...)
}

func javaSemanticCallFact(ctx *api.Context, call uint32) (javafacts.CallFact, bool) {
	if ctx.File == nil || ctx.JavaSemanticFacts == nil || ctx.File.Language != scanner.LangJava {
		return javafacts.CallFact{}, false
	}
	return ctx.JavaSemanticFacts.CallAt(ctx.File.Path, ctx.File.FlatRow(call)+1, ctx.File.FlatCol(call)+1)
}

func javaSemanticCallReceiverTypeMatches(ctx *api.Context, call uint32, types ...string) bool {
	fact, ok := javaSemanticCallFact(ctx, call)
	return ok && javaProfileTypeMatches(ctx, fact.ReceiverType, types...)
}

func javaSemanticCallTargetOrReceiverType(ctx *api.Context, call uint32, owners []string, receiverTypes []string) bool {
	fact, ok := javaSemanticCallFact(ctx, call)
	if !ok {
		return false
	}
	return javaProfileTypeMatches(ctx, fact.MethodOwner, owners...) ||
		javaProfileTypeMatches(ctx, fact.ReceiverType, receiverTypes...)
}

func javaProfileTypeMatches(ctx *api.Context, got string, wants ...string) bool {
	profile := librarymodel.EnsureFacts(ctx.LibraryFacts).Java
	for _, want := range wants {
		if profile.IsSubtypeCandidate(want, got) || semanticTypeNameMatches(got, want) {
			return true
		}
	}
	return false
}

func javaProfileMethodReturn(ctx *api.Context, owner, receiver, method string, arity int) string {
	profile := librarymodel.EnsureFacts(ctx.LibraryFacts).Java
	if ret := profile.MethodReturn(owner, method, arity); ret != "" {
		return ret
	}
	return profile.MethodReturn(receiver, method, arity)
}

func obsoleteComposeModifierCall(ctx *api.Context, call uint32) (name string, replacement string, ok bool) {
	file := ctx.File
	name = flatCallExpressionName(file, call)
	replacements := map[string]string{
		"preferredWidth":  "width",
		"preferredHeight": "height",
		"preferredSize":   "size",
	}
	replacement, ok = replacements[name]
	if !ok {
		return "", "", false
	}
	target, targetOK := semantics.ResolveCallTarget(ctx, call)
	if targetOK && target.Resolved {
		if semanticTargetOwnerMatches(target.QualifiedName, "androidx.compose.ui.Modifier", "androidx.compose.foundation.layout") {
			return name, replacement, true
		}
		return "", "", false
	}
	nav, _ := flatCallExpressionParts(file, call)
	if nav == 0 {
		return "", "", false
	}
	navSegments := flatNavigationChainIdentifiers(file, nav)
	if len(navSegments) >= 2 && navSegments[0] == "Modifier" && navSegments[len(navSegments)-1] == name {
		return name, replacement, true
	}
	receiver := file.FlatNamedChild(nav, 0)
	if receiver != 0 {
		segments := flatNavigationChainIdentifiers(file, receiver)
		if len(segments) > 0 && segments[0] == "Modifier" {
			return name, replacement, true
		}
		if semanticTypeNameMatches(semanticReferenceTypeName(ctx, receiver), "androidx.compose.ui.Modifier", "Modifier") {
			return name, replacement, true
		}
	}
	return "", "", false
}

func semanticReferenceTypeName(ctx *api.Context, node uint32) string {
	typ, ok := semantics.ExpressionType(ctx, node)
	if !ok {
		return ""
	}
	if typ.FQN != "" {
		return typ.FQN
	}
	return typ.Name
}

func semanticsReceiverNode(ctx *api.Context, call uint32) uint32 {
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok || !target.Receiver.Valid() {
		return 0
	}
	return target.Receiver.Node
}

func astReceiverNodeFromCall(file *scanner.File, call uint32) uint32 {
	nav, _ := flatCallExpressionParts(file, call)
	if nav == 0 {
		return 0
	}
	var receiver uint32
	for child := file.FlatFirstChild(nav); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if file.FlatType(child) == "navigation_suffix" {
			break
		}
		receiver = child
	}
	return receiver
}

func receiverContainsCallName(file *scanner.File, call uint32, names ...string) bool {
	receiver := semanticsReceiverNode(&api.Context{File: file}, call)
	if receiver == 0 {
		receiver = astReceiverNodeFromCall(file, call)
	}
	if receiver == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(receiver, "call_expression", func(candidate uint32) {
		if found {
			return
		}
		callee := flatCallExpressionName(file, candidate)
		for _, name := range names {
			if callee == name {
				found = true
				return
			}
		}
	})
	return found
}

func callReceiverConstructedOrTyped(ctx *api.Context, call uint32, constructorName string, receiverTypes ...string) bool {
	if semanticCallTargetOrReceiverType(ctx, call, receiverTypes, receiverTypes) {
		return true
	}
	return receiverDeclaredFromCall(ctx.File, call, constructorName) || receiverContainsCallName(ctx.File, call, constructorName)
}

func callReceiverParameterHasType(ctx *api.Context, call uint32, typeNames ...string) bool {
	if ctx.File == nil || call == 0 {
		return false
	}
	receiver := semanticsReceiverNode(ctx, call)
	if receiver == 0 {
		receiver = astReceiverNodeFromCall(ctx.File, call)
	}
	name := semantics.ReferenceName(ctx.File, receiver)
	if name == "" {
		return false
	}
	fn, ok := flatEnclosingFunction(ctx.File, call)
	if !ok {
		return false
	}
	for _, typeName := range typeNames {
		if parameterHasTypeFlat(ctx.File, fn, name, typeName) {
			return true
		}
	}
	return false
}

func whileConditionHasCallName(file *scanner.File, stmt uint32, names ...string) bool {
	if file == nil || stmt == 0 || file.FlatType(stmt) != "while_statement" {
		return false
	}
	for child := file.FlatFirstChild(stmt); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "control_structure_body" || file.FlatType(child) == "statements" {
			return false
		}
		if subtreeHasCallName(file, child, names...) {
			return true
		}
	}
	return false
}

func subtreeHasCallName(file *scanner.File, root uint32, names ...string) bool {
	if file == nil || root == 0 || len(names) == 0 {
		return false
	}
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	found := false
	file.FlatWalkNodes(root, "call_expression", func(call uint32) {
		if found {
			return
		}
		if allowed[flatCallNameAny(file, call)] {
			found = true
		}
	})
	file.FlatWalkNodes(root, "method_invocation", func(call uint32) {
		if found {
			return
		}
		if allowed[databaseCallName(file, call)] {
			found = true
		}
	})
	return found
}

func subtreeHasReferenceName(file *scanner.File, root uint32, names ...string) bool {
	if file == nil || root == 0 || len(names) == 0 {
		return false
	}
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	found := false
	file.FlatWalkAllNodes(root, func(n uint32) {
		if found {
			return
		}
		switch file.FlatType(n) {
		case "simple_identifier", "type_identifier", "identifier", "scoped_type_identifier":
			if allowed[file.FlatNodeText(n)] {
				found = true
			}
		}
	})
	return found
}

func constructorParameterTypeFlags(file *scanner.File, ctor uint32, typeNames ...string) map[string]bool {
	flags := make(map[string]bool, len(typeNames))
	if file == nil || ctor == 0 {
		return flags
	}
	allowed := make(map[string]bool, len(typeNames))
	for _, name := range typeNames {
		allowed[name] = true
	}
	for _, paramType := range []string{"parameter", "class_parameter"} {
		file.FlatWalkNodes(ctor, paramType, func(param uint32) {
			if nearestConstructorAncestorFlat(file, param) != ctor {
				return
			}
			file.FlatWalkNodes(param, "user_type", func(typ uint32) {
				name := apiNodeNameFlat(file, typ)
				for want := range allowed {
					if semanticTypeNameMatches(name, want) {
						flags[want] = true
					}
				}
			})
		})
	}
	return flags
}

func callHasBooleanArgument(file *scanner.File, call uint32, want bool) bool {
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(args, "boolean_literal", func(lit uint32) {
		if found {
			return
		}
		found = (file.FlatNodeText(lit) == "true") == want
	})
	return found
}

func callArgHasBoolean(file *scanner.File, arg uint32, want bool) bool {
	expr := flatValueArgumentExpression(file, arg)
	if expr == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(expr, "boolean_literal", func(lit uint32) {
		if found {
			return
		}
		found = (file.FlatNodeText(lit) == "true") == want
	})
	return found
}

func callHasReferenceArgument(file *scanner.File, call uint32, names ...string) bool {
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return false
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) == "value_argument" && callArgHasReference(file, arg, names...) {
			return true
		}
	}
	return false
}

func callArgHasReference(file *scanner.File, arg uint32, names ...string) bool {
	expr := flatValueArgumentExpression(file, arg)
	if expr == 0 {
		return false
	}
	return subtreeHasReferenceName(file, expr, names...)
}

func postfixExpressionHasBangBang(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "postfix_expression" {
		return false
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) && file.FlatNodeTextEquals(child, "!!") {
			return true
		}
	}
	return false
}
