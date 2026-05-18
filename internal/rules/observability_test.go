package rules_test

import (
	"testing"

	rulespkg "github.com/kaeawc/krit/internal/rules"
)

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

func TestLogLevelGuardMissing_PositiveForCompactLogReceiver(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

object AppLog {
    fun d(message: String) = Unit
}

fun serialize(value: String): String = value.uppercase()

fun logPayload(value: String) {
    AppLog.d("payload=${serialize(value)}")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected compact *Log.d receiver to be treated as logging, got %d: %v", len(findings), findings)
	}
}

func TestLogLevelGuardMissing_PositiveForTimberReceiver(t *testing.T) {
	findings := runRuleByName(t, "LogLevelGuardMissing", `
package test

object Timber {
    fun d(message: String) = Unit
}

fun serialize(value: String): String = value.uppercase()

fun logPayload(value: String) {
    Timber.d("payload=${serialize(value)}")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected Timber.d receiver to be treated as logging, got %d: %v", len(findings), findings)
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

func TestWithContextWithoutTracingContext_PositiveStartSpanUse(t *testing.T) {
	findings := runRuleByName(t, "WithContextWithoutTracingContext", `
package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withContext

interface Tracer { fun spanBuilder(name: String): SpanBuilder }
interface SpanBuilder { fun startSpan(): Span }
interface Span { fun close() }
fun <T : Span, R> T.use(block: (T) -> R): R = block(this)

fun handle(tracer: Tracer) {
    tracer.spanBuilder("handle").startSpan().use {
        runBlocking {
            withContext(Dispatchers.IO) { fetch() }
        }
    }
}
fun fetch() {}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestWithContextWithoutTracingContext_PositiveWithSpan(t *testing.T) {
	findings := runRuleByName(t, "WithContextWithoutTracingContext", `
package test

import io.opentelemetry.instrumentation.annotations.WithSpan
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

@WithSpan
suspend fun handle() {
    withContext(Dispatchers.Default) { fetch() }
}
fun fetch() {}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestWithContextWithoutTracingContext_PositivePriorSpanBinding(t *testing.T) {
	findings := runRuleByName(t, "WithContextWithoutTracingContext", `
package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

interface Tracer { fun spanBuilder(name: String): SpanBuilder }
interface SpanBuilder { fun startSpan(): Span }
interface Span { fun end() }

suspend fun handle(tracer: Tracer) {
    val span = tracer.spanBuilder("handle").startSpan()
    withContext(Dispatchers.Unconfined) { fetch() }
    span.end()
}
fun fetch() {}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestWithContextWithoutTracingContext_NegativePropagatedOrNoSpan(t *testing.T) {
	findings := runRuleByName(t, "WithContextWithoutTracingContext", `
package test

import io.opentelemetry.context.Context
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

interface Tracer { fun spanBuilder(name: String): SpanBuilder }
interface SpanBuilder { fun startSpan(): Span }
interface Span { fun close() }
fun <T : Span, R> T.use(block: (T) -> R): R = block(this)
fun Context.asContextElement(): Any = this

fun propagated(tracer: Tracer) {
    tracer.spanBuilder("handle").startSpan().use {
        withContext(Dispatchers.IO + Context.current().asContextElement()) { fetch() }
    }
}

suspend fun noSpan() {
    withContext(Dispatchers.IO) { fetch() }
}
fun fetch() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestWithContextWithoutTracingContext_AllowedDispatcher(t *testing.T) {
	rule := buildRuleIndex()["WithContextWithoutTracingContext"]
	if rule == nil {
		t.Fatal("WithContextWithoutTracingContext not registered")
	}
	impl := rule.Implementation.(*rulespkg.WithContextWithoutTracingContextRule)
	original := impl.AllowedDispatchers
	impl.AllowedDispatchers = []string{"Dispatchers.IO"}
	defer func() { impl.AllowedDispatchers = original }()

	findings := runRuleByName(t, "WithContextWithoutTracingContext", `
package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

interface Tracer { fun spanBuilder(name: String): SpanBuilder }
interface SpanBuilder { fun startSpan(): Span }
interface Span { fun close() }
fun <T : Span, R> T.use(block: (T) -> R): R = block(this)

fun handle(tracer: Tracer) {
    tracer.spanBuilder("handle").startSpan().use {
        withContext(Dispatchers.IO) { fetch() }
    }
}
fun fetch() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestSpanAttributeWithHighCardinality_PositiveSetAttribute(t *testing.T) {
	findings := runRuleByName(t, "SpanAttributeWithHighCardinality", `
package test

interface Span {
    fun setAttribute(key: String, value: String)
}

fun handle(span: Span, userId: String) {
    span.setAttribute("user_id", userId)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestSpanAttributeWithHighCardinality_PositiveSetAttributes(t *testing.T) {
	findings := runRuleByName(t, "SpanAttributeWithHighCardinality", `
package test

interface Span {
    fun setAttributes(attributes: Attributes)
}
class Attributes {
    companion object {
        fun of(key: AttributeKey, value: String): Attributes = Attributes()
    }
}
class AttributeKey {
    companion object {
        fun stringKey(name: String): AttributeKey = AttributeKey()
    }
}

fun handle(span: Span, sessionId: String) {
    span.setAttributes(Attributes.of(AttributeKey.stringKey("session_id"), sessionId))
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestSpanAttributeWithHighCardinality_NegativeLowCardinality(t *testing.T) {
	findings := runRuleByName(t, "SpanAttributeWithHighCardinality", `
package test

interface Span {
    fun setAttribute(key: String, value: String)
    fun setAttributes(attributes: Attributes)
}
class Attributes {
    companion object {
        fun of(key: AttributeKey, value: String): Attributes = Attributes()
    }
}
class AttributeKey {
    companion object {
        fun stringKey(name: String): AttributeKey = AttributeKey()
    }
}

fun handle(span: Span, userTier: String) {
    span.setAttribute("user_tier", userTier)
    span.setAttributes(Attributes.of(AttributeKey.stringKey("region"), "us-central"))
    span.setAttributes(Attributes.of(AttributeKey.stringKey("debug_value"), "user_id"))
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestSpanAttributeWithHighCardinality_ConfigurableKeys(t *testing.T) {
	rule := buildRuleIndex()["SpanAttributeWithHighCardinality"]
	if rule == nil {
		t.Fatal("SpanAttributeWithHighCardinality not registered")
	}
	impl := rule.Implementation.(*rulespkg.SpanAttributeWithHighCardinalityRule)
	original := impl.Keys
	impl.Keys = []string{"account_id"}
	defer func() { impl.Keys = original }()

	findings := runRuleByName(t, "SpanAttributeWithHighCardinality", `
package test

interface Span {
    fun setAttribute(key: String, value: String)
}

fun handle(span: Span, accountId: String) {
    span.setAttribute("account_id", accountId)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestNullableStructuredField_Positive(t *testing.T) {
	findings := runRuleByName(t, "NullableStructuredField", `
package test

interface Logger {
    fun atInfo(): Event
}
interface Event {
    fun addKeyValue(key: String, value: Any?): Event
    fun log(message: String)
}
data class User(val id: String)

fun handle(logger: Logger, user: User?) {
    logger.atInfo().addKeyValue("user_id", user?.id).log("ready")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestNullableStructuredField_NegativeFallbackOrNonNullable(t *testing.T) {
	findings := runRuleByName(t, "NullableStructuredField", `
package test

interface Logger {
    fun atInfo(): Event
}
interface Event {
    fun addKeyValue(key: String, value: Any?): Event
    fun log(message: String)
}
data class User(val id: String, val tier: String)

fun handle(logger: Logger, user: User) {
    logger.atInfo().addKeyValue("user_id", user.id).log("ready")
    logger.atInfo().addKeyValue("user_tier", user.tier).log("ready")
}

fun nullable(logger: Logger, user: User?) {
    logger.atInfo().addKeyValue("user_id", user?.id ?: "anonymous").log("ready")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

// TestNullableStructuredField_NegativeSafeCallInStringLiteral is a regression
// test for the prior raw-text scan that flagged any value-expression text
// containing "?." even when those bytes appeared inside a string literal.
func TestNullableStructuredField_NegativeSafeCallInStringLiteral(t *testing.T) {
	findings := runRuleByName(t, "NullableStructuredField", `
package test

interface Logger {
    fun atInfo(): Event
}
interface Event {
    fun addKeyValue(key: String, value: Any?): Event
    fun log(message: String)
}

fun documented(logger: Logger) {
    logger.atInfo().addKeyValue("note", "use ?. operator when nullable").log("ready")
    logger.atInfo().addKeyValue("hint", "x?.y").log("ready")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestMetricTimerOutsideBlock_PositivePropertyRead(t *testing.T) {
	findings := runRuleByName(t, "MetricTimerOutsideBlock", `
package test

interface Timer {
    fun <T> record(block: () -> T): T
}
data class Holder(val field: String)

fun handle(timer: Timer, holder: Holder) {
    timer.record { holder.field }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestMetricTimerOutsideBlock_PositiveEmpty(t *testing.T) {
	findings := runRuleByName(t, "MetricTimerOutsideBlock", `
package test

interface Timer {
    fun record(block: () -> Unit)
}

fun handle(timer: Timer) {
    timer.record { }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestMetricTimerOutsideBlock_NegativeCallWork(t *testing.T) {
	findings := runRuleByName(t, "MetricTimerOutsideBlock", `
package test

interface Timer {
    fun <T> record(block: () -> T): T
}

fun handle(timer: Timer) {
    timer.record { expensiveIo() }
}
fun expensiveIo(): String = "ok"
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestMetricTagHighCardinality_Positive(t *testing.T) {
	findings := runRuleByName(t, "MetricTagHighCardinality", `
package test

interface Registry {
    fun counter(name: String, vararg tags: String): Counter
}
interface Counter {
    fun increment()
}
data class User(val id: String)

fun handle(registry: Registry, user: User) {
    registry.counter("events", "user_id", user.id).increment()
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestMetricTagHighCardinality_NegativeLowCardinality(t *testing.T) {
	findings := runRuleByName(t, "MetricTagHighCardinality", `
package test

interface Registry {
    fun counter(name: String, vararg tags: String): Counter
}
interface Counter {
    fun increment()
}
data class User(val tier: String)

fun handle(registry: Registry, user: User) {
    registry.counter("events", "tier", user.tier).increment()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestMetricTagHighCardinality_ConfigurableKeys(t *testing.T) {
	rule := buildRuleIndex()["MetricTagHighCardinality"]
	if rule == nil {
		t.Fatal("MetricTagHighCardinality not registered")
	}
	impl := rule.Implementation.(*rulespkg.MetricTagHighCardinalityRule)
	original := impl.Keys
	impl.Keys = []string{"account_id"}
	defer func() { impl.Keys = original }()

	findings := runRuleByName(t, "MetricTagHighCardinality", `
package test

interface Registry {
    fun counter(name: String, vararg tags: String): Counter
}
interface Counter {
    fun increment()
}

fun handle(registry: Registry, accountId: String) {
    registry.counter("events", "account_id", accountId).increment()
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestMetricNameMissingUnit_Positive(t *testing.T) {
	findings := runRuleByName(t, "MetricNameMissingUnit", `
package test

interface Registry {
    fun counter(name: String): Counter
}
interface Counter {
    fun increment()
}

fun handle(registry: Registry) {
    registry.counter("requests").increment()
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestMetricNameMissingUnit_NegativeSuffix(t *testing.T) {
	findings := runRuleByName(t, "MetricNameMissingUnit", `
package test

interface Registry {
    fun counter(name: String): Counter
    fun timer(name: String): Timer
}
interface Counter { fun increment() }
interface Timer { fun record(block: () -> Unit) }

fun handle(registry: Registry) {
    registry.counter("requests_total").increment()
    registry.timer("request_duration_seconds").record { work() }
}
fun work() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestMetricCounterNotMonotonic_Positive(t *testing.T) {
	findings := runRuleByName(t, "MetricCounterNotMonotonic", `
package test

interface Counter {
    fun increment(amount: Double)
}

fun handle(counter: Counter) {
    counter.increment(-1.0)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestMetricCounterNotMonotonic_Negative(t *testing.T) {
	findings := runRuleByName(t, "MetricCounterNotMonotonic", `
package test

interface Counter {
    fun increment()
    fun increment(amount: Double)
}
interface Gauge {
    fun decrement()
}

fun handle(counter: Counter, gauge: Gauge) {
    counter.increment()
    counter.increment(1.0)
    gauge.decrement()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestSpanStartWithoutFinish_Positive(t *testing.T) {
	findings := runRuleByName(t, "SpanStartWithoutFinish", `
package test

interface Tracer { fun spanBuilder(name: String): SpanBuilder }
interface SpanBuilder { fun startSpan(): Span }
interface Span { fun end() }

fun handle(tracer: Tracer) {
    val span = tracer.spanBuilder("handle").startSpan()
    doWork()
}
fun doWork() {}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestSpanStartWithoutFinish_NegativeEndInFinally(t *testing.T) {
	findings := runRuleByName(t, "SpanStartWithoutFinish", `
package test

interface Tracer { fun spanBuilder(name: String): SpanBuilder }
interface SpanBuilder { fun startSpan(): Span }
interface Span { fun end() }

fun handle(tracer: Tracer) {
    val span = tracer.spanBuilder("handle").startSpan()
    try {
        doWork()
    } finally {
        span.end()
    }
}
fun doWork() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestSpanStartWithoutFinish_NegativeUseForms(t *testing.T) {
	findings := runRuleByName(t, "SpanStartWithoutFinish", `
package test

interface Tracer { fun spanBuilder(name: String): SpanBuilder }
interface SpanBuilder { fun startSpan(): Span }
interface Span { fun end(); fun makeCurrent(): Scope }
interface Scope
fun <T : Span, R> T.use(block: (T) -> R): R = block(this)
fun <T : Scope, R> T.use(block: (T) -> R): R = block(this)

fun directUse(tracer: Tracer) {
    tracer.spanBuilder("direct").startSpan().use { span ->
        doWork()
    }
}

fun assignedUse(tracer: Tracer) {
    val span = tracer.spanBuilder("assigned").startSpan()
    span.use {
        doWork()
    }
}

fun currentUse(tracer: Tracer) {
    val span = tracer.spanBuilder("current").startSpan()
    span.makeCurrent().use {
        doWork()
    }
}
fun doWork() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestSpanStartWithoutFinish_NegativeNestedEndIgnored(t *testing.T) {
	findings := runRuleByName(t, "SpanStartWithoutFinish", `
package test

interface Tracer { fun spanBuilder(name: String): SpanBuilder }
interface SpanBuilder { fun startSpan(): Span }
interface Span { fun end() }

fun handle(tracer: Tracer) {
    val span = tracer.spanBuilder("handle").startSpan()
    fun cleanup() {
        span.end()
    }
    doWork()
}
fun doWork() {}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLoggerInterpolatedMessage_PositiveFullyQualifiedLoggerType(t *testing.T) {
	findings := runRuleByName(t, "LoggerInterpolatedMessage", `
package test

fun recordLogin(logger: org.slf4j.Logger, id: String) {
    logger.info("user $id logged in")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLoggerInterpolatedMessage_PositiveImportedLoggerType(t *testing.T) {
	findings := runRuleByName(t, "LoggerInterpolatedMessage", `
package test
import org.slf4j.Logger

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

func TestLoggerInterpolatedMessage_PositiveGradleTaskLogger(t *testing.T) {
	findings := runRuleByName(t, "LoggerInterpolatedMessage", `
package test
import org.gradle.api.DefaultTask

abstract class ReportTask : DefaultTask() {
    fun render(id: String) {
        logger.warn("task $id")
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLoggerInterpolatedMessage_PositiveAcrossLevels(t *testing.T) {
	findings := runRuleByName(t, "LoggerInterpolatedMessage", `
package test

fun all(logger: org.slf4j.Logger, id: String) {
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

func TestLoggerInterpolatedMessage_NegativeLocalLoggerType(t *testing.T) {
	findings := runRuleByName(t, "LoggerInterpolatedMessage", `
package test

interface Logger { fun info(message: String) }

fun recordLogin(logger: Logger, id: String) {
    logger.info("user $id logged in")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings on local logger type, got %d: %v", len(findings), findings)
	}
}

func TestLoggerInterpolatedMessage_NegativeImportedNonPlaceholderLogger(t *testing.T) {
	findings := runRuleByName(t, "LoggerInterpolatedMessage", `
package test
import com.example.logging.Logger

fun recordLogin(logger: Logger, id: String) {
    logger.info("user $id logged in")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings on non-placeholder logger type, got %d: %v", len(findings), findings)
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

func TestUnstructuredErrorLog_PositiveInterpolation(t *testing.T) {
	findings := runRuleByName(t, "UnstructuredErrorLog", `
package test

interface Logger { fun error(message: String) }

fun record(logger: Logger, e: Throwable) {
    logger.error("failure: $e")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestUnstructuredErrorLog_PositiveConcatAndFormat(t *testing.T) {
	findings := runRuleByName(t, "UnstructuredErrorLog", `
package test

interface Logger {
    fun warn(message: String)
    fun error(message: String)
}

object String {
    fun format(template: String, value: Any): kotlin.String = ""
}

fun record(logger: Logger, ex: Exception) {
    logger.warn("failure: " + ex)
    logger.error(String.format("failure: %s", ex))
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
	}
}

func TestUnstructuredErrorLog_NegativeStructuredOrLookalike(t *testing.T) {
	findings := runRuleByName(t, "UnstructuredErrorLog", `
package test

interface Logger {
    fun error(message: String)
    fun error(message: String, throwable: Throwable)
}

class Builder {
    fun error(message: String) {}
}

fun record(logger: Logger, builder: Builder, e: Throwable, message: String) {
    logger.error("failure", e)
    logger.error("user error: $message")
    logger.error("user error: " + message)
    builder.error("failure: $e")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestTraceIdLoggedAsPlainMessage_Positive(t *testing.T) {
	findings := runRuleByName(t, "TraceIdLoggedAsPlainMessage", `
package test

interface Logger { fun info(message: String) }

fun record(logger: Logger, traceId: String, span_id: String, requestId: String) {
    logger.info("trace=$traceId processed")
    logger.info("span=" + span_id)
    logger.info(String.format("request=%s", requestId))
}
`)
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %d: %v", len(findings), findings)
	}
}

func TestTraceIdLoggedAsPlainMessage_Negative(t *testing.T) {
	findings := runRuleByName(t, "TraceIdLoggedAsPlainMessage", `
package test

interface Logger { fun info(message: String) }
object MDC { fun put(key: String, value: String) {} }
class Builder { fun info(message: String) {} }

fun record(logger: Logger, builder: Builder, traceId: String, traceFile: String) {
    MDC.put("trace_id", traceId)
    logger.info("processed")
    logger.info("trace file=$traceFile")
    builder.info("trace=$traceId")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestStructuredLogKeyMixedCase_PositiveSnakeMajority(t *testing.T) {
	findings := runRuleByName(t, "StructuredLogKeyMixedCase", `
package test

interface Logger { fun atInfo(): EventBuilder }
interface EventBuilder {
    fun addKeyValue(key: String, value: Any): EventBuilder
    fun log(message: String)
}

fun record(logger: Logger, id: String, req: String) {
    logger.atInfo()
        .addKeyValue("user_id", id)
        .addKeyValue("account_id", id)
        .addKeyValue("session_id", id)
        .addKeyValue("requestId", req)
        .log("done")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestStructuredLogKeyMixedCase_PositiveCamelMajority(t *testing.T) {
	findings := runRuleByName(t, "StructuredLogKeyMixedCase", `
package test

object MDC { fun put(key: String, value: String) {} }

fun record(id: String, req: String) {
    MDC.put("userId", id)
    MDC.put("accountId", id)
    MDC.put("sessionId", id)
    MDC.put("request_id", req)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestStructuredLogKeyMixedCase_NegativeConsistentSparseLookalike(t *testing.T) {
	findings := runRuleByName(t, "StructuredLogKeyMixedCase", `
package test

interface Logger { fun atInfo(): EventBuilder }
interface EventBuilder {
    fun addKeyValue(key: String, value: Any): EventBuilder
    fun log(message: String)
}
class MapLike { fun put(key: String, value: String) {} }

fun consistent(logger: Logger, map: MapLike, id: String) {
    logger.atInfo()
        .addKeyValue("user_id", id)
        .addKeyValue("account_id", id)
        .addKeyValue("request_id", id)
        .log("done")
    map.put("requestId", id)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLoggerStringConcat_PositiveBareLoggerReceiver(t *testing.T) {
	findings := runRuleByName(t, "LoggerStringConcat", `
package test

interface Logger { fun info(message: String) }

fun recordValue(logger: Logger, value: Int) {
    logger.info("value=" + value)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLoggerStringConcat_PositiveSlf4jImport(t *testing.T) {
	findings := runRuleByName(t, "LoggerStringConcat", `
package test
import org.slf4j.LoggerFactory

private val LOG = LoggerFactory.getLogger("x")

fun recordValue(value: Int) {
    LOG.warn("value=" + value)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLoggerStringConcat_NegativeParameterized(t *testing.T) {
	findings := runRuleByName(t, "LoggerStringConcat", `
package test

interface Logger { fun info(message: String, arg: Any) }

fun recordValue(logger: Logger, value: Int) {
    logger.info("value={}", value)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLoggerStringConcat_NegativeTimber(t *testing.T) {
	findings := runRuleByName(t, "LoggerStringConcat", `
package test

object Timber {
    fun i(message: String) {}
}

fun recordValue(value: Int) {
    Timber.i("value=" + value)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLoggerStringConcat_NegativeNonLoggerReceiver(t *testing.T) {
	findings := runRuleByName(t, "LoggerStringConcat", `
package test

class Reporter { fun info(message: String) {} }

fun recordValue(reporter: Reporter, value: Int) {
    reporter.info("value=" + value)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings on non-logger receiver, got %d: %v", len(findings), findings)
	}
}

func TestLoggerStringConcat_NegativeLiteralOnly(t *testing.T) {
	findings := runRuleByName(t, "LoggerStringConcat", `
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

func TestLoggerStringConcat_NegativeNumericConcat(t *testing.T) {
	findings := runRuleByName(t, "LoggerStringConcat", `
package test

interface Logger { fun info(message: String) }

fun ping(logger: Logger, a: Int, b: Int) {
    val sum = a + b
    logger.info(sum.toString())
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings on numeric concat outside arg, got %d: %v", len(findings), findings)
	}
}

// TestMdcPutNoRemove_StopsAtLocalScopeBoundaries verifies that
// MDC.clear() or MDC.remove() inside a nested lambda, local function,
// or anonymous object is NOT treated as a matching cleanup for an
// MDC.put on the enclosing function — those bodies run at a different
// time or on a different thread, so the MDC value still leaks out of
// the put's scope.
func TestMdcPutNoRemove_StopsAtLocalScopeBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		code string
		want int
	}{
		{
			name: "clear inside coroutine launch lambda does not match",
			code: `
package test

import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.launch
import org.slf4j.MDC

fun emit(req: String) {
    MDC.put("reqId", req)
    GlobalScope.launch {
        MDC.clear()
    }
}
`,
			want: 1,
		},
		{
			name: "remove inside nested local fun does not match",
			code: `
package test

import org.slf4j.MDC

fun emit(req: String) {
    MDC.put("reqId", req)
    fun cleanup() {
        MDC.remove("reqId")
    }
}
`,
			want: 1,
		},
		{
			name: "clear inside anonymous object does not match",
			code: `
package test

import org.slf4j.MDC

interface Hook {
    fun run()
}

fun emit(req: String) {
    MDC.put("reqId", req)
    val hook = object : Hook {
        override fun run() {
            MDC.clear()
        }
    }
    hook.run()
}
`,
			want: 1,
		},
		{
			name: "remove in same scope still matches",
			code: `
package test

import org.slf4j.MDC

fun emit(req: String) {
    MDC.put("reqId", req)
    try {
        // ...
    } finally {
        MDC.remove("reqId")
    }
}
`,
			want: 0,
		},
		{
			name: "clear in same scope still matches",
			code: `
package test

import org.slf4j.MDC

fun emit(req: String) {
    MDC.put("reqId", req)
    MDC.clear()
}
`,
			want: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleByName(t, "MdcPutNoRemove", tc.code)
			if len(findings) != tc.want {
				t.Fatalf("expected %d findings, got %d: %v", tc.want, len(findings), findings)
			}
		})
	}
}
