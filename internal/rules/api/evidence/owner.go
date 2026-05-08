package evidence

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// ResolveOwner returns the FQN of the call's receiver type and the
// backend that proved it. Returns ("", OwnerUnknown) when no backend
// could prove an answer — rules that need receiver proof should bail
// out on that case rather than falling back to substring matching.
//
// Backends are tried in cheap-to-expensive order:
//  1. ImportEvidence: receiver is a type-name (`Foo.bar()`) named in
//     the file's imports. Free.
//  2. Resolver: source-level Kotlin scope/import resolution
//     (only when the rule declared NeedsResolver, otherwise nil).
//  3. JavaSource: walk the enclosing Java method/class for a
//     parameter, field, or local declaration of the receiver name,
//     then resolve via JavaFacts. Java-only; no-op for Kotlin.
//
// Per-call results are memoized for the lifetime of this Evidence.
func (e *Evidence) ResolveOwner(c *Call) (fqn string, source OwnerSource) {
	if e == nil || e.file == nil || c == nil {
		return "", OwnerUnknown
	}
	if c.Receiver == "" {
		return "", OwnerUnknown
	}
	if e.ownerCache != nil {
		if hit, ok := e.ownerCache[c.Idx]; ok {
			return hit.fqn, hit.source
		}
	}
	fqn, source = e.resolveOwnerUncached(c)
	if e.ownerCache == nil {
		e.ownerCache = make(map[uint32]ownerEntry)
	}
	e.ownerCache[c.Idx] = ownerEntry{fqn: fqn, source: source}
	return fqn, source
}

func (e *Evidence) resolveOwnerUncached(c *Call) (string, OwnerSource) {
	// Type-name receiver via import table — works without a resolver.
	if simple := lastSegment(c.Receiver); simple != "" && isLikelyTypeName(simple) {
		if e.resolver != nil {
			if fqn := e.resolver.ResolveImport(simple, e.file); fqn != "" {
				return fqn, OwnerImportEvidence
			}
		}
		if e.file.Language == scanner.LangJava && e.javaFacts != nil {
			if fqn := e.javaFacts.ResolveType(simple, e.javaIndex); fqn != "" && fqn != simple {
				return fqn, OwnerImportEvidence
			}
		}
	}

	// Resolver-backed name lookup — Kotlin scopes, parameters, etc.
	if e.resolver != nil {
		if t := e.resolver.ResolveByNameFlat(c.Receiver, c.Idx, e.file); t != nil {
			if t.FQN != "" {
				return t.FQN, OwnerResolver
			}
			if t.Name != "" {
				if fqn := e.resolver.ResolveImport(t.Name, e.file); fqn != "" {
					return fqn, OwnerResolver
				}
			}
		}
	}

	// Java AST + JavaFacts: find a parameter / field / local in scope
	// whose name matches the receiver and read its declared type.
	if e.file != nil && e.file.Language == scanner.LangJava {
		if typeName := javaReceiverDeclaredTypeName(e.file, c.Idx, c.Receiver); typeName != "" {
			if e.javaFacts != nil {
				if fqn := e.javaFacts.ResolveType(typeName, e.javaIndex); fqn != "" {
					return fqn, OwnerJavaSource
				}
			}
			return typeName, OwnerJavaSource
		}
	}

	return "", OwnerUnknown
}

// ResolveCalleeFQN returns the FQN of an unqualified call's callee using
// the file's import table. Useful for constructor-style patterns like
// `SimpleSQLiteQuery(...)` or `ProcessBuilder(...)` where the callee is
// the type name and the rule wants to verify which FQN it refers to.
//
// Returns ("", OwnerUnknown) when the callee is qualified (Foo.bar()),
// not import-resolvable, or empty. For qualified calls, use ResolveOwner
// against the receiver instead.
func (e *Evidence) ResolveCalleeFQN(c *Call) (fqn string, source OwnerSource) {
	if e == nil || e.file == nil || c == nil {
		return "", OwnerUnknown
	}
	if c.Callee == "" || c.Receiver != "" {
		return "", OwnerUnknown
	}
	if !isLikelyTypeName(c.Callee) {
		return "", OwnerUnknown
	}
	if e.resolver != nil {
		if resolved := e.resolver.ResolveImport(c.Callee, e.file); resolved != "" {
			return resolved, OwnerImportEvidence
		}
	}
	if e.file.Language == scanner.LangJava && e.javaFacts != nil {
		if resolved := e.javaFacts.ResolveType(c.Callee, e.javaIndex); resolved != "" && resolved != c.Callee {
			return resolved, OwnerImportEvidence
		}
	}
	return "", OwnerUnknown
}

func lastSegment(receiver string) string {
	if dot := strings.LastIndex(receiver, "."); dot >= 0 {
		return receiver[dot+1:]
	}
	return receiver
}

// isLikelyTypeName treats receivers starting with an uppercase ASCII
// letter as type names. Kotlin/Java naming convention. False positives
// (constants like `MAX_VALUE`) are harmless since the import lookup
// will simply miss.
func isLikelyTypeName(name string) bool {
	if name == "" {
		return false
	}
	c := name[0]
	return c >= 'A' && c <= 'Z'
}

// javaReceiverDeclaredTypeName walks up from idx to find a Java
// declaration of `name` (formal_parameter, field_declaration, or
// local_variable_declaration) and returns the textual type identifier.
// Returns "" when no in-scope declaration is found.
func javaReceiverDeclaredTypeName(file *scanner.File, idx uint32, name string) string {
	if file == nil || idx == 0 || name == "" {
		return ""
	}
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "method_declaration", "constructor_declaration":
			if t := javaParamType(file, current, name); t != "" {
				return t
			}
		case "class_body":
			if t := javaScanDecls(file, current, "field_declaration", name); t != "" {
				return t
			}
		case "block":
			if t := javaScanDecls(file, current, "local_variable_declaration", name); t != "" {
				return t
			}
		case "program":
			return ""
		}
	}
	return ""
}

// javaParamType scans a method/constructor declaration for a
// formal_parameter named `name`, returning its declared type identifier.
func javaParamType(file *scanner.File, decl uint32, name string) string {
	params, ok := file.FlatFindChild(decl, "formal_parameters")
	if !ok {
		return ""
	}
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "formal_parameter" {
			continue
		}
		var typeName, paramName string
		for part := file.FlatFirstChild(child); part != 0; part = file.FlatNextSib(part) {
			switch file.FlatType(part) {
			case "type_identifier", "scoped_type_identifier", "scoped_identifier", "generic_type":
				if typeName == "" {
					typeName = file.FlatNodeText(part)
				}
			case "identifier":
				paramName = file.FlatNodeText(part)
			}
		}
		if paramName == name {
			return typeName
		}
	}
	return ""
}

// javaScanDecls scans `parent` for child declarations of the given Java
// node type (`field_declaration` or `local_variable_declaration`) whose
// variable_declarator names `name`, returning the declared type identifier.
func javaScanDecls(file *scanner.File, parent uint32, declType, name string) string {
	for child := file.FlatFirstChild(parent); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != declType {
			continue
		}
		if t := javaDeclaredType(file, child, name); t != "" {
			return t
		}
	}
	return ""
}

// javaDeclaredType walks a field_declaration / local_variable_declaration
// to find a variable_declarator named `name` and returns the declaration's
// type identifier text. Returns "" if no matching declarator is present.
func javaDeclaredType(file *scanner.File, decl uint32, name string) string {
	var typeName string
	matched := false
	for part := file.FlatFirstChild(decl); part != 0; part = file.FlatNextSib(part) {
		switch file.FlatType(part) {
		case "type_identifier", "scoped_type_identifier", "scoped_identifier", "generic_type":
			if typeName == "" {
				typeName = file.FlatNodeText(part)
			}
		case "variable_declarator":
			for vc := file.FlatFirstChild(part); vc != 0; vc = file.FlatNextSib(vc) {
				if file.FlatType(vc) == "identifier" && file.FlatNodeText(vc) == name {
					matched = true
				}
			}
		}
	}
	if matched {
		return typeName
	}
	return ""
}
