package dev.jasonpearson.krit.fir.checkers.privacy

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.hasEncryptedStorageReceiver
import dev.jasonpearson.krit.fir.support.isPlainFileWrite
import dev.jasonpearson.krit.fir.support.isSensitiveFileName
import dev.jasonpearson.krit.fir.support.plainFileReceiverLiteral
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall

internal object PlainFileWriteOfSensitive : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "PlainFileWriteOfSensitive"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(PlainFileWriteOfSensitive)
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (!isPlainFileWrite(expression) || hasEncryptedStorageReceiver(expression)) return
        val path = plainFileReceiverLiteral(expression) ?: return
        if (!isSensitiveFileName(path)) return
        report(
            expression.source,
            "Plain-file write to a path containing sensitive terms. Use EncryptedFile or the Android Keystore for sensitive data.",
        )
    }
}
