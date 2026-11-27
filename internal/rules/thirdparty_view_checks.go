package rules

// Ports of two slack-lints 0.11.1 View checks (slack-lint-checks/src/main/java/slack/lint/ui/):
//
//   - DoNotCallViewToString reports a toString() call that resolves to an override declared in a subclass of
//     android.view.View. As in Lint (InheritanceUtil.isInheritor is strict), View's own toString() is not reported.
//   - InflationInItemDecoration (ItemDecorationViewBindingDetector) reports, inside a class that extends
//     RecyclerView.ItemDecoration, an inflate() call declared in LayoutInflater or in a ViewBinding class.
//
// Lint resolves calls against the classpath. krit proves the receiver's class from the source resolver, a declared
// type or the class name, and walks source class hierarchies; library classes other than the target end the walk.

import (
	"bytes"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	slackDoNotCallViewToString     = "DoNotCallViewToString"
	slackInflationInItemDecoration = "InflationInItemDecoration"

	viewItemDecorationFQN = "androidx.recyclerview.widget.RecyclerView.ItemDecoration"
	viewLayoutInflaterFQN = "android.view.LayoutInflater"
	viewViewBindingFQN    = "androidx.viewbinding.ViewBinding"

	viewInflationMessage = "ViewBinding should not be used in `ItemDecoration`. If an inflated view contains meaningful textual information, it will not be visible to TalkBack. This means that screen reader users will not be able to know what is on the screen."
)

// The gates match the method name without call syntax, which may be spaced or commented (`v.toString ()`).
func viewToStringFileGate(file *scanner.File) bool {
	return bytes.Contains(file.Content, []byte("toString"))
}

func viewInflateFileGate(file *scanner.File) bool {
	return bytes.Contains(file.Content, []byte("inflate"))
}

// checkDoNotCallViewToString reports a toString() call whose resolved method is declared in a View subclass.
func checkDoNotCallViewToString(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	name := slackCalleeNameNode(file, idx)
	if name == 0 || file.FlatNodeText(name) != "toString" {
		return
	}
	call, ok := lintCallAt(file, idx)
	if !ok || len(call.Args) != 0 || denyTrailingLambda(file, idx) || denyNullableAnyExtension(ctx, call) {
		return
	}
	var owner string
	if call.ReceiverNode == 0 || call.ReceiverText == "this" {
		owner = viewImplicitReceiverFQN(ctx, idx)
	} else {
		owner = denyReceiverType(ctx, call.ReceiverNode)
	}
	declaring := viewMethodDeclarer(ctx, viewSourceClassFQN(ctx, owner), "toString", map[string]bool{})
	if declaring != "" && declaring != "android.view.View" && slackClassExtends(ctx.Resolver, declaring, "android.view.View") {
		ctx.EmitAtNode(name, "Do not call `View.toString()`")
	}
}

// viewMethodDeclarer returns the source class, fqn or one of its source superclasses, that declares a no-argument
// override called name, or "". The resolver records Kotlin members only, so an override declared in Java is not seen.
func viewMethodDeclarer(ctx *api.Context, fqn, name string, seen map[string]bool) string {
	if ctx.Resolver == nil || fqn == "" || seen[fqn] {
		return ""
	}
	seen[fqn] = true
	info := ctx.Resolver.ClassHierarchy(fqn)
	if info == nil || info.FQN != fqn {
		return ""
	}
	for _, m := range info.Members {
		// The no-argument override only: a `toString(radix: Int)` overload is not what `x.toString()` calls.
		if m.Name == name && m.Kind == "function" && m.IsOverride && len(m.Params) == 0 {
			return fqn
		}
	}
	for _, super := range slackRecordedSupertypes(info) {
		if d := viewMethodDeclarer(ctx, slackSupertypeFQN(ctx.Resolver, info, super), name, seen); d != "" {
			return d
		}
	}
	return ""
}

// viewSourceClassFQN qualifies a class name the source shows (resolved through imports, or written without a
// package) with the package of the source class it names.
func viewSourceClassFQN(ctx *api.Context, name string) string {
	if name == "" || strings.Contains(name, ".") || ctx.Resolver == nil {
		return name
	}
	if pkg := moshiFilePackage(ctx.File); pkg != "" {
		if info := ctx.Resolver.ClassHierarchy(pkg + "." + name); info != nil {
			return info.FQN
		}
	}
	if info := ctx.Resolver.ClassHierarchy(name); info != nil && info.FQN != "" {
		return info.FQN
	}
	return name
}

// viewReceiverClassFQN returns the FQN of the class an extension receiver's type node typ (unwrapped) names,
// expanding a typealias (`typealias KV = KView; fun KV.f()`) through the resolver, and whether that alias is
// nullable (`typealias KVN = KView?`).
func viewReceiverClassFQN(ctx *api.Context, typ uint32) (fqn string, nullable bool) {
	fqn = denyAnyClassFQN(ctx.Resolver, ctx.File, flatTypeNameIdentifier(ctx.File, typ))
	if ctx.Resolver == nil || ctx.Resolver.ClassHierarchy(fqn) != nil || ctx.File.FlatType(typ) != "user_type" {
		return fqn, false
	}
	if t := ctx.Resolver.ResolveFlatNode(typ, ctx.File); t != nil && t.FQN != "" && ctx.Resolver.ClassHierarchy(t.FQN) != nil {
		return t.FQN, t.IsNullable()
	}
	return fqn, false
}

// viewReceiverFQN returns the FQN of an extension function's receiver class: the receiver's, or the first upper
// bound of a type-parameter receiver that is a View, else its first bound. It returns "" when the receiver is
// nullable (toString() is then Any?.toString()) or has no class (a function type).
func viewReceiverFQN(ctx *api.Context, fn uint32) string {
	types, nullable, _ := flatFunctionReceiverClassTypes(ctx.File, fn)
	if nullable {
		return ""
	}
	first := ""
	for _, typ := range types {
		if flatTypeNameIdentifier(ctx.File, typ) == "" {
			continue
		}
		fqn, aliasNullable := viewReceiverClassFQN(ctx, typ)
		if aliasNullable {
			return ""
		}
		if len(types) == 1 {
			return fqn
		}
		if slackClassExtends(ctx.Resolver, viewSourceClassFQN(ctx, fqn), "android.view.View") {
			return fqn
		}
		if first == "" {
			first = fqn
		}
	}
	return first
}

// viewImplicitReceiverFQN returns the FQN of the class whose `this` an unqualified call reaches: the receiver of the
// innermost Kotlin extension function (which wins over the enclosing class), else the named enclosing class; or ""
// when a receiver-binding Kotlin lambda, a companion or an anonymous object comes first.
func viewImplicitReceiverFQN(ctx *api.Context, idx uint32) string {
	file := ctx.File
	var names []string
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration":
			if len(names) == 0 && file.Language == scanner.LangKotlin {
				if _, _, ok := flatFunctionReceiverType(file, p); ok {
					// An extension's receiver is `this`, whatever the enclosing class. On a nullable receiver,
					// toString() is the Any?.toString() extension, not the class's; a function-type receiver has
					// no class (#1949).
					fqn := viewReceiverFQN(ctx, p)
					if fqn == "" {
						return ""
					}
					return viewSourceClassFQN(ctx, fqn)
				}
			}
		case "lambda_literal":
			// A lambda keeps the class's `this` unless it binds a receiver (`apply`, `run`, `with`, `buildString`, …).
			if len(names) == 0 && viewLambdaBindsReceiver(file, p) {
				return ""
			}
		case "object_literal", "companion_object":
			if len(names) == 0 {
				return ""
			}
		case "class_body":
			if parent, ok := file.FlatParent(p); ok && file.FlatType(parent) == "object_creation_expression" && len(names) == 0 {
				return ""
			}
		case "class_declaration", "object_declaration":
			if file.Language == scanner.LangKotlin {
				names = append(names, extractIdentifierFlat(file, p))
			} else if id, ok := file.FlatFindChild(p, "identifier"); ok {
				names = append(names, file.FlatNodeText(id))
			}
		}
	}
	if len(names) == 0 || names[0] == "" {
		return ""
	}
	for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
		names[i], names[j] = names[j], names[i]
	}
	fqn := strings.Join(names, ".")
	if pkg := moshiFilePackage(file); pkg != "" {
		fqn = pkg + "." + fqn
	}
	return fqn
}

// checkInflationInItemDecoration reports a LayoutInflater or ViewBinding inflate() call in an ItemDecoration.
func checkInflationInItemDecoration(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	name := slackCalleeNameNode(file, idx)
	if name == 0 || file.FlatNodeText(name) != "inflate" {
		return
	}
	call, ok := lintCallAt(file, idx)
	if !ok || call.ReceiverNode == 0 || !viewInflateIsLayoutOrBinding(ctx, call) {
		return
	}
	if viewEnclosingClassExtends(ctx, idx, viewItemDecorationFQN) {
		ctx.EmitAtNode(name, viewInflationMessage)
	}
}

// viewInflateIsLayoutOrBinding reports whether an inflate() call is LayoutInflater's (its receiver is a
// LayoutInflater) or a ViewBinding's (a static call on a generated `…Binding` class in a `databinding` package, or a
// static or instance call on a source class implementing ViewBinding).
func viewInflateIsLayoutOrBinding(ctx *api.Context, call lintCall) bool {
	file := ctx.File
	if last := call.ReceiverText[strings.LastIndexByte(call.ReceiverText, '.')+1:]; last != "" && last[0] >= 'A' && last[0] <= 'Z' &&
		file.FlatType(call.ReceiverNode) != "call_expression" && file.FlatType(call.ReceiverNode) != "method_invocation" {
		fqn := viewSourceClassFQN(ctx, denyAnyClassFQN(ctx.Resolver, file, call.ReceiverText))
		// A source class must implement ViewBinding; the generated-binding naming is the fallback for a class krit
		// has no source for.
		if ctx.Resolver != nil && ctx.Resolver.ClassHierarchy(fqn) != nil {
			return slackClassExtends(ctx.Resolver, fqn, viewViewBindingFQN)
		}
		if strings.Contains(fqn, ".databinding.") && strings.HasSuffix(fqn, "Binding") {
			return true
		}
		if fqn != "" && strings.Contains(fqn, ".") {
			// A class name: a static call, not a LayoutInflater instance.
			return false
		}
	}
	typ := viewSourceClassFQN(ctx, viewInflaterReceiverType(ctx, call.ReceiverNode))
	if typ == viewLayoutInflaterFQN || slackClassExtends(ctx.Resolver, typ, viewLayoutInflaterFQN) {
		return true
	}
	// An instance inflate() on a source class implementing ViewBinding: Lint checks the method's declaring class, not
	// whether the call is static.
	// An extension inflate() is declared in its file facade, not the class, so the member must be the class's own or
	// a source supertype's.
	return ctx.Resolver != nil && typ != "" && ctx.Resolver.ClassHierarchy(typ) != nil &&
		slackClassExtends(ctx.Resolver, typ, viewViewBindingFQN) && viewClassDeclaresFunction(ctx, typ, "inflate", map[string]bool{})
}

// viewClassDeclaresFunction reports whether the source class fqn, or one of its source superclasses, declares a
// member function called name.
func viewClassDeclaresFunction(ctx *api.Context, fqn, name string, seen map[string]bool) bool {
	if fqn == "" || seen[fqn] {
		return false
	}
	seen[fqn] = true
	info := ctx.Resolver.ClassHierarchy(fqn)
	if info == nil || info.FQN != fqn {
		return false
	}
	for _, m := range info.Members {
		if m.Name == name && m.Kind == "function" {
			return true
		}
	}
	for _, super := range slackRecordedSupertypes(info) {
		if viewClassDeclaresFunction(ctx, slackSupertypeFQN(ctx.Resolver, info, super), name, seen) {
			return true
		}
	}
	return false
}

// viewInflaterReceiverType is denyReceiverType that also types `LayoutInflater.from(context)`.
func viewInflaterReceiverType(ctx *api.Context, recv uint32) string {
	file := ctx.File
	if t := file.FlatType(recv); t == "call_expression" || t == "method_invocation" {
		if c, ok := lintCallAt(file, recv); ok && c.Name == "from" && denyAnyClassFQN(ctx.Resolver, file, c.ReceiverText) == viewLayoutInflaterFQN {
			return viewLayoutInflaterFQN
		}
	}
	if t := denyReceiverType(ctx, recv); t != "" {
		return t
	}
	// An untyped property or local initialized by `LayoutInflater.from(context)`.
	if init := viewInitializerOf(file, recv); init != 0 && init != recv {
		if t := file.FlatType(init); t == "call_expression" || t == "method_invocation" {
			return viewInflaterReceiverType(ctx, init)
		}
	}
	return ""
}

// viewInitializerOf returns the initializer of the Kotlin property or local, or Java local or field, a name refers
// to (the nearest declaration in an enclosing block, class body or file), or 0.
func viewInitializerOf(file *scanner.File, use uint32) uint32 {
	switch file.FlatType(use) {
	case "simple_identifier", "identifier":
	default:
		return 0
	}
	name := file.FlatNodeText(use)
	child := use
	for parent, ok := file.FlatParent(use); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "statements", "block", "class_body", "source_file":
			for sib := file.FlatFirstChild(parent); sib != 0; sib = file.FlatNextSib(sib) {
				if (file.FlatType(parent) == "statements" || file.FlatType(parent) == "block") && sib == child {
					break
				}
				switch file.FlatType(sib) {
				case "property_declaration":
					if v, ok := file.FlatFindChild(sib, "variable_declaration"); ok && extractIdentifierFlat(file, v) == name {
						return lintInitializerAfterEquals(file, sib)
					}
				case "local_variable_declaration", "field_declaration":
					for d := file.FlatFirstChild(sib); d != 0; d = file.FlatNextSib(d) {
						if file.FlatType(d) == "variable_declarator" && extractIdentifierFlat(file, d) == name {
							return lintInitializerAfterEquals(file, d)
						}
					}
				}
			}
		}
		child = parent
	}
	return 0
}

// viewEnclosingClassExtends reports whether the innermost class, object or anonymous class around idx extends
// target, directly (through its file's imports) or through source superclasses.
func viewEnclosingClassExtends(ctx *api.Context, idx uint32, target string) bool {
	file := ctx.File
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "class_declaration", "object_declaration", "object_literal":
			return viewClassNodeExtends(ctx, p, target)
		case "companion_object":
			// A companion is its own class, and it extends nothing here.
			return false
		case "class_body":
			if parent, ok := file.FlatParent(p); ok && file.FlatType(parent) == "object_creation_expression" {
				return viewClassNodeExtends(ctx, parent, target)
			}
		}
	}
	return false
}

// viewClassNodeExtends reports whether a Kotlin class, object or object expression, or a Java class or anonymous
// class, extends target.
func viewClassNodeExtends(ctx *api.Context, cls uint32, target string) bool {
	file := ctx.File
	var supers []string
	for c := file.FlatFirstChild(cls); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "delegation_specifier":
			if t := viewDelegationType(file, c); t != "" {
				supers = append(supers, t)
			}
		case "superclass", "super_interfaces":
			supers = append(supers, viewJavaTypeNames(file, c)...)
		case "type_identifier", "scoped_type_identifier", "generic_type":
			// The type an anonymous Java class extends.
			if file.FlatType(cls) == "object_creation_expression" {
				supers = append(supers, viewJavaTypeName(file, c))
			}
		}
	}
	for _, s := range supers {
		if slackClassExtends(ctx.Resolver, viewSourceClassFQN(ctx, denyAnyClassFQN(ctx.Resolver, file, s)), target) {
			return true
		}
	}
	return false
}

// viewDelegationType returns the type name a Kotlin delegation_specifier names, `RecyclerView.ItemDecoration` for
// `RecyclerView.ItemDecoration()`.
func viewDelegationType(file *scanner.File, spec uint32) string {
	var found string
	file.FlatWalkNodes(spec, "user_type", func(n uint32) {
		if found == "" {
			found = denyKotlinTypeText(file, n)
		}
	})
	return found
}

func viewJavaTypeNames(file *scanner.File, n uint32) []string {
	var out []string
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "type_identifier", "scoped_type_identifier", "generic_type":
			out = append(out, viewJavaTypeName(file, c))
		case "type_list":
			out = append(out, viewJavaTypeNames(file, c)...)
		}
	}
	return out
}

func viewJavaTypeName(file *scanner.File, n uint32) string {
	text := strings.Join(strings.Fields(file.FlatNodeText(n)), "")
	if i := strings.IndexByte(text, '<'); i >= 0 {
		text = text[:i]
	}
	return text
}

// viewReceiverLambdaCalls are the standard-library functions whose lambda has a receiver.
var viewReceiverLambdaCalls = map[string]bool{"apply": true, "run": true, "with": true, "buildString": true, "buildList": true,
	"buildSet": true, "buildMap": true}

// viewLambdaBindsReceiver reports whether a Kotlin lambda is passed to a function known to give it a receiver, or to
// a function krit cannot tell about (anything but a plain call by name), which may.
func viewLambdaBindsReceiver(file *scanner.File, lambda uint32) bool {
	call := lambda
	for call != 0 && file.FlatType(call) != "call_expression" {
		call, _ = file.FlatParent(call)
	}
	if call == 0 {
		return true
	}
	if inner := file.FlatFirstChild(call); file.FlatType(inner) == "call_expression" {
		call = inner
	}
	c, ok := lintCallAt(file, call)
	if !ok {
		return true
	}
	return viewReceiverLambdaCalls[c.Name]
}
