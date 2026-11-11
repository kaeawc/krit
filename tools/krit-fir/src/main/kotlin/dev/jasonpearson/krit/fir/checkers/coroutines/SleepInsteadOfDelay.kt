package dev.jasonpearson.krit.fir.checkers.coroutines

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.hasSuspendFunctionType
import dev.jasonpearson.krit.fir.support.unwrapArgument
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirFunction
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.InlineStatus
import org.jetbrains.kotlin.fir.declarations.utils.isSuspend
import org.jetbrains.kotlin.fir.expressions.FirCallableReferenceAccess
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags java.lang.Thread.sleep inside a suspend function or a suspend lambda:
// it blocks the thread the coroutine runs on, where delay() suspends.
//
// Mirrors the Go SleepInsteadOfDelay rule, which matches the text `Thread.sleep`
// (or the qualified name) and then decides by call name whether an enclosing
// lambda runs as a coroutine (a fixed list of builders) or in the caller's
// context (a fixed list of inline stdlib functions). The checker resolves the
// callee to java.lang.Thread.sleep and reads the context from the compiler:
// the enclosing function is `suspend`, or the enclosing lambda has a suspend
// function type, looking outward through lambdas of inline calls. Like Go it
// also reports a `Thread::sleep` reference passed as a call argument, and it
// reports once per call on the line the call starts.
internal object SleepInsteadOfDelay : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "SleepInsteadOfDelay"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(SleepInsteadOfDelay)
    }

    private const val MESSAGE = "This use of Thread.sleep() inside a suspend function should be replaced by delay()."
    private val threadFqName = FqName("java.lang.Thread")
    private val sleepName = Name.identifier("sleep")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val sleepCall = isThreadSleep(expression.calleeReference.toResolvedCallableSymbol())
        val sleepReference = expression.argumentList.arguments.any {
            val argument = unwrapArgument(it)
            argument is FirCallableReferenceAccess && isThreadSleep(argument.calleeReference.toResolvedCallableSymbol())
        }
        if (!sleepCall && !sleepReference) return
        val source = expression.source ?: return
        if (!inSuspendContext()) return
        report(source, MESSAGE)
    }

    private fun isThreadSleep(symbol: FirCallableSymbol<*>?): Boolean {
        val id = symbol?.callableId ?: return false
        return id.callableName == sleepName && id.classId?.asSingleFqName() == threadFqName
    }

    // The nearest enclosing function decides, passing through lambdas that
    // run in place because the call they belong to is inline.
    context(context: CheckerContext)
    private fun inSuspendContext(): Boolean {
        for (element in context.containingElements.asReversed()) {
            when (element) {
                is FirAnonymousFunction -> {
                    if (hasSuspendFunctionType(element)) return true
                    if (element.inlineStatus != InlineStatus.Inline && element.inlineStatus != InlineStatus.CrossInline) return false
                }
                is FirNamedFunction -> return element.symbol.isSuspend
                is FirFunction, is FirClass, is FirProperty -> return false
                else -> {}
            }
        }
        return false
    }
}
