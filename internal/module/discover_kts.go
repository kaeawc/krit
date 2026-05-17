package module

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/kotlin"
)

// parsedSettings is the result of parsing settings.gradle.kts.
type parsedSettings struct {
	paths     []string
	overrides map[string]string
}

// parseSettingsKts walks a Kotlin-DSL settings file's tree-sitter AST to
// extract include() module paths. Handles:
//
//   - Static include(":a", ":b") with literal string arguments.
//   - Static include(":${prefix}:foo") with simple `val prefix = "..."`
//     bindings substituted.
//   - Dynamic iteration through forEach { ... } and `for (x in ...)` with
//     iteration sources:
//     <dir>.listFiles()                            (flat)
//     <dir>.walk() / <dir>.walkTopDown()           (recursive)
//     Files.list(<dir>)                            (flat; any qualifier)
//     Pass-through ops (.filter, .map, .asSequence, .toList, .sortedBy,
//     .sorted, .filterNot, .mapNotNull) between the iteration source
//     and the consumer are walked through.
//   - Variable indirection via `val name = <expr>` for the iteration
//     receiver, recursively (multi-hop).
//   - String interpolations resolved: `<varName>`, `<varName>.name`,
//     `<varName>.fileName`, and string-typed `val` bindings.
//
// Filter predicates are not interpreted; build-script presence
// (build.gradle / build.gradle.kts) gates emission, since that's
// Gradle's own ground truth for "this is a module."
//
// includeBuild() is intentionally not handled — composite builds have
// independent module graphs and shouldn't appear as subprojects.
//
// projectDir overrides go through the regex parseProjectDirOverrides
// because tree-sitter Kotlin doesn't model script-level assignments
// cleanly.
func parseSettingsKts(ctx context.Context, rootDir, src string) parsedSettings {
	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(kotlin.GetLanguage())

	tree, err := parser.ParseCtx(ctx, nil, []byte(src))
	if err != nil || tree == nil {
		return parsedSettings{overrides: parseProjectDirOverrides(src)}
	}
	defer tree.Close()

	w := &ktsWalker{
		rootDir:        rootDir,
		src:            []byte(src),
		seen:           make(map[string]bool),
		bindings:       make(map[string]*sitter.Node),
		stringBindings: make(map[string]string),
	}
	w.walk(tree.RootNode(), nil)
	return parsedSettings{paths: w.paths, overrides: parseProjectDirOverrides(src)}
}

// ktsWalker carries the state needed to resolve receivers and bindings
// during the AST traversal.
type ktsWalker struct {
	rootDir string
	src     []byte
	paths   []string
	seen    map[string]bool
	// bindings maps `val <name> = <rhs>` to the RHS node so chains like
	// `val a = root.resolve("X"); val b = a.resolve("Y"); b.listFiles()`
	// resolve through arbitrary depth.
	bindings map[string]*sitter.Node
	// stringBindings holds resolved string-literal `val` values, used to
	// substitute simple-identifier interpolations like `${prefix}` in
	// include arguments.
	stringBindings map[string]string
}

// iterSource describes a directory iteration encountered as the
// receiver of forEach / for-in. recursive=true for walk/walkTopDown.
type iterSource struct {
	dir       string
	recursive bool
}

// iterCtx carries the iteration state into the body of a forEach
// lambda or for-statement so nested include() calls can expand
// templates.
type iterCtx struct {
	src     iterSource
	varName string // lambda/loop parameter name; "it" for implicit
}

// addPath records a resolved module path, deduping.
func (w *ktsWalker) addPath(p string) {
	p = strings.TrimSpace(p)
	if p == "" {
		return
	}
	if !strings.HasPrefix(p, ":") {
		p = ":" + p
	}
	if !w.seen[p] {
		w.seen[p] = true
		w.paths = append(w.paths, p)
	}
}

// walk dispatches per node type. ctx carries iteration state into the
// body of a forEach lambda or for-statement.
func (w *ktsWalker) walk(n *sitter.Node, ctx *iterCtx) {
	if n == nil {
		return
	}
	switch n.Type() {
	case "property_declaration":
		w.recordBinding(n)
	case "call_expression":
		if w.handleCallExpression(n, ctx) {
			return
		}
	case "for_statement":
		w.handleForStatement(n, ctx)
		return
	}
	for i := 0; i < int(n.NamedChildCount()); i++ {
		w.walk(n.NamedChild(i), ctx)
	}
}

// recordBinding captures `val name = rhs`. Stores the AST node so
// later directory expressions resolve through the binding; also
// extracts the string value if the RHS is a literal string, so
// `val prefix = "compat"; include(":${prefix}:foo")` works.
func (w *ktsWalker) recordBinding(n *sitter.Node) {
	var name string
	var rhs *sitter.Node
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		switch c.Type() {
		case "variable_declaration":
			if c.NamedChildCount() > 0 {
				name = w.text(c.NamedChild(0))
			}
		case "binding_pattern_kind", "user_type", "type_reference":
			// declarator metadata; skip
		default:
			rhs = c
		}
	}
	if name == "" || rhs == nil {
		return
	}
	w.bindings[name] = rhs
	if rhs.Type() == "string_literal" {
		pieces := stringPieces(rhs, w.src)
		if isStatic(pieces) {
			w.stringBindings[name] = staticString(pieces)
		}
	}
}

// handleCallExpression dispatches on the callee name. Returns true if
// the call captured iteration semantics (forEach) and the caller
// should not recurse blindly through it.
func (w *ktsWalker) handleCallExpression(n *sitter.Node, ctx *iterCtx) bool {
	callee := calleeName(n, w.src)
	switch callee {
	case "include":
		w.handleInclude(n, ctx)
		return false
	case "forEach":
		return w.handleForEach(n, ctx)
	}
	return false
}

// handleInclude resolves each argument's string template against the
// current iteration context and any string `val` bindings, emitting
// one path per fully-resolvable result.
func (w *ktsWalker) handleInclude(n *sitter.Node, ctx *iterCtx) {
	args := valueArguments(n)
	for _, arg := range args {
		strNode := stringLiteral(arg)
		if strNode == nil {
			continue
		}
		pieces := stringPieces(strNode, w.src)
		if isStatic(pieces) {
			w.addPath(staticString(pieces))
			continue
		}
		// Templated. With an iteration context, expand against the
		// filesystem; otherwise try `val`-binding substitution alone.
		if ctx != nil {
			w.expandTemplate(pieces, ctx)
			continue
		}
		if path, ok := w.substitute(pieces, "", ""); ok {
			w.addPath(path)
		}
	}
}

// expandTemplate iterates the source's directory entries (recursively
// for .walk()), substitutes the loop variable's accessor in the
// template, and emits one module per entry that contains a build
// script.
func (w *ktsWalker) expandTemplate(pieces []stringPiece, ctx *iterCtx) {
	if ctx.src.dir == "" {
		return
	}
	if ctx.src.recursive {
		_ = filepath.Walk(ctx.src.dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				// Permission errors etc. shouldn't abort the walk; we
				// just skip subtrees we can't enter and continue.
				if info != nil && info.IsDir() {
					return filepath.SkipDir
				}
				return nil //nolint:nilerr // intentional: skip unreadable files but keep walking
			}
			if !info.IsDir() {
				return nil
			}
			if path == ctx.src.dir {
				return nil // skip the iteration root itself
			}
			if !hasBuildScript(path) {
				return nil
			}
			name := filepath.Base(path)
			if p, ok := w.substitute(pieces, ctx.varName, name); ok {
				w.addPath(p)
			}
			return nil
		})
		return
	}

	entries, err := os.ReadDir(ctx.src.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(ctx.src.dir, e.Name())
		if !hasBuildScript(sub) {
			continue
		}
		if p, ok := w.substitute(pieces, ctx.varName, e.Name()); ok {
			w.addPath(p)
		}
	}
}

// handleForEach walks a `<receiver>.forEach { ... }` chain.
func (w *ktsWalker) handleForEach(n *sitter.Node, parent *iterCtx) bool {
	receiver := callReceiver(n)
	src, ok := w.resolveIterationSource(receiver)

	lambda := callLambda(n)
	if lambda == nil {
		return false
	}
	varName := lambdaParamName(lambda, w.src)

	ctx := parent
	if ok {
		ctx = &iterCtx{src: src, varName: varName}
	}
	for i := 0; i < int(lambda.NamedChildCount()); i++ {
		w.walk(lambda.NamedChild(i), ctx)
	}
	return true
}

// handleForStatement processes `for (x in <expr>) { ... }` with the
// same iteration semantics as forEach.
func (w *ktsWalker) handleForStatement(n *sitter.Node, parent *iterCtx) {
	var varName string
	var iterExpr *sitter.Node
	var body *sitter.Node
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		switch c.Type() {
		case "variable_declaration":
			if c.NamedChildCount() > 0 {
				varName = w.text(c.NamedChild(0))
			}
		case "control_structure_body":
			body = c
		default:
			// Anything that isn't the loop variable or body is the
			// iteration expression. The grammar leaves this anonymous,
			// so we take the most-recent non-decl, non-body child.
			iterExpr = c
		}
	}
	src, ok := w.resolveIterationSource(iterExpr)
	ctx := parent
	if ok {
		ctx = &iterCtx{src: src, varName: varName}
	}
	if body != nil {
		w.walk(body, ctx)
	}
}

// resolveIterationSource walks back through transparent ops on a
// forEach/for-in receiver to find the underlying directory iteration.
// Recognized terminal calls: <dir>.listFiles(), <dir>.walk(),
// <dir>.walkTopDown(), Files.list(<dir>) (any qualifier on Files).
// Pass-through ops walked through: filter, filterNot, map, mapNotNull,
// asSequence, asIterable, toList, toSet, toMutableList, sorted,
// sortedBy, sortedDescending, take, drop, distinct, reversed, plus the
// postfix `!!` and `?.` decorators.
func (w *ktsWalker) resolveIterationSource(n *sitter.Node) (iterSource, bool) {
	cur := n
	for cur != nil {
		switch cur.Type() {
		case "postfix_expression":
			if cur.NamedChildCount() == 0 {
				return iterSource{}, false
			}
			cur = cur.NamedChild(0)
			continue
		case "call_expression":
			callee := calleeName(cur, w.src)
			switch callee {
			case "listFiles":
				d := w.evalDirExpr(callReceiver(cur))
				if d == "" {
					return iterSource{}, false
				}
				return iterSource{dir: d}, true
			case "walk", "walkTopDown":
				d := w.evalDirExpr(callReceiver(cur))
				if d == "" {
					return iterSource{}, false
				}
				return iterSource{dir: d, recursive: true}, true
			case "list":
				// Files.list(<dir>) — we accept any qualifier so long
				// as the immediate parent of `list` is `Files`.
				if !calleeQualifierEndsWith(cur, "Files", w.src) {
					return iterSource{}, false
				}
				args := valueArguments(cur)
				if len(args) == 0 {
					return iterSource{}, false
				}
				argExpr := argInner(args[0])
				d := w.evalDirExpr(argExpr)
				if d == "" {
					return iterSource{}, false
				}
				return iterSource{dir: d}, true
			case "filter", "filterNot", "map", "mapNotNull", "asSequence",
				"asIterable", "toList", "toSet", "toMutableList",
				"sorted", "sortedBy", "sortedDescending",
				"take", "drop", "distinct", "reversed":
				rcv := callReceiver(cur)
				if rcv == nil {
					return iterSource{}, false
				}
				cur = rcv
				continue
			}
			return iterSource{}, false
		}
		return iterSource{}, false
	}
	return iterSource{}, false
}

// evalDirExpr translates an expression that names a directory into an
// absolute path, returning "" for shapes we don't understand.
//
// Recognized shapes:
//
//	rootProject.projectDir, projectDir, rootDir → rootDir
//	<dir>.resolve("X")                         → <dir>/X
//	file("X") / File("X")                       → rootDir/X (or absolute)
//	<simple_identifier> bound by `val name = <expr>` → recurse on RHS
func (w *ktsWalker) evalDirExpr(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	switch n.Type() {
	case "simple_identifier":
		name := w.text(n)
		if name == "rootDir" || name == "projectDir" {
			return w.rootDir
		}
		if rhs, ok := w.bindings[name]; ok {
			return w.evalDirExpr(rhs)
		}
		return ""
	case "navigation_expression":
		// rootProject.projectDir / settings.rootDir etc.
		right := navSuffixIdent(n, w.src)
		if right == "projectDir" || right == "rootDir" {
			return w.rootDir
		}
		return ""
	case "call_expression":
		callee := calleeName(n, w.src)
		args := valueArguments(n)
		switch callee {
		case "resolve":
			if len(args) == 0 {
				return ""
			}
			sub := literalArg(args[0], w.src)
			if sub == "" {
				return ""
			}
			parent := w.evalDirExpr(callReceiver(n))
			if parent == "" {
				return ""
			}
			return filepath.Join(parent, filepath.FromSlash(sub))
		case "file", "File":
			if len(args) == 0 {
				return ""
			}
			s := literalArg(args[0], w.src)
			if s == "" {
				return ""
			}
			if filepath.IsAbs(s) {
				return s
			}
			return filepath.Join(w.rootDir, filepath.FromSlash(s))
		}
	case "postfix_expression":
		if n.NamedChildCount() > 0 {
			return w.evalDirExpr(n.NamedChild(0))
		}
	}
	return ""
}

// substitute resolves a templated string against the iteration context
// and string `val` bindings. varName/entry may be empty when called
// without a loop context. Returns ok=false if any interpolation can't
// be resolved.
//
// Recognized interpolations:
//
//   - <varName>                       → entry
//   - <varName>.name                  → entry
//   - <varName>.fileName              → entry (Path API)
//   - <simple_identifier>             → string `val` binding
func (w *ktsWalker) substitute(pieces []stringPiece, varName, entry string) (string, bool) {
	var sb strings.Builder
	for _, p := range pieces {
		if p.interpolation == nil {
			sb.WriteString(p.literal)
			continue
		}
		inner := p.interpolation
		if inner.NamedChildCount() > 0 {
			inner = inner.NamedChild(0)
		}
		// Loop-variable accessors first.
		if varName != "" && isLoopVarAccessor(inner, w.src, varName) {
			sb.WriteString(entry)
			continue
		}
		// String binding.
		if inner.Type() == "simple_identifier" {
			name := inner.Content(w.src)
			if v, ok := w.stringBindings[name]; ok {
				sb.WriteString(v)
				continue
			}
		}
		return "", false
	}
	return sb.String(), true
}

// isLoopVarAccessor recognizes `<varName>`, `<varName>.name`, and
// `<varName>.fileName`. The latter handles Path entries from
// Files.list().
func isLoopVarAccessor(n *sitter.Node, src []byte, varName string) bool {
	if n == nil {
		return false
	}
	switch n.Type() {
	case "simple_identifier":
		return identMatches(n, src, varName)
	case "navigation_expression":
		if n.NamedChildCount() < 1 {
			return false
		}
		if !identMatches(n.NamedChild(0), src, varName) {
			return false
		}
		for i := 0; i < int(n.NamedChildCount()); i++ {
			c := n.NamedChild(i)
			if c.Type() == "navigation_suffix" && c.NamedChildCount() > 0 {
				name := c.NamedChild(0).Content(src)
				if name == "name" || name == "fileName" {
					return true
				}
			}
		}
		return false
	}
	return false
}

// identMatches reports whether n is a simple_identifier with the given text.
func identMatches(n *sitter.Node, src []byte, name string) bool {
	if n == nil || n.Type() != "simple_identifier" {
		return false
	}
	if int(n.EndByte()-n.StartByte()) != len(name) {
		return false
	}
	return n.Content(src) == name
}

// stringPiece is one chunk of a string_literal: a literal text run or
// an interpolation.
type stringPiece struct {
	literal       string
	interpolation *sitter.Node // non-nil if this is ${...}
}

// stringPieces decomposes a string_literal in source order.
func stringPieces(strNode *sitter.Node, src []byte) []stringPiece {
	var pieces []stringPiece
	for i := 0; i < int(strNode.NamedChildCount()); i++ {
		c := strNode.NamedChild(i)
		switch c.Type() {
		case "string_content":
			pieces = append(pieces, stringPiece{literal: c.Content(src)})
		case "interpolated_expression", "interpolation":
			pieces = append(pieces, stringPiece{interpolation: c})
		}
	}
	return pieces
}

func isStatic(pieces []stringPiece) bool {
	for _, p := range pieces {
		if p.interpolation != nil {
			return false
		}
	}
	return true
}

func staticString(pieces []stringPiece) string {
	var sb strings.Builder
	for _, p := range pieces {
		sb.WriteString(p.literal)
	}
	return sb.String()
}

// --- Generic AST helpers -------------------------------------------------

func (w *ktsWalker) text(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	return n.Content(w.src)
}

// calleeName returns the rightmost identifier name of a call_expression's
// callee. For `a.b.c(args)` returns "c"; for `c(args)` returns "c".
func calleeName(callExpr *sitter.Node, src []byte) string {
	if callExpr == nil || callExpr.NamedChildCount() == 0 {
		return ""
	}
	expr := callExpr.NamedChild(0)
	switch expr.Type() {
	case "simple_identifier":
		return expr.Content(src)
	case "navigation_expression":
		return navSuffixIdent(expr, src)
	}
	return ""
}

// calleeQualifierEndsWith reports whether the callee of a navigation-style
// call has a qualifier whose rightmost part matches `name`. For example,
// `Files.list(...)` and `java.nio.file.Files.list(...)` both have a
// callee whose immediate qualifier is `Files`.
func calleeQualifierEndsWith(callExpr *sitter.Node, name string, src []byte) bool {
	if callExpr == nil || callExpr.NamedChildCount() == 0 {
		return false
	}
	expr := callExpr.NamedChild(0)
	if expr.Type() != "navigation_expression" {
		return false
	}
	if expr.NamedChildCount() == 0 {
		return false
	}
	left := expr.NamedChild(0)
	switch left.Type() {
	case "simple_identifier":
		return left.Content(src) == name
	case "navigation_expression":
		return navSuffixIdent(left, src) == name
	}
	return false
}

// navSuffixIdent returns the rightmost identifier inside a navigation_expression.
func navSuffixIdent(navExpr *sitter.Node, src []byte) string {
	for i := int(navExpr.NamedChildCount()) - 1; i >= 0; i-- {
		c := navExpr.NamedChild(i)
		if c.Type() == "navigation_suffix" && c.NamedChildCount() > 0 {
			return c.NamedChild(0).Content(src)
		}
	}
	return ""
}

// callReceiver returns the receiver expression of a navigation-style
// call: for `a.b.foo()` returns the node for `a.b`. Returns nil for
// bare-identifier calls.
func callReceiver(callExpr *sitter.Node) *sitter.Node {
	if callExpr == nil || callExpr.NamedChildCount() == 0 {
		return nil
	}
	expr := callExpr.NamedChild(0)
	if expr.Type() != "navigation_expression" {
		return nil
	}
	if expr.NamedChildCount() == 0 {
		return nil
	}
	return expr.NamedChild(0)
}

// callLambda returns the trailing lambda_literal of a call_expression.
func callLambda(callExpr *sitter.Node) *sitter.Node {
	for i := 0; i < int(callExpr.NamedChildCount()); i++ {
		c := callExpr.NamedChild(i)
		if c.Type() != "call_suffix" {
			continue
		}
		for j := 0; j < int(c.NamedChildCount()); j++ {
			cc := c.NamedChild(j)
			if cc.Type() == "annotated_lambda" {
				if cc.NamedChildCount() > 0 && cc.NamedChild(0).Type() == "lambda_literal" {
					return cc.NamedChild(0)
				}
			}
			if cc.Type() == "lambda_literal" {
				return cc
			}
		}
	}
	return nil
}

// valueArguments returns the value_argument children of a call_expression's
// value_arguments group.
func valueArguments(callExpr *sitter.Node) []*sitter.Node {
	var out []*sitter.Node
	for i := 0; i < int(callExpr.NamedChildCount()); i++ {
		c := callExpr.NamedChild(i)
		if c.Type() != "call_suffix" {
			continue
		}
		for j := 0; j < int(c.NamedChildCount()); j++ {
			va := c.NamedChild(j)
			if va.Type() == "value_arguments" {
				for k := 0; k < int(va.NamedChildCount()); k++ {
					arg := va.NamedChild(k)
					if arg.Type() == "value_argument" {
						out = append(out, arg)
					}
				}
			}
		}
	}
	return out
}

// argInner returns the inner expression of a value_argument node.
func argInner(arg *sitter.Node) *sitter.Node {
	if arg == nil || arg.NamedChildCount() == 0 {
		return nil
	}
	return arg.NamedChild(0)
}

// stringLiteral returns the string_literal child of a value_argument.
func stringLiteral(arg *sitter.Node) *sitter.Node {
	if arg == nil {
		return nil
	}
	for i := 0; i < int(arg.NamedChildCount()); i++ {
		c := arg.NamedChild(i)
		if c.Type() == "string_literal" {
			return c
		}
	}
	return nil
}

// literalArg returns the literal text of a value_argument's
// string_literal, or "" if the arg isn't a static string.
func literalArg(arg *sitter.Node, src []byte) string {
	s := stringLiteral(arg)
	if s == nil {
		return ""
	}
	pieces := stringPieces(s, src)
	if !isStatic(pieces) {
		return ""
	}
	return staticString(pieces)
}

// lambdaParamName returns the explicit parameter name of a lambda
// (`{ f -> ... }` → "f"). Returns "it" for lambdas without an explicit
// parameter list — Kotlin's implicit single-parameter convention.
func lambdaParamName(lambda *sitter.Node, src []byte) string {
	for i := 0; i < int(lambda.NamedChildCount()); i++ {
		c := lambda.NamedChild(i)
		if c.Type() != "lambda_parameters" {
			continue
		}
		for j := 0; j < int(c.NamedChildCount()); j++ {
			vd := c.NamedChild(j)
			if vd.Type() == "variable_declaration" && vd.NamedChildCount() > 0 {
				id := vd.NamedChild(0)
				if id.Type() == "simple_identifier" {
					return id.Content(src)
				}
			}
		}
	}
	return "it"
}
