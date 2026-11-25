package rules

import (
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// SyntheticAccessorRule mirrors Lint's SyntheticAccessor: a private member used from another class of the same
// file (an inner, nested, local or anonymous class, the outer class, or a Kotlin top-level function's file class),
// which javac and kotlinc reach through a synthetic accessor method.
//
// Lint resolves every reference with the compiler. The port resolves names lexically within the file: an
// unqualified name binds to the innermost enclosing class that declares it, a qualified one to the class its
// receiver is declared or constructed as. It abstains where a supertype declared elsewhere could supply the name
// (a class extending a class from another file, or a lambda passed to apply/run/with, whose receiver may), and for
// receivers whose type the file does not show. As in Lint, Kotlin properties, compile-time constants, calls from a
// companion object, sealed-class constructors and constructors taking a value class are not reported.
type SyntheticAccessorRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: references are resolved lexically within the file.
func (r *SyntheticAccessorRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *SyntheticAccessorRule) check(ctx *api.Context) {
	file := ctx.File
	if !strings.Contains(string(file.Content), "private") {
		return
	}
	s := newSyntheticScan(file)
	if file.Language == scanner.LangJava {
		s.scanJava()
	} else {
		s.scanKotlin()
	}
	for _, f := range s.out {
		ctx.EmitAt(f.line, f.col, f.msg)
	}
}

type syntheticFinding struct {
	line, col int
	msg       string
}

// syntheticMember is a declaration a reference can bind to.
type syntheticMember struct {
	decl     uint32
	name     string
	kind     string // "method", "field", "constructor"
	private  bool
	class    uint32 // declaring class node; 0 for a Kotlin top-level declaration (the file class)
	params   int    // parameter count; for Kotlin, the maximum
	minArgs  int    // Kotlin: parameters without a default value
	static   bool
	constant bool // a Java compile-time constant field
	// extRecv is the receiver type of a Kotlin extension function, as written ("" for a member).
	extRecv string
}

type syntheticScan struct {
	file *scanner.File
	// members maps a class node (0 for the Kotlin file class) to its declared members.
	members map[uint32][]syntheticMember
	// classes maps a simple class name to its declarations in the file.
	classes map[string][]uint32
	out     []syntheticFinding
	seen    map[[2]int]bool
}

func newSyntheticScan(file *scanner.File) *syntheticScan {
	s := &syntheticScan{file: file, members: map[uint32][]syntheticMember{}, classes: map[string][]uint32{}, seen: map[[2]int]bool{}}
	if file.Language == scanner.LangJava {
		s.indexJava()
	} else {
		s.indexKotlin()
	}
	return s
}

func (s *syntheticScan) report(at uint32, msg string) {
	line, col := s.file.FlatRow(at)+1, s.file.FlatCol(at)+1
	key := [2]int{line, col}
	if s.seen[key] {
		return
	}
	s.seen[key] = true
	s.out = append(s.out, syntheticFinding{line: line, col: col, msg: msg})
}

// className is the JVM simple name Lint shows for a class node; 0 is the Kotlin file class (FooKt).
func (s *syntheticScan) className(cls uint32) string {
	file := s.file
	if cls == 0 {
		if name := kotlinJvmFileName(file); name != "" {
			return name
		}
		base := strings.TrimSuffix(filepath.Base(file.Path), filepath.Ext(file.Path))
		if base == "" {
			return "Kt"
		}
		return strings.ToUpper(base[:1]) + base[1:] + "Kt"
	}
	if file.FlatType(cls) == "companion_object" {
		if id, ok := file.FlatFindChild(cls, "type_identifier"); ok {
			return file.FlatNodeText(id)
		}
		return "Companion"
	}
	if n := lintClassNameNode(file, cls); n != 0 {
		return file.FlatNodeText(n)
	}
	for _, kind := range []string{"identifier", "type_identifier", "simple_identifier"} {
		if id, ok := file.FlatFindChild(cls, kind); ok {
			return file.FlatNodeText(id)
		}
	}
	return ""
}

// kotlinJvmFileName returns the name given by @file:JvmName("..."), or "".
func kotlinJvmFileName(file *scanner.File) string {
	text := string(file.Content)
	i := strings.Index(text, "@file:JvmName(")
	if i < 0 {
		return ""
	}
	rest := text[i+len("@file:JvmName("):]
	if q := strings.IndexByte(rest, '"'); q >= 0 {
		rest = rest[q+1:]
		if e := strings.IndexByte(rest, '"'); e >= 0 {
			return rest[:e]
		}
	}
	return ""
}

// syntheticDeclName returns the name node of a class or object declaration.
func syntheticDeclName(file *scanner.File, decl uint32) uint32 {
	if n := lintClassNameNode(file, decl); n != 0 {
		return n
	}
	for _, kind := range []string{"type_identifier", "simple_identifier", "identifier"} {
		if n, ok := file.FlatFindChild(decl, kind); ok {
			return n
		}
	}
	return 0
}

// syntheticIsClass reports whether a node opens a JVM class: Kotlin classes, objects, companions, object
// expressions and enum entries with a body; Java classes, interfaces, enums, records and anonymous classes.
func syntheticIsClass(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "class_declaration", "object_declaration", "companion_object", "object_literal", "interface_declaration",
		"enum_declaration", "record_declaration":
		return true
	case "enum_entry", "enum_constant":
		_, ok := file.FlatFindChild(idx, "class_body")
		return ok
	case "object_creation_expression":
		_, ok := file.FlatFindChild(idx, "class_body")
		return ok
	}
	return false
}

// ownerClass is the innermost class around idx (not idx itself), or 0 for Kotlin top-level code.
func (s *syntheticScan) ownerClass(idx uint32) uint32 {
	prev := idx
	for cur, ok := s.file.FlatParent(idx); ok; cur, ok = s.file.FlatParent(cur) {
		if syntheticIsClass(s.file, cur) && syntheticInClassBody(s.file, cur, prev) {
			return cur
		}
		prev = cur
	}
	return 0
}

// syntheticInClassBody reports whether child (a direct child of cls on the path to a reference) is in the class's
// body. An anonymous class's constructor arguments run in the surrounding class, not in the anonymous one.
func syntheticInClassBody(file *scanner.File, cls, child uint32) bool {
	switch file.FlatType(cls) {
	case "object_creation_expression", "object_literal":
		return file.FlatType(child) == "class_body"
	}
	return true
}

// enclosingClasses lists the classes around idx, innermost first, ending with 0 (the Kotlin file class) for Kotlin.
func (s *syntheticScan) enclosingClasses(idx uint32) []uint32 {
	var out []uint32
	prev := idx
	for cur, ok := s.file.FlatParent(idx); ok; cur, ok = s.file.FlatParent(cur) {
		if syntheticIsClass(s.file, cur) && syntheticInClassBody(s.file, cur, prev) {
			out = append(out, cur)
		}
		prev = cur
	}
	if s.file.Language == scanner.LangKotlin {
		out = append(out, 0)
	}
	return out
}

// ---------------------------------------------------------------------------
// Kotlin
// ---------------------------------------------------------------------------

func (s *syntheticScan) indexKotlin() {
	file := s.file
	file.FlatWalkAllNodes(0, func(idx uint32) {
		switch file.FlatType(idx) {
		case "class_declaration", "object_declaration":
			if n := syntheticDeclName(file, idx); n != 0 {
				name := file.FlatNodeText(n)
				s.classes[name] = append(s.classes[name], idx)
			}
			s.indexKotlinConstructors(idx)
		case "function_declaration":
			cls, top := s.kotlinMemberOwner(idx)
			if !top && cls == 0 {
				return // local function
			}
			id, ok := file.FlatFindChild(idx, "simple_identifier")
			if !ok {
				return
			}
			params, required := kotlinParamCounts(file, idx, "function_value_parameters", "parameter")
			s.members[cls] = append(s.members[cls], syntheticMember{decl: idx, name: file.FlatNodeText(id), kind: "method",
				private: file.FlatHasModifier(idx, "private"), class: cls, params: params, minArgs: required,
				extRecv: kotlinExtensionReceiver(file, idx)})
		}
	})
}

// kotlinMemberOwner returns the class a Kotlin declaration is a member of, or top for a top-level declaration.
// A declaration inside a function body is neither.
func (s *syntheticScan) kotlinMemberOwner(decl uint32) (cls uint32, top bool) {
	parent, ok := s.file.FlatParent(decl)
	if !ok {
		return 0, false
	}
	switch s.file.FlatType(parent) {
	case "source_file":
		return 0, true
	case "class_body", "enum_class_body":
		owner, ok := s.file.FlatParent(parent)
		if ok && syntheticIsClass(s.file, owner) {
			return owner, false
		}
	}
	return 0, false
}

func (s *syntheticScan) indexKotlinConstructors(cls uint32) {
	file := s.file
	if ctor, ok := file.FlatFindChild(cls, "primary_constructor"); ok {
		params, required := kotlinParamCounts(file, ctor, "class_parameters", "class_parameter")
		if params == 0 && required == 0 {
			params, required = kotlinParamCounts(file, ctor, "", "class_parameter")
		}
		s.members[cls] = append(s.members[cls], syntheticMember{decl: ctor, kind: "constructor",
			private: file.FlatHasModifier(ctor, "private"), class: cls, params: params, minArgs: required})
	}
	if body, ok := file.FlatFindChild(cls, "class_body"); ok {
		for c := file.FlatFirstChild(body); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) != "secondary_constructor" {
				continue
			}
			params, required := kotlinParamCounts(file, c, "function_value_parameters", "parameter")
			s.members[cls] = append(s.members[cls], syntheticMember{decl: c, kind: "constructor",
				private: file.FlatHasModifier(c, "private"), class: cls, params: params, minArgs: required})
		}
	}
}

// kotlinParamCounts counts the parameters of a declaration and those without a default value.
func kotlinParamCounts(file *scanner.File, decl uint32, listType, paramType string) (total, required int) {
	list := decl
	if listType != "" {
		l, ok := file.FlatFindChild(decl, listType)
		if !ok {
			return 0, 0
		}
		list = l
	}
	pendingDefault, vararg := false, false
	for c := file.FlatFirstChild(list); c != 0; c = file.FlatNextSib(c) {
		switch {
		case file.FlatType(c) == "parameter_modifiers" && strings.Contains(file.FlatNodeText(c), "vararg"):
			vararg = true
		case file.FlatType(c) == paramType:
			total++
			required++
			pendingDefault = true
			if strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(c)), "vararg") || vararg {
				// A vararg parameter takes any number of arguments, including none.
				required--
				total += syntheticVarargMax
				vararg = false
			}
			if perfHasToken(file, c, "=") {
				required--
				pendingDefault = false
			}
		case !file.FlatIsNamed(c) && strings.TrimSpace(file.FlatNodeText(c)) == "=" && pendingDefault:
			// function_value_parameters puts the default after the parameter node.
			required--
			pendingDefault = false
		case strings.TrimSpace(file.FlatNodeText(c)) == ",":
			pendingDefault = false
		}
	}
	return total, required
}

func (s *syntheticScan) scanKotlin() {
	file := s.file
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		if perfIsTrailingLambdaInner(file, idx) {
			return
		}
		call := perfKotlinCall(file, idx)
		if call.kind != perfCall || call.name == "" {
			return
		}
		s.kotlinCall(idx, call)
	})
}

func (s *syntheticScan) kotlinCall(idx uint32, call *perfExpr) {
	file := s.file
	owner := s.ownerClass(idx)
	var target *syntheticMember
	if call.recv == nil {
		if s.inReceiverLambda(idx) {
			return
		}
		m, ok := s.kotlinUnqualified(idx, call.name, len(call.kids))
		if !ok {
			return
		}
		target = m
	} else if cls, ok := s.kotlinReceiverClass(idx, call.recv); ok && s.hasMember(cls, call.name, "method") {
		target = s.pickMember(cls, call.name, "method", len(call.kids))
	} else {
		// No member of that name: the call can only be an extension.
		target = s.kotlinExtensionTarget(idx, call)
	}
	if target == nil || !target.private || target.class == owner {
		return
	}
	if file.FlatType(owner) == "companion_object" {
		if _, named := file.FlatFindChild(owner, "type_identifier"); !named {
			return // Lint skips calls made from a companion object
		}
	}
	if target.kind == "constructor" {
		if s.kotlinSkipConstructor(target) {
			return
		}
		s.report(idx, "Access to `private` constructor of class `"+s.className(target.class)+"` requires synthetic accessor")
		return
	}
	s.report(idx, "Access to `private` method `"+target.name+"` of class `"+s.className(target.class)+"` requires synthetic accessor")
}

// kotlinUnqualified resolves an unqualified Kotlin call: a local function or variable of an enclosing scope hides
// members; otherwise the innermost enclosing class (then its companion) declaring the name, then top-level
// functions; a capitalized name with no function is a constructor of a visible class of the file.
func (s *syntheticScan) kotlinUnqualified(idx uint32, name string, args int) (*syntheticMember, bool) {
	file := s.file
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if t := file.FlatType(cur); t == "statements" || t == "function_body" {
			if len(perfDirectFunctions(file, cur, name)) > 0 {
				return nil, false
			}
		}
	}
	if lintNameIsParameterAt(file, idx, name) {
		return nil, false
	}
	for _, cls := range s.enclosingClasses(idx) {
		if m := s.pickMember(cls, name, "method", args); m != nil {
			return m, true
		}
		if s.hasMember(cls, name, "method") {
			return nil, false
		}
		if cls != 0 {
			if comp := kotlinCompanionOf(file, cls); comp != 0 {
				if m := s.pickMember(comp, name, "method", args); m != nil {
					return m, true
				}
				if s.hasMember(comp, name, "method") {
					return nil, false
				}
			}
			// A superclass from elsewhere, or a same-file superclass declaring the name, can supply it first.
			if s.kotlinHasForeignClassSupertype(cls) || s.inheritsMember(cls, name, "method", 0) {
				return nil, false
			}
		}
	}
	if name != "" && name[0] >= 'A' && name[0] <= 'Z' {
		if decls := s.classes[name]; len(decls) == 1 {
			return s.pickMember(decls[0], "", "constructor", args), true
		}
	}
	return nil, false
}

// kotlinReceiverClass resolves the class of an explicit Kotlin receiver: this, this@Label, a constructor call of a
// class of the file, a class or object name, or a variable declared or constructed as one.
func (s *syntheticScan) kotlinReceiverClass(at uint32, recv *perfExpr) (uint32, bool) {
	file := s.file
	text := strings.TrimSpace(recv.text(file))
	switch recv.kind {
	case perfInstance:
		if text == "this" {
			owner := s.ownerClass(at)
			return owner, owner != 0
		}
		if strings.HasPrefix(text, "this@") {
			label := strings.TrimPrefix(text, "this@")
			for _, cls := range s.enclosingClasses(at) {
				if cls != 0 && s.className(cls) == label {
					return cls, true
				}
			}
		}
		return 0, false
	case perfCall:
		if recv.recv == nil && recv.name != "" && recv.name[0] >= 'A' && recv.name[0] <= 'Z' {
			if decls := s.classes[recv.name]; len(decls) == 1 && len(perfEnclosingFunctions(file, at, recv.name, false)) == 0 {
				return decls[0], true
			}
		}
		return 0, false
	case perfName:
		// A local, parameter or property of that name is a value receiver; its type decides, not a class name.
		isValue := lintNameIsParameterAt(file, at, recv.name) || lintDeclaredTypeText(file, at, recv.name) != "" ||
			lintInitializerOf(file, at, recv.name, true) != 0
		if decls := s.classes[recv.name]; !isValue && len(decls) == 1 && recv.name[0] >= 'A' && recv.name[0] <= 'Z' {
			// `Outer.f()` reaches Outer's companion (or the object itself).
			if file.FlatType(decls[0]) == "object_declaration" {
				return decls[0], true
			}
			if comp := kotlinCompanionOf(file, decls[0]); comp != 0 {
				return comp, true
			}
			return 0, false
		}
		typ := lintTrimGenerics(strings.TrimSuffix(strings.TrimSpace(lintDeclaredTypeText(file, at, recv.name)), "?"))
		if typ == "" {
			if init := lintInitializerOf(file, at, recv.name, true); init != 0 {
				if c, ok := lintCallAt(file, init); ok && file.FlatType(init) == "call_expression" && c.ReceiverNode == 0 {
					typ = c.Name
				}
			}
		}
		if decls := s.classes[typ]; len(decls) == 1 {
			return decls[0], true
		}
	}
	return 0, false
}

// inReceiverLambda reports whether idx sits in a lambda passed to apply/run/with (or buildString and friends), whose
// receiver may supply an unqualified name.
func (s *syntheticScan) inReceiverLambda(idx uint32) bool {
	file := s.file
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "class_body", "source_file":
			return false
		case "lambda_literal":
			owner := syntheticLambdaOwner(file, cur)
			if owner == 0 {
				continue
			}
			if first := file.FlatFirstChild(owner); file.FlatType(first) == "call_expression" {
				owner = first
			}
			switch flatCallExpressionName(file, owner) {
			case "apply", "run", "with", "buildString", "buildList", "buildSet", "buildMap", "use":
				return true
			}
		}
	}
	return false
}

// kotlinHasForeignClassSupertype reports whether a Kotlin class extends a class (a supertype written with a
// constructor call) that the file does not declare, whose members could hide an outer declaration.
func (s *syntheticScan) kotlinHasForeignClassSupertype(cls uint32) bool {
	file := s.file
	foreign := false
	for c := file.FlatFirstChild(cls); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) != "delegation_specifier" && file.FlatType(c) != "delegation_specifiers" {
			continue
		}
		file.FlatWalkNodes(c, "constructor_invocation", func(inv uint32) {
			name := strings.TrimSpace(file.FlatNodeText(inv))
			if i := strings.IndexAny(name, "(<"); i >= 0 {
				name = name[:i]
			}
			name = name[strings.LastIndex(name, ".")+1:]
			if len(s.classes[name]) == 0 {
				foreign = true
			}
		})
	}
	return foreign
}

func kotlinCompanionOf(file *scanner.File, cls uint32) uint32 {
	body, ok := file.FlatFindChild(cls, "class_body")
	if !ok {
		return 0
	}
	for c := file.FlatFirstChild(body); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "companion_object" {
			return c
		}
	}
	return 0
}

// kotlinSkipConstructor mirrors Lint's exemptions for constructors: sealed classes and value-class parameters.
func (s *syntheticScan) kotlinSkipConstructor(m *syntheticMember) bool {
	file := s.file
	if file.FlatHasModifier(m.class, "sealed") {
		return true
	}
	valueClass := false
	file.FlatWalkNodes(m.decl, "user_type", func(t uint32) {
		name := strings.TrimSpace(file.FlatNodeText(t))
		for _, decl := range s.classes[name] {
			if file.FlatHasModifier(decl, "value") || file.FlatHasModifier(decl, "inline") {
				valueClass = true
			}
		}
	})
	return valueClass
}

// ---------------------------------------------------------------------------
// Java
// ---------------------------------------------------------------------------

func (s *syntheticScan) indexJava() {
	file := s.file
	file.FlatWalkAllNodes(0, func(idx uint32) {
		switch file.FlatType(idx) {
		case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration":
			if n := lintClassNameNode(file, idx); n != 0 {
				name := file.FlatNodeText(n)
				s.classes[name] = append(s.classes[name], idx)
			}
		case "method_declaration", "constructor_declaration":
			cls := s.javaMemberOwner(idx)
			if cls == 0 {
				return
			}
			m := syntheticMember{decl: idx, kind: "method", private: file.FlatHasModifier(idx, "private"), class: cls,
				static: file.FlatHasModifier(idx, "static")}
			n, vararg := syntheticJavaParamCount(file, idx)
			m.params, m.minArgs = n, n
			if vararg {
				m.minArgs, m.params = n-1, n-1+syntheticVarargMax
			}
			if file.FlatType(idx) == "constructor_declaration" {
				m.kind = "constructor"
			} else if id, ok := file.FlatFindChild(idx, "identifier"); ok {
				m.name = file.FlatNodeText(id)
			}
			s.members[cls] = append(s.members[cls], m)
		case "field_declaration":
			cls := s.javaMemberOwner(idx)
			if cls == 0 {
				return
			}
			private, final := file.FlatHasModifier(idx, "private"), file.FlatHasModifier(idx, "final")
			file.FlatWalkNodes(idx, "variable_declarator", func(d uint32) {
				id, ok := file.FlatFindChild(d, "identifier")
				if !ok {
					return
				}
				init := lintInitializerAfterEquals(file, d)
				s.members[cls] = append(s.members[cls], syntheticMember{decl: d, name: file.FlatNodeText(id), kind: "field",
					private: private, class: cls, constant: final && init != 0 && javaIsConstantExpression(file, init)})
			})
		}
	})
}

func (s *syntheticScan) javaMemberOwner(decl uint32) uint32 {
	parent, ok := s.file.FlatParent(decl)
	if !ok {
		return 0
	}
	switch s.file.FlatType(parent) {
	case "class_body", "interface_body", "enum_body_declarations", "enum_body":
		for cur, ok := s.file.FlatParent(parent); ok; cur, ok = s.file.FlatParent(cur) {
			if syntheticIsClass(s.file, cur) {
				return cur
			}
			if t := s.file.FlatType(cur); t != "enum_body" && t != "enum_body_declarations" {
				return 0
			}
		}
	}
	return 0
}

// javaIsConstantExpression reports whether an initializer may be a compile-time constant: no calls, object or array
// creation. Names of other fields count as constants, so a final field Lint's ConstantEvaluator might not fold is a
// known false negative rather than a false positive.
func javaIsConstantExpression(file *scanner.File, init uint32) bool {
	constant := true
	file.FlatWalkAllNodes(init, func(n uint32) {
		switch file.FlatType(n) {
		case "method_invocation", "object_creation_expression", "array_creation_expression", "array_initializer",
			"lambda_expression", "method_reference", "this":
			constant = false
		}
	})
	return constant
}

func (s *syntheticScan) scanJava() {
	file := s.file
	file.FlatWalkNodes(0, "method_invocation", func(idx uint32) { s.javaInvocation(idx) })
	file.FlatWalkNodes(0, "object_creation_expression", func(idx uint32) { s.javaCreation(idx) })
	file.FlatWalkNodes(0, "identifier", func(idx uint32) { s.javaFieldRef(idx) })
}

func (s *syntheticScan) javaInvocation(idx uint32) {
	file := s.file
	call, ok := lintCallAt(file, idx)
	if !ok || call.ReceiverText == "super" || strings.HasSuffix(call.ReceiverText, ".super") {
		return
	}
	var cls uint32
	if call.ReceiverNode == 0 {
		m, ok := s.javaUnqualified(idx, call.Name, "method")
		if !ok {
			return
		}
		cls = m
	} else {
		c, ok := s.javaReceiverClass(idx, call.ReceiverNode)
		if !ok {
			return
		}
		cls = c
	}
	target := s.pickMember(cls, call.Name, "method", len(call.Args))
	if target == nil || !target.private || target.class == s.ownerClass(idx) {
		return
	}
	s.report(idx, "Access to `private` method `"+target.name+"` of class `"+s.className(target.class)+"` requires synthetic accessor")
}

func (s *syntheticScan) javaCreation(idx uint32) {
	file := s.file
	if _, anonymous := file.FlatFindChild(idx, "class_body"); anonymous {
		return
	}
	call, ok := lintCallAt(file, idx)
	if !ok {
		return
	}
	decls := s.classes[call.Name]
	if len(decls) != 1 {
		return
	}
	cls := decls[0]
	owner := s.ownerClass(idx)
	if cls == owner {
		return
	}
	var ctors []syntheticMember
	for _, m := range s.members[cls] {
		if m.kind == "constructor" {
			ctors = append(ctors, m)
		}
	}
	if len(ctors) == 0 {
		// The default constructor of a private class: Lint names the class itself a "member".
		if file.FlatHasModifier(cls, "private") {
			s.report(idx, "Access to `private` member of class `"+s.className(cls)+"` requires synthetic accessor")
		}
		return
	}
	target := s.pickMember(cls, "", "constructor", len(call.Args))
	if target == nil || !target.private || target.static {
		return
	}
	s.report(idx, "Access to `private` constructor of class `"+s.className(cls)+"` requires synthetic accessor")
}

// javaFieldRef handles a name that reads or writes a field: an unqualified identifier in expression position, or
// the field of a field access.
func (s *syntheticScan) javaFieldRef(idx uint32) {
	file := s.file
	parent, ok := file.FlatParent(idx)
	if !ok {
		return
	}
	name := file.FlatNodeText(idx)
	var cls uint32
	switch file.FlatType(parent) {
	case "field_access":
		if kids := perfNamedChildren(file, parent); len(kids) == 2 && kids[1] == idx {
			c, ok := s.javaReceiverClass(idx, kids[0])
			if !ok {
				return
			}
			cls = c
			break
		}
		fallthrough
	default:
		if !javaIdentifierIsExpression(file, idx, parent) {
			return
		}
		c, ok := s.javaUnqualified(idx, name, "field")
		if !ok {
			return
		}
		cls = c
	}
	target := s.pickMember(cls, name, "field", -1)
	if target == nil || !target.private || target.constant || target.class == s.ownerClass(idx) {
		return
	}
	s.report(idx, "Access to `private` field `"+name+"` of class `"+s.className(target.class)+"` requires synthetic accessor")
}

// javaIdentifierIsExpression reports whether an identifier is used as a value, not declared, called or labeled.
func javaIdentifierIsExpression(file *scanner.File, idx, parent uint32) bool {
	switch file.FlatType(parent) {
	case "variable_declarator":
		// The declared name is the declarator's first identifier; the initializer's names are reads.
		first, _ := file.FlatFindChild(parent, "identifier")
		return first != idx
	case "formal_parameter", "catch_formal_parameter", "spread_parameter", "method_declaration",
		"constructor_declaration", "class_declaration", "interface_declaration", "enum_declaration", "record_declaration",
		"enum_constant", "labeled_statement", "break_statement", "continue_statement", "annotation", "marker_annotation",
		"element_value_pair", "scoped_identifier", "inferred_parameters", "lambda_expression", "resource",
		"annotation_type_declaration", "type_parameter", "import_declaration", "package_declaration",
		"method_reference", "explicit_constructor_invocation":
		return false
	case "method_invocation":
		// The method name is the last identifier before the arguments; an earlier one is the receiver.
		return lintCallNameNode(file, parent) != idx
	}
	return true
}

// javaUnqualified finds the class an unqualified Java name binds to, walking scopes outward as javac does: locals
// and parameters of each method, lambda or block (for fields), then the members of each enclosing class. It abstains
// past a class that extends a class declared elsewhere, whose inherited members could supply the name.
func (s *syntheticScan) javaUnqualified(idx uint32, name, kind string) (uint32, bool) {
	file := s.file
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if kind == "field" && javaScopeDeclaresLocal(file, cur, idx, name) {
			return 0, false
		}
		if syntheticIsClass(file, cur) {
			if s.hasMember(cur, name, kind) {
				return cur, true
			}
			if s.javaExtendsForeignClass(cur) || s.inheritsMember(cur, name, kind, 0) {
				return 0, false
			}
		}
	}
	return 0, false
}

// javaScopeDeclaresLocal reports whether scope itself declares a parameter, lambda parameter or local variable
// called name that is visible at idx.
func javaScopeDeclaresLocal(file *scanner.File, scope, idx uint32, name string) bool {
	found := false
	direct := func(container uint32, kind string, check func(uint32)) {
		for c := file.FlatFirstChild(container); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == kind {
				check(c)
			}
		}
	}
	declarator := func(d uint32) {
		if id, ok := file.FlatFindChild(d, "identifier"); ok && file.FlatNodeText(id) == name &&
			file.FlatStartByte(d) < file.FlatStartByte(idx) {
			found = true
		}
	}
	switch file.FlatType(scope) {
	case "method_declaration", "constructor_declaration":
		if params, ok := file.FlatFindChild(scope, "formal_parameters"); ok {
			direct(params, "formal_parameter", func(p uint32) {
				if id, ok := file.FlatFindChild(p, "identifier"); ok && file.FlatNodeText(id) == name {
					found = true
				}
			})
		}
	case "lambda_expression":
		for c := file.FlatFirstChild(scope); c != 0 && file.FlatType(c) != "->"; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "identifier":
				found = found || file.FlatNodeText(c) == name
			case "inferred_parameters", "formal_parameters":
				file.FlatWalkNodes(c, "identifier", func(p uint32) { found = found || file.FlatNodeText(p) == name })
			}
		}
	case "block", "switch_block_statement_group":
		direct(scope, "local_variable_declaration", func(decl uint32) { direct(decl, "variable_declarator", declarator) })
	case "for_statement", "try_with_resources_statement":
		file.FlatWalkNodes(scope, "local_variable_declaration", func(decl uint32) {
			if p, ok := file.FlatParent(decl); ok && p == scope {
				direct(decl, "variable_declarator", declarator)
			}
		})
		file.FlatWalkNodes(scope, "resource", func(r uint32) {
			if id, ok := file.FlatFindChild(r, "identifier"); ok && file.FlatNodeText(id) == name {
				found = true
			}
		})
	case "enhanced_for_statement", "catch_clause":
		file.FlatWalkNodes(scope, "identifier", func(id uint32) {
			if p, ok := file.FlatParent(id); ok && (p == scope || file.FlatType(p) == "catch_formal_parameter") &&
				file.FlatNodeText(id) == name && file.FlatStartByte(id) < file.FlatStartByte(idx) {
				found = true
			}
		})
	}
	return found
}

// javaExtendsForeignClass reports whether a Java class (named or anonymous) extends a class the file does not
// declare. Interfaces do not count.
func (s *syntheticScan) javaExtendsForeignClass(cls uint32) bool {
	if cls == 0 {
		return false
	}
	file := s.file
	if file.FlatType(cls) == "object_creation_expression" {
		// An anonymous class inherits its supertype's members first. Only a class of the file or a well-known
		// interface has a known member set; any other supertype could declare the name.
		c, ok := lintCallAt(file, cls)
		if !ok {
			return true
		}
		simple := c.Name
		return len(s.classes[simple]) == 0 && !syntheticKnownInterfaces[simple]
	}
	sc, ok := file.FlatFindChild(cls, "superclass")
	if !ok {
		return false
	}
	foreign := false
	for c := file.FlatFirstChild(sc); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "type_identifier", "scoped_type_identifier", "generic_type":
			name := lintTrimGenerics(file.FlatNodeText(c))
			name = name[strings.LastIndex(name, ".")+1:]
			if len(s.classes[name]) == 0 {
				foreign = true
			}
		}
	}
	return foreign
}

// javaReceiverClass resolves the class of an explicit Java receiver: this, Outer.this, a class name, a `new X()`
// expression, or a variable or field declared as a class of the file.
func (s *syntheticScan) javaReceiverClass(at, recv uint32) (uint32, bool) {
	file := s.file
	text := strings.TrimSpace(file.FlatNodeText(recv))
	switch file.FlatType(recv) {
	case "this":
		owner := s.ownerClass(at)
		return owner, owner != 0
	case "field_access":
		if strings.HasSuffix(text, ".this") {
			label := strings.TrimSuffix(text, ".this")
			label = label[strings.LastIndex(label, ".")+1:]
			for _, cls := range s.enclosingClasses(at) {
				if cls != 0 && s.className(cls) == label {
					return cls, true
				}
			}
		}
		return 0, false
	case "object_creation_expression":
		if _, anonymous := file.FlatFindChild(recv, "class_body"); anonymous {
			return 0, false
		}
		if c, ok := lintCallAt(file, recv); ok {
			if decls := s.classes[c.Name]; len(decls) == 1 {
				return decls[0], true
			}
		}
		return 0, false
	case "parenthesized_expression":
		if kids := perfNamedChildren(file, recv); len(kids) == 1 {
			return s.javaReceiverClass(at, kids[0])
		}
		return 0, false
	case "identifier":
		if !s.javaNameIsValue(at, text) {
			if decls := s.classes[text]; len(decls) == 1 {
				return decls[0], true // a static member: `Outer.helper()`
			}
		}
		typ := lintTrimGenerics(strings.TrimSpace(lintDeclaredTypeText(file, at, text)))
		typ = typ[strings.LastIndex(typ, ".")+1:]
		if decls := s.classes[typ]; len(decls) == 1 {
			return decls[0], true
		}
	}
	return 0, false
}

// javaNameIsValue reports whether name at idx is a local, parameter or field rather than a class name.
func (s *syntheticScan) javaNameIsValue(idx uint32, name string) bool {
	file := s.file
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if javaScopeDeclaresLocal(file, cur, idx, name) {
			return true
		}
		if syntheticIsClass(file, cur) && s.hasMember(cur, name, "field") {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Shared
// ---------------------------------------------------------------------------

func (s *syntheticScan) hasMember(cls uint32, name, kind string) bool {
	for _, m := range s.members[cls] {
		if m.kind == kind && m.name == name {
			return true
		}
	}
	return false
}

// pickMember selects the member of cls with the given kind and name that a call with args arguments targets (args
// < 0 for fields), or nil when none or several match.
func (s *syntheticScan) pickMember(cls uint32, name, kind string, args int) *syntheticMember {
	var match *syntheticMember
	count := 0
	for i := range s.members[cls] {
		m := &s.members[cls][i]
		if m.kind != kind || m.name != name {
			continue
		}
		if args >= 0 && (args < m.minArgs || args > m.params) {
			continue
		}
		match = m
		count++
	}
	if count != 1 {
		return nil
	}
	return match
}

// syntheticLambdaOwner returns the call a Kotlin trailing lambda belongs to, or 0.
func syntheticLambdaOwner(file *scanner.File, lambda uint32) uint32 {
	p, ok := file.FlatParent(lambda)
	if ok && file.FlatType(p) == "annotated_lambda" {
		p, ok = file.FlatParent(p)
	}
	if !ok || file.FlatType(p) != "call_suffix" {
		return 0
	}
	owner, ok := file.FlatParent(p)
	if !ok || file.FlatType(owner) != "call_expression" {
		return 0
	}
	return owner
}

// syntheticJavaParamCount counts the formal parameters of a Java method or constructor.
// syntheticVarargMax stands for the unbounded argument count a variable-arity parameter accepts.
const syntheticVarargMax = 1 << 20

// syntheticJavaParamCount counts the formal parameters of a Java method or constructor and reports a trailing
// variable-arity parameter.
func syntheticJavaParamCount(file *scanner.File, method uint32) (n int, vararg bool) {
	params, ok := file.FlatFindChild(method, "formal_parameters")
	if !ok {
		return 0, false
	}
	for c := file.FlatFirstChild(params); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "formal_parameter":
			n++
		case "spread_parameter":
			n++
			vararg = true
		}
	}
	return n, vararg
}

// kotlinExtensionReceiver returns the receiver type written on an extension function (`fun String.f()`), or "".
func kotlinExtensionReceiver(file *scanner.File, fn uint32) string {
	for c := file.FlatFirstChild(fn); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "simple_identifier":
			return ""
		case "user_type", "nullable_type":
			return lintTrimGenerics(strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(c)), "?"))
		}
	}
	return ""
}

// kotlinExtensionTarget resolves `recv.f()` to a private top-level extension function of the file when the
// receiver's type is shown (a string literal, or a variable declared with the extension's receiver type).
func (s *syntheticScan) kotlinExtensionTarget(idx uint32, call *perfExpr) *syntheticMember {
	file := s.file
	recvType := ""
	switch call.recv.kind {
	case perfLiteral, perfBinary:
		// A String has members of its own; only a name none of them uses can be the extension.
		if call.recv.node != 0 && file.FlatType(call.recv.node) == "string_literal" && !syntheticStringMembers[call.name] {
			recvType = "String"
		}
	case perfName:
		// The receiver's declared type must be a class of the file, whose members are known (kotlinCall routes here
		// only when that class has no member of the name). A type from elsewhere may declare the member.
		typ := lintTrimGenerics(strings.TrimSuffix(strings.TrimSpace(lintDeclaredTypeText(file, idx, call.recv.name)), "?"))
		if typ != "" && len(s.classes[typ[strings.LastIndex(typ, ".")+1:]]) == 1 {
			recvType = typ
		}
	}
	if recvType == "" {
		return nil
	}
	recvType = recvType[strings.LastIndex(recvType, ".")+1:]
	var match *syntheticMember
	for i := range s.members[0] {
		m := &s.members[0][i]
		if m.name != call.name || m.extRecv == "" || m.extRecv[strings.LastIndex(m.extRecv, ".")+1:] != recvType {
			continue
		}
		if len(call.kids) < m.minArgs || len(call.kids) > m.params {
			continue
		}
		if match != nil {
			return nil
		}
		match = m
	}
	return match
}

// syntheticStringMembers are member names of kotlin.String (and Any/CharSequence/Comparable), which win over an
// extension of the same name.
var syntheticStringMembers = map[string]bool{
	"toString": true, "equals": true, "hashCode": true, "length": true, "get": true, "subSequence": true,
	"compareTo": true, "plus": true,
}

// inheritsMember reports whether a same-file superclass (or interface) of cls declares a non-private member of that
// kind and name, which the class inherits before any outer declaration is reached.
func (s *syntheticScan) inheritsMember(cls uint32, name, kind string, depth int) bool {
	if cls == 0 || depth > 6 {
		return false
	}
	for _, ref := range lintDirectSupertypeRefs(s.file, cls) {
		decls := s.classes[ref.simple]
		if len(decls) != 1 {
			continue
		}
		for _, m := range s.members[decls[0]] {
			if m.kind == kind && m.name == name && !m.private {
				return true
			}
		}
		if s.inheritsMember(decls[0], name, kind, depth+1) {
			return true
		}
	}
	return false
}

// syntheticKnownInterfaces are interfaces whose members an anonymous class implements in its own body, so an
// unqualified name the body does not declare reaches the surrounding class.
var syntheticKnownInterfaces = map[string]bool{
	"Runnable": true, "Callable": true, "Comparator": true, "OnClickListener": true, "OnLongClickListener": true,
	"OnTouchListener": true, "OnCheckedChangeListener": true, "Supplier": true, "Consumer": true, "Function": true,
	"Predicate": true,
}
