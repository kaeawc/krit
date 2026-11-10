package rules

import (
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Shared helpers for the Android Lint source checks that work on both Kotlin and Java call sites.

// lintCall is a Kotlin call_expression or Java method_invocation seen through one shape.
type lintCall struct {
	File *scanner.File
	Idx  uint32
	Name string
	// ReceiverText is the source text of the receiver, or "" for an unqualified call.
	ReceiverText string
	// ReceiverNode is the receiver expression node, or 0 for an unqualified call.
	ReceiverNode uint32
	// Args holds the argument expression nodes in source order; Labels holds each argument's Kotlin label.
	Args   []uint32
	Labels []string
	// TypeText is the type named by a Java `new` expression, or the qualified callee of a Kotlin call, which is a
	// constructor when the callee is a class.
	TypeText string
}

// lintCallAt describes the call at idx, or reports false when idx is not a call.
func lintCallAt(file *scanner.File, idx uint32) (lintCall, bool) {
	if file == nil {
		return lintCall{}, false
	}
	c := lintCall{File: file, Idx: idx}
	switch file.FlatType(idx) {
	case "call_expression":
		c.Name = flatCallExpressionName(file, idx)
		if c.Name == "" {
			return c, false
		}
		if nav, _ := flatCallExpressionParts(file, idx); nav != 0 {
			c.ReceiverNode = astReceiverNodeFromCall(file, idx)
			if c.ReceiverNode != 0 {
				c.ReceiverText = strings.TrimSpace(file.FlatNodeText(c.ReceiverNode))
			}
		}
		c.TypeText = c.Name
		if c.ReceiverText != "" {
			c.TypeText = c.ReceiverText + "." + c.Name
		}
		args := flatCallKeyArguments(file, idx)
		for arg := file.FlatFirstChild(args); args != 0 && arg != 0; arg = file.FlatNextSib(arg) {
			if file.FlatType(arg) != "value_argument" {
				continue
			}
			// A keyword literal such as `null` has no named node; keep its slot so positions stay right.
			expr := flatValueArgumentExpression(file, arg)
			c.Args = append(c.Args, expr)
			c.Labels = append(c.Labels, flatValueArgumentLabel(file, arg))
		}
	case "object_creation_expression":
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "type_identifier", "scoped_type_identifier", "generic_type":
				c.TypeText = strings.TrimSpace(file.FlatNodeText(child))
			}
		}
		if i := strings.Index(c.TypeText, "<"); i >= 0 {
			c.TypeText = c.TypeText[:i]
		}
		c.Name = c.TypeText[strings.LastIndex(c.TypeText, ".")+1:]
		if c.Name == "" {
			return c, false
		}
		c.Args = javaArgumentExpressions(file, idx)
		c.Labels = make([]string, len(c.Args))
	case "method_invocation":
		c.Name = javaMethodInvocationName(file, idx)
		if c.Name == "" {
			return c, false
		}
		var named []uint32
		for child := file.FlatFirstChild(idx); child != 0 && file.FlatType(child) != "argument_list"; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) && file.FlatType(child) != "type_arguments" {
				named = append(named, child)
			}
		}
		// The last named child before the arguments is the method name; a receiver expression comes before it.
		if len(named) >= 2 {
			c.ReceiverNode = named[0]
			c.ReceiverText = strings.TrimSpace(file.FlatNodeText(named[0]))
		}
		c.Args = javaArgumentExpressions(file, idx)
		c.Labels = make([]string, len(c.Args))
	default:
		return c, false
	}
	return c, true
}

// Arg returns the argument for the parameter at position index, honoring a Kotlin label equal to name.
func (c lintCall) Arg(index int, name string) uint32 {
	if name != "" {
		for i, label := range c.Labels {
			if label == name {
				return c.Args[i]
			}
		}
	}
	pos := 0
	for i, arg := range c.Args {
		if c.Labels[i] != "" {
			continue
		}
		if pos == index {
			return arg
		}
		pos++
	}
	return 0
}

var lintImportRe = regexp.MustCompile(`(?m)^\s*import\s+(static\s+)?([A-Za-z_][\w.]*?)(\.\*)?(?:\s+as\s+(\w+))?\s*;?\s*$`)

// lintImports is the import set of one source file.
type lintImports struct {
	// bySimple maps a simple or aliased name to the imported name (a class or a static member).
	bySimple map[string]string
	// stars holds the package or class prefixes of on-demand imports, without the trailing ".*".
	stars []string
}

func lintImportsOf(file *scanner.File) *lintImports {
	return filefacts.FileFact(fileFactsCache(), file, slotLintImports, func() *lintImports {
		imp := &lintImports{bySimple: map[string]string{}}
		for _, m := range lintImportRe.FindAllStringSubmatch(string(file.Content), -1) {
			full := m[2]
			if m[3] != "" {
				imp.stars = append(imp.stars, full)
				continue
			}
			simple := full[strings.LastIndex(full, ".")+1:]
			if m[4] != "" {
				simple = m[4]
			}
			imp.bySimple[simple] = full
		}
		return imp
	})
}

const slotLintImports = "lintImports"

// lintTypeRefMatches reports whether the source text names the class fqn: spelled in full, imported, or reached
// through an on-demand import. Nested classes also match through their outer class ("PowerManager.WakeLock").
func lintTypeRefMatches(file *scanner.File, text, fqn string) bool {
	text = strings.TrimSpace(text)
	if text == "" || fqn == "" {
		return false
	}
	text = strings.ReplaceAll(text, "?.", ".")
	if text == fqn {
		return true
	}
	imp := lintImportsOf(file)
	head, rest, _ := strings.Cut(text, ".")
	full, ok := imp.bySimple[head]
	if ok {
		resolved := full
		if rest != "" {
			resolved += "." + rest
		}
		return resolved == fqn
	}
	if !strings.HasSuffix(fqn, "."+text) {
		return false
	}
	pkg := strings.TrimSuffix(fqn, "."+text)
	for _, star := range imp.stars {
		if star == pkg {
			return true
		}
	}
	return false
}

// lintBinaryOps orders the integer operators of the small constant evaluator.
var lintBinaryOps = map[string]int{
	"or": 1, "and": 1, "xor": 1, "|": 1, "^": 2, "&": 3, "shl": 4, "shr": 4, "ushr": 4, "<<": 4, ">>": 4, ">>>": 4,
	"+": 5, "-": 5, "*": 6, "/": 6, "%": 6,
}

// lintEvalInt folds an integer expression made of literals, Kotlin infix and Java bitwise operators, and the
// named constants in consts (keyed by "package.Class.NAME"). It reports false when any part is not constant.
func lintEvalInt(file *scanner.File, idx uint32, consts map[string]int64) (int64, bool) {
	if idx == 0 {
		return 0, false
	}
	return lintEvalIntDepth(file, idx, idx, consts, 0)
}

func lintEvalIntDepth(file *scanner.File, idx, scope uint32, consts map[string]int64, depth int) (int64, bool) {
	if depth > 6 {
		return 0, false
	}
	p := &lintEvalParser{file: file, consts: consts, scope: scope, depth: depth, toks: lintEvalTokens(file.FlatNodeText(idx))}
	v, ok := p.expr(0)
	if !ok || p.pos != len(p.toks) {
		return 0, false
	}
	return v, true
}

func lintEvalTokens(s string) []string {
	var toks []string
	for i := 0; i < len(s); {
		ch := s[i]
		switch {
		case ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r':
			i++
		case ch >= '0' && ch <= '9':
			j := i
			for j < len(s) && lintIdentByte(s[j]) {
				j++
			}
			toks = append(toks, s[i:j])
			i = j
		case lintIdentByte(ch):
			j := i
			for j < len(s) && (lintIdentByte(s[j]) || s[j] == '.' && j+1 < len(s) && lintIdentByte(s[j+1])) {
				j++
			}
			toks = append(toks, s[i:j])
			i = j
		default:
			if strings.HasPrefix(s[i:], ">>>") || strings.HasPrefix(s[i:], "<<") || strings.HasPrefix(s[i:], ">>") {
				n := 2
				if strings.HasPrefix(s[i:], ">>>") {
					n = 3
				}
				toks = append(toks, s[i:i+n])
				i += n
				continue
			}
			toks = append(toks, string(ch))
			i++
		}
	}
	return toks
}

func lintIdentByte(b byte) bool {
	return b == '_' || b == '$' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

type lintEvalParser struct {
	file   *scanner.File
	consts map[string]int64
	toks   []string
	pos    int
	scope  uint32
	depth  int
}

func (p *lintEvalParser) expr(minPrec int) (int64, bool) {
	left, ok := p.unary()
	if !ok {
		return 0, false
	}
	for p.pos < len(p.toks) {
		op := p.toks[p.pos]
		prec, isOp := lintBinaryOps[op]
		if !isOp || prec < minPrec || prec == 0 {
			break
		}
		p.pos++
		right, ok := p.expr(prec + 1)
		if !ok {
			return 0, false
		}
		switch op {
		case "or", "|":
			left |= right
		case "and", "&":
			left &= right
		case "xor", "^":
			left ^= right
		case "shl", "<<":
			left <<= uint(right)
		case "shr", ">>":
			left >>= uint(right)
		case "ushr", ">>>":
			left = int64(uint32(left) >> uint(right))
		case "+":
			left += right
		case "-":
			left -= right
		case "*":
			left *= right
		case "/":
			if right == 0 {
				return 0, false
			}
			left /= right
		case "%":
			if right == 0 {
				return 0, false
			}
			left %= right
		}
	}
	return left, true
}

func (p *lintEvalParser) unary() (int64, bool) {
	if p.pos >= len(p.toks) {
		return 0, false
	}
	tok := p.toks[p.pos]
	switch {
	case tok == "(":
		p.pos++
		v, ok := p.expr(1)
		if !ok || p.pos >= len(p.toks) || p.toks[p.pos] != ")" {
			return 0, false
		}
		p.pos++
		return v, true
	case tok == "-" || tok == "+" || tok == "~":
		p.pos++
		v, ok := p.unary()
		if !ok {
			return 0, false
		}
		switch tok {
		case "-":
			return -v, true
		case "~":
			return ^v, true
		}
		return v, true
	case tok[0] >= '0' && tok[0] <= '9':
		p.pos++
		return parseKotlinIntegerLiteral(tok)
	case lintIdentByte(tok[0]):
		p.pos++
		return p.constant(tok)
	}
	return 0, false
}

func (p *lintEvalParser) constant(name string) (int64, bool) {
	qualifier, simple := "", name
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		qualifier, simple = name[:dot], name[dot+1:]
	}
	imp := lintImportsOf(p.file)
	if qualifier == "" {
		if full, ok := imp.bySimple[simple]; ok {
			v, found := p.consts[full]
			return v, found
		}
		for _, star := range imp.stars {
			if v, ok := p.consts[star+"."+simple]; ok {
				return v, true
			}
		}
		return p.resolveLocal(simple)
	}
	for full, v := range p.consts {
		cut := strings.LastIndex(full, ".")
		if full[cut+1:] == simple && lintTypeRefMatches(p.file, qualifier, full[:cut]) {
			return v, true
		}
	}
	if qualifier == "this" {
		return p.resolveLocal(simple)
	}
	return 0, false
}

// resolveLocal folds the initializer of a never-reassigned local, property, or final field declared in this file.
func (p *lintEvalParser) resolveLocal(name string) (int64, bool) {
	init := lintConstantInitializer(p.file, p.scope, name)
	if init == 0 {
		return 0, false
	}
	return lintEvalIntDepth(p.file, init, init, p.consts, p.depth+1)
}

// lintConstantInitializer returns the initializer node of the declaration of name visible from scope, or 0 when the
// name is not declared in the file, is mutable, or is assigned again.
func lintConstantInitializer(file *scanner.File, scope uint32, name string) uint32 {
	return lintInitializerOf(file, scope, name, false)
}

// lintInitializerOf is lintConstantInitializer that can also accept a mutable declaration, which still resolves when
// nothing in the file assigns it again (Lint's ConstantEvaluator follows such variables).
func lintInitializerOf(file *scanner.File, scope uint32, name string, allowMutable bool) uint32 {
	fn, hasFn := flatEnclosingCallable(file, scope)
	var local, member uint32
	var localRoot uint32
	consider := func(decl, init uint32, mutable bool) {
		if init == 0 {
			return
		}
		mutable = mutable && !allowMutable
		inFn := hasFn && file.FlatStartByte(decl) >= file.FlatStartByte(fn) && file.FlatEndByte(decl) <= file.FlatEndByte(fn)
		if inFn {
			if file.FlatStartByte(decl) < file.FlatStartByte(scope) && !mutable {
				local, localRoot = init, fn
			}
			return
		}
		if _, nested := flatEnclosingCallable(file, decl); nested {
			return
		}
		if !mutable {
			member = init
		}
	}
	file.FlatWalkNodes(0, "property_declaration", func(decl uint32) {
		vd, ok := file.FlatFindChild(decl, "variable_declaration")
		if !ok || lintDeclaredName(file, vd) != name {
			return
		}
		kind, _ := file.FlatFindChild(decl, "binding_pattern_kind")
		mutable := strings.TrimSpace(file.FlatNodeText(kind)) == "var"
		consider(decl, lintInitializerAfterEquals(file, decl), mutable)
	})
	file.FlatWalkNodes(0, "variable_declarator", func(decl uint32) {
		id, ok := file.FlatFindChild(decl, "identifier")
		if !ok || file.FlatNodeText(id) != name {
			return
		}
		parent, _ := file.FlatParent(decl)
		mutable := file.FlatType(parent) == "field_declaration" && !file.FlatHasModifier(parent, "final")
		consider(decl, lintInitializerAfterEquals(file, decl), mutable)
	})
	switch {
	case local != 0:
		if lintNameReassigned(file, localRoot, name) {
			return 0
		}
		return local
	case member != 0:
		if lintNameReassigned(file, 0, name) {
			return 0
		}
		return member
	}
	return 0
}

func lintDeclaredName(file *scanner.File, node uint32) string {
	if id, ok := file.FlatFindChild(node, "simple_identifier"); ok {
		return file.FlatNodeText(id)
	}
	return ""
}

func lintInitializerAfterEquals(file *scanner.File, decl uint32) uint32 {
	seen := false
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			seen = true
			continue
		}
		if seen && file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

// lintNameReassigned reports whether name is the target of an assignment or increment under root (the whole file when 0).
func lintNameReassigned(file *scanner.File, root uint32, name string) bool {
	found := false
	check := func(idx uint32) {
		if found {
			return
		}
		first := file.FlatNamedChild(idx, 0)
		if first != 0 && strings.TrimSpace(file.FlatNodeText(first)) == name {
			found = true
		}
	}
	for _, kind := range []string{"assignment", "assignment_expression", "update_expression"} {
		file.FlatWalkNodes(root, kind, check)
	}
	for _, kind := range []string{"postfix_expression", "prefix_expression"} {
		file.FlatWalkNodes(root, kind, func(idx uint32) {
			text := strings.TrimSpace(file.FlatNodeText(idx))
			if text == name+"++" || text == name+"--" || text == "++"+name || text == "--"+name {
				found = true
			}
		})
	}
	return found
}

// lintTargetSdk returns the project's target SDK, or 0 when Gradle metadata is unavailable.
func lintTargetSdk(ctx *api.Context) int {
	facts := librarymodel.EnsureFacts(ctx.LibraryFacts)
	if facts == nil {
		return 0
	}
	return facts.Profile.TargetSdkVersion
}

// lintAndroidRule builds the AndroidRule metadata shared by the Lint ports.
func lintAndroidRule(id, brief string, cat AndroidLintCategory, sev AndroidLintSeverity, prio int) AndroidRule {
	r := alcRule(id, brief, sev, prio)
	r.Category = cat
	return r
}

// lintFindingAtNode builds a finding spanning the node; severity overrides the rule default when non-empty.
func lintFindingAtNode(ctx *api.Context, idx uint32, severity, msg string) scanner.Finding {
	file := ctx.File
	return scanner.Finding{
		Line:      file.FlatRow(idx) + 1,
		Col:       file.FlatCol(idx) + 1,
		StartByte: int(file.FlatStartByte(idx)),
		EndByte:   int(file.FlatEndByte(idx)),
		Severity:  severity,
		Message:   msg,
	}
}

// lintCallNodeTypes are the node types lintCallAt understands.
var lintCallNodeTypes = []string{"call_expression", "method_invocation", "object_creation_expression"}

var lintInterpolationRe = regexp.MustCompile(`\$\{[^}]*\}|\$[A-Za-z_]\w*`)

// lintEvalString folds a string expression of literals, `+` concatenation, and constants declared in the file.
// Parts that are not constant are dropped, as in Lint's ConstantEvaluator with unknowns allowed; it reports false
// when no part is constant.
func lintEvalString(file *scanner.File, idx uint32) (string, bool) {
	return lintEvalStringDepth(file, idx, idx, 0)
}

func lintEvalStringDepth(file *scanner.File, idx, scope uint32, depth int) (string, bool) {
	if idx == 0 || depth > 6 {
		return "", false
	}
	switch file.FlatType(idx) {
	case "string_literal":
		return lintStringLiteralValue(file, idx), true
	case "parenthesized_expression":
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				return lintEvalStringDepth(file, child, scope, depth)
			}
		}
	case "additive_expression", "binary_expression":
		var sb strings.Builder
		known := false
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				if t := file.FlatNodeText(child); t != "+" {
					return "", false
				}
				continue
			}
			if part, ok := lintEvalStringDepth(file, child, scope, depth); ok {
				sb.WriteString(part)
				known = true
			}
		}
		return sb.String(), known
	case "simple_identifier", "identifier":
		if init := lintConstantInitializer(file, scope, file.FlatNodeText(idx)); init != 0 {
			return lintEvalStringDepth(file, init, init, depth+1)
		}
	}
	return "", false
}

// lintStringLiteralValue returns the literal's text without quotes and without Kotlin interpolations.
func lintStringLiteralValue(file *scanner.File, idx uint32) string {
	raw := file.FlatNodeText(idx)
	switch {
	case strings.HasPrefix(raw, `"""`) && strings.HasSuffix(raw, `"""`) && len(raw) >= 6:
		raw = raw[3 : len(raw)-3]
	case len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"':
		raw = raw[1 : len(raw)-1]
	}
	return lintInterpolationRe.ReplaceAllString(raw, "")
}

// lintLiteralContentOffset is the byte offset inside the literal node where its content starts.
func lintLiteralContentOffset(file *scanner.File, idx uint32) int {
	raw := file.FlatNodeText(idx)
	if strings.HasPrefix(raw, `"""`) {
		return 3
	}
	return 1
}

// lintFindingAtOffset builds a finding for the byte range [start,end) of the file.
func lintFindingAtOffset(file *scanner.File, start, end int, severity, msg string) scanner.Finding {
	line, col := 1, 1
	for i := 0; i < start && i < len(file.Content); i++ {
		if file.Content[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return scanner.Finding{Line: line, Col: col, StartByte: start, EndByte: end, Severity: severity, Message: msg}
}

// lintReceiverIsType reports whether the call's receiver is an instance of fqn, using resolved call targets and
// javac facts when present and the receiver's declared type in this file otherwise.
func lintReceiverIsType(ctx *api.Context, call lintCall, fqn string) bool {
	if call.ReceiverNode == 0 {
		return false
	}
	file := ctx.File
	simple := fqn[strings.LastIndex(fqn, ".")+1:]
	if semanticCallTargetOrReceiverType(ctx, call.Idx, []string{fqn}, []string{fqn, simple}) {
		return true
	}
	return lintTypeRefMatches(file, lintReceiverTypeText(file, call), fqn)
}

// lintReceiverTypeText names the receiver's type from its declaration or constructor expression, or "" when unknown.
func lintReceiverTypeText(file *scanner.File, call lintCall) string {
	text := strings.TrimSpace(call.ReceiverText)
	text = strings.TrimSuffix(strings.TrimSuffix(text, "!!"), "?")
	text = strings.TrimPrefix(text, "this.")
	switch file.FlatType(call.ReceiverNode) {
	case "object_creation_expression":
		if inner, ok := lintCallAt(file, call.ReceiverNode); ok {
			return inner.TypeText
		}
	case "call_expression":
		if inner, ok := lintCallAt(file, call.ReceiverNode); ok && inner.Name != "" && inner.Name[0] >= 'A' && inner.Name[0] <= 'Z' {
			return inner.TypeText
		}
	case "simple_identifier", "identifier":
		return lintDeclaredTypeText(file, call.Idx, text)
	}
	if lintIdentOnly(text) {
		return lintDeclaredTypeText(file, call.Idx, text)
	}
	return ""
}

func lintIdentOnly(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !lintIdentByte(s[i]) {
			return false
		}
	}
	return true
}

// lintDeclaredTypeText returns the declared (or constructor-inferred) type of the parameter, local, or field called
// name that is visible from scope, preferring the enclosing callable's own declarations.
func lintDeclaredTypeText(file *scanner.File, scope uint32, name string) string {
	fn, hasFn := flatEnclosingCallable(file, scope)
	var local, outer string
	record := func(decl uint32, typ string) {
		if typ == "" {
			return
		}
		if hasFn && file.FlatStartByte(decl) >= file.FlatStartByte(fn) && file.FlatEndByte(decl) <= file.FlatEndByte(fn) {
			if local == "" {
				local = typ
			}
			return
		}
		if outer == "" {
			outer = typ
		}
	}
	typeOf := func(node uint32) string { return lintChildTypeText(file, node) }
	initType := func(init uint32) string { return lintConstructorCallType(file, init) }
	for _, kind := range []string{"parameter", "class_parameter"} {
		file.FlatWalkNodes(0, kind, func(decl uint32) {
			if lintDeclaredName(file, decl) == name {
				record(decl, typeOf(decl))
			}
		})
	}
	file.FlatWalkNodes(0, "property_declaration", func(decl uint32) {
		vd, ok := file.FlatFindChild(decl, "variable_declaration")
		if !ok || lintDeclaredName(file, vd) != name {
			return
		}
		typ := typeOf(vd)
		if typ == "" {
			typ = initType(lintInitializerAfterEquals(file, decl))
		}
		record(decl, typ)
	})
	for _, kind := range []string{"formal_parameter", "enhanced_for_statement", "resource"} {
		file.FlatWalkNodes(0, kind, func(decl uint32) {
			if id, ok := file.FlatFindChild(decl, "identifier"); ok && file.FlatNodeText(id) == name {
				record(decl, typeOf(decl))
			}
		})
	}
	for _, kind := range []string{"field_declaration", "local_variable_declaration"} {
		file.FlatWalkNodes(0, kind, func(decl uint32) {
			for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) != "variable_declarator" {
					continue
				}
				id, ok := file.FlatFindChild(child, "identifier")
				if !ok || file.FlatNodeText(id) != name {
					continue
				}
				typ := typeOf(decl)
				if typ == "var" {
					typ = initType(lintInitializerAfterEquals(file, child))
				}
				record(decl, typ)
			}
		})
	}
	if local != "" {
		return local
	}
	return outer
}

// lintChildTypeText returns the first type child of node without type arguments or a trailing `?`.
func lintChildTypeText(file *scanner.File, node uint32) string {
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type", "type_identifier", "scoped_type_identifier", "generic_type":
			return lintTrimGenerics(file.FlatNodeText(child))
		case "nullable_type":
			return lintTrimGenerics(strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(child)), "?"))
		}
	}
	return ""
}

// lintConstructorCallType returns the type an initializer constructs when it is a call to a capitalized name.
func lintConstructorCallType(file *scanner.File, init uint32) string {
	if init == 0 {
		return ""
	}
	if inner, ok := lintCallAt(file, init); ok && inner.Name != "" && inner.Name[0] >= 'A' && inner.Name[0] <= 'Z' {
		return inner.TypeText
	}
	return ""
}

func lintTrimGenerics(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "<"); i >= 0 {
		s = s[:i]
	}
	return s
}

// lintEvalBool folds a boolean literal or a constant that is initialized with one.
func lintEvalBool(file *scanner.File, idx uint32) (value, ok bool) {
	return lintEvalBoolDepth(file, idx, idx, 0)
}

func lintEvalBoolDepth(file *scanner.File, idx, scope uint32, depth int) (value, ok bool) {
	if idx == 0 || depth > 6 {
		return false, false
	}
	switch file.FlatType(idx) {
	case "boolean_literal", "true", "false":
		return strings.TrimSpace(file.FlatNodeText(idx)) == "true", true
	case "parenthesized_expression":
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				return lintEvalBoolDepth(file, child, scope, depth)
			}
		}
	case "simple_identifier", "identifier":
		if init := lintConstantInitializer(file, scope, file.FlatNodeText(idx)); init != 0 {
			return lintEvalBoolDepth(file, init, init, depth+1)
		}
	}
	return false, false
}
