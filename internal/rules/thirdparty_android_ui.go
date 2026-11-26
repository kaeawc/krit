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
		case "method_declaration", "constructor_declaration":
			if params, ok := file.FlatFindChild(parent, "formal_parameters"); ok {
				for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
					if id, ok := file.FlatFindChild(p, "identifier"); ok && file.FlatType(p) == "formal_parameter" && file.FlatNodeText(id) == name {
						return slackJavaTypeChildText(file, p)
					}
				}
			}
		}
		child = parent
	}
	return ""
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
	text := strings.Join(strings.Fields(call.ReceiverText), "")
	last := text[strings.LastIndexAny(text, ".")+1:]
	if last == "resources" || last == "getResources()" {
		return true
	}
	typ := slackReceiverType(ctx, call.ReceiverNode)
	return typ != nil && (typ.FQN == fqcnAndroidResources ||
		(typ.Name == "Resources" && lintTypeRefMatches(ctx.File, "Resources", fqcnAndroidResources)))
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
	case "infix_expression", "binary_expression", "additive_expression", "multiplicative_expression":
	default:
		return false
	}
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		if name, ok := slackSpannedConstant(file, child); ok && name == "SPAN_POINT_MARK_MASK" {
			return true
		}
	}
	return false
}
