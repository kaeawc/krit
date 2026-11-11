package dev.jasonpearson.krit.fir.checkers.coroutines

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.unwrapArgument
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirAnonymousInitializer
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirFunction
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

internal object WithContextInSuspendFunctionNoop : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "WithContextInSuspendFunctionNoop"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(WithContextInSuspendFunctionNoop)
    }

    private val coroutinesPackage = FqName("kotlinx.coroutines")
    private val withContextId = CallableId(coroutinesPackage, Name.identifier("withContext"))
    private val dispatchersClass = "kotlinx/coroutines/Dispatchers"

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val dispatcher = withContextDispatcher(expression) ?: return
        if (enclosingWithContextDispatcher() != dispatcher) return
        report(
            expression.source,
            "Redundant nested withContext(Dispatchers.$dispatcher). The parent already switches to this dispatcher.",
        )
    }

    // The `X` of `withContext(Dispatchers.X)`, or null for any other call or context argument.
    private fun withContextDispatcher(call: FirFunctionCall): String? {
        if (call.calleeReference.toResolvedCallableSymbol()?.callableId != withContextId) return null
        val argument = call.argumentList.arguments.firstOrNull()?.let(::unwrapArgument) ?: return null
        val access = argument as? FirPropertyAccessExpression ?: return null
        val id = access.calleeReference.toResolvedCallableSymbol()?.callableId ?: return null
        if (id.classId?.asString() != dispatchersClass) return null
        return id.callableName.asString()
    }

    // The dispatcher of the withContext whose lambda directly encloses the current call. Any other
    // lambda, function, class or initializer on the way out starts a new context, so it ends the search.
    context(context: CheckerContext)
    private fun enclosingWithContextDispatcher(): String? {
        val elements = context.containingElements
        for (index in elements.indices.reversed()) {
            when (val element = elements[index]) {
                is FirAnonymousFunction -> {
                    if (!element.isLambda) return null
                    val owner = lambdaOwner(elements, index, element) ?: return null
                    return withContextDispatcher(owner)
                }
                is FirFunction, is FirClass, is FirAnonymousInitializer -> return null
                else -> {}
            }
        }
        return null
    }

    // The call that takes `lambda` as an argument: the nearest enclosing call listing it.
    private fun lambdaOwner(elements: List<FirElement>, lambdaIndex: Int, lambda: FirAnonymousFunction): FirFunctionCall? {
        for (index in lambdaIndex - 1 downTo 0) {
            val call = elements[index] as? FirFunctionCall ?: continue
            val takesLambda = call.argumentList.arguments.any {
                (unwrapArgument(it) as? FirAnonymousFunctionExpression)?.anonymousFunction === lambda
            }
            if (takesLambda) return call
        }
        return null
    }
}
