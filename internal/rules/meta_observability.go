// Descriptor metadata for internal/rules/observability.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *LogLevelGuardMissingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LogLevelGuardMissing",
		RuleSet:       "observability",
		DefaultActive: false,
	}
}

func (r *LogWithoutCorrelationIDRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LogWithoutCorrelationId",
		RuleSet:       "observability",
		DefaultActive: false,
	}
}

func (r *WithContextWithoutTracingContextRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WithContextWithoutTracingContext",
		RuleSet:       "observability",
		DefaultActive: false,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[WithContextWithoutTracingContextRule]{
				Name:        "allowedDispatchers",
				Default:     []string{},
				Description: "Dispatcher names such as IO, Default, or Dispatchers.IO to ignore.",
				Apply:       func(r *WithContextWithoutTracingContextRule, v []string) { r.AllowedDispatchers = v },
			}),
		},
	}
}

func (r *SpanStartWithoutFinishRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SpanStartWithoutFinish",
		RuleSet:       "observability",
		DefaultActive: true,
	}
}

func (r *SpanAttributeWithHighCardinalityRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SpanAttributeWithHighCardinality",
		RuleSet:       "observability",
		DefaultActive: false,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[SpanAttributeWithHighCardinalityRule]{
				Name:        "keys",
				Default:     []string{"user_id", "session_id", "trace_id"},
				Description: "Span attribute keys that should be treated as high-cardinality.",
				Apply:       func(r *SpanAttributeWithHighCardinalityRule, v []string) { r.Keys = v },
			}),
		},
	}
}

func (r *NullableStructuredFieldRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NullableStructuredField",
		RuleSet:       "observability",
		DefaultActive: false,
	}
}

func (r *MetricTimerOutsideBlockRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MetricTimerOutsideBlock",
		RuleSet:       "observability",
		DefaultActive: false,
	}
}

func (r *MetricTagHighCardinalityRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MetricTagHighCardinality",
		RuleSet:       "observability",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[MetricTagHighCardinalityRule]{
				Name:        "keys",
				Default:     []string{"user_id", "session_id", "trace_id"},
				Description: "Metric tag keys that should be treated as high-cardinality.",
				Apply:       func(r *MetricTagHighCardinalityRule, v []string) { r.Keys = v },
			}),
		},
	}
}

func (r *MetricNameMissingUnitRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MetricNameMissingUnit",
		RuleSet:       "observability",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[MetricNameMissingUnitRule]{
				Name:        "suffixes",
				Default:     []string{"_total", "_seconds", "_bytes", "_count"},
				Description: "Metric name suffixes that satisfy the unit requirement.",
				Apply:       func(r *MetricNameMissingUnitRule, v []string) { r.Suffixes = v },
			}),
		},
	}
}

func (r *MetricCounterNotMonotonicRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MetricCounterNotMonotonic",
		RuleSet:       "observability",
		DefaultActive: true,
	}
}

func (r *MdcAcrossCoroutineBoundaryRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MdcAcrossCoroutineBoundary",
		RuleSet:       "observability",
		DefaultActive: true,
	}
}

func (r *LoggerInterpolatedMessageRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LoggerInterpolatedMessage",
		RuleSet:       "observability",
		DefaultActive: true,
	}
}

func (r *UnstructuredErrorLogRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnstructuredErrorLog",
		RuleSet:       "observability",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[UnstructuredErrorLogRule]{
				Name:        "methods",
				Default:     []string{"error", "warn", "warning"},
				Description: "Logger methods to inspect for embedded Throwable values.",
				Apply:       func(r *UnstructuredErrorLogRule, v []string) { r.Methods = v },
			}),
		},
	}
}

func (r *TraceIDLoggedAsPlainMessageRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TraceIdLoggedAsPlainMessage",
		RuleSet:       "observability",
		DefaultActive: false,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[TraceIDLoggedAsPlainMessageRule]{
				Name:        "identifiers",
				Default:     []string{"traceId", "trace_id", "spanId", "span_id", "requestId", "request_id", "correlationId", "correlation_id"},
				Description: "Identifier names that should be logged via MDC or structured fields instead of message text.",
				Apply:       func(r *TraceIDLoggedAsPlainMessageRule, v []string) { r.Identifiers = v },
			}),
		},
	}
}

func (r *StructuredLogKeyMixedCaseRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StructuredLogKeyMixedCase",
		RuleSet:       "observability",
		DefaultActive: false,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[StructuredLogKeyMixedCaseRule]{
				Name:        "minKeys",
				Default:     3,
				Description: "Minimum structured log keys in a file before checking convention consistency.",
				Apply:       func(r *StructuredLogKeyMixedCaseRule, v int) { r.MinKeys = v },
			}),
			api.IntOption(api.IntOptionSpec[StructuredLogKeyMixedCaseRule]{
				Name:        "thresholdPercent",
				Default:     70,
				Description: "Percentage required for one convention to be treated as the file majority.",
				Apply:       func(r *StructuredLogKeyMixedCaseRule, v int) { r.ThresholdPercent = v },
			}),
		},
	}
}

func (r *LoggerStringConcatRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LoggerStringConcat",
		RuleSet:       "observability",
		DefaultActive: true,
	}
}

func (r *MdcPutNoRemoveRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MdcPutNoRemove",
		RuleSet:       "observability",
		DefaultActive: true,
	}
}

func (r *LoggerWithoutLoggerFieldRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LoggerWithoutLoggerField",
		RuleSet:       "observability",
		DefaultActive: true,
	}
}
