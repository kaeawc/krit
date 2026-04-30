package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"strings"
)

func registerSecurityRules() {

	// --- from security.go ---
	{
		r := &ContentProviderQueryWithSelectionInterpolationRule{BaseRule: BaseRule{RuleName: "ContentProviderQueryWithSelectionInterpolation", RuleSetName: "security", Sev: "info", Desc: "Detects interpolated selection strings passed to ContentResolver.query() that may enable SQL injection."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "query" {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				selectionArg := flatNamedValueArgument(file, args, "selection")
				if selectionArg == 0 {
					selectionArg = flatPositionalValueArgument(file, args, 2)
				}
				if selectionArg == 0 || !flatContainsStringInterpolation(file, selectionArg) {
					return
				}
				if !isLikelyContentResolverQueryFlat(file, idx, args) {
					return
				}
				ctx.EmitAt(file.FlatRow(selectionArg)+1, file.FlatCol(selectionArg)+1, "Interpolated ContentResolver selection string. Use selectionArgs placeholders instead.")
			},
		})
	}
	{
		r := &FileFromUntrustedPathRule{BaseRule: BaseRule{RuleName: "FileFromUntrustedPath", RuleSetName: "security", Sev: "info", Desc: "Detects File construction from untrusted input in extraction or download functions without path traversal guards."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "File" {
					return
				}
				fn, ok := flatEnclosingFunction(file, idx)
				if !ok {
					return
				}
				fnName := strings.ToLower(extractIdentifierFlat(file, fn))
				if !isRiskyFileFromPathFunction(fnName) {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				parentArg := flatPositionalValueArgument(file, args, 0)
				childArg := flatPositionalValueArgument(file, args, 1)
				if parentArg == 0 || childArg == 0 {
					return
				}
				parentExpr := valueArgumentExpressionTextFlat(file, parentArg)
				childExpr := valueArgumentExpressionTextFlat(file, childArg)
				if childExpr == "" {
					return
				}
				if isStringLiteralExpr(childExpr) {
					if !strings.Contains(childExpr, "..") {
						return
					}
				} else if hasCanonicalPathContainmentGuardFlat(file, fn, parentExpr) {
					return
				}
				ctx.EmitAt(file.FlatRow(childArg)+1, file.FlatCol(childArg)+1, "File child path comes from untrusted input in extraction/download code. Reject '..' segments or enforce canonical-path containment before writing.")
			},
		})
	}
	{
		r := &HardcodedGcpServiceAccountRule{BaseRule: BaseRule{RuleName: "HardcodedGcpServiceAccount", RuleSetName: "security", Sev: "warning", Desc: "Detects embedded GCP service-account JSON or private keys committed into source files."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"string_literal"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				lowerPath := strings.ToLower(file.Path)
				if strings.HasSuffix(lowerPath, ".pem") || strings.HasSuffix(lowerPath, ".json") {
					return
				}
				text := file.FlatNodeText(idx)
				body, ok := kotlinStringLiteralBody(text)
				if !ok || !looksLikeHardcodedGcpServiceAccount(body) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Hardcoded GCP service account credential literal. Load it from a file or secret storage instead of embedding it in source.")
			},
		})
	}
	{
		r := &HardcodedBearerTokenRule{BaseRule: BaseRule{RuleName: "HardcodedBearerToken", RuleSetName: "security", Sev: "warning", Desc: "Detects bearer authorization strings with hardcoded tokens embedded directly in source code."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"string_literal"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				if _, ok := extractHardcodedBearerToken(text); !ok {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Hardcoded bearer token literal. Load the token from config or secret storage instead of embedding it in source.")
			},
		})
	}
}
