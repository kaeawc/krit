package rules

import (
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

type ObjectAnimatorBindingRule struct {
	FlatDispatchBase
	AndroidRule
}

var knownAnimatorProperties = map[string]bool{
	"alpha": true, "translationX": true, "translationY": true, "translationZ": true,
	"rotation": true, "rotationX": true, "rotationY": true,
	"scaleX": true, "scaleY": true, "x": true, "y": true, "z": true,
	"elevation": true, "pivotX": true, "pivotY": true,
}

func (r *ObjectAnimatorBindingRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ObjectAnimatorBindingRule) check(ctx *api.Context) {
	file := ctx.File
	targetExpr, propertyArg, propertyName, ok := objectAnimatorBindingCall(ctx, ctx.Idx)
	if !ok {
		return
	}
	targetType := objectAnimatorResolveTargetType(ctx.Resolver, file, targetExpr)
	if targetType == nil || targetType.Kind == typeinfer.TypeUnknown || (targetType.Name == "" && targetType.FQN == "") {
		return
	}
	classInfo := objectAnimatorClassInfo(ctx.Resolver, targetType)
	if objectAnimatorClassHasProperty(classInfo, propertyName) {
		return
	}
	if objectAnimatorTypeIsView(ctx.Resolver, targetType) {
		if knownAnimatorProperties[propertyName] {
			return
		}
		ctx.Emit(r.Finding(file, file.FlatRow(propertyArg)+1, file.FlatCol(propertyArg)+1,
			"ObjectAnimator property \""+propertyName+"\" is not a standard View property. Verify the target has a setter for this property."))
		return
	}
	if classInfo != nil {
		ctx.Emit(r.Finding(file, file.FlatRow(propertyArg)+1, file.FlatCol(propertyArg)+1,
			"ObjectAnimator property \""+propertyName+"\" does not match a setter or property on target type "+classInfo.Name+"."))
	}
}

func objectAnimatorBindingCall(ctx *api.Context, call uint32) (targetExpr, propertyArg uint32, propertyName string, ok bool) {
	file := ctx.File
	if file == nil || file.FlatType(call) != "call_expression" {
		return 0, 0, "", false
	}
	if !objectAnimatorCallIsObjectAnimator(ctx, call) {
		return 0, 0, "", false
	}
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return 0, 0, "", false
	}
	targetArg := flatNamedValueArgument(file, args, "target")
	if targetArg == 0 {
		targetArg = flatPositionalValueArgument(file, args, 0)
	}
	propertyArg = flatNamedValueArgument(file, args, "propertyName")
	if propertyArg == 0 {
		propertyArg = flatNamedValueArgument(file, args, "property")
	}
	if propertyArg == 0 {
		propertyArg = flatPositionalValueArgument(file, args, 1)
	}
	if targetArg == 0 || propertyArg == 0 {
		return 0, 0, "", false
	}
	targetExpr = objectAnimatorValueArgumentExpression(file, targetArg)
	propertyExpr := objectAnimatorValueArgumentExpression(file, propertyArg)
	propertyName, ok = objectAnimatorStringLiteralValue(file, propertyExpr)
	if targetExpr == 0 || !ok || propertyName == "" {
		return 0, 0, "", false
	}
	return targetExpr, propertyArg, propertyName, true
}

func objectAnimatorCallIsObjectAnimator(ctx *api.Context, call uint32) bool {
	file := ctx.File
	name := flatCallExpressionName(file, call)
	if name != "ofFloat" && name != "ofInt" && name != "ofObject" {
		return false
	}
	if target := objectAnimatorOracleCallTarget(ctx, call); target != "" {
		return objectAnimatorCallTargetMatches(target, name)
	}
	navExpr, _ := flatCallExpressionParts(file, call)
	ids := flatNavigationIdentifierParts(file, navExpr)
	if len(ids) == 1 {
		if ctx.Resolver == nil {
			return false
		}
		return ctx.Resolver.ResolveImport(name, file) == "android.animation.ObjectAnimator."+name
	}
	if len(ids) < 2 {
		return false
	}
	receiver := ids[len(ids)-2]
	receiverFQN := strings.Join(ids[:len(ids)-1], ".")
	switch receiverFQN {
	case "android.animation.ObjectAnimator":
		return true
	}
	if ctx.Resolver != nil {
		if imported := ctx.Resolver.ResolveImport(receiver, file); imported != "" {
			return imported == "android.animation.ObjectAnimator"
		}
		if resolved := ctx.Resolver.ResolveByNameFlat(receiver, call, file); resolved != nil &&
			resolved.Kind != typeinfer.TypeUnknown && (resolved.Name != "" || resolved.FQN != "") {
			return resolved.FQN == "android.animation.ObjectAnimator"
		}
	}
	return false
}

func objectAnimatorOracleCallTarget(ctx *api.Context, call uint32) string {
	if ctx.Resolver == nil || ctx.File == nil {
		return ""
	}
	cr, ok := ctx.Resolver.(*oracle.CompositeResolver)
	if !ok {
		return ""
	}
	lookup := cr.Oracle()
	if lookup == nil {
		return ""
	}
	return oracleLookupCallTargetFlat(lookup, ctx.File, call)
}

func objectAnimatorCallTargetMatches(target, methodName string) bool {
	want := "android.animation.ObjectAnimator." + methodName
	return target == want || strings.Contains(target, want+"(") ||
		target == "android.animation.ObjectAnimator#"+methodName ||
		strings.Contains(target, "android.animation.ObjectAnimator#"+methodName+"(")
}

func flatNavigationIdentifierParts(file *scanner.File, idx uint32) []string {
	if file == nil || idx == 0 {
		return nil
	}
	var ids []string
	file.FlatWalkAllNodes(idx, func(node uint32) {
		switch file.FlatType(node) {
		case "simple_identifier", "type_identifier":
			ids = append(ids, file.FlatNodeText(node))
		}
	})
	return ids
}

func objectAnimatorValueArgumentExpression(file *scanner.File, arg uint32) uint32 {
	if file == nil || arg == 0 {
		return 0
	}
	for child := file.FlatFirstChild(arg); child != 0; child = file.FlatNextSib(child) {
		typ := file.FlatType(child)
		if typ == "value_argument_label" || typ == "=" || typ == "," || !file.FlatIsNamed(child) {
			continue
		}
		if typ == "simple_identifier" {
			if next, ok := file.FlatNextSibling(child); ok && file.FlatType(next) == "=" {
				continue
			}
		}
		return child
	}
	return 0
}

func objectAnimatorStringLiteralValue(file *scanner.File, idx uint32) (string, bool) {
	if file == nil || idx == 0 {
		return "", false
	}
	idx = flatUnwrapParenExpr(file, idx)
	switch file.FlatType(idx) {
	case "string_literal", "line_string_literal", "multi_line_string_literal":
	default:
		return "", false
	}
	if flatContainsStringInterpolation(file, idx) {
		return "", false
	}
	text := strings.TrimSpace(file.FlatNodeText(idx))
	if strings.HasPrefix(text, "\"\"\"") && strings.HasSuffix(text, "\"\"\"") && len(text) >= 6 {
		return strings.TrimSuffix(strings.TrimPrefix(text, "\"\"\""), "\"\"\""), true
	}
	value, err := strconv.Unquote(text)
	if err != nil {
		return "", false
	}
	return value, true
}

func objectAnimatorResolveTargetType(resolver typeinfer.TypeResolver, file *scanner.File, targetExpr uint32) *typeinfer.ResolvedType {
	if resolver == nil || file == nil || targetExpr == 0 {
		return nil
	}
	targetExpr = flatUnwrapParenExpr(file, targetExpr)
	resolved := resolver.ResolveFlatNode(targetExpr, file)
	if resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
		return resolved
	}
	if file.FlatType(targetExpr) == "simple_identifier" {
		return resolver.ResolveByNameFlat(file.FlatNodeText(targetExpr), targetExpr, file)
	}
	return resolved
}

func objectAnimatorClassInfo(resolver typeinfer.TypeResolver, typ *typeinfer.ResolvedType) *typeinfer.ClassInfo {
	if resolver == nil || typ == nil {
		return nil
	}
	for _, name := range []string{typ.FQN, typ.Name} {
		if name == "" {
			continue
		}
		if info := resolver.ClassHierarchy(name); info != nil {
			return info
		}
	}
	return nil
}

func objectAnimatorClassHasProperty(info *typeinfer.ClassInfo, propertyName string) bool {
	if info == nil || propertyName == "" {
		return false
	}
	setterName := "set" + strings.ToUpper(propertyName[:1]) + propertyName[1:]
	for _, member := range info.Members {
		switch member.Kind {
		case "property":
			if member.Name == propertyName {
				return true
			}
		case "function":
			if member.Name == setterName {
				return true
			}
		}
	}
	return false
}

func objectAnimatorTypeIsView(resolver typeinfer.TypeResolver, typ *typeinfer.ResolvedType) bool {
	if typ == nil {
		return false
	}
	seen := make(map[string]bool)
	var visit func(string) bool
	visit = func(name string) bool {
		if name == "" || seen[name] {
			return false
		}
		seen[name] = true
		if name == "View" || name == "android.view.View" {
			return true
		}
		if resolver == nil {
			return false
		}
		info := resolver.ClassHierarchy(name)
		if info == nil {
			return false
		}
		if info.Name == "View" || info.FQN == "android.view.View" {
			return true
		}
		for _, supertype := range info.Supertypes {
			if visit(supertype) {
				return true
			}
		}
		return false
	}
	return visit(typ.FQN) || visit(typ.Name)
}
