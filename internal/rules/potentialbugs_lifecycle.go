package rules

import (
	"fmt"
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

func (r *ExitOutsideMainRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *ExitOutsideMainRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if !strings.HasPrefix(text, "exitProcess(") && !strings.HasPrefix(text, "System.exit(") {
		return nil
	}
	// Walk ancestors to see if any function_declaration is named "main"
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) == "function_declaration" {
			name := extractIdentifierFlat(file, parent)
			if name == "main" {
				return nil // inside main — allowed
			}
		}
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Do not call exitProcess() or System.exit() outside of the main function.")}
}

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

func (r *ExplicitGarbageCollectionCallRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *ExplicitGarbageCollectionCallRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if text != "System.gc()" && text != "Runtime.getRuntime().gc()" {
		return nil
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Do not call garbage collector explicitly. It is rarely necessary and can degrade performance.")
	// Remove the call statement using AST node byte offsets
	startByte := int(file.FlatStartByte(idx))
	endByte := int(file.FlatEndByte(idx))
	// Expand to cover the full line (leading whitespace + trailing newline)
	for startByte > 0 && file.Content[startByte-1] != '\n' {
		startByte--
	}
	if endByte < len(file.Content) && file.Content[endByte] == '\n' {
		endByte++
	}
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   startByte,
		EndByte:     endByte,
		Replacement: "",
	}
	return []scanner.Finding{f}
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
func (r *InvalidRangeRule) Confidence() float64 { return 0.75 }

func (r *InvalidRangeRule) NodeTypes() []string { return []string{"range_expression"} }

func (r *InvalidRangeRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// range_expression children: left, "..", right
	if file.FlatChildCount(idx) < 3 {
		return nil
	}
	left := file.FlatChild(idx, 0)
	right := file.FlatChild(idx, file.FlatChildCount(idx)-1)
	if left == 0 || right == 0 {
		return nil
	}
	if file.FlatType(left) != "integer_literal" || file.FlatType(right) != "integer_literal" {
		return nil
	}
	startText := file.FlatNodeText(left)
	endText := file.FlatNodeText(right)
	// Skip lines that already use downTo
	lineIdx := file.FlatRow(idx)
	if lineIdx < len(file.Lines) && strings.Contains(file.Lines[lineIdx], "downTo") {
		return nil
	}
	// Numeric comparison via string length then lexicographic order
	if len(startText) > len(endText) || (len(startText) == len(endText) && startText > endText) {
		f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			fmt.Sprintf("Invalid range: %s..%s. The range is empty. Use 'downTo' for descending ranges.", startText, endText))
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(idx)),
			EndByte:     int(file.FlatEndByte(idx)),
			Replacement: startText + " downTo " + endText,
		}
		return []scanner.Finding{f}
	}
	return nil
}

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

func (r *IteratorHasNextCallsNextMethodRule) NodeTypes() []string {
	return []string{"function_declaration"}
}

func (r *IteratorHasNextCallsNextMethodRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := extractIdentifierFlat(file, idx)
	if name != "hasNext" {
		return nil
	}
	body := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return nil
	}
	// Walk body for call_expression containing "next"
	found := false
	file.FlatWalkNodes(body, "call_expression", func(callIdx uint32) {
		if found {
			return
		}
		if flatCallExpressionName(file, callIdx) == "next" {
			found = true
		}
	})
	if found {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			"hasNext() should not call next(). This modifies the iterator state.")}
	}
	return nil
}

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

func (r *IteratorNotThrowingNoSuchElementExceptionRule) NodeTypes() []string {
	return []string{"function_declaration"}
}

func (r *IteratorNotThrowingNoSuchElementExceptionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := extractIdentifierFlat(file, idx)
	if name != "next" {
		return nil
	}
	// Only flag when the enclosing class actually implements
	// kotlin.collections.Iterator or java.util.Iterator — custom iterator
	// shapes (cursor readers, archive exporters) have a different
	// contract and should not be forced to throw NoSuchElementException.
	if !enclosingImplementsIteratorFlat(file, idx) {
		return nil
	}
	body := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return nil
	}
	// Walk body for any throw with NoSuchElementException
	found := false
	file.FlatWalkNodes(body, "jump_expression", func(jmp uint32) {
		if found {
			return
		}
		jmpText := file.FlatNodeText(jmp)
		if strings.HasPrefix(jmpText, "throw") && strings.Contains(jmpText, "NoSuchElementException") {
			found = true
		}
	})
	if !found {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			"Iterator's next() method should throw NoSuchElementException when there are no more elements.")}
	}
	return nil
}

func (r *IteratorNotThrowingNoSuchElementExceptionRule) Check(file *scanner.File) []scanner.Finding {
	return nil
}

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

func (r *LateinitUsageRule) NodeTypes() []string { return []string{"property_declaration"} }

func (r *LateinitUsageRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Check if the property has a "lateinit" modifier
	if file.FlatHasModifier(idx, "lateinit") {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			"'lateinit' usage detected. Consider using lazy initialization or nullable types instead.")}
	}
	return nil
}

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

func (r *MissingSuperCallRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *MissingSuperCallRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Must have override modifier
	if !file.FlatHasModifier(idx, "override") {
		return nil
	}
	name := extractIdentifierFlat(file, idx)
	if name == "" {
		return nil
	}
	body := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return nil // abstract or expression body without braces
	}
	// Walk body descendants for call_expression containing super.name(
	superFound := false
	file.FlatWalkNodes(body, "call_expression", func(callNode uint32) {
		if superFound {
			return
		}
		callText := file.FlatNodeText(callNode)
		if strings.Contains(callText, "super."+name+"(") || strings.Contains(callText, "super<") {
			superFound = true
		}
	})
	// Also check for super<Type>.name pattern outside call_expression
	if !superFound {
		bodyText := file.FlatNodeText(body)
		if strings.Contains(bodyText, "super<") {
			superFound = true
		}
	}
	if superFound {
		return nil
	}
	// Check if the enclosing class has a class supertype (not just interfaces).
	// Walk up to find the class_declaration ancestor.
	classNode, ok := flatEnclosingAncestor(file, idx, "class_declaration")
	if ok {
		// Look for constructor_invocation among class children (delegation_specifier)
		hasClassSupertype := false
		file.FlatWalkNodes(classNode, "constructor_invocation", func(uint32) {
			hasClassSupertype = true
		})
		if !hasClassSupertype {
			return nil // only interfaces or no supertypes, no super to call
		}
	}
	f := r.Finding(file, file.FlatRow(idx)+1, 1,
		fmt.Sprintf("Override function '%s' does not call super.%s().", name, name))
	// Fix: insert "super.name()" as first statement after the opening brace
	// Find the opening brace of function_body
	bodyText := file.FlatNodeText(body)
	if strings.HasPrefix(strings.TrimSpace(bodyText), "{") {
		bracePos := int(file.FlatStartByte(body)) + strings.Index(bodyText, "{") + 1
		// Determine indentation
		funcLine := file.Lines[file.FlatRow(idx)]
		indent := ""
		for _, ch := range funcLine {
			if ch == ' ' || ch == '\t' {
				indent += string(ch)
			} else {
				break
			}
		}
		insertion := "\n" + indent + "    super." + name + "()"
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   bracePos,
			EndByte:     bracePos,
			Replacement: insertion,
		}
	}
	return []scanner.Finding{f}
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

func (r *MissingUseCallRule) NodeTypes() []string { return []string{"call_expression"} }

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

func (r *MissingUseCallRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Extract the callee name from the call_expression.
	callee := missingUseCalleeIdentFlat(file, idx)
	if callee == "" || !closeableTypes[callee] {
		return nil
	}

	// Check if this call is already wrapped in a .use {} chain.
	// Pattern: navigation_expression(call_expression, navigation_suffix("use"))
	//   -> outer call_expression with lambda_literal in call_suffix
	if missingUseHasUseChainFlat(file, idx) {
		return nil
	}

	// If assigned to a val/var, check if the variable is used with .use {} in the same scope.
	if missingUseAssignedWithUseFlat(file, idx) {
		return nil
	}

	// If this is a class-level property, skip it (fields may be closed elsewhere).
	if missingUseIsClassPropertyFlat(file, idx) {
		return nil
	}

	// If passed as an argument to another call, skip (the caller may manage the resource).
	if missingUseIsArgumentFlat(file, idx) {
		return nil
	}

	// If this call is the return expression of a function, skip (caller manages it).
	if missingUseIsReturnExpressionFlat(file, idx) {
		return nil
	}

	return []scanner.Finding{r.Finding(file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		fmt.Sprintf("%s opened without .use {}. This may lead to resource leaks.", callee))}
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
