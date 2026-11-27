package rules

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of Android Lint 32.4.1's ReturnThisDetector (ReturnThis), SdkSuppressDetector (UseSdkSuppress) and
// SdkIntDetector (AnnotateVersionCheck).

// ReturnThisRule mirrors Lint's ReturnThis: a method annotated with (or overriding one annotated with) @ReturnThis that
// returns something other than `this`.
type ReturnThisRule struct {
	AndroidRule
}

// UseSdkSuppressRule mirrors Lint's UseSdkSuppress: @RequiresApi on a test class or test method.
type UseSdkSuppressRule struct {
	AndroidRule
}

// AnnotateVersionCheckRule mirrors Lint's AnnotateVersionCheck: a public SDK_INT check in an Android library without
// @ChecksSdkIntAtLeast.
type AnnotateVersionCheckRule struct {
	AndroidRule
}

// Confidence reports a high base confidence: the annotation and the returns are read from source.
func (r *ReturnThisRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the annotations are read from source.
func (r *UseSdkSuppressRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a medium-high base confidence: the SDK_INT shape and the library module are read from source.
func (r *AnnotateVersionCheckRule) Confidence() float64 { return api.ConfidenceMediumHigh }

// annotationNamed reports whether the declaration carries the annotation fqn, written by its simple or full name.
func annotationNamed(file *scanner.File, decl uint32, fqn string) (uint32, bool) {
	simple := fqn[strings.LastIndexByte(fqn, '.')+1:]
	mods, ok := file.FlatFindChild(decl, "modifiers")
	if !ok {
		return 0, false
	}
	var found uint32
	file.FlatWalkAllNodes(mods, func(n uint32) {
		if found != 0 {
			return
		}
		switch file.FlatType(n) {
		case "annotation", "marker_annotation":
			text := strings.TrimPrefix(strings.TrimSpace(file.FlatNodeText(n)), "@")
			if i := strings.IndexByte(text, '('); i >= 0 {
				text = text[:i]
			}
			text = strings.TrimSpace(text)
			if text == fqn || (text == simple && (lintTypeRefMatches(file, simple, fqn) || annotationDefaultImported(file, simple, fqn))) {
				found = n
			}
		}
	})
	return found, found != 0
}

// annotationDefaultImported reports a Kotlin annotation from a package Kotlin imports by default (kotlin.jvm,
// kotlin), named by its simple name with no import giving that name to another type.
func annotationDefaultImported(file *scanner.File, simple, fqn string) bool {
	defaultPkg := strings.HasPrefix(fqn, "kotlin.jvm.") || (strings.Count(fqn, ".") == 1 && strings.HasPrefix(fqn, "kotlin."))
	if file.Language == scanner.LangJava || !defaultPkg {
		return false
	}
	imported, ok := lintImportsOf(file).bySimple[simple]
	return !ok || imported == fqn
}

// declHasModifier reports a modifier keyword such as `override` or `private` on a declaration.
func declHasModifier(file *scanner.File, decl uint32, word string) bool {
	mods, ok := file.FlatFindChild(decl, "modifiers")
	if !ok {
		return false
	}
	found := false
	file.FlatWalkAllNodes(mods, func(n uint32) {
		if !found && file.FlatChildCount(n) == 0 && file.FlatNodeText(n) == word {
			found = true
		}
	})
	return found
}

// ---------------------------------------------------------------------------
// ReturnThis
// ---------------------------------------------------------------------------

const returnThisFQN = "androidx.annotation.ReturnThis"

var (
	returnThisBytes  = []byte("ReturnThis")
	requiresAPIBytes = []byte("RequiresApi")
)

func (r *ReturnThisRule) check(ctx *api.Context) {
	file := ctx.File
	fn := ctx.Idx
	if !bytes.Contains(file.Content, returnThisBytes) || !returnThisApplies(ctx, fn) {
		return
	}
	const msg = "This method should `return this` (because it has been annotated with `@ReturnThis`)"
	body := recyclerFunctionBody(file, fn)
	if body == 0 {
		return
	}
	// A Kotlin expression body is an implicit return of the expression.
	if file.Language != scanner.LangJava && file.FlatType(body) == "function_body" &&
		strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(body)), "=") {
		if expr := file.FlatNamedChild(body, 0); expr != 0 && file.FlatType(expr) != "this_expression" {
			// Lint reports the implicit return at the whole declaration, annotations included.
			ctx.Emit(lintFindingAtNode(ctx, fn, "", msg))
		}
		return
	}
	returnThisVisit(file, body, func(ret uint32) {
		if !returnThisReturnsThis(file, ret) {
			ctx.Emit(lintFindingAtNode(ctx, ret, "", msg))
		}
	})
}

// returnThisApplies reports a method annotated @ReturnThis, or an override of a same-file supertype method that is.
func returnThisApplies(ctx *api.Context, fn uint32) bool {
	file := ctx.File
	if _, ok := annotationNamed(file, fn, returnThisFQN); ok {
		return true
	}
	isOverride := declHasModifier(file, fn, "override")
	if file.Language == scanner.LangJava {
		_, isOverride = annotationNamed(file, fn, "java.lang.Override")
		// java.lang is imported implicitly: @Override is java.lang.Override unless an import says otherwise.
		if !isOverride {
			_, isOverride = annotationNamed(file, fn, "Override")
			if imported, ok := lintImportsOf(file).bySimple["Override"]; ok && imported != "java.lang.Override" {
				isOverride = false
			}
		}
	}
	if !isOverride {
		return false
	}
	name := recyclerName(file, fn)
	cls := recyclerEnclosingClass(file, fn)
	seen := map[uint32]bool{}
	for depth := 0; cls != 0 && depth < 8 && !seen[cls]; depth++ {
		seen[cls] = true
		var next uint32
		for _, ref := range lintDirectSupertypeRefs(file, cls) {
			decl := lintSameFileClass(file, ref.simple)
			if decl == 0 || decl == cls {
				continue
			}
			if body, ok := file.FlatFindChild(decl, "class_body"); ok {
				for m := file.FlatFirstChild(body); m != 0; m = file.FlatNextSib(m) {
					if t := file.FlatType(m); (t == "function_declaration" || t == "method_declaration") && recyclerName(file, m) == name {
						if _, ok := annotationNamed(file, m, returnThisFQN); ok {
							return true
						}
					}
				}
			}
			next = decl
		}
		cls = next
	}
	return false
}

// returnThisVisit calls visit for each return that targets the function: labeled returns, nested functions, Java
// lambdas and classes are skipped, as UAST's jump targets and Lint's visitClass skip them.
func returnThisVisit(file *scanner.File, body uint32, visit func(uint32)) {
	var walk func(n uint32)
	walk = func(n uint32) {
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "function_declaration", "anonymous_function", "method_declaration", "lambda_expression",
				"class_declaration", "object_literal", "object_declaration", "class_body":
				continue
			case "jump_expression":
				text := strings.TrimSpace(file.FlatNodeText(c))
				if strings.HasPrefix(text, "return") && !strings.HasPrefix(text, "return@") {
					visit(c)
				}
			case "return_statement":
				visit(c)
			}
			walk(c)
		}
	}
	walk(body)
}

func returnThisReturnsThis(file *scanner.File, ret uint32) bool {
	expr := file.FlatNamedChild(ret, 0)
	// Kotlin parses `this` as this_expression, Java as this.
	return expr != 0 && (file.FlatType(expr) == "this_expression" || file.FlatType(expr) == "this")
}

// ---------------------------------------------------------------------------
// UseSdkSuppress
// ---------------------------------------------------------------------------

var requiresAPIAnnotations = []string{"androidx.annotation.RequiresApi", "android.support.annotation.RequiresApi"}

func (r *UseSdkSuppressRule) check(ctx *api.Context) {
	file := ctx.File
	decl := ctx.Idx
	if !isProjectTestFile(ctx, file.Path) || !bytes.Contains(file.Content, requiresAPIBytes) {
		return
	}
	var ann uint32
	for _, fqn := range requiresAPIAnnotations {
		if a, ok := annotationNamed(file, decl, fqn); ok {
			ann = a
			break
		}
	}
	if ann == 0 || !sdkSuppressIsPublic(file, decl) {
		return
	}
	switch file.FlatType(decl) {
	case "function_declaration", "method_declaration":
		if !sdkSuppressIsTest(file, decl) && !sdkSuppressIsTestCaseMethod(file, decl) {
			return
		}
	default:
		if !sdkSuppressIsTestCase(file, decl, 0) && !sdkSuppressHasTestMethod(file, decl) {
			return
		}
	}
	ctx.Emit(lintFindingAtNode(ctx, ann, "", "Don't use @RequiresApi from tests; use @SdkSuppress on `"+
		sdkSuppressName(file, decl)+"` instead"))
}

func sdkSuppressName(file *scanner.File, decl uint32) string {
	if n := recyclerName(file, decl); n != "" {
		return n
	}
	for _, t := range []string{"type_identifier", "identifier"} {
		if n, ok := file.FlatFindChild(decl, t); ok {
			return file.FlatNodeText(n)
		}
	}
	return ""
}

// sdkSuppressIsPublic reports Kotlin declarations without a narrowing visibility and Java ones marked public.
func sdkSuppressIsPublic(file *scanner.File, decl uint32) bool {
	if file.Language == scanner.LangJava {
		return declHasModifier(file, decl, "public")
	}
	return !declHasModifier(file, decl, "private") && !declHasModifier(file, decl, "protected") &&
		!declHasModifier(file, decl, "internal")
}

func sdkSuppressIsTest(file *scanner.File, fn uint32) bool {
	_, ok := annotationNamed(file, fn, "org.junit.Test")
	return ok
}

func sdkSuppressIsTestCaseMethod(file *scanner.File, fn uint32) bool {
	if !strings.HasPrefix(recyclerName(file, fn), "test") {
		return false
	}
	cls := recyclerEnclosingClass(file, fn)
	return cls != 0 && sdkSuppressIsTestCase(file, cls, 0)
}

// sdkSuppressIsTestCase reports a class extending junit.framework.TestCase, directly or through same-file classes.
func sdkSuppressIsTestCase(file *scanner.File, cls uint32, depth int) bool {
	if depth > 6 {
		return false
	}
	for _, ref := range lintDirectSupertypeRefs(file, cls) {
		if lintTypeRefMatches(file, ref.name, "junit.framework.TestCase") {
			return true
		}
		if decl := lintSameFileClass(file, ref.simple); decl != 0 && decl != cls && sdkSuppressIsTestCase(file, decl, depth+1) {
			return true
		}
	}
	return false
}

func sdkSuppressHasTestMethod(file *scanner.File, cls uint32) bool {
	body, ok := file.FlatFindChild(cls, "class_body")
	if !ok {
		return false
	}
	for m := file.FlatFirstChild(body); m != 0; m = file.FlatNextSib(m) {
		if t := file.FlatType(m); (t == "function_declaration" || t == "method_declaration") && sdkSuppressIsTest(file, m) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// AnnotateVersionCheck
// ---------------------------------------------------------------------------

const checksSdkIntAtLeastFQN = "androidx.annotation.ChecksSdkIntAtLeast"

func (r *AnnotateVersionCheckRule) check(ctx *api.Context) {
	file := ctx.File
	cmp := ctx.Idx
	op, left, right, ok := versionCheckComparison(file, cmp)
	if !ok || (op != ">" && op != ">=") || !versionCheckIsSdkInt(file, left) || !versionCheckInLibrary(file) {
		return
	}
	parent := versionCheckParent(file, cmp)
	if parent == 0 {
		return
	}
	// SDK_INT > 36 && SDK_INT_FULL >= ... counts as one check; any other && does not.
	if t := file.FlatType(parent); t == "conjunction_expression" || (t == "binary_expression" && strings.Contains(file.FlatNodeText(parent), "&&")) {
		l, rr := file.FlatNamedChild(parent, 0), file.FlatNamedChild(parent, file.FlatNamedChildCount(parent)-1)
		switch {
		case l == cmp && strings.Contains(file.FlatNodeText(rr), "SDK_INT_FULL"):
			return
		case rr == cmp:
			if parent = versionCheckParent(file, parent); parent == 0 {
				return
			}
		default:
			return
		}
	}
	atLeast, code, param := versionCheckAPI(file, right, op == ">=")
	switch file.FlatType(parent) {
	case "property_declaration", "variable_declarator":
		decl := parent
		if file.FlatType(parent) == "variable_declarator" {
			decl, _ = file.FlatParent(parent)
		}
		if atLeast > 0 && versionCheckFieldIsPublic(file, decl) {
			r.report(ctx, decl, atLeast, "field", "ChecksSdkIntAtLeast(api="+code+")")
		}
	case "jump_expression", "return_statement", "function_body":
		fn := versionCheckSingleStatementMethod(file, parent)
		if fn != 0 {
			r.reportMethod(ctx, fn, atLeast, code, param, -1)
		}
	case "if_expression", "if_statement":
		param, fn := versionCheckLambdaParam(file, parent)
		if fn != 0 && param >= 0 {
			r.reportMethod(ctx, fn, atLeast, code, -1, param)
		}
	}
}

func (r *AnnotateVersionCheckRule) reportMethod(ctx *api.Context, fn uint32, atLeast int, code string, param, lambda int) {
	file := ctx.File
	if !sdkSuppressIsPublic(file, fn) {
		return
	}
	suffix := ""
	if lambda >= 0 {
		suffix = ", lambda=" + strconv.Itoa(lambda)
	}
	switch {
	case atLeast > 0:
		r.report(ctx, fn, atLeast, "method", "@ChecksSdkIntAtLeast(api="+code+suffix+")")
	case param >= 0:
		if _, ok := annotationNamed(file, fn, checksSdkIntAtLeastFQN); !ok {
			ctx.Emit(lintFindingAtNode(ctx, versionCheckNameNode(file, fn), "",
				"This method should be annotated with `@ChecksSdkIntAtLeast(parameter="+strconv.Itoa(param)+suffix+")`"))
		}
	}
}

// report emits the missing-annotation finding, or the discrepancy finding when the annotation names another level.
func (r *AnnotateVersionCheckRule) report(ctx *api.Context, decl uint32, atLeast int, kind, args string) {
	file := ctx.File
	if ann, ok := annotationNamed(file, decl, checksSdkIntAtLeastFQN); ok {
		if declared, ok := versionCheckAnnotationAPI(file, ann); ok && declared != atLeast {
			ctx.Emit(lintFindingAtNode(ctx, ann, "", fmt.Sprintf("API level discrepancy: annotation says %d and code checks %d", declared, atLeast)))
		}
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, versionCheckNameNode(file, decl), "", "This "+kind+" should be annotated with `"+args+"`"))
}

// versionCheckComparison splits a Kotlin comparison or a Java relational binary expression.
func versionCheckComparison(file *scanner.File, n uint32) (op string, left, right uint32, ok bool) {
	switch file.FlatType(n) {
	case "comparison_expression", "binary_expression":
	default:
		return "", 0, 0, false
	}
	if file.FlatNamedChildCount(n) != 2 {
		return "", 0, 0, false
	}
	left, right = file.FlatNamedChild(n, 0), file.FlatNamedChild(n, 1)
	op = strings.TrimSpace(string(file.Content[file.FlatEndByte(left):file.FlatStartByte(right)]))
	return op, trimSkipParens(file, left), trimSkipParens(file, right), true
}

// versionCheckIsSdkInt reports android.os.Build.VERSION.SDK_INT written fully, as Build.VERSION.SDK_INT, VERSION.SDK_INT
// or an imported SDK_INT.
func versionCheckIsSdkInt(file *scanner.File, n uint32) bool {
	text := strings.Join(strings.Fields(file.FlatNodeText(n)), "")
	switch text {
	case "android.os.Build.VERSION.SDK_INT":
		return true
	case "Build.VERSION.SDK_INT":
		return lintTypeRefMatches(file, "Build", "android.os.Build")
	case "VERSION.SDK_INT":
		return lintTypeRefMatches(file, "VERSION", "android.os.Build.VERSION")
	case "SDK_INT":
		return lintImportsOf(file).bySimple["SDK_INT"] == "android.os.Build.VERSION.SDK_INT"
	}
	return false
}

// versionCheckInLibrary reports a file in an Android library module, where Lint asks for the annotation; an app
// module has the method bodies in source anyway. The module's build script must apply the library plugin.
func versionCheckInLibrary(file *scanner.File) bool {
	root := workModuleRoot(file.Path, map[string]string{})
	if root == "" {
		return false
	}
	return filefacts.StringFact(fileFactsCache(), root, slotVersionCheckLibrary, func() bool {
		return readModuleIsLibrary(root)
	})
}

// slotVersionCheckLibrary memoizes versionCheckInLibrary per module directory for one run.
const slotVersionCheckLibrary = "lintVersionCheckLibrary"

func readModuleIsLibrary(root string) bool {
	for _, name := range []string{"build.gradle.kts", "build.gradle"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue
		}
		// The cached Gradle facts resolve version-catalog plugin aliases; the application check reads the plugins
		// alone, without comments.
		content := gradleWithoutComments(string(data))
		info, _ := tryReadGradleInfo(root)
		return info.isLibrary && !moduleAppliesApplicationPlugin(root, content)
	}
	return false
}

// versionCheckParent returns the expression's parent, skipping parentheses.
func versionCheckParent(file *scanner.File, n uint32) uint32 {
	for {
		p, ok := file.FlatParent(n)
		if !ok {
			return 0
		}
		if file.FlatType(p) != "parenthesized_expression" {
			return p
		}
		n = p
	}
}

// versionCheckAPI evaluates the level operand: the checked level (one more for `>`), the code Lint prints for it
// (the operand's source for `>=` with a constant reference, else the VERSION_CODES field), and the parameter index
// when the operand is a parameter of the enclosing function.
func versionCheckAPI(file *scanner.File, operand uint32, orEqual bool) (atLeast int, code string, param int) {
	param = -1
	text := strings.TrimSpace(file.FlatNodeText(operand))
	v, ok := lintEvalInt(file, operand, sdkVersionCodes)
	if i := strings.LastIndex(text, "VERSION_CODES."); !ok && i >= 0 {
		if lvl, known := versionCodeLevels[text[i+len("VERSION_CODES."):]]; known {
			v, ok = int64(lvl), true
		}
	}
	if !ok {
		if lintIdentOnly(text) {
			if fn, _ := flatEnclosingCallable(file, operand); fn != 0 {
				for i, p := range recyclerParams(file, fn) {
					if recyclerName(file, p) == text {
						param = i
					}
				}
			}
		}
		return 0, "", param
	}
	atLeast = int(v)
	if !orEqual {
		atLeast++
	}
	if orEqual && file.FlatType(operand) != "integer_literal" && file.FlatType(operand) != "decimal_integer_literal" {
		return atLeast, text, -1
	}
	return atLeast, versionCodeField(atLeast), -1
}

// versionCodeField returns android.os.Build.VERSION_CODES.<NAME> for a level, or the number when it has no name.
func versionCodeField(level int) string {
	for name, l := range versionCodeLevels {
		if l == level {
			return "android.os.Build.VERSION_CODES." + name
		}
	}
	return strconv.Itoa(level)
}

func versionCheckAnnotationAPI(file *scanner.File, ann uint32) (int, bool) {
	text := file.FlatNodeText(ann)
	i := strings.Index(text, "api")
	if i < 0 {
		return 0, false
	}
	rest := strings.TrimSpace(text[i+3:])
	rest = strings.TrimSpace(strings.TrimPrefix(rest, "="))
	end := strings.IndexAny(rest, ",)")
	if end < 0 {
		return 0, false
	}
	value := strings.TrimSpace(rest[:end])
	if n, err := strconv.Atoi(value); err == nil {
		return n, true
	}
	if j := strings.LastIndex(value, "VERSION_CODES."); j >= 0 {
		if lvl, ok := versionCodeLevels[value[j+len("VERSION_CODES."):]]; ok {
			return lvl, true
		}
	}
	return 0, false
}

// versionCheckFieldIsPublic reports a public Java field, or a Kotlin property compiled to a public field
// (@JvmField or const) outside a function body.
func versionCheckFieldIsPublic(file *scanner.File, decl uint32) bool {
	switch file.FlatType(decl) {
	case "field_declaration":
		return declHasModifier(file, decl, "public")
	case "property_declaration":
		if p, ok := file.FlatParent(decl); !ok || (file.FlatType(p) != "class_body" && file.FlatType(p) != "source_file") {
			return false
		}
		_, jvmField := annotationNamed(file, decl, "kotlin.jvm.JvmField")
		return sdkSuppressIsPublic(file, decl) && (jvmField || declHasModifier(file, decl, "const"))
	}
	return false
}

// versionCheckSingleStatementMethod returns the function whose body is only the return (or expression body) holding
// the comparison.
func versionCheckSingleStatementMethod(file *scanner.File, parent uint32) uint32 {
	if file.FlatType(parent) == "function_body" {
		if fn, ok := file.FlatParent(parent); ok && file.FlatType(fn) == "function_declaration" {
			return versionCheckNonLocal(file, fn)
		}
		return 0
	}
	if t := file.FlatType(parent); t == "jump_expression" && !strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(parent)), "return") {
		return 0
	}
	block, ok := file.FlatParent(parent)
	if ok && file.FlatType(block) == "statements" {
		block, ok = file.FlatParent(block)
	}
	if !ok || versionCheckStatementCount(file, versionCheckStatements(file, block)) != 1 {
		return 0
	}
	fn, ok := file.FlatParent(block)
	if ok && file.FlatType(fn) == "function_body" {
		fn, ok = file.FlatParent(fn)
	}
	if !ok || (file.FlatType(fn) != "function_declaration" && file.FlatType(fn) != "method_declaration") {
		return 0
	}
	return versionCheckNonLocal(file, fn)
}

// versionCheckNonLocal returns fn unless it is a local function, which is not API Lint asks to annotate.
func versionCheckNonLocal(file *scanner.File, fn uint32) uint32 {
	if p, ok := file.FlatParent(fn); ok && file.FlatType(p) == "statements" {
		return 0
	}
	return fn
}

// versionCheckStatementCount counts the statements of a block, leaving out comments.
func versionCheckStatementCount(file *scanner.File, block uint32) int {
	n := 0
	for c := file.FlatFirstChild(block); c != 0; c = file.FlatNextSib(c) {
		if t := file.FlatType(c); file.FlatIsNamed(c) && t != "line_comment" && t != "block_comment" && t != "multiline_comment" {
			n++
		}
	}
	return n
}

func versionCheckStatements(file *scanner.File, block uint32) uint32 {
	if s, ok := file.FlatFindChild(block, "statements"); ok {
		return s
	}
	return block
}

// versionCheckLambdaParam handles `if (SDK_INT >= N) { param.call() }` as the body of a function: it returns the index
// of the lambda-typed parameter called first in the branch and the function.
func versionCheckLambdaParam(file *scanner.File, ifNode uint32) (int, uint32) {
	stmts := versionCheckIfBody(file, ifNode)
	if stmts == 0 {
		return -1, 0
	}
	first := trimSkipParens(file, file.FlatNamedChild(stmts, 0))
	recv := ""
	switch file.FlatType(first) {
	case "call_expression", "method_invocation":
		if call, ok := lintCallAt(file, first); ok && call.ReceiverNode != 0 {
			recv = strings.TrimSpace(call.ReceiverText)
		} else if ok && file.Language == scanner.LangKotlin {
			// `block()` invokes a function-typed parameter directly.
			recv = call.Name
		}
	}
	if !lintIdentOnly(recv) {
		return -1, 0
	}
	fn := versionCheckSingleStatementMethod(file, ifNode)
	if fn == 0 {
		// The if may also be followed by a return.
		fn = versionCheckIfThenReturnMethod(file, ifNode)
	}
	if fn == 0 {
		return -1, 0
	}
	for i, p := range recyclerParams(file, fn) {
		if recyclerName(file, p) == recv && versionCheckIsLambdaType(lintChildTypeText(file, p), file.FlatNodeText(p)) {
			return i, fn
		}
	}
	return -1, 0
}

func versionCheckIfBody(file *scanner.File, ifNode uint32) uint32 {
	for c := file.FlatFirstChild(ifNode); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "control_structure_body":
			if b, ok := file.FlatFindChild(c, "statements"); ok {
				return b
			}
			return c
		case "block":
			return c
		case "expression_statement":
			return ifNode
		}
	}
	return 0
}

func versionCheckIfThenReturnMethod(file *scanner.File, ifNode uint32) uint32 {
	block, ok := file.FlatParent(ifNode)
	if !ok {
		return 0
	}
	stmts := versionCheckStatements(file, block)
	if file.FlatNamedChildCount(stmts) != 2 || file.FlatNamedChild(stmts, 0) != ifNode {
		return 0
	}
	second := file.FlatNamedChild(stmts, 1)
	isReturn := file.FlatType(second) == "return_statement" ||
		(file.FlatType(second) == "jump_expression" && strings.HasPrefix(file.FlatNodeText(second), "return"))
	if !isReturn {
		return 0
	}
	if b := block; file.FlatType(b) == "statements" {
		block, _ = file.FlatParent(b)
	}
	fn, ok := file.FlatParent(block)
	if ok && file.FlatType(fn) == "function_body" {
		fn, ok = file.FlatParent(fn)
	}
	if !ok {
		return 0
	}
	return fn
}

// versionCheckIsLambdaType mirrors isLambdaType for the types a parameter declares: Runnable, Kotlin function types and
// the java.util.function interfaces.
func versionCheckIsLambdaType(typ, paramText string) bool {
	if strings.Contains(paramText, "->") {
		return true
	}
	switch typ {
	case "Runnable", "java.lang.Runnable", "Function", "java.util.function.Function", "Supplier", "Consumer", "Callable":
		return true
	}
	return strings.HasPrefix(typ, "Function") || strings.HasPrefix(typ, "kotlin.jvm.functions.Function")
}

func versionCheckNameNode(file *scanner.File, decl uint32) uint32 {
	switch file.FlatType(decl) {
	case "property_declaration":
		if vd, ok := file.FlatFindChild(decl, "variable_declaration"); ok {
			return recyclerParamNameNode(file, vd)
		}
	case "field_declaration":
		if vd, ok := file.FlatFindChild(decl, "variable_declarator"); ok {
			return recyclerParamNameNode(file, vd)
		}
	}
	return recyclerParamNameNode(file, decl)
}
