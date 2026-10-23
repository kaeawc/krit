package rules

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/javafacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
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
func (r *ExitOutsideMainRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *ExplicitGarbageCollectionCallRule) Confidence() float64 { return api.ConfidenceMedium }

func javaSourceResolvesSimpleType(file *scanner.File, simpleName, fqn string) bool {
	facts := javafacts.SourceFactsForFile(file)
	if facts == nil {
		return false
	}
	return facts.ResolveType(simpleName, nil) == fqn
}

func exitOutsideMainCallMatches(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	switch file.FlatType(idx) {
	case "call_expression":
		name := flatCallExpressionName(file, idx)
		if name == "exitProcess" {
			return true
		}
		return name == "exit" && exitOutsideMainKotlinSystemReceiver(file, idx)
	case "method_invocation":
		if javaMethodInvocationName(file, idx) != "exit" || javaMethodReceiverText(file, idx) != "System" {
			return false
		}
		return javaSourceResolvesSimpleType(file, "System", "java.lang.System")
	default:
		return false
	}
}

func exitOutsideMainKotlinSystemReceiver(file *scanner.File, idx uint32) bool {
	nav, _ := flatCallExpressionParts(file, idx)
	receiver := flatNavigationReceiver(file, nav)
	path := flatNullOrEmptyReferencePath(file, receiver)
	if len(path) == 1 && path[0] == "System" {
		return true
	}
	return len(path) == 3 && path[0] == "java" && path[1] == "lang" && path[2] == "System"
}

func exitOutsideMainInsideMain(file *scanner.File, idx uint32) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) != "function_declaration" && file.FlatType(parent) != "method_declaration" {
			continue
		}
		return extractIdentifierFlat(file, parent) == "main"
	}
	return false
}

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
func (r *InvalidRangeRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *IteratorHasNextCallsNextMethodRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *IteratorNotThrowingNoSuchElementExceptionRule) Confidence() float64 {
	return api.ConfidenceMedium
}

// enclosingImplementsIterator returns true if the node's enclosing class
// has a delegation specifier naming Iterator / MutableIterator / ListIterator
// (with or without a qualifier).
func enclosingImplementsIteratorFlat(ctx *api.Context, idx uint32) bool {
	if ctx.File == nil {
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

func iteratorSupertypeConfirmed(ctx *api.Context, idx uint32) bool {
	if ctx.File == nil || idx == 0 {
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

func functionThrowsNoSuchElementExceptionFlat(ctx *api.Context, body uint32) bool {
	if ctx.File == nil || body == 0 {
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
func (r *LateinitUsageRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// MissingPackageDeclarationRule detects Kotlin/Java source files without package statements.
// ---------------------------------------------------------------------------
type MissingPackageDeclarationRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence bumps this rule from the line-rule default to 0.95 — the
// check asks tree-sitter whether the file's root has a package_header
// (Kotlin) or package_declaration (Java) child. Deterministic with no
// heuristic path and no dependence on line-level comment detection.
func (r *MissingPackageDeclarationRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func (r *MissingPackageDeclarationRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	var pkgType string
	switch {
	case strings.HasSuffix(file.Path, ".java"):
		pkgType = "package_declaration"
	case strings.HasSuffix(file.Path, ".kt"):
		pkgType = "package_header"
	default:
		return
	}
	if _, ok := file.FlatFindChild(idx, pkgType); ok {
		return
	}
	f := r.Finding(file, 1, 1,
		"Missing package declaration in source file.")
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
