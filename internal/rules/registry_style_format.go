package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerStyleFormatRules() {

	// --- from style_format.go ---
	{
		r := &TrailingWhitespaceRule{BaseRule: BaseRule{RuleName: "TrailingWhitespace", RuleSetName: "style", Sev: "warning", Desc: "Detects lines that end with trailing whitespace characters."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsLinePass, Languages: []scanner.Language{scanner.LangKotlin}, Fix: api.FixCosmetic, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &NoTabsRule{BaseRule: BaseRule{RuleName: "NoTabs", RuleSetName: "style", Sev: "warning", Desc: "Detects tab characters used for indentation instead of spaces."}, IndentSize: 4}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsLinePass, Languages: []scanner.Language{scanner.LangKotlin}, Fix: api.FixCosmetic, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &MaxLineLengthRule{
			BaseRule:                 BaseRule{RuleName: "MaxLineLength", RuleSetName: "style", Sev: "warning", Desc: "Detects lines that exceed the configured maximum character length."},
			Max:                      120,
			ExcludePackageStatements: true,
			ExcludeImportStatements:  true,
			ExcludeRawStrings:        true,
			IgnoreUnwrappableLines:   true,
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsLinePass, Languages: []scanner.Language{scanner.LangKotlin}, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &NewLineAtEndOfFileRule{BaseRule: BaseRule{RuleName: "NewLineAtEndOfFile", RuleSetName: "style", Sev: "warning", Desc: "Detects files that do not end with a newline character."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsLinePass, Languages: []scanner.Language{scanner.LangKotlin}, Fix: api.FixCosmetic, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &SpacingAfterPackageAndImportsRule{BaseRule: BaseRule{RuleName: "SpacingAfterPackageAndImports", RuleSetName: "style", Sev: "warning", Desc: "Detects missing blank lines after package and import declarations."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{
				"package_header", "import_header",
				"package_declaration", "import_declaration",
			},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: api.ConfidenceVeryHigh,
			Fix:        api.FixCosmetic, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &MaxChainedCallsOnSameLineRule{BaseRule: BaseRule{RuleName: "MaxChainedCallsOnSameLine", RuleSetName: "style", Sev: "warning", Desc: "Detects lines with more chained method calls than the configured maximum."}, MaxCalls: 5}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:      []string{"call_expression", "navigation_expression", "method_invocation", "field_access"},
			Languages:      []scanner.Language{scanner.LangKotlin},
			Confidence:     0.95,
			Implementation: r,
			Check:          r.check,
		})
	}
	{
		r := &CascadingCallWrappingRule{BaseRule: BaseRule{RuleName: "CascadingCallWrapping", RuleSetName: "style", Sev: "warning", Desc: "Detects chained calls that stay on one line after a preceding call in the chain was wrapped."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "navigation_expression", "elvis_expression"}, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &UnderscoresInNumericLiteralsRule{BaseRule: BaseRule{RuleName: "UnderscoresInNumericLiterals", RuleSetName: "style", Sev: "warning", Desc: "Detects large numeric literals that should use underscore separators for readability."}, AcceptableLength: 4}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"integer_literal", "long_literal", "decimal_integer_literal", "real_literal"}, Languages: []scanner.Language{scanner.LangKotlin}, Confidence: api.ConfidenceMedium, Fix: api.FixCosmetic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				// 123L is a long_literal wrapping an integer_literal; report it once.
				if p, ok := file.FlatParent(idx); ok && (file.FlatType(p) == "long_literal" || file.FlatType(p) == "integer_literal") {
					return
				}
				text := file.FlatNodeText(idx)
				lower := strings.ToLower(text)
				// Hex and binary literals, and unsigned ones (the grammar splits off the `u` suffix), are not checked.
				if end := int(file.FlatEndByte(idx)); end < len(file.Content) && (file.Content[end] == 'u' || file.Content[end] == 'U') {
					return
				}
				if strings.HasPrefix(lower, "0x") || strings.HasPrefix(lower, "0b") || strings.HasSuffix(lower, "u") || strings.HasSuffix(lower, "ul") {
					return
				}
				// Only the integer part matters: the digits before a fraction or exponent.
				end := len(text)
				if i := strings.IndexAny(text, ".eE"); i >= 0 {
					end = i
				}
				intPart := text[:end]
				suffixAt := strings.LastIndexAny(intPart, "lLfF")
				if suffixAt >= 0 && suffixAt == len(intPart)-1 {
					intPart = intPart[:suffixAt]
				}
				if intPart == "" || (len(intPart) > 1 && intPart[0] == '0') {
					return
				}
				acceptLen := r.AcceptableLength
				if acceptLen <= 0 {
					acceptLen = 4
				}
				var msg string
				switch {
				case !strings.Contains(intPart, "_"):
					if len(intPart) <= acceptLen {
						return
					}
					msg = "This number should be separated by underscores in order to increase readability."
				case !r.AllowNonStandardGrouping && !hasStandardNumericGrouping(intPart):
					msg = "The number contains a non standard grouping."
				default:
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
				formatted := formatWithUnderscores(stripNumericLiteralUnderscores(intPart)) + text[len(intPart):]
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: formatted,
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &EqualsOnSignatureLineRule{BaseRule: BaseRule{RuleName: "EqualsOnSignatureLine", RuleSetName: "style", Sev: "warning", Desc: "Detects expression body equals signs placed on a separate line from the function signature."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Fix: api.FixCosmetic, Implementation: r,
			Check: r.check,
		})
	}
}
