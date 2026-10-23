package rules

import (
	"sort"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ---------------------------------------------------------------------------
// MissingSuperCallRule detects high-confidence framework lifecycle overrides
// that omit their required super call.
// ---------------------------------------------------------------------------
type MissingSuperCallRule struct {
	FlatDispatchBase
	BaseRule
	MustInvokeSuperAnnotations []string
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs lifecycle rule. Detection matches framework lifecycle
// hook shapes by name and annotation; project-specific wrappers can escape
// detection. Classified per roadmap/17.
func (r *MissingSuperCallRule) Confidence() float64 { return api.ConfidenceMedium }

var missingSuperCallLifecycleMethodsByOwner = map[string]map[string]bool{
	"Activity":          missingSuperCallActivityLifecycleMethods,
	"AppCompatActivity": missingSuperCallActivityLifecycleMethods,
	"ComponentActivity": missingSuperCallActivityLifecycleMethods,
	"FragmentActivity":  missingSuperCallActivityLifecycleMethods,
	"Application":       missingSuperCallApplicationLifecycleMethods,
	"Fragment":          missingSuperCallFragmentLifecycleMethods,
	"DialogFragment":    missingSuperCallFragmentLifecycleMethods,
	"Service":           missingSuperCallServiceLifecycleMethods,
}

var missingSuperCallOwnerImports = map[string][]string{
	"Activity": {
		"android.app.Activity",
	},
	"AppCompatActivity": {
		"androidx.appcompat.app.AppCompatActivity",
	},
	"ComponentActivity": {
		"androidx.activity.ComponentActivity",
	},
	"FragmentActivity": {
		"androidx.fragment.app.FragmentActivity",
	},
	"Application": {
		"android.app.Application",
	},
	"Fragment": {
		"android.app.Fragment",
		"androidx.fragment.app.Fragment",
	},
	"DialogFragment": {
		"android.app.DialogFragment",
		"androidx.fragment.app.DialogFragment",
	},
	"Service": {
		"android.app.Service",
	},
}

var missingSuperCallActivityLifecycleMethods = map[string]bool{
	"onActivityResult":           true,
	"onConfigurationChanged":     true,
	"onCreate":                   true,
	"onDestroy":                  true,
	"onLowMemory":                true,
	"onNewIntent":                true,
	"onPause":                    true,
	"onPostCreate":               true,
	"onPostResume":               true,
	"onRequestPermissionsResult": true,
	"onRestart":                  true,
	"onRestoreInstanceState":     true,
	"onResume":                   true,
	"onSaveInstanceState":        true,
	"onStart":                    true,
	"onStop":                     true,
	"onTrimMemory":               true,
}

var missingSuperCallApplicationLifecycleMethods = map[string]bool{
	"onConfigurationChanged": true,
	"onCreate":               true,
	"onLowMemory":            true,
	"onTerminate":            true,
	"onTrimMemory":           true,
}

var missingSuperCallFragmentLifecycleMethods = map[string]bool{
	"onActivityCreated":          true,
	"onAttach":                   true,
	"onConfigurationChanged":     true,
	"onCreate":                   true,
	"onDestroy":                  true,
	"onDestroyView":              true,
	"onDetach":                   true,
	"onLowMemory":                true,
	"onPause":                    true,
	"onRequestPermissionsResult": true,
	"onResume":                   true,
	"onSaveInstanceState":        true,
	"onStart":                    true,
	"onStop":                     true,
	"onTrimMemory":               true,
	"onViewStateRestored":        true,
}

var missingSuperCallServiceLifecycleMethods = map[string]bool{
	"onConfigurationChanged": true,
	"onCreate":               true,
	"onDestroy":              true,
	"onLowMemory":            true,
	"onRebind":               true,
	"onStart":                true,
	"onStartCommand":         true,
	"onTaskRemoved":          true,
	"onTrimMemory":           true,
	"onUnbind":               true,
}

func missingSuperCallHasRequiredSuperEvidence(file *scanner.File, idx uint32, name string) bool {
	classNode, ok := flatEnclosingAncestor(file, idx, "class_declaration")
	if !ok {
		return false
	}
	for owner, methods := range missingSuperCallLifecycleMethodsByOwner {
		if !methods[name] {
			continue
		}
		if missingSuperCallClassExtendsOwner(file, classNode, owner) {
			return true
		}
	}
	return false
}

// missingSuperCallParentMethodHasAnnotation reports whether the method
// being overridden has, on a same-file supertype declaration, an
// annotation in the configured list (e.g. `@CallSuper`,
// `@OverridingMethodsMustInvokeSuper`). Cross-file lookup is
// intentionally out of scope; same-file lookup is sufficient to
// observe the configured option for projects where the base class
// lives in the same source file as its subclasses (a common
// fixture/dispatch pattern).
//
// The walk is bounded:
//   - only the immediate enclosing class is consulted as the subclass
//   - supertypes are matched by simple name against same-file
//     class_declaration nodes
//   - candidate method declarations must share the override's name
//   - annotation matching uses the simple name (last `.`-segment) of
//     each pattern entry to allow either FQN or simple-name configs.
func missingSuperCallParentMethodHasAnnotation(file *scanner.File, idx uint32, name string, annotations []string) bool {
	if file == nil || idx == 0 || name == "" || len(annotations) == 0 {
		return false
	}
	subclass, ok := flatEnclosingAncestor(file, idx, "class_declaration")
	if !ok {
		return false
	}
	supertypes := androidDirectSupertypesFlat(file, subclass)
	if len(supertypes) == 0 {
		return false
	}
	supers := make(map[string]bool, len(supertypes))
	for _, s := range supertypes {
		if s.simple != "" {
			supers[s.simple] = true
		}
	}
	if len(supers) == 0 {
		return false
	}
	annSimple := make(map[string]bool, len(annotations))
	for _, ann := range annotations {
		ann = strings.TrimSpace(ann)
		if ann == "" {
			continue
		}
		if dot := strings.LastIndex(ann, "."); dot >= 0 && dot+1 < len(ann) {
			annSimple[ann[dot+1:]] = true
		} else {
			annSimple[ann] = true
		}
	}
	if len(annSimple) == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(0, "class_declaration", func(candidate uint32) {
		if found || candidate == subclass {
			return
		}
		clsName := extractIdentifierFlat(file, candidate)
		if clsName == "" || !supers[clsName] {
			return
		}
		body, _ := file.FlatFindChild(candidate, "class_body")
		if body == 0 {
			return
		}
		for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) != "function_declaration" {
				continue
			}
			if extractIdentifierFlat(file, child) != name {
				continue
			}
			for ann := range annSimple {
				if flatHasAnnotationNamed(file, child, ann) {
					found = true
					return
				}
			}
		}
	})
	return found
}

func missingSuperCallClassExtendsOwner(file *scanner.File, classNode uint32, owner string) bool {
	if file == nil || classNode == 0 || owner == "" {
		return false
	}
	for _, supertype := range androidDirectSupertypesFlat(file, classNode) {
		if supertype.simple != owner {
			continue
		}
		for _, fqn := range missingSuperCallOwnerImports[owner] {
			if supertype.name == fqn || (!supertype.qualified && fileImportsFQN(file, fqn)) {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// MissingUseCallRule detects Closeable/AutoCloseable resource creation without
// .use {} block. Operates on call_expression AST nodes.
// ---------------------------------------------------------------------------
type MissingUseCallRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs lifecycle rule. Detection matches framework lifecycle
// hook shapes by name and annotation; project-specific wrappers can escape
// detection. Classified per roadmap/17.
func (r *MissingUseCallRule) Confidence() float64 { return api.ConfidenceMedium }

// Known Closeable/AutoCloseable types commonly constructed directly.
var closeableTypes = map[string]bool{
	"FileInputStream":       true,
	"FileOutputStream":      true,
	"BufferedReader":        true,
	"BufferedWriter":        true,
	"InputStreamReader":     true,
	"OutputStreamWriter":    true,
	"PrintWriter":           true,
	"RandomAccessFile":      true,
	"Socket":                true,
	"ServerSocket":          true,
	"DataInputStream":       true,
	"DataOutputStream":      true,
	"ObjectInputStream":     true,
	"ObjectOutputStream":    true,
	"FileReader":            true,
	"FileWriter":            true,
	"PrintStream":           true,
	"ByteArrayInputStream":  true,
	"ByteArrayOutputStream": true,
	"BufferedInputStream":   true,
	"BufferedOutputStream":  true,
	"GZIPInputStream":       true,
	"GZIPOutputStream":      true,
	"ZipInputStream":        true,
	"ZipOutputStream":       true,
}

var closeableTypeFQNs = map[string]bool{
	"java.io.FileInputStream":        true,
	"java.io.FileOutputStream":       true,
	"java.io.BufferedReader":         true,
	"java.io.BufferedWriter":         true,
	"java.io.InputStreamReader":      true,
	"java.io.OutputStreamWriter":     true,
	"java.io.PrintWriter":            true,
	"java.io.RandomAccessFile":       true,
	"java.net.Socket":                true,
	"java.net.ServerSocket":          true,
	"java.io.DataInputStream":        true,
	"java.io.DataOutputStream":       true,
	"java.io.ObjectInputStream":      true,
	"java.io.ObjectOutputStream":     true,
	"java.io.FileReader":             true,
	"java.io.FileWriter":             true,
	"java.io.PrintStream":            true,
	"java.io.ByteArrayInputStream":   true,
	"java.io.ByteArrayOutputStream":  true,
	"java.io.BufferedInputStream":    true,
	"java.io.BufferedOutputStream":   true,
	"java.util.zip.GZIPInputStream":  true,
	"java.util.zip.GZIPOutputStream": true,
	"java.util.zip.ZipInputStream":   true,
	"java.util.zip.ZipOutputStream":  true,
}

func closeableConstructorCallees() []string {
	callees := make([]string, 0, len(closeableTypes))
	for name := range closeableTypes {
		callees = append(callees, name)
	}
	sort.Strings(callees)
	return callees
}

func missingUseCalleeIdentFlat(file *scanner.File, idx uint32) string {
	return flatCallExpressionName(file, idx)
}

func missingUseCloseableConstructorConfirmed(ctx *api.Context, idx uint32) (string, bool) {
	if ctx.File == nil || ctx.File.FlatType(idx) != "call_expression" {
		return "", false
	}
	file := ctx.File
	callee := missingUseCalleeIdentFlat(file, idx)
	if callee == "" {
		return "", false
	}
	if ctx.Resolver != nil {
		if target, ok := semantics.ResolveCallTarget(ctx, idx); ok && target.Resolved {
			if closeableTypeFQNs[strings.TrimSuffix(strings.ReplaceAll(target.QualifiedName, "#", "."), ".<init>")] {
				return callee, true
			}
		}
		if typ := ctx.Resolver.ResolveFlatNode(idx, file); closeableResolvedTypeMatches(typ, ctx.Resolver) {
			return callee, true
		}
		if fqn := ctx.Resolver.ResolveImport(callee, file); closeableTypeFQNs[fqn] {
			return callee, true
		}
	}
	if closeableTypes[callee] && !sameFileDeclarationNamed(file, callee) {
		return callee, true
	}
	return "", false
}

func closeableResolvedTypeMatches(typ *typeinfer.ResolvedType, resolver typeinfer.TypeResolver) bool {
	if typ == nil || typ.Kind == typeinfer.TypeUnknown {
		return false
	}
	if closeableTypeFQNs[typ.FQN] || typ.FQN == "java.io.Closeable" || typ.FQN == "java.lang.AutoCloseable" {
		return true
	}
	for _, st := range typ.Supertypes {
		if closeableTypeFQNs[st] || st == "java.io.Closeable" || st == "java.lang.AutoCloseable" {
			return true
		}
	}
	if resolver != nil {
		name := typ.FQN
		if name == "" {
			name = typ.Name
		}
		if name != "" {
			if info := resolver.ClassHierarchy(name); info != nil {
				for _, st := range info.Supertypes {
					if st == "java.io.Closeable" || st == "java.lang.AutoCloseable" {
						return true
					}
				}
			}
		}
	}
	return false
}

// Restructured: check .use {} chain using file content.
func missingUseHasUseChainFlat(file *scanner.File, idx uint32) bool {
	parent, ok := file.FlatParent(idx)
	if !ok {
		return false
	}

	if file.FlatType(parent) == "navigation_expression" {
		if missingUseFlatNavEndsWithUse(file, parent) {
			return true
		}
		if gp, ok := file.FlatParent(parent); ok && file.FlatType(gp) == "call_expression" {
			if ggp, ok := file.FlatParent(gp); ok && file.FlatType(ggp) == "navigation_expression" {
				if missingUseFlatNavEndsWithUse(file, ggp) {
					return true
				}
			}
		}
	}

	return false
}

func missingUseFlatNavEndsWithUse(file *scanner.File, nav uint32) bool {
	for i := 0; i < file.FlatChildCount(nav); i++ {
		child := file.FlatChild(nav, i)
		if file.FlatType(child) != "navigation_suffix" {
			continue
		}
		if ident, ok := file.FlatFindChild(child, "simple_identifier"); ok && file.FlatNodeTextEquals(ident, "use") {
			return true
		}
	}
	return false
}

func missingUseAssignedWithUseFlat(file *scanner.File, idx uint32) bool {
	parent, ok := file.FlatParent(idx)
	if !ok {
		return false
	}
	for ok && file.FlatType(parent) != "property_declaration" {
		if file.FlatType(parent) == "call_expression" || file.FlatType(parent) == "navigation_expression" {
			parent, ok = file.FlatParent(parent)
			continue
		}
		break
	}
	if !ok || file.FlatType(parent) != "property_declaration" {
		return false
	}
	varName := propertyDeclarationNameFlat(file, parent)
	if varName == "" {
		return false
	}
	scope, ok := file.FlatParent(parent)
	if !ok {
		return false
	}
	return missingUseLaterCallOnVariableFlat(file, scope, parent, varName, "use")
}

func missingUseLaterCallOnVariableFlat(file *scanner.File, scope uint32, declaration uint32, varName string, callee string) bool {
	if file == nil || scope == 0 || declaration == 0 || varName == "" || callee == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(scope, "call_expression", func(call uint32) {
		if found || file.FlatStartByte(call) <= file.FlatEndByte(declaration) {
			return
		}
		if flatCallExpressionName(file, call) != callee {
			return
		}
		navExpr, _ := flatCallExpressionParts(file, call)
		if navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
			return
		}
		receiver := file.FlatNamedChild(navExpr, 0)
		if semantics.ReferenceName(file, receiver) == varName {
			found = true
		}
	})
	return found
}

// missingUseIsClassProperty checks if the node is a class-level property declaration.
func missingUseIsClassPropertyFlat(file *scanner.File, idx uint32) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) == "property_declaration" {
			if gp, ok := file.FlatParent(parent); ok && file.FlatType(gp) == "class_body" {
				return true
			}
		}
		if file.FlatType(parent) == "function_declaration" || file.FlatType(parent) == "function_body" {
			return false
		}
	}
	return false
}

// missingUseIsArgument checks if the node is passed as an argument to another function call.
func missingUseIsArgumentFlat(file *scanner.File, idx uint32) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "value_argument":
			return true
		case "call_suffix", "statements", "function_body":
			return false
		}
	}
	return false
}

// missingUseIsReturnExpression checks if the node is a return expression.
func missingUseIsReturnExpressionFlat(file *scanner.File, idx uint32) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "jump_expression":
			return true
		case "statements", "function_body":
			return false
		}
	}
	return false
}
