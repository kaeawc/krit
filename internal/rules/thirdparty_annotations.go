package rules

// Ports of three slack-lints checks driven by annotations (slackhq/slack-lints 0.11.1,
// slack-lint-checks/src/main/java/slack/lint/): TestParameterSiteTargetDetector (TestParameterSiteTarget),
// MustUseNamedParamsDetector (MustUseNamedParams) and RestrictCallsToDetector (RestrictCallsTo).
//
// MustUseNamedParams and RestrictCallsTo resolve a call to a declaration annotated with slack-lint-annotations'
// @MustUseNamedParams or @RestrictCallsTo. One pass per run indexes the annotated project declarations, then
// matches Kotlin call sites (Lint 0.11.1 reports neither from Java callers): a top-level function through its
// import or package, a constructor through its class, a member through the receiver's type or the enclosing class.
// A call whose target the source does not show abstains.

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

const (
	slackTestParameterSiteTarget = "TestParameterSiteTarget"
	slackMustUseNamedParams      = "MustUseNamedParams"
	slackRestrictCallsTo         = "RestrictCallsTo"

	fqcnTestParameter      = "com.google.testing.junit.testparameterinjector.TestParameter"
	fqcnMustUseNamedParams = "slack.lint.annotations.MustUseNamedParams"
	fqcnRestrictCallsTo    = "slack.lint.annotations.RestrictCallsTo"

	slotSlackAnnotatedCalls = "slackAnnotatedCalls"
)

// --- TestParameterSiteTarget ---

func testParameterFileGate(file *scanner.File) bool {
	return bytes.Contains(file.Content, []byte("TestParameter"))
}

// checkTestParameterSiteTarget reports a @TestParameter on a constructor property (`val`/`var`) without the
// `param:` site target. Like Lint, it also accepts an annotation named TestParameter from another package.
func checkTestParameterSiteTarget(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	if !file.FlatHasChildOfType(idx, "binding_pattern_kind") {
		return
	}
	if parent, _ := file.FlatParent(idx); file.FlatType(parent) != "primary_constructor" {
		return
	}
	for _, ann := range moshiAnnotationsOf(file, idx) {
		name := ann.name[strings.LastIndexByte(ann.name, '.')+1:]
		if name != "TestParameter" && !moshiRefIs(file, ann.name, fqcnTestParameter) {
			continue
		}
		if ann.site != 0 && strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(ann.site)), ":") == "param" {
			continue
		}
		ctx.EmitAtNode(ann.node, "TestParameter annotation has the wrong site target")
	}
}

// --- MustUseNamedParams and RestrictCallsTo ---

// slackAnnotatedDecl is a project function or constructor carrying one of the annotations.
type slackAnnotatedDecl struct {
	annotation string
	name       string // function name, or the class's simple name for a constructor
	owner      string // FQN of the declaring class, "" for a top-level function
	pkg        string
	file       *scanner.File
	ctor       bool
	// arity is the declaration's argument count range; overloads are the ranges of same-named declarations of the
	// same owner in the same file (constructors: the class's other constructors).
	arity     slackArity
	overloads []slackArity
}

// slackArity is the number of arguments a function or constructor accepts: min (parameters without a default or
// vararg) to max (-1 with a vararg).
type slackArity struct{ min, max int }

func (a slackArity) accepts(n int) bool { return n >= a.min && (a.max < 0 || n <= a.max) }

type slackAnnotatedResult struct {
	index  *scanner.CodeIndex
	byRule map[string][]mockFinding
}

func slackAnnotatedCallsFor(ctx *api.Context) *slackAnnotatedResult {
	index := ctx.CodeIndex
	if index == nil || len(index.Files) == 0 {
		return &slackAnnotatedResult{}
	}
	return filefacts.StringFact(fileFactsCache(), fmt.Sprintf("%p", index), slotSlackAnnotatedCalls, func() *slackAnnotatedResult {
		return analyzeSlackAnnotatedCalls(index, ctx.Resolver)
	})
}

func emitSlackAnnotatedCalls(ctx *api.Context) {
	if ctx.Rule == nil {
		return
	}
	for _, f := range slackAnnotatedCallsFor(ctx).byRule[ctx.Rule.ID] {
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

func analyzeSlackAnnotatedCalls(index *scanner.CodeIndex, resolver typeinfer.TypeResolver) *slackAnnotatedResult {
	result := &slackAnnotatedResult{index: index, byRule: map[string][]mockFinding{}}
	byName := map[string][]slackAnnotatedDecl{}
	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if !bytes.Contains(file.Content, []byte("MustUseNamedParams")) && !bytes.Contains(file.Content, []byte("RestrictCallsTo")) {
			continue
		}
		for _, d := range slackCollectAnnotatedDecls(file) {
			byName[d.name] = append(byName[d.name], d)
		}
	}
	if len(byName) == 0 {
		return result
	}
	names := make([][]byte, 0, len(byName))
	for name := range byName {
		names = append(names, []byte(name))
	}
	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin || !slackMentionsAny(file.Content, names) {
			continue
		}
		file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
			// `f(x) { }` parses as a call whose callee is the call `f(x)`; the inner call is the one checked.
			if file.FlatType(file.FlatFirstChild(idx)) == "call_expression" {
				return
			}
			call, ok := lintCallAt(file, idx)
			if !ok {
				return
			}
			for _, d := range byName[call.Name] {
				if !slackCallTargets(file, call, d, index, resolver) {
					continue
				}
				switch d.annotation {
				case fqcnMustUseNamedParams:
					if !slackAllArgumentsNamed(call) {
						result.byRule[slackMustUseNamedParams] = append(result.byRule[slackMustUseNamedParams],
							mockFinding{file: file, node: idx, msg: "Calls to @MustUseNamedParams-annotated methods must name all parameters."})
					}
				case fqcnRestrictCallsTo:
					if !slackSameFile(file, d.file) {
						result.byRule[slackRestrictCallsTo] = append(result.byRule[slackRestrictCallsTo],
							mockFinding{file: file, node: idx, msg: "Methods annotated with @RestrictedCallsTo should only be called from the specified scope."})
					}
				}
				return
			}
		})
	}
	return result
}

func slackMentionsAny(content []byte, names [][]byte) bool {
	for _, n := range names {
		if bytes.Contains(content, n) {
			return true
		}
	}
	return false
}

// slackCollectAnnotatedDecls returns the file's functions and constructors annotated @MustUseNamedParams (Kotlin
// declarations only, as Lint skips Java methods) or @RestrictCallsTo.
func slackCollectAnnotatedDecls(file *scanner.File) []slackAnnotatedDecl {
	pkg := moshiFilePackage(file)
	var out []slackAnnotatedDecl
	add := func(decl uint32, name, owner string, ctor bool) {
		anns := moshiAnnotationsOf(file, decl)
		for _, fqn := range []string{fqcnMustUseNamedParams, fqcnRestrictCallsTo} {
			if fqn == fqcnMustUseNamedParams && file.Language != scanner.LangKotlin {
				continue
			}
			if moshiFindAnnotation(file, anns, fqn) != nil {
				out = append(out, slackAnnotatedDecl{annotation: fqn, name: name, owner: owner, pkg: pkg, file: file, ctor: ctor,
					arity: slackArityOf(file, decl), overloads: slackOverloadArities(file, decl, name, ctor)})
			}
		}
	}
	for _, kind := range []string{"function_declaration", "method_declaration"} {
		file.FlatWalkNodes(0, kind, func(fn uint32) {
			owner, ok := slackDeclOwner(file, fn, pkg)
			if !ok {
				return
			}
			name := extractIdentifierFlat(file, fn)
			if kind == "method_declaration" {
				if id, found := file.FlatFindChild(fn, "identifier"); found {
					name = file.FlatNodeText(id)
				}
			}
			add(fn, name, owner, false)
		})
	}
	for _, kind := range []string{"primary_constructor", "secondary_constructor"} {
		file.FlatWalkNodes(0, kind, func(ctor uint32) {
			class := ctor
			for class != 0 && file.FlatType(class) != "class_declaration" {
				class, _ = file.FlatParent(class)
			}
			if class == 0 {
				return
			}
			owner, ok := slackDeclOwner(file, class, pkg)
			if !ok {
				return
			}
			name := extractIdentifierFlat(file, class)
			fqn := name
			if owner != "" {
				fqn = owner + "." + name
			} else if pkg != "" {
				fqn = pkg + "." + name
			}
			add(ctor, name, fqn, true)
		})
	}
	return out
}

// slackDeclOwner returns the FQN of the class declaring a member, "" for a top-level declaration, or false for a
// local one.
func slackDeclOwner(file *scanner.File, decl uint32, pkg string) (string, bool) {
	var names []string
	parent, _ := file.FlatParent(decl)
	for n := parent; n != 0; n, _ = file.FlatParent(n) {
		switch file.FlatType(n) {
		case "class_declaration", "object_declaration", "interface_declaration", "enum_declaration":
			names = append([]string{extractIdentifierFlat(file, n)}, names...)
		case "companion_object":
		case "function_body", "block", "lambda_literal":
			return "", false
		}
	}
	if len(names) == 0 {
		return "", true
	}
	if pkg != "" {
		return pkg + "." + strings.Join(names, "."), true
	}
	return strings.Join(names, "."), true
}

// slackCallTargets reports whether a Kotlin call resolves to the annotated declaration.
func slackCallTargets(file *scanner.File, call lintCall, d slackAnnotatedDecl, index *scanner.CodeIndex, resolver typeinfer.TypeResolver) bool {
	if !slackArityPicks(file, call, d) {
		return false
	}
	switch {
	case d.ctor:
		if call.ReceiverText != "" {
			// `pkg.Outer.Inner(...)`, or `Outer.Inner(...)` with Outer imported.
			return call.ReceiverText+"."+call.Name == d.owner || slackClassFQN(index, file, call.ReceiverText+"."+call.Name) == d.owner
		}
		return slackClassFQN(index, file, call.Name) == d.owner
	case d.owner == "":
		if call.ReceiverText != "" {
			return call.ReceiverText == d.pkg
		}
		// A local function, a member of an enclosing class or of its project supertypes wins over any import.
		if slackCallShadowedByMember(file, call, index) {
			return false
		}
		imp := lintImportsOf(file)
		if full, ok := imp.bySimple[call.Name]; ok {
			return full == d.pkg+"."+call.Name
		}
		if moshiFilePackage(file) == d.pkg {
			return true
		}
		for _, star := range imp.stars {
			if star == d.pkg {
				return true
			}
		}
		return false
	}
	// A member: the receiver's type, or for an unqualified call the enclosing class, must be the owner or extend it.
	if call.ReceiverNode == 0 || call.ReceiverText == "this" {
		class, ok := flatEnclosingAncestor(file, call.Idx, "class_declaration", "object_declaration")
		if !ok {
			return false
		}
		owner, _ := slackDeclOwner(file, class, moshiFilePackage(file))
		name := extractIdentifierFlat(file, class)
		self := name
		if owner != "" {
			self = owner + "." + name
		} else if pkg := moshiFilePackage(file); pkg != "" {
			self = pkg + "." + name
		}
		return slackTypeExtends(index, self, d.owner)
	}
	if slackClassFQN(index, file, call.ReceiverText) == d.owner {
		return true // an object or companion member called through its class
	}
	// Candidates for the receiver's class, best first: the resolver's FQN, its simple name resolved in this file,
	// and the declared type of the parameter or property the receiver names (which the resolver may not type).
	var candidates []string
	if resolver != nil {
		if typ := resolver.ResolveFlatNode(call.ReceiverNode, file); typ != nil && typ.Name != "" {
			candidates = append(candidates, typ.FQN, slackClassFQN(index, file, typ.Name))
		}
	}
	if file.FlatType(call.ReceiverNode) == "simple_identifier" {
		candidates = append(candidates, slackClassFQN(index, file, slackKotlinDeclaredTypeText(file, call.ReceiverNode)))
	}
	for _, name := range candidates {
		if name == "" {
			continue
		}
		if _, indexed := index.SymbolByFQN(name); indexed || name == d.owner {
			return slackTypeExtends(index, name, d.owner)
		}
	}
	return false
}

// slackKotlinDeclaredTypeText returns the declared type (without type arguments or nullability) of the Kotlin
// parameter, local, property or constructor property a name refers to, or "". The nearest declaration wins.
func slackKotlinDeclaredTypeText(file *scanner.File, use uint32) string {
	name := file.FlatNodeText(use)
	typeText := func(typ uint32) string {
		if typ == 0 {
			return ""
		}
		if file.FlatType(typ) == "nullable_type" {
			typ = file.FlatFirstChild(typ)
		}
		if file.FlatType(typ) != "user_type" {
			return ""
		}
		var parts []string
		for c := file.FlatFirstChild(typ); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "type_identifier" {
				parts = append(parts, file.FlatNodeText(c))
			}
		}
		return strings.Join(parts, ".")
	}
	property := func(decl uint32) (string, bool) {
		if file.FlatType(decl) != "property_declaration" {
			return "", false
		}
		v, ok := file.FlatFindChild(decl, "variable_declaration")
		if !ok || extractIdentifierFlat(file, v) != name {
			return "", false
		}
		return typeText(slackDeclaredTypeAfterColonOf(file, v)), true
	}
	child := use
	for parent, ok := file.FlatParent(use); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "statements":
			for sib, found := file.FlatPrevSibling(child); found; sib, found = file.FlatPrevSibling(sib) {
				if t, matched := property(sib); matched {
					return t
				}
			}
		case "class_body", "source_file":
			for m := file.FlatFirstChild(parent); m != 0; m = file.FlatNextSib(m) {
				if t, matched := property(m); matched {
					return t
				}
			}
		case "function_declaration", "secondary_constructor", "anonymous_function":
			if params, ok := file.FlatFindChild(parent, "function_value_parameters"); ok {
				for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
					if file.FlatType(p) == "parameter" && extractIdentifierFlat(file, p) == name {
						return typeText(slackDeclaredTypeAfterColonOf(file, p))
					}
				}
			}
		case "class_declaration":
			if ctor, ok := file.FlatFindChild(parent, "primary_constructor"); ok {
				for p := file.FlatFirstChild(ctor); p != 0; p = file.FlatNextSib(p) {
					if file.FlatType(p) == "class_parameter" && extractIdentifierFlat(file, p) == name {
						return typeText(slackDeclaredTypeAfterColonOf(file, p))
					}
				}
			}
		case "lambda_literal":
			return ""
		}
		child = parent
	}
	return ""
}

// slackDeclaredTypeAfterColonOf returns the type node after the `:` of a Kotlin declaration, or 0.
func slackDeclaredTypeAfterColonOf(file *scanner.File, decl uint32) uint32 {
	afterColon := false
	for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == ":" {
			afterColon = true
			continue
		}
		if afterColon && file.FlatIsNamed(c) {
			return c
		}
	}
	return 0
}

// slackClassFQN resolves a class name written in file to the FQN of an indexed class: in full, through an import
// (of the class or of its outer class), in the file's package, or through an on-demand import. It returns "" when
// the name is not a project class the source shows.
func slackClassFQN(index *scanner.CodeIndex, file *scanner.File, text string) string {
	text = strings.Join(strings.Fields(text), "")
	if text == "" {
		return ""
	}
	if _, ok := index.SymbolByFQN(text); ok {
		return text
	}
	head, rest, nested := strings.Cut(text, ".")
	imp := lintImportsOf(file)
	if full, ok := imp.bySimple[head]; ok {
		if nested {
			full += "." + rest
		}
		if _, ok := index.SymbolByFQN(full); ok {
			return full
		}
		return ""
	}
	candidates := []string{}
	if pkg := moshiFilePackage(file); pkg != "" {
		candidates = append(candidates, pkg+"."+text)
	}
	for _, star := range imp.stars {
		candidates = append(candidates, star+"."+text)
	}
	for _, c := range candidates {
		if _, ok := index.SymbolByFQN(c); ok {
			return c
		}
	}
	return ""
}

// slackTypeExtends reports whether the type (FQN, or simple name when the resolver gives no FQN) is owner or a
// project class whose supertypes reach it. Supertypes are read from each class's declaration and resolved through
// that file's imports and package, not by simple name, so a same-named class elsewhere does not match.
func slackTypeExtends(index *scanner.CodeIndex, name, owner string) bool {
	seen := map[string]bool{}
	var visit func(string) bool
	visit = func(fqn string) bool {
		if fqn == "" || seen[fqn] {
			return false
		}
		seen[fqn] = true
		if fqn == owner {
			return true
		}
		for _, super := range slackDirectSupertypes(index, fqn) {
			if visit(super) {
				return true
			}
		}
		return false
	}
	return visit(name)
}

// slackDirectSupertypes returns the FQNs of an indexed class's direct supertypes that are project classes.
func slackDirectSupertypes(index *scanner.CodeIndex, fqn string) []string {
	sym, ok := index.SymbolByFQN(fqn)
	if !ok {
		return nil
	}
	var file *scanner.File
	for _, f := range index.Files {
		if f != nil && f.Path == sym.File {
			file = f
			break
		}
	}
	if file == nil || file.FlatTree == nil {
		return nil
	}
	decl, ok := file.FlatNamedDescendantForByteRange(uint32(sym.StartByte), uint32(sym.EndByte))
	if !ok {
		return nil
	}
	var out []string
	file.FlatForEachChild(decl, func(child uint32) {
		switch file.FlatType(child) {
		case "delegation_specifier", "delegation_specifiers", "superclass", "super_interfaces", "extends_interfaces":
		default:
			return
		}
		file.FlatWalkAllNodes(child, func(n uint32) {
			switch file.FlatType(n) {
			case "user_type", "type_identifier", "scoped_type_identifier":
			default:
				return
			}
			if parent, _ := file.FlatParent(n); file.FlatType(parent) == "user_type" || file.FlatType(parent) == "type_arguments" ||
				file.FlatType(parent) == "type_projection" || file.FlatType(parent) == "scoped_type_identifier" {
				return
			}
			text := strings.Join(strings.Fields(file.FlatNodeText(n)), "")
			if i := strings.IndexByte(text, '<'); i >= 0 {
				text = text[:i]
			}
			if super := slackClassFQN(index, file, text); super != "" {
				out = append(out, super)
			}
		})
	})
	return out
}

// slackAllArgumentsNamed reports whether every argument in the call's parentheses is named (a trailing lambda is
// outside them).
func slackAllArgumentsNamed(call lintCall) bool {
	for _, label := range call.Labels {
		if label == "" {
			return false
		}
	}
	return true
}

// slackSameFile mirrors Lint's file comparison: the same package and the same file name.
func slackSameFile(a, b *scanner.File) bool {
	return moshiFilePackage(a) == moshiFilePackage(b) && filepath.Base(a.Path) == filepath.Base(b.Path)
}

// slackArityPicks reports whether the call's argument count fits the declaration and no same-named overload of it,
// so Kotlin and Lint resolve the call to this declaration; with an overload that also fits, krit abstains.
func slackArityPicks(file *scanner.File, call lintCall, d slackAnnotatedDecl) bool {
	n := len(call.Args)
	if file.Language == scanner.LangKotlin {
		// A trailing lambda is an argument too; `f(x) { }` puts it on the call wrapping this one.
		if slackHasTrailingLambda(file, call.Idx) {
			n++
		}
		if parent, ok := file.FlatParent(call.Idx); ok && file.FlatType(parent) == "call_expression" && file.FlatFirstChild(parent) == call.Idx &&
			slackHasTrailingLambda(file, parent) {
			n++
		}
	}
	if !d.arity.accepts(n) {
		return false
	}
	for _, o := range d.overloads {
		if o.accepts(n) {
			return false
		}
	}
	return true
}

// slackArityOf returns the argument count range of a Kotlin function, secondary or primary constructor, or a Java
// method or constructor.
func slackArityOf(file *scanner.File, decl uint32) slackArity {
	list := decl
	for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
		if t := file.FlatType(c); t == "function_value_parameters" || t == "formal_parameters" || t == "class_parameters" {
			list = c
		}
	}
	a := slackArity{}
	pendingVararg := false
	for c := file.FlatFirstChild(list); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "parameter_modifiers":
			pendingVararg = strings.Contains(file.FlatNodeText(c), "vararg")
		case "parameter", "class_parameter", "formal_parameter", "spread_parameter":
			vararg := pendingVararg || file.FlatType(c) == "spread_parameter" || strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(c)), "vararg")
			pendingVararg = false
			_, inlineDefault := file.FlatFindChild(c, "=")
			next, _ := file.FlatNextSibling(c)
			hasDefault := inlineDefault || file.FlatType(next) == "="
			if vararg {
				a.max = -1
				continue
			}
			if a.max >= 0 {
				a.max++
			}
			if !hasDefault {
				a.min++
			}
		}
	}
	return a
}

// slackOverloadArities returns the argument count ranges of decl's same-named siblings: the other functions or
// methods of the same class body or file, or for a constructor the class's other constructors.
func slackOverloadArities(file *scanner.File, decl uint32, name string, ctor bool) []slackArity {
	var out []slackArity
	if ctor {
		class := decl
		for class != 0 && file.FlatType(class) != "class_declaration" {
			class, _ = file.FlatParent(class)
		}
		if class == 0 {
			return nil
		}
		if pc, ok := file.FlatFindChild(class, "primary_constructor"); ok && pc != decl {
			out = append(out, slackArityOf(file, pc))
		}
		if body, ok := file.FlatFindChild(class, "class_body"); ok {
			for m := file.FlatFirstChild(body); m != 0; m = file.FlatNextSib(m) {
				if (file.FlatType(m) == "secondary_constructor" || file.FlatType(m) == "constructor_declaration") && m != decl {
					out = append(out, slackArityOf(file, m))
				}
			}
		}
		return out
	}
	parent, ok := file.FlatParent(decl)
	if !ok {
		return nil
	}
	for m := file.FlatFirstChild(parent); m != 0; m = file.FlatNextSib(m) {
		if m == decl || file.FlatType(m) != file.FlatType(decl) {
			continue
		}
		other := extractIdentifierFlat(file, m)
		if file.FlatType(m) == "method_declaration" {
			if id, ok := file.FlatFindChild(m, "identifier"); ok {
				other = file.FlatNodeText(id)
			}
		}
		if other == name {
			out = append(out, slackArityOf(file, m))
		}
	}
	return out
}

// slackCallShadowedByMember reports whether an unqualified Kotlin call's name is a local function in scope, a member
// function of an enclosing class, or a member of one of that class's project supertypes, any of which Kotlin
// resolves before a top-level function.
func slackCallShadowedByMember(file *scanner.File, call lintCall, index *scanner.CodeIndex) bool {
	child := call.Idx
	for parent, ok := file.FlatParent(call.Idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "statements":
			for sib, found := file.FlatPrevSibling(child); found; sib, found = file.FlatPrevSibling(sib) {
				if file.FlatType(sib) == "function_declaration" && extractIdentifierFlat(file, sib) == call.Name {
					return true
				}
			}
		case "class_declaration", "object_declaration":
			if slackClassDeclaresFunction(file, parent, call.Name) {
				return true
			}
			owner, _ := slackDeclOwner(file, parent, moshiFilePackage(file))
			self := extractIdentifierFlat(file, parent)
			if owner != "" {
				self = owner + "." + self
			} else if pkg := moshiFilePackage(file); pkg != "" {
				self = pkg + "." + self
			}
			if slackSupertypeDeclaresFunction(index, self, call.Name, map[string]bool{}) {
				return true
			}
		}
		child = parent
	}
	return false
}

func slackClassDeclaresFunction(file *scanner.File, class uint32, name string) bool {
	body, ok := file.FlatFindChild(class, "class_body")
	if !ok {
		return false
	}
	for m := file.FlatFirstChild(body); m != 0; m = file.FlatNextSib(m) {
		if file.FlatType(m) == "function_declaration" && extractIdentifierFlat(file, m) == name {
			return true
		}
	}
	return false
}

// slackSupertypeDeclaresFunction reports whether a project supertype of the class fqn declares a function called
// name.
func slackSupertypeDeclaresFunction(index *scanner.CodeIndex, fqn, name string, seen map[string]bool) bool {
	for _, super := range slackDirectSupertypes(index, fqn) {
		if seen[super] {
			continue
		}
		seen[super] = true
		sym, ok := index.SymbolByFQN(super)
		if !ok {
			continue
		}
		for _, f := range index.Files {
			if f == nil || f.Path != sym.File || f.FlatTree == nil {
				continue
			}
			if decl, ok := f.FlatNamedDescendantForByteRange(uint32(sym.StartByte), uint32(sym.EndByte)); ok && slackClassDeclaresFunction(f, decl, name) {
				return true
			}
		}
		if slackSupertypeDeclaresFunction(index, super, name, seen) {
			return true
		}
	}
	return false
}

// slackHasTrailingLambda reports whether a Kotlin call passes a lambda after its parentheses.
func slackHasTrailingLambda(file *scanner.File, call uint32) bool {
	suffix, ok := file.FlatFindChild(call, "call_suffix")
	if !ok {
		return false
	}
	_, ok = file.FlatFindChild(suffix, "annotated_lambda")
	return ok
}
