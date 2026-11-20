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

// typeDeclarationVisibleAt reports whether a declaration's enclosing scope (the file, or a class body or block)
// contains at, so the declaration can be named there without a qualifier.
func typeDeclarationVisibleAt(file *scanner.File, decl, at uint32) bool {
	container, ok := file.FlatParent(decl)
	for ok && !lintTypeScope(file.FlatType(container)) {
		container, ok = file.FlatParent(container)
	}
	if !ok || file.FlatType(container) == "source_file" {
		return true
	}
	// Forward references intentionally suppress more: false negatives are preferable to false positives.
	if lintNodeContains(file, container, at) {
		return true
	}
	if lintClassBody(file.FlatType(container)) {
		if companion, found := enclosingCompanionObject(file, container); found {
			if body, found := enclosingClassBody(file, companion); found {
				if lintNodeContains(file, body, at) {
					return true
				}
				if owner, found := file.FlatParent(body); found && visibleThroughSameFileSuperclass(file, owner, at) {
					return true
				}
			}
		}
		for owner, found := file.FlatParent(container); found; owner, found = file.FlatParent(owner) {
			if lintClassLike(file.FlatType(owner)) {
				return visibleThroughSameFileSuperclass(file, owner, at)
			}
			if !lintClassBody(file.FlatType(owner)) {
				break
			}
		}
	}
	return false
}

func lintClassBody(kind string) bool {
	switch kind {
	case "class_body", "enum_class_body", "interface_body", "enum_body", "enum_body_declarations":
		return true
	}
	return false
}

func lintTypeScope(kind string) bool {
	if lintClassBody(kind) {
		return true
	}
	switch kind {
	case "source_file", "program", "statements", "function_body", "block", "constructor_body", "lambda_literal", "lambda_expression":
		return true
	}
	return false
}

func lintClassLike(kind string) bool {
	switch kind {
	case "class_declaration", "object_declaration", "companion_object", "object_literal",
		"interface_declaration", "enum_declaration", "record_declaration", "object_creation_expression":
		return true
	}
	return false
}

func lintNodeContains(file *scanner.File, container, node uint32) bool {
	return file.FlatStartByte(container) <= file.FlatStartByte(node) && file.FlatEndByte(node) <= file.FlatEndByte(container)
}

func enclosingCompanionObject(file *scanner.File, node uint32) (uint32, bool) {
	for parent, ok := file.FlatParent(node); ok; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) == "companion_object" {
			return parent, true
		}
		if file.FlatType(parent) == "class_declaration" || file.FlatType(parent) == "source_file" {
			return 0, false
		}
	}
	return 0, false
}

func enclosingClassBody(file *scanner.File, node uint32) (uint32, bool) {
	for parent, ok := file.FlatParent(node); ok; parent, ok = file.FlatParent(parent) {
		if lintClassBody(file.FlatType(parent)) {
			return parent, true
		}
		if file.FlatType(parent) == "source_file" {
			return 0, false
		}
	}
	return 0, false
}

func visibleThroughSameFileSuperclass(file *scanner.File, declaration, at uint32) bool {
	classesByName := make(map[string][]uint32)
	typeAliasesByName := make(map[string]string)
	file.FlatWalkAllNodes(0, func(class uint32) {
		kind := file.FlatType(class)
		if kind == "type_alias" {
			name := lintDeclTypeName(file, class)
			target := file.FlatNamedChild(class, file.FlatNamedChildCount(class)-1)
			if name != "" && target != 0 {
				targetName := strings.TrimSuffix(lintTrimGenerics(file.FlatNodeText(target)), "?")
				if dot := strings.LastIndexByte(targetName, '.'); dot >= 0 {
					targetName = targetName[dot+1:]
				}
				if targetName != "" {
					typeAliasesByName[name] = targetName
				}
			}
			return
		}
		if !lintClassLike(kind) || kind == "object_literal" || kind == "object_creation_expression" {
			return
		}
		if name := lintDeclTypeName(file, class); name != "" {
			classesByName[name] = append(classesByName[name], class)
		}
	})
	visited := make(map[uint32]bool)
	var inherits func(uint32) bool
	inherits = func(class uint32) bool {
		if class == declaration {
			return true
		}
		if visited[class] {
			return false
		}
		visited[class] = true
		supers := directSuperNames(file, class)
		if file.Language == scanner.LangJava {
			supers = lintSupertypeNames(file, class)
		}
		for _, superName := range supers {
			// Simple-name matches intentionally suppress more: false negatives are preferable to false positives.
			if dot := strings.LastIndexByte(superName, '.'); dot >= 0 {
				superName = superName[dot+1:]
			}
			seenAliases := make(map[string]bool)
			for superName != "" && !seenAliases[superName] {
				seenAliases[superName] = true
				for _, superClass := range classesByName[superName] {
					if inherits(superClass) {
						return true
					}
				}
				superName = typeAliasesByName[superName]
			}
		}
		return false
	}
	for current, ok := file.FlatParent(at); ok; current, ok = file.FlatParent(current) {
		if lintClassLike(file.FlatType(current)) && inherits(current) {
			return true
		}
	}
	return false
}

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
			// A class declared in the file shadows a wildcard import of the same simple name.
			return lintSameFileClass(file, head) == 0
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
	// lenient folds more than a plain constant: it ignores Int conversions and resolves `Class.NAME` constants
	// declared in this file, as Lint's constant evaluators do. It also rejects a non-final Java local.
	lenient bool
	// mutableLocals follows a mutable local that nothing reassigns, as Lint's ConstantEvaluator does.
	mutableLocals bool
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
	if p.lenient {
		p.skipIntCast()
	}
	v, ok := p.primary()
	if ok && p.lenient {
		p.skipIntConversion()
	}
	return v, ok
}

// skipIntCast consumes a Java `(int)` cast prefix.
func (p *lintEvalParser) skipIntCast() {
	if p.pos+2 < len(p.toks) && p.toks[p.pos] == "(" && p.toks[p.pos+1] == "int" && p.toks[p.pos+2] == ")" {
		p.pos += 3
	}
}

// skipIntConversion consumes a Kotlin `.toInt()` call or `as Int` cast after an operand.
func (p *lintEvalParser) skipIntConversion() {
	for p.pos < len(p.toks) {
		switch {
		case p.pos+3 < len(p.toks) && p.toks[p.pos] == "." && p.toks[p.pos+1] == "toInt" && p.toks[p.pos+2] == "(" && p.toks[p.pos+3] == ")":
			p.pos += 4
		case p.pos+1 < len(p.toks) && p.toks[p.pos] == "as" && p.toks[p.pos+1] == "Int":
			p.pos += 2
		default:
			return
		}
	}
}

func (p *lintEvalParser) primary() (int64, bool) {
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
	if p.lenient {
		return p.resolveQualified(qualifier, simple)
	}
	return 0, false
}

// resolveQualified folds the initializer of a constant `qualifier.name` declared in this file.
func (p *lintEvalParser) resolveQualified(qualifier, name string) (int64, bool) {
	init := lintQualifiedConstantInitializer(p.file, qualifier, name)
	if init == 0 || p.depth+1 > 6 {
		return 0, false
	}
	sub := &lintEvalParser{file: p.file, consts: p.consts, scope: init, depth: p.depth + 1,
		toks: lintEvalTokens(p.file.FlatNodeText(init)), lenient: true}
	v, ok := sub.expr(0)
	if !ok || sub.pos != len(sub.toks) {
		return 0, false
	}
	return v, true
}

// resolveLocal folds the initializer of a never-reassigned local, property, or final field declared in this file.
func (p *lintEvalParser) resolveLocal(name string) (int64, bool) {
	init := lintConstantInitializer(p.file, p.scope, name)
	if init == 0 && p.mutableLocals {
		init = lintMutableLocalInitializer(p.file, p.scope, name)
	}
	if init != 0 && p.lenient && !p.mutableLocals && lintIsNonFinalJavaLocal(p.file, init) {
		return 0, false
	}
	if init == 0 || p.depth+1 > 6 {
		return 0, false
	}
	sub := &lintEvalParser{file: p.file, consts: p.consts, scope: init, depth: p.depth + 1,
		toks: lintEvalTokens(p.file.FlatNodeText(init)), lenient: p.lenient, mutableLocals: p.mutableLocals}
	v, ok := sub.expr(0)
	if !ok || sub.pos != len(sub.toks) {
		return 0, false
	}
	return v, true
}

// lintIsNonFinalJavaLocal reports whether the initializer belongs to a Java local variable declared without `final`.
func lintIsNonFinalJavaLocal(file *scanner.File, init uint32) bool {
	declarator, ok := file.FlatParent(init)
	if !ok || file.FlatType(declarator) != "variable_declarator" {
		return false
	}
	decl, ok := file.FlatParent(declarator)
	return ok && file.FlatType(decl) == "local_variable_declaration" && !file.FlatHasModifier(decl, "final")
}

// lintMutableLocalInitializer returns the initializer of a mutable local called name that nothing reassigns, or 0
// when the name is not a local of the enclosing callable.
func lintMutableLocalInitializer(file *scanner.File, scope uint32, name string) uint32 {
	fn, ok := flatEnclosingCallable(file, scope)
	if !ok {
		return 0
	}
	init := lintInitializerOf(file, scope, name, true)
	if init == 0 || file.FlatStartByte(init) < file.FlatStartByte(fn) || file.FlatEndByte(init) > file.FlatEndByte(fn) {
		return 0
	}
	return init
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
	// A parameter or an earlier local of the same name hides the member declaration.
	shadowed := lintNameIsParameterAt(file, scope, name)
	consider := func(decl, init uint32, mutable bool) {
		mutable = mutable && !allowMutable
		inFn := hasFn && file.FlatStartByte(decl) >= file.FlatStartByte(fn) && file.FlatEndByte(decl) <= file.FlatEndByte(fn)
		if inFn {
			if file.FlatStartByte(decl) < file.FlatStartByte(scope) {
				shadowed = true
				if !mutable && init != 0 {
					local, localRoot = init, fn
				}
			}
			return
		}
		if init == 0 {
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
	case member != 0 && !shadowed:
		if lintNameReassigned(file, 0, name) {
			return 0
		}
		return member
	}
	return 0
}

// lintNameIsParameterAt reports whether name is a parameter of a function, constructor or lambda enclosing idx.
func lintNameIsParameterAt(file *scanner.File, idx uint32, name string) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		var params uint32
		switch file.FlatType(cur) {
		case "function_declaration", "secondary_constructor", "anonymous_function":
			params, _ = file.FlatFindChild(cur, "function_value_parameters")
		case "lambda_literal":
			params, _ = file.FlatFindChild(cur, "lambda_parameters")
		case "method_declaration", "constructor_declaration":
			params, _ = file.FlatFindChild(cur, "formal_parameters")
		case "lambda_expression":
			params = file.FlatNamedChild(cur, 0)
		default:
			continue
		}
		if params != 0 && lintParamsDeclare(file, params, name) {
			return true
		}
	}
	return false
}

// lintParamsDeclare reports whether a parameter list, or a single Java lambda parameter, declares name.
func lintParamsDeclare(file *scanner.File, params uint32, name string) bool {
	switch file.FlatType(params) {
	case "identifier":
		return file.FlatNodeText(params) == name
	case "inferred_parameters":
		for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
			if file.FlatType(p) == "identifier" && file.FlatNodeText(p) == name {
				return true
			}
		}
		return false
	}
	for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
		switch file.FlatType(p) {
		case "parameter", "variable_declaration":
			if lintDeclaredName(file, p) == name {
				return true
			}
		case "formal_parameter":
			if id, ok := file.FlatFindChild(p, "identifier"); ok && file.FlatNodeText(id) == name {
				return true
			}
		}
	}
	return false
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

// lintInterpolationRe matches a Kotlin string template entry at the start of its input.
var lintInterpolationRe = regexp.MustCompile(`^(?:\$\{[^}]*\}|\$[A-Za-z_]\w*)`)

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

// lintStringLiteralValue returns the literal's text without quotes and without Kotlin interpolations. In a Kotlin
// string that is not raw, `\$` is an escaped dollar sign, not the start of an interpolation, so `"%1\$d"` reads as
// `%1$d`. Java has no interpolation, so a Java literal keeps every `$`.
func lintStringLiteralValue(file *scanner.File, idx uint32) string {
	raw := file.FlatNodeText(idx)
	rawString := false
	switch {
	case strings.HasPrefix(raw, `"""`) && strings.HasSuffix(raw, `"""`) && len(raw) >= 6:
		raw = raw[3 : len(raw)-3]
		rawString = true
	case len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"':
		raw = raw[1 : len(raw)-1]
	}
	if file.Language == scanner.LangJava {
		return raw
	}
	var sb strings.Builder
	for i := 0; i < len(raw); {
		if !rawString && raw[i] == '\\' && i+1 < len(raw) {
			if raw[i+1] == '$' {
				sb.WriteByte('$')
			} else {
				sb.WriteString(raw[i : i+2])
			}
			i += 2
			continue
		}
		if raw[i] == '$' {
			if loc := lintInterpolationRe.FindStringIndex(raw[i:]); loc != nil {
				i += loc[1]
				continue
			}
		}
		sb.WriteByte(raw[i])
		i++
	}
	return sb.String()
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
	receiverTypes := []string{fqn}
	// The simple name stands for fqn unless an import or a class of this file gives it to another type.
	if imported, ok := lintImportsOf(file).bySimple[simple]; (!ok || imported == fqn) && lintSameFileClass(file, simple) == 0 {
		receiverTypes = append(receiverTypes, simple)
	}
	if semanticCallTargetOrReceiverType(ctx, call.Idx, []string{fqn}, receiverTypes) {
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
	case "unary_expression", "prefix_expression":
		if operand := lintNegatedOperand(file, idx); operand != 0 {
			v, ok := lintEvalBoolDepth(file, operand, scope, depth+1)
			return !v, ok
		}
	}
	return false, false
}

// lintNegatedOperand returns the operand of a `!x` expression, or 0 for any other unary or prefix operator.
func lintNegatedOperand(file *scanner.File, idx uint32) uint32 {
	negated := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "!" {
			negated = true
		} else if negated && file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

// lintQualifiedConstantInitializer returns the initializer of the read-only property or final field called name that
// the class (or Kotlin companion) named by the last segment of qualifier declares in this file, or 0.
func lintQualifiedConstantInitializer(file *scanner.File, qualifier, name string) uint32 {
	owner := qualifier[strings.LastIndex(qualifier, ".")+1:]
	var found uint32
	match := func(decl uint32) bool {
		for cur, ok := file.FlatParent(decl); ok; cur, ok = file.FlatParent(cur) {
			switch file.FlatType(cur) {
			case "class_declaration", "object_declaration":
				return lintDeclTypeName(file, cur) == owner
			case "companion_object":
				if lintDeclTypeName(file, cur) == owner {
					return true
				}
				parent, ok := file.FlatParent(cur)
				for ok && file.FlatType(parent) != "class_declaration" && file.FlatType(parent) != "object_declaration" {
					parent, ok = file.FlatParent(parent)
				}
				return ok && lintDeclTypeName(file, parent) == owner
			}
		}
		return false
	}
	file.FlatWalkNodes(0, "property_declaration", func(decl uint32) {
		vd, ok := file.FlatFindChild(decl, "variable_declaration")
		if !ok || found != 0 || lintDeclaredName(file, vd) != name {
			return
		}
		if kind, _ := file.FlatFindChild(decl, "binding_pattern_kind"); strings.TrimSpace(file.FlatNodeText(kind)) != "val" {
			return
		}
		if match(decl) {
			found = lintInitializerAfterEquals(file, decl)
		}
	})
	file.FlatWalkNodes(0, "variable_declarator", func(decl uint32) {
		id, ok := file.FlatFindChild(decl, "identifier")
		parent, _ := file.FlatParent(decl)
		if found != 0 || !ok || file.FlatNodeText(id) != name || file.FlatType(parent) != "field_declaration" || !file.FlatHasModifier(parent, "final") {
			return
		}
		if match(decl) {
			found = lintInitializerAfterEquals(file, decl)
		}
	})
	return found
}

// lintDeclTypeName returns the name of a class, object, or companion declaration; an unnamed companion is "Companion".
func lintDeclTypeName(file *scanner.File, decl uint32) string {
	for _, kind := range []string{"type_identifier", "identifier"} {
		if n, ok := file.FlatFindChild(decl, kind); ok {
			return file.FlatNodeText(n)
		}
	}
	if file.FlatType(decl) == "companion_object" {
		return "Companion"
	}
	return ""
}
