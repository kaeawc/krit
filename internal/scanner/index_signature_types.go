package scanner

import "strings"

// Declared-type fingerprinting for function, method, constructor and Java
// field symbols.
//
// Symbol.Signature is "owner#name/arity", which says nothing about parameter
// or return types, so a public API change such as `fun f(x: Int): String` ->
// `fun f(x: Long): String?` or `public String name;` -> `public Integer name;`
// would leave the structural fingerprint untouched and look like a body-only
// edit. Mirroring propertySymbolSignature, the explicitly declared type text
// is appended to the signature as `@Receiver(P1,P2):Return`. Nothing parses
// Signature (consumers compare it, key on it, or test it for non-empty), so
// extending it is safe.
//
// Only source text is used. Inferred return types (`fun g() = 1`) have none
// and are left out, so an expression-body edit stays non-structural.

// typedSignature appends the declared receiver, parameter types and return
// type to the base owner#name/arity signature. With no type text at all it
// returns base unchanged.
func typedSignature(base, receiver string, params []string, ret string) string {
	if receiver == "" && len(params) == 0 && ret == "" {
		return base
	}
	var sb strings.Builder
	sb.WriteString(base)
	if receiver != "" {
		sb.WriteByte('@')
		sb.WriteString(receiver)
	}
	if len(params) > 0 {
		sb.WriteByte('(')
		sb.WriteString(strings.Join(params, ","))
		sb.WriteByte(')')
	}
	if ret != "" {
		sb.WriteByte(':')
		sb.WriteString(ret)
	}
	return internString(sb.String())
}

// normalizeTypeText collapses whitespace in a type's source text so that
// formatting-only edits (`Map<String,Int>` vs `Map<String, Int>`) do not move
// the fingerprint. Whitespace survives only between two word characters
// (`in T`, `vararg String`), where it separates tokens.
func normalizeTypeText(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	pendingSpace := false
	var prev byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case ' ', '\t', '\n', '\r', '\f', '\v':
			pendingSpace = sb.Len() > 0
			continue
		}
		if pendingSpace && isTypeWordByte(prev) && isTypeWordByte(c) {
			sb.WriteByte(' ')
		}
		pendingSpace = false
		sb.WriteByte(c)
		prev = c
	}
	return sb.String()
}

func isTypeWordByte(c byte) bool {
	return c == '_' || c == '$' || c >= 0x80 ||
		(c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// kotlinTypeNodeType reports whether a tree-sitter-kotlin node type spells a
// type.
func kotlinTypeNodeType(t string) bool {
	switch t {
	case "user_type", "nullable_type", "function_type", "parenthesized_type", "not_nullable_type":
		return true
	}
	return false
}

// KotlinFunctionDeclaredTypes returns the explicitly declared extension
// receiver type, parameter types (in order, with vararg/noinline/crossinline
// modifiers) and return type of a Kotlin function_declaration, whitespace
// normalized. Missing pieces are empty: an expression-bodied function with an
// inferred return type has ret == "".
func KotlinFunctionDeclaredTypes(file *File, idx uint32) (receiver string, params []string, ret string) {
	seenName, seenParams := false, false
	mods := ""
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch t := file.FlatType(child); {
		case t == "simple_identifier":
			seenName = true
		case t == "function_value_parameters":
			seenParams = true
			params = kotlinParameterTypes(file, child)
		case t == "type_modifiers":
			// Directly under the declaration these annotate the receiver or
			// return type that follows them.
			mods = normalizeTypeText(file.FlatNodeText(child)) + " "
		case kotlinTypeNodeType(t):
			text := mods + normalizeTypeText(file.FlatNodeText(child))
			mods = ""
			switch {
			case !seenName:
				if receiver == "" {
					receiver = text
				}
			case seenParams && ret == "":
				ret = text
			}
		}
	}
	return receiver, params, ret
}

func kotlinParameterTypes(file *File, paramsNode uint32) []string {
	var out []string
	pending := ""
	for p := file.FlatFirstChild(paramsNode); p != 0; p = file.FlatNextSib(p) {
		switch file.FlatType(p) {
		case "parameter_modifiers":
			pending += normalizeTypeText(file.FlatNodeText(p)) + " "
		case "parameter":
			out = append(out, pending+kotlinParameterType(file, p))
			pending = ""
		}
	}
	return out
}

// kotlinParameterType is the declared type of one `parameter` node, including
// any type modifiers (`suspend`, annotations) that precede it.
func kotlinParameterType(file *File, param uint32) string {
	mods := ""
	for c := file.FlatFirstChild(param); c != 0; c = file.FlatNextSib(c) {
		switch t := file.FlatType(c); {
		case t == "type_modifiers":
			mods = normalizeTypeText(file.FlatNodeText(c)) + " "
		case kotlinTypeNodeType(t):
			return mods + normalizeTypeText(file.FlatNodeText(c))
		}
	}
	return ""
}

func kotlinFunctionSignature(owner, name string, file *File, idx uint32) string {
	base := symbolSignature(owner, name, kotlinFunctionArity(file, idx))
	receiver, params, ret := KotlinFunctionDeclaredTypes(file, idx)
	return typedSignature(base, receiver, params, ret)
}

// javaTypeNodeType reports whether a tree-sitter-java node type spells a type.
func javaTypeNodeType(t string) bool {
	switch t {
	case "type_identifier", "generic_type", "scoped_type_identifier",
		"integral_type", "floating_point_type", "boolean_type", "void_type",
		"array_type", "annotated_type":
		return true
	}
	return false
}

// javaDeclaredType returns the normalized text of the first type child of a
// field_declaration, formal_parameter or spread_parameter, or of the return
// type of a method_declaration (the type that precedes the method name).
// Modifiers, annotations and type parameters are not types and are skipped.
func javaDeclaredType(file *File, idx uint32) string {
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		t := file.FlatType(c)
		if t == "identifier" || t == "variable_declarator" || t == "formal_parameters" {
			return ""
		}
		if javaTypeNodeType(t) {
			return normalizeTypeText(file.FlatNodeText(c))
		}
	}
	return ""
}

// javaFormalParameterTypes returns the declared parameter types of a Java
// method or constructor, with `...` appended for varargs and C-style
// dimensions (`int a[]`) folded in.
func javaFormalParameterTypes(file *File, idx uint32) []string {
	params, ok := file.FlatFindChild(idx, "formal_parameters")
	if !ok {
		return nil
	}
	var out []string
	for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
		switch file.FlatType(p) {
		case "formal_parameter":
			out = append(out, javaDeclaredType(file, p)+javaDimensions(file, p))
		case "spread_parameter":
			out = append(out, javaDeclaredType(file, p)+"...")
		}
	}
	return out
}

// javaDimensions returns the normalized C-style array dimensions that follow a
// declarator name (`int a[]`, `String names[][]`), or "".
func javaDimensions(file *File, idx uint32) string {
	if dims, ok := file.FlatFindChild(idx, "dimensions"); ok {
		return normalizeTypeText(file.FlatNodeText(dims))
	}
	return ""
}

func javaMethodSignature(owner, name string, file *File, idx uint32, arity int, constructor bool) string {
	base := symbolSignature(owner, name, arity)
	ret := ""
	if !constructor {
		ret = javaDeclaredType(file, idx)
		if ret != "" {
			// C-style dimensions after the parameter list: `String f()[]`.
			ret += javaDimensions(file, idx)
		}
	}
	params := javaFormalParameterTypes(file, idx)
	if file.FlatType(idx) == "compact_constructor_declaration" {
		// A compact constructor has no parameter list of its own; it takes
		// the enclosing record's components.
		for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
			if file.FlatType(p) == "record_declaration" {
				params = javaFormalParameterTypes(file, p)
				break
			}
		}
	}
	return typedSignature(base, "", params, ret)
}

// javaClassSignature returns the class-like symbol signature: the FQN, plus
// the component types for a record so component type edits move the
// structural fingerprint.
func javaClassSignature(file *File, idx uint32, fqn string) string {
	if file.FlatType(idx) != "record_declaration" {
		return fqn
	}
	return typedSignature(fqn, "", javaFormalParameterTypes(file, idx), "")
}

// javaFieldSignature appends the declared field type (and the declarator's
// own C-style dimensions) to the field's owner#name/0 signature.
func javaFieldSignature(owner, name string, file *File, fieldDecl, declarator uint32) string {
	return typedSignature(symbolSignature(owner, name, 0), "", nil,
		javaDeclaredType(file, fieldDecl)+javaDimensions(file, declarator))
}
