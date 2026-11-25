package rules

import (
	"strings"
	"unicode"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of the literal checks bundled with androidx.compose.ui:ui-graphics-android and ui-text-android 1.12.1
// (ColorDetector and LocaleInvalidLanguageTagDetector).

const (
	acuiGraphicsPackage = "androidx.compose.ui.graphics"
	acuiTextIntlPackage = "androidx.compose.ui.text.intl"
)

// MissingColorAlphaChannelRule ports ui-graphics-lint's MissingColorAlphaChannel: Color(0xRRGGBB) leaves the alpha
// channel at zero, so the color is invisible.
type MissingColorAlphaChannelRule struct {
	FlatDispatchBase
	BaseRule
}

// InvalidColorHexValueRule ports ui-graphics-lint's InvalidColorHexValue: Color(0x...) with neither 6 nor 8 hex
// digits leaves channels undefined.
type InvalidColorHexValueRule struct {
	FlatDispatchBase
	BaseRule
}

// InvalidLanguageTagDelimiterRule ports ui-text-lint's InvalidLanguageTagDelimiter: Compose Locale and LocaleList
// language tags must separate subtags with hyphens, not underscores.
type InvalidLanguageTagDelimiterRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *MissingColorAlphaChannelRule) fileMentions() []string { return []string{acuiGraphicsPackage} }
func (r *InvalidColorHexValueRule) fileMentions() []string     { return []string{acuiGraphicsPackage} }
func (r *InvalidLanguageTagDelimiterRule) fileMentions() []string {
	return []string{acuiTextIntlPackage}
}

func (r *MissingColorAlphaChannelRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "MissingColorAlphaChannel", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"Only Color(...) calls whose Color name the file binds to androidx.compose.ui.graphics are checked.",
		}}
}

func (r *InvalidColorHexValueRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "InvalidColorHexValue", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"Only Color(...) calls whose Color name the file binds to androidx.compose.ui.graphics are checked.",
		}}
}

func (r *InvalidLanguageTagDelimiterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "InvalidLanguageTagDelimiter", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"Language tags are evaluated from string literals, + concatenation and same-file vals and constants; " +
				"constants declared in other files are not followed.",
		}}
}

// registerAcuiRule registers a Compose UI Lint port. Like the Compose runtime ports, these run on test sources too
// (Lint's scope includes TEST_SOURCES), and an impl declaring fileMentions() only runs on files mentioning them.
func registerAcuiRule(rule BaseRule, impl any, nodeTypes []string, languages []scanner.Language, fix api.FixLevel,
	check func(*api.Context)) {
	confidence := api.ConfidenceHigh
	if c, ok := impl.(interface{ Confidence() float64 }); ok {
		confidence = c.Confidence()
	}
	var gate func(*scanner.File) bool
	if g, ok := impl.(interface{ fileMentions() []string }); ok {
		texts := g.fileMentions()
		gate = func(file *scanner.File) bool {
			for _, text := range texts {
				if !fileFactsCache().Mentions(file, text) {
					return false
				}
			}
			return true
		}
	}
	api.Register(&api.Rule{
		ID: rule.RuleName, Category: rule.RuleSetName, Description: rule.Desc, Sev: api.Severity(rule.Sev),
		NodeTypes: nodeTypes, Languages: languages, Confidence: confidence, Fix: fix, Implementation: impl,
		Check: check, FileGate: gate,
	})
}

func registerAndroidxComposeUILiteralRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}
	missing := &MissingColorAlphaChannelRule{BaseRule: BaseRule{RuleName: "MissingColorAlphaChannel", RuleSetName: "compose",
		Sev: "warning", Desc: "Detects Compose Color(0xRRGGBB) hex values without an alpha channel, which are fully transparent."}}
	registerAcuiRule(missing.BaseRule, missing, []string{"call_expression"}, kotlin, api.FixSemantic,
		func(ctx *api.Context) { acuiCheckColorHex(ctx, true) })

	invalid := &InvalidColorHexValueRule{BaseRule: BaseRule{RuleName: "InvalidColorHexValue", RuleSetName: "compose",
		Sev: "warning", Desc: "Detects Compose Color(0x...) hex values that are neither 6 nor 8 hex digits long."}}
	registerAcuiRule(invalid.BaseRule, invalid, []string{"call_expression"}, kotlin, api.FixNone,
		func(ctx *api.Context) { acuiCheckColorHex(ctx, false) })

	tag := &InvalidLanguageTagDelimiterRule{BaseRule: BaseRule{RuleName: "InvalidLanguageTagDelimiter", RuleSetName: "compose",
		Sev: "error", Desc: "Detects Compose Locale and LocaleList language tags that use _ instead of - between subtags."}}
	registerAcuiRule(tag.BaseRule, tag, []string{"call_expression", "object_creation_expression"},
		[]scanner.Language{scanner.LangKotlin, scanner.LangJava}, api.FixSemantic, tag.check)
}

// acuiCallTargets reports whether a lintCall targets pkg.name: the callee is the fully qualified pkg.name, or a
// simple (possibly aliased) name the file binds to pkg.name and no enclosing scope redeclares.
func acuiCallTargets(file *scanner.File, call lintCall, pkg, name string) bool {
	if file.Language == scanner.LangJava {
		text := strings.Join(strings.Fields(call.TypeText), "")
		if text == pkg+"."+name {
			return true
		}
		return text == name && acrJavaImports(file, pkg, name)
	}
	if call.ReceiverNode != 0 {
		return strings.Join(strings.Fields(call.ReceiverText), "") == pkg && call.Name == name
	}
	if call.Name == name {
		return acrCalleeResolvesTo(file, call.Idx, pkg, name)
	}
	return !acrFunctionNameShadowed(file, call.Idx, call.Name) && acrSimpleNameRefersTo(file, call.Name, pkg, name)
}

// acuiCheckColorHex mirrors ColorDetector: a one-argument androidx.compose.ui.graphics.Color call whose argument is
// a literal starting with 0x has its digits counted after dropping `_` and `L`; 6 digits is a missing alpha
// channel, 8 is valid, anything else is invalid.
func acuiCheckColorHex(ctx *api.Context, missingAlpha bool) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || len(call.Args) != 1 || call.Args[0] == 0 || !acuiCallTargets(file, call, acuiGraphicsPackage, "Color") {
		return
	}
	arg := call.Args[0]
	text := file.FlatNodeText(arg)
	if !acuiIsNumericLiteral(file, arg) || len(text) < 2 || !strings.EqualFold(text[:2], "0x") {
		return
	}
	digits := text[2:]
	if strings.ContainsAny(digits, "uU") {
		// Lint cannot resolve Color(UInt) (there is no such overload), so it reports nothing.
		return
	}
	switch len(strings.ReplaceAll(strings.ReplaceAll(digits, "_", ""), "L", "")) {
	case 8:
		return
	case 6:
		if !missingAlpha {
			return
		}
		alpha := "FF"
		for _, c := range digits {
			if !unicode.IsDigit(c) {
				if unicode.IsLower(c) {
					alpha = "ff"
				}
				break
			}
		}
		ctx.Emit(scanner.Finding{
			Line: file.FlatRow(arg) + 1, Col: file.FlatCol(arg) + 1,
			StartByte: int(file.FlatStartByte(arg)), EndByte: int(file.FlatEndByte(arg)),
			Message: "Missing Color alpha channel",
			Fix: &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(arg)), EndByte: int(file.FlatEndByte(arg)),
				Replacement: "0x" + alpha + digits},
		})
	default:
		if !missingAlpha {
			ctx.EmitAtNode(arg, "Invalid Color hex value")
		}
	}
}

// acuiIsNumericLiteral reports whether node is an integer literal token (hex, long or unsigned), not a
// parenthesized, prefixed or computed expression.
func acuiIsNumericLiteral(file *scanner.File, node uint32) bool {
	switch file.FlatType(node) {
	case "hex_literal", "integer_literal", "long_literal", "unsigned_literal", "bin_literal":
		return true
	}
	return false
}

// check mirrors LocaleInvalidLanguageTagDetector: the first argument bound to a `languageTag`/`languageTags`
// parameter of a Compose Locale or LocaleList constructor is reported when its constant value contains `_`.
func (r *InvalidLanguageTagDelimiterRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok {
		return
	}
	var param string
	switch {
	case acuiCallTargets(file, call, acuiTextIntlPackage, "Locale"):
		param = "languageTag"
	case acuiCallTargets(file, call, acuiTextIntlPackage, "LocaleList"):
		param = "languageTags"
	default:
		return
	}
	arg := acuiLanguageTagArgument(file, call, param)
	if arg == 0 {
		return
	}
	arg = acuiSkipParens(file, arg)
	value, ok := lintEvalStringStrict(file, arg)
	if !ok || !strings.Contains(value, "_") {
		return
	}
	f := scanner.Finding{
		Line: file.FlatRow(arg) + 1, Col: file.FlatCol(arg) + 1,
		StartByte: int(file.FlatStartByte(arg)), EndByte: int(file.FlatEndByte(arg)),
		Message: "A hyphen (-), not an underscore (_) delimiter should be used in a language tag",
	}
	// Lint's fix replaces the tag text where it is written; only a literal argument holds that text.
	if file.FlatType(arg) == "string_literal" && !strings.Contains(file.FlatNodeText(arg), "\\") {
		f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(arg)), EndByte: int(file.FlatEndByte(arg)),
			Replacement: strings.ReplaceAll(file.FlatNodeText(arg), "_", "-")}
	}
	ctx.Emit(f)
}

// acuiLanguageTagArgument returns the argument bound to the String tag parameter: the label when named, otherwise
// the only positional argument (Locale(String) and LocaleList(String) take exactly one; the vararg
// LocaleList(Locale...) and LocaleList(List<Locale>) overloads take Locale values, not tags).
func acuiLanguageTagArgument(file *scanner.File, call lintCall, param string) uint32 {
	for i, label := range call.Labels {
		if label == param {
			return call.Args[i]
		}
	}
	if len(call.Args) != 1 || call.Labels[0] != "" || call.Args[0] == 0 {
		return 0
	}
	if _, ok := lintEvalStringStrict(file, acuiSkipParens(file, call.Args[0])); !ok {
		return 0
	}
	return call.Args[0]
}

// acuiSkipParens strips enclosing parentheses from an expression.
func acuiSkipParens(file *scanner.File, node uint32) uint32 {
	for node != 0 && file.FlatType(node) == "parenthesized_expression" {
		inner := uint32(0)
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				inner = child
				break
			}
		}
		node = inner
	}
	return node
}
