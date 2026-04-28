package rules

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/rules/semantics"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ---------------------------------------------------------------------------
// ExitOutsideMainRule detects exitProcess()/System.exit() outside main.
// ---------------------------------------------------------------------------
type ExitOutsideMainRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs lifecycle rule. Detection matches framework lifecycle
// hook shapes by name and annotation; project-specific wrappers can escape
// detection. Classified per roadmap/17.
func (r *ExitOutsideMainRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// ExplicitGarbageCollectionCallRule detects System.gc() calls.
// ---------------------------------------------------------------------------
type ExplicitGarbageCollectionCallRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs lifecycle rule. Detection matches framework lifecycle
// hook shapes by name and annotation; project-specific wrappers can escape
// detection. Classified per roadmap/17.
func (r *ExplicitGarbageCollectionCallRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// InvalidRangeRule detects backwards ranges like 10..1.
// ---------------------------------------------------------------------------
type InvalidRangeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs lifecycle rule. Detection matches framework lifecycle
// hook shapes by name and annotation; project-specific wrappers can escape
// detection. Classified per roadmap/17.
func (r *InvalidRangeRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// IteratorHasNextCallsNextMethodRule detects hasNext() calling next().
// ---------------------------------------------------------------------------
type IteratorHasNextCallsNextMethodRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs lifecycle rule. Detection matches framework lifecycle
// hook shapes by name and annotation; project-specific wrappers can escape
// detection. Classified per roadmap/17.
func (r *IteratorHasNextCallsNextMethodRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// IteratorNotThrowingNoSuchElementExceptionRule detects next() without throw.
// ---------------------------------------------------------------------------
type IteratorNotThrowingNoSuchElementExceptionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs lifecycle rule. Detection matches framework lifecycle
// hook shapes by name and annotation; project-specific wrappers can escape
// detection. Classified per roadmap/17.
func (r *IteratorNotThrowingNoSuchElementExceptionRule) Confidence() float64 { return 0.75 }

// enclosingImplementsIterator returns true if the node's enclosing class
// has a delegation specifier naming Iterator / MutableIterator / ListIterator
// (with or without a qualifier).
func enclosingImplementsIteratorFlat(ctx *v2.Context, idx uint32) bool {
	if ctx == nil || ctx.File == nil {
		return false
	}
	file := ctx.File
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		pt := file.FlatType(p)
		if pt != "class_declaration" && pt != "object_declaration" {
			continue
		}
		found := false
		file.FlatWalkNodes(p, "delegation_specifier", func(c uint32) {
			if found {
				return
			}
			if iteratorSupertypeConfirmed(ctx, c) {
				found = true
			}
		})
		return found
	}
	return false
}

func iteratorSupertypeConfirmed(ctx *v2.Context, idx uint32) bool {
	if ctx == nil || ctx.File == nil || idx == 0 {
		return false
	}
	file := ctx.File
	if ctx.Resolver != nil {
		if typ := ctx.Resolver.ResolveFlatNode(idx, file); iteratorTypeMatches(typ) {
			return true
		}
	}
	found := false
	file.FlatWalkAllNodes(idx, func(n uint32) {
		if found {
			return
		}
		switch file.FlatType(n) {
		case "user_type", "navigation_expression", "simple_identifier", "type_identifier":
			name := semantics.ReferenceName(file, n)
			if name == "" {
				name = extractIdentifierFlat(file, n)
			}
			if name == "" {
				return
			}
			if iteratorSimpleName(name) {
				if ctx.Resolver != nil {
					if fqn := ctx.Resolver.ResolveImport(name, file); fqn != "" {
						found = iteratorFQN(fqn)
						return
					}
				}
				if !sameFileDeclarationNamed(file, name) {
					found = true
				}
			}
			segments := flatNavigationChainIdentifiers(file, n)
			if len(segments) > 0 && iteratorFQN(strings.Join(segments, ".")) {
				found = true
			}
		}
	})
	return found
}

func iteratorTypeMatches(typ *typeinfer.ResolvedType) bool {
	if typ == nil || typ.Kind == typeinfer.TypeUnknown {
		return false
	}
	if iteratorFQN(typ.FQN) || iteratorSimpleName(typ.Name) {
		return true
	}
	for _, st := range typ.Supertypes {
		if iteratorFQN(st) || iteratorSimpleName(st) {
			return true
		}
	}
	return false
}

func iteratorSimpleName(name string) bool {
	switch name {
	case "Iterator", "MutableIterator", "ListIterator":
		return true
	default:
		return false
	}
}

func iteratorFQN(name string) bool {
	switch name {
	case "kotlin.collections.Iterator", "kotlin.collections.MutableIterator", "kotlin.collections.ListIterator", "java.util.Iterator":
		return true
	default:
		return false
	}
}

func sameFileDeclarationNamed(file *scanner.File, name string) bool {
	if file == nil || name == "" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(0, func(n uint32) {
		if found {
			return
		}
		switch file.FlatType(n) {
		case "class_declaration", "object_declaration", "function_declaration", "property_declaration", "type_alias":
			if extractIdentifierFlat(file, n) == name || propertyDeclarationNameFlat(file, n) == name {
				found = true
			}
		}
	})
	return found
}

func functionThrowsNoSuchElementExceptionFlat(ctx *v2.Context, body uint32) bool {
	if ctx == nil || ctx.File == nil || body == 0 {
		return false
	}
	file := ctx.File
	found := false
	file.FlatWalkNodes(body, "jump_expression", func(jmp uint32) {
		if found {
			return
		}
		first := file.FlatFirstChild(jmp)
		if first == 0 || !file.FlatNodeTextEquals(first, "throw") {
			return
		}
		file.FlatWalkNodes(jmp, "call_expression", func(call uint32) {
			if found || flatCallExpressionName(file, call) != "NoSuchElementException" {
				return
			}
			if ctx.Resolver != nil {
				if target, ok := semantics.ResolveCallTarget(ctx, call); ok && target.Resolved {
					found = strings.HasPrefix(strings.ReplaceAll(target.QualifiedName, "#", "."), "kotlin.NoSuchElementException.") ||
						strings.HasPrefix(strings.ReplaceAll(target.QualifiedName, "#", "."), "java.util.NoSuchElementException.")
					return
				}
			}
			if !sameFileDeclarationNamed(file, "NoSuchElementException") {
				found = true
			}
		})
	})
	return found
}

// ---------------------------------------------------------------------------
// LateinitUsageRule detects lateinit var.
// ---------------------------------------------------------------------------
type LateinitUsageRule struct {
	FlatDispatchBase
	BaseRule
	IgnoreOnClassesPattern *regexp.Regexp
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs lifecycle rule. Detection matches framework lifecycle
// hook shapes by name and annotation; project-specific wrappers can escape
// detection. Classified per roadmap/17.
func (r *LateinitUsageRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// MissingPackageDeclarationRule detects .kt file without package statement.
// ---------------------------------------------------------------------------
type MissingPackageDeclarationRule struct {
	LineBase
	BaseRule
}

// Confidence bumps this line rule from the 0.75 line-rule default to
// 0.95 — the check walks the first non-blank, non-comment line and
// verifies it starts with "package ". Deterministic with no
// heuristic path.
func (r *MissingPackageDeclarationRule) Confidence() float64 { return 0.95 }

func (r *MissingPackageDeclarationRule) check(ctx *v2.Context) {
	file := ctx.File
	if !strings.HasSuffix(file.Path, ".kt") {
		return
	}
	for _, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || scanner.IsCommentLine(line) {
			continue
		}
		if strings.HasPrefix(trimmed, "package ") {
			return
		}
		// First non-comment, non-blank line is not a package declaration
		f := r.Finding(file, 1, 1,
			"Missing package declaration in Kotlin file.")
		f.Fix = derivePackageFix(file)
		ctx.Emit(f)
		return
	}
	f := r.Finding(file, 1, 1,
		"Missing package declaration in Kotlin file.")
	f.Fix = derivePackageFix(file)
	ctx.Emit(f)
}

// derivePackageFix derives a package statement from the file path by looking for
// known source roots (src/main/kotlin, src/test/kotlin, etc.) and using the
// remaining directory path as the package name.
func derivePackageFix(file *scanner.File) *scanner.Fix {
	absPath, err := filepath.Abs(file.Path)
	if err != nil {
		return nil
	}
	dir := filepath.Dir(absPath)
	// Normalise to forward slashes for matching
	dirSlash := filepath.ToSlash(dir)

	// Known source roots in priority order
	roots := []string{
		"src/main/kotlin/",
		"src/test/kotlin/",
		"src/commonMain/kotlin/",
		"src/commonTest/kotlin/",
		"src/androidMain/kotlin/",
		"src/androidTest/kotlin/",
		"src/main/java/",
		"src/test/java/",
	}

	var pkg string
	for _, root := range roots {
		idx := strings.Index(dirSlash, root)
		if idx >= 0 {
			remainder := dirSlash[idx+len(root):]
			remainder = strings.TrimSuffix(remainder, "/")
			if remainder != "" {
				pkg = strings.ReplaceAll(remainder, "/", ".")
			}
			break
		}
	}
	if pkg == "" {
		return nil
	}
	// Insert package declaration at the very beginning of the file
	return &scanner.Fix{
		ByteMode:    true,
		StartByte:   0,
		EndByte:     0,
		Replacement: "package " + pkg + "\n\n",
	}
}

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
func (r *MissingSuperCallRule) Confidence() float64 { return 0.75 }

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
func (r *MissingUseCallRule) Confidence() float64 { return 0.75 }

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

func missingUseCloseableConstructorConfirmed(ctx *v2.Context, idx uint32) (string, bool) {
	if ctx == nil || ctx.File == nil || ctx.File.FlatType(idx) != "call_expression" {
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
