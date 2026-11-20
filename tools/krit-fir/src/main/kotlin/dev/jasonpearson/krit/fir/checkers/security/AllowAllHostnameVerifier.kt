package dev.jasonpearson.krit.fir.checkers.security

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirCallableReferenceAccessChecker
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirCallChecker
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirPropertyAccessExpressionChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirCallableReferenceAccess
import org.jetbrains.kotlin.fir.expressions.FirDelegatedConstructorCall
import org.jetbrains.kotlin.fir.expressions.FirCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.Name

internal object AllowAllHostnameVerifier : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "AllowAllHostnameVerifier"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(AllowAllHostnameVerifier)
        override val propertyAccessExpressionCheckers = setOf(FieldUse)
        override val callableReferenceAccessCheckers = setOf(ConstructorReference)
        override val callCheckers = setOf(SuperConstructor)
    }

    private val verifier = classIdOf("org.apache.http.conn.ssl", "AllowAllHostnameVerifier")
    private val field = CallableId(
        classIdOf("org.apache.http.conn.ssl", "SSLSocketFactory"),
        Name.identifier("ALLOW_ALL_HOSTNAME_VERIFIER"),
    )

    private fun message(name: String) =
        "Using the `$name` HostnameVerifier is unsafe because it always returns true, which could cause insecure network traffic due to trusting TLS/SSL server certificates for wrong hostnames"

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val source = expression.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        val constructor = expression.calleeReference.toResolvedCallableSymbol() as? FirConstructorSymbol ?: return
        if (constructor.callableId.classId != verifier) return
        report(source, message("AllowAllHostnameVerifier"))
    }

    private object FieldUse : FirPropertyAccessExpressionChecker(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(expression: FirPropertyAccessExpression) {
            val source = expression.source ?: return
            if (source.kind !is KtRealSourceElementKind) return
            if (expression.calleeReference.toResolvedCallableSymbol()?.callableId != field) return
            report(source, message("ALLOW_ALL_HOSTNAME_VERIFIER"))
        }
    }

    private object ConstructorReference : FirCallableReferenceAccessChecker(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(expression: FirCallableReferenceAccess) {
            val source = expression.source ?: return
            if (source.kind !is KtRealSourceElementKind) return
            val symbol = expression.calleeReference.toResolvedCallableSymbol() ?: return
            when {
                symbol is FirConstructorSymbol && symbol.callableId.classId == verifier ->
                    report(source, message("AllowAllHostnameVerifier"))
                symbol.callableId == field -> report(source, message("ALLOW_ALL_HOSTNAME_VERIFIER"))
            }
        }
    }

    private object SuperConstructor : FirCallChecker(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(expression: FirCall) {
            val delegated = expression as? FirDelegatedConstructorCall ?: return
            val source = delegated.source ?: return
            if (source.kind !is KtRealSourceElementKind) return
            val constructor = delegated.calleeReference.toResolvedCallableSymbol() as? FirConstructorSymbol ?: return
            if (constructor.callableId.classId != verifier) return
            report(source, message("AllowAllHostnameVerifier"))
        }
    }
}
