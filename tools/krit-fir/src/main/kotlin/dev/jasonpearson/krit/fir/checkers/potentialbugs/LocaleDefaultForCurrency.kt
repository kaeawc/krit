package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.qualifiedCall
import dev.jasonpearson.krit.fir.support.unwrapArgument
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.Name

// Flags `Currency.getInstance(Locale.getDefault())` inside a class whose name says it handles money (contains "price",
// "money" or "amount", ignoring case). The currency of such a class should come from the order or transaction being
// formatted, not from the device locale.
//
// Mirrors the Go LocaleDefaultForCurrency rule:
// - the call is `java.util.Currency.getInstance` with the single argument `java.util.Locale.getDefault()` with no
//   arguments (a Locale held in a variable, or the `getDefault(Locale.Category)` overload, is not flagged);
// - the class is the nearest enclosing class; the message names it as Go does;
// - the finding is reported on the line where the whole qualified call starts.
//
// Deliberate differences from Go, pinned by goldens:
// - Precision: Go matches the texts `Currency.getInstance` and `Locale.getDefault()`, so a project `Currency` with its
//   own `getInstance` or a project `Locale` is reported. FIR requires the JDK functions.
// - Recall: an import alias or type alias of either JDK class is checked. Objects defer to the surrounding class,
//   matching Go's class_declaration ancestors rather than widening the money-related-name heuristic.
internal object LocaleDefaultForCurrency : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "LocaleDefaultForCurrency"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(LocaleDefaultForCurrency)
    }

    private val currencyGetInstance = CallableId(classIdOf("java.util", "Currency"), Name.identifier("getInstance"))
    private val localeGetDefault = CallableId(classIdOf("java.util", "Locale"), Name.identifier("getDefault"))
    private val carrierWords = listOf("price", "money", "amount")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (expression.calleeReference.toResolvedCallableSymbol()?.callableId != currencyGetInstance) return
        val argument = expression.argumentList.arguments.singleOrNull() ?: return
        if (!isLocaleGetDefault(argument)) return
        val className = enclosingClass()?.name?.asString() ?: return
        val lower = className.lowercase()
        if (carrierWords.none { it in lower }) return

        val source = expression.source ?: return
        report(
            qualifiedCall(source)?.let { lightSourceOf(it, source) } ?: source,
            "Class '$className' derives currency from Locale.getDefault(); use order or transaction currency data instead.",
        )
    }

    context(context: CheckerContext)
    private fun enclosingClass(): FirRegularClassSymbol? =
        context.containingDeclarations.filterIsInstance<FirRegularClassSymbol>()
            .lastOrNull { it.classKind != ClassKind.OBJECT }

    private fun isLocaleGetDefault(argument: FirExpression): Boolean {
        val call = unwrapArgument(argument) as? FirFunctionCall ?: return false
        return call.calleeReference.toResolvedCallableSymbol()?.callableId == localeGetDefault &&
            call.argumentList.arguments.isEmpty()
    }
}
