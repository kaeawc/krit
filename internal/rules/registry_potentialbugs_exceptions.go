package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"strings"
)

func registerPotentialbugsExceptionsRules() {

	// --- from potentialbugs_exceptions.go ---
	{
		r := &PrintStackTraceRule{BaseRule: BaseRule{RuleName: "PrintStackTrace", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects printStackTrace() calls that should use a logger instead."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if strings.HasSuffix(file.Path, ".gradle.kts") {
					return
				}
				text := file.FlatNodeText(idx)
				if !strings.HasSuffix(text, ".printStackTrace()") {
					return
				}
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					if file.FlatType(child) == "navigation_expression" {
						navText := file.FlatNodeText(child)
						if strings.HasSuffix(navText, ".printStackTrace") {
							f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
								"Use a logger instead of printStackTrace().")
							startByte := int(file.FlatStartByte(idx))
							endByte := int(file.FlatEndByte(idx))
							for startByte > 0 && file.Content[startByte-1] != '\n' {
								startByte--
							}
							if endByte < len(file.Content) && file.Content[endByte] == '\n' {
								endByte++
							}
							f.Fix = &scanner.Fix{
								ByteMode:    true,
								StartByte:   startByte,
								EndByte:     endByte,
								Replacement: "",
							}
							ctx.Emit(f)
							return
						}
					}
				}
			},
		})
	}
	{
		r := &TooGenericExceptionCaughtRule{BaseRule: BaseRule{RuleName: "TooGenericExceptionCaught", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects catching overly generic exception types like Exception or Throwable."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"catch_block"}, OriginalV1: r,
			Needs: v2.NeedsResolver,
			Check: r.checkNode,
		})
	}
	{
		r := &TooGenericExceptionThrownRule{BaseRule: BaseRule{RuleName: "TooGenericExceptionThrown", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects throwing overly generic exception types like Exception or Throwable."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsLinePass | v2.NeedsResolver, OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &UnreachableCatchBlockRule{BaseRule: BaseRule{RuleName: "UnreachableCatchBlock", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects catch blocks that are unreachable because a more general exception type is caught above."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"try_expression"}, Confidence: 0.75, OriginalV1: r,
			Needs: v2.NeedsResolver,
			Check: r.checkFlatNode,
		})
	}
	{
		r := &UnreachableCodeRule{BaseRule: BaseRule{RuleName: "UnreachableCode", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects code after return, throw, break, or continue statements that can never execute."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"statements"}, Confidence: 0.75, Fix: v2.FixSemantic, OriginalV1: r,
			Needs: v2.NeedsTypeInfo,
			// Narrow by the four jump keywords the rule actually dispatches
			// on. Without any jump keyword a file cannot produce an
			// UNREACHABLE_CODE finding; USELESS_ELVIS diagnostics in files
			// lacking all four keywords are a documented trade-off (see
			// issue #306).
			Oracle: &v2.OracleFilter{Identifiers: []string{"return", "throw", "break", "continue"}},
			// Consumes file-level compiler diagnostics (UNREACHABLE_CODE,
			// USELESS_ELVIS) via LookupDiagnostics; never reads declarations.
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check:                  r.checkNode,
		})
	}
}
