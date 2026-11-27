package rules

import (
	"bytes"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Messages of compose-lint-checks 1.6.1's emitter detectors, as Lint reports them.
const (
	composeContentEmitterReturningValuesMessage = "Composable functions should either emit content into the " +
		"composition or return a value, but not both. If a composable should offer additional control surfaces to " +
		"its caller, those control surfaces or callbacks should be provided as parameters to the composable function " +
		"by the caller. See https://slackhq.github.io/compose-lints/rules/#do-not-emit-content-and-return-a-result " +
		"for more information."
	composeMultipleContentEmittersMessage = "Composable functions should only be emitting content into the " +
		"composition from one source at their top level. See " +
		"https://slackhq.github.io/compose-lints/rules/#do-not-emit-multiple-pieces-of-content for more information."
	composeNonRestartableComposableMessage = "This composable's body only delegates to another composable. Consider " +
		"marking it @NonRestartableComposable when its own restart and skip boundary is unlikely to be useful. The " +
		"annotation avoids generating that code, but also prevents the wrapper from being restarted or skipped " +
		"independently. See https://slackhq.github.io/compose-lints/rules/#avoid-unnecessary-restart-groups for more " +
		"information."
)

// ComposeContentEmitterReturningValuesRule ports compose-lints' ComposeContentEmitterReturningValues: a composable
// that emits content (directly, or through a same-file composable that does) and also returns a value.
type ComposeContentEmitterReturningValuesRule struct {
	FlatDispatchBase
	BaseRule
	// ContentEmitters are extra composables that emit content (compose-lints `content-emitters`).
	ContentEmitters []string
}

// ComposeMultipleContentEmittersRule ports compose-lints' ComposeMultipleContentEmitters: a composable whose top
// level emits from more than one source (counting same-file composables that emit, and emitting for loops).
type ComposeMultipleContentEmittersRule struct {
	FlatDispatchBase
	BaseRule
	// ContentEmitters are extra composables that emit content (compose-lints `content-emitters`).
	ContentEmitters []string
}

// ComposeNonRestartableComposableRule ports compose-lints' ComposeNonRestartableComposable: a Unit composable whose
// body is a single call forwarding its parameters to a non-inline, non-read-only Unit composable.
type ComposeNonRestartableComposableRule struct {
	BaseRule
}

// composeLintsEmitterSet decides whether a single call emits content (compose-lints' KtCallExpression.emitsContent).
type composeLintsEmitterSet []string

func (e composeLintsEmitterSet) emits(file *scanner.File, call uint32) bool {
	r := ComposeModifierMissingRule{ContentEmitters: e}
	return r.isEmitter(composeLintsCalleeName(file, call)) || composeLintsHasNamedArgument(file, call, "modifier")
}

// composeLintsFileComposable is a top-level or member composable of a file with a block body and no receiver.
type composeLintsFileComposable struct {
	fn    uint32
	name  string
	stmts []uint32
}

// composeLintsFileComposables lists the composables compose-lints' emitter detectors analyze: @Composable member
// and top-level functions with a block (or no) body and no extension receiver.
func composeLintsFileComposables(file *scanner.File) []composeLintsFileComposable {
	var out []composeLintsFileComposable
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		if !acrFunctionIsComposable(file, fn) || acrIsLocalDeclaration(file, fn) || composeLintsReceiverType(file, fn) != 0 {
			return
		}
		c := composeLintsFileComposable{fn: fn, name: acrDeclarationName(file, fn)}
		if body, ok := file.FlatFindChild(fn, "function_body"); ok {
			first := file.FlatFirstChild(body)
			if first == 0 || file.FlatType(first) != "{" {
				return // expression body
			}
			if stmts, ok := file.FlatFindChild(body, "statements"); ok {
				for s := file.FlatFirstChild(stmts); s != 0; s = file.FlatNextSib(s) {
					if file.FlatIsNamed(s) && !composeLintsIsComment(file, s) {
						c.stmts = append(c.stmts, composeLintsUnwrapParens(file, s))
					}
				}
			}
		}
		out = append(out, c)
	})
	return out
}

// composeLintsStatementCall returns the call a top-level statement is: the statement itself, or, when qualified is
// set, the selector call of `a.b()`. 0 otherwise.
func composeLintsStatementCall(file *scanner.File, stmt uint32, qualified bool) uint32 {
	if stmt == 0 || file.FlatType(stmt) != "call_expression" {
		return 0
	}
	callee := file.FlatFirstChild(stmt)
	if file.FlatType(callee) == "navigation_expression" && !qualified {
		return 0 // `a.b()` is a qualified expression in PSI, not a call statement
	}
	if file.FlatType(callee) == "navigation_expression" {
		if s, ok := file.FlatFindChild(callee, "navigation_suffix"); ok && strings.HasPrefix(file.FlatNodeText(s), "?.") {
			return 0
		}
	}
	return stmt
}

// composeLintsEmissionFixpoint computes compose-lints' per-composable emission counts: the direct count, then
// repeated passes counting top-level calls that emit or that call a same-file composable with a positive count,
// until nothing changes. It returns the direct counts and the final counts.
func composeLintsEmissionFixpoint(file *scanner.File, composables []composeLintsFileComposable, emitters composeLintsEmitterSet,
	qualified bool, extra func(c composeLintsFileComposable) int) (direct, final []int) {
	direct = make([]int, len(composables))
	for i, c := range composables {
		for _, s := range c.stmts {
			if call := composeLintsStatementCall(file, s, qualified); call != 0 && emitters.emits(file, call) {
				direct[i]++
			}
		}
		if extra != nil {
			direct[i] += extra(c)
		}
	}
	current := append([]int(nil), direct...)
	for pass := 0; pass < len(composables)+2; pass++ {
		byName := map[string]int{}
		for i, c := range composables {
			byName[c.name] = current[i] // compose-lints keys the mapping by name; the last one wins
		}
		next := make([]int, len(composables))
		for i, c := range composables {
			for _, s := range c.stmts {
				call := composeLintsStatementCall(file, s, qualified)
				if call == 0 {
					continue
				}
				if emitters.emits(file, call) {
					next[i]++
					continue
				}
				if v, ok := byName[composeLintsCalleeName(file, call)]; ok && v > 0 {
					next[i]++
				}
			}
		}
		same := true
		for i := range next {
			if next[i] != current[i] {
				same = false
			}
		}
		current = next
		if same {
			break
		}
	}
	return direct, current
}

func (r *ComposeMultipleContentEmittersRule) check(ctx *api.Context) {
	file := ctx.File
	composables := composeLintsFileComposables(file)
	if len(composables) == 0 {
		return
	}
	emitters := composeLintsEmitterSet(r.ContentEmitters)
	direct, final := composeLintsEmissionFixpoint(file, composables, emitters, false, func(c composeLintsFileComposable) int {
		if r.forLoopEmits(file, c.stmts, emitters) {
			return 2
		}
		return 0
	})
	for i, c := range composables {
		if direct[i] > 1 || final[i] > 1 {
			line, col := composeLintsDeclarationStart(file, c.fn)
			ctx.EmitAt(line, col, composeMultipleContentEmittersMessage)
		}
	}
}

// forLoopEmits mirrors compose-lints' forLoopHasUiEmitters: a top-level for loop whose block body has a top-level
// emitting call, or whose body is a single emitting call.
func (r *ComposeMultipleContentEmittersRule) forLoopEmits(file *scanner.File, stmts []uint32, emitters composeLintsEmitterSet) bool {
	for _, s := range stmts {
		if file.FlatType(s) != "for_statement" {
			continue
		}
		body, ok := file.FlatFindChild(s, "control_structure_body")
		if !ok {
			continue
		}
		if block, ok := file.FlatFindChild(body, "statements"); ok {
			for st := file.FlatFirstChild(block); st != 0; st = file.FlatNextSib(st) {
				if call := composeLintsStatementCall(file, composeLintsUnwrapParens(file, st), false); call != 0 && emitters.emits(file, call) {
					return true
				}
			}
			continue
		}
		// compose-lints' KtCallExpression.emitsContent is shallow: the call itself must emit, so `key(t) { Text(t) }`
		// does not count.
		if single := file.FlatNamedChild(body, 0); single != 0 && file.FlatType(single) == "call_expression" &&
			emitters.emits(file, single) {
			return true
		}
	}
	return false
}

func (r *ComposeContentEmitterReturningValuesRule) check(ctx *api.Context) {
	file := ctx.File
	composables := composeLintsFileComposables(file)
	returnsValue := false
	for _, c := range composables {
		if acrFunctionReturnKind(file, c.fn) == acrReturnValue {
			returnsValue = true
			break
		}
	}
	if !returnsValue {
		return
	}
	direct, final := composeLintsEmissionFixpoint(file, composables, composeLintsEmitterSet(r.ContentEmitters), true, nil)
	for i, c := range composables {
		if (direct[i] > 0 || final[i] > 0) && acrFunctionReturnKind(file, c.fn) == acrReturnValue {
			if name := acrNameNode(file, c.fn); name != 0 {
				ctx.EmitAtNode(name, composeContentEmitterReturningValuesMessage)
			}
		}
	}
}

// composeLintsNonRestartableModifiers disqualify a composable from @NonRestartableComposable.
var composeLintsNonRestartableModifiers = []string{"inline", "open", "override", "abstract", "expect", "actual",
	"external", "tailrec"}

// composeLintsIneligibleAnnotations are the runtime annotations that already control the restart group.
var composeLintsIneligibleAnnotations = map[string]bool{"ExplicitGroupsComposable": true,
	"NonRestartableComposable": true, "NonSkippableComposable": true, "ReadOnlyComposable": true}

// composeLintsLibraryDelegates are Compose library composables known to be non-inline, Unit-returning and not
// read-only in every overload, matched by name through an import from their packages.
var composeLintsLibraryDelegates = map[string][]string{
	"Text":                      {"androidx.compose.material3", "androidx.compose.material", "androidx.compose.foundation.text"},
	"Icon":                      {"androidx.compose.material3", "androidx.compose.material"},
	"Image":                     {"androidx.compose.foundation"},
	"Spacer":                    {"androidx.compose.foundation.layout"},
	"HorizontalDivider":         {"androidx.compose.material3"},
	"VerticalDivider":           {"androidx.compose.material3"},
	"Divider":                   {"androidx.compose.material3", "androidx.compose.material"},
	"CircularProgressIndicator": {"androidx.compose.material3", "androidx.compose.material"},
	"LinearProgressIndicator":   {"androidx.compose.material3", "androidx.compose.material"},
	"Checkbox":                  {"androidx.compose.material3", "androidx.compose.material"},
	"Switch":                    {"androidx.compose.material3", "androidx.compose.material"},
	"RadioButton":               {"androidx.compose.material3", "androidx.compose.material"},
	"Snackbar":                  {"androidx.compose.material3", "androidx.compose.material"},
}

func (r *ComposeNonRestartableComposableRule) check(ctx *api.Context) {
	if ctx.CodeIndex == nil {
		return
	}
	var previews *composeLintsPreviews
	targets := newComposeLintsDelegateTargets(ctx.CodeIndex)
	qualifiers := newComposeLintsQualifiers(ctx.CodeIndex)
	for _, file := range ctx.CodeIndex.Files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin ||
			!bytes.Contains(file.Content, []byte("Composable")) {
			continue
		}
		file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
			annotation := composeLintsComposableAnnotation(file, fn)
			if annotation == 0 || acrIsLocalDeclaration(file, fn) || composeLintsInInterface(file, fn) ||
				acrFunctionReturnKind(file, fn) != acrReturnUnit || !r.eligible(file, fn) {
				return
			}
			call, receiver := composeLintsSingleDelegateCall(file, fn)
			if call == 0 || !composeLintsForwardsOnly(file, fn, call, receiver, qualifiers) {
				return
			}
			if !targets.delegatable(file, composeLintsCalleeName(file, call)) {
				return
			}
			if previews == nil {
				previews = newComposeLintsPreviews(ctx.CodeIndex.Files)
			}
			if previews.annotatedIsPreview(file, fn, 0) {
				return
			}
			ctx.Emit(r.Finding(file, file.FlatRow(annotation)+1, file.FlatCol(annotation)+1, composeNonRestartableComposableMessage))
		})
	}
}

// eligible rejects the modifiers and annotations compose-lints excludes.
func (r *ComposeNonRestartableComposableRule) eligible(file *scanner.File, fn uint32) bool {
	for _, m := range composeLintsNonRestartableModifiers {
		if acrFunctionHasModifier(file, fn, m) {
			return false
		}
	}
	ok := true
	composeLintsEachAnnotation(file, fn, func(a uint32) {
		text := composeLintsAnnotationTypeText(file, a)
		if composeLintsIneligibleAnnotations[text[strings.LastIndex(text, ".")+1:]] {
			ok = false
		}
	})
	return ok
}

// composeLintsComposableAnnotation returns the @Composable annotation node on a function, or 0.
func composeLintsComposableAnnotation(file *scanner.File, fn uint32) uint32 {
	var found uint32
	composeLintsEachAnnotation(file, fn, func(a uint32) {
		if found == 0 && acrAnnotationIsComposable(file, a) {
			found = a
		}
	})
	return found
}

// composeLintsSingleDelegateCall returns the only call of a function body that is a single expression (an
// expression body, or a block with one statement): `Label(text)` or `receiver.Label(text)`, with no other call and
// no lambda inside, and the explicit receiver if any.
func composeLintsSingleDelegateCall(file *scanner.File, fn uint32) (uint32, uint32) {
	body, ok := file.FlatFindChild(fn, "function_body")
	if !ok {
		return 0, 0
	}
	var expr uint32
	if first := file.FlatFirstChild(body); first != 0 && file.FlatType(first) == "=" {
		expr = file.FlatNextSib(first)
	} else if stmts, ok := file.FlatFindChild(body, "statements"); ok {
		for s := file.FlatFirstChild(stmts); s != 0; s = file.FlatNextSib(s) {
			if !file.FlatIsNamed(s) || composeLintsIsComment(file, s) {
				continue
			}
			if expr != 0 {
				return 0, 0
			}
			expr = s
		}
	}
	expr = composeLintsUnwrapParens(file, expr)
	if expr == 0 || file.FlatType(expr) != "call_expression" {
		return 0, 0
	}
	calls, lambdas := 0, 0
	file.FlatWalkAllNodes(expr, func(n uint32) {
		switch file.FlatType(n) {
		case "call_expression":
			calls++
		case "lambda_literal", "anonymous_function", "annotated_lambda":
			lambdas++
		}
	})
	if calls != 1 || lambdas != 0 {
		return 0, 0
	}
	var receiver uint32
	if callee := file.FlatFirstChild(expr); file.FlatType(callee) == "navigation_expression" {
		suffix, ok := file.FlatFindChild(callee, "navigation_suffix")
		if !ok || strings.HasPrefix(file.FlatNodeText(suffix), "?.") {
			return 0, 0
		}
		receiver = file.FlatFirstChild(callee)
	}
	return expr, receiver
}

// composeLintsForwardsOnly mirrors compose-lints' isForwardedValue/isForwardedReceiver: every argument is a
// constant, a parameter, an uninterpolated string or `this`, and the receiver is `this`, a parameter, or a type or
// package qualifier.
func composeLintsForwardsOnly(file *scanner.File, fn, call, receiver uint32, qualifiers *composeLintsQualifiers) bool {
	params := map[string]bool{}
	for _, p := range composeLintsParams(file, fn) {
		params[strings.Trim(file.FlatChildTextOrEmpty(p.node, "simple_identifier"), "`")] = true
	}
	if suffix, ok := file.FlatFindChild(call, "call_suffix"); ok {
		if args, ok := file.FlatFindChild(suffix, "value_arguments"); ok {
			for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
				if file.FlatType(arg) != "value_argument" {
					continue
				}
				expr := file.FlatFirstChild(arg)
				if expr != 0 && file.FlatType(expr) == "simple_identifier" {
					if eq := file.FlatNextSib(expr); eq != 0 && file.FlatType(eq) == "=" {
						expr = file.FlatNextSib(eq)
					}
				}
				if expr != 0 && file.FlatType(expr) == "*" {
					return false // spread
				}
				if !composeLintsIsForwardedValue(file, composeLintsUnwrapParens(file, expr), params) {
					return false
				}
			}
		}
	}
	if receiver == 0 {
		return true
	}
	receiver = composeLintsUnwrapParens(file, receiver)
	switch file.FlatType(receiver) {
	case "this_expression":
		return true
	case "simple_identifier":
		name := file.FlatNodeText(receiver)
		return params[name] || qualifiers.isType(name)
	case "navigation_expression":
		root, selectors := composeLintsChain(file, receiver)
		if root == 0 || file.FlatType(root) != "simple_identifier" || params[file.FlatNodeText(root)] ||
			strings.Contains(file.FlatNodeText(receiver), "(") {
			return false
		}
		return qualifiers.isQualifier(file, append([]string{file.FlatNodeText(root)}, selectors...))
	}
	return false
}

// composeLintsQualifiers mirrors compose-lints' isTypeOrPackageQualifier over the project: each segment of a
// qualified receiver must name a type (class, object, interface or type alias) or extend a package prefix.
type composeLintsQualifiers struct {
	index *scanner.CodeIndex
	// packages holds every project package and each of its dotted prefixes.
	packages map[string]bool
	types    map[string]bool
}

func newComposeLintsQualifiers(index *scanner.CodeIndex) *composeLintsQualifiers {
	q := &composeLintsQualifiers{index: index, packages: map[string]bool{}, types: map[string]bool{}}
	for _, f := range index.Files {
		if f == nil {
			continue
		}
		pkg := filePackageName(f)
		for pkg != "" && !q.packages[pkg] {
			q.packages[pkg] = true
			i := strings.LastIndexByte(pkg, '.')
			if i < 0 {
				break
			}
			pkg = pkg[:i]
		}
	}
	return q
}

// isType reports whether name resolves to a type: the project declares it as a class, object, interface or type
// alias, or the project does not declare it at all and it is capitalized (a library type such as MaterialTheme).
// A name the project declares only as a property or function (`val holder`, `val Holder`) is no type.
func (q *composeLintsQualifiers) isType(name string) bool {
	if v, ok := q.types[name]; ok {
		return v
	}
	typeDecl, other := false, false
	for _, sym := range q.index.SymbolsNamed(name) {
		switch sym.Kind {
		case "class", "object", "interface", "typealias":
			typeDecl = true
		default:
			other = true
		}
	}
	v := typeDecl || !other && acrFirstRuneIsUpper(name)
	q.types[name] = v
	return v
}

// isQualifier reports whether every segment of a dotted receiver is a package prefix or a type.
func (q *composeLintsQualifiers) isQualifier(file *scanner.File, segments []string) bool {
	prefix := ""
	inPackage := true
	for _, seg := range segments {
		if prefix == "" {
			prefix = seg
		} else {
			prefix += "." + seg
		}
		if inPackage && q.isPackage(file, prefix) {
			continue
		}
		inPackage = false
		if !q.isType(seg) {
			return false
		}
	}
	return true
}

// isPackage reports whether a dotted name is a project package (or a prefix of one) or a package prefix of one of
// the file's imports.
func (q *composeLintsQualifiers) isPackage(file *scanner.File, name string) bool {
	if q.packages[name] {
		return true
	}
	facts := fileFactsCache().Imports(file)
	if facts == nil {
		return false
	}
	for fqn := range facts.FQNs {
		if strings.HasPrefix(fqn, name+".") {
			return true
		}
	}
	for base := range facts.Wildcards {
		if strings.HasPrefix(base, name+".") {
			return true
		}
	}
	return false
}

func composeLintsIsForwardedValue(file *scanner.File, expr uint32, params map[string]bool) bool {
	if expr == 0 {
		return false
	}
	switch file.FlatType(expr) {
	case "integer_literal", "real_literal", "boolean_literal", "character_literal", "long_literal", "hex_literal",
		"bin_literal", "unsigned_literal", "null", "this_expression":
		return true
	case "simple_identifier":
		name := file.FlatNodeText(expr)
		return params[name] || name == "null" || name == "true" || name == "false"
	case "string_literal":
		return !file.FlatHasChildOfType(expr, "interpolated_identifier") && !file.FlatHasChildOfType(expr, "interpolated_expression")
	}
	return false
}

// composeLintsDelegateTargets decides whether a callee name is a composable compose-lints would accept as the
// delegate: every project function of that name is @Composable, not @ReadOnlyComposable, not inline and returns
// Unit; without project functions of that name, a curated library composable imported from its package.
type composeLintsDelegateTargets struct {
	index *scanner.CodeIndex
	files map[string]*scanner.File
	nodes map[*scanner.File]map[int]uint32
	memo  map[string]int8
}

func newComposeLintsDelegateTargets(index *scanner.CodeIndex) *composeLintsDelegateTargets {
	files := make(map[string]*scanner.File, len(index.Files))
	for _, f := range index.Files {
		if f != nil {
			files[f.Path] = f
		}
	}
	return &composeLintsDelegateTargets{index: index, files: files, nodes: map[*scanner.File]map[int]uint32{},
		memo: map[string]int8{}}
}

func (t *composeLintsDelegateTargets) delegatable(file *scanner.File, name string) bool {
	if name == "" {
		return false
	}
	v, ok := t.memo[name]
	if !ok {
		v = t.project(name)
		t.memo[name] = v
	}
	switch v {
	case 1:
		return true
	case -1:
		return false
	}
	pkgs, ok := composeLintsLibraryDelegates[name]
	if !ok {
		return false
	}
	facts := fileFactsCache().Imports(file)
	if facts == nil {
		return false
	}
	target, imported := facts.Aliases[name]
	for _, pkg := range pkgs {
		if imported && target == pkg+"."+name {
			return true
		}
	}
	return false
}

// project classifies the project's functions named name: 1 all are delegatable, -1 some are not, 0 none exist.
func (t *composeLintsDelegateTargets) project(name string) int8 {
	var result int8
	for _, sym := range t.index.SymbolsNamed(name) {
		if sym.Kind != "function" {
			continue
		}
		file := t.files[sym.File]
		fn := t.node(file, sym.StartByte)
		if fn == 0 || !acrFunctionIsComposable(file, fn) || acrFunctionHasModifier(file, fn, "inline") ||
			acrFunctionReturnKind(file, fn) != acrReturnUnit || composeLintsHasAnnotationNamed(file, fn, "ReadOnlyComposable") {
			return -1
		}
		result = 1
	}
	return result
}

func (t *composeLintsDelegateTargets) node(file *scanner.File, start int) uint32 {
	if file == nil || file.FlatTree == nil {
		return 0
	}
	byStart, ok := t.nodes[file]
	if !ok {
		byStart = map[int]uint32{}
		file.FlatWalkNodes(0, "function_declaration", func(fn uint32) { byStart[int(file.FlatStartByte(fn))] = fn })
		t.nodes[file] = byStart
	}
	return byStart[start]
}

// composeLintsHasAnnotationNamed reports whether decl carries an annotation whose simple name is name.
func composeLintsHasAnnotationNamed(file *scanner.File, decl uint32, name string) bool {
	found := false
	composeLintsEachAnnotation(file, decl, func(a uint32) {
		text := composeLintsAnnotationTypeText(file, a)
		if text[strings.LastIndex(text, ".")+1:] == name {
			found = true
		}
	})
	return found
}
