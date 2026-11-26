package rules

// Ports of slack-lints' MockDetector (slackhq/slack-lints 0.11.1,
// slack-lint-checks/src/main/java/slack/lint/mocking/MockDetector.kt).
//
// Lint runs MockDetector as one detector over test sources: it finds mock
// sites (calls to the Mockito and mockito-kotlin factories, and @Mock/@Spy
// fields), resolves the mocked class, and asks an ordered list of type
// checkers about it; the first checker that objects reports and the rest stay
// silent. mockAnalysisFor runs that once per project over the test files and
// every registered DoNotMock* rule emits the findings stamped with its own ID.
//
// The mocked type is what the source proves: a type argument (`mock<T>()`), a
// class literal (`mock(T::class.java)`, `mock(T.class)`), the declared type of
// a @Mock field or of the variable a no-argument `mock()` initializes, or the
// type of a local that one of those initializes. A `mock()` whose type Lint
// infers from an enclosing call's parameter abstains. The class is resolved
// through imports, the file's package and the cross-file index; library classes
// outside the index are known only by name, which is enough for the platform
// check.

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	mockDoNotMockAnything      = "DoNotMockAnything"
	mockDoNotMockPlatformTypes = "DoNotMockPlatformTypes"
	mockDoNotMockDataClass     = "DoNotMockDataClass"
	mockDoNotMockObjectClass   = "DoNotMockObjectClass"
	mockDoNotMockSealedClass   = "DoNotMockSealedClass"
	mockDoNotMockAutoValue     = "DoNotMockAutoValue"
	mockDoNotMockRecordClass   = "DoNotMockRecordClass"

	slotMockUsage = "mockUsage"
)

// mockFactories are MockDetector's default mock-factories, keyed by the
// callable's FQN: a static method's class plus name, or a top-level Kotlin
// function's package plus name.
var mockFactories = map[string]bool{
	"org.mockito.Mockito.mock":               true,
	"org.mockito.Mockito.spy":                true,
	"slack.test.mockito.MockitoHelpers.mock": true,
	"slack.test.mockito.mock":                true,
	"org.mockito.kotlin.mock":                true,
	"org.mockito.kotlin.spy":                 true,
}

// mockFactoryOwners are the classes and packages the factories live in.
var mockFactoryOwners = []string{"org.mockito.Mockito", "slack.test.mockito.MockitoHelpers", "slack.test.mockito", "org.mockito.kotlin"}

// mockAnnotations are MockDetector's default mock-annotations.
var mockAnnotations = []string{"org.mockito.Mock", "org.mockito.Spy"}

// mockPlatformRoots are the package roots PlatformTypeMockDetector treats as platform types.
var mockPlatformRoots = map[string]bool{"android": true, "androidx": true, "java": true, "javax": true, "kotlin": true, "kotlinx": true, "scala": true}

// mockDoNotMockAnnotations make a type DoNotMockMockDetector's (not ported), which runs before the sealed,
// AutoValue, object and record checkers and so hides them.
var mockDoNotMockAnnotations = []string{"slack.lint.annotations.DoNotMock", "com.google.errorprone.annotations.DoNotMock"}

// mockAutoValueAnnotations are AutoValueMockDetector's non-mockable annotations, in its order.
var mockAutoValueAnnotations = []string{"com.google.auto.value.AutoValue", "com.google.auto.value.AutoValue.Builder"}

type mockFinding struct {
	rule string
	file *scanner.File
	node uint32
	msg  string
}

type mockResult struct {
	index  *scanner.CodeIndex
	byRule map[string][]mockFinding
}

// mockAnalysisFor returns the run's DoNotMock* findings, computing them once for all the rules.
func mockAnalysisFor(ctx *api.Context) *mockResult {
	index := ctx.CodeIndex
	if index == nil || len(index.Files) == 0 {
		return &mockResult{}
	}
	return filefacts.StringFact(fileFactsCache(), fmt.Sprintf("%p", index), slotMockUsage, func() *mockResult {
		return analyzeMockProject(index)
	})
}

// emitMockFindings reports the shared pass's findings for the rule ctx runs.
func emitMockFindings(ctx *api.Context) {
	if ctx.Rule == nil {
		return
	}
	for _, f := range mockAnalysisFor(ctx).byRule[ctx.Rule.ID] {
		ctx.Emit(scanner.Finding{
			File:      f.file.Path,
			Line:      f.file.FlatRow(f.node) + 1,
			Col:       f.file.FlatCol(f.node) + 1,
			StartByte: int(f.file.FlatStartByte(f.node)),
			EndByte:   int(f.file.FlatEndByte(f.node)),
			Message:   f.msg,
		})
	}
}

type mockAnalyzer struct {
	result *mockResult
	byPath map[string]*scanner.File
}

func analyzeMockProject(index *scanner.CodeIndex) *mockResult {
	a := &mockAnalyzer{result: &mockResult{index: index, byRule: map[string][]mockFinding{}}}
	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil || !scanner.IsTestFile(file.Path) || !mockFileMayMatch(file.Content) {
			continue
		}
		a.analyzeFile(file)
	}
	return a.result
}

// mockFileMayMatch is a cheap prefilter: every mock site names a factory or an annotation, and resolving
// either needs its package in the source.
func mockFileMayMatch(content []byte) bool {
	return bytes.Contains(content, []byte("org.mockito")) || bytes.Contains(content, []byte("slack.test.mockito"))
}

func (a *mockAnalyzer) analyzeFile(file *scanner.File) {
	kotlin := file.Language == scanner.LangKotlin
	file.FlatWalkAllNodes(0, func(idx uint32) {
		switch file.FlatType(idx) {
		case "call_expression", "method_invocation":
			if t, ok := mockCallType(file, idx, 0); ok {
				a.check(file, idx, t)
			}
		case "property_declaration":
			if kotlin && mockIsMember(file, idx) && mockHasAnnotation(file, idx) {
				if t, ok := mockDeclaredType(file, idx); ok {
					a.check(file, idx, t)
				}
			}
		case "field_declaration":
			if !kotlin && mockHasAnnotation(file, idx) {
				if t, ok := mockDeclaredType(file, idx); ok {
					a.check(file, idx, t)
				}
			}
		}
	})
}

// mockType is a type reference as written in file.
type mockType struct {
	file *scanner.File
	text string // without type arguments or nullability, e.g. "Foo" or "Outer.Inner"
	// classOf marks a java.lang.Class<text> (from a class literal), which a `mock` call unwraps.
	classOf bool
}

func newMockType(file *scanner.File, node uint32, classOf bool) (mockType, bool) {
	text := strings.Join(strings.Fields(file.FlatNodeText(node)), "")
	if i := strings.IndexByte(text, '<'); i >= 0 {
		text = text[:i]
	}
	text = strings.TrimSuffix(text, "?")
	if text == "" || strings.ContainsAny(text, "()->*[]@") {
		return mockType{}, false
	}
	return mockType{file: file, text: text, classOf: classOf}, true
}

// mockCallType returns the type a mock factory call creates, or false when idx is not one or the type is not
// provable from source.
func mockCallType(file *scanner.File, idx uint32, depth int) (mockType, bool) {
	call, ok := lintCallAt(file, idx)
	if !ok {
		return mockType{}, false
	}
	kind := mockFactoryKind(file, call)
	if kind == "" {
		return mockType{}, false
	}
	if file.FlatType(idx) == "call_expression" {
		if typeArg := mockFirstTypeArgument(file, idx); typeArg != 0 {
			return newMockType(file, typeArg, false)
		}
	}
	if len(call.Args) > 0 {
		t, ok := mockExpressionType(file, call.Args[0], depth+1)
		if !ok {
			return mockType{}, false
		}
		if kind == "mock" {
			// mock(Class<T>) creates a T; a mock of anything else is not a factory MockDetector reads.
			if !t.classOf {
				return mockType{}, false
			}
			t.classOf = false
			return t, true
		}
		// spy(instance) creates a spy of the instance's type.
		return t, !t.classOf
	}
	if kind == "mock" && file.Language == scanner.LangKotlin {
		return mockExpectedType(file, idx)
	}
	return mockType{}, false
}

// mockFactoryKind returns "mock" or "spy" when the call resolves to one of mockFactories, under its own name or
// an import alias (`import org.mockito.Mockito.mock as m`), and "" otherwise.
func mockFactoryKind(file *scanner.File, call lintCall) string {
	if call.ReceiverText != "" {
		for _, owner := range mockFactoryOwners {
			if mockFactories[owner+"."+call.Name] && lintTypeRefMatches(file, call.ReceiverText, owner) {
				return call.Name
			}
		}
		return ""
	}
	if mockFileDeclaresFunction(file, call.Name) {
		return ""
	}
	imp := lintImportsOf(file)
	if full, ok := imp.bySimple[call.Name]; ok {
		if mockFactories[full] {
			return full[strings.LastIndex(full, ".")+1:]
		}
		return ""
	}
	for _, star := range imp.stars {
		if mockFactories[star+"."+call.Name] {
			return call.Name
		}
	}
	return ""
}

// mockFileDeclaresFunction reports whether the file declares a function called name, which shadows an import.
func mockFileDeclaresFunction(file *scanner.File, name string) bool {
	return filefacts.FileFact(fileFactsCache(), file, slotMockLocalFunctions, func() map[string]bool {
		names := map[string]bool{}
		for _, kind := range []string{"function_declaration", "method_declaration"} {
			file.FlatWalkNodes(0, kind, func(fn uint32) {
				names[extractIdentifierFlat(file, fn)] = true
			})
		}
		return names
	})[name]
}

const slotMockLocalFunctions = "mockLocalFunctions"

// mockFirstTypeArgument returns the type of a Kotlin call's first explicit type argument, or 0.
func mockFirstTypeArgument(file *scanner.File, call uint32) uint32 {
	suffix, ok := file.FlatFindChild(call, "call_suffix")
	if !ok {
		return 0
	}
	args, ok := file.FlatFindChild(suffix, "type_arguments")
	if !ok {
		return 0
	}
	projection, ok := file.FlatFindChild(args, "type_projection")
	if !ok {
		return 0
	}
	for child := file.FlatFirstChild(projection); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) && file.FlatType(child) != "type_projection_modifiers" {
			return child
		}
	}
	return 0
}

// mockExpressionType returns the type of a factory argument: a class literal, a constructor call, another mock,
// or a variable whose declaration shows its type.
func mockExpressionType(file *scanner.File, expr uint32, depth int) (mockType, bool) {
	if expr == 0 || depth > 4 {
		return mockType{}, false
	}
	switch file.FlatType(expr) {
	case "class_literal": // Java Foo.class
		if t, ok := file.FlatFindChild(expr, "type_identifier"); ok {
			return newMockType(file, t, true)
		}
		if t, ok := file.FlatFindChild(expr, "scoped_type_identifier"); ok {
			return newMockType(file, t, true)
		}
	case "navigation_expression": // Kotlin Foo::class.java, or Outer.Inner::class.java
		ref := file.FlatFirstChild(expr)
		suffix := file.FlatNextSib(ref)
		if strings.TrimSpace(strings.TrimPrefix(file.FlatNodeText(suffix), ".")) != "java" {
			break
		}
		switch file.FlatType(ref) {
		case "callable_reference":
			if t := mockClassReferenceType(file, ref); t != 0 {
				return newMockType(file, t, true)
			}
		case "navigation_expression":
			// Outer.Inner::class parses as a navigation whose last suffix is `::class`.
			owner := file.FlatFirstChild(ref)
			classSuffix := file.FlatNextSib(owner)
			if file.FlatNextSib(classSuffix) == 0 && strings.Join(strings.Fields(file.FlatNodeText(classSuffix)), "") == "::class" {
				return newMockType(file, owner, true)
			}
		}
	case "call_expression", "method_invocation":
		if t, ok := mockCallType(file, expr, depth); ok {
			return t, true
		}
		// A Kotlin constructor call Foo(...).
		if file.FlatType(expr) == "call_expression" {
			callee := file.FlatFirstChild(expr)
			if file.FlatType(callee) == "simple_identifier" {
				name := file.FlatNodeText(callee)
				if name != "" && name[0] >= 'A' && name[0] <= 'Z' {
					return newMockType(file, callee, false)
				}
			}
		}
	case "object_creation_expression": // Java new Foo(...)
		for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "type_identifier", "scoped_type_identifier", "generic_type":
				return newMockType(file, child, false)
			}
		}
	case "simple_identifier", "identifier":
		return mockVariableType(file, expr, depth)
	}
	return mockType{}, false
}

// mockClassReferenceType returns the type named by a Kotlin `Foo::class` reference, or 0.
func mockClassReferenceType(file *scanner.File, ref uint32) uint32 {
	first := file.FlatFirstChild(ref)
	if first == 0 || file.FlatType(first) == "::" {
		return 0
	}
	last := file.FlatNextSib(first)
	if file.FlatType(last) != "::" || !strings.HasSuffix(strings.TrimSpace(file.FlatNodeText(ref)), "::class") {
		return 0
	}
	return first
}

// mockVariableType returns the type of the local variable (or Java field) a name refers to, from its declared
// type or its initializer. The nearest declaration before the use wins, so an inner local shadows an outer one;
// the search ends at the enclosing named function or class.
func mockVariableType(file *scanner.File, use uint32, depth int) (mockType, bool) {
	name := file.FlatNodeText(use)
	child := use
	for parent, ok := file.FlatParent(use); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "statements", "block", "class_body":
			for sib, found := file.FlatPrevSibling(child); found; sib, found = file.FlatPrevSibling(sib) {
				if t, matched, ok := mockDeclarationType(file, sib, name, depth); matched {
					return t, ok
				}
			}
			if file.FlatType(parent) == "class_body" {
				return mockType{}, false
			}
		case "function_declaration", "method_declaration", "constructor_declaration", "class_declaration", "object_declaration":
			return mockType{}, false
		}
		child = parent
	}
	return mockType{}, false
}

// mockDeclarationType reports whether decl declares name and, if so, the type it shows.
func mockDeclarationType(file *scanner.File, decl uint32, name string, depth int) (t mockType, matched, ok bool) {
	switch file.FlatType(decl) {
	case "property_declaration":
		varDecl, found := file.FlatFindChild(decl, "variable_declaration")
		if !found || extractIdentifierFlat(file, varDecl) != name {
			return mockType{}, false, false
		}
		if typ := mockVariableDeclaredType(file, varDecl); typ != 0 {
			t, ok = mockTypeOrClassOf(file, typ)
			return t, true, ok
		}
		init := lintInitializerAfterEquals(file, decl)
		t, ok = mockExpressionType(file, init, depth+1)
		return t, true, ok
	case "local_variable_declaration", "field_declaration":
		for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) != "variable_declarator" || extractIdentifierFlat(file, child) != name {
				continue
			}
			if typ := mockJavaDeclaredType(file, decl); typ != 0 {
				t, ok = mockTypeOrClassOf(file, typ)
				return t, true, ok
			}
			return mockType{}, true, false
		}
	}
	return mockType{}, false, false
}

// mockTypeOrClassOf reads a declared type, unwrapping Class<T> or KClass-free `Class<T>` to a class-of T.
func mockTypeOrClassOf(file *scanner.File, typ uint32) (mockType, bool) {
	t, ok := newMockType(file, typ, false)
	if !ok {
		return t, false
	}
	if t.text == "Class" || t.text == "java.lang.Class" {
		if arg := mockTypeArgumentOf(file, typ); arg != 0 {
			return newMockType(file, arg, true)
		}
		return mockType{}, false
	}
	return t, true
}

// mockTypeArgumentOf returns the first type argument of a Kotlin user_type or Java generic_type, or 0.
func mockTypeArgumentOf(file *scanner.File, typ uint32) uint32 {
	var found uint32
	file.FlatWalkAllNodes(typ, func(n uint32) {
		if found != 0 || file.FlatType(n) != "type_arguments" {
			return
		}
		for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			if file.FlatType(child) == "type_projection" {
				for inner := file.FlatFirstChild(child); inner != 0; inner = file.FlatNextSib(inner) {
					if file.FlatIsNamed(inner) {
						found = inner
						return
					}
				}
			}
			found = child
			return
		}
	})
	return found
}

// mockVariableDeclaredType returns the declared type node of a Kotlin variable_declaration, or 0.
func mockVariableDeclaredType(file *scanner.File, varDecl uint32) uint32 {
	afterColon := false
	for child := file.FlatFirstChild(varDecl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == ":" {
			afterColon = true
			continue
		}
		if afterColon && file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

// mockJavaDeclaredType returns the type node of a Java field or local declaration, or 0 for `var`.
func mockJavaDeclaredType(file *scanner.File, decl uint32) uint32 {
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "scoped_type_identifier", "generic_type":
			if file.FlatNodeTextEquals(child, "var") {
				return 0
			}
			return child
		case "variable_declarator":
			return 0
		}
	}
	return 0
}

// mockExpectedType returns the type a no-argument Kotlin `mock()` takes from the declaration it initializes.
func mockExpectedType(file *scanner.File, call uint32) (mockType, bool) {
	parent, _ := file.FlatParent(call)
	if file.FlatType(parent) != "property_declaration" || lintInitializerAfterEquals(file, parent) != call {
		return mockType{}, false
	}
	varDecl, ok := file.FlatFindChild(parent, "variable_declaration")
	if !ok {
		return mockType{}, false
	}
	if typ := mockVariableDeclaredType(file, varDecl); typ != 0 {
		return newMockType(file, typ, false)
	}
	return mockType{}, false
}

// mockDeclaredType returns the declared type of a @Mock/@Spy Kotlin property or Java field.
func mockDeclaredType(file *scanner.File, decl uint32) (mockType, bool) {
	if file.FlatType(decl) == "field_declaration" {
		if typ := mockJavaDeclaredType(file, decl); typ != 0 {
			return newMockType(file, typ, false)
		}
		return mockType{}, false
	}
	varDecl, ok := file.FlatFindChild(decl, "variable_declaration")
	if !ok {
		return mockType{}, false
	}
	if typ := mockVariableDeclaredType(file, varDecl); typ != 0 {
		return newMockType(file, typ, false)
	}
	return mockExpressionType(file, lintInitializerAfterEquals(file, decl), 1)
}

// mockIsMember reports whether a Kotlin property is a class member or top-level property (a UField in Lint),
// not a local variable.
func mockIsMember(file *scanner.File, decl uint32) bool {
	parent, _ := file.FlatParent(decl)
	switch file.FlatType(parent) {
	case "class_body", "source_file", "enum_class_body":
		return true
	}
	return false
}

func mockHasAnnotation(file *scanner.File, decl uint32) bool {
	anns := moshiAnnotationsOf(file, decl)
	for _, fqn := range mockAnnotations {
		if moshiFindAnnotation(file, anns, fqn) != nil {
			return true
		}
	}
	return false
}

// mockClass is a resolved mocked class.
type mockClass struct {
	fqn  string        // Lint's qualified name, with Kotlin built-ins mapped to their Java classes
	file *scanner.File // the declaring file when the class is in the project, else nil
	decl uint32
}

// resolve resolves a type reference to a class, or reports false when the source does not show which class it
// names.
func (a *mockAnalyzer) resolve(t mockType) (mockClass, bool) {
	fqn, ok := mockQualifiedName(a.result.index, t.file, t.text)
	if !ok {
		return mockClass{}, false
	}
	c := mockClass{fqn: mockJavaName(fqn)}
	if sym, found := a.result.index.SymbolByFQN(fqn); found {
		c.file, c.decl = a.declaration(sym)
	}
	return c, true
}

// mockQualifiedName resolves a type reference written in file to a fully qualified name.
func mockQualifiedName(index *scanner.CodeIndex, file *scanner.File, text string) (string, bool) {
	head, rest, nested := strings.Cut(text, ".")
	imp := lintImportsOf(file)
	if full, ok := imp.bySimple[head]; ok {
		if nested {
			return full + "." + rest, true
		}
		return full, true
	}
	if nested {
		if _, ok := index.SymbolByFQN(text); ok || (head != "" && head[0] >= 'a' && head[0] <= 'z') {
			return text, true
		}
	}
	// A class declared in the file, such as a test's private nested class, is in scope by its simple name.
	if !nested {
		if fqn, ok := mockSameFileClass(index, file, text); ok {
			return fqn, true
		}
	}
	if pkg := moshiFilePackage(file); pkg != "" {
		if _, ok := index.SymbolByFQN(pkg + "." + text); ok {
			return pkg + "." + text, true
		}
	} else if _, ok := index.SymbolByFQN(text); ok {
		return text, true
	}
	for _, star := range imp.stars {
		if _, ok := index.SymbolByFQN(star + "." + text); ok {
			return star + "." + text, true
		}
	}
	if nested {
		return "", false
	}
	if file.Language == scanner.LangKotlin {
		if fqn, ok := mockKotlinDefaultImports[text]; ok {
			return fqn, true
		}
	}
	if javaLangClasses[text] {
		return "java.lang." + text, true
	}
	// A class outside the project named through the file's only on-demand import of a platform package
	// (`import android.content.*`) comes from that package. With several on-demand imports the package is unknown.
	if len(imp.stars) == 1 && mockPlatformRoots[strings.SplitN(imp.stars[0], ".", 2)[0]] && mockLooksLikeClass(text) &&
		len(index.SymbolsNamed(text)) == 0 {
		return imp.stars[0] + "." + text, true
	}
	return "", false
}

// mockSameFileClass returns the FQN of the one class the file declares with this simple name, at any nesting.
func mockSameFileClass(index *scanner.CodeIndex, file *scanner.File, name string) (string, bool) {
	fqn := ""
	for _, sym := range index.SymbolsNamed(name) {
		if sym.File != file.Path || sym.Name != name {
			continue
		}
		switch sym.Kind {
		case "class", "interface", "object", "enum", "record", "annotation":
			if fqn != "" && fqn != sym.FQN {
				return "", false
			}
			fqn = sym.FQN
		}
	}
	return fqn, fqn != ""
}

// declaration returns the declaration node of an indexed class.
func (a *mockAnalyzer) declaration(sym scanner.Symbol) (*scanner.File, uint32) {
	if a.byPath == nil {
		a.byPath = make(map[string]*scanner.File, len(a.result.index.Files))
		for _, f := range a.result.index.Files {
			if f != nil {
				a.byPath[f.Path] = f
			}
		}
	}
	file := a.byPath[sym.File]
	if file == nil || file.FlatTree == nil {
		return nil, 0
	}
	node, ok := file.FlatNamedDescendantForByteRange(uint32(sym.StartByte), uint32(sym.EndByte))
	if !ok {
		return nil, 0
	}
	for n := node; n != 0; n, _ = file.FlatParent(n) {
		switch file.FlatType(n) {
		case "class_declaration", "object_declaration", "interface_declaration", "record_declaration", "enum_declaration", "annotation_type_declaration":
			if int(file.FlatStartByte(n)) == sym.StartByte {
				return file, n
			}
		}
		if int(file.FlatStartByte(n)) < sym.StartByte {
			break
		}
	}
	return nil, 0
}

// check runs MockDetector's type checkers, in its order, on a mock site.
func (a *mockAnalyzer) check(file *scanner.File, site uint32, t mockType) {
	c, ok := a.resolve(t)
	if !ok {
		// Lint still knows a class it can resolve from the classpath; DoNotMockAnything needs no more than that.
		if mockLooksLikeClass(t.text) {
			a.report(mockDoNotMockAnything, file, site, "Do not add new mocks.")
		}
		return
	}
	a.report(mockDoNotMockAnything, file, site, "Do not add new mocks.")
	if mockPlatformRoots[strings.SplitN(c.fqn, ".", 2)[0]] {
		a.report(mockDoNotMockPlatformTypes, file, site, fmt.Sprintf("platform type '%s' should not be mocked", c.fqn))
		return
	}
	if c.file == nil {
		return
	}
	if c.file.FlatType(c.decl) == "class_declaration" && c.file.FlatFindModifierNode(c.decl, "data") != 0 &&
		!mockClassHasAnnotation(c, "kotlin.jvm.JvmRecord") {
		a.report(mockDoNotMockDataClass, file, site, fmt.Sprintf("'%s' is a data class, so mocking it should not be necessary", c.fqn))
		return
	}
	for _, fqn := range mockDoNotMockAnnotations {
		if mockClassHasAnnotation(c, fqn) {
			return
		}
	}
	if mockIsSealed(c) {
		a.report(mockDoNotMockSealedClass, file, site, fmt.Sprintf("'%s' is a sealed type and has a restricted type hierarchy, use a subtype instead.", c.fqn))
		return
	}
	for _, fqn := range mockAutoValueAnnotations {
		if mockClassHasAnnotation(c, fqn) {
			a.report(mockDoNotMockAutoValue, file, site, "Mocked type is annotated with non-mockable annotation "+fqn)
			return
		}
	}
	if c.file.FlatType(c.decl) == "object_declaration" {
		a.report(mockDoNotMockObjectClass, file, site, fmt.Sprintf("'%s' is an object, so mocking it should not be necessary", c.fqn))
		return
	}
	if c.file.FlatType(c.decl) == "record_declaration" ||
		(c.file.FlatFindModifierNode(c.decl, "data") != 0 && mockClassHasAnnotation(c, "kotlin.jvm.JvmRecord")) {
		a.report(mockDoNotMockRecordClass, file, site, fmt.Sprintf("'%s' is a record class, so mocking it should not be necessary", c.fqn))
	}
}

// mockIsSealed reports a Kotlin sealed class or interface, or a Java type declared sealed or with a permits clause.
func mockIsSealed(c mockClass) bool {
	if c.file.FlatFindModifierNode(c.decl, "sealed") != 0 {
		return true
	}
	for child := c.file.FlatFirstChild(c.decl); child != 0; child = c.file.FlatNextSib(child) {
		if c.file.FlatType(child) == "permits" {
			return true
		}
	}
	return false
}

func (a *mockAnalyzer) report(rule string, file *scanner.File, node uint32, msg string) {
	a.result.byRule[rule] = append(a.result.byRule[rule], mockFinding{rule: rule, file: file, node: node, msg: msg})
}

func mockClassHasAnnotation(c mockClass, fqn string) bool {
	anns := moshiAnnotationsOf(c.file, c.decl)
	if moshiFindAnnotation(c.file, anns, fqn) != nil {
		return true
	}
	// kotlin.jvm.* is a Kotlin default import.
	if c.file.Language == scanner.LangKotlin && strings.HasPrefix(fqn, "kotlin.jvm.") {
		return moshiFindAnnotation(c.file, anns, strings.TrimPrefix(fqn, "kotlin.jvm.")) != nil
	}
	return false
}

// mockLooksLikeClass reports whether an unresolved type reference names a class, not a type parameter.
func mockLooksLikeClass(text string) bool {
	last := text[strings.LastIndex(text, ".")+1:]
	return len(last) > 1 && last[0] >= 'A' && last[0] <= 'Z'
}

// mockJavaName maps a Kotlin built-in to the Java class Lint's PSI sees, as Lint names it in messages.
func mockJavaName(fqn string) string {
	if mapped, ok := mockKotlinToJava[fqn]; ok {
		return mapped
	}
	return fqn
}

var mockKotlinToJava = map[string]string{
	"kotlin.Any":                             "java.lang.Object",
	"kotlin.String":                          "java.lang.String",
	"kotlin.CharSequence":                    "java.lang.CharSequence",
	"kotlin.Number":                          "java.lang.Number",
	"kotlin.Comparable":                      "java.lang.Comparable",
	"kotlin.Throwable":                       "java.lang.Throwable",
	"kotlin.Enum":                            "java.lang.Enum",
	"kotlin.Annotation":                      "java.lang.annotation.Annotation",
	"kotlin.Cloneable":                       "java.lang.Cloneable",
	"kotlin.collections.Iterable":            "java.lang.Iterable",
	"kotlin.collections.MutableIterable":     "java.lang.Iterable",
	"kotlin.collections.Iterator":            "java.util.Iterator",
	"kotlin.collections.MutableIterator":     "java.util.Iterator",
	"kotlin.collections.ListIterator":        "java.util.ListIterator",
	"kotlin.collections.MutableListIterator": "java.util.ListIterator",
	"kotlin.collections.Collection":          "java.util.Collection",
	"kotlin.collections.MutableCollection":   "java.util.Collection",
	"kotlin.collections.List":                "java.util.List",
	"kotlin.collections.MutableList":         "java.util.List",
	"kotlin.collections.Set":                 "java.util.Set",
	"kotlin.collections.MutableSet":          "java.util.Set",
	"kotlin.collections.Map":                 "java.util.Map",
	"kotlin.collections.MutableMap":          "java.util.Map",
	"kotlin.collections.ArrayList":           "java.util.ArrayList",
	"kotlin.collections.HashMap":             "java.util.HashMap",
	"kotlin.collections.LinkedHashMap":       "java.util.LinkedHashMap",
	"kotlin.collections.HashSet":             "java.util.HashSet",
	"kotlin.collections.LinkedHashSet":       "java.util.LinkedHashSet",
	"kotlin.Exception":                       "java.lang.Exception",
	"kotlin.RuntimeException":                "java.lang.RuntimeException",
	"kotlin.Error":                           "java.lang.Error",
	"kotlin.IllegalStateException":           "java.lang.IllegalStateException",
	"kotlin.IllegalArgumentException":        "java.lang.IllegalArgumentException",
	"kotlin.text.StringBuilder":              "java.lang.StringBuilder",
	"kotlin.text.Appendable":                 "java.lang.Appendable",
	"kotlin.Comparator":                      "java.util.Comparator",
}

// mockKotlinDefaultImports maps the class names a Kotlin file can use without an import to their FQNs:
// kotlin.*, kotlin.collections.*, kotlin.io.*, kotlin.ranges.*, kotlin.sequences.*, kotlin.text.* and
// java.lang.* classes that are commonly mocked or referenced.
var mockKotlinDefaultImports = func() map[string]string {
	m := map[string]string{}
	for fqn := range mockKotlinToJava {
		m[fqn[strings.LastIndex(fqn, ".")+1:]] = fqn
	}
	for _, fqn := range []string{
		"kotlin.Lazy", "kotlin.Pair", "kotlin.Triple", "kotlin.Result", "kotlin.Unit", "kotlin.Function",
		"kotlin.sequences.Sequence", "kotlin.ranges.IntRange", "kotlin.ranges.LongRange", "kotlin.text.Regex",
		"kotlin.io.FileTreeWalk",
	} {
		m[fqn[strings.LastIndex(fqn, ".")+1:]] = fqn
	}
	for name := range javaLangClasses {
		if _, ok := m[name]; !ok {
			m[name] = "java.lang." + name
		}
	}
	return m
}()

// javaLangClasses are java.lang classes, which Java and Kotlin files use without an import.
var javaLangClasses = map[string]bool{
	"Object": true, "String": true, "CharSequence": true, "Comparable": true, "Iterable": true, "Number": true,
	"Integer": true, "Long": true, "Short": true, "Byte": true, "Boolean": true, "Character": true, "Double": true,
	"Float": true, "Void": true, "Runnable": true, "Thread": true, "ThreadLocal": true, "Throwable": true,
	"Exception": true, "RuntimeException": true, "Error": true, "IllegalStateException": true,
	"IllegalArgumentException": true, "StringBuilder": true, "Appendable": true, "AutoCloseable": true,
	"Cloneable": true, "Class": true, "ClassLoader": true, "Enum": true, "Process": true, "Runtime": true,
	"System": true, "Math": true,
}
