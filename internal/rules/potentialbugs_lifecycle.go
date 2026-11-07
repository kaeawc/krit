package rules

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/javafacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
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
		if name == "exit" && exitOutsideMainKotlinSystemReceiver(file, idx) {
			return true
		}
		return (name == "exit" || name == "halt") && exitOutsideMainKotlinRuntimeReceiver(file, idx)
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

// exitOutsideMainKotlinRuntimeReceiver matches `Runtime.getRuntime().exit(..)` and its qualified spelling.
func exitOutsideMainKotlinRuntimeReceiver(file *scanner.File, idx uint32) bool {
	nav, _ := flatCallExpressionParts(file, idx)
	receiver := flatNavigationReceiver(file, nav)
	if receiver == 0 {
		return false
	}
	text := strings.Join(strings.Fields(file.FlatNodeText(receiver)), "")
	return text == "Runtime.getRuntime()" || text == "java.lang.Runtime.getRuntime()"
}

func exitOutsideMainInsideMain(file *scanner.File, idx uint32) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "method_declaration":
			return extractIdentifierFlat(file, parent) == "main"
		case "function_declaration":
			return kotlinIsMainFunction(file, parent)
		}
	}
	return false
}

// kotlinIsMainFunction follows detekt: a public, non-override `main` whose parameters are none, `Array<String>` or
// `vararg String`, declared at the top level or as a `@JvmStatic` member of an object.
func kotlinIsMainFunction(file *scanner.File, fn uint32) bool {
	if strings.Trim(extractIdentifierFlat(file, fn), "`") != "main" {
		return false
	}
	for _, mod := range []string{"private", "protected", "internal", "override"} {
		if file.FlatHasModifier(fn, mod) {
			return false
		}
	}
	types, hasReceiver := functionParameterTypeTexts(file, fn)
	if hasReceiver || len(types) > 1 {
		return false
	}
	if len(types) == 1 {
		params, _ := file.FlatFindChild(fn, "function_value_parameters")
		mods, _ := file.FlatFindChild(params, "parameter_modifiers")
		vararg := mods != 0 && strings.Contains(file.FlatNodeText(mods), "vararg")
		if (types[0] != "Array<String>" || vararg) && (!vararg || types[0] != "String") {
			return false
		}
	}
	parent, ok := file.FlatParent(fn)
	if !ok {
		return false
	}
	if file.FlatType(parent) == "source_file" {
		return true
	}
	body, ok := file.FlatParent(fn)
	if !ok || file.FlatType(body) != "class_body" {
		return false
	}
	owner, ok := file.FlatParent(body)
	if !ok || (file.FlatType(owner) != "object_declaration" && file.FlatType(owner) != "companion_object") {
		return false
	}
	return hasAnnotationNamed(file, fn, "JvmStatic") || hasAnnotationNamed(file, fn, "kotlin.jvm.JvmStatic")
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
// directSuperNames returns the simple names of the direct supertypes, like detekt's getSuperNames.
func directSuperNames(file *scanner.File, decl uint32) []string {
	var names []string
	for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) != "delegation_specifier" {
			continue
		}
		var walk func(n uint32) bool
		walk = func(n uint32) bool {
			if file.FlatType(n) == "user_type" {
				last := ""
				for t := file.FlatFirstChild(n); t != 0; t = file.FlatNextSib(t) {
					if file.FlatType(t) == "type_identifier" {
						last = file.FlatNodeText(t)
					}
				}
				if last != "" {
					names = append(names, last)
				}
				return true
			}
			for t := file.FlatFirstChild(n); t != 0; t = file.FlatNextSib(t) {
				if walk(t) {
					return true
				}
			}
			return false
		}
		walk(c)
	}
	return names
}

// memberFunctionNamed returns the first function declared directly in the class body.
func memberFunctionNamed(file *scanner.File, decl uint32, name string) uint32 {
	for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
		if t := file.FlatType(c); t != "class_body" && t != "enum_class_body" {
			continue
		}
		for m := file.FlatFirstChild(c); m != 0; m = file.FlatNextSib(m) {
			if file.FlatType(m) == "function_declaration" && extractIdentifierFlat(file, m) == name {
				return m
			}
		}
	}
	return 0
}

// emitAtClassName reports on the class name, or the declaration start for an object literal.
func emitAtClassName(ctx *api.Context, decl uint32, msg string) {
	file := ctx.File
	at := decl
	for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
		if t := file.FlatType(c); t == "type_identifier" || t == "simple_identifier" {
			at = c
			break
		}
	}
	ctx.EmitAt(file.FlatRow(at)+1, file.FlatCol(at)+1, msg)
}

func hasSuperNamed(file *scanner.File, decl uint32, name string) bool {
	for _, n := range directSuperNames(file, decl) {
		if n == name {
			return true
		}
	}
	return false
}

// hasNextCallsNext matches detekt: any call whose callee text is `next` inside hasNext.
func hasNextCallsNext(file *scanner.File, fn uint32) bool {
	found := false
	file.FlatWalkNodes(fn, "call_expression", func(call uint32) {
		if flatCallExpressionName(file, call) == "next" {
			found = true
		}
	})
	return found
}

// throwsNoSuchElement matches detekt: a `throw NoSuchElementException(...)` anywhere in next.
func throwsNoSuchElement(file *scanner.File, fn uint32) bool {
	found := false
	file.FlatWalkNodes(fn, "jump_expression", func(jmp uint32) {
		first := file.FlatFirstChild(jmp)
		if first == 0 || file.FlatType(first) != "throw" {
			return
		}
		for c := file.FlatNextSib(first); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) != "call_expression" {
				continue
			}
			// Only a bare call counts; `java.util.NoSuchElementException()` is a qualified expression.
			if callee := file.FlatFirstChild(c); callee != 0 && file.FlatType(callee) == "simple_identifier" &&
				file.FlatNodeText(callee) == "NoSuchElementException" {
				found = true
			}
		}
	})
	return found
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

// intLiteralValue returns the value of a decimal, hex, long or unsigned integer literal node.
func intLiteralValue(file *scanner.File, idx uint32) (int64, bool) {
	switch file.FlatType(idx) {
	case "integer_literal", "hex_literal", "long_literal", "unsigned_literal":
	default:
		return 0, false
	}
	text := strings.ReplaceAll(file.FlatNodeText(idx), "_", "")
	text = strings.TrimRight(text, "LlUu")
	value, err := strconv.ParseInt(text, 0, 64)
	if err != nil || strings.HasPrefix(text, "0") && len(text) > 1 && text[1] != 'x' && text[1] != 'X' {
		return 0, false
	}
	return value, true
}

// leadingCommentStart returns the first of the comments that sit directly above a declaration, with no blank
// line between them. The Kotlin parser binds those comments into the declaration, so its text range, and the
// line detekt reports, start there. It returns idx when there is no such comment.
func leadingCommentStart(file *scanner.File, idx uint32) uint32 {
	start := idx
	for prev, ok := file.FlatPrevSibling(start); ok; prev, ok = file.FlatPrevSibling(prev) {
		if !isCommentNodeType(file.FlatType(prev)) {
			break
		}
		gap := file.Content[file.FlatEndByte(prev):file.FlatStartByte(start)]
		if strings.Count(string(gap), "\n") > 1 {
			break
		}
		start = prev
	}
	return start
}
