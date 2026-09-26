package dev.jasonpearson.krit.fir.checkers.coroutines

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.significantChildren
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.descriptors.Modality
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds

// Flags a `for` loop over a `java.util.Collections.synchronized*` wrapper that
// is not inside a `synchronized(...)` call: the wrapper's iterator is not
// thread-safe, so iteration must hold the wrapper's lock.
//
// Mirrors the Go CollectionsSynchronizedListIteration rule, which reports a
// `for` statement (on its `for` line) whose text contains
// `Collections.synchronizedList`, `Collections.synchronizedSet`, or
// `Collections.synchronizedMap`, unless an enclosing call written
// `synchronized` (any lock, any owner) sits between the loop and the nearest
// named function. Lambdas, anonymous functions, and classes are not
// boundaries for that walk, and the checker walks the same light tree.
//
// The checker reads the loop's iterable instead of its text:
// - It reports when a call resolved to `java.util.Collections.synchronized*`
//   appears in the iterable expression, as Go's text match of the loop header
//   does, unless it sits under a call or property read that yields a scalar
//   (`0 until wrapper.size`, `filter { wrapper.contains(it) }`), where the
//   loop does not iterate the wrapper. That includes the forms Go's text match misses: a
//   static or aliased import (`synchronizedList(xs)`), an import alias of
//   `Collections`, and the other wrappers (`synchronizedCollection`,
//   `synchronizedSortedSet`, `synchronizedNavigableMap`, ...), which the
//   message's `Collections.synchronized*` names too.
// - It also reports a loop over a `val` initialized with such a call (member,
//   top-level, or local), iterated directly or through a lazy view or adapter
//   (`keys`, `values`, `entries`, `withIndex()`, `asSequence()`,
//   `asIterable()`, `iterator()`): that is the wrapper being iterated, which Go
//   cannot see from the loop's text.
// - It does not report a loop that only mentions the wrapper in its body, a
//   comment, or a string, or a lookalike `Collections` class: the loop does
//   not iterate a `java.util.Collections` wrapper there.
internal object CollectionsSynchronizedListIteration : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "CollectionsSynchronizedListIteration"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(CollectionsSynchronizedListIteration)
    }

    private const val MESSAGE =
        "Iterating over a Collections.synchronized* wrapper without external synchronization. The iterator is not thread-safe."

    private val collections = ClassId(FqName("java.util"), Name.identifier("Collections"))
    private val wrapperFactories = setOf(
        "synchronizedCollection",
        "synchronizedList",
        "synchronizedSet",
        "synchronizedSortedSet",
        "synchronizedNavigableSet",
        "synchronizedMap",
        "synchronizedSortedMap",
        "synchronizedNavigableMap",
    ).mapTo(HashSet(), Name::identifier)

    private val scalarTypes = setOf(
        StandardClassIds.Boolean,
        StandardClassIds.Char,
        StandardClassIds.Byte,
        StandardClassIds.Short,
        StandardClassIds.Int,
        StandardClassIds.Long,
        StandardClassIds.Float,
        StandardClassIds.Double,
        StandardClassIds.String,
        StandardClassIds.Unit,
    )

    private val iteratorName = Name.identifier("iterator")
    private val synchronizedName = "synchronized"

    // Stdlib members and extensions whose result iterates the receiver itself,
    // lazily, while the loop runs.
    private val viewProperties = setOf("keys", "values", "entries").mapTo(HashSet(), Name::identifier)
    private val adapterFunctions =
        setOf("withIndex", "asSequence", "asIterable", "iterator").mapTo(HashSet(), Name::identifier)
    // Java collection interfaces that redeclare these members (`SortedMap`,
    // `NavigableSet`) own them in java.util.
    private val stdlibPackages = setOf(FqName("kotlin.collections"), FqName("kotlin.sequences"), FqName("java.util"))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        // FIR rewrites `for (x in xs)` into `val it = xs.iterator()` and a
        // while loop; that iterator() call carries the loop's desugared source
        // and xs as its receiver.
        val source = expression.source ?: return
        if (source.kind != KtFakeSourceElementKind.DesugaredForLoop) return
        if (expression.calleeReference.name != iteratorName) return
        val iterable = expression.explicitReceiver ?: return

        if (!containsWrapperCall(iterable) && !iteratesWrapperValue(iterable)) return
        val loop = enclosingFor(source) ?: return
        if (insideSynchronizedCall(source, loop)) return
        // Go reports the `for` statement's first line; the keyword is on it.
        val keyword = lightChildren(source, loop).firstOrNull { it.tokenType == KtTokens.FOR_KEYWORD } ?: loop
        report(lightSourceOf(keyword, source), MESSAGE)
    }

    private fun isWrapperCall(element: FirElement): Boolean {
        val call = element as? FirFunctionCall ?: return false
        val symbol = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return false
        val callableId = symbol.callableId
        return callableId.classId == collections && callableId.callableName in wrapperFactories
    }

    // A wrapper call in the iterable, as Go's text match of the loop header
    // finds one, except under a call or property read that yields a scalar
    // (`0 until wrapper.size`, `filter { wrapper.contains(it) }`): the loop
    // does not iterate the wrapper through that value.
    context(context: CheckerContext)
    private fun containsWrapperCall(iterable: FirExpression): Boolean {
        var found = false
        iterable.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (isWrapperCall(element)) {
                    found = true
                    return
                }
                if (element is FirQualifiedAccessExpression && isScalar(element.resolvedType)) return
                element.acceptChildren(this)
            }
        })
        return found
    }

    context(context: CheckerContext)
    private fun isScalar(type: ConeKotlinType): Boolean {
        val bound = type.fullyExpandedType().lowerBoundIfFlexible() as? ConeClassLikeType ?: return false
        return bound.classId in scalarTypes
    }

    // The iterable is a `val` holding a wrapper, directly or through a view or
    // adapter that iterates it lazily.
    private fun iteratesWrapperValue(iterable: FirExpression): Boolean {
        var current = unwrapSmartCast(iterable)
        while (true) {
            val symbol = (current as? FirPropertyAccessExpression)?.calleeReference?.toResolvedCallableSymbol()
            val receiver = when {
                current is FirPropertyAccessExpression && symbol is FirPropertySymbol &&
                    symbol.name in viewProperties && isStdlib(symbol.callableId?.packageName) ->
                    current.explicitReceiver
                current is FirFunctionCall && isAdapterCall(current) -> current.explicitReceiver
                else -> return symbol is FirPropertySymbol && holdsWrapper(symbol)
            } ?: return false
            current = unwrapSmartCast(receiver)
        }
    }

    private fun isAdapterCall(call: FirFunctionCall): Boolean {
        if (call.argumentList.arguments.isNotEmpty()) return false
        val symbol = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return false
        return symbol.name in adapterFunctions && isStdlib(symbol.callableId.packageName)
    }

    private fun isStdlib(packageName: FqName?) = packageName in stdlibPackages

    // A read-only property whose value is the wrapper its initializer
    // creates: a `val` that is not open and has no explicit getter.
    private fun holdsWrapper(symbol: FirPropertySymbol): Boolean {
        if (symbol.isVar) return false
        if (symbol.resolvedStatus.modality == Modality.OPEN || symbol.resolvedStatus.modality == Modality.ABSTRACT) {
            return false
        }
        if (symbol.getterSymbol?.isDefault == false) return false
        val initializer = symbol.resolvedInitializer ?: return false
        return isWrapperCall(unwrapSmartCast(initializer))
    }

    private fun unwrapSmartCast(expression: FirExpression): FirExpression {
        var current = expression
        while (current is FirSmartCastExpression) current = current.originalExpression
        return current
    }

    // The `for` statement the desugared source belongs to: the source is the
    // loop's range expression (or the loop itself).
    private fun enclosingFor(source: KtSourceElement): LighterASTNode? {
        val tree = source.treeStructure
        var node: LighterASTNode? = source.lighterASTNode
        while (node != null) {
            if (node.tokenType == KtNodeTypes.FOR) return node
            node = tree.getParent(node)
        }
        return null
    }

    // Go's hasAncestorCallNamedFlat(for, "synchronized"): an enclosing call
    // whose callee is written `synchronized`, bare or after a qualifier, up to
    // the nearest named function.
    private fun insideSynchronizedCall(source: KtSourceElement, loop: LighterASTNode): Boolean {
        val tree = source.treeStructure
        var node = tree.getParent(loop)
        while (node != null) {
            when (node.tokenType) {
                KtNodeTypes.FUN -> if (significantChildren(source, node).any { it.tokenType == KtTokens.IDENTIFIER }) {
                    return false
                }
                KtNodeTypes.CALL_EXPRESSION -> {
                    val callee = significantChildren(source, node).firstOrNull()
                    if (callee?.tokenType == KtNodeTypes.REFERENCE_EXPRESSION &&
                        lightText(source, callee) == synchronizedName
                    ) {
                        return true
                    }
                }
            }
            node = tree.getParent(node)
        }
        return false
    }
}
