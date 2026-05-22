package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerObservabilityRules() {

	// --- from observability.go ---
	{
		r := &LogLevelGuardMissingRule{BaseRule: BaseRule{RuleName: "LogLevelGuardMissing", RuleSetName: "observability", Sev: "info", Desc: "Detects debug/trace log messages with interpolated calls not guarded by a log-level check."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				level := compactLoggerLevel(flatCallExpressionName(file, idx))
				if level == "" {
					return
				}
				receiver := flatReceiverNameFromCall(file, idx)
				if receiver == "" {
					return
				}
				knownLoggerImport, loggerAliases := buildLoggerImportsFromAST(file)
				if !genericLogReceiverName(receiver) && !isLikelyLogReceiver(receiver, loggerAliases) && !knownLoggerImport && !receiverHasKnownLoggerTypeFlat(file, idx, receiver) {
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
		r := &LogWithoutCorrelationIDRule{BaseRule: BaseRule{RuleName: "LogWithoutCorrelationId", RuleSetName: "observability", Sev: "info", Desc: "Detects logger calls inside coroutine builders whose context does not include MDCContext."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
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
		r := &WithContextWithoutTracingContextRule{BaseRule: BaseRule{RuleName: "WithContextWithoutTracingContext", RuleSetName: "observability", Sev: "info", Desc: "Detects coroutine dispatcher switches inside active spans without tracing context propagation."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				name, ok := r.shouldFlag(ctx.File, ctx.Idx)
				if !ok {
					return
				}
				ctx.EmitAt(ctx.File.FlatRow(ctx.Idx)+1, ctx.File.FlatCol(ctx.Idx)+1,
					"Coroutine builder `"+name+"` switches dispatchers inside an active span without tracing context propagation. Add `Context.current().asContextElement()` or the current OpenTelemetry context to the coroutine context.")
			},
		})
	}
	{
		r := &SpanStartWithoutFinishRule{BaseRule: BaseRule{RuleName: "SpanStartWithoutFinish", RuleSetName: "observability", Sev: "warning", Desc: "Detects spans started into a local variable without a matching end(), use, or makeCurrent().use lifecycle call."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				name, ok := r.shouldFlag(ctx.File, ctx.Idx)
				if !ok {
					return
				}
				ctx.EmitAt(ctx.File.FlatRow(ctx.Idx)+1, ctx.File.FlatCol(ctx.Idx)+1,
					"Span `"+name+"` is started but not finished in the same block. Wrap it in `use { ... }`, use `makeCurrent().use { ... }`, or call `"+name+".end()` in a finally block.")
			},
		})
	}
	{
		r := &SpanAttributeWithHighCardinalityRule{
			BaseRule: BaseRule{RuleName: "SpanAttributeWithHighCardinality", RuleSetName: "observability", Sev: "info", Desc: "Detects OpenTelemetry span attributes that use high-cardinality keys."},
			Keys:     []string{"user_id", "session_id", "trace_id"},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				key, ok := r.shouldFlag(ctx.File, ctx.Idx)
				if !ok {
					return
				}
				ctx.EmitAt(ctx.File.FlatRow(ctx.Idx)+1, ctx.File.FlatCol(ctx.Idx)+1,
					"Span attribute key `"+key+"` is likely high-cardinality. Prefer a bounded attribute such as user tier, account type, or another low-cardinality dimension.")
			},
		})
	}
	{
		r := &NullableStructuredFieldRule{BaseRule: BaseRule{RuleName: "NullableStructuredField", RuleSetName: "observability", Sev: "info", Desc: "Detects structured log fields whose nullable value lacks an Elvis fallback."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				if !r.shouldFlag(ctx.File, ctx.Idx) {
					return
				}
				ctx.EmitAt(ctx.File.FlatRow(ctx.Idx)+1, ctx.File.FlatCol(ctx.Idx)+1,
					"Structured log field value uses a nullable safe-call without an Elvis fallback. Add `?:` with a bounded fallback value to avoid emitting null into log aggregation.")
			},
		})
	}
	{
		r := &MetricTimerOutsideBlockRule{BaseRule: BaseRule{RuleName: "MetricTimerOutsideBlock", RuleSetName: "observability", Sev: "info", Desc: "Detects timer record blocks that are empty or only read values instead of timing meaningful work."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				if !r.shouldFlag(ctx.File, ctx.Idx) {
					return
				}
				ctx.EmitAt(ctx.File.FlatRow(ctx.Idx)+1, ctx.File.FlatCol(ctx.Idx)+1,
					"Timer record block is empty or only reads a value. Move `record` around meaningful blocking or expensive work.")
			},
		})
	}
	{
		r := &MetricTagHighCardinalityRule{
			BaseRule: BaseRule{RuleName: "MetricTagHighCardinality", RuleSetName: "observability", Sev: "warning", Desc: "Detects metric tag keys that are likely high-cardinality."},
			Keys:     []string{"user_id", "session_id", "trace_id"},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				key, ok := r.shouldFlag(ctx.File, ctx.Idx)
				if !ok {
					return
				}
				ctx.EmitAt(ctx.File.FlatRow(ctx.Idx)+1, ctx.File.FlatCol(ctx.Idx)+1,
					"Metric tag key `"+key+"` is likely high-cardinality. Use bounded labels such as tier, outcome, region, or type instead.")
			},
		})
	}
	{
		r := &MetricNameMissingUnitRule{
			BaseRule: BaseRule{RuleName: "MetricNameMissingUnit", RuleSetName: "observability", Sev: "info", Desc: "Detects metric names that do not include a recognized unit suffix."},
			Suffixes: []string{"_total", "_seconds", "_bytes", "_count"},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				name, ok := r.shouldFlag(ctx.File, ctx.Idx)
				if !ok {
					return
				}
				ctx.EmitAt(ctx.File.FlatRow(ctx.Idx)+1, ctx.File.FlatCol(ctx.Idx)+1,
					"Metric name `"+name+"` does not end with a recognized unit suffix such as `_total`, `_seconds`, `_bytes`, or `_count`.")
			},
		})
	}
	{
		r := &MetricCounterNotMonotonicRule{BaseRule: BaseRule{RuleName: "MetricCounterNotMonotonic", RuleSetName: "observability", Sev: "warning", Desc: "Detects counter increment calls with negative literal amounts."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				if !r.shouldFlag(ctx.File, ctx.Idx) {
					return
				}
				ctx.EmitAt(ctx.File.FlatRow(ctx.Idx)+1, ctx.File.FlatCol(ctx.Idx)+1,
					"Counter increment uses a negative amount. Counters are monotonic; use a gauge or record a positive counter event instead.")
			},
		})
	}
	{
		r := &MdcAcrossCoroutineBoundaryRule{BaseRule: BaseRule{RuleName: "MdcAcrossCoroutineBoundary", RuleSetName: "observability", Sev: "warning", Desc: "Detects MDC.put(...) followed by a coroutine builder without MDCContext(); MDC values do not propagate across dispatchers."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "put" {
					return
				}
				if flatReceiverNameFromCall(file, idx) != "MDC" {
					return
				}
				builder, name := firstUnpropagatedCoroutineBuilderAfterFlat(file, idx)
				if builder == 0 {
					return
				}
				ctx.EmitAt(file.FlatRow(builder)+1, file.FlatCol(builder)+1,
					"Coroutine builder `"+name+"` after `MDC.put(...)` does not include `MDCContext()`. MDC values are not propagated across dispatchers; pass `MDCContext()` in the coroutine context.")
			},
		})
	}
	{
		r := &LoggerInterpolatedMessageRule{BaseRule: BaseRule{RuleName: "LoggerInterpolatedMessage", RuleSetName: "observability", Sev: "warning", Desc: "Detects SLF4J/Logback/log4j logger calls whose message uses Kotlin string interpolation instead of parameterized placeholders."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				method := flatCallExpressionName(file, idx)
				if !loggerLevelMethods[method] {
					return
				}
				receiver := flatReceiverNameFromCall(file, idx)
				if !receiverIsKnownParameterizedLoggerFlat(file, idx, receiver) {
					return
				}
				message := loggerInterpolatedMessageArgFlat(file, idx)
				if message == 0 {
					return
				}
				ctx.EmitAt(file.FlatRow(message)+1, file.FlatCol(message)+1,
					"Logger call uses string interpolation. Use parameterized placeholders ('{}') so the call skips argument evaluation when the level is disabled.")
			},
		})
	}
	{
		r := &UnstructuredErrorLogRule{BaseRule: BaseRule{RuleName: "UnstructuredErrorLog", RuleSetName: "observability", Sev: "warning", Desc: "Detects logger error/warn calls that interpolate Throwable values instead of passing them as structured throwable arguments."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				method := flatCallExpressionName(file, idx)
				if !r.methodEnabled(method) {
					return
				}
				receiver := flatReceiverNameFromCall(file, idx)
				if !receiverIsKnownLoggerFlat(file, idx, receiver) {
					return
				}
				message := unstructuredErrorLogMessageArgFlat(file, idx)
				if message == 0 {
					return
				}
				ctx.EmitAt(file.FlatRow(message)+1, file.FlatCol(message)+1,
					"Throwable value is embedded in the log message. Pass it as the throwable argument, e.g. logger."+method+"(\"failure\", e), so the stacktrace stays structured.")
			},
		})
	}
	{
		r := &TraceIDLoggedAsPlainMessageRule{BaseRule: BaseRule{RuleName: "TraceIdLoggedAsPlainMessage", RuleSetName: "observability", Sev: "info", Desc: "Detects trace, span, request, or correlation IDs embedded directly in log messages."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				method := flatCallExpressionName(file, idx)
				if !loggerLevelMethods[method] {
					return
				}
				receiver := flatReceiverNameFromCall(file, idx)
				if !receiverIsKnownLoggerFlat(file, idx, receiver) {
					return
				}
				message := traceIDPlainMessageArgFlat(file, idx, r.identifierSet())
				if message == 0 {
					return
				}
				ctx.EmitAt(file.FlatRow(message)+1, file.FlatCol(message)+1,
					"Trace, span, request, or correlation ID is embedded in the log message. Put it in MDC or a structured field instead.")
			},
		})
	}
	{
		r := &StructuredLogKeyMixedCaseRule{
			BaseRule:         BaseRule{RuleName: "StructuredLogKeyMixedCase", RuleSetName: "observability", Sev: "info", Desc: "Detects mixed snake_case and camelCase structured logging keys within one file."},
			MinKeys:          3,
			ThresholdPercent: 70,
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:          []string{"call_expression"},
			LexicalCalleeNames: []string{"addKeyValue", "put"},
			Confidence:         r.Confidence(),
			Implementation:     r,
			Check:              r.check,
		})
	}
	{
		r := &LoggerStringConcatRule{BaseRule: BaseRule{RuleName: "LoggerStringConcat", RuleSetName: "observability", Sev: "warning", Desc: "Detects SLF4J/Logback/log4j logger calls whose message uses `+` string concatenation instead of parameterized placeholders."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				method := flatCallExpressionName(file, idx)
				if !loggerLevelMethods[method] {
					return
				}
				receiver := flatReceiverNameFromCall(file, idx)
				if !receiverIsKnownLoggerFlat(file, idx, receiver) {
					return
				}
				message := loggerStringConcatMessageArgFlat(file, idx)
				if message == 0 {
					return
				}
				ctx.EmitAt(file.FlatRow(message)+1, file.FlatCol(message)+1,
					"Logger call uses string concatenation. Use parameterized placeholders ('{}') so the call skips argument evaluation when the level is disabled.")
			},
		})
	}
	{
		r := &LoggerWithoutLoggerFieldRule{BaseRule: BaseRule{RuleName: "LoggerWithoutLoggerField", RuleSetName: "observability", Sev: "warning", Desc: "Detects LoggerFactory.getLogger() calls inside function bodies instead of a class-level logger field."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
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
	{
		r := &MdcPutNoRemoveRule{BaseRule: BaseRule{RuleName: "MdcPutNoRemove", RuleSetName: "observability", Sev: "warning", Desc: "Detects MDC.put(...) inside a function with no matching MDC.remove(...), MDC.clear(), or MDCCloseable, which leaks values across reused threads."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "put" {
					return
				}
				if flatReceiverNameFromCall(file, idx) != "MDC" {
					return
				}
				fn, ok := flatEnclosingFunction(file, idx)
				if !ok {
					return
				}
				key, keyKnown := mdcStaticKeyFlat(file, idx)
				if mdcRemoveOrClearMatchesFlat(file, fn, key, keyKnown) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "MDC.put(...) is not paired with MDC.remove(...) or MDC.clear() in this function. Use MDC.putCloseable(...).use { } or remove the key in a finally block to avoid leaking values across threads.")
			},
		})
	}
}
