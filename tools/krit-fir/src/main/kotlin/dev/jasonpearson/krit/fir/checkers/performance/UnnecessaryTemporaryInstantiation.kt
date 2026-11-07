package dev.jasonpearson.krit.fir.checkers.performance

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags `toString()` called directly on a boxed-primitive constructor call,
// `Integer(1).toString()`: the temporary wrapper exists only to be turned back
// into a String. Like detekt 1.23.8 and the Go rule, the constructor is one of
// Boolean, Byte, Short, Integer, Long, Float or Double, and the finding sits on
// the constructor call. Here the class is identified by resolution, so an
// import alias or a type alias of the wrapper is reported too.
internal object UnnecessaryTemporaryInstantiation : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UnnecessaryTemporaryInstantiation"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(UnnecessaryTemporaryInstantiation)
    }

    private const val MESSAGE = "Avoid temporary objects when converting primitive types to `String`."

    private val toStringName = Name.identifier("toString")

    private val javaLang = FqName("java.lang")
    private val wrapperClassIds: Set<ClassId> =
        listOf("Boolean", "Byte", "Short", "Integer", "Long", "Float", "Double")
            .mapTo(HashSet()) { ClassId(javaLang, Name.identifier(it)) }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (expression.calleeReference.name != toStringName) return
        val receiver = when (val explicit = expression.explicitReceiver) {
            is FirCheckedSafeCallSubject -> explicit.originalReceiverRef.value
            else -> explicit
        }
        val constructorCall = receiver as? FirFunctionCall ?: return
        val symbol = constructorCall.calleeReference.toResolvedCallableSymbol() as? FirConstructorSymbol ?: return
        if (symbol.resolvedReturnType.classId !in wrapperClassIds) return
        report(constructorCall.source, MESSAGE)
    }
}
