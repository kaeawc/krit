package rules

import (
	"regexp"
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
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixSemantic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				const message = "Do not print a stack trace. These debug statements should be removed or replaced with a logger."
				// detekt: Thread.dumpStack(), and <caught>.printStackTrace() inside the catch of <caught>.
				if flatCallExpressionNameEquals(file, idx, "dumpStack") {
					if receiverName := flatReceiverNameFromCall(file, idx); receiverName == "Thread" {
						anchor := calleeNameNode(file, idx)
						ctx.EmitAt(file.FlatRow(anchor)+1, file.FlatCol(anchor)+1, message)
					}
					return
				}
				if !flatCallExpressionNameEquals(file, idx, "printStackTrace") {
					return
				}
				if !printStackTraceReceiverIsThrowable(ctx, idx) {
					return
				}
				recv := file.FlatFirstChild(file.FlatFirstChild(idx))
				f := r.Finding(file, file.FlatRow(recv)+1, file.FlatCol(recv)+1, message)
				startByte := int(file.FlatStartByte(idx))
				endByte := int(file.FlatEndByte(idx))
				callText := string(file.Content[startByte:endByte])
				lineStart, lineEnd := startByte, endByte
				for lineStart > 0 && file.Content[lineStart-1] != '\n' {
					lineStart--
				}
				for lineEnd < len(file.Content) && file.Content[lineEnd] != '\n' {
					lineEnd++
				}
				// Remove the line only when the call is all it holds; otherwise other code would go with it.
				if lineText := strings.TrimSpace(string(file.Content[lineStart:lineEnd])); lineText == callText || lineText == callText+";" {
					if lineEnd < len(file.Content) {
						lineEnd++
					}
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   lineStart,
						EndByte:     lineEnd,
						Replacement: "",
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &TooGenericExceptionCaughtRule{BaseRule: BaseRule{RuleName: "TooGenericExceptionCaught", RuleSetName: "exceptions", Sev: "warning", Desc: "Detects catching overly generic exception types like Exception or Throwable."}, AllowedExceptionNameRegex: regexp.MustCompile(`^(?:_|(ignore|expected).*)$`)}
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
