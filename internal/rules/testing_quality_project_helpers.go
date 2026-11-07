package rules

import (
	"bytes"
	"path/filepath"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// testingQualityHelperDepth bounds how many helper-calls-helper hops count as an assertion.
const testingQualityHelperDepth = 3

// testingQualityHelperDenylist holds names that never match as cross-file assertion helpers. They are
// lifecycle hooks and generic utility verbs that production code, fakes and JUnit rules declare for
// unrelated reasons, so a body containing check/error/throw says nothing about the call site. Names
// are compared case-insensitively. Prefix rules live in testingQualityHelperNameIsGeneric.
var testingQualityHelperDenylist = map[string]struct{}{
	"setup": {}, "teardown": {}, "reset": {}, "attach": {}, "detach": {}, "oncreate": {}, "onstart": {},
	"onstop": {}, "build": {}, "invoke": {}, "first": {}, "last": {}, "test": {}, "process": {},
	"run": {}, "start": {}, "stop": {}, "clear": {}, "init": {}, "tostring": {}, "equals": {},
	"hashcode": {}, "enqueue": {}, "get": {}, "set": {}, "apply": {}, "create": {},
	"advanceuntilidle": {}, "runcurrent": {}, "advancetimeby": {}, "emit": {}, "collect": {},
	"bind": {}, "unbind": {}, "add": {}, "remove": {}, "put": {}, "load": {}, "open": {},
	"close": {}, "dispose": {}, "cancel": {}, "launch": {}, "send": {}, "post": {}, "update": {},
	"execute": {}, "dispatch": {}, "register": {}, "unregister": {}, "show": {}, "hide": {},
	"render": {}, "initialize": {}, "configure": {}, "prepare": {}, "finish": {}, "destroy": {},
}

// testingQualityHelperGenericPrefixes are denied when followed by an upper-case letter or nothing,
// so `handleSaveClick`, `onResume` and `updateSettings` never match but `onlyOnce` is untouched.
var testingQualityHelperGenericPrefixes = []string{"handle", "on", "get", "set", "update", "create", "build"}

func testingQualityHelperNameIsGeneric(name string) bool {
	if _, ok := testingQualityHelperDenylist[strings.ToLower(name)]; ok {
		return true
	}
	for _, prefix := range testingQualityHelperGenericPrefixes {
		if testingQualityHasLowerPrefixWord(name, prefix) {
			return true
		}
	}
	return false
}

// testingQualityAssertionLikeName reports names that read as a check, for call sites whose receiver
// type is unknown and whose helper name is declared in more than one place.
func testingQualityAssertionLikeName(name string) bool {
	for _, prefix := range []string{"assert", "verify", "expect", "check", "await", "ensure", "require", "waitFor", "validate", "confirm"} {
		if testingQualityHasLowerPrefixWord(name, prefix) {
			return true
		}
	}
	return false
}

// testingQualityHelperDecl is one function declaration anywhere in the project. Only declarations in
// non-test test-support files can be asserting; every other declaration still counts toward ambiguity.
type testingQualityHelperDecl struct {
	path      string
	owner     string // enclosing class/object name; empty for top-level functions
	extension bool
	asserting bool
	callees   []string
	body      uint32
	file      *scanner.File
}

// testingQualityProjectHelpers indexes every function declaration by name. A name is an asserting
// helper only when every declaration of it is asserting.
type testingQualityProjectHelpers struct {
	decls map[string][]*testingQualityHelperDecl
	// fileTypes caches, per file path, the declared type names of identifiers.
	fileTypes map[string]map[string][]string
}

func (h *testingQualityProjectHelpers) empty() bool { return h == nil || len(h.decls) == 0 }

// nameAsserts reports whether every declaration of name is asserting.
func (h *testingQualityProjectHelpers) nameAsserts(name string) bool {
	decls := h.decls[name]
	if len(decls) == 0 {
		return false
	}
	for _, d := range decls {
		if !d.asserting {
			return false
		}
	}
	return true
}

// bodyCallsAssertingHelper reports whether the body calls a function that a test-support file other
// than file declares as asserting. Same-file helpers are handled by testingQualityAssertionHelperNames.
func (h *testingQualityProjectHelpers) bodyCallsAssertingHelper(file *scanner.File, body uint32) bool {
	if h.empty() || file == nil || body == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(body, "call_expression", func(call uint32) {
		if !found && h.callMatches(file, call) {
			found = true
		}
	})
	return found
}

func (h *testingQualityProjectHelpers) callMatches(file *scanner.File, call uint32) bool {
	name := flatCallNameAny(file, call)
	if name == "" || testingQualityHelperNameIsGeneric(name) || !h.nameAsserts(name) {
		return false
	}
	decls := h.decls[name]
	receiverKnown, receiverTypes := h.receiverTypes(file, call)
	var candidates []*testingQualityHelperDecl
	for _, d := range decls {
		if d.path == file.Path {
			continue
		}
		if receiverKnown && !d.extension && d.owner != "" && !containsString(receiverTypes, d.owner) {
			continue
		}
		candidates = append(candidates, d)
	}
	if len(candidates) == 0 {
		return false
	}
	if receiverKnown {
		return true
	}
	return testingQualityAssertionLikeName(name) || len(decls) == 1
}

// receiverTypes returns the declared type names of a call's explicit receiver when it is a simple
// identifier whose type is visible in the file. known is false for implicit or complex receivers.
func (h *testingQualityProjectHelpers) receiverTypes(file *scanner.File, call uint32) (known bool, types []string) {
	nav, _ := flatCallExpressionParts(file, call)
	if nav == 0 && file.FlatType(call) == "call_expression" {
		// Trailing-lambda form nests the navigation under an inner call.
		for child := file.FlatFirstChild(call); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "call_expression" {
				nav, _ = flatCallExpressionParts(file, child)
			}
		}
	}
	if nav == 0 {
		return false, nil
	}
	recv := flatNavigationExpressionReceiver(file, nav)
	if recv == 0 || file.FlatType(recv) != "simple_identifier" {
		return false, nil
	}
	ident := file.FlatNodeText(recv)
	if h.fileTypes == nil {
		h.fileTypes = make(map[string]map[string][]string)
	}
	byName, ok := h.fileTypes[file.Path]
	if !ok {
		byName = testingQualityDeclaredTypes(file)
		h.fileTypes[file.Path] = byName
	}
	types = byName[ident]
	return len(types) > 0, types
}

// testingQualityDeclaredTypes maps property, parameter and local names to their declared type name
// and constructor-call initializer name.
func testingQualityDeclaredTypes(file *scanner.File) map[string][]string {
	out := make(map[string][]string)
	add := func(name, typ string) {
		if name != "" && typ != "" && !containsString(out[name], typ) {
			out[name] = append(out[name], typ)
		}
	}
	file.FlatWalkAllNodes(0, func(n uint32) {
		switch file.FlatType(n) {
		case "property_declaration":
			variable, _ := file.FlatFindChild(n, "variable_declaration")
			if variable == 0 {
				return
			}
			name := ""
			if id, ok := file.FlatFindChild(variable, "simple_identifier"); ok {
				name = file.FlatNodeText(id)
			}
			add(name, testingQualityFirstTypeName(file, variable))
			for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) == "call_expression" {
					init := flatCallExpressionName(file, child)
					if init != "" && init[0] >= 'A' && init[0] <= 'Z' {
						add(name, init)
					}
				}
			}
		case "parameter", "class_parameter":
			name := ""
			if id, ok := file.FlatFindChild(n, "simple_identifier"); ok {
				name = file.FlatNodeText(id)
			}
			add(name, testingQualityFirstTypeName(file, n))
		}
	})
	return out
}

func testingQualityFirstTypeName(file *scanner.File, parent uint32) string {
	name := ""
	file.FlatWalkAllNodes(parent, func(n uint32) {
		if name == "" && file.FlatType(n) == "type_identifier" {
			name = file.FlatNodeText(n)
		}
	})
	return name
}

// testingQualityHelperEligible reports whether a file may host asserting helpers: Kotlin test-support
// sources that are neither production code nor test classes.
func testingQualityHelperEligible(file *scanner.File) bool {
	if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin || !isTestSupportFile(file.Path) {
		return false
	}
	lower := strings.ToLower(filepath.ToSlash(file.Path))
	if strings.Contains(lower, "/src/main/") && !testingQualityInTestingDir(lower) {
		return false
	}
	hasTest := false
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		if !hasTest && testingQualityIsTestFunction(file, fn) {
			hasTest = true
		}
	})
	return !hasTest
}

func testingQualityInTestingDir(lowerPath string) bool {
	return strings.Contains(lowerPath, "-testing/") || strings.Contains(lowerPath, "/testing/") ||
		strings.Contains(lowerPath, "/test-utils/") || strings.Contains(lowerPath, "-test-utils/")
}

// testingQualityHelperFunctionExcluded reports members that are lifecycle hooks, rules or interface
// overrides rather than purpose-built assertion helpers.
func testingQualityHelperFunctionExcluded(file *scanner.File, fn uint32) bool {
	if file.FlatHasModifier(fn, "override") || file.FlatHasModifier(fn, "operator") {
		return true
	}
	for _, annotation := range []string{"Before", "After", "BeforeEach", "AfterEach", "BeforeClass", "AfterClass", "BeforeAll", "AfterAll", "Test", "Rule", "ParameterizedTest"} {
		if flatHasAnnotationNamed(file, fn, annotation) {
			return true
		}
	}
	return false
}

func testingQualityHelperOwner(file *scanner.File, fn uint32) string {
	for current, ok := file.FlatParent(fn); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "class_declaration", "object_declaration":
			if name := extractIdentifierFlat(file, current); name != "" {
				return name
			}
		}
	}
	return ""
}

// testingQualityProjectAssertionHelpers indexes every function in the project and marks the ones in
// test-support helper files whose bodies fail on a violated condition: they call check/require, fail
// or throw inside a condition, call an assert/verify/expect style function, or call another such
// helper within testingQualityHelperDepth hops.
func testingQualityProjectAssertionHelpers(files []*scanner.File) *testingQualityProjectHelpers {
	helpers := &testingQualityProjectHelpers{decls: make(map[string][]*testingQualityHelperDecl)}
	for _, file := range files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin {
			continue
		}
		eligible := testingQualityHelperEligible(file)
		file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
			name := testingQualityFunctionName(file, fn)
			if name == "" {
				return
			}
			decl := &testingQualityHelperDecl{
				path:      file.Path,
				owner:     testingQualityHelperOwner(file, fn),
				extension: flatFunctionReceiverTypeName(file, fn) != "",
				file:      file,
			}
			helpers.decls[name] = append(helpers.decls[name], decl)
			if !eligible || testingQualityHelperNameIsGeneric(name) || testingQualityHelperFunctionExcluded(file, fn) {
				return
			}
			body, _ := file.FlatFindChild(fn, "function_body")
			if body == 0 {
				return
			}
			if testingQualityHelperBodyAsserts(file, body) {
				decl.asserting = true
				return
			}
			decl.body = body
			decl.callees = testingQualityCallNames(file, body)
		})
	}
	for depth := 0; depth < testingQualityHelperDepth; depth++ {
		var added []*testingQualityHelperDecl
		for name, decls := range helpers.decls {
			for _, d := range decls {
				if d.asserting || d.body == 0 {
					continue
				}
				for _, callee := range d.callees {
					if callee != name && !testingQualityHelperNameIsGeneric(callee) && helpers.nameAsserts(callee) {
						added = append(added, d)
						break
					}
				}
			}
		}
		// Apply after the scan so each round adds exactly one hop.
		for _, d := range added {
			d.asserting = true
		}
		if len(added) == 0 {
			break
		}
	}
	return helpers
}

func testingQualityCallNames(file *scanner.File, body uint32) []string {
	var names []string
	seen := make(map[string]struct{})
	file.FlatWalkNodes(body, "call_expression", func(call uint32) {
		name := flatCallNameAny(file, call)
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	})
	return names
}

// testingQualityHelperBodyAsserts reports a helper body with an assertion-like construct of its own.
// A throw, error or fail counts only inside a condition, so a stub that throws unconditionally
// (NotImplementedError, TODO()) is not an assertion helper.
func testingQualityHelperBodyAsserts(file *scanner.File, body uint32) bool {
	if testingQualityBodyHasAssertionOrVerificationWithHelpers(file, body, nil) ||
		testingQualityBodyHasExpectOrAwaitCall(file, body) {
		return true
	}
	found := false
	file.FlatWalkAllNodes(body, func(n uint32) {
		if found {
			return
		}
		switch file.FlatType(n) {
		case "jump_expression":
			found = strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(n)), "throw") &&
				testingQualityIsConditional(file, n, body)
		case "call_expression":
			switch flatCallNameAny(file, n) {
			case "require", "requireNotNull", "checkNotNull", "check":
				found = true
			case "error", "fail":
				found = testingQualityIsConditional(file, n, body)
			}
		}
	})
	return found
}

// testingQualityIsConditional reports whether n sits under an if, when, elvis or catch inside body.
func testingQualityIsConditional(file *scanner.File, n, body uint32) bool {
	for current, ok := file.FlatParent(n); ok && current != body; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "if_expression", "when_expression", "elvis_expression", "catch_block":
			return true
		}
	}
	return false
}

// check runs once per project. It indexes assertion helpers declared in test-support files, then
// inspects every @Test function in the indexed Kotlin files.
func (r *TestWithoutAssertionRule) check(ctx *api.Context) {
	index := ctx.CodeIndex
	if index == nil || len(index.Files) == 0 {
		return
	}
	helpers := testingQualityProjectAssertionHelpers(index.Files)
	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin {
			continue
		}
		if !bytes.Contains(file.Content, []byte("Test")) {
			continue
		}
		file.FlatWalkNodes(0, "function_declaration", func(idx uint32) {
			r.checkFunction(ctx, file, idx, helpers)
		})
	}
}

func (r *TestWithoutAssertionRule) checkFunction(ctx *api.Context, file *scanner.File, idx uint32, helpers *testingQualityProjectHelpers) {
	if testingQualityIsEditorTemplatePath(file.Path) {
		return
	}
	if !testingQualityIsTestFunction(file, idx) {
		return
	}
	if r.AllowNoAssertionTests {
		return
	}
	if testingQualityTestExpectsException(file, idx) {
		return
	}
	if testingQualityIsIgnoredTest(file, idx) {
		return
	}
	if testingQualityTestAllowsNoCrash(file, idx) {
		return
	}
	if testingQualityIsBenchmarkOrGoldenFile(file) {
		return
	}
	if testingQualityTestNameDocumentsNoException(file, idx) {
		return
	}
	body, _ := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return
	}
	// Error recovery (e.g. on `x.value = y`) can strand part of the body, so abstain.
	if flatSubtreeHasNodeType(file, body, "ERROR") || file.FlatHasChildOfType(idx, "ERROR") {
		return
	}
	// A fixture test with no body (`= Unit` or `{}`) exists to be discovered, not to verify.
	if testingQualityBodyIsEmpty(file, body) {
		return
	}
	// TestWithOnlyTodo reports a body that is just TODO().
	if testingQualityBodyIsOnlyTodoOrFail(file, body) {
		return
	}
	if testingQualityTextHasVerifyCall(file.FlatNodeText(idx)) {
		return
	}
	if testingQualityNameDocumentsNoOp(file, idx) || testingQualityIsConstructionDoesNotThrowTest(file, idx, body) {
		return
	}
	// A trailing cast throws on a wrong type, which is the check.
	if testingQualityBodyEndsWithCast(file, body) {
		return
	}
	if testingQualityBodyHasAssertionOrVerificationWithPatterns(file, body, r.AssertionMethodPatterns) ||
		testingQualityBodyHasExpectOrAwaitCall(file, body) ||
		helpers.bodyCallsAssertingHelper(file, body) {
		return
	}
	// A planted `throw`, `error(...)` or `fail(...)` makes the test fail when the code under test misbehaves.
	if testingQualityBodyPlantsFailure(file, body) {
		return
	}
	ctx.Emit(r.Finding(file, declLine(file, idx), declCol(file, idx), "Test function has no assertion; add a verification."))
}
