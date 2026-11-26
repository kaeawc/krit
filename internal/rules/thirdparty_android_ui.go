package rules

// Ports of three slack-lints Android checks (slackhq/slack-lints 0.11.1, slack-lint-checks/src/main/java/slack/lint/):
// ViewContextDetector (CastingViewContextToActivity), ArgInFormattedQuantityStringResDetector
// (ArgInFormattedQuantityStringRes) and text/SpanMarkPointMissingMaskDetector (SpanMarkPointMissingMask).

import (
	"bytes"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

const (
	slackCastingViewContextToActivity    = "CastingViewContextToActivity"
	slackArgInFormattedQuantityStringRes = "ArgInFormattedQuantityStringRes"
	slackSpanMarkPointMissingMask        = "SpanMarkPointMissingMask"

	fqcnAndroidActivity  = "android.app.Activity"
	fqcnAndroidResources = "android.content.res.Resources"
	fqcnAndroidSpanned   = "android.text.Spanned"
)

// --- CastingViewContextToActivity ---

func viewContextFileGate(file *scanner.File) bool {
	return bytes.Contains(file.Content, []byte("getContext")) && bytes.Contains(file.Content, []byte("Activity"))
}

// checkCastingViewContextToActivity reports `view.getContext() as Activity` (Kotlin `as`/`as?`, a Java cast) where
// getContext is View's. As in Lint, the Kotlin property form `view.context` and a parenthesized call are not casts
// of a getContext() call.
func checkCastingViewContextToActivity(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	var operand, typ uint32
	switch file.FlatType(idx) {
	case "as_expression":
		operand = file.FlatFirstChild(idx)
		for child := file.FlatNextSib(operand); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				typ = child
			}
		}
	case "cast_expression":
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			if typ == 0 {
				typ = child
			} else {
				operand = child
			}
		}
	}
	if operand == 0 || typ == 0 || !lintTypeRefMatches(file, strings.Join(strings.Fields(file.FlatNodeText(typ)), ""), fqcnAndroidActivity) {
		return
	}
	call, ok := lintCallAt(file, operand)
	if !ok || call.Name != "getContext" || len(call.Args) != 0 || !slackReceiverIsView(ctx, call) {
		return
	}
	ctx.EmitAtNode(idx, "Unsafe cast of Context to Activity")
}

// slackReceiverIsView reports whether a call's receiver is proven to be an android.view.View: a receiver whose
// resolved type is a View subtype, or an implicit receiver in a class that extends View.
func slackReceiverIsView(ctx *api.Context, call lintCall) bool {
	file := ctx.File
	if call.ReceiverNode == 0 || call.ReceiverText == "this" {
		if file.Language == scanner.LangJava {
			return slackJavaEnclosingClassExtendsView(ctx, call.Idx)
		}
		return androidEnclosingClassExtendsView(file, call.Idx, ctx.Resolver)
	}
	if androidReceiverIsKnownNonView(file, call.ReceiverNode) {
		return false
	}
	if typ := slackReceiverType(ctx, call.ReceiverNode); typ != nil {
		return androidTypeIsViewSubtype(ctx.Resolver, typ)
	}
	return false
}

// slackJavaEnclosingClassExtendsView reports whether the Java class whose method idx is in extends View, directly or
// through source superclasses. An anonymous class around idx comes first; its supertype is not read, so it abstains.
func slackJavaEnclosingClassExtendsView(ctx *api.Context, idx uint32) bool {
	file := ctx.File
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "class_body":
			if parent, ok := file.FlatParent(p); ok && file.FlatType(parent) == "object_creation_expression" {
				return false
			}
		case "class_declaration":
			sc, ok := file.FlatFindChild(p, "superclass")
			if !ok {
				return false
			}
			for c := file.FlatFirstChild(sc); c != 0; c = file.FlatNextSib(c) {
				if !file.FlatIsNamed(c) {
					continue
				}
				name := slackJavaTypeName(file, c)
				ref := androidSupertypeRef{name: name, simple: androidSimpleName(name), qualified: strings.Contains(name, ".")}
				return androidSupertypeIsView(ref, file, ctx.Resolver)
			}
			return false
		case "interface_declaration", "enum_declaration", "record_declaration":
			return false
		}
	}
	return false
}

// slackReceiverType returns the type of a receiver expression: the source resolver's answer, or for a Java name
// the declared type of the parameter, local or field it refers to (the resolver does not type Java variables).
func slackReceiverType(ctx *api.Context, recv uint32) *typeinfer.ResolvedType {
	file := ctx.File
	if ctx.Resolver != nil {
		if typ := ctx.Resolver.ResolveFlatNode(recv, file); typ != nil && typ.Name != "" {
			return typ
		}
	}
	if file.FlatType(recv) != "identifier" {
		return nil
	}
	text := slackJavaDeclaredTypeText(file, recv)
	if text == "" {
		return nil
	}
	simple := text[strings.LastIndexByte(text, '.')+1:]
	fqn := text
	if imported, ok := lintImportsOf(file).bySimple[text]; ok {
		fqn = imported
	}
	return &typeinfer.ResolvedType{Name: simple, FQN: fqn}
}

// slackJavaDeclaredTypeText returns the declared type (without type arguments) of the Java parameter, local or field
// a name refers to, or "".
func slackJavaDeclaredTypeText(file *scanner.File, use uint32) string {
	name := file.FlatNodeText(use)
	child := use
	for parent, ok := file.FlatParent(use); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "block", "class_body":
			for sib, found := file.FlatPrevSibling(child); found; sib, found = file.FlatPrevSibling(sib) {
				if t := slackJavaDeclarationType(file, sib, name); t != "" {
					return t
				}
			}
			if file.FlatType(parent) == "class_body" {
				for sib, found := file.FlatNextSibling(child); found; sib, found = file.FlatNextSibling(sib) {
					if t := slackJavaDeclarationType(file, sib, name); t != "" {
						return t
					}
				}
			}
		case "method_declaration", "constructor_declaration", "lambda_expression":
			if t, bound := slackJavaParameterType(file, parent, name); bound {
				return t
			}
		case "enhanced_for_statement":
			// `for (Fake value : values)` binds value in the body, not in the iterable.
			if id, ok := file.FlatFindChild(parent, "identifier"); ok && file.FlatNodeText(id) == name && child != slackJavaForIterable(file, parent) {
				return slackJavaTypeChildText(file, parent)
			}
		case "catch_clause":
			if param, ok := file.FlatFindChild(parent, "catch_formal_parameter"); ok {
				if id, ok := file.FlatFindChild(param, "identifier"); ok && file.FlatNodeText(id) == name {
					// A catch parameter is a Throwable, never a View or Resources.
					return "Throwable"
				}
			}
		case "try_with_resources_statement":
			if spec, ok := file.FlatFindChild(parent, "resource_specification"); ok && child != spec {
				for res := file.FlatFirstChild(spec); res != 0; res = file.FlatNextSib(res) {
					if id, ok := file.FlatFindChild(res, "identifier"); ok && file.FlatType(res) == "resource" && file.FlatNodeText(id) == name {
						return slackJavaTypeChildText(file, res)
					}
				}
			}
		}
		child = parent
	}
	return ""
}

// slackJavaParameterType returns the declared type of a parameter called name of a Java method, constructor or
// lambda, and whether the scope declares one. An inferred lambda parameter's type is unknown ("").
func slackJavaParameterType(file *scanner.File, scope uint32, name string) (string, bool) {
	for c := file.FlatFirstChild(scope); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "identifier":
			if file.FlatType(scope) == "lambda_expression" && file.FlatNodeText(c) == name {
				return "", true
			}
		case "inferred_parameters", "formal_parameters":
			for p := file.FlatFirstChild(c); p != 0; p = file.FlatNextSib(p) {
				switch file.FlatType(p) {
				case "identifier":
					if file.FlatNodeText(p) == name {
						return "", true
					}
				case "formal_parameter", "spread_parameter":
					if id, ok := file.FlatFindChild(p, "identifier"); ok && file.FlatNodeText(id) == name {
						return slackJavaTypeChildText(file, p), true
					}
				}
			}
		}
	}
	return "", false
}

// slackJavaForIterable returns the iterable expression of an enhanced for statement (the named child after the
// loop variable), or 0.
func slackJavaForIterable(file *scanner.File, loop uint32) uint32 {
	seenVar := false
	for c := file.FlatFirstChild(loop); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "identifier" && !seenVar {
			seenVar = true
			continue
		}
		if seenVar && file.FlatIsNamed(c) {
			return c
		}
	}
	return 0
}

// slackJavaTypeName returns a Java type node's name without type arguments.
func slackJavaTypeName(file *scanner.File, typ uint32) string {
	text := strings.Join(strings.Fields(file.FlatNodeText(typ)), "")
	if i := strings.IndexByte(text, '<'); i >= 0 {
		text = text[:i]
	}
	return text
}

func slackJavaDeclarationType(file *scanner.File, decl uint32, name string) string {
	switch file.FlatType(decl) {
	case "local_variable_declaration", "field_declaration":
	default:
		return ""
	}
	for d := file.FlatFirstChild(decl); d != 0; d = file.FlatNextSib(d) {
		if file.FlatType(d) == "variable_declarator" && extractIdentifierFlat(file, d) == name {
			return slackJavaTypeChildText(file, decl)
		}
	}
	return ""
}

func slackJavaTypeChildText(file *scanner.File, decl uint32) string {
	for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "type_identifier", "scoped_type_identifier", "generic_type":
			text := strings.Join(strings.Fields(file.FlatNodeText(c)), "")
			if i := strings.IndexByte(text, '<'); i >= 0 {
				text = text[:i]
			}
			if text == "var" {
				return ""
			}
			return text
		}
	}
	return ""
}

// --- ArgInFormattedQuantityStringRes ---

func quantityStringFileGate(file *scanner.File) bool {
	return bytes.Contains(file.Content, []byte("getQuantityString"))
}

// checkArgInFormattedQuantityStringRes reports a format argument of Resources.getQuantityString (after the id and
// the quantity) that is not a getFormattedCount() call. In Java a local variable initialized by one also counts, as
// in Lint, which reads only PsiLocalVariable initializers; a Kotlin variable never does.
func checkArgInFormattedQuantityStringRes(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	if file.FlatType(idx) == "method_invocation" && javaMethodInvocationName(file, idx) != "getQuantityString" {
		return
	}
	call, ok := lintCallAt(file, idx)
	if !ok || call.Name != "getQuantityString" || len(call.Args) <= 2 || !slackReceiverIsResources(ctx, call) {
		return
	}
	for _, arg := range call.Args[2:] {
		if arg == 0 || slackIsFormattedCount(file, arg) {
			continue
		}
		ctx.EmitAtNode(arg, "This may require a localized count modifier. If so, use `LocalizationUtils.getFormattedCount()`. Consult #plz-localization if you are unsure.")
	}
}

// slackReceiverIsResources reports whether getQuantityString is called on android.content.res.Resources: a
// receiver whose resolved type is Resources, or one spelled `resources`/`getResources()`, Android's accessor.
func slackReceiverIsResources(ctx *api.Context, call lintCall) bool {
	if call.ReceiverNode == 0 {
		return false
	}
	// Android's accessor, `context.resources` or `getResources()`, counts when its owner is shown to be a Context,
	// View or Fragment. The resolver is not asked for the accessor's own type: for a member it does not know, it
	// answers with the owner's type.
	file := ctx.File
	recv := call.ReceiverNode
	switch file.FlatType(recv) {
	case "navigation_expression":
		if slackNavigationSelector(file, recv) == "resources" {
			return slackIsResourcesOwner(ctx, file.FlatFirstChild(recv))
		}
	case "call_expression", "method_invocation":
		if c, ok := lintCallAt(file, recv); ok && c.Name == "getResources" && len(c.Args) == 0 {
			if c.ReceiverNode == 0 {
				return slackEnclosingClassIsResourcesOwner(ctx, recv)
			}
			return slackIsResourcesOwner(ctx, c.ReceiverNode)
		}
	}
	if typ := slackReceiverType(ctx, recv); typ != nil {
		return slackTypeIsResources(ctx, typ)
	}
	// An unqualified `resources` with no declaration in scope is the enclosing Activity's, View's or Fragment's.
	return file.FlatType(recv) == "simple_identifier" && file.FlatNodeText(recv) == "resources" &&
		slackEnclosingClassIsResourcesOwner(ctx, recv)
}

func slackIsResourcesOwner(ctx *api.Context, owner uint32) bool {
	typ := slackReceiverType(ctx, owner)
	return typ != nil && slackTypeIsAny(ctx.Resolver, typ, slackResourcesOwners)
}

// slackResourcesOwners are the Android types whose getResources() returns android.content.res.Resources.
var slackResourcesOwners = []string{"android.content.Context", "android.view.View", "androidx.fragment.app.Fragment",
	"android.app.Fragment", "android.content.ContextWrapper", "android.app.Activity", "android.app.Application",
	"android.app.Service", "androidx.appcompat.app.AppCompatActivity", "androidx.activity.ComponentActivity",
	"androidx.fragment.app.FragmentActivity"}

func slackTypeIsResources(ctx *api.Context, typ *typeinfer.ResolvedType) bool {
	if typ.Name == "Resources" && !strings.Contains(typ.FQN, ".") && lintTypeRefMatches(ctx.File, "Resources", fqcnAndroidResources) {
		return true
	}
	return slackTypeIsAny(ctx.Resolver, typ, []string{fqcnAndroidResources})
}

// slackTypeIsAny reports whether a resolved type is one of roots (FQNs) or a source class extending one.
func slackTypeIsAny(resolver typeinfer.TypeResolver, typ *typeinfer.ResolvedType, roots []string) bool {
	seen := map[string]bool{}
	var visit func(string) bool
	visit = func(name string) bool {
		if name == "" || seen[name] {
			return false
		}
		seen[name] = true
		for _, r := range roots {
			if name == r {
				return true
			}
		}
		if resolver == nil {
			return false
		}
		info := resolver.ClassHierarchy(name)
		if info == nil {
			return false
		}
		for _, r := range roots {
			if info.FQN == r {
				return true
			}
		}
		for _, super := range info.Supertypes {
			if visit(super) {
				return true
			}
		}
		return false
	}
	return visit(typ.FQN)
}

// slackEnclosingClassIsResourcesOwner reports whether the class around an unqualified `resources`/`getResources()`
// extends a Context, View or Fragment type, so the accessor is Android's.
func slackEnclosingClassIsResourcesOwner(ctx *api.Context, idx uint32) bool {
	file := ctx.File
	cls, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_declaration")
	if !ok {
		return false
	}
	var supers []string
	if file.Language == scanner.LangJava {
		if sc, ok := file.FlatFindChild(cls, "superclass"); ok {
			for c := file.FlatFirstChild(sc); c != 0; c = file.FlatNextSib(c) {
				if file.FlatIsNamed(c) {
					supers = append(supers, slackJavaTypeName(file, c))
				}
			}
		}
	} else {
		for _, ref := range androidDirectSupertypesFlat(file, cls) {
			supers = append(supers, ref.name)
		}
	}
	imp := lintImportsOf(file)
	for _, name := range supers {
		fqn := name
		if full, ok := imp.bySimple[name]; ok {
			fqn = full
		}
		if slackTypeIsAny(ctx.Resolver, &typeinfer.ResolvedType{Name: androidSimpleName(name), FQN: fqn}, slackResourcesOwners) {
			return true
		}
	}
	return false
}

// slackNavigationSelector returns the member name of a Kotlin navigation_expression.
func slackNavigationSelector(file *scanner.File, nav uint32) string {
	var last uint32
	for c := file.FlatFirstChild(nav); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "navigation_suffix" {
			if id, ok := file.FlatFindChild(c, "simple_identifier"); ok {
				last = id
			}
		}
	}
	if last == 0 {
		return ""
	}
	return file.FlatNodeText(last)
}

func slackIsFormattedCount(file *scanner.File, arg uint32) bool {
	switch file.FlatType(arg) {
	case "call_expression", "method_invocation":
		c, ok := lintCallAt(file, arg)
		return ok && c.Name == "getFormattedCount"
	case "identifier":
		init, ok := slackJavaLocalInitializer(file, arg)
		if !ok || file.FlatType(init) != "method_invocation" {
			return false
		}
		c, ok := lintCallAt(file, init)
		return ok && c.Name == "getFormattedCount"
	}
	return false
}

// slackJavaLocalInitializer returns the initializer of the Java local variable (not a field or parameter) a name
// refers to, declared before the use in an enclosing block.
func slackJavaLocalInitializer(file *scanner.File, use uint32) (uint32, bool) {
	name := file.FlatNodeText(use)
	child := use
	for parent, ok := file.FlatParent(use); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "block":
			for sib, found := file.FlatPrevSibling(child); found; sib, found = file.FlatPrevSibling(sib) {
				if file.FlatType(sib) != "local_variable_declaration" {
					continue
				}
				for d := file.FlatFirstChild(sib); d != 0; d = file.FlatNextSib(d) {
					if file.FlatType(d) == "variable_declarator" && extractIdentifierFlat(file, d) == name {
						init := lintInitializerAfterEquals(file, d)
						return init, init != 0
					}
				}
			}
		case "method_declaration", "constructor_declaration", "lambda_expression", "class_body":
			return 0, false
		}
		child = parent
	}
	return 0, false
}

// --- SpanMarkPointMissingMask ---

// spannedClasses are android.text types that inherit Spanned's flag constants.
var spannedClasses = []string{fqcnAndroidSpanned, "android.text.Spannable", "android.text.SpannableString",
	"android.text.SpannableStringBuilder", "android.text.SpannedString", "android.text.Editable"}

var spanMarkPointFields = map[string]bool{
	"SPAN_INCLUSIVE_INCLUSIVE": true, "SPAN_INCLUSIVE_EXCLUSIVE": true,
	"SPAN_EXCLUSIVE_INCLUSIVE": true, "SPAN_EXCLUSIVE_EXCLUSIVE": true,
}

func spanMarkPointFileGate(file *scanner.File) bool {
	return bytes.Contains(file.Content, []byte("SPAN_")) && bytes.Contains(file.Content, []byte("android.text"))
}

// checkSpanMarkPointMissingMask reports an ==/!= (Kotlin also ===/!==) comparison against a Spanned mark-point flag
// whose other side does not mask with SPAN_POINT_MARK_MASK. Each side is checked, so comparing two flags reports
// twice, as in Lint.
func checkSpanMarkPointMissingMask(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	var operands []uint32
	isEquality := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			operands = append(operands, child)
			continue
		}
		switch file.FlatType(child) {
		case "==", "!=", "===", "!==":
			isEquality = true
		}
	}
	if !isEquality || len(operands) != 2 {
		return
	}
	for i, operand := range operands {
		other := operands[1-i]
		name, ok := slackSpannedConstant(file, operand)
		if !ok || !spanMarkPointFields[name] || slackMasksWithPointMark(file, other) {
			continue
		}
		ctx.EmitAtNode(idx, "Do not check against "+strings.TrimSpace(file.FlatNodeText(operand))+
			" directly. Instead mask flag with Spanned.SPAN_POINT_MARK_MASK to only check MARK_POINT flags.")
	}
}

// slackSpannedConstant returns the name of the android.text.Spanned constant a reference names: `Spanned.X` (or a
// Spanned subtype such as Spannable), its full name, or a statically imported X.
func slackSpannedConstant(file *scanner.File, node uint32) (string, bool) {
	text := strings.Join(strings.Fields(file.FlatNodeText(node)), "")
	switch file.FlatType(node) {
	case "navigation_expression", "field_access":
		dot := strings.LastIndexByte(text, '.')
		if dot < 0 {
			return "", false
		}
		owner, name := text[:dot], text[dot+1:]
		for _, cls := range spannedClasses {
			if lintTypeRefMatches(file, owner, cls) {
				return name, true
			}
		}
	case "simple_identifier", "identifier":
		// A parameter or local of that name shadows the static import.
		if lintNameIsParameterAt(file, node, text) || (file.Language == scanner.LangJava && slackJavaDeclaredTypeText(file, node) != "") {
			return "", false
		}
		imp := lintImportsOf(file)
		if full, ok := imp.bySimple[text]; ok {
			for _, cls := range spannedClasses {
				if full == cls+"."+text {
					return text, true
				}
			}
			return "", false
		}
		for _, star := range imp.stars {
			for _, cls := range spannedClasses {
				if star == cls {
					return text, true
				}
			}
		}
	}
	return "", false
}

// slackMasksWithPointMark reports whether an operand is a binary expression with SPAN_POINT_MARK_MASK on either
// side. Deliberate divergence: parentheses are unwrapped. Lint does not, so it reports the correctly masked
// `(flags & Spanned.SPAN_POINT_MARK_MASK) == ...`, the only way to write the check in Java.
func slackMasksWithPointMark(file *scanner.File, node uint32) bool {
	for file.FlatType(node) == "parenthesized_expression" {
		next := uint32(0)
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				next = child
				break
			}
		}
		node = next
	}
	switch file.FlatType(node) {
	case "infix_expression", "binary_expression":
	default:
		return false
	}
	// Only a bitwise AND masks (Kotlin `and`, Java `&`); `flags or MASK` / `flags | MASK` does not.
	masked := false
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		t := file.FlatType(child)
		if t == "&" || (t == "simple_identifier" && file.FlatNodeText(child) == "and" && file.FlatType(node) == "infix_expression") {
			masked = true
		}
	}
	if !masked {
		return false
	}
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		if name, ok := slackSpannedConstant(file, child); ok && name == "SPAN_POINT_MARK_MASK" {
			return true
		}
	}
	return false
}
