package dev.jasonpearson.krit.fir

import dev.jasonpearson.krit.fir.checkers.ComposeRememberWithoutKey
import dev.jasonpearson.krit.fir.checkers.FlowCollectInOnCreate
import dev.jasonpearson.krit.fir.checkers.InjectDispatcher
import dev.jasonpearson.krit.fir.checkers.UnsafeCastWhenNullable
import dev.jasonpearson.krit.fir.oracle.OracleClassChecker
import dev.jasonpearson.krit.fir.rules.SmokeChecker
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.extensions.FirAdditionalCheckersExtension

class KritFirCheckers(session: FirSession) : FirAdditionalCheckersExtension(session) {
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(
            FlowCollectInOnCreate,
            ComposeRememberWithoutKey,
            InjectDispatcher,
        )
        override val typeOperatorCallCheckers = setOf(
            UnsafeCastWhenNullable,
        )
    }

    // OracleClassChecker is included unconditionally — it gates itself on
    // `OracleCollectorRegistry.current() != null`, so non-oracle paths
    // (diagnostic `check` command) pay only a thread-local lookup per
    // visited class.
    override val declarationCheckers = object : DeclarationCheckers() {
        override val classCheckers = setOf(SmokeChecker, OracleClassChecker)
    }
}
