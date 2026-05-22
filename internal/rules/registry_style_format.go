package rules

import (
	"fmt"
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
			Needs: api.NeedsLinePass, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Fix: api.FixCosmetic, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &NoTabsRule{BaseRule: BaseRule{RuleName: "NoTabs", RuleSetName: "style", Sev: "warning", Desc: "Detects tab characters used for indentation instead of spaces."}, IndentSize: 4}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsLinePass, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Fix: api.FixCosmetic, Implementation: r,
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
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsLinePass, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &NewLineAtEndOfFileRule{BaseRule: BaseRule{RuleName: "NewLineAtEndOfFile", RuleSetName: "style", Sev: "warning", Desc: "Detects files that do not end with a newline character."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsLinePass, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Fix: api.FixCosmetic, Implementation: r,
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
			Languages:      []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence:     0.95,
			Implementation: r,
			Check:          r.check,
		})
	}
	{
		r := &CascadingCallWrappingRule{BaseRule: BaseRule{RuleName: "CascadingCallWrapping", RuleSetName: "style", Sev: "warning", Desc: "Detects chained calls that are not properly indented from the previous line."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsLinePass, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &UnderscoresInNumericLiteralsRule{BaseRule: BaseRule{RuleName: "UnderscoresInNumericLiterals", RuleSetName: "style", Sev: "warning", Desc: "Detects large numeric literals that should use underscore separators for readability."}, AcceptableLength: 4}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"integer_literal", "long_literal", "decimal_integer_literal"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: api.ConfidenceMedium, Fix: api.FixCosmetic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				clean := strings.TrimRight(text, "lLfFdD")
				if strings.HasPrefix(clean, "0x") || strings.HasPrefix(clean, "0b") || strings.HasPrefix(clean, "0o") {
					return
				}
				acceptLen := r.AcceptableLength
				if acceptLen <= 0 {
					acceptLen = 4
				}
				usesUnderscores := strings.Contains(clean, "_")
				if maxConsecutiveDigits(clean) <= acceptLen &&
					(!usesUnderscores || r.AllowNonStandardGrouping || hasStandardNumericGrouping(clean)) {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Numeric literal '%s' should use underscores for readability.", text))
				suffix := ""
				digits := stripNumericLiteralUnderscores(clean)
				if strings.HasSuffix(text, "L") || strings.HasSuffix(text, "l") {
					suffix = text[len(text)-1:]
				}
				formatted := formatWithUnderscores(digits) + suffix
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
			Needs: api.NeedsLinePass, Fix: api.FixCosmetic, Implementation: r,
			Check: r.check,
		})
	}
}
