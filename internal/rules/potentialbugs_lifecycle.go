package rules

import (
	"path/filepath"
	"regexp"
	"strings"

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
func enclosingImplementsIteratorFlat(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		pt := file.FlatType(p)
		if pt != "class_declaration" && pt != "object_declaration" {
			continue
		}
		for i := 0; i < file.FlatChildCount(p); i++ {
			c := file.FlatChild(p, i)
			if c == 0 || file.FlatType(c) != "delegation_specifier" {
				continue
			}
			t := file.FlatNodeText(c)
			if strings.HasPrefix(t, "Iterator") || strings.HasPrefix(t, "Iterator<") ||
				strings.HasPrefix(t, "MutableIterator") || strings.HasPrefix(t, "ListIterator") ||
				strings.HasPrefix(t, "kotlin.collections.Iterator") ||
				strings.HasPrefix(t, "java.util.Iterator") {
				return true
			}
		}
		return false
	}
	return false
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

func (r *MissingPackageDeclarationRule) CheckLines(file *scanner.File) []scanner.Finding {
	if !strings.HasSuffix(file.Path, ".kt") {
		return nil
	}
	for _, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || scanner.IsCommentLine(line) {
			continue
		}
		if strings.HasPrefix(trimmed, "package ") {
			return nil
		}
		// First non-comment, non-blank line is not a package declaration
		f := r.Finding(file, 1, 1,
			"Missing package declaration in Kotlin file.")
		f.Fix = derivePackageFix(file)
		return []scanner.Finding{f}
	}
	f := r.Finding(file, 1, 1,
		"Missing package declaration in Kotlin file.")
	f.Fix = derivePackageFix(file)
	return []scanner.Finding{f}
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
// MissingSuperCallRule detects override without super call.
// MustInvokeSuperAnnotations lists the annotations that require a super call
// (e.g., "androidx.annotation.CallSuper"). Without cross-file type resolution
// the rule fires on all overrides that lack a super call; the annotation list
// is stored for future use when type oracle support is extended.
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

func missingUseCalleeIdentFlat(file *scanner.File, idx uint32) string {
	return flatCallExpressionName(file, idx)
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
		if ident := file.FlatFindChild(child, "simple_identifier"); ident != 0 && file.FlatNodeTextEquals(ident, "use") {
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
	for i := 0; i < file.FlatChildCount(scope); i++ {
		child := file.FlatChild(scope, i)
		if file.FlatStartByte(child) <= file.FlatEndByte(parent) {
			continue
		}
		childText := file.FlatNodeText(child)
		if strings.Contains(childText, varName+".use") {
			return true
		}
	}
	return false
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
