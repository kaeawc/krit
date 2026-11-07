package dev.jasonpearson.krit.fir

import dev.jasonpearson.krit.fir.incremental.CompileRecordingRegistry
import dev.jasonpearson.krit.fir.incremental.SurfaceFileChecker
import dev.jasonpearson.krit.fir.oracle.OracleBlockTaintChecker
import dev.jasonpearson.krit.fir.oracle.OracleCheckNotNullTaintChecker
import dev.jasonpearson.krit.fir.oracle.OracleClassChecker
import dev.jasonpearson.krit.fir.oracle.OracleElvisTaintChecker
import dev.jasonpearson.krit.fir.oracle.OracleEqualityTaintChecker
import dev.jasonpearson.krit.fir.oracle.OracleExpressionChecker
import dev.jasonpearson.krit.fir.oracle.OracleFileChecker
import dev.jasonpearson.krit.fir.oracle.OracleQualifiedAccessChecker
import dev.jasonpearson.krit.fir.oracle.OracleSafeCallTaintChecker
import dev.jasonpearson.krit.fir.oracle.OracleShadowedCallTaintChecker
import dev.jasonpearson.krit.fir.oracle.OracleSmartCastChecker
import dev.jasonpearson.krit.fir.oracle.OracleTypeOperatorTaintChecker
import dev.jasonpearson.krit.fir.rules.SmokeChecker
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.extra.UnreachableCodeChecker
import org.jetbrains.kotlin.fir.analysis.extensions.FirAdditionalCheckersExtension

class KritFirCheckers(session: FirSession) : FirAdditionalCheckersExtension(session) {
    // Oracle and smoke registration stays unconditional. Oracle checkers self-gate
    // on OracleCollectorRegistry; built-in rules are selected before K2 dispatch.
    private val merged = mergeFirRules(
        FirRuleDiscovery.enabled(),
        baseExpressions = listOf(object : ExpressionCheckers() {
            override val functionCallCheckers = setOf(OracleExpressionChecker, OracleShadowedCallTaintChecker)
            override val qualifiedAccessExpressionCheckers = setOf(OracleQualifiedAccessChecker)
            override val smartCastExpressionCheckers = setOf(OracleSmartCastChecker)
            override val safeCallExpressionCheckers = setOf(OracleSafeCallTaintChecker)
            override val checkNotNullCallCheckers = setOf(OracleCheckNotNullTaintChecker)
            override val elvisExpressionCheckers = setOf(OracleElvisTaintChecker)
            override val typeOperatorCallCheckers = setOf(OracleTypeOperatorTaintChecker)
            override val equalityOperatorCallCheckers = setOf(OracleEqualityTaintChecker)
            override val blockCheckers = setOf(OracleBlockTaintChecker)
        }),
        baseDeclarations = listOf(object : DeclarationCheckers() {
            override val classCheckers = setOf(SmokeChecker, OracleClassChecker)
            // Gives every compiled file an entry in the oracle result.
            // The surface checker is added only while the session records for incremental checking; it reports nothing.
            override val fileCheckers = setOf(OracleFileChecker) + listOfNotNull(CompileRecordingRegistry.current()?.let(::SurfaceFileChecker))
            override val controlFlowAnalyserCheckers = setOf(UnreachableCodeChecker)
        }),
        // Captured here, on the thread running the compile, so rule isolation
        // does not depend on which thread K2 dispatches checkers from.
        recorder = FirRuleErrors.current(),
    )
    override val expressionCheckers = merged.expression
    override val declarationCheckers = merged.declaration
    override val typeCheckers = merged.type
}
