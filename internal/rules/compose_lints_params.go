package rules

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Messages of compose-lint-checks 1.6.1's parameter-type and ViewModel detectors, as Lint reports them.
const (
	composeMutableParametersMessage = "Using mutable objects as state in Compose will cause your users to see " +
		"incorrect or stale data in your app. Mutable objects that are not observable, such as ArrayList<T> or a " +
		"mutable data class, cannot be observed by Compose to trigger recomposition when they change. See " +
		"https://slackhq.github.io/compose-lints/rules/#do-not-use-inherently-mutable-types-as-parameters for more " +
		"information."
	composeUnstableCollectionsMessage = "The Compose Compiler cannot infer the stability of a parameter if a %s is " +
		"used in it, even if the item type is stable. You should use Kotlinx Immutable Collections instead: `%s: " +
		"Immutable%s` or create an `@Immutable` wrapper for this class: `@Immutable data class %s%s(val items: %s)` " +
		"See https://slackhq.github.io/compose-lints/rules/#avoid-using-unstable-collections for more information."
	composeViewModelForwardingMessage = "Forwarding a ViewModel through multiple @Composable functions should be " +
		"avoided. Consider using state hoisting. See https://slackhq.github.io/compose-lints/rules/#hoist-all-the-things " +
		"for more information."
	composeViewModelInjectionMessage = "Implicit dependencies of composables should be made explicit. Usages of %s to " +
		"acquire a ViewModel should be done in composable default parameters, so that it is more testable and " +
		"flexible. See https://slackhq.github.io/compose-lints/rules/#viewmodels for more information."
)

// composeLintsMutableTypeNames are the simple class names compose-lints treats as mutable
// (KnownMutableCommonTypesSimpleNames), plus MutableCollection, which it matches by its written form.
var composeLintsMutableTypeNames = map[string]bool{
	"MutableSet": true, "MutableScatterSet": true, "ArraySet": true, "HashSet": true, "LinkedHashSet": true,
	"TreeSet": true, "CopyOnWriteArraySet": true, "ConcurrentSkipListSet": true, "MutableList": true,
	"MutableObjectList": true, "ArrayList": true, "LinkedList": true, "Vector": true, "Stack": true,
	"CopyOnWriteArrayList": true, "SparseArray": true, "SparseArrayCompat": true, "LongSparseArray": true,
	"SparseBooleanArray": true, "SparseIntArray": true, "MutableMap": true, "MutableScatterMap": true,
	"MutableIntObjectMap": true, "MutableObjectIntMap": true, "MutableLongObjectMap": true,
	"MutableObjectLongMap": true, "HashMap": true, "LinkedHashMap": true, "TreeMap": true, "Hashtable": true,
	"ConcurrentHashMap": true, "ConcurrentSkipListMap": true, "IdentityHashMap": true, "WeakHashMap": true,
	"ArrayDeque": true, "PriorityQueue": true, "MutableState": true, "MutableStateFlow": true,
	"MutableSharedFlow": true, "PublishSubject": true, "BehaviorSubject": true, "ReplaySubject": true,
	"PublishRelay": true, "BehaviorRelay": true, "ReplayRelay": true, "MutableCollection": true,
}

// composeLintsUnstableCollectionNames are the Kotlin collection types that map to java.util.Collection, Set, List
// or Map, which compose-lints reports as unstable.
var composeLintsUnstableCollectionNames = map[string]bool{
	"List": true, "Set": true, "Map": true, "Collection": true,
	"MutableList": true, "MutableSet": true, "MutableMap": true, "MutableCollection": true,
}

// composeLintsKnownViewModelFactories are compose-lints' KnownViewModelFactories.
var composeLintsKnownViewModelFactories = []string{"viewModel", "weaverViewModel", "hiltViewModel",
	"injectedViewModel", "mavericksViewModel"}

// composeLintsRestartableEffects take a ViewModel as a key legitimately.
var composeLintsRestartableEffects = map[string]bool{"LaunchedEffect": true, "produceState": true,
	"produceRetainedState": true, "DisposableEffect": true}

// ComposeMutableParametersRule ports compose-lints' ComposeMutableParameters: a @Composable parameter (or
// extension receiver) of an inherently mutable type such as MutableList, ArrayList or MutableState.
type ComposeMutableParametersRule struct {
	FlatDispatchBase
	BaseRule
}

// ComposeUnstableCollectionsRule ports compose-lints' ComposeUnstableCollections: a @Composable parameter typed as a
// Kotlin List, Set, Map or Collection, whose stability the Compose compiler cannot infer.
//
// It runs over the cross-file index so a type alias of a collection declared in another file expands.
type ComposeUnstableCollectionsRule struct {
	BaseRule
}

// ComposeViewModelForwardingRule ports compose-lints' ComposeViewModelForwarding: a ViewModel parameter passed whole
// to another composable. It runs over the cross-file index to tell composable callees from class constructors.
type ComposeViewModelForwardingRule struct {
	BaseRule
}

// ComposeViewModelInjectionRule ports compose-lints' ComposeViewModelInjection: a local `val x = viewModel()` (or
// another ViewModel factory) in a composable body instead of a default parameter.
type ComposeViewModelInjectionRule struct {
	FlatDispatchBase
	BaseRule
	// ViewModelFactories are extra factory function names (compose-lints `viewmodel-factories`).
	ViewModelFactories []string
}

// composeLintsTypeSimpleName returns a type's simple name without type arguments or nullability: `Map` for
// `kotlin.collections.Map<K, V>?`.
func composeLintsTypeSimpleName(file *scanner.File, t uint32) string {
	if t == 0 {
		return ""
	}
	if file.FlatType(t) == "nullable_type" {
		t, _ = file.FlatFindChild(t, "user_type")
		if t == 0 {
			return ""
		}
	}
	if file.FlatType(t) != "user_type" {
		return ""
	}
	text := composeLintsTypeText(file, t)
	return text[strings.LastIndex(text, ".")+1:]
}

func (r *ComposeMutableParametersRule) check(ctx *api.Context) {
	file, fn := ctx.File, ctx.Idx
	if !acrFunctionIsComposable(file, fn) || acrIsLocalDeclaration(file, fn) {
		return
	}
	// UAST lists the extension receiver as the first parameter.
	if recv := composeLintsReceiverType(file, fn); recv != 0 && composeLintsMutableTypeNames[composeLintsTypeSimpleName(file, recv)] {
		ctx.EmitAtNode(recv, composeMutableParametersMessage)
	}
	for _, p := range composeLintsParams(file, fn) {
		t := composeLintsDeclaredType(file, p.node)
		if t == 0 {
			continue
		}
		// A vararg parameter's type is an array, so only compose-lints' text match on the written type reports it.
		if p.isVararg(file) {
			if composeLintsMutableKotlinCollectionText.MatchString(file.FlatNodeText(t)) {
				ctx.EmitAtNode(t, composeMutableParametersMessage)
			}
			continue
		}
		if composeLintsMutableTypeNames[composeLintsTypeSimpleName(file, t)] {
			ctx.EmitAtNode(t, composeMutableParametersMessage)
		}
	}
}

// composeLintsMutableKotlinCollectionText is compose-lints' KnownMutableKotlinCollections, which must match the
// whole written type.
var composeLintsMutableKotlinCollectionText = regexp.MustCompile(
	`^(?:MutableMap(\s)?<.*,(\s)?.*>|MutableSet(\s)?<.*>|MutableList(\s)?<.*>|MutableCollection(\s)?<.*>)\??$`)

// composeLintsDiamond matches the type arguments compose-lints strips for the wrapper class name.
var composeLintsDiamond = regexp.MustCompile(`<.*>\??`)

func (r *ComposeUnstableCollectionsRule) check(ctx *api.Context) {
	if ctx.CodeIndex == nil {
		return
	}
	aliases := newComposeLintsTypeAliases(ctx.CodeIndex.Files)
	for _, file := range ctx.CodeIndex.Files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin ||
			!bytes.Contains(file.Content, []byte("Composable")) {
			continue
		}
		file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
			if !acrFunctionIsComposable(file, fn) || acrIsLocalDeclaration(file, fn) {
				return
			}
			emit := func(at uint32, msg string) {
				ctx.Emit(r.Finding(file, file.FlatRow(at)+1, file.FlatCol(at)+1, msg))
			}
			// UAST lists the extension receiver first, named $this$<function>, with no source text for its type,
			// so compose-lints falls back to "List/Set/Map" in the message.
			if recv := composeLintsReceiverType(file, fn); recv != 0 && aliases.isKotlinCollection(file, recv, 0) {
				name := "$this$" + acrDeclarationName(file, fn)
				emit(recv, fmt.Sprintf(composeUnstableCollectionsMessage, "List/Set/Map", name, "List/Set/Map",
					name, "List/Set/Map", "List/Set/Map"))
			}
			for _, p := range composeLintsParams(file, fn) {
				// A vararg parameter's type is an array, which compose-lints does not report.
				t := composeLintsDeclaredType(file, p.node)
				if t == 0 || p.isVararg(file) || !aliases.isKotlinCollection(file, t, 0) {
					continue
				}
				typeText := strings.Join(strings.Fields(file.FlatNodeText(t)), " ")
				name := strings.Trim(file.FlatChildTextOrEmpty(p.node, "simple_identifier"), "`")
				raw := composeLintsDiamond.ReplaceAllString(typeText, "")
				emit(t, fmt.Sprintf(composeUnstableCollectionsMessage, typeText, name, typeText,
					composeLintsCapitalize(name), raw, typeText))
			}
		})
	}
}

// isKotlinCollection is composeLintsIsKotlinCollection after expanding project type aliases.
func (a composeLintsTypeAliases) isKotlinCollection(file *scanner.File, t uint32, depth int) bool {
	if composeLintsIsKotlinCollection(file, t) {
		return true
	}
	if depth > 4 {
		return false
	}
	ut := t
	if file.FlatType(ut) == "nullable_type" {
		ut, _ = file.FlatFindChild(ut, "user_type")
	}
	if ut == 0 || file.FlatType(ut) != "user_type" {
		return false
	}
	alias, ok := a[a.resolve(file, composeLintsTypeText(file, ut))]
	if !ok {
		return false
	}
	target := composeLintsTypeAliasTarget(alias.file, alias.decl)
	return target != 0 && a.isKotlinCollection(alias.file, target, depth+1)
}

// composeLintsIsKotlinCollection reports whether a declared type is kotlin.collections' (or java.util's) List,
// Set, Map or Collection, or a mutable variant: the name binds to the default import unless an explicit import or
// a same-file declaration takes it.
func composeLintsIsKotlinCollection(file *scanner.File, t uint32) bool {
	if file.FlatType(t) == "nullable_type" {
		t, _ = file.FlatFindChild(t, "user_type")
	}
	if t == 0 || file.FlatType(t) != "user_type" {
		return false
	}
	text := composeLintsTypeText(file, t)
	if dot := strings.LastIndex(text, "."); dot >= 0 {
		prefix, last := text[:dot], text[dot+1:]
		return composeLintsUnstableCollectionNames[last] && (prefix == "kotlin.collections" ||
			prefix == "java.util" && !strings.HasPrefix(last, "Mutable"))
	}
	if !composeLintsUnstableCollectionNames[text] {
		return false
	}
	if facts := fileFactsCache().Imports(file); facts != nil {
		if fqn, ok := facts.Aliases[text]; ok {
			return fqn == "kotlin.collections."+text || fqn == "java.util."+text
		}
	}
	return !acrFileDeclaresTopLevel(file, text)
}

func composeLintsCapitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToTitle(r)) + s[size:]
}

func (r *ComposeViewModelInjectionRule) check(ctx *api.Context) {
	file, fn := ctx.File, ctx.Idx
	if !acrFunctionIsComposable(file, fn) || acrIsLocalDeclaration(file, fn) ||
		acrFunctionHasModifier(file, fn, "override") || composeLintsInInterface(file, fn) {
		return
	}
	body, _ := file.FlatFindChild(fn, "function_body")
	if body == 0 {
		return
	}
	if first := file.FlatFirstChild(body); first == 0 || file.FlatType(first) != "{" {
		return
	}
	// compose-lints searches the whole body, lambdas and nested declarations included, for properties whose
	// initializer (through parentheses) is a factory call.
	file.FlatWalkNodes(body, "property_declaration", func(prop uint32) {
		init := composeLintsUnwrapParens(file, composeLintsPropertyInitializer(file, prop))
		if init == 0 || file.FlatType(init) != "call_expression" {
			return
		}
		callee := composeLintsUnwrapParens(file, file.FlatFirstChild(init))
		for callee != 0 && file.FlatType(callee) == "call_expression" {
			callee = composeLintsUnwrapParens(file, file.FlatFirstChild(callee)) // f<T>(args) { } parses as nested calls
		}
		if callee == 0 || file.FlatType(callee) != "simple_identifier" {
			return
		}
		if name := file.FlatNodeText(callee); r.isFactory(name) {
			ctx.EmitAtNode(prop, fmt.Sprintf(composeViewModelInjectionMessage, name))
		}
	})
}

func (r *ComposeViewModelInjectionRule) isFactory(name string) bool {
	for _, f := range composeLintsKnownViewModelFactories {
		if f == name {
			return true
		}
	}
	for _, f := range r.ViewModelFactories {
		if f == name {
			return true
		}
	}
	return false
}

// composeLintsCallees answers whether a capitalized callee name is a composable, looking the name up in the
// cross-file index's declarations on demand (memoized per run).
type composeLintsCallees struct {
	index *scanner.CodeIndex
	files map[string]*scanner.File
	memo  map[string]bool
	// declared records, for each name in memo, whether the project declares it.
	declared map[string]bool
}

func newComposeLintsCallees(index *scanner.CodeIndex) *composeLintsCallees {
	files := make(map[string]*scanner.File, len(index.Files))
	for _, f := range index.Files {
		if f != nil {
			files[f.Path] = f
		}
	}
	return &composeLintsCallees{index: index, files: files, memo: map[string]bool{}, declared: map[string]bool{}}
}

// likelyComposable mirrors compose-lints' isLikelyComposableCall: a capitalized callee that, when it resolves, is
// @Composable. A name imported from a JDK, Kotlin or Android framework package resolves to a class there (no
// composables live in them). A project function name counts when some declaration of it is composable; a project
// class or object (a constructor call) or only non-composable functions do not. A name the project does not
// declare is a default-imported Kotlin/JVM class from composeLintsDefaultImportedClasses, which resolves in Lint, or
// else falls back to the capitalization heuristic.
func (c *composeLintsCallees) likelyComposable(file *scanner.File, name string) bool {
	if !acrFirstRuneIsUpper(name) {
		return false
	}
	fqn := acrImportedFQN(file, name)
	if fqn != "" && composeLintsPlatformPackage(fqn) {
		return false
	}
	if !c.projectComposable(name) {
		return false
	}
	return fqn != "" || !composeLintsDefaultImportedClasses[name] || !acrDefaultImported(file, name) ||
		c.declared[name]
}

// composeLintsPlatformPackage reports whether an FQN lives in a JDK, Kotlin or Android framework package.
func composeLintsPlatformPackage(fqn string) bool {
	for _, prefix := range []string{"java.", "javax.", "kotlin.", "android."} {
		if strings.HasPrefix(fqn, prefix) {
			return true
		}
	}
	return false
}

// composeLintsDefaultImportedClasses are classes Kotlin imports by default (kotlin.*, kotlin.collections.*,
// kotlin.text.*, java.lang.*) that are commonly constructed: with a classpath Lint resolves them, so a call to one
// is not a composable.
var composeLintsDefaultImportedClasses = map[string]bool{
	"Pair": true, "Triple": true, "Regex": true, "StringBuilder": true, "ArrayList": true, "HashMap": true,
	"HashSet": true, "LinkedHashMap": true, "LinkedHashSet": true, "ArrayDeque": true, "Thread": true,
	"Any": true, "Throwable": true, "Error": true, "Exception": true, "RuntimeException": true,
	"IllegalStateException": true, "IllegalArgumentException": true, "UnsupportedOperationException": true,
	"NullPointerException": true, "IndexOutOfBoundsException": true, "NoSuchElementException": true,
	"ArithmeticException": true, "ClassCastException": true, "NumberFormatException": true, "AssertionError": true,
}

// projectComposable reports whether the project's declarations of name leave it a possible composable: some
// declaration is a composable function, or nothing in the project declares it.
func (c *composeLintsCallees) projectComposable(name string) bool {
	if v, ok := c.memo[name]; ok {
		return v
	}
	declared, composable := false, false
	for _, sym := range c.index.SymbolsNamed(name) {
		switch sym.Kind {
		case "function":
			declared = true
			if f := c.files[sym.File]; f != nil && composeLintsDeclarationIsComposable(f, sym) {
				composable = true
			}
		case "class", "object", "interface":
			declared = true
		}
	}
	result := composable || !declared
	c.memo[name] = result
	c.declared[name] = declared
	return result
}

// composeLintsDeclarationIsComposable reports whether the function declaration a symbol names is annotated
// @Composable (read from its modifiers, so a comment or string that says "Composable" does not count).
func composeLintsDeclarationIsComposable(file *scanner.File, sym scanner.Symbol) bool {
	if file.FlatTree == nil || sym.StartByte < 0 || sym.EndByte > len(file.Content) || sym.StartByte >= sym.EndByte {
		return false
	}
	node, ok := file.FlatNamedDescendantForByteRange(uint32(sym.StartByte), uint32(sym.EndByte))
	for ok && file.FlatType(node) != "function_declaration" {
		node, ok = file.FlatParent(node)
	}
	return ok && acrFunctionIsComposable(file, node)
}

func (r *ComposeViewModelForwardingRule) check(ctx *api.Context) {
	if ctx.CodeIndex == nil {
		return
	}
	callees := newComposeLintsCallees(ctx.CodeIndex)
	for _, file := range ctx.CodeIndex.Files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin ||
			!bytes.Contains(file.Content, []byte("ViewModel")) || !bytes.Contains(file.Content, []byte("Composable")) {
			continue
		}
		file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
			if !acrFunctionIsComposable(file, fn) || acrIsLocalDeclaration(file, fn) ||
				acrFunctionHasModifier(file, fn, "override") || acrFunctionHasModifier(file, fn, "actual") ||
				composeLintsInInterface(file, fn) {
				return
			}
			body, _ := file.FlatFindChild(fn, "function_body")
			if body == 0 || file.FlatFirstChild(body) == 0 || file.FlatType(file.FlatFirstChild(body)) != "{" {
				return
			}
			var names []string
			for _, p := range composeLintsParams(file, fn) {
				if strings.HasSuffix(composeLintsTypeSimpleName(file, composeLintsDeclaredType(file, p.node)), "ViewModel") {
					names = append(names, strings.Trim(file.FlatChildTextOrEmpty(p.node, "simple_identifier"), "`"))
				}
			}
			if len(names) == 0 {
				return
			}
			params := map[string]bool{}
			for _, n := range names {
				params[n] = true
			}
			names = composeLintsViewModelFlow(file, body, names)
			reported := map[uint32]bool{}
			file.FlatWalkNodes(body, "value_argument", func(arg uint32) {
				call, at := composeLintsForwardedCall(file, arg, names, params)
				if call == 0 || reported[call] {
					return
				}
				name := composeLintsCalleeName(file, call)
				if composeLintsRestartableEffects[name] || !callees.likelyComposable(file, name) {
					return
				}
				reported[call] = true
				ctx.Emit(r.Finding(file, file.FlatRow(at)+1, file.FlatCol(at)+1, composeViewModelForwardingMessage))
			})
		})
	}
}

// composeLintsViewModelFlow extends the tracked names the way Lint's DataFlowAnalyzer does for compose-lints: a
// local initialized with a tracked name, or with a call that takes a tracked name as a whole argument
// (`val content = rememberContent(viewModel)`), is tracked too.
func composeLintsViewModelFlow(file *scanner.File, body uint32, names []string) []string {
	tracked := map[string]bool{}
	for _, n := range names {
		tracked[n] = true
	}
	isTracked := func(expr uint32) bool {
		return expr != 0 && file.FlatType(expr) == "simple_identifier" && tracked[file.FlatNodeText(expr)]
	}
	file.FlatWalkNodes(body, "property_declaration", func(prop uint32) {
		v, ok := file.FlatFindChild(prop, "variable_declaration")
		if !ok {
			return
		}
		name := file.FlatChildTextOrEmpty(v, "simple_identifier")
		init := composeLintsUnwrapParens(file, composeLintsPropertyInitializer(file, prop))
		switch {
		case name == "" || tracked[name] || init == 0:
			return
		case isTracked(init):
		case file.FlatType(init) == "call_expression":
			passes := false
			file.FlatWalkNodes(init, "value_argument", func(arg uint32) {
				if parentCall := composeLintsArgCall(file, arg); parentCall != init {
					return
				}
				expr := file.FlatFirstChild(arg)
				if expr != 0 && file.FlatType(expr) == "simple_identifier" {
					if eq := file.FlatNextSib(expr); eq != 0 && file.FlatType(eq) == "=" {
						expr = file.FlatNextSib(eq)
					}
				}
				if isTracked(expr) && file.FlatNextSib(expr) == 0 {
					passes = true
				}
			})
			if !passes {
				return
			}
		default:
			return
		}
		tracked[name] = true
		names = append(names, name)
	})
	return names
}

// composeLintsArgCall returns the call expression a value_argument belongs to, or 0.
func composeLintsArgCall(file *scanner.File, arg uint32) uint32 {
	if !composeLintsArgumentOfCall(file, arg) {
		return 0
	}
	args, _ := file.FlatParent(arg)
	suffix, _ := file.FlatParent(args)
	call, _ := file.FlatParent(suffix)
	return call
}

// composeLintsForwardedCall returns the call whose argument arg is exactly one of the tracked names (not a
// property of it, not parenthesized, and for a parameter not shadowed by a lambda parameter or local), and the
// node where Lint's
// location for that call starts (its callee name, after any receiver).
func composeLintsForwardedCall(file *scanner.File, arg uint32, names []string, params map[string]bool) (uint32, uint32) {
	expr := file.FlatFirstChild(arg)
	if expr != 0 && file.FlatType(expr) == "simple_identifier" {
		if eq := file.FlatNextSib(expr); eq != 0 && file.FlatType(eq) == "=" {
			expr = file.FlatNextSib(eq)
		}
	}
	if expr == 0 || file.FlatType(expr) != "simple_identifier" || file.FlatNextSib(expr) != 0 {
		return 0, 0
	}
	name := file.FlatNodeText(expr)
	found := false
	for _, n := range names {
		if n == name {
			found = true
		}
	}
	// A tracked local is its own declaration; only the parameters can be shadowed by a lambda parameter or local.
	if !found || !composeLintsArgumentOfCall(file, arg) || params[name] && composeLintsNameShadowed(file, expr, name) {
		return 0, 0
	}
	args, _ := file.FlatParent(arg)
	suffix, _ := file.FlatParent(args)
	call, _ := file.FlatParent(suffix)
	at := call
	if callee := file.FlatFirstChild(call); file.FlatType(callee) == "navigation_expression" {
		if s, ok := file.FlatFindChild(callee, "navigation_suffix"); ok {
			if id, ok := file.FlatFindChild(s, "simple_identifier"); ok {
				at = id
			}
		}
	}
	return call, at
}

// composeLintsNameShadowed reports whether a lambda parameter or an earlier local declaration between node and
// its enclosing function rebinds name.
func composeLintsNameShadowed(file *scanner.File, node uint32, name string) bool {
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal":
			if params, ok := file.FlatFindChild(p, "lambda_parameters"); ok {
				shadowed := false
				file.FlatWalkNodes(params, "simple_identifier", func(id uint32) {
					if file.FlatNodeTextEquals(id, name) {
						shadowed = true
					}
				})
				if shadowed {
					return true
				}
			}
		case "statements":
			for child := file.FlatFirstChild(p); child != 0 && file.FlatStartByte(child) < file.FlatStartByte(node); child = file.FlatNextSib(child) {
				if file.FlatType(child) != "property_declaration" {
					continue
				}
				if v, ok := file.FlatFindChild(child, "variable_declaration"); ok && file.FlatChildTextOrEmpty(v, "simple_identifier") == name {
					return true
				}
			}
		case "function_declaration":
			// A local function's own parameters rebind the name; the checked composable itself is not local.
			if !acrIsLocalDeclaration(file, p) {
				return false
			}
			for _, param := range composeLintsParams(file, p) {
				if strings.Trim(file.FlatChildTextOrEmpty(param.node, "simple_identifier"), "`") == name {
					return true
				}
			}
		}
	}
	return false
}
