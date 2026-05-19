package dev.jasonpearson.krit.fir

import dev.jasonpearson.krit.fir.checkers.ComposeRememberWithoutKey
import dev.jasonpearson.krit.fir.checkers.FlowCollectInOnCreate
import dev.jasonpearson.krit.fir.checkers.InjectDispatcher
import dev.jasonpearson.krit.fir.checkers.UnsafeCastWhenNullable
import dev.jasonpearson.krit.fir.oracle.OracleClassChecker
import dev.jasonpearson.krit.fir.oracle.OracleExpressionChecker
import dev.jasonpearson.krit.fir.rules.SmokeChecker
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.extensions.FirAdditionalCheckersExtension

class KritFirCheckers(session: FirSession) : FirAdditionalCheckersExtension(session) {
    // The oracle expression checker is included unconditionally and gates
    // itself on `OracleCollectorRegistry.current() != null`, so non-oracle
    // paths (diagnostic `check` command) pay only a thread-local lookup
    // per call expression.
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(
            FlowCollectInOnCreate,
            ComposeRememberWithoutKey,
            InjectDispatcher,
            OracleExpressionChecker,
        )
        override val typeOperatorCallCheckers = setOf(
            UnsafeCastWhenNullable,
        )
    }

    // Same self-gating story for OracleClassChecker on the declaration
    // side: non-oracle paths pay only the thread-local probe.
    override val declarationCheckers = object : DeclarationCheckers() {
        override val classCheckers = setOf(SmokeChecker, OracleClassChecker)
    }
}
