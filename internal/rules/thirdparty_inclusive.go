package rules

// Port of slack-lints' InclusiveNamingSourceCodeScanner (slackhq/slack-lints 0.11.1,
// slack-lint-checks/src/main/java/slack/lint/inclusive/): InclusiveNaming reports a name that contains a word of
// the configured block list (Lint's `block-list` option, here blockList), with nothing to report when the list is
// empty. A name matches when its lower-cased text contains a list entry as written, so an entry with upper-case
// letters never matches, as in Lint; the first matching entry in list order is reported.
//
// Lint checks the names of the file, its package, and the UAST declarations of the file's light classes: classes,
// methods, fields, parameters, local variables and labels. A Kotlin property therefore shows up as its backing field
// ("property") and its getter ("function", named getX), and each finding sits where Lint's location for that element
// starts: a class or property at its KDoc, a function at its name. Locations Lint would report twice are reported
// once. The resource-file half of the detector (XML attribute names) is not ported.

import (
	"bytes"
	"slices"
	"strings"
	"sync/atomic"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const slackInclusiveNaming = "InclusiveNaming"

// InclusiveNamingRule reports names containing a block-listed word; BlockList is its blockList option.
type InclusiveNamingRule struct {
	FlatDispatchBase
	BaseRule
	BlockList []string

	cache atomic.Pointer[inclusiveWords]
}

// Confidence reports a high base confidence: a finding is a substring match on a declared name.
func (r *InclusiveNamingRule) Confidence() float64 { return api.ConfidenceHigh }

// words returns the block list, split on commas as Lint splits its option, in order. The split list is cached
// until the option changes.
func (r *InclusiveNamingRule) words() []string {
	if len(r.BlockList) == 0 {
		return nil
	}
	if c := r.cache.Load(); c != nil && slices.Equal(c.src, r.BlockList) {
		return c.words
	}
	words := splitInclusiveBlockList(r.BlockList)
	r.cache.Store(&inclusiveWords{src: slices.Clone(r.BlockList), words: words})
	return words
}

type inclusiveWords struct{ src, words []string }

func splitInclusiveBlockList(list []string) []string {
	var out []string
	for _, entry := range list {
		for _, w := range strings.Split(entry, ",") {
			if w = strings.TrimSpace(w); w != "" {
				out = append(out, w)
			}
		}
	}
	return out
}

func (r *InclusiveNamingRule) check(ctx *api.Context) {
	words := r.words()
	if len(words) == 0 {
		return
	}
	s := &inclusiveScan{ctx: ctx, words: words}
	if ctx.File.Language == scanner.LangJava {
		s.java(ctx.Idx)
	} else {
		s.kotlin(ctx.Idx)
	}
}

type inclusiveScan struct {
	ctx   *api.Context
	words []string
}

// report emits a finding for name at node (or, with node 0, at the byte offset at) when it contains a listed word.
func (s *inclusiveScan) report(name, kind string, node uint32, at int) {
	w := s.match(name)
	if w == "" {
		return
	}
	msg := "Use inclusive naming. Matched string is '" + w + "' in " + kind + " name '" + name + "'"
	if node != 0 {
		s.ctx.EmitAtNode(node, msg)
	} else {
		line, col := inclusiveLineCol(s.ctx.File, at)
		s.ctx.EmitAt(line, col, msg)
	}
}

// match returns the first listed word the lower-cased name contains, or "".
func (s *inclusiveScan) match(name string) string {
	if name == "" {
		return ""
	}
	lower := strings.ToLower(name)
	for _, w := range s.words {
		if strings.Contains(lower, w) {
			return w
		}
	}
	return ""
}

// reportAtDecl reports name at a declaration, starting at its doc comment when one directly precedes it.
func (s *inclusiveScan) reportAtDecl(name, kind string, decl uint32) {
	file := s.ctx.File
	start := int(file.FlatStartByte(decl))
	if doc := inclusiveDocStart(file, start); doc >= 0 {
		start = doc
	}
	s.report(name, kind, 0, start)
}

// inclusiveKotlinChecks and inclusiveJavaChecks map each dispatched node type to its check.
var inclusiveKotlinChecks = map[string]func(*inclusiveScan, uint32){
	"source_file":           (*inclusiveScan).kotlinFile,
	"class_declaration":     (*inclusiveScan).kotlinClass,
	"object_declaration":    (*inclusiveScan).kotlinClass,
	"companion_object":      (*inclusiveScan).kotlinCompanion,
	"enum_entry":            func(s *inclusiveScan, n uint32) { s.reportChild(n, "simple_identifier", "property", n) },
	"function_declaration":  (*inclusiveScan).kotlinFunction,
	"secondary_constructor": (*inclusiveScan).kotlinParameters,
	"anonymous_function":    (*inclusiveScan).kotlinParameters,
	"property_declaration":  (*inclusiveScan).kotlinProperty,
	"setter":                (*inclusiveScan).kotlinSetter,
	"lambda_parameters":     (*inclusiveScan).kotlinLambdaParameters,
	"catch_block":           func(s *inclusiveScan, n uint32) { s.reportChild(n, "simple_identifier", "parameter", 0) },
	"when_subject":          (*inclusiveScan).kotlinWhenSubject,
	"label":                 (*inclusiveScan).kotlinLabel,
}

func (s *inclusiveScan) kotlin(idx uint32) {
	if check := inclusiveKotlinChecks[s.ctx.File.FlatType(idx)]; check != nil {
		check(s, idx)
	}
}

// reportChild reports the text of parent's first childType child as a kind name, at the node at, or at the child
// when at is 0.
func (s *inclusiveScan) reportChild(parent uint32, childType, kind string, at uint32) {
	id, ok := s.ctx.File.FlatFindChild(parent, childType)
	if !ok {
		return
	}
	if at == 0 {
		at = id
	}
	s.report(s.ctx.File.FlatNodeText(id), kind, at, 0)
}

func (s *inclusiveScan) kotlinFile(idx uint32) {
	file := s.ctx.File
	s.report(inclusiveBaseName(file.Path), "file", 0, 0)
	if pkg, ok := file.FlatFindChild(idx, "package_header"); ok {
		if id, ok := file.FlatFindChild(pkg, "identifier"); ok {
			s.report(strings.Join(strings.Fields(file.FlatNodeText(id)), ""), "package", 0, 0)
		}
	}
}

func (s *inclusiveScan) kotlinClass(idx uint32) {
	file := s.ctx.File
	if id, ok := file.FlatFindChild(idx, "type_identifier"); ok {
		s.reportAtDecl(file.FlatNodeText(id), "class", idx)
	}
	if file.FlatType(idx) == "class_declaration" {
		s.kotlinConstructorParameters(idx)
	}
}

// kotlinCompanion reports a companion object once, as its field: the field has the class's name and location.
func (s *inclusiveScan) kotlinCompanion(idx uint32) {
	file := s.ctx.File
	name := "Companion"
	if id, ok := file.FlatFindChild(idx, "type_identifier"); ok {
		name = file.FlatNodeText(id)
	}
	s.reportAtDecl(name, "property", idx)
}

func (s *inclusiveScan) kotlinFunction(idx uint32) {
	if inclusiveIsLocal(s.ctx.File, idx) {
		// A local function is not a UMethod.
		return
	}
	s.reportChild(idx, "simple_identifier", "function", 0)
	s.kotlinParameters(idx)
}

func (s *inclusiveScan) kotlinSetter(idx uint32) {
	if p, ok := s.ctx.File.FlatFindChild(idx, "parameter_with_optional_type"); ok {
		s.reportChild(p, "simple_identifier", "parameter", 0)
	}
}

// kotlinLambdaParameters reports named lambda parameters; destructured ones are not UParameters with a name.
func (s *inclusiveScan) kotlinLambdaParameters(idx uint32) {
	file := s.ctx.File
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "variable_declaration" {
			s.reportChild(c, "simple_identifier", "parameter", 0)
		}
	}
}

func (s *inclusiveScan) kotlinWhenSubject(idx uint32) {
	file := s.ctx.File
	v, ok := file.FlatFindChild(idx, "variable_declaration")
	if !ok {
		return
	}
	at := idx
	if kw, ok := file.FlatFindChild(idx, "val"); ok {
		at = kw
	}
	s.report(extractIdentifierFlat(file, v), "local variable", at, 0)
}

// kotlinLabel reports a label definition (`name@ for`, `name@{ }`), not a `return@name` reference.
func (s *inclusiveScan) kotlinLabel(idx uint32) {
	file := s.ctx.File
	if parent, ok := file.FlatParent(idx); ok && file.FlatType(parent) == "jump_expression" {
		return
	}
	s.report(strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(idx)), "@"), "label", idx, 0)
}

// kotlinParameters reports a function's or constructor's value parameters at their start.
func (s *inclusiveScan) kotlinParameters(fn uint32) {
	file := s.ctx.File
	params, ok := file.FlatFindChild(fn, "function_value_parameters")
	if !ok {
		return
	}
	for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
		if file.FlatType(p) != "parameter" {
			continue
		}
		at := p
		if prev, ok := file.FlatPrevSibling(p); ok && file.FlatType(prev) == "parameter_modifiers" {
			at = prev
		}
		s.report(extractIdentifierFlat(file, p), "parameter", at, 0)
	}
}

// kotlinConstructorParameters reports a class's primary constructor parameters: a `val`/`var` one as its property
// (and getter), a plain one as a parameter. For a data class, the generated copy() takes every property again, at
// the location of the parameter list, so the first matching name is reported there once more.
func (s *inclusiveScan) kotlinConstructorParameters(cls uint32) {
	file := s.ctx.File
	ctor, ok := file.FlatFindChild(cls, "primary_constructor")
	if !ok {
		return
	}
	list := ctor
	if params, ok := file.FlatFindChild(ctor, "class_parameters"); ok {
		list = params
	}
	var copyNames []string
	for p := file.FlatFirstChild(list); p != 0; p = file.FlatNextSib(p) {
		if file.FlatType(p) != "class_parameter" {
			continue
		}
		id, ok := file.FlatFindChild(p, "simple_identifier")
		if !ok {
			continue
		}
		name := file.FlatNodeText(id)
		if _, isProperty := file.FlatFindChild(p, "binding_pattern_kind"); !isProperty {
			s.report(name, "parameter", p, 0)
			continue
		}
		copyNames = append(copyNames, name)
		s.report(name, "property", p, 0)
		if !inclusiveHasModifier(file, p, "private") && !inclusiveHasAnnotation(file, p, "JvmField") {
			s.report(inclusiveGetterName(name), "function", id, 0)
		}
	}
	if inclusiveHasModifier(file, cls, "data") {
		if open := inclusiveFirstToken(file, list, "("); open != 0 {
			for _, name := range copyNames {
				if s.match(name) != "" {
					s.report(name, "parameter", open, 0)
					break
				}
			}
		}
	}
}

// kotlinProperty reports a property: a local variable, or a member or top-level property's backing field and
// accessor methods as Lint's light classes have them.
func (s *inclusiveScan) kotlinProperty(prop uint32) {
	file := s.ctx.File
	if inclusiveIsLocal(file, prop) {
		if multi, ok := file.FlatFindChild(prop, "multi_variable_declaration"); ok {
			for v := file.FlatFirstChild(multi); v != 0; v = file.FlatNextSib(v) {
				if file.FlatType(v) == "variable_declaration" {
					if id, ok := file.FlatFindChild(v, "simple_identifier"); ok {
						s.report(file.FlatNodeText(id), "local variable", id, 0)
					}
				}
			}
			return
		}
		if v, ok := file.FlatFindChild(prop, "variable_declaration"); ok {
			s.report(extractIdentifierFlat(file, v), "local variable", prop, 0)
		}
		return
	}
	v, ok := file.FlatFindChild(prop, "variable_declaration")
	if !ok {
		return
	}
	id, ok := file.FlatFindChild(v, "simple_identifier")
	if !ok {
		return
	}
	name := file.FlatNodeText(id)
	receiver := inclusivePropertyReceiver(file, prop)
	_, delegated := file.FlatFindChild(prop, "property_delegate")
	abstract := inclusiveHasModifier(file, prop, "abstract") || inclusiveInInterface(file, prop)
	if !abstract && receiver == 0 {
		hasField := delegated || inclusiveHasModifier(file, prop, "lateinit") || lintInitializerAfterEquals(file, prop) != 0 ||
			inclusiveAccessorUsesField(file, prop)
		if hasField {
			fieldName := name
			if delegated {
				fieldName += "$delegate"
			}
			s.reportAtDecl(fieldName, "property", prop)
		}
	}
	if receiver != 0 {
		// An extension property's getter takes its receiver as `$this$name`, located at the receiver type.
		s.report("$this$"+name, "parameter", receiver, 0)
	}
	if inclusiveHasModifier(file, prop, "private") || inclusiveHasModifier(file, prop, "const") ||
		inclusiveHasAnnotation(file, prop, "JvmField") {
		return
	}
	s.report(inclusiveGetterName(name), "function", id, 0)
}

var inclusiveJavaChecks = map[string]func(*inclusiveScan, uint32){
	"program":                             (*inclusiveScan).javaFile,
	"class_declaration":                   (*inclusiveScan).javaClass,
	"interface_declaration":               (*inclusiveScan).javaClass,
	"enum_declaration":                    (*inclusiveScan).javaClass,
	"annotation_type_declaration":         (*inclusiveScan).javaClass,
	"record_declaration":                  (*inclusiveScan).javaClass,
	"method_declaration":                  func(s *inclusiveScan, n uint32) { s.reportChild(n, "identifier", "method", 0) },
	"annotation_type_element_declaration": func(s *inclusiveScan, n uint32) { s.reportChild(n, "identifier", "method", 0) },
	"field_declaration":                   func(s *inclusiveScan, n uint32) { s.javaVariables(n, "field") },
	"local_variable_declaration":          func(s *inclusiveScan, n uint32) { s.javaVariables(n, "local variable") },
	"resource":                            func(s *inclusiveScan, n uint32) { s.reportChild(n, "identifier", "local variable", n) },
	"enum_constant":                       func(s *inclusiveScan, n uint32) { s.reportChild(n, "identifier", "field", 0) },
	"formal_parameter":                    (*inclusiveScan).javaParameter,
	"spread_parameter":                    (*inclusiveScan).javaParameter,
	"catch_formal_parameter":              (*inclusiveScan).javaParameter,
	"lambda_expression":                   (*inclusiveScan).javaLambdaParameters,
	"labeled_statement":                   func(s *inclusiveScan, n uint32) { s.reportChild(n, "identifier", "label", n) },
}

func (s *inclusiveScan) java(idx uint32) {
	if check := inclusiveJavaChecks[s.ctx.File.FlatType(idx)]; check != nil {
		check(s, idx)
	}
}

func (s *inclusiveScan) javaFile(idx uint32) {
	file := s.ctx.File
	s.report(inclusiveBaseName(file.Path), "file", 0, 0)
	if pkg, ok := file.FlatFindChild(idx, "package_declaration"); ok {
		for c := file.FlatFirstChild(pkg); c != 0; c = file.FlatNextSib(c) {
			if t := file.FlatType(c); t == "scoped_identifier" || t == "identifier" {
				s.report(strings.Join(strings.Fields(file.FlatNodeText(c)), ""), "package", 0, 0)
			}
		}
	}
}

func (s *inclusiveScan) javaClass(idx uint32) {
	if id, ok := s.ctx.File.FlatFindChild(idx, "identifier"); ok {
		s.reportAtDecl(s.ctx.File.FlatNodeText(id), "class", idx)
	}
}

// javaVariables reports a field or local declaration's variables: the first at the declaration's location (a
// field's starting at its Javadoc), each later one at its name.
func (s *inclusiveScan) javaVariables(idx uint32, kind string) {
	file := s.ctx.File
	first := true
	for d := file.FlatFirstChild(idx); d != 0; d = file.FlatNextSib(d) {
		if file.FlatType(d) != "variable_declarator" {
			continue
		}
		id, ok := file.FlatFindChild(d, "identifier")
		if !ok {
			continue
		}
		switch {
		case !first:
			s.report(file.FlatNodeText(id), kind, id, 0)
		case kind == "field":
			s.reportAtDecl(file.FlatNodeText(id), kind, idx)
		default:
			s.report(file.FlatNodeText(id), kind, idx, 0)
		}
		first = false
	}
}

func (s *inclusiveScan) javaParameter(idx uint32) {
	file := s.ctx.File
	var name string
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "identifier":
			name = file.FlatNodeText(c)
		case "variable_declarator":
			if id, ok := file.FlatFindChild(c, "identifier"); ok {
				name = file.FlatNodeText(id)
			}
		}
	}
	s.report(name, "parameter", idx, 0)
}

// javaLambdaParameters reports an inferred lambda parameter, `x -> …` or `(x, y) -> …`.
func (s *inclusiveScan) javaLambdaParameters(idx uint32) {
	file := s.ctx.File
	for c := file.FlatFirstChild(idx); c != 0 && file.FlatType(c) != "->"; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "identifier":
			s.report(file.FlatNodeText(c), "parameter", c, 0)
		case "inferred_parameters":
			for p := file.FlatFirstChild(c); p != 0; p = file.FlatNextSib(p) {
				if file.FlatType(p) == "identifier" {
					s.report(file.FlatNodeText(p), "parameter", p, 0)
				}
			}
		}
	}
}

// inclusiveIsLocal reports whether a Kotlin declaration is inside a function, accessor, lambda or initializer body.
func inclusiveIsLocal(file *scanner.File, decl uint32) bool {
	for p, ok := file.FlatParent(decl); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_body", "lambda_literal", "anonymous_initializer", "getter", "setter", "statements",
			"control_structure_body":
			return true
		case "class_body", "source_file", "enum_class_body":
			return false
		}
	}
	return false
}

func inclusiveInInterface(file *scanner.File, decl uint32) bool {
	body, ok := file.FlatParent(decl)
	if !ok || file.FlatType(body) != "class_body" {
		return false
	}
	cls, ok := file.FlatParent(body)
	if !ok || file.FlatType(cls) != "class_declaration" {
		return false
	}
	_, isInterface := file.FlatFindChild(cls, "interface")
	return isInterface
}

func inclusiveHasModifier(file *scanner.File, decl uint32, modifier string) bool {
	return file.FlatFindModifierNode(decl, modifier) != 0
}

func inclusiveHasAnnotation(file *scanner.File, decl uint32, name string) bool {
	mods, ok := file.FlatFindChild(decl, "modifiers")
	if !ok {
		return false
	}
	found := false
	file.FlatWalkNodes(mods, "type_identifier", func(n uint32) {
		if file.FlatNodeText(n) == name {
			found = true
		}
	})
	return found
}

// inclusivePropertyReceiver returns the receiver type of an extension property, or 0.
func inclusivePropertyReceiver(file *scanner.File, prop uint32) uint32 {
	for c := file.FlatFirstChild(prop); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "user_type", "nullable_type", "function_type", "parenthesized_type":
			return c
		case "variable_declaration":
			return 0
		}
	}
	return 0
}

// inclusiveAccessorUsesField reports whether a property's custom accessor reads or writes its backing `field`. The
// parser can put a setter on the next line beside the property instead of inside it.
func inclusiveAccessorUsesField(file *scanner.File, prop uint32) bool {
	uses := false
	check := func(acc uint32) {
		file.FlatWalkNodes(acc, "simple_identifier", func(n uint32) {
			if file.FlatNodeText(n) == "field" {
				uses = true
			}
		})
	}
	hasAccessor := false
	for c := file.FlatFirstChild(prop); c != 0; c = file.FlatNextSib(c) {
		if t := file.FlatType(c); t == "getter" || t == "setter" {
			hasAccessor = true
			check(c)
		}
	}
	if next, ok := file.FlatNextSibling(prop); ok && file.FlatType(next) == "setter" {
		hasAccessor = true
		check(next)
	}
	// A property with no initializer and no accessor is abstract or an error, and has no field either way.
	return hasAccessor && uses
}

func inclusiveGetterName(name string) string {
	if len(name) > 2 && strings.HasPrefix(name, "is") && name[2] >= 'A' && name[2] <= 'Z' {
		return name
	}
	return "get" + strings.ToUpper(name[:1]) + name[1:]
}

func inclusiveFirstToken(file *scanner.File, n uint32, text string) uint32 {
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == text {
			return c
		}
	}
	return 0
}

func inclusiveBaseName(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		return path[i+1:]
	}
	return path
}

// inclusiveDocStart returns the offset of the `/**` comment that ends directly (whitespace aside) before start, or
// -1. The parser attaches a comment to whatever precedes it, so the text is read instead of the tree.
func inclusiveDocStart(file *scanner.File, start int) int {
	c := file.Content
	i := start - 1
	for i >= 0 && (c[i] == ' ' || c[i] == '\t' || c[i] == '\n' || c[i] == '\r') {
		i--
	}
	if i < 1 || c[i] != '/' || c[i-1] != '*' {
		return -1
	}
	open := bytes.LastIndex(c[:i-1], []byte("/*"))
	if open < 0 || open+2 >= len(c) || c[open+2] != '*' || open+3 > i-1 {
		return -1
	}
	return open
}

// inclusiveLineCol converts a byte offset to a 1-based line and column.
func inclusiveLineCol(file *scanner.File, off int) (int, int) {
	line, col := 1, 1
	for _, b := range file.Content[:off] {
		if b == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}
