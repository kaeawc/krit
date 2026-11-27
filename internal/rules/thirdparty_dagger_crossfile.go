package rules

// Ports of the two slack-lints 0.11.1 Dagger checks that need the whole project:
//
//   - DoNotCallProviders (DoNotCallProvidersDetector.kt) reports a call, from main sources, to a method annotated
//     @dagger.Binds, @dagger.Provides or @dagger.producers.Produces, unless the innermost class around the call is
//     annotated @Generated (javax.annotation or javax.annotation.processing).
//   - BindsTypeMismatch (DaggerIssuesDetector.kt, its last check) reports a @Binds method whose parameter type is not
//     assignable to its return type, when none of the detector's earlier checks reported the method.
//
// Lint resolves calls and types against the classpath. krit indexes the project's provider methods once per run
// (only files that mention one of the annotations), then matches a call whose name is a provider's through what the
// source shows: the class name (also `X.INSTANCE.m()` and `X.Companion.m()` from Java), the receiver's resolved or
// declared type, the receiver of an enclosing `with`/`run`/`apply` lambda, or for an unqualified call the enclosing
// class. Provider methods in libraries are not seen. BindsTypeMismatch walks project class hierarchies through the
// cross-file index and abstains whenever a library type could make the types assignable.

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

const (
	daggerDoNotCallProviders = "DoNotCallProviders"
	daggerBindsTypeMismatch  = "BindsTypeMismatch"
	fqcnDaggerProduces       = "dagger.producers.Produces"

	daggerDoNotCallProvidersMessage = "Dagger provider methods should not be called directly by user code."
	daggerBindsTypeMismatchMessage  = "@Binds parameter/return must be type-assignable"

	slotDaggerProviderCalls = "daggerProviderCalls"
	slotDaggerBindsMismatch = "daggerBindsMismatch"
)

var daggerProviderAnnotations = []string{fqcnDaggerBinds, fqcnDaggerProvides, fqcnDaggerProduces}

var daggerGeneratedAnnotations = []string{"javax.annotation.Generated", "javax.annotation.processing.Generated"}

// daggerProvider is a project method carrying a provider annotation.
type daggerProvider struct {
	name      string
	owner     string // FQN of the declaring class (a companion's members belong to its class)
	arity     slackArity
	overloads []slackArity // same-named methods of the same class body
}

// emitDaggerProviderCalls emits the run's DoNotCallProviders findings, computed once per CodeIndex.
func emitDaggerProviderCalls(ctx *api.Context) {
	index := ctx.CodeIndex
	if index == nil || len(index.Files) == 0 {
		return
	}
	// The test-source check reads paths relative to the project root, so the
	// memo is per index and root.
	findings := filefacts.StringFact(ctx.Facts, fmt.Sprintf("%p|%s", index, projectRoot(ctx)), slotDaggerProviderCalls, func() []mockFinding {
		return analyzeDaggerProviderCalls(ctx, index, ctx.Resolver)
	})
	for _, f := range findings {
		ctx.Emit(scanner.Finding{
			File:      f.file.Path,
			Line:      f.file.FlatRow(f.node) + 1,
			Col:       f.file.FlatCol(f.node) + 1,
			StartByte: int(f.file.FlatStartByte(f.node)),
			EndByte:   int(f.file.FlatEndByte(f.node)),
			Message:   f.msg,
		})
	}
}

func analyzeDaggerProviderCalls(ctx *api.Context, index *scanner.CodeIndex, resolver typeinfer.TypeResolver) []mockFinding {
	byName := map[string][]daggerProvider{}
	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil || !daggerMentionsProviderAnnotation(file.Content) {
			continue
		}
		for _, p := range daggerCollectProviders(file) {
			byName[p.name] = append(byName[p.name], p)
		}
	}
	if len(byName) == 0 {
		return nil
	}
	h := newSlackHier(index)
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	// Only files that mention a provider's name can call one.
	files := h.candidateFiles(names)
	var out []mockFinding
	for _, file := range files {
		if file == nil || file.FlatTree == nil || isProjectTestFile(ctx, file.Path) {
			continue
		}
		visit := func(idx uint32) {
			if file.FlatType(idx) == "call_expression" && file.FlatType(file.FlatFirstChild(idx)) == "call_expression" {
				// `f(x) { }` parses as a call whose callee is the call `f(x)`; the inner call is the one checked.
				return
			}
			call, ok := lintCallAt(file, idx)
			if !ok {
				return
			}
			for _, p := range byName[call.Name] {
				if daggerCallTargets(file, call, p, h, resolver) {
					if !daggerInGeneratedClass(file, idx) {
						out = append(out, mockFinding{file: file, node: idx, msg: daggerDoNotCallProvidersMessage})
					}
					return
				}
			}
		}
		if file.Language == scanner.LangJava {
			file.FlatWalkNodes(0, "method_invocation", visit)
		} else {
			file.FlatWalkNodes(0, "call_expression", visit)
		}
	}
	return out
}

func daggerMentionsProviderAnnotation(content []byte) bool {
	return bytes.Contains(content, []byte("Provides")) || bytes.Contains(content, []byte("Binds")) ||
		bytes.Contains(content, []byte("Produces"))
}

// daggerCollectProviders returns the file's Kotlin functions and Java methods annotated with a provider annotation.
func daggerCollectProviders(file *scanner.File) []daggerProvider {
	pkg := moshiFilePackage(file)
	var out []daggerProvider
	for _, kind := range []string{"function_declaration", "method_declaration"} {
		file.FlatWalkNodes(0, kind, func(fn uint32) {
			anns := moshiAnnotationsOf(file, fn)
			found := false
			for _, fqn := range daggerProviderAnnotations {
				if moshiFindAnnotation(file, anns, fqn) != nil {
					found = true
				}
			}
			if !found {
				return
			}
			owner, ok := slackDeclOwner(file, fn, pkg)
			if !ok || owner == "" {
				return
			}
			name := extractIdentifierFlat(file, fn)
			if kind == "method_declaration" {
				if id, ok := file.FlatFindChild(fn, "identifier"); ok {
					name = file.FlatNodeText(id)
				}
			}
			out = append(out, daggerProvider{name: name, owner: owner, arity: slackArityOf(file, fn),
				overloads: slackOverloadArities(file, fn, name, false)})
		})
	}
	return out
}

// daggerCallTargets reports whether a Kotlin call or Java method invocation resolves to the provider method.
func daggerCallTargets(file *scanner.File, call lintCall, p daggerProvider, h *slackHier, resolver typeinfer.TypeResolver) bool {
	index := h.index
	decl := slackAnnotatedDecl{name: p.name, owner: p.owner, arity: p.arity, overloads: p.overloads}
	if !slackArityPicks(file, call, decl) {
		return false
	}
	if file.Language == scanner.LangKotlin {
		if call.ReceiverNode == 0 {
			// `with(Module) { provide() }`, `Module.run { provide() }`: the scope lambda's receiver is the call's.
			if recv := daggerScopeReceiver(file, call.Idx); recv != "" {
				return slackClassFQN(index, file, recv) == p.owner
			}
		}
		return slackCallTargets(file, call, decl, h, resolver)
	}
	if call.ReceiverNode == 0 || call.ReceiverText == "this" {
		class, ok := flatEnclosingAncestor(file, call.Idx, "class_declaration", "interface_declaration", "enum_declaration")
		if !ok {
			return false
		}
		owner, _ := slackDeclOwner(file, class, moshiFilePackage(file))
		name := ""
		if id, ok := file.FlatFindChild(class, "identifier"); ok {
			name = file.FlatNodeText(id)
		}
		self := name
		if owner != "" {
			self = owner + "." + name
		} else if pkg := moshiFilePackage(file); pkg != "" {
			self = pkg + "." + name
		}
		return slackTypeExtends(h, self, p.owner)
	}
	// A Kotlin object's or companion's member called from Java: `Module.INSTANCE.m()`, `Module.Companion.m()`.
	recv := strings.Join(strings.Fields(call.ReceiverText), "")
	recv = strings.TrimSuffix(strings.TrimSuffix(recv, ".INSTANCE"), ".Companion")
	if slackClassFQN(index, file, recv) == p.owner {
		return true
	}
	if file.FlatType(call.ReceiverNode) == "identifier" {
		if fqn := slackClassFQN(index, file, daggerJavaVariableType(file, call.ReceiverNode)); fqn != "" {
			return slackTypeExtends(h, fqn, p.owner)
		}
	}
	if resolver != nil {
		if typ := resolver.ResolveFlatNode(call.ReceiverNode, file); typ != nil && typ.Name != "" {
			for _, name := range []string{typ.FQN, slackClassFQN(index, file, typ.Name)} {
				if name == "" {
					continue
				}
				if _, indexed := index.SymbolByFQN(name); indexed || name == p.owner {
					return slackTypeExtends(h, name, p.owner)
				}
			}
		}
	}
	return false
}

// daggerInGeneratedClass reports whether the innermost class around idx is annotated @Generated.
func daggerInGeneratedClass(file *scanner.File, idx uint32) bool {
	class, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_declaration", "interface_declaration",
		"enum_declaration", "companion_object", "object_literal", "class_body")
	if !ok {
		return false
	}
	if file.FlatType(class) == "class_body" {
		parent, _ := file.FlatParent(class)
		if file.FlatType(parent) == "object_creation_expression" {
			// A Java anonymous class is the innermost class, and it carries no annotation.
			return false
		}
		class = parent
	}
	anns := moshiAnnotationsOf(file, class)
	for _, fqn := range daggerGeneratedAnnotations {
		if moshiFindAnnotation(file, anns, fqn) != nil {
			return true
		}
	}
	return false
}

// daggerScopeReceiver returns the source text of the receiver an unqualified Kotlin call inside a `with(x) { }`,
// `x.run { }` or `x.apply { }` lambda is bound to, or "" when the innermost lambda around the call is not one.
func daggerScopeReceiver(file *scanner.File, idx uint32) string {
	lambda, ok := flatEnclosingAncestor(file, idx, "lambda_literal")
	if !ok {
		return ""
	}
	call := lambda
	for call != 0 && file.FlatType(call) != "call_expression" {
		call, _ = file.FlatParent(call)
	}
	if call == 0 {
		return ""
	}
	// `with(x) { }` parses as a call whose callee is the call `with(x)`.
	if inner := file.FlatFirstChild(call); file.FlatType(inner) == "call_expression" {
		if c, ok := lintCallAt(file, inner); ok && c.Name == "with" && c.ReceiverNode == 0 && len(c.Args) == 1 {
			return strings.TrimSpace(file.FlatNodeText(c.Args[0]))
		}
		return ""
	}
	if c, ok := lintCallAt(file, call); ok && (c.Name == "run" || c.Name == "apply") && c.ReceiverNode != 0 {
		return c.ReceiverText
	}
	return ""
}

// daggerJavaVariableType returns the declared type (without type arguments) of the Java parameter, local or field
// a name refers to, or "".
func daggerJavaVariableType(file *scanner.File, use uint32) string {
	name := file.FlatNodeText(use)
	typeOf := func(decl uint32) string {
		for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "type_identifier", "scoped_type_identifier", "generic_type":
				text := strings.Join(strings.Fields(file.FlatNodeText(c)), "")
				if i := strings.IndexByte(text, '<'); i >= 0 {
					text = text[:i]
				}
				return text
			}
		}
		return ""
	}
	declares := func(decl uint32) bool {
		for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "identifier":
				if file.FlatNodeText(c) == name {
					return true
				}
			case "variable_declarator":
				if id, ok := file.FlatFindChild(c, "identifier"); ok && file.FlatNodeText(id) == name {
					return true
				}
			}
		}
		return false
	}
	child := use
	for parent, ok := file.FlatParent(use); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "block", "class_body":
			for sib, found := file.FlatPrevSibling(child); found; sib, found = file.FlatPrevSibling(sib) {
				if t := file.FlatType(sib); (t == "local_variable_declaration" || t == "field_declaration") && declares(sib) {
					return typeOf(sib)
				}
			}
			if file.FlatType(parent) == "class_body" {
				for sib, found := file.FlatNextSibling(child); found; sib, found = file.FlatNextSibling(sib) {
					if file.FlatType(sib) == "field_declaration" && declares(sib) {
						return typeOf(sib)
					}
				}
			}
		case "method_declaration", "constructor_declaration", "lambda_expression":
			if params, ok := file.FlatFindChild(parent, "formal_parameters"); ok {
				for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
					if file.FlatType(p) == "formal_parameter" && declares(p) {
						return typeOf(p)
					}
				}
			}
		}
		child = parent
	}
	return ""
}

// emitDaggerBindsTypeMismatch emits the run's BindsTypeMismatch findings, computed once per CodeIndex.
func emitDaggerBindsTypeMismatch(ctx *api.Context) {
	index := ctx.CodeIndex
	if index == nil || len(index.Files) == 0 {
		return
	}
	findings := filefacts.StringFact(ctx.Facts, fmt.Sprintf("%p", index), slotDaggerBindsMismatch, func() []mockFinding {
		return analyzeDaggerBindsTypeMismatch(index)
	})
	for _, f := range findings {
		ctx.Emit(scanner.Finding{
			File:      f.file.Path,
			Line:      f.file.FlatRow(f.node) + 1,
			Col:       f.file.FlatCol(f.node) + 1,
			StartByte: int(f.file.FlatStartByte(f.node)),
			EndByte:   int(f.file.FlatEndByte(f.node)),
			Message:   f.msg,
		})
	}
}

func analyzeDaggerBindsTypeMismatch(index *scanner.CodeIndex) []mockFinding {
	h := newSlackHier(index)
	var out []mockFinding
	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil || !bytes.Contains(file.Content, []byte("Binds")) {
			continue
		}
		for _, kind := range []string{"function_declaration", "method_declaration"} {
			file.FlatWalkNodes(0, kind, func(fn uint32) {
				if daggerBindsMismatched(h, file, fn) {
					out = append(out, mockFinding{file: file, node: fn, msg: daggerBindsTypeMismatchMessage})
				}
			})
		}
	}
	return out
}

// daggerBindsMismatched reports whether fn is a @Binds method that reaches the detector's last check (no earlier
// issue) and whose parameter type the source proves is not assignable to its return type: the return type is a
// project class, and the parameter type is a project class whose project supertypes never reach it, or a library
// type, which cannot implement a project type.
func daggerBindsMismatched(h *slackHier, file *scanner.File, fn uint32) bool {
	if moshiFindAnnotation(file, moshiAnnotationsOf(file, fn), fqcnDaggerBinds) == nil {
		return false
	}
	if _, reported := daggerIssueOf(file, fn); reported {
		return false
	}
	param, ret := daggerBindsTypes(file, fn)
	if param == 0 || ret == 0 {
		return false
	}
	retText, retGeneric := daggerTypeName(file, ret)
	paramText, _ := daggerTypeName(file, param)
	// Assignability between parameterized types needs their type arguments; Lint compares those, krit abstains.
	if retGeneric || retText == "" || paramText == "" {
		return false
	}
	retFQN, _ := h.slackTypeNameKind(file, fn, retText)
	if retFQN == "" {
		return false
	}
	paramFQN, library := h.slackTypeNameKind(file, fn, paramText)
	if paramFQN == "" {
		// A library type cannot implement a project type; a name krit cannot place (a typealias) abstains.
		return library
	}
	extends, complete := slackTypeExtendsKnown(h, paramFQN, retFQN)
	return !extends && complete
}

// daggerBindsTypes returns a @Binds method's only parameter's type node and its return type node, or 0s.
func daggerBindsTypes(file *scanner.File, fn uint32) (param, ret uint32) {
	if file.Language == scanner.LangJava {
		params, ok := file.FlatFindChild(fn, "formal_parameters")
		if !ok {
			return 0, 0
		}
		for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
			if file.FlatType(p) == "formal_parameter" {
				param = daggerJavaTypeNode(file, p)
			}
		}
		return param, daggerJavaTypeNode(file, fn)
	}
	params, ok := file.FlatFindChild(fn, "function_value_parameters")
	if !ok {
		return 0, 0
	}
	for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
		if file.FlatType(p) == "parameter" {
			param = slackDeclaredTypeAfterColonOf(file, p)
		}
	}
	return param, slackDeclaredTypeAfterColonOf(file, fn)
}

func daggerJavaTypeNode(file *scanner.File, decl uint32) uint32 {
	for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "type_identifier", "scoped_type_identifier", "generic_type":
			return c
		}
	}
	return 0
}

// daggerTypeName returns the class name a type node names, without nullability or type arguments, and whether it
// has type arguments. A function, tuple or other non-class type gives "".
func daggerTypeName(file *scanner.File, typ uint32) (string, bool) {
	if file.FlatType(typ) == "nullable_type" {
		typ = file.FlatFirstChild(typ)
	}
	switch file.FlatType(typ) {
	case "user_type":
		var parts []string
		generic := false
		for c := file.FlatFirstChild(typ); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "type_identifier":
				parts = append(parts, file.FlatNodeText(c))
			case "type_arguments":
				generic = true
			}
		}
		return strings.Join(parts, "."), generic
	case "type_identifier", "scoped_type_identifier", "generic_type":
		text := strings.Join(strings.Fields(file.FlatNodeText(typ)), "")
		if i := strings.IndexByte(text, '<'); i >= 0 {
			return text[:i], true
		}
		return text, false
	}
	return "", false
}

// daggerLibraryTypes are the Kotlin and Java built-in types a file can name without an import.
var daggerLibraryTypes = map[string]bool{"String": true, "CharSequence": true, "Int": true, "Long": true, "Short": true,
	"Byte": true, "Double": true, "Float": true, "Boolean": true, "Char": true, "Any": true, "Number": true, "Unit": true,
	"List": true, "MutableList": true, "Set": true, "MutableSet": true, "Map": true, "MutableMap": true,
	"Collection": true, "Iterable": true, "Object": true, "Integer": true, "Character": true, "Runnable": true}
