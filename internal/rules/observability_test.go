package rules_test

import "testing"

func TestLogLevelGuardMissing_Positive(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

import org.slf4j.Logger

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    logger.debug("payload=${serialize(thing)}")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_PositiveWhenFileImportsLoggingAPI(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

import org.slf4j.Logger

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(audit: Logger, thing: Thing) {
    audit.debug("payload=${serialize(thing)}")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_PositiveWhenParameterHasFullyQualifiedLoggerType(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(audit: org.slf4j.Logger, thing: Thing) {
    audit.debug("payload=${serialize(thing)}")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_PositiveWhenMarkerOverloadInterpolatesCall(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

import org.slf4j.Logger
import org.slf4j.Marker

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, marker: Marker, thing: Thing) {
    logger.debug(marker, "payload=${serialize(thing)}")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuarded(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    val isDebugEnabled: Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    if (logger.isDebugEnabled) {
        logger.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedMarkerOverload(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Marker

interface Logger {
    val isDebugEnabled: Boolean
    fun debug(marker: Marker, message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, marker: Marker, thing: Thing) {
    if (logger.isDebugEnabled) {
        logger.debug(marker, "payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeForNonLoggerReceiverWithoutLoggingImports(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface AuditSink {
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(audit: AuditSink, thing: Thing) {
    audit.debug("payload=${serialize(thing)}")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeForFullyQualifiedNonLoggerParameterType(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(audit: com.example.AuditSink, thing: Thing) {
    audit.debug("payload=${serialize(thing)}")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedByConjunction(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    val isDebugEnabled: Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing, shouldLog: Boolean) {
    if (shouldLog && logger.isDebugEnabled) {
        logger.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedByMethodCall(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    fun isDebugEnabled(): Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    if (logger.isDebugEnabled()) {
        logger.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedByBooleanComparison(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    val isDebugEnabled: Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    if (logger.isDebugEnabled == true) {
        logger.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedBySafeCallBooleanComparison(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    val isDebugEnabled: Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger?, thing: Thing) {
    if (logger?.isDebugEnabled == true) {
        logger?.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedByReversedBooleanComparison(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    val isDebugEnabled: Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    if (true == logger.isDebugEnabled) {
        logger.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedByNegatedConditionElseBranch(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    val isDebugEnabled: Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    if (!logger.isDebugEnabled) {
        println("skipping payload")
    } else {
        logger.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedByReversedNegatedBooleanComparisonElseBranch(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    val isDebugEnabled: Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    if (false == logger.isDebugEnabled) {
        println("skipping payload")
    } else {
        logger.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedByNegatedMethodCallElseBranch(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    fun isDebugEnabled(): Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    if (!logger.isDebugEnabled()) {
        println("skipping payload")
    } else {
        logger.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedByNegatedBooleanComparisonElseBranch(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    fun isDebugEnabled(): Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    if (logger.isDebugEnabled() == false) {
        println("skipping payload")
    } else {
        logger.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedBySubjectlessWhen(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    val isDebugEnabled: Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    when {
        logger.isDebugEnabled -> logger.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_PositiveWhenSubjectlessWhenHasOptionalCondition(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

import org.slf4j.Logger

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing, includePayload: Boolean) {
    when {
        logger.isDebugEnabled, includePayload -> logger.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedByWhenSubject(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    fun isDebugEnabled(): Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    when (logger.isDebugEnabled()) {
        true -> logger.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_PositiveWhenSubjectWhenHasOptionalCondition(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

import org.slf4j.Logger

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing, includePayload: Boolean) {
    when (logger.isDebugEnabled()) {
        true, includePayload -> logger.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedBySubjectlessWhenElseBranch(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    val isDebugEnabled: Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    when {
        !logger.isDebugEnabled -> println("skipping payload")
        else -> logger.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedByWhenSubjectElseBranch(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    fun isTraceEnabled(): Boolean
    fun trace(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    when (logger.isTraceEnabled()) {
        false -> println("skipping payload")
        else -> logger.trace("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_PositiveWhenOnlyOptionallyGuarded(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

import org.slf4j.Logger

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing, shouldLog: Boolean) {
    if (shouldLog || logger.isDebugEnabled) {
        logger.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_PositiveWhenMixedBooleanConditionOnlyOptionallyGuards(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

import org.slf4j.Logger

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing, shouldLog: Boolean, forceLog: Boolean) {
    if (logger.isDebugEnabled && shouldLog || forceLog) {
        logger.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenEachBooleanBranchRequiresGuard(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    val isDebugEnabled: Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing, includePayload: Boolean) {
    if ((logger.isDebugEnabled && includePayload) || logger.isDebugEnabled) {
        logger.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedByEarlyReturn(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    val isDebugEnabled: Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    if (!logger.isDebugEnabled) return
    logger.debug("payload=${serialize(thing)}")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenMixedEarlyReturnStillAlwaysExitsForDisabledLevel(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    val isDebugEnabled: Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing, shouldSkip: Boolean) {
    if ((!logger.isDebugEnabled || shouldSkip) && !logger.isDebugEnabled) return
    logger.debug("payload=${serialize(thing)}")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedByWhenEarlyExit(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    val isDebugEnabled: Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    when {
        !logger.isDebugEnabled -> return
    }
    logger.debug("payload=${serialize(thing)}")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedByEarlyReturnMethodCall(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    fun isDebugEnabled(): Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    if (!logger.isDebugEnabled()) return
    logger.debug("payload=${serialize(thing)}")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedByEarlyReturnDisjunction(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    val isDebugEnabled: Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing, shouldLog: Boolean) {
    if (!logger.isDebugEnabled || !shouldLog) return
    logger.debug("payload=${serialize(thing)}")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedByEarlyReturnBooleanComparison(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    val isDebugEnabled: Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    if (logger.isDebugEnabled != true) return
    logger.debug("payload=${serialize(thing)}")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedBySafeCallEarlyReturnBooleanComparison(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger?, thing: Thing) {
    if (logger?.isDebugEnabled != true) return
    logger?.debug("payload=${serialize(thing)}")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedByEarlyReturnReversedBooleanComparison(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    val isDebugEnabled: Boolean
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    if (false == logger.isDebugEnabled) return
    logger.debug("payload=${serialize(thing)}")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_PositiveWhenEarlyReturnOnlyOptionallyGuards(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

import org.slf4j.Logger

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing, shouldSkip: Boolean) {
    if (!logger.isDebugEnabled && shouldSkip) return
    logger.debug("payload=${serialize(thing)}")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedByWhenEarlyExitSubject(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    fun isTraceEnabled(): Boolean
    fun trace(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    when (logger.isTraceEnabled()) {
        false -> return
    }
    logger.trace("payload=${serialize(thing)}")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenGuardedByWhenSubjectBooleanComparison(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Logger {
    fun isTraceEnabled(): Boolean
    fun trace(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    when (logger.isTraceEnabled() != false) {
        true -> logger.trace("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_PositiveWhenReceiverIsLoggerTypedProperty(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

class AuditService(
    private val audit: org.slf4j.Logger
) {
    fun logPayload(thing: Thing) {
        audit.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_PositiveWhenReceiverIsLoggerTypedLocalProperty(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logger(): org.slf4j.Logger = TODO()

fun logPayload(thing: Thing) {
    val audit: org.slf4j.Logger = logger()
    audit.debug("payload=${serialize(thing)}")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenReceiverPropertyHasNonLoggerType(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

class AuditService(
    private val audit: com.example.AuditSink
) {
    fun logPayload(thing: Thing) {
        audit.debug("payload=${serialize(thing)}")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_PositiveWhenTrailingLambdaInterpolatesCall(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: mu.KLogger, thing: Thing) {
    logger.debug { "payload=${serialize(thing)}" }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeWhenTrailingLambdaIsGuarded(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: mu.KLogger, thing: Thing) {
    if (logger.isDebugEnabled) {
        logger.debug { "payload=${serialize(thing)}" }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeCatalogLoggerNotLogReceiver(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface CatalogLogger {
    fun debug(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun send(catalogLogger: CatalogLogger, thing: Thing) {
    catalogLogger.debug("payload=${serialize(thing)}")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_NegativeDialoggerNotLogReceiver(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

interface Dialogger {
    fun debug(message: String)
    fun show(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun show(dialogger: Dialogger, thing: Thing) {
    dialogger.debug("payload=${serialize(thing)}")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_PositiveAliasedLoggerImport(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

import org.slf4j.Logger as L

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(l: L, thing: Thing) {
    l.debug("payload=${serialize(thing)}")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLogWithoutCorrelationId_Positive(t *testing.T) {
	findings := runRuleByName(t, "LogWithoutCorrelationId", `
package test
import kotlinx.coroutines.launch

interface Logger {
    fun info(message: String)
}

fun run(logger: Logger) {
    launch {
        logger.info("work started")
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLogWithoutCorrelationId_Negative(t *testing.T) {
	findings := runRuleByName(t, "LogWithoutCorrelationId", `
package test
import kotlinx.coroutines.launch
import kotlinx.coroutines.slf4j.MDCContext

interface Logger {
    fun info(message: String)
}

fun run(logger: Logger) {
    launch(MDCContext()) {
        logger.info("work started")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLoggerInterpolatedMessage_PositiveBareLoggerReceiver(t *testing.T) {
	findings := runRuleByName(t, "LoggerInterpolatedMessage", `
package test

interface Logger { fun info(message: String) }

fun recordLogin(logger: Logger, id: String) {
    logger.info("user $id logged in")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLoggerInterpolatedMessage_PositiveSlf4jImport(t *testing.T) {
	findings := runRuleByName(t, "LoggerInterpolatedMessage", `
package test
import org.slf4j.LoggerFactory

private val LOG = LoggerFactory.getLogger("x")

fun recordLogin(id: String) {
    LOG.warn("user $id logged in")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLoggerInterpolatedMessage_PositiveAcrossLevels(t *testing.T) {
	findings := runRuleByName(t, "LoggerInterpolatedMessage", `
package test

interface Logger {
    fun trace(message: String)
    fun debug(message: String)
    fun info(message: String)
    fun warn(message: String)
    fun error(message: String)
}

fun all(logger: Logger, id: String) {
    logger.trace("t $id")
    logger.debug("d $id")
    logger.info("i $id")
    logger.warn("w $id")
    logger.error("e $id")
}
`)
	if len(findings) != 5 {
		t.Fatalf("expected 5 findings, got %d: %v", len(findings), findings)
	}
}

func TestLoggerInterpolatedMessage_NegativeParameterized(t *testing.T) {
	findings := runRuleByName(t, "LoggerInterpolatedMessage", `
package test

interface Logger { fun info(message: String, arg: Any) }

fun recordLogin(logger: Logger, id: String) {
    logger.info("user {} logged in", id)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLoggerInterpolatedMessage_NegativeTimber(t *testing.T) {
	findings := runRuleByName(t, "LoggerInterpolatedMessage", `
package test

object Timber {
    fun i(message: String) {}
}

fun recordLogin(id: String) {
    Timber.i("user $id logged in")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLoggerInterpolatedMessage_NegativeLambdaForm(t *testing.T) {
	findings := runRuleByName(t, "LoggerInterpolatedMessage", `
package test
import io.github.oshai.kotlinlogging.KLogger

fun recordLogin(logger: KLogger, id: String) {
    logger.info { "user $id logged in" }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLoggerInterpolatedMessage_NegativeNonLoggerReceiver(t *testing.T) {
	findings := runRuleByName(t, "LoggerInterpolatedMessage", `
package test

class Reporter { fun info(message: String) {} }

fun recordLogin(reporter: Reporter, id: String) {
    reporter.info("user $id logged in")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings on non-logger receiver, got %d: %v", len(findings), findings)
	}
}

func TestLoggerInterpolatedMessage_NegativeLiteralOnly(t *testing.T) {
	findings := runRuleByName(t, "LoggerInterpolatedMessage", `
package test

interface Logger { fun info(message: String) }

fun ping(logger: Logger) {
    logger.info("pong")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings on literal-only message, got %d: %v", len(findings), findings)
	}
}
