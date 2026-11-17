package dev.jasonpearson.krit.fir.checkers.exceptions

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.expandedClassType
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.types.resolvedType

// Flags a call of a listed exception constructor that passes no argument at all: no message, no cause.
//
// Mirrors the Go ThrowingExceptionsWithoutMessageOrCause rule, which reads the written callee name of
// any call and looks it up in the `exceptions` list. This checker resolves the call to a constructor
// and takes the name of the class it builds (type aliases expanded), so an import alias or a type alias
// of a listed class is found, and a function or a class in another package that is only named like a
// listed exception is not mistaken for it.
//
// Same as Go: the call may stand anywhere, not only under a `throw`; any argument, trailing lambda
// included, silences it; the finding sits on the callee name. Test-source exclusion comes from the
// rule's `excludes`, applied by Go.
internal object ThrowingExceptionsWithoutMessageOrCause :
    FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ThrowingExceptionsWithoutMessageOrCause"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(ThrowingExceptionsWithoutMessageOrCause)
    }

    // Defaults of config/default-krit.yml.
    private val defaultNames = setOf(
        "ArrayIndexOutOfBoundsException",
        "Exception",
        "IllegalArgumentException",
        "IllegalMonitorStateException",
        "IllegalStateException",
        "IndexOutOfBoundsException",
        "NullPointerException",
        "RuntimeException",
        "Throwable",
    )

    private const val MESSAGE =
        "A call to the default constructor of an exception was detected. Instead one of the constructor overloads should be called. This allows to provide more meaningful exceptions."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val source = expression.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        if (expression.argumentList.arguments.isNotEmpty()) return
        if (expression.calleeReference.toResolvedCallableSymbol() !is FirConstructorSymbol) return
        val built = expression.resolvedType.expandedClassType()?.lookupTag?.classId ?: return
        if (built.shortClassName.asString() !in exceptionNames()) return
        report(expression.calleeReference.source?.takeIf { it.kind is KtRealSourceElementKind } ?: source, MESSAGE)
    }

    private fun exceptionNames(): Set<String> {
        val configured = (config()["exceptions"] as? List<*>)?.mapNotNull { it as? String }.orEmpty()
        return configured.ifEmpty { defaultNames.toList() }.toSet()
    }
}
