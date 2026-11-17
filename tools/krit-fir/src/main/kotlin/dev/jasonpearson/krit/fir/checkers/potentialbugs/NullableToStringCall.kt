package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.oracle.ErrorInvolvement
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.isNullableOperand
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirStringConcatenationCallChecker
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirStringConcatenationCall
import org.jetbrains.kotlin.fir.expressions.arguments
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags `x.toString()` and `"$x"` where the compiler types `x` as nullable, which produce the string "null".
//
// This mirrors the Go NullableToStringCall rule: a safe call (`x?.toString()`), a call with arguments and a
// `toString` that is not the stdlib `Any?.toString()` extension (a project `Foo?.toString()`) are not reported,
// and a template part is reported by its own type. The compiler's type replaces Go's source inference, so a
// stable smart cast to non-null and a platform type are not nullable, and an inferred or typealiased nullable
// type is. A bare `toString()` on a nullable implicit receiver (`fun T?.f() = toString()`, `with(x) { toString() }`)
// is reported as detekt reports it; Go does not type implicit receivers.
internal object NullableToStringCall : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "NullableToStringCall"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(NullableToStringCall)
        override val stringConcatenationCallCheckers = setOf(TemplateChecker)
    }

    private const val CALL_MESSAGE =
        "Calling toString() on a nullable receiver may produce the string \"null\". Use a safe call with an explicit fallback instead."
    private const val TEMPLATE_MESSAGE =
        "Interpolating a nullable expression may produce the string \"null\". Use an explicit fallback instead."
    private val TO_STRING = Name.identifier("toString")
    private val STDLIB_TO_STRING = CallableId(FqName("kotlin"), TO_STRING)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val source = expression.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        if (expression.calleeReference.name != TO_STRING || expression.arguments.isNotEmpty()) return
        // A bare `toString()` reads an implicit receiver: the nullable `this` of a `T?` extension or of `with(x)`.
        val receiver = expression.explicitReceiver ?: expression.extensionReceiver ?: return
        if (receiver is FirCheckedSafeCallSubject) return
        if (expression.calleeReference.toResolvedCallableSymbol()?.callableId != STDLIB_TO_STRING) return
        if (ErrorInvolvement.involvesError(receiver) || !receiver.isNullableOperand()) return
        report(source, CALL_MESSAGE)
    }

    private object TemplateChecker : FirStringConcatenationCallChecker(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(expression: FirStringConcatenationCall) {
            for (part in expression.argumentList.arguments) {
                if (part is FirLiteralExpression) continue
                // A smart-cast part has a synthetic source; the written expression is behind it.
                val source = (part as? FirSmartCastExpression)?.originalExpression?.source ?: part.source ?: continue
                if (source.kind !is KtRealSourceElementKind) continue
                if (ErrorInvolvement.involvesError(part) || !part.isNullableOperand()) continue
                report(source, TEMPLATE_MESSAGE)
            }
        }
    }
}
