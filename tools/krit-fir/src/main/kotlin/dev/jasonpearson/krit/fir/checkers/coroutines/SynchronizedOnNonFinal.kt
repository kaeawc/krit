package dev.jasonpearson.krit.fir.checkers.coroutines

import com.intellij.lang.LighterASTNode
import com.intellij.openapi.util.Ref
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirImplicitInvokeCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirSyntheticPropertySymbol
import org.jetbrains.kotlin.text

// Flags `synchronized(lock) { }` whose lock is a `var`: reassigning it swaps
// the monitor object, so two threads can hold "the" lock at the same time.
//
// Mirrors the Go SynchronizedOnNonFinal rule:
// - The call is written `synchronized(...)` (optionally qualified), whatever
//   it resolves to, as Go matches the call by name: kotlin.synchronized, a
//   wrapper, or a value named synchronized called through `invoke`.
// - The lock is the first unlabelled argument inside the parentheses, and it
//   must be a bare name. A `lock = ...` argument, a qualified `this.lock`, or
//   a parenthesized or compound expression is not inspected.
// - The message names the lock as written.
//
// Where Go looks the name up among the property declarations (members or
// locals, at any depth) of the nearest enclosing class or object, the checker
// reads the declaration the name resolves to, and reports when it is a
// Kotlin `var` (a member, top-level, or local variable). So:
// - A parameter or `val` that shares its name with a `var` declared somewhere
//   in the class is not reported: the lock is final (Go reports it).
// - A `var` Go cannot see from the class body is reported: a top-level
//   property, a primary-constructor `var`, an outer or inherited class's
//   property, a scope receiver's property (`with(o)`), a destructured local,
//   or any `var` used outside a class (Go needs an enclosing class).
// Java getter/setter pairs (synthetic properties) and Java fields are not
// reported, as Go never sees their declarations either.
internal object SynchronizedOnNonFinal : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "SynchronizedOnNonFinal"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(SynchronizedOnNonFinal)
    }

    private const val SYNCHRONIZED = "synchronized"

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (writtenCalleeName(expression) != SYNCHRONIZED) return

        for (argument in arguments(expression)) {
            val lock = (if (argument is FirSmartCastExpression) argument.originalExpression else argument)
                as? FirPropertyAccessExpression ?: continue
            if (lock.explicitReceiver != null) continue
            val source = lock.source ?: continue
            if (source.kind !is KtRealSourceElementKind) continue
            if (source.elementType != KtNodeTypes.REFERENCE_EXPRESSION) continue
            if (!isFirstPositionalArgument(source)) continue

            val symbol = lock.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol ?: return
            if (symbol is FirSyntheticPropertySymbol || !symbol.isVar) return
            val name = source.text?.toString() ?: return
            report(
                expression.source,
                "synchronized() on non-final property '$name'. Reassignment changes the monitor object. Use val instead of var.",
            )
            return
        }
    }

    // The name the call is written with, as Go reads it: the callee name, or
    // for `synchronized(...)` resolved to `synchronized.invoke(...)`, the name
    // of the value being invoked.
    private fun writtenCalleeName(expression: FirFunctionCall): String? {
        val nameSource = if (expression is FirImplicitInvokeCall) {
            val receiver = expression.explicitReceiver as? FirPropertyAccessExpression ?: return null
            receiver.calleeReference.source
        } else {
            expression.calleeReference.source
        }
        return nameSource?.text?.toString()
    }

    // The call's argument expressions, with varargs flattened. Labelled,
    // spread, and lambda arguments stay wrapped and are skipped by the caller.
    private fun arguments(expression: FirFunctionCall): List<FirExpression> =
        expression.argumentList.arguments
            .flatMap { if (it is FirVarargArgumentsExpression) it.arguments else listOf(it) }
            .filter { it !is FirWrappedArgumentExpression }

    // The expression is a whole value argument (not wrapped in parentheses or
    // an operator), the argument has no `name =` label, and it is the first
    // unlabelled argument in the parentheses, as Go's
    // flatPositionalValueArgument(args, 0) picks it.
    private fun isFirstPositionalArgument(source: KtSourceElement): Boolean {
        val tree = source.treeStructure
        val argument = tree.getParent(source.lighterASTNode) ?: return false
        if (argument.tokenType != KtNodeTypes.VALUE_ARGUMENT) return false
        val list = tree.getParent(argument) ?: return false
        if (list.tokenType != KtNodeTypes.VALUE_ARGUMENT_LIST) return false
        val first = children(source, list).firstOrNull {
            it.tokenType == KtNodeTypes.VALUE_ARGUMENT &&
                children(source, it).none { child -> child.tokenType == KtNodeTypes.VALUE_ARGUMENT_NAME }
        }
        return first != null && first.startOffset == argument.startOffset && first.endOffset == argument.endOffset
    }

    private fun children(source: KtSourceElement, node: LighterASTNode): List<LighterASTNode> {
        val ref = Ref<Array<LighterASTNode?>>()
        source.treeStructure.getChildren(node, ref)
        return ref.get()?.filterNotNull().orEmpty()
    }
}
