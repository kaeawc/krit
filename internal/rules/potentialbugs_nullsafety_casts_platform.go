package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func unsafeCastKnownPlatformCastFlat(ctx *api.Context, source, target uint32, targetName string) bool {
	if ctx.File == nil {
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

func unsafeCastGetSystemServiceMatchesFlat(ctx *api.Context, call, target uint32, targetName string) bool {
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

func unsafeCastPlatformCallConfirmedFlat(ctx *api.Context, call uint32, callee string, receiverOK func(*typeinfer.ResolvedType) bool) bool {
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

func unsafeCastEnclosingOwnerMatchesFlat(ctx *api.Context, idx uint32, receiverOK func(*typeinfer.ResolvedType) bool) bool {
	if ctx.Resolver == nil || ctx.File == nil {
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

func unsafeCastOracleCallTargetFlat(ctx *api.Context, idx uint32) string {
	if ctx.Resolver == nil || ctx.File == nil {
		return ""
	}
	var oracleLookup oracle.Lookup
	if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
		oracleLookup = cr.Oracle()
	}
	if oracleLookup == nil {
		return ""
	}
	return oracleLookupCallTargetFlat(oracleLookup, ctx.File, idx)
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
