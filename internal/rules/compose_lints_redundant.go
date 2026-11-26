package rules

import (
	"bytes"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Messages of compose-lint-checks 1.6.1's RedundantComposableDetector, as Lint reports them.
const (
	composeRedundantComposableMessage = "This declaration is annotated with @Composable but doesn't call any other " +
		"@Composable functions or read any @Composable properties (like a CompositionLocal's current), so it doesn't " +
		"use the composition and the @Composable annotation can be removed. See " +
		"https://slackhq.github.io/compose-lints/rules/#remove-unnecessary-composable-annotations for more information."
	composeReadOnlyComposableMessage = "All composable functions and properties used by this declaration are marked " +
		"@ReadOnlyComposable, so this declaration can be marked @ReadOnlyComposable too. See " +
		"https://slackhq.github.io/compose-lints/rules/#remove-unnecessary-composable-annotations for more information."
)

// ComposeRedundantComposableRule ports compose-lints' ComposeRedundantComposable: a @Composable function, getter or
// annotated lambda that uses nothing from the composition. krit reports only what it can prove: every call and
// property read must resolve to a non-composable declaration in the project or a scanned library.
type ComposeRedundantComposableRule struct {
	BaseRule
	// IgnoreAnnotated lists annotation FQNs whose declarations are not reported (compose-lints `ignore-annotated`).
	IgnoreAnnotated []string
}

// ComposeReadOnlyComposableRule ports compose-lints' ComposeReadOnlyComposable: a @Composable declaration whose
// composable uses are all @ReadOnlyComposable (CompositionLocal reads, MaterialTheme.colorScheme, stringResource,
// ...).
type ComposeReadOnlyComposableRule struct {
	BaseRule
}

// composeLintsUsage orders compose-lints' CompositionUsage, plus unknown for what krit cannot classify.
type composeLintsUsage int8

const (
	composeLintsUsageNone composeLintsUsage = iota
	composeLintsUsageReadOnly
	composeLintsUsageOther
	composeLintsUsageUnknown
)

// composeLintsMergeUsage combines two usages: OTHER dominates, then unknown, then READ_ONLY.
func composeLintsMergeUsage(a, b composeLintsUsage) composeLintsUsage {
	switch {
	case a == composeLintsUsageOther || b == composeLintsUsageOther:
		return composeLintsUsageOther
	case a == composeLintsUsageUnknown || b == composeLintsUsageUnknown:
		return composeLintsUsageUnknown
	case a == composeLintsUsageReadOnly || b == composeLintsUsageReadOnly:
		return composeLintsUsageReadOnly
	}
	return composeLintsUsageNone
}

func composeLintsUsageOfKind(kind int8) composeLintsUsage {
	switch kind {
	case 1:
		return composeLintsUsageOther
	case 2:
		return composeLintsUsageReadOnly
	}
	return composeLintsUsageNone
}

// composeLintsStateTypes are Compose State types: reading `.value` uses the composition.
var composeLintsStateTypes = map[string]bool{"State": true, "MutableState": true, "IntState": true,
	"MutableIntState": true, "LongState": true, "MutableLongState": true, "FloatState": true,
	"MutableFloatState": true, "DoubleState": true, "MutableDoubleState": true}

// composeLintsStateFactories create a State without remember.
var composeLintsStateFactories = map[string]bool{"mutableStateOf": true, "derivedStateOf": true,
	"mutableIntStateOf": true, "mutableLongStateOf": true, "mutableFloatStateOf": true,
	"mutableDoubleStateOf": true}

// composeLintsUsageClassifier classifies calls and reads against the cross-file index and the library tables.
type composeLintsUsageClassifier struct {
	index *scanner.CodeIndex
	files map[string]*scanner.File
	nodes map[*scanner.File]map[int]uint32
	memo  map[string]composeLintsUsage
}

func newComposeLintsUsageClassifier(index *scanner.CodeIndex) *composeLintsUsageClassifier {
	files := make(map[string]*scanner.File, len(index.Files))
	for _, f := range index.Files {
		if f != nil {
			files[f.Path] = f
		}
	}
	return &composeLintsUsageClassifier{index: index, files: files, nodes: map[*scanner.File]map[int]uint32{},
		memo: map[string]composeLintsUsage{}}
}

// projectUsage classifies the project's functions and properties named name; ok is false when there are none.
func (c *composeLintsUsageClassifier) projectUsage(name string) (composeLintsUsage, bool) {
	if v, ok := c.memo[name]; ok {
		return v, v != composeLintsUsage(-1)
	}
	found := false
	result := composeLintsUsageNone
	mixed := false
	for _, sym := range c.index.SymbolsNamed(name) {
		file := c.files[sym.File]
		if file == nil {
			continue
		}
		var u composeLintsUsage
		switch sym.Kind {
		case "function":
			fn := c.node(file, sym.StartByte, "function_declaration")
			if fn == 0 {
				return c.remember(name, composeLintsUsageUnknown), true
			}
			u = composeLintsDeclUsage(file, fn)
		case "property":
			u = composeLintsPropertyUsage(file, c.node(file, sym.StartByte, "property_declaration"))
		case "class", "object", "interface":
			u = composeLintsUsageNone // a constructor or a type reference
		default:
			continue
		}
		if found && u != result {
			mixed = true
		}
		found, result = true, u
	}
	if !found {
		c.memo[name] = -1
		return composeLintsUsageNone, false
	}
	if mixed {
		result = composeLintsUsageUnknown
	}
	return c.remember(name, result), true
}

func (c *composeLintsUsageClassifier) remember(name string, u composeLintsUsage) composeLintsUsage {
	c.memo[name] = u
	return u
}

func (c *composeLintsUsageClassifier) node(file *scanner.File, start int, kind string) uint32 {
	byStart, ok := c.nodes[file]
	if !ok {
		byStart = map[int]uint32{}
		file.FlatWalkAllNodes(0, func(n uint32) {
			if t := file.FlatType(n); t == "function_declaration" || t == "property_declaration" {
				byStart[int(file.FlatStartByte(n))] = n
			}
		})
		c.nodes[file] = byStart
	}
	if n := byStart[start]; n != 0 && file.FlatType(n) == kind {
		return n
	}
	return 0
}

// composeLintsDeclUsage is how calling a function uses the composition: OTHER for @Composable, READ_ONLY for
// @ReadOnlyComposable, NONE otherwise.
func composeLintsDeclUsage(file *scanner.File, fn uint32) composeLintsUsage {
	if !acrFunctionIsComposable(file, fn) {
		return composeLintsUsageNone
	}
	if composeLintsHasAnnotationNamed(file, fn, "ReadOnlyComposable") {
		return composeLintsUsageReadOnly
	}
	return composeLintsUsageOther
}

// composeLintsPropertyUsage is how reading a property uses the composition, from its getter's annotations.
func composeLintsPropertyUsage(file *scanner.File, prop uint32) composeLintsUsage {
	if prop == 0 {
		return composeLintsUsageUnknown
	}
	getter := composeLintsPropertyGetter(file, prop)
	if getter == 0 || !acrHasComposableAnnotation(file, getter) {
		return composeLintsUsageNone
	}
	if composeLintsHasAnnotationNamed(file, getter, "ReadOnlyComposable") {
		return composeLintsUsageReadOnly
	}
	return composeLintsUsageOther
}

// composeLintsPropertyGetter returns a property's getter, inside it or as the detached sibling tree-sitter
// produces for a getter on its own line.
func composeLintsPropertyGetter(file *scanner.File, prop uint32) uint32 {
	if g, ok := file.FlatFindChild(prop, "getter"); ok {
		return g
	}
	for next := file.FlatNextSib(prop); next != 0; next = file.FlatNextSib(next) {
		switch file.FlatType(next) {
		case "getter":
			return next
		case "setter", "line_comment", "multiline_comment":
			continue
		}
		return 0
	}
	return 0
}

// importedUsage classifies a name bound by an explicit import to fqn.
func (c *composeLintsUsageClassifier) importedUsage(fqn string) composeLintsUsage {
	dot := strings.LastIndex(fqn, ".")
	if dot < 0 {
		return composeLintsUsageUnknown
	}
	pkg, name := fqn[:dot], fqn[dot+1:]
	if composeLintsScannedPackages[pkg] {
		return composeLintsUsageOfKind(composeLintsLibraryComposableFunctions[fqn])
	}
	if _, ok := c.index.SymbolByFQN(fqn); ok {
		u, _ := c.projectUsage(name)
		return u
	}
	return composeLintsUsageUnknown
}

// nameUsage classifies an unqualified function or property name used in file.
func (c *composeLintsUsageClassifier) nameUsage(file *scanner.File, name string) composeLintsUsage {
	facts := fileFactsCache().Imports(file)
	if facts != nil {
		if fqn, ok := facts.Aliases[name]; ok {
			return c.importedUsage(fqn)
		}
	}
	if u, ok := c.projectUsage(name); ok {
		return u
	}
	if facts != nil {
		for wildcard := range facts.Wildcards {
			pkg := strings.TrimSuffix(wildcard, ".*")
			if composeLintsScannedPackages[pkg] {
				if kind := composeLintsLibraryComposableFunctions[pkg+"."+name]; kind != 0 {
					return composeLintsUsageOfKind(kind)
				}
				continue
			}
			return composeLintsUsageUnknown // an unscanned package may declare it
		}
	}
	return composeLintsUsageNone // a default import (kotlin stdlib)
}

// memberUsage classifies a selector name of a qualified call or read. A receiver that is an imported library class
// or object is looked up in that class's members; otherwise krit does not know the receiver type and classifies
// the name across the project and the libraries.
func (c *composeLintsUsageClassifier) memberUsage(file *scanner.File, receiver uint32, name string) composeLintsUsage {
	facts := fileFactsCache().Imports(file)
	if facts != nil {
		if fqn, ok := facts.Aliases[name]; ok {
			return c.importedUsage(fqn) // an imported extension
		}
		if receiver != 0 && file.FlatType(receiver) == "simple_identifier" {
			if cls, ok := facts.Aliases[file.FlatNodeText(receiver)]; ok && acrFirstRuneIsUpper(file.FlatNodeText(receiver)) {
				if dot := strings.LastIndex(cls, "."); dot > 0 && composeLintsScannedPackages[cls[:dot]] {
					return composeLintsUsageOfKind(composeLintsLibraryComposableMembersByClass[cls+"."+name])
				}
			}
		}
	}
	result := composeLintsUsageNone
	if u, ok := c.projectUsage(name); ok {
		result = u
	}
	if kind := composeLintsLibraryComposableMembers[name]; kind != 0 {
		result = composeLintsMergeUsage(result, composeLintsUsageOfKind(kind))
	}
	if result == composeLintsUsageNone && facts != nil {
		for wildcard := range facts.Wildcards {
			if pkg := strings.TrimSuffix(wildcard, ".*"); !composeLintsScannedPackages[pkg] && !c.projectPackage(pkg) {
				return composeLintsUsageUnknown
			}
		}
	}
	return result
}

func (c *composeLintsUsageClassifier) projectPackage(pkg string) bool {
	for _, f := range c.files {
		if filePackageName(f) == pkg {
			return true
		}
	}
	return false
}

// composeLintsScope is the function (or getter or lambda) being classified: its parameters and locals.
type composeLintsScope struct {
	params map[string]uint32
	root   uint32
}

// usageOf classifies everything under root: calls, qualified reads and name reads.
func (c *composeLintsUsageClassifier) usageOf(file *scanner.File, scope composeLintsScope, roots ...uint32) composeLintsUsage {
	result := composeLintsUsageNone
	for _, root := range roots {
		if root == 0 {
			continue
		}
		file.FlatWalkAllNodes(root, func(n uint32) {
			if result == composeLintsUsageOther {
				return
			}
			switch file.FlatType(n) {
			case "call_expression":
				result = composeLintsMergeUsage(result, c.callUsage(file, scope, n))
			case "navigation_expression":
				if parent, ok := file.FlatParent(n); ok && file.FlatType(parent) == "call_expression" && file.FlatFirstChild(parent) == n {
					return // the selector is the callee; classified as the call
				}
				result = composeLintsMergeUsage(result, c.readUsage(file, scope, n))
			case "simple_identifier", "interpolated_identifier":
				result = composeLintsMergeUsage(result, c.nameReadUsage(file, scope, n))
			}
		})
	}
	return result
}

func (c *composeLintsUsageClassifier) callUsage(file *scanner.File, scope composeLintsScope, call uint32) composeLintsUsage {
	callee := file.FlatFirstChild(call)
	if file.FlatType(callee) == "call_expression" {
		return composeLintsUsageNone // f<T>(x) { }: the inner call is classified on its own
	}
	switch file.FlatType(callee) {
	case "simple_identifier":
		name := file.FlatNodeText(callee)
		if decl := composeLintsLocalDecl(file, scope, callee, name); decl != 0 {
			return composeLintsInvokedValueUsage(file, decl)
		}
		return c.nameUsage(file, name)
	case "navigation_expression":
		suffix, ok := file.FlatFindChild(callee, "navigation_suffix")
		if !ok {
			return composeLintsUsageUnknown
		}
		name := file.FlatChildTextOrEmpty(suffix, "simple_identifier")
		if name == "" {
			return composeLintsUsageUnknown
		}
		if root, selectors := composeLintsChain(file, callee); root != 0 && file.FlatType(root) == "simple_identifier" &&
			file.FlatNodeText(root) == "androidx" {
			return c.importedUsage("androidx." + strings.Join(selectors, "."))
		}
		// receiver.slot(x) invokes a function-typed parameter with a receiver.
		if decl := composeLintsLocalDecl(file, scope, callee, name); decl != 0 && file.FlatType(decl) == "parameter" {
			return composeLintsInvokedValueUsage(file, decl)
		}
		return c.memberUsage(file, file.FlatFirstChild(callee), name)
	case "parenthesized_expression", "lambda_literal", "this_expression":
		return composeLintsUsageUnknown
	}
	return composeLintsUsageUnknown
}

// readUsage classifies a qualified read `receiver.name` (not a call).
func (c *composeLintsUsageClassifier) readUsage(file *scanner.File, scope composeLintsScope, nav uint32) composeLintsUsage {
	suffix, ok := file.FlatFindChild(nav, "navigation_suffix")
	if !ok {
		return composeLintsUsageUnknown
	}
	name := file.FlatChildTextOrEmpty(suffix, "simple_identifier")
	receiver := file.FlatFirstChild(nav)
	switch name {
	case "":
		return composeLintsUsageNone // `::class` and the like
	case "current":
		// CompositionLocal.current is read-only; Locals are named Local* by convention (CompositionLocalNaming).
		if file.FlatType(receiver) == "simple_identifier" && strings.HasPrefix(file.FlatNodeText(receiver), "Local") {
			return composeLintsUsageReadOnly
		}
	case "value":
		return composeLintsValueReadUsage(file, scope, receiver)
	}
	if root, selectors := composeLintsChain(file, nav); root != 0 && file.FlatType(root) == "simple_identifier" &&
		file.FlatNodeText(root) == "androidx" {
		return c.importedUsage("androidx." + strings.Join(selectors, "."))
	}
	return c.memberUsage(file, receiver, name)
}

// nameReadUsage classifies a bare name read: a delegated local, or a top-level property with a composable getter.
func (c *composeLintsUsageClassifier) nameReadUsage(file *scanner.File, scope composeLintsScope, id uint32) composeLintsUsage {
	parent, ok := file.FlatParent(id)
	if !ok {
		return composeLintsUsageNone
	}
	switch file.FlatType(parent) {
	case "navigation_suffix", "variable_declaration", "parameter", "class_parameter", "function_declaration",
		"lambda_parameters", "import_header", "package_header", "identifier", "value_parameter", "type_alias",
		"class_declaration", "object_declaration", "label", "annotation", "user_type", "enum_entry",
		"directly_assignable_expression", "binding_pattern_kind", "multi_variable_declaration", "catch_block",
		"for_statement", "when_subject":
		return composeLintsUsageNone
	case "call_expression":
		if file.FlatFirstChild(parent) == id {
			return composeLintsUsageNone // a callee, classified as the call
		}
	case "navigation_expression":
		// A receiver root (MaterialTheme in MaterialTheme.colorScheme) reads an object or a value; fall through.
	case "value_argument":
		if next := file.FlatNextSib(id); next != 0 && file.FlatType(next) == "=" {
			return composeLintsUsageNone // a named-argument label
		}
	}
	name := strings.TrimPrefix(file.FlatNodeText(id), "$")
	if decl := composeLintsLocalDecl(file, scope, id, name); decl != 0 {
		return composeLintsDelegatedReadUsage(file, scope, decl)
	}
	if !acrFirstRuneIsUpper(name) && !strings.Contains(name, "_") {
		// A lowercase name that is not a local: a property (top-level or member of an implicit receiver).
		if u, ok := c.projectUsage(name); ok {
			return u
		}
		facts := fileFactsCache().Imports(file)
		if facts != nil {
			if fqn, ok := facts.Aliases[name]; ok {
				return c.importedUsage(fqn)
			}
		}
		if kind := composeLintsLibraryComposableMembers[name]; kind != 0 {
			return composeLintsUsageOfKind(kind)
		}
		return composeLintsUsageNone
	}
	// A capitalized name read is an object, class or CompositionLocal reference (MaterialTheme in
	// MaterialTheme.colorScheme), not a call of the same-named composable function.
	return composeLintsUsageNone
}

// composeLintsLocalDecl returns the parameter, or the local property declared before node inside the scope, that
// name refers to, or 0.
func composeLintsLocalDecl(file *scanner.File, scope composeLintsScope, node uint32, name string) uint32 {
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal":
			// A lambda parameter (or the implicit `it`) is a local value; it does not read the composition.
			if params, ok := file.FlatFindChild(p, "lambda_parameters"); ok {
				bound := uint32(0)
				file.FlatWalkNodes(params, "simple_identifier", func(id uint32) {
					if bound == 0 && file.FlatNodeTextEquals(id, name) {
						bound = id
					}
				})
				if bound != 0 {
					return bound
				}
			} else if name == "it" {
				return p
			}
		case "for_statement", "catch_block":
			found := uint32(0)
			for child := file.FlatFirstChild(p); child != 0; child = file.FlatNextSib(child) {
				switch file.FlatType(child) {
				case "variable_declaration", "simple_identifier":
					if file.FlatNodeTextEquals(child, name) || file.FlatChildTextOrEmpty(child, "simple_identifier") == name {
						found = child
					}
				}
			}
			if found != 0 {
				return found
			}
		}
		if file.FlatType(p) == "statements" {
			for child := file.FlatFirstChild(p); child != 0 && file.FlatStartByte(child) < file.FlatStartByte(node); child = file.FlatNextSib(child) {
				if file.FlatType(child) != "property_declaration" {
					continue
				}
				if v, ok := file.FlatFindChild(child, "variable_declaration"); ok && file.FlatChildTextOrEmpty(v, "simple_identifier") == name {
					return child
				}
			}
		}
		if p == scope.root && file.FlatType(p) == "function_declaration" {
			break
		}
		if file.FlatType(p) == "function_declaration" {
			// A lambda or getter inside a function sees that function's parameters (slots included).
			for _, param := range composeLintsParams(file, p) {
				if strings.Trim(file.FlatChildTextOrEmpty(param.node, "simple_identifier"), "`") == name {
					return param.node
				}
			}
		}
	}
	return scope.params[name]
}

// composeLintsInvokedValueUsage classifies invoking a parameter or local as a function.
func composeLintsInvokedValueUsage(file *scanner.File, decl uint32) composeLintsUsage {
	switch file.FlatType(decl) {
	case "parameter":
		if acrComposableFunctionType(file, decl) != 0 {
			return composeLintsUsageOther // invoking a composable lambda (compose-lints' invokesComposableLambda)
		}
		t := composeLintsDeclaredTypeAny(file, decl)
		if t != 0 && (file.FlatType(t) == "function_type" || file.FlatType(t) == "nullable_type") {
			return composeLintsUsageNone
		}
		return composeLintsUsageUnknown
	case "property_declaration":
		init := composeLintsUnwrapParens(file, composeLintsPropertyInitializer(file, decl))
		switch {
		case init != 0 && file.FlatType(init) == "lambda_literal":
			return composeLintsUsageNone
		case init != 0 && file.FlatType(init) == "prefix_expression" && composeLintsIsComposableLambda(file, init):
			return composeLintsUsageOther
		}
	}
	return composeLintsUsageUnknown
}

// composeLintsDelegatedReadUsage classifies reading a local or parameter by name: a `by` delegate over a State
// uses the composition (compose-lints' isDelegatedStateRead), `by lazy` does not.
func composeLintsDelegatedReadUsage(file *scanner.File, scope composeLintsScope, decl uint32) composeLintsUsage {
	if file.FlatType(decl) != "property_declaration" {
		return composeLintsUsageNone
	}
	delegate, ok := file.FlatFindChild(decl, "property_delegate")
	if !ok {
		return composeLintsUsageNone
	}
	expr := composeLintsUnwrapParens(file, flatLastNamedChild(file, delegate))
	switch file.FlatType(expr) {
	case "call_expression":
		if name := composeLintsCalleeName(file, expr); name == "lazy" {
			return composeLintsUsageNone
		} else if composeLintsStateFactories[name] {
			return composeLintsUsageOther
		}
		return composeLintsUsageUnknown // classified as a call too; a non-composable call may still return a State
	case "simple_identifier":
		return composeLintsValueReadUsage(file, scope, expr)
	}
	return composeLintsUsageUnknown
}

// composeLintsValueReadUsage classifies `receiver.value` or delegating to receiver: OTHER for a Compose State,
// NONE for a provably different type, unknown otherwise.
func composeLintsValueReadUsage(file *scanner.File, scope composeLintsScope, receiver uint32) composeLintsUsage {
	if file.FlatType(receiver) != "simple_identifier" {
		return composeLintsUsageUnknown
	}
	decl := composeLintsLocalDecl(file, scope, receiver, file.FlatNodeText(receiver))
	if decl == 0 {
		return composeLintsUsageUnknown
	}
	var t uint32
	if file.FlatType(decl) == "parameter" {
		t = composeLintsDeclaredType(file, decl)
	} else if v, ok := file.FlatFindChild(decl, "variable_declaration"); ok {
		t = composeLintsDeclaredType(file, v)
		if t == 0 {
			init := composeLintsUnwrapParens(file, composeLintsPropertyInitializer(file, decl))
			if init != 0 && file.FlatType(init) == "call_expression" {
				name := composeLintsCalleeName(file, init)
				if composeLintsStateFactories[name] || strings.HasPrefix(name, "remember") || strings.HasPrefix(name, "collectAs") {
					return composeLintsUsageOther
				}
			}
		}
	}
	name := composeLintsTypeSimpleName(file, t)
	switch {
	case name == "":
		return composeLintsUsageUnknown
	case composeLintsStateTypes[name]:
		return composeLintsUsageOther
	case name == "StateFlow" || name == "MutableStateFlow" || name == "LiveData" || name == "MutableLiveData" ||
		name == "Result" || name == "Lazy":
		return composeLintsUsageNone
	}
	return composeLintsUsageUnknown
}

// composeLintsIsComposableLambda reports whether a prefix_expression is `@Composable { ... }`.
func composeLintsIsComposableLambda(file *scanner.File, prefix uint32) bool {
	annotated := false
	for child := file.FlatFirstChild(prefix); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "annotation":
			if acrAnnotationIsComposable(file, child) {
				annotated = true
			}
		case "lambda_literal":
			return annotated
		}
	}
	return false
}

// composeLintsHasSlotParameter reports whether a function takes a @Composable function-typed parameter.
func composeLintsHasSlotParameter(file *scanner.File, fn uint32) bool {
	for _, p := range composeLintsParams(file, fn) {
		if acrComposableFunctionType(file, p.node) != 0 {
			return true
		}
	}
	return false
}

// composeLintsIsContractDeclaration mirrors compose-lints' isContractDeclaration: a non-top-level member of an
// interface, or one marked override, open, abstract, expect, actual or external.
func composeLintsIsContractDeclaration(file *scanner.File, decl uint32) bool {
	if parent, ok := file.FlatParent(decl); ok && file.FlatType(parent) == "source_file" {
		return false
	}
	if composeLintsInInterface(file, decl) {
		return true
	}
	for _, m := range []string{"override", "open", "abstract", "expect", "actual", "external"} {
		if acrFunctionHasModifier(file, decl, m) {
			return true
		}
	}
	return false
}

// composeLintsComposableTarget is one declaration both rules judge.
type composeLintsComposableTarget struct {
	file       *scanner.File
	decl       uint32 // function_declaration, getter or prefix_expression (annotated lambda)
	annotation uint32
	usage      composeLintsUsage
	readOnly   bool // already @ReadOnlyComposable
	lambda     bool
}

// composeLintsComposableTargets classifies the project's composable functions, getters and annotated lambdas.
func composeLintsComposableTargets(index *scanner.CodeIndex, visit func(composeLintsComposableTarget)) {
	var c *composeLintsUsageClassifier
	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin ||
			!bytes.Contains(file.Content, []byte("Composable")) {
			continue
		}
		classifier := func() *composeLintsUsageClassifier {
			if c == nil {
				c = newComposeLintsUsageClassifier(index)
			}
			return c
		}
		file.FlatWalkAllNodes(0, func(n uint32) {
			switch file.FlatType(n) {
			case "function_declaration":
				annotation := composeLintsComposableAnnotation(file, n)
				if annotation == 0 || acrIsLocalDeclaration(file, n) || composeLintsIsContractDeclaration(file, n) ||
					composeLintsHasSlotParameter(file, n) {
					return
				}
				body, _ := file.FlatFindChild(n, "function_body")
				if body == 0 {
					return
				}
				scope := composeLintsScope{params: map[string]uint32{}, root: n}
				roots := []uint32{body}
				for _, p := range composeLintsParams(file, n) {
					scope.params[strings.Trim(file.FlatChildTextOrEmpty(p.node, "simple_identifier"), "`")] = p.node
					if p.def != 0 {
						roots = append(roots, p.def)
					}
				}
				visit(composeLintsComposableTarget{file: file, decl: n, annotation: annotation,
					usage: classifier().usageOf(file, scope, roots...), readOnly: composeLintsHasAnnotationNamed(file, n, "ReadOnlyComposable")})
			case "getter":
				var annotation uint32
				composeLintsEachAnnotation(file, n, func(a uint32) {
					if annotation == 0 && acrAnnotationIsComposable(file, a) {
						annotation = a
					}
				})
				body, _ := file.FlatFindChild(n, "function_body")
				if annotation == 0 || body == 0 {
					return
				}
				visit(composeLintsComposableTarget{file: file, decl: n, annotation: annotation,
					usage:    classifier().usageOf(file, composeLintsScope{params: map[string]uint32{}, root: n}, body),
					readOnly: composeLintsHasAnnotationNamed(file, n, "ReadOnlyComposable")})
			case "prefix_expression":
				if !composeLintsIsComposableLambda(file, n) {
					return
				}
				lambda, _ := file.FlatFindChild(n, "lambda_literal")
				var annotation uint32
				for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
					if file.FlatType(child) == "annotation" && acrAnnotationIsComposable(file, child) {
						annotation = child
					}
				}
				visit(composeLintsComposableTarget{file: file, decl: n, annotation: annotation, lambda: true,
					usage: classifier().usageOf(file, composeLintsScope{params: map[string]uint32{}, root: lambda}, lambda)})
			}
		})
	}
}

func (r *ComposeRedundantComposableRule) check(ctx *api.Context) {
	if ctx.CodeIndex == nil {
		return
	}
	composeLintsComposableTargets(ctx.CodeIndex, func(t composeLintsComposableTarget) {
		if t.usage != composeLintsUsageNone || r.ignored(t) {
			return
		}
		ctx.Emit(r.Finding(t.file, t.file.FlatRow(t.annotation)+1, t.file.FlatCol(t.annotation)+1, composeRedundantComposableMessage))
	})
}

// ignored applies the ignore-annotated option.
func (r *ComposeRedundantComposableRule) ignored(t composeLintsComposableTarget) bool {
	if len(r.IgnoreAnnotated) == 0 || t.lambda {
		return false
	}
	found := false
	composeLintsEachAnnotation(t.file, t.decl, func(a uint32) {
		text := composeLintsAnnotationTypeText(t.file, a)
		fqn := text
		if facts := fileFactsCache().Imports(t.file); facts != nil && facts.Aliases[text] != "" {
			fqn = facts.Aliases[text]
		}
		for _, ignored := range r.IgnoreAnnotated {
			if fqn == ignored {
				found = true
			}
		}
	})
	return found
}

func (r *ComposeReadOnlyComposableRule) check(ctx *api.Context) {
	if ctx.CodeIndex == nil {
		return
	}
	composeLintsComposableTargets(ctx.CodeIndex, func(t composeLintsComposableTarget) {
		if t.lambda || t.usage != composeLintsUsageReadOnly || t.readOnly {
			return
		}
		ctx.Emit(r.Finding(t.file, t.file.FlatRow(t.annotation)+1, t.file.FlatCol(t.annotation)+1, composeReadOnlyComposableMessage))
	})
}
