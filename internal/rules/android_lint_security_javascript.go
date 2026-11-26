package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// JavascriptInterfaceRule mirrors Lint's JavascriptInterface (JavaScriptInterfaceDetector): an object passed to
// WebView.addJavascriptInterface whose class has no @JavascriptInterface method, and a non-public declaration
// annotated with @JavascriptInterface.
type JavascriptInterfaceRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the WebView receiver and the bridge class are resolved from
// declarations in the file.
func (r *JavascriptInterfaceRule) Confidence() float64 { return api.ConfidenceMedium }

const javascriptInterfaceFQN = "android.webkit.JavascriptInterface"

// javascriptInterfaceNodeTypes are the calls and declarations the rule visits.
var javascriptInterfaceNodeTypes = []string{
	"call_expression", "method_invocation",
	"function_declaration", "property_declaration", "class_declaration", "object_declaration",
	"method_declaration", "field_declaration", "constructor_declaration",
}

func (r *JavascriptInterfaceRule) check(ctx *api.Context) {
	switch ctx.File.FlatType(ctx.Idx) {
	case "call_expression", "method_invocation":
		r.checkCall(ctx)
	default:
		r.checkDeclaration(ctx)
	}
}

func (r *JavascriptInterfaceRule) checkCall(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "addJavascriptInterface" || len(call.Args) != 2 {
		return
	}
	confidence, ok := addJavascriptInterfaceCallConfidence(ctx, ctx.Idx)
	if !ok || javascriptInterfaceReceiverOverrides(file, call) {
		return
	}
	// Lint reports through Constraints.targetSdkAtLeast(17).
	if addJavascriptInterfaceSDKContextForFile(file).targetSdk < 17 {
		return
	}
	// Lint evaluates the first argument in source order, even when Kotlin names it `name`.
	bridge, known := javascriptInterfaceBridge(file, call.Args[0], ctx.Idx, 0)
	if !known || bridge.annotated {
		return
	}
	at := lintCallNameNode(file, ctx.Idx)
	if at == 0 {
		at = ctx.Idx
	}
	f := lintFindingAtNode(ctx, at, "",
		"None of the methods in the added interface ("+bridge.name+") have been annotated with "+
			"`@android.webkit.JavascriptInterface`; they will not be visible in API 17")
	f.Confidence = confidence
	ctx.Emit(f)
}

// javascriptInterfaceReceiverOverrides reports whether the receiver's declared type is a class in the file that
// declares its own addJavascriptInterface: Lint only checks the WebView method itself.
func javascriptInterfaceReceiverOverrides(file *scanner.File, call lintCall) bool {
	typ := lintReceiverTypeText(file, call)
	if typ == "" {
		return false
	}
	cls := lintSameFileClass(file, typ[strings.LastIndex(typ, ".")+1:])
	if cls == 0 {
		return false
	}
	found := false
	javascriptInterfaceEachMember(file, cls, func(member uint32) {
		if !found && javascriptInterfaceMemberName(file, member) == "addJavascriptInterface" {
			found = true
		}
	})
	return found
}

// javascriptInterfaceClass is what the rule knows about the class of a bridge object.
type javascriptInterfaceClass struct {
	// name is the simple class name Lint prints, or "null" for an anonymous class.
	name string
	// annotated reports whether the class, a superclass, or a method of either has @JavascriptInterface.
	annotated bool
}

// javascriptInterfaceBridge resolves the class of the bridge expression the way Lint's TypeEvaluator does: a
// constructor call, a string, `this`, an anonymous object, or a local whose initializer is one of those. It reports
// false when the class cannot be resolved from the file or from well-known platform packages.
func javascriptInterfaceBridge(file *scanner.File, expr, scope uint32, depth int) (javascriptInterfaceClass, bool) {
	expr = lintSkipParensJS(file, expr)
	if expr == 0 || depth > 6 {
		return javascriptInterfaceClass{}, false
	}
	switch file.FlatType(expr) {
	case "string_literal":
		return javascriptInterfaceClass{name: "String"}, true
	case "call_expression":
		call, ok := lintCallAt(file, expr)
		if !ok || call.Name == "" || call.Name[0] < 'A' || call.Name[0] > 'Z' {
			return javascriptInterfaceClass{}, false
		}
		if call.ReceiverNode != 0 && !strings.Contains(call.TypeText, ".") {
			return javascriptInterfaceClass{}, false
		}
		return javascriptInterfaceType(file, call.TypeText, depth+1)
	case "object_creation_expression":
		call, ok := lintCallAt(file, expr)
		if !ok {
			return javascriptInterfaceClass{}, false
		}
		if body, hasBody := file.FlatFindChild(expr, "class_body"); hasBody {
			return javascriptInterfaceAnonymous(file, body, []string{call.TypeText}, depth)
		}
		return javascriptInterfaceType(file, call.TypeText, depth+1)
	case "object_literal":
		body, _ := file.FlatFindChild(expr, "class_body")
		return javascriptInterfaceAnonymous(file, body, javascriptInterfaceSuperclassNames(file, expr), depth)
	case "this_expression", "this":
		if strings.TrimSpace(file.FlatNodeText(expr)) != "this" {
			return javascriptInterfaceClass{}, false
		}
		for cur, ok := file.FlatParent(expr); ok; cur, ok = file.FlatParent(cur) {
			switch file.FlatType(cur) {
			case "lambda_literal", "annotated_lambda", "object_literal", "lambda_expression", "class_body":
				if file.FlatType(cur) == "class_body" {
					if parent, ok := file.FlatParent(cur); ok && file.FlatType(parent) == "object_creation_expression" {
						return javascriptInterfaceClass{}, false
					}
					continue
				}
				return javascriptInterfaceClass{}, false
			case "class_declaration", "object_declaration":
				return javascriptInterfaceDeclared(file, cur, depth+1)
			}
		}
	case "simple_identifier", "identifier":
		name := file.FlatNodeText(expr)
		if _, inFn := flatEnclosingCallable(file, scope); inFn {
			if init := lintInitializerOf(file, scope, name, true); init != 0 {
				if _, initInFn := flatEnclosingCallable(file, init); initInFn {
					return javascriptInterfaceBridge(file, init, init, depth+1)
				}
			}
		}
		if typ := lintDeclaredTypeText(file, scope, name); typ != "" {
			return javascriptInterfaceType(file, typ, depth+1)
		}
	}
	return javascriptInterfaceClass{}, false
}

// lintSkipParensJS unwraps parenthesized expressions.
func lintSkipParensJS(file *scanner.File, idx uint32) uint32 {
	for idx != 0 && file.FlatType(idx) == "parenthesized_expression" {
		inner := uint32(0)
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				inner = child
				break
			}
		}
		idx = inner
	}
	return idx
}

// javascriptInterfaceAnonymous describes an anonymous class: its own methods, then its superclass.
func javascriptInterfaceAnonymous(file *scanner.File, body uint32, supers []string, depth int) (javascriptInterfaceClass, bool) {
	out := javascriptInterfaceClass{name: "null"}
	if body != 0 && javascriptInterfaceHasAnnotatedMember(file, body) {
		out.annotated = true
		return out, true
	}
	for _, super := range supers {
		info, ok := javascriptInterfaceType(file, super, depth+1)
		if !ok {
			return out, false
		}
		out.annotated = out.annotated || info.annotated
	}
	return out, true
}

// javascriptInterfacePlatformPackages are packages whose classes never carry @JavascriptInterface, so a bridge of one
// of their types has no annotated method.
var javascriptInterfacePlatformPackages = []string{"java.", "javax.", "kotlin.", "android.", "androidx."}

// javascriptInterfaceImplicitTypes are the types a file can name without an import, mapped to Lint's class name.
var javascriptInterfaceImplicitTypes = map[string]string{
	"Any": "Object", "Object": "Object", "String": "String", "Runnable": "Runnable", "CharSequence": "CharSequence",
	"Number": "Number", "Thread": "Thread",
}

// javascriptInterfaceType resolves a written type name to the class Lint would evaluate.
func javascriptInterfaceType(file *scanner.File, typ string, depth int) (javascriptInterfaceClass, bool) {
	typ = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(typ), "?"))
	if i := strings.IndexAny(typ, "<("); i >= 0 {
		typ = typ[:i]
	}
	if typ == "" || depth > 6 {
		return javascriptInterfaceClass{}, false
	}
	simple := typ[strings.LastIndex(typ, ".")+1:]
	if !strings.Contains(typ, ".") {
		if decl := lintSameFileClassOrObjectJS(file, simple); decl != 0 {
			return javascriptInterfaceDeclared(file, decl, depth+1)
		}
		if full, ok := lintImportsOf(file).bySimple[simple]; ok {
			typ = full
		} else if name, ok := javascriptInterfaceImplicitTypes[simple]; ok {
			return javascriptInterfaceClass{name: name}, true
		} else {
			return javascriptInterfaceClass{}, false
		}
	}
	switch typ {
	case "kotlin.Any", "java.lang.Object":
		return javascriptInterfaceClass{name: "Object"}, true
	}
	for _, pkg := range javascriptInterfacePlatformPackages {
		if strings.HasPrefix(typ, pkg) {
			return javascriptInterfaceClass{name: typ[strings.LastIndex(typ, ".")+1:]}, true
		}
	}
	return javascriptInterfaceClass{}, false
}

// lintSameFileClassOrObjectJS returns the class, interface or object declared in the file with the simple name.
func lintSameFileClassOrObjectJS(file *scanner.File, name string) uint32 {
	if cls := lintSameFileClass(file, name); cls != 0 {
		return cls
	}
	var found uint32
	for _, kind := range []string{"object_declaration", "interface_declaration"} {
		file.FlatWalkNodes(0, kind, func(decl uint32) {
			if found != 0 {
				return
			}
			for _, idType := range []string{"type_identifier", "identifier"} {
				if id, ok := file.FlatFindChild(decl, idType); ok && file.FlatNodeText(id) == name {
					found = decl
				}
			}
		})
	}
	return found
}

// javascriptInterfaceDeclared describes a class declared in the file: Lint walks it and its superclasses, looking for
// the annotation on the class or on any of its methods.
func javascriptInterfaceDeclared(file *scanner.File, decl uint32, depth int) (javascriptInterfaceClass, bool) {
	name := ""
	for _, idType := range []string{"type_identifier", "identifier"} {
		if id, ok := file.FlatFindChild(decl, idType); ok {
			name = file.FlatNodeText(id)
			break
		}
	}
	if name == "" {
		return javascriptInterfaceClass{}, false
	}
	out := javascriptInterfaceClass{name: name}
	if javascriptInterfaceAnnotated(file, decl) {
		out.annotated = true
		return out, true
	}
	if body := javascriptInterfaceBody(file, decl); body != 0 && javascriptInterfaceHasAnnotatedMember(file, body) {
		out.annotated = true
		return out, true
	}
	for _, super := range javascriptInterfaceSuperclassNames(file, decl) {
		info, ok := javascriptInterfaceType(file, super, depth+1)
		if !ok {
			return out, false
		}
		out.annotated = info.annotated
	}
	return out, true
}

// javascriptInterfaceSuperclassNames returns the written superclass of a class or anonymous object: the Kotlin
// supertype called as a constructor, or the Java `extends` type. Interfaces are not superclasses.
func javascriptInterfaceSuperclassNames(file *scanner.File, decl uint32) []string {
	var out []string
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "delegation_specifier":
			if ctor, ok := file.FlatFindChild(child, "constructor_invocation"); ok {
				if ut, ok := file.FlatFindChild(ctor, "user_type"); ok {
					out = append(out, strings.TrimSpace(file.FlatNodeText(ut)))
				}
			}
		case "delegation_specifiers":
			out = append(out, javascriptInterfaceSuperclassNames(file, child)...)
		case "superclass":
			for nested := file.FlatFirstChild(child); nested != 0; nested = file.FlatNextSib(nested) {
				switch file.FlatType(nested) {
				case "type_identifier", "scoped_type_identifier", "generic_type":
					out = append(out, strings.TrimSpace(file.FlatNodeText(nested)))
				}
			}
		}
	}
	return out
}

// javascriptInterfaceBody returns the body node of a class-like declaration.
func javascriptInterfaceBody(file *scanner.File, decl uint32) uint32 {
	for _, kind := range []string{"class_body", "enum_class_body", "interface_body"} {
		if body, ok := file.FlatFindChild(decl, kind); ok {
			return body
		}
	}
	return 0
}

// javascriptInterfaceEachMember calls fn for each function or method declared directly in the class-like decl.
func javascriptInterfaceEachMember(file *scanner.File, decl uint32, fn func(member uint32)) {
	body := javascriptInterfaceBody(file, decl)
	if body == 0 {
		return
	}
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "function_declaration", "method_declaration":
			fn(child)
		}
	}
}

// javascriptInterfaceHasAnnotatedMember reports whether a class body declares a method annotated with
// @JavascriptInterface.
func javascriptInterfaceHasAnnotatedMember(file *scanner.File, body uint32) bool {
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "function_declaration", "method_declaration":
			if javascriptInterfaceAnnotated(file, child) {
				return true
			}
		case "property_declaration":
			// A Kotlin property's getter and setter are methods (#1620).
			if _, accessor := javascriptInterfaceAnnotationTargets(file, child); accessor {
				return true
			}
		case "getter", "setter":
			if javascriptInterfaceAnnotated(file, child) {
				return true
			}
		}
	}
	return false
}

func javascriptInterfaceMemberName(file *scanner.File, member uint32) string {
	for _, idType := range []string{"simple_identifier", "identifier"} {
		if id, ok := file.FlatFindChild(member, idType); ok {
			return file.FlatNodeText(id)
		}
	}
	return ""
}

// javascriptInterfaceAnnotated reports whether the declaration carries an annotation that resolves to
// android.webkit.JavascriptInterface: written in full, or imported (explicitly or through android.webkit.*) and not
// shadowed by a declaration of the same name in the file.
func javascriptInterfaceAnnotated(file *scanner.File, decl uint32) bool {
	direct, _ := javascriptInterfaceAnnotationTargets(file, decl)
	return direct
}

// javascriptInterfaceAnnotationTargets reports @JavascriptInterface on a declaration: direct without a use-site
// target, and accessor for a Kotlin property annotated @get:/@set: or whose getter or setter carries it, which UAST
// sees as an annotated method. @field: and other targets count as neither.
func javascriptInterfaceAnnotationTargets(file *scanner.File, decl uint32) (direct, accessor bool) {
	if mods, ok := file.FlatFindChild(decl, "modifiers"); ok {
		for child := file.FlatFirstChild(mods); child != 0; child = file.FlatNextSib(child) {
			var written, target string
			switch file.FlatType(child) {
			case "annotation":
				if file.Language == scanner.LangJava {
					written = javaAnnotationName(file, child)
				} else {
					written = annotationQualifiedName(file, child)
					if t, ok := file.FlatFindChild(child, "use_site_target"); ok {
						target = strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(t)), ":")
					}
				}
			case "marker_annotation":
				written = javaAnnotationName(file, child)
			default:
				continue
			}
			if !javascriptInterfaceNameResolves(file, written) {
				continue
			}
			switch target {
			case "":
				direct = true
			case "get", "set":
				accessor = true
			}
		}
	}
	if file.FlatType(decl) == "property_declaration" && !accessor {
		for _, acc := range javascriptInterfacePropertyAccessors(file, decl) {
			if javascriptInterfaceAnnotated(file, acc) {
				accessor = true
			}
		}
	}
	return direct, accessor
}

// javascriptInterfacePropertyAccessors returns a property's getter and setter: children of the declaration, or, on
// their own lines, the siblings that follow it.
func javascriptInterfacePropertyAccessors(file *scanner.File, prop uint32) []uint32 {
	var out []uint32
	for child := file.FlatFirstChild(prop); child != 0; child = file.FlatNextSib(child) {
		if t := file.FlatType(child); t == "getter" || t == "setter" {
			out = append(out, child)
		}
	}
	for next := file.FlatNextSib(prop); next != 0; next = file.FlatNextSib(next) {
		t := file.FlatType(next)
		if t != "getter" && t != "setter" {
			if file.FlatIsNamed(next) {
				break
			}
			continue
		}
		out = append(out, next)
	}
	return out
}

func javaAnnotationName(file *scanner.File, ann uint32) string {
	for child := file.FlatFirstChild(ann); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "identifier", "scoped_identifier":
			return strings.TrimSpace(file.FlatNodeText(child))
		}
	}
	return ""
}

func javascriptInterfaceNameResolves(file *scanner.File, written string) bool {
	written = strings.TrimSpace(written)
	if written == javascriptInterfaceFQN {
		return true
	}
	if written != "JavascriptInterface" || lintSameFileClassOrObjectJS(file, written) != 0 {
		return false
	}
	return lintTypeRefMatches(file, written, javascriptInterfaceFQN)
}

func (r *JavascriptInterfaceRule) checkDeclaration(ctx *api.Context) {
	file, decl := ctx.File, ctx.Idx
	direct, accessor := javascriptInterfaceAnnotationTargets(file, decl)
	annotated := direct || accessor && file.FlatType(decl) == "property_declaration"
	if !annotated || javascriptInterfaceDeclIsPublic(file, decl) {
		return
	}
	name := javascriptInterfaceDeclarationName(file, decl)
	if name == 0 {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, name, "", "Must be public when using `@JavascriptInterface`"))
}

// javascriptInterfaceDeclIsPublic reports whether UAST would give the declaration public visibility. Kotlin
// declarations are public unless private or protected (internal maps to public). Java members need the public
// modifier, except members of an interface, which are implicitly public. Local declarations are left alone.
func javascriptInterfaceDeclIsPublic(file *scanner.File, decl uint32) bool {
	if file.FlatType(decl) == "property_declaration" || file.FlatType(decl) == "class_declaration" {
		if _, local := flatEnclosingCallable(file, decl); local {
			return true
		}
	}
	mods, _ := file.FlatFindChild(decl, "modifiers")
	text := ""
	if mods != 0 {
		text = " " + strings.Join(strings.Fields(file.FlatNodeText(mods)), " ") + " "
	}
	if file.Language != scanner.LangJava {
		return !strings.Contains(text, " private ") && !strings.Contains(text, " protected ")
	}
	if strings.Contains(text, " public ") {
		return true
	}
	if strings.Contains(text, " private ") || strings.Contains(text, " protected ") {
		return false
	}
	parent, _ := file.FlatParent(decl)
	if file.FlatType(parent) == "interface_body" || file.FlatType(parent) == "annotation_type_body" {
		return true
	}
	return false
}

// javascriptInterfaceDeclarationName returns the identifier Lint's getNameLocation points at.
func javascriptInterfaceDeclarationName(file *scanner.File, decl uint32) uint32 {
	switch file.FlatType(decl) {
	case "property_declaration":
		if vd, ok := file.FlatFindChild(decl, "variable_declaration"); ok {
			if id, ok := file.FlatFindChild(vd, "simple_identifier"); ok {
				return id
			}
		}
	case "field_declaration":
		if vd, ok := file.FlatFindChild(decl, "variable_declarator"); ok {
			if id, ok := file.FlatFindChild(vd, "identifier"); ok {
				return id
			}
		}
	}
	for _, idType := range []string{"simple_identifier", "type_identifier", "identifier"} {
		if id, ok := file.FlatFindChild(decl, idType); ok {
			return id
		}
	}
	return 0
}
