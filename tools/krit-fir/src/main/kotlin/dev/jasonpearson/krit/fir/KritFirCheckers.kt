package dev.jasonpearson.krit.fir

import dev.jasonpearson.krit.fir.oracle.OracleClassChecker
import dev.jasonpearson.krit.fir.oracle.OracleExpressionChecker
import dev.jasonpearson.krit.fir.oracle.OracleFileChecker
import dev.jasonpearson.krit.fir.oracle.OracleQualifiedAccessChecker
import dev.jasonpearson.krit.fir.oracle.OracleSmartCastChecker
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
            override val functionCallCheckers = setOf(OracleExpressionChecker)
            override val qualifiedAccessExpressionCheckers = setOf(OracleQualifiedAccessChecker)
            override val smartCastExpressionCheckers = setOf(OracleSmartCastChecker)
        }),
        baseDeclarations = listOf(object : DeclarationCheckers() {
            override val classCheckers = setOf(SmokeChecker, OracleClassChecker)
            // Gives every compiled file an entry in the oracle result.
            override val fileCheckers = setOf(OracleFileChecker)
            override val controlFlowAnalyserCheckers = setOf(UnreachableCodeChecker)
        }),
    )
    override val expressionCheckers = merged.expression
    override val declarationCheckers = merged.declaration
    override val typeCheckers = merged.type
}
