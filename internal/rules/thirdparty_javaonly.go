package rules

// Port of slack-lints' JavaOnlyDetector (slackhq/slack-lints 0.11.1, slack-lint-checks/src/main/java/slack/lint/
// JavaOnlyDetector.kt): JavaOnlyDetector checks Kotlin sources against slack-lint-annotations' @JavaOnly and
// @KotlinOnly. It reports a class or function annotated with both; a class that subclasses or implements an annotated
// type, or a function that overrides an annotated function, without repeating the annotation; an anonymous object
// or SAM-constructor lambda of a @JavaOnly type (unless returned from a @JavaOnly function); and a call or callable
// reference to a @JavaOnly function or a member of a @JavaOnly class.
//
// Lint resolves each element. krit indexes the project's annotated declarations (Kotlin and Java) once per run and
// matches through what the source shows: supertypes as each declaring file names them, a call through its class name,
// receiver type or enclosing class (shared with MustUseNamedParams). Annotated library declarations are not seen, and
// a lambda passed where a @JavaOnly interface is expected (rather than through its SAM constructor) is not checked.

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

const (
	slackJavaOnlyDetector = "JavaOnlyDetector"
	fqcnJavaOnly          = "slack.lint.annotations.JavaOnly"
	fqcnKotlinOnly        = "slack.lint.annotations.KotlinOnly"
	slotSlackJavaOnly     = "slackJavaOnly"
)

// javaOnlyClass is a project class and the annotations it carries.
type javaOnlyClass struct {
	fqn, display, fileName string
	javaOnly, kotlinOnly   bool
	reason                 string
	file                   *scanner.File
	decl                   uint32
}

// javaOnlyMethod is a project function or method, annotated or in a @JavaOnly class.
type javaOnlyMethod struct {
	decl                 slackAnnotatedDecl
	javaOnly, kotlinOnly bool
	reason               string
}

type javaOnlyIndex struct {
	classes map[string]*javaOnlyClass
	methods map[string][]javaOnlyMethod // by name
}

func emitJavaOnlyFindings(ctx *api.Context) {
	index := ctx.CodeIndex
	if index == nil || len(index.Files) == 0 {
		return
	}
	findings := filefacts.StringFact(fileFactsCache(), fmt.Sprintf("%p", index), slotSlackJavaOnly, func() []mockFinding {
		return analyzeJavaOnly(index, ctx.Resolver)
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

func javaOnlyMentions(content []byte) bool {
	return bytes.Contains(content, []byte("JavaOnly")) || bytes.Contains(content, []byte("KotlinOnly"))
}

func analyzeJavaOnly(index *scanner.CodeIndex, resolver typeinfer.TypeResolver) []mockFinding {
	ix := &javaOnlyIndex{classes: map[string]*javaOnlyClass{}, methods: map[string][]javaOnlyMethod{}}
	for _, file := range index.Files {
		if file != nil && file.FlatTree != nil && javaOnlyMentions(file.Content) {
			ix.collect(file)
		}
	}
	if len(ix.classes) == 0 && len(ix.methods) == 0 {
		return nil
	}
	var names [][]byte
	for _, c := range ix.classes {
		names = append(names, []byte(c.display[strings.LastIndexByte(c.display, '.')+1:]))
	}
	for name := range ix.methods {
		names = append(names, []byte(name))
	}
	var out []mockFinding
	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin {
			continue
		}
		if !javaOnlyMentions(file.Content) && !slackMentionsAny(file.Content, names) {
			continue
		}
		s := &javaOnlyScan{ix: ix, index: index, resolver: resolver, file: file}
		s.scan()
		out = append(out, s.out...)
	}
	return out
}

var javaOnlyReasonRe = regexp.MustCompile(`reason\s*=\s*"((?:[^"\\]|\\.)*)"`)

// javaOnlyAnnotations reads @JavaOnly/@KotlinOnly on decl and the @JavaOnly reason (only a `reason =` argument, as
// Lint reads it).
func javaOnlyAnnotations(file *scanner.File, decl uint32) (javaOnly, kotlinOnly bool, reason string) {
	anns := moshiAnnotationsOf(file, decl)
	if a := moshiFindAnnotation(file, anns, fqcnJavaOnly); a != nil {
		javaOnly = true
		if m := javaOnlyReasonRe.FindStringSubmatch(file.FlatNodeText(a.node)); m != nil {
			reason = m[1]
		}
	}
	kotlinOnly = moshiFindAnnotation(file, anns, fqcnKotlinOnly) != nil
	return javaOnly, kotlinOnly, reason
}

var javaOnlyClassKinds = []string{"class_declaration", "object_declaration", "interface_declaration", "enum_declaration",
	"record_declaration"}

func (ix *javaOnlyIndex) collect(file *scanner.File) {
	pkg := moshiFilePackage(file)
	base := file.Path[strings.LastIndexAny(file.Path, `/\`)+1:]
	for _, kind := range javaOnlyClassKinds {
		file.FlatWalkNodes(0, kind, func(decl uint32) {
			owner, ok := slackDeclOwner(file, decl, pkg)
			name := javaOnlyDeclName(file, decl)
			if !ok || name == "" {
				return
			}
			display := name
			if owner != "" {
				display = strings.TrimPrefix(owner, pkg+".") + "." + name
			}
			fqn := display
			if pkg != "" {
				fqn = pkg + "." + display
			}
			j, k, reason := javaOnlyAnnotations(file, decl)
			ix.classes[fqn] = &javaOnlyClass{fqn: fqn, display: display, fileName: base, javaOnly: j, kotlinOnly: k,
				reason: reason, file: file, decl: decl}
		})
	}
	for _, kind := range []string{"function_declaration", "method_declaration"} {
		file.FlatWalkNodes(0, kind, func(fn uint32) {
			owner, ok := slackDeclOwner(file, fn, pkg)
			if !ok {
				return
			}
			name := javaOnlyDeclName(file, fn)
			j, k, reason := javaOnlyAnnotations(file, fn)
			if !j && !k && !ix.ownerIsJavaOnly(file, fn) {
				return
			}
			ix.methods[name] = append(ix.methods[name], javaOnlyMethod{
				decl: slackAnnotatedDecl{name: name, owner: owner, pkg: pkg, file: file,
					arity: slackArityOf(file, fn), overloads: slackOverloadArities(file, fn, name, false)},
				javaOnly: j, kotlinOnly: k, reason: reason,
			})
		})
	}
}

// ownerIsJavaOnly reports whether the class directly declaring fn is annotated @JavaOnly.
func (ix *javaOnlyIndex) ownerIsJavaOnly(file *scanner.File, fn uint32) bool {
	cls, ok := flatEnclosingAncestor(file, fn, javaOnlyClassKinds...)
	if !ok {
		return false
	}
	j, _, _ := javaOnlyAnnotations(file, cls)
	return j
}

func javaOnlyDeclName(file *scanner.File, decl uint32) string {
	if file.Language == scanner.LangJava {
		if id, ok := file.FlatFindChild(decl, "identifier"); ok {
			return file.FlatNodeText(id)
		}
		return ""
	}
	switch file.FlatType(decl) {
	case "class_declaration", "object_declaration":
		if id, ok := file.FlatFindChild(decl, "type_identifier"); ok {
			return file.FlatNodeText(id)
		}
		return ""
	}
	return extractIdentifierFlat(file, decl)
}

type javaOnlyScan struct {
	ix       *javaOnlyIndex
	index    *scanner.CodeIndex
	resolver typeinfer.TypeResolver
	file     *scanner.File
	out      []mockFinding
}

func (s *javaOnlyScan) report(node uint32, msg string) {
	s.out = append(s.out, mockFinding{file: s.file, node: node, msg: msg})
}

func (s *javaOnlyScan) scan() {
	file := s.file
	for _, kind := range []string{"class_declaration", "object_declaration"} {
		file.FlatWalkNodes(0, kind, s.checkClass)
	}
	file.FlatWalkNodes(0, "object_literal", s.checkAnonymous)
	file.FlatWalkNodes(0, "function_declaration", s.checkFunction)
	file.FlatWalkNodes(0, "call_expression", s.checkCall)
	file.FlatWalkNodes(0, "callable_reference", s.checkReference)
}

// classFQN returns the FQN of a class declared in the scanned file.
func (s *javaOnlyScan) classFQN(decl uint32) string {
	pkg := moshiFilePackage(s.file)
	owner, _ := slackDeclOwner(s.file, decl, pkg)
	name := javaOnlyDeclName(s.file, decl)
	switch {
	case owner != "":
		return owner + "." + name
	case pkg != "":
		return pkg + "." + name
	}
	return name
}

// checkClass reports a class annotated with both, or one that subclasses or implements an annotated project type
// without carrying the annotation itself (KotlinOnly is checked first, then JavaOnly, the superclass before the
// interfaces, as in Lint).
func (s *javaOnlyScan) checkClass(decl uint32) {
	j, k, _ := javaOnlyAnnotations(s.file, decl)
	// Lint's location for a class starts at its KDoc; krit reports at the declaration, inside the range a
	// @Suppress on the class covers.
	if j && k {
		s.report(decl, "Cannot annotate types with both `@KotlinOnly` and `@JavaOnly`")
		return
	}
	if j || k {
		return
	}
	supers := slackDirectSupertypes(s.index, s.classFQN(decl))
	for _, want := range []string{"KotlinOnly", "JavaOnly"} {
		for _, super := range supers {
			c := s.ix.classes[super]
			if c == nil || (want == "KotlinOnly" && !c.kotlinOnly) || (want == "JavaOnly" && !c.javaOnly) {
				continue
			}
			s.report(decl, "Type subclasses/implements "+c.display+" in "+c.fileName+" which is annotated @"+want+
				", it should also be annotated.")
			return
		}
	}
}

// checkAnonymous reports an object expression of a @JavaOnly type, unless it is returned from a @JavaOnly function.
func (s *javaOnlyScan) checkAnonymous(obj uint32) {
	file := s.file
	for c := file.FlatFirstChild(obj); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) != "delegation_specifier" {
			continue
		}
		cls := s.ix.classes[slackClassFQN(s.index, file, javaOnlyDelegationType(file, c))]
		if cls == nil || !cls.javaOnly {
			continue
		}
		if s.returnedFromJavaOnlyFunction(obj) {
			return
		}
		s.report(obj, "Cannot create anonymous instances of @JavaOnly-annotated type "+cls.display+" (in "+cls.fileName+
			") in Kotlin. Make a concrete class instead.")
		return
	}
}

// returnedFromJavaOnlyFunction reports whether expr is returned (`return expr`, or an expression body) from a
// function that is @JavaOnly or declared in a @JavaOnly class.
func (s *javaOnlyScan) returnedFromJavaOnlyFunction(expr uint32) bool {
	file := s.file
	parent, ok := file.FlatParent(expr)
	if !ok {
		return false
	}
	returned := false
	switch file.FlatType(parent) {
	case "jump_expression":
		returned = strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(parent)), "return")
	case "function_body":
		returned = !strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(parent)), "{")
	}
	if !returned {
		return false
	}
	fn, ok := flatEnclosingAncestor(file, expr, "function_declaration")
	if !ok {
		return false
	}
	j, _, _ := javaOnlyAnnotations(file, fn)
	return j || s.ix.ownerIsJavaOnly(file, fn)
}

// checkFunction reports a function annotated with both, or an override of an annotated project function that does
// not repeat the annotation.
func (s *javaOnlyScan) checkFunction(fn uint32) {
	file := s.file
	j, k, _ := javaOnlyAnnotations(file, fn)
	if j && k {
		s.report(fn, "Cannot annotate functions with both `@KotlinOnly` and `@JavaOnly`")
		return
	}
	if j || k || file.FlatFindModifierNode(fn, "override") == 0 {
		return
	}
	cls, ok := flatEnclosingAncestor(file, fn, "class_declaration", "object_declaration")
	if !ok {
		return
	}
	name := extractIdentifierFlat(file, fn)
	if m, owner := s.superMethod(s.classFQN(cls), name, map[string]bool{}); m != nil {
		want := "JavaOnly"
		if m.kotlinOnly {
			want = "KotlinOnly"
		}
		s.report(fn, "Function overrides "+name+" in "+owner.display+" which is annotated @"+want+
			", it should also be annotated.")
	}
}

// superMethod returns the nearest annotated method called name of a project supertype of the class fqn, and the
// supertype declaring it.
func (s *javaOnlyScan) superMethod(fqn, name string, seen map[string]bool) (*javaOnlyMethod, *javaOnlyClass) {
	supers := slackDirectSupertypes(s.index, fqn)
	for _, super := range supers {
		for i := range s.ix.methods[name] {
			m := &s.ix.methods[name][i]
			if m.decl.owner == super && (m.javaOnly || m.kotlinOnly) {
				return m, s.ix.classes[super]
			}
		}
	}
	for _, super := range supers {
		if seen[super] {
			continue
		}
		seen[super] = true
		if m, owner := s.superMethod(super, name, seen); m != nil {
			return m, owner
		}
	}
	return nil, nil
}

// checkCall reports a call to a @JavaOnly function or a member of a @JavaOnly class, and a SAM-constructor lambda of
// a @JavaOnly interface (or of one whose function is @JavaOnly).
func (s *javaOnlyScan) checkCall(idx uint32) {
	file := s.file
	if file.FlatType(file.FlatFirstChild(idx)) == "call_expression" {
		return
	}
	call, ok := lintCallAt(file, idx)
	if !ok {
		return
	}
	if lambda := slackTrailingLambdaNode(file, idx); lambda != 0 && len(call.Args) == 0 && call.ReceiverNode == 0 {
		// A SAM constructor exists for a Java interface (a Kotlin `fun interface` aside, which is not checked).
		if cls := s.ix.classes[slackClassFQN(s.index, file, call.Name)]; cls != nil && cls.file.Language == scanner.LangJava {
			if cls.javaOnly {
				s.report(lambda, "Cannot create lambda instances of @JavaOnly-annotated type "+cls.display+" (in "+cls.fileName+
					") in Kotlin. Make a concrete class instead.")
				return
			}
			if m := s.samMethod(cls); m != nil && m.javaOnly {
				s.report(lambda, javaOnlyCallMessage(m.reason, "expressed as a lambda in Kotlin"))
				return
			}
		}
	}
	for i := range s.ix.methods[call.Name] {
		m := &s.ix.methods[call.Name][i]
		reason, applies := s.javaOnlyReason(m)
		if applies && slackCallTargets(file, call, m.decl, s.index, s.resolver) {
			s.report(idx, javaOnlyCallMessage(reason, "called from Kotlin"))
			return
		}
	}
}

// checkReference reports a callable reference (`Api::method`) to a @JavaOnly function.
func (s *javaOnlyScan) checkReference(ref uint32) {
	file := s.file
	var recv, name string
	for c := file.FlatFirstChild(ref); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "simple_identifier":
			name = file.FlatNodeText(c)
		case "user_type", "type_identifier":
			recv = strings.Join(strings.Fields(file.FlatNodeText(c)), "")
		}
	}
	if recv == "" || name == "" {
		return
	}
	owner := slackClassFQN(s.index, file, recv)
	for i := range s.ix.methods[name] {
		m := &s.ix.methods[name][i]
		if reason, applies := s.javaOnlyReason(m); applies && m.decl.owner == owner {
			s.report(ref, javaOnlyCallMessage(reason, "called from Kotlin"))
			return
		}
	}
}

// javaOnlyReason returns the reason of the @JavaOnly that applies to a method: its own, or its class's.
func (s *javaOnlyScan) javaOnlyReason(m *javaOnlyMethod) (string, bool) {
	if m.javaOnly {
		return m.reason, true
	}
	if c := s.ix.classes[m.decl.owner]; c != nil && c.javaOnly {
		return c.reason, true
	}
	return "", false
}

// samMethod returns the indexed method of a single-method interface, or nil.
func (s *javaOnlyScan) samMethod(cls *javaOnlyClass) *javaOnlyMethod {
	var found *javaOnlyMethod
	for name, ms := range s.ix.methods {
		for i := range ms {
			if ms[i].decl.owner == cls.fqn {
				if found != nil {
					return nil
				}
				found = &s.ix.methods[name][i]
			}
		}
	}
	return found
}

func javaOnlyCallMessage(reason, what string) string {
	if strings.TrimSpace(reason) == "" {
		return "This method should not be " + what + ", see its documentation for details."
	}
	return "This method should not be " + what + ": " + reason
}

// slackTrailingLambdaNode returns the lambda_literal a Kotlin call passes after its parentheses, or 0.
func slackTrailingLambdaNode(file *scanner.File, call uint32) uint32 {
	suffix, ok := file.FlatFindChild(call, "call_suffix")
	if !ok {
		return 0
	}
	ann, ok := file.FlatFindChild(suffix, "annotated_lambda")
	if !ok {
		return 0
	}
	lambda, _ := file.FlatFindChild(ann, "lambda_literal")
	return lambda
}

// javaOnlyDelegationType returns the type name a Kotlin delegation_specifier names, `Outer.Callback` for
// `Outer.Callback` or `Outer.Callback()`.
func javaOnlyDelegationType(file *scanner.File, spec uint32) string {
	var found string
	file.FlatWalkNodes(spec, "user_type", func(n uint32) {
		if found != "" {
			return
		}
		var parts []string
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "type_identifier" {
				parts = append(parts, file.FlatNodeText(c))
			}
		}
		found = strings.Join(parts, ".")
	})
	return found
}
