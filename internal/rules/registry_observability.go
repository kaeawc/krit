package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerObservabilityRules() {

	// --- from observability.go ---
	{
		r := &LogLevelGuardMissingRule{BaseRule: BaseRule{RuleName: "LogLevelGuardMissing", RuleSetName: "observability", Sev: "info", Desc: "Detects debug/trace log messages with interpolated calls not guarded by a log-level check."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				level := flatCallExpressionName(file, idx)
				if level != "debug" && level != "trace" {
					return
				}
				receiver := flatReceiverNameFromCall(file, idx)
				if receiver == "" || receiver == "Timber" {
					return
				}
				knownLoggerImport, loggerAliases := buildLoggerImportsFromAST(file)
				if !isLikelyLogReceiver(receiver, loggerAliases) && !knownLoggerImport && !receiverHasKnownLoggerTypeFlat(file, idx, receiver) {
					return
				}
				messageNode := logLevelGuardMessageNodeFlat(file, idx)
				if messageNode == 0 || !flatContainsStringInterpolation(file, messageNode) || !containsInterpolatedCallFlat(file, messageNode) {
					return
				}
				if isInsideMatchingLogLevelGuardFlat(file, idx, receiver, level) {
					return
				}
				if isAfterMatchingLogLevelEarlyExitFlatObs(file, idx, receiver, level) {
					return
				}
				ctx.EmitAt(file.FlatRow(messageNode)+1, file.FlatCol(messageNode)+1, "Interpolated call in "+level+" log message can do work even when the level is disabled. Guard it with the matching "+logLevelGuardProperty(level)+" check or use parameterized logging.")
			},
		})
	}
	{
		r := &LogWithoutCorrelationIdRule{BaseRule: BaseRule{RuleName: "LogWithoutCorrelationId", RuleSetName: "observability", Sev: "info", Desc: "Detects logger calls inside coroutine builders whose context does not include MDCContext."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				builderName, args, lambda := coroutineBuilderPartsFlat(file, idx)
				if builderName != "launch" && builderName != "async" && builderName != "withContext" {
					return
				}
				if lambda == 0 || coroutineContextHasMDCFlat(file, args) {
					return
				}
				logCall := firstCorrelationSensitiveLogCallFlat(file, lambda)
				if logCall == 0 {
					return
				}
				ctx.EmitAt(file.FlatRow(logCall)+1, file.FlatCol(logCall)+1, "Logger call inside coroutine without MDCContext(). Add MDCContext to preserve correlation IDs.")
			},
		})
	}
	{
		r := &LoggerWithoutLoggerFieldRule{BaseRule: BaseRule{RuleName: "LoggerWithoutLoggerField", RuleSetName: "observability", Sev: "warning", Desc: "Detects LoggerFactory.getLogger() calls inside function bodies instead of a class-level logger field."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "getLogger" {
					return
				}
				if flatReceiverNameFromCall(file, idx) != "LoggerFactory" {
					return
				}
				if _, ok := flatEnclosingFunction(file, idx); !ok {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Move LoggerFactory.getLogger(...) to a class-level logger field instead of creating a logger inside the function body.")
			},
		})
	}
}
