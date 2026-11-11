package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

var trustManagerTypes = []string{"javax.net.ssl.X509TrustManager", "javax.net.ssl.X509ExtendedTrustManager"}

// CustomX509TrustManagerRule mirrors Lint's CustomX509TrustManager: any class implementing X509TrustManager.
type CustomX509TrustManagerRule struct {
	FlatDispatchBase
	AndroidRule
}

// TrustAllX509TrustManagerRule mirrors Lint's TrustAllX509TrustManager: checkServerTrusted or
// checkClientTrusted with an empty body.
type TrustAllX509TrustManagerRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: supertypes are matched by name and imports.
func (r *CustomX509TrustManagerRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence: supertypes are matched by name and imports.
func (r *TrustAllX509TrustManagerRule) Confidence() float64 { return api.ConfidenceMedium }

var trustManagerClassNodes = []string{"class_declaration", "object_literal", "object_creation_expression"}

func (r *CustomX509TrustManagerRule) check(ctx *api.Context) {
	file := ctx.File
	if !lintIsTrustManagerClass(file, ctx.Idx, 0) {
		return
	}
	at := lintClassNameNode(file, ctx.Idx)
	if at == 0 {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, at, "",
		"Implementing a custom `X509TrustManager` is error-prone and likely to be insecure. It is likely to disable certificate validation altogether, and is non-trivial to implement correctly without calling Android's default implementation."))
}

func (r *TrustAllX509TrustManagerRule) check(ctx *api.Context) {
	file := ctx.File
	if !lintIsTrustManagerClass(file, ctx.Idx, 0) {
		return
	}
	body, _ := file.FlatFindChild(ctx.Idx, "class_body")
	if body == 0 {
		return
	}
	for member := file.FlatFirstChild(body); member != 0; member = file.FlatNextSib(member) {
		var name, block uint32
		switch file.FlatType(member) {
		case "function_declaration":
			name, _ = file.FlatFindChild(member, "simple_identifier")
			if file.FlatFindModifierNode(member, "abstract") != 0 {
				continue
			}
			fb, ok := file.FlatFindChild(member, "function_body")
			if !ok {
				continue
			}
			block = fb
		case "method_declaration":
			name, _ = file.FlatFindChild(member, "identifier")
			if file.FlatFindModifierNode(member, "abstract") != 0 {
				continue
			}
			b, ok := file.FlatFindChild(member, "block")
			if !ok {
				continue
			}
			block = b
		default:
			continue
		}
		method := file.FlatNodeText(name)
		if method != "checkServerTrusted" && method != "checkClientTrusted" {
			continue
		}
		if !lintBodyIsTrivial(file, block) {
			continue
		}
		ctx.Emit(lintFindingAtNode(ctx, name, "",
			"`"+method+"` is empty, which could cause insecure network traffic due to trusting arbitrary TLS/SSL certificates presented by peers"))
	}
}

// lintBodyIsTrivial reports whether a method body holds nothing but empty blocks, comments, and bare returns.
func lintBodyIsTrivial(file *scanner.File, body uint32) bool {
	text := strings.TrimSpace(file.FlatNodeText(body))
	// An expression body such as `= Unit` is an expression, so it counts as non-empty.
	if !strings.HasPrefix(text, "{") {
		return false
	}
	trivial := true
	var walk func(idx uint32)
	walk = func(idx uint32) {
		for child := file.FlatFirstChild(idx); child != 0 && trivial; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			switch file.FlatType(child) {
			case "block", "statements", "function_body":
				walk(child)
			case "line_comment", "block_comment", "multiline_comment", "comment":
			case "return_statement", "jump_expression":
				if t := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(child)), ";")); t != "return" && !strings.HasPrefix(t, "return@") {
					trivial = false
				}
			default:
				if strings.TrimSpace(file.FlatNodeText(child)) != ";" {
					trivial = false
				}
			}
		}
	}
	walk(body)
	return trivial
}

// lintClassNameNode is the node Lint reports a class at: its name, the `object` keyword of a Kotlin object
// literal, or the instantiated type of a Java anonymous class.
func lintClassNameNode(file *scanner.File, idx uint32) uint32 {
	switch file.FlatType(idx) {
	case "class_declaration":
		for _, kind := range []string{"type_identifier", "identifier"} {
			if n, ok := file.FlatFindChild(idx, kind); ok {
				return n
			}
		}
	case "object_literal":
		return file.FlatFirstChild(idx)
	case "object_creation_expression":
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "type_identifier", "scoped_type_identifier", "generic_type":
				return child
			}
		}
	}
	return 0
}

const slotLintTrustClasses = "lintTrustManagerClasses"

// lintIsTrustManagerClass reports whether the class, object literal, or anonymous class has an X509TrustManager
// supertype, directly or through a class declared in the same file.
func lintIsTrustManagerClass(file *scanner.File, idx uint32, depth int) bool {
	return lintHasSupertype(file, idx, trustManagerTypes, false, depth)
}

// lintHasSupertype reports whether the class, object literal, or anonymous class has one of the supertypes fqns,
// directly or through a class declared in the same file. Interfaces are skipped unless interfaces is set.
func lintHasSupertype(file *scanner.File, idx uint32, fqns []string, interfaces bool, depth int) bool {
	if depth > 5 {
		return false
	}
	switch file.FlatType(idx) {
	case "class_declaration":
		if !interfaces && lintDeclaresInterface(file, idx) {
			return false
		}
	case "object_literal":
	case "object_creation_expression":
		if _, ok := file.FlatFindChild(idx, "class_body"); !ok {
			return false
		}
	default:
		return false
	}
	for _, super := range lintSupertypeNames(file, idx) {
		for _, fqn := range fqns {
			if lintTypeRefMatches(file, super, fqn) {
				return true
			}
		}
		if decl := lintSameFileClass(file, super); decl != 0 && decl != idx && lintHasSupertype(file, decl, fqns, interfaces, depth+1) {
			return true
		}
	}
	return false
}

func lintDeclaresInterface(file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "interface" {
			return true
		}
		if file.FlatType(child) == "class_body" || file.FlatType(child) == "primary_constructor" {
			break
		}
	}
	return false
}

// lintSupertypeNames lists the written supertype names of a class-like node.
func lintSupertypeNames(file *scanner.File, idx uint32) []string {
	var names []string
	add := func(node uint32) {
		text := strings.TrimSpace(file.FlatNodeText(node))
		if i := strings.IndexAny(text, "<("); i >= 0 {
			text = text[:i]
		}
		if text != "" {
			names = append(names, strings.TrimSpace(text))
		}
	}
	switch file.FlatType(idx) {
	case "object_creation_expression":
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "type_identifier", "scoped_type_identifier", "generic_type":
				add(child)
			}
		}
		return names
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "delegation_specifier":
			add(child)
		case "superclass", "super_interfaces":
			file.FlatWalkAllNodes(child, func(n uint32) {
				switch file.FlatType(n) {
				case "type_identifier", "scoped_type_identifier":
					if parent, ok := file.FlatParent(n); ok && file.FlatType(parent) != "scoped_type_identifier" && file.FlatType(parent) != "generic_type" {
						add(n)
					}
				case "generic_type":
					add(n)
				}
			})
		}
	}
	return names
}

// lintSameFileClass finds the class declaration with the given simple name in the file, or 0.
func lintSameFileClass(file *scanner.File, name string) uint32 {
	index := filefacts.FileFact(fileFactsCache(), file, slotLintTrustClasses, func() map[string]uint32 {
		out := map[string]uint32{}
		file.FlatWalkNodes(0, "class_declaration", func(decl uint32) {
			if n := lintClassNameNode(file, decl); n != 0 {
				out[file.FlatNodeText(n)] = decl
			}
		})
		return out
	})
	return index[name]
}
