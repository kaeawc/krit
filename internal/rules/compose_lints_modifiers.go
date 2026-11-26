package rules

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Messages of compose-lint-checks 1.6.1's modifier and parameter-order detectors, as Lint reports them.
const (
	composeModifierMissingMessage = "This @Composable function emits content but doesn't have a modifier parameter. " +
		"See https://slackhq.github.io/compose-lints/rules/#when-should-i-expose-modifier-parameters for more information."
	composeModifierWithoutDefaultMessage = "This @Composable function has a modifier parameter but it doesn't have a " +
		"default value. See https://slackhq.github.io/compose-lints/rules/#modifiers-should-have-default-parameters for " +
		"more information."
	composeParameterOrderMessage = "Parameters in a composable function should be ordered following this pattern: " +
		"params without defaults, modifiers, params with defaults and optionally, a trailing function that might not " +
		"have a default param. Current params are: `[%s]` but should be `[%s]`. See " +
		"https://slackhq.github.io/compose-lints/rules/#ordering-composable-parameters-properly for more information."
)

// composeLintsEmitters are the composables compose-lints knows to emit content (ComposableEmittersList).
var composeLintsEmitters = map[string]bool{
	"BasicTextField": true, "Box": true, "Canvas": true, "ClickableText": true, "Column": true, "Icon": true,
	"Image": true, "Layout": true, "LazyColumn": true, "LazyRow": true, "LazyVerticalGrid": true, "Row": true,
	"Text": true, "BottomDrawer": true, "Button": true, "Card": true, "Checkbox": true,
	"CircularProgressIndicator": true, "Divider": true, "DropdownMenu": true, "DropdownMenuItem": true,
	"ExposedDropdownMenuBox": true, "ExtendedFloatingActionButton": true, "FloatingActionButton": true,
	"IconButton": true, "IconToggleButton": true, "LeadingIconTab": true, "LinearProgressIndicator": true,
	"ListItem": true, "ModalBottomSheetLayout": true, "ModalDrawer": true, "NavigationRail": true,
	"NavigationRailItem": true, "OutlinedButton": true, "OutlinedTextField": true, "RadioButton": true,
	"Scaffold": true, "ScrollableTabRow": true, "Slider": true, "SnackbarHost": true, "Surface": true,
	"SwipeToDismiss": true, "Switch": true, "Tab": true, "TabRow": true, "TextButton": true, "TopAppBar": true,
	"BottomNavigation": true, "BottomNavigationContent": true, "BottomNavigationSurface": true, "FlowColumn": true,
	"FlowRow": true, "HorizontalPager": true, "HorizontalPagerIndicator": true, "SwipeRefresh": true,
	"SwipeRefreshIndicator": true, "TopAppBarContent": true, "TopAppBarSurface": true, "VerticalPager": true,
	"VerticalPagerIndicator": true, "WebView": true,
}

// composeLintsNonEmitters emit into their own window; compose-lints does not look inside them.
var composeLintsNonEmitters = map[string]bool{"AlertDialog": true, "ModalBottomSheetLayout": true}

// ComposeModifierMissingRule ports compose-lints' ComposeModifierMissing: a Unit @Composable that emits content
// but has no Modifier parameter. Previews, overrides and interface members are exempt.
type ComposeModifierMissingRule struct {
	BaseRule
	// ContentEmitters are extra composables that emit content (compose-lints `content-emitters`).
	ContentEmitters []string
	// VisibilityThreshold is only_public (default), public_and_internal or all (compose-lints
	// `visibility-threshold`).
	VisibilityThreshold string
}

// ComposeModifierWithoutDefaultRule ports compose-lints' ComposeModifierWithoutDefault: a Modifier parameter of a
// @Composable function without a default value.
type ComposeModifierWithoutDefaultRule struct {
	FlatDispatchBase
	BaseRule
}

// ComposeParameterOrderRule ports compose-lints' ComposeParameterOrder: parameters without defaults, then
// modifiers, then parameters with defaults, then an optional trailing function.
//
// It runs once per analysis over the cross-file index: whether the last parameter is a trailing function depends
// on type aliases that may be declared in other files.
type ComposeParameterOrderRule struct {
	BaseRule
}

// composeLintsParam is one value parameter: its optional parameter_modifiers, the parameter node and its default.
type composeLintsParam struct {
	start, end uint32 // byte range, from the modifiers through the default value
	node       uint32
	def        uint32
}

// composeLintsParams returns a function's value parameters in order.
func composeLintsParams(file *scanner.File, fn uint32) []composeLintsParam {
	list, ok := file.FlatFindChild(fn, "function_value_parameters")
	if !ok {
		return nil
	}
	var out []composeLintsParam
	var pendingStart uint32
	havePending := false
	for child := file.FlatFirstChild(list); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "parameter_modifiers":
			pendingStart, havePending = file.FlatStartByte(child), true
		case "parameter":
			p := composeLintsParam{start: file.FlatStartByte(child), end: file.FlatEndByte(child), node: child}
			if havePending {
				p.start = pendingStart
			}
			havePending = false
			if def := acuiParameterDefault(file, child); def != 0 {
				p.def = def
				p.end = file.FlatEndByte(def)
			}
			out = append(out, p)
		}
	}
	return out
}

func (p composeLintsParam) text(file *scanner.File) string {
	return strings.Join(strings.Fields(string(file.Content[p.start:p.end])), " ")
}

// composeLintsParamIsModifier mirrors compose-lints' isModifier: the type is written Modifier or GlanceModifier,
// or resolves to androidx.compose.ui.Modifier or androidx.glance.GlanceModifier (also when nullable).
func composeLintsParamIsModifier(file *scanner.File, param uint32) bool {
	t := composeLintsDeclaredType(file, param)
	if t == 0 {
		return false
	}
	if file.FlatType(t) == "user_type" {
		if text := composeLintsTypeText(file, t); text == "Modifier" || text == "GlanceModifier" {
			return true
		}
	}
	if composeLintsTypeIsModifier(file, t) {
		return true
	}
	if file.FlatType(t) == "nullable_type" {
		t, _ = file.FlatFindChild(t, "user_type")
	}
	if t == 0 {
		return false
	}
	text := composeLintsTypeText(file, t)
	return text == "androidx.glance.GlanceModifier" || text == "GlanceModifier" && acrNameResolvesTo(file, "androidx.glance", "GlanceModifier")
}

// composeLintsInInterface reports whether a member function is declared in an interface body.
func composeLintsInInterface(file *scanner.File, fn uint32) bool {
	body, ok := file.FlatParent(fn)
	if !ok || file.FlatType(body) != "class_body" {
		return false
	}
	decl, ok := file.FlatParent(body)
	if !ok || file.FlatType(decl) != "class_declaration" {
		return false
	}
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "interface":
			return true
		case "class", "type_identifier":
			return false
		}
	}
	return false
}

func (r *ComposeModifierMissingRule) check(ctx *api.Context) {
	if ctx.CodeIndex == nil {
		return
	}
	var previews *composeLintsPreviews
	for _, file := range ctx.CodeIndex.Files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin ||
			!bytes.Contains(file.Content, []byte("Composable")) {
			continue
		}
		file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
			if !acrFunctionIsComposable(file, fn) || acrIsLocalDeclaration(file, fn) ||
				acrFunctionHasModifier(file, fn, "override") || composeLintsInInterface(file, fn) ||
				acrFunctionReturnKind(file, fn) != acrReturnUnit || !r.visible(file, fn) {
				return
			}
			// UAST lists an extension receiver among the parameters, so a Modifier receiver counts.
			if composeLintsReceiverIsModifier(file, fn) {
				return
			}
			for _, p := range composeLintsParams(file, fn) {
				if composeLintsParamIsModifier(file, p.node) {
					return
				}
			}
			if !r.emitsContent(file, fn) {
				return
			}
			if previews == nil {
				previews = newComposeLintsPreviews(ctx.CodeIndex.Files)
			}
			if previews.annotatedIsPreview(file, fn, 0) {
				return
			}
			if name := acrNameNode(file, fn); name != 0 {
				ctx.Emit(r.Finding(file, file.FlatRow(name)+1, file.FlatCol(name)+1, composeModifierMissingMessage))
			}
		})
	}
}

// visible applies the visibility threshold.
func (r *ComposeModifierMissingRule) visible(file *scanner.File, fn uint32) bool {
	switch r.VisibilityThreshold {
	case "all":
		return true
	case "public_and_internal":
		return composeLintsIsPublic(file, fn) || acrFunctionHasModifier(file, fn, "internal")
	}
	return composeLintsIsPublic(file, fn)
}

// emitsContent mirrors compose-lints' emitsContent: some call in the function (looking inside lambdas, but not
// inside AlertDialog or ModalBottomSheetLayout) is a known emitter, a Spacer, a configured content emitter, or
// passes a named `modifier` argument.
func (r *ComposeModifierMissingRule) emitsContent(file *scanner.File, fn uint32) bool {
	found := false
	var walk func(n uint32)
	walk = func(n uint32) {
		for child := file.FlatFirstChild(n); child != 0 && !found; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "call_expression" {
				name := composeLintsCalleeName(file, child)
				if composeLintsNonEmitters[name] {
					continue
				}
				if r.isEmitter(name) || composeLintsHasNamedArgument(file, child, "modifier") {
					found = true
					return
				}
			}
			walk(child)
		}
	}
	walk(fn)
	return found
}

func (r *ComposeModifierMissingRule) isEmitter(name string) bool {
	if name == "" {
		return false
	}
	if composeLintsEmitters[name] {
		return true
	}
	if digits, ok := strings.CutPrefix(name, "Spacer"); ok && strings.Trim(digits, "0123456789") == "" {
		return true
	}
	for _, e := range r.ContentEmitters {
		if e == name {
			return true
		}
	}
	return false
}

// composeLintsCalleeName returns the simple name a call invokes (`Text` for `Text()` and `x.Text()`), or "".
func composeLintsCalleeName(file *scanner.File, call uint32) string {
	callee := file.FlatFirstChild(call)
	switch file.FlatType(callee) {
	case "simple_identifier":
		return file.FlatNodeText(callee)
	case "navigation_expression":
		if suffix, ok := file.FlatFindChild(callee, "navigation_suffix"); ok {
			return file.FlatChildTextOrEmpty(suffix, "simple_identifier")
		}
	}
	return ""
}

// composeLintsHasNamedArgument reports whether a call passes `name = ...`.
func composeLintsHasNamedArgument(file *scanner.File, call uint32, name string) bool {
	suffix, ok := file.FlatFindChild(call, "call_suffix")
	if !ok {
		return false
	}
	args, ok := file.FlatFindChild(suffix, "value_arguments")
	if !ok {
		return false
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		id := file.FlatFirstChild(arg)
		if id != 0 && file.FlatType(id) == "simple_identifier" && file.FlatNodeTextEquals(id, name) {
			if eq := file.FlatNextSib(id); eq != 0 && file.FlatType(eq) == "=" {
				return true
			}
		}
	}
	return false
}

func (r *ComposeModifierWithoutDefaultRule) check(ctx *api.Context) {
	file, fn := ctx.File, ctx.Idx
	if !acrFunctionIsComposable(file, fn) || acrIsLocalDeclaration(file, fn) || composeLintsInInterface(file, fn) ||
		acrFunctionHasModifier(file, fn, "actual") || acrFunctionHasModifier(file, fn, "override") ||
		acrFunctionHasModifier(file, fn, "abstract") || composeLintsReceiverText(file, fn) == "Modifier" ||
		composeLintsReceiverText(file, fn) == "GlanceModifier" {
		return
	}
	for _, p := range composeLintsParams(file, fn) {
		if p.def != 0 || !composeLintsParamIsModifier(file, p.node) {
			continue
		}
		finding := r.Finding(file, file.FlatRow(p.node)+1, file.FlatCol(p.node)+1, composeModifierWithoutDefaultMessage)
		if p.start != file.FlatStartByte(p.node) {
			// Lint's location starts at the parameter's annotations and modifiers.
			finding.Line, finding.Col = composeLintsByteLineCol(file, p.start)
		}
		end := int(file.FlatEndByte(p.node))
		finding.Fix = &scanner.Fix{ByteMode: true, StartByte: end, EndByte: end, Replacement: " = Modifier"}
		ctx.Emit(finding)
	}
}

// composeLintsReceiverText returns the extension receiver type written on a function, or "".
func composeLintsReceiverText(file *scanner.File, fn uint32) string {
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type", "nullable_type":
			if next := file.FlatNextSib(child); next != 0 && file.FlatType(next) == "." {
				return strings.Join(strings.Fields(file.FlatNodeText(child)), "")
			}
			return ""
		case "simple_identifier", "function_value_parameters":
			return ""
		}
	}
	return ""
}

// composeLintsByteLineCol converts a byte offset to a 1-based line and column.
func composeLintsByteLineCol(file *scanner.File, offset uint32) (int, int) {
	line, col := 1, 1
	for i := uint32(0); i < offset && int(i) < len(file.Content); i++ {
		if file.Content[i] == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}
	return line, col
}

func (r *ComposeParameterOrderRule) check(ctx *api.Context) {
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
			params := composeLintsParams(file, fn)
			if len(params) < 2 {
				return
			}
			trailing := aliases.isFunctionType(file, composeLintsDeclaredTypeAny(file, params[len(params)-1].node), 0)
			proper := composeLintsProperOrder(file, params, trailing)
			if composeLintsSameOrder(params, proper) {
				return
			}
			list, _ := file.FlatFindChild(fn, "function_value_parameters")
			ctx.Emit(r.Finding(file, file.FlatRow(list)+1, file.FlatCol(list)+1, fmt.Sprintf(composeParameterOrderMessage,
				composeLintsParamTexts(file, params), composeLintsParamTexts(file, proper))))
		})
	}
}

// composeLintsJavaFunctionalInterfaces are common library types annotated @FunctionalInterface, which Lint's
// hasTrailingFunction treats as functions. Kotlin `fun interface`s are not annotated and do not count.
var composeLintsJavaFunctionalInterfaces = map[string]bool{
	"java.lang.Runnable": true, "java.util.concurrent.Callable": true, "java.util.Comparator": true,
}

// composeLintsTypeAliases indexes the project's top-level type aliases by FQN.
type composeLintsTypeAliases map[string]composeLintsAnnotationClass

func newComposeLintsTypeAliases(files []*scanner.File) composeLintsTypeAliases {
	out := composeLintsTypeAliases{}
	for _, file := range files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin ||
			!bytes.Contains(file.Content, []byte("typealias")) {
			continue
		}
		pkg := filePackageName(file)
		for child := file.FlatFirstChild(0); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "type_alias" {
				out[composeLintsJoin(pkg, acrDeclarationName(file, child))] = composeLintsAnnotationClass{file: file, decl: child}
			}
		}
	}
	return out
}

// isFunctionType mirrors Lint's hasTrailingFunction for a parameter type node: a function type (possibly
// nullable or parenthesized), a type alias of one, or a Java functional interface.
func (a composeLintsTypeAliases) isFunctionType(file *scanner.File, t uint32, depth int) bool {
	for t != 0 && depth < 8 {
		switch file.FlatType(t) {
		case "function_type":
			return true
		case "nullable_type", "parenthesized_type":
			t = file.FlatNamedChild(t, 0)
			for t != 0 && file.FlatType(t) == "type_modifiers" {
				t = file.FlatNextSib(t)
			}
		case "user_type":
			name := composeLintsTypeText(file, t)
			fqn := a.resolve(file, name)
			if composeLintsJavaFunctionalInterfaces[fqn] || strings.HasPrefix(fqn, "java.util.function.") {
				return true
			}
			alias, ok := a[fqn]
			if !ok {
				return false
			}
			file, t, depth = alias.file, composeLintsTypeAliasTarget(alias.file, alias.decl), depth+1
		default:
			return false
		}
	}
	return false
}

// resolve returns the FQN a type name refers to in file: an import or alias, a same-package or same-file alias,
// or a wildcard-imported project alias; the name itself otherwise.
func (a composeLintsTypeAliases) resolve(file *scanner.File, name string) string {
	head, rest, qualified := strings.Cut(name, ".")
	if facts := fileFactsCache().Imports(file); facts != nil {
		if fqn, ok := facts.Aliases[head]; ok {
			if qualified {
				return fqn + "." + rest
			}
			return fqn
		}
		if !qualified {
			for wildcard := range facts.Wildcards {
				if candidate := strings.TrimSuffix(wildcard, "*") + name; a[candidate].file != nil {
					return candidate
				}
			}
		}
	}
	if candidate := composeLintsJoin(filePackageName(file), name); a[candidate].file != nil {
		return candidate
	}
	switch name {
	case "Runnable":
		return "java.lang.Runnable"
	}
	return name
}

// composeLintsTypeAliasTarget returns the aliased type of a type_alias declaration.
func composeLintsTypeAliasTarget(file *scanner.File, decl uint32) uint32 {
	seenEq := false
	for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "=" {
			seenEq = true
		} else if seenEq && file.FlatIsNamed(c) {
			return c
		}
	}
	return 0
}

// composeLintsDeclaredTypeAny returns the declared type of a parameter, including function and parenthesized
// types.
func composeLintsDeclaredTypeAny(file *scanner.File, param uint32) uint32 {
	for child := file.FlatFirstChild(param); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "function_type", "nullable_type", "user_type", "parenthesized_type":
			return child
		}
	}
	return 0
}

// composeLintsProperOrder builds compose-lints' ideal order: parameters without defaults, then those with
// defaults (modifiers first, `modifier` first among them), then the trailing function.
func composeLintsProperOrder(file *scanner.File, params []composeLintsParam, trailing bool) []composeLintsParam {
	body := params
	var tail []composeLintsParam
	if trailing {
		body, tail = params[:len(params)-1], params[len(params)-1:]
	}
	var without, with []composeLintsParam
	for _, p := range body {
		if p.def != 0 {
			with = append(with, p)
		} else {
			without = append(without, p)
		}
	}
	rank := func(p composeLintsParam) int {
		r := 0
		if composeLintsParamIsModifier(file, p.node) {
			r += 2
		}
		if file.FlatChildTextOrEmpty(p.node, "simple_identifier") == "modifier" {
			r++
		}
		return r
	}
	// A stable insertion sort by descending rank keeps the original order among equals, like Kotlin's sortedWith.
	for i := 1; i < len(with); i++ {
		for j := i; j > 0 && rank(with[j]) > rank(with[j-1]); j-- {
			with[j], with[j-1] = with[j-1], with[j]
		}
	}
	out := append(append(without, with...), tail...)
	return out
}

func composeLintsSameOrder(a, b []composeLintsParam) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].node != b[i].node {
			return false
		}
	}
	return true
}

func composeLintsParamTexts(file *scanner.File, params []composeLintsParam) string {
	texts := make([]string, len(params))
	for i, p := range params {
		texts[i] = p.text(file)
	}
	return strings.Join(texts, ", ")
}
