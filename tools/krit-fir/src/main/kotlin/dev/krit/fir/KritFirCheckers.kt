package dev.krit.fir

import dev.krit.fir.checkers.ComposeRememberWithoutKey
import dev.krit.fir.checkers.FlowCollectInOnCreate
import dev.krit.fir.checkers.UnsafeCastWhenNullable
import dev.krit.fir.rules.SmokeChecker
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.extensions.FirAdditionalCheckersExtension

class KritFirCheckers(session: FirSession) : FirAdditionalCheckersExtension(session) {
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(
            FlowCollectInOnCreate,
            ComposeRememberWithoutKey,
        )
        override val typeOperatorCallCheckers = setOf(
            UnsafeCastWhenNullable,
        )
    }

    override val declarationCheckers = object : DeclarationCheckers() {
        override val classCheckers = setOf(SmokeChecker)
    }
}
