package dev.krit.fir.checkers

import dev.krit.fir.KritDiagnostics
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.diagnostics.reportOn
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.name.FqName

internal object ComposeRememberWithoutKey : FirFunctionCallChecker(MppCheckerKind.Common) {
    private val rememberFqName = FqName("androidx.compose.runtime.remember")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val source = expression.source ?: return

        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId?.asSingleFqName() != rememberFqName) return

        // remember(vararg keys, calculation) — if only 1 argument it's the keyless overload
        val argCount = expression.argumentList.arguments.size
        if (argCount != 1) return

        reporter.reportOn(source, KritDiagnostics.COMPOSE_REMEMBER_WITHOUT_KEY, callee.name.asString())
    }
}
