package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerPotentialbugsExceptionsRules() {

	// --- from potentialbugs_exceptions.go ---
	{
		r := &PrintStackTraceRule{BaseRule: BaseRule{RuleName: "PrintStackTrace", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects printStackTrace() calls that should use a logger instead."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if strings.HasSuffix(file.Path, ".gradle.kts") {
					return
				}
				if !flatCallExpressionNameEquals(file, idx, "printStackTrace") {
					return
				}
				if !printStackTraceReceiverIsThrowable(ctx, idx) {
					return
				}
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
			},
		})
	}
	{
		r := &TooGenericExceptionCaughtRule{BaseRule: BaseRule{RuleName: "TooGenericExceptionCaught", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects catching overly generic exception types like Exception or Throwable."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"catch_block"}, Implementation: r,
			Check: r.checkNode,
		})
	}
	{
		r := &TooGenericExceptionThrownRule{BaseRule: BaseRule{RuleName: "TooGenericExceptionThrown", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects throwing overly generic exception types like Exception or Throwable."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"jump_expression", "throw_expression", "throw_statement"}, Needs: api.NeedsResolver, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &UnreachableCatchBlockRule{BaseRule: BaseRule{RuleName: "UnreachableCatchBlock", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects catch blocks that are unreachable because a more general exception type is caught above."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"try_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Needs: api.NeedsResolver,
			Tags:  []string{"precompile"},
			Check: r.checkFlatNode,
		})
	}
	{
		r := &MissingReturnRule{BaseRule: BaseRule{RuleName: "MissingReturn", RuleSetName: "potential-bugs", Sev: "error", Desc: "Detects block-bodied functions with a non-Unit return type whose body does not terminate on every path."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceHigh, Implementation: r,
			Needs: api.NeedsResolver,
			Tags:  []string{"precompile"},
			Check: r.check,
		})
	}
}
