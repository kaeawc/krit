package dev.jasonpearson.krit.fir.checkers.resourcecost

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.pathTo
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirValueParameter
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.FirWhileLoop
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.FqName

// Flags `painterResource(...)` inside a list or loop body: `items { }`, `itemsIndexed { }`, a `for` loop, or
// `forEach`/`map`/`repeat` inside a lazy list. The painter is rebuilt for every iteration, so it belongs outside.
//
// Mirrors the Go rule: the call must sit directly in the loop (a slot lambda in between composes on its own
// schedule), an argument that reads anything bound per iteration cannot be hoisted, and a call under an if or when
// inside the loop paints only some items. Where Go matches the written names, this resolves the painter function
// and each loop call.
internal object ComposePainterResourceInLoop : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ComposePainterResourceInLoop"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(ComposePainterResourceInLoop)
    }

    private val painterResources = setOf(
        FqName("androidx.compose.ui.res.painterResource"),
        FqName("org.jetbrains.compose.resources.painterResource"),
    )

    // Lazy list builders whose content lambda runs once per item (`item { }` renders once and is not a loop).
    private val lazyLoops = setOf("items", "itemsIndexed")

    // Lazy list builders including the singular `item`, which still marks a lazy list lambda.
    private val lazyBuilders = lazyLoops + "item"

    // Inline iteration helpers; outside a lazy list they run a small, fixed number of times.
    private val inlineLoops = setOf("forEach", "forEachIndexed", "map", "mapIndexed", "repeat")

    private const val FOR = "for"
    private const val LAMBDA_MESSAGE = "painterResource() inside list/loop lambda creates a fresh painter per iteration; hoist it outside the lambda."
    private const val FOR_MESSAGE = "painterResource() inside for loop creates a fresh painter per iteration; hoist it outside the loop."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val source = expression.source ?: return
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId?.asSingleFqName() !in painterResources) return
        if (expression.argumentList.arguments.isEmpty()) return

        val path = pathTo(expression) ?: return
        val ancestors = path.dropLast(1).asReversed()

        // The nearest lambda or `for` that holds the call decides which loop it is in.
        val direct = ancestors.firstOrNull { isLoopLambda(it) || isForLoop(it) } ?: return
        val kind = loopKind(direct, ancestors) ?: return

        val loops = ancestors.filter { isForLoop(it) || (it is FirAnonymousFunction && it.isLambda && loopKind(it, ancestors) != null) }
        if (loops.isEmpty()) return
        if (readsPerIterationValue(expression, loops, source.startOffset)) return
        if (guardedInsideLoop(ancestors)) return
        if (kind in inlineLoops && ancestors.none { it is FirFunctionCall && builderName(it) != null }) return

        val inLambda = ancestors.any { it is FirFunctionCall && loopCallKind(it) != null }
        report(source, if (inLambda) LAMBDA_MESSAGE else FOR_MESSAGE)
    }

    private fun isForLoop(element: FirElement): Boolean =
        element is FirWhileLoop && element.source?.lighterASTNode?.tokenType == KtNodeTypes.FOR

    private fun isLoopLambda(element: FirElement): Boolean = element is FirAnonymousFunction && element.isLambda

    // The loop an element opens: FOR for a `for` statement, the loop call's name for a lambda handed to one, and
    // null for any other lambda. The owner of a lambda is the nearest call above it in [ancestors].
    private fun loopKind(element: FirElement, ancestors: List<FirElement>): String? {
        if (isForLoop(element)) return FOR
        val owner = ancestors.drop(ancestors.indexOf(element) + 1).firstOrNull { it is FirFunctionCall } as? FirFunctionCall
        return owner?.let(::loopCallKind)
    }

    // The written name of an iteration call whose lambda runs once per element, proven by the resolved callee.
    private fun loopCallKind(call: FirFunctionCall): String? {
        val symbol = call.calleeReference.toResolvedCallableSymbol() ?: return null
        val name = symbol.name.asString()
        val packageName = symbol.callableId?.packageName?.asString() ?: return null
        return when (name) {
            in lazyLoops -> name.takeIf { packageName.startsWith("androidx.") }
            in inlineLoops -> name.takeIf { packageName == "kotlin" || packageName.startsWith("kotlin.") || forEachMember(name, packageName) }
            else -> null
        }
    }

    // `Iterable.forEach` and `Map.forEach` resolve to the Java members when no Kotlin extension wins.
    private fun forEachMember(name: String, packageName: String): Boolean =
        name == "forEach" && (packageName == "java.lang" || packageName == "java.util")

    // The name of a lazy list builder call (`item`, `items`, `itemsIndexed`).
    private fun builderName(call: FirFunctionCall): String? {
        val symbol = call.calleeReference.toResolvedCallableSymbol() ?: return null
        val name = symbol.name.asString()
        return name.takeIf { it in lazyBuilders && symbol.callableId?.packageName?.asString()?.startsWith("androidx.") == true }
    }

    // True when the loop call holding the lambda sits below an if or when inside the loop, so the call runs for
    // some iterations only.
    private fun guardedInsideLoop(ancestors: List<FirElement>): Boolean {
        var guarded = false
        for (element in ancestors) {
            when {
                element is FirWhenExpression -> guarded = true
                isForLoop(element) -> return guarded
                element is FirFunctionCall && loopCallKind(element) != null -> return guarded
            }
        }
        return false
    }

    // True when an argument reads a parameter or local declared inside an enclosing loop before the call: the
    // value changes per iteration, so the painter cannot be hoisted.
    private fun readsPerIterationValue(call: FirFunctionCall, loops: List<FirElement>, callStart: Int): Boolean {
        val bound = HashSet<Any>()
        for (loop in loops) {
            loop.accept(object : FirVisitorVoid() {
                override fun visitElement(element: FirElement) {
                    val startsBefore = (element.source?.startOffset ?: Int.MAX_VALUE) < callStart
                    if (startsBefore && element is FirValueParameter) bound += element.symbol
                    if (startsBefore && element is FirProperty && element.isLocal) bound += element.symbol
                    element.acceptChildren(this)
                }
            })
        }
        var reads = false
        call.argumentList.arguments.forEach { argument ->
            argument.accept(object : FirVisitorVoid() {
                override fun visitElement(element: FirElement) {
                    if (reads) return
                    if (element is FirQualifiedAccessExpression && element.calleeReference.toResolvedCallableSymbol()?.let { it in bound } == true) {
                        reads = true
                        return
                    }
                    element.acceptChildren(this)
                }
            })
        }
        return reads
    }
}
