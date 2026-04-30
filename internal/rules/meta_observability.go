// Descriptor metadata for internal/rules/observability.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *LogLevelGuardMissingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LogLevelGuardMissing",
		RuleSet:       "observability",
		Severity:      "info",
		Description:   "Detects debug/trace log messages with interpolated calls not guarded by a log-level check.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LogWithoutCorrelationIdRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LogWithoutCorrelationId",
		RuleSet:       "observability",
		Severity:      "info",
		Description:   "Detects logger calls inside coroutine builders whose context does not include MDCContext.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LoggerInterpolatedMessageRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LoggerInterpolatedMessage",
		RuleSet:       "observability",
		Severity:      "warning",
		Description:   "Detects SLF4J/Logback/log4j logger calls whose message uses Kotlin string interpolation instead of parameterized placeholders.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LoggerWithoutLoggerFieldRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LoggerWithoutLoggerField",
		RuleSet:       "observability",
		Severity:      "warning",
		Description:   "Detects LoggerFactory.getLogger() calls inside function bodies instead of a class-level logger field.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
