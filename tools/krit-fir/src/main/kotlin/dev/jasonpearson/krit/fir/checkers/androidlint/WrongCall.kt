package dev.jasonpearson.krit.fir.checkers.androidlint

import com.intellij.lang.LighterASTNode
import com.intellij.util.diff.FlyweightCapableTreeStructure
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.utils.isOverride
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirSuperReceiverExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.text
import org.jetbrains.kotlin.toKtLightSourceElement
import org.jetbrains.kotlin.util.getChildren

/**
 * Flags a direct call of a View's `onDraw`, `onMeasure`, or `onLayout` on an
 * explicit receiver (`child.onDraw(canvas)`), where the caller almost always
 * meant `draw`, `measure`, or `layout`.
 *
 * Like the Go rule, the call must have an explicit receiver (a bare
 * `onDraw(canvas)` is not reported), a plain `super` receiver is allowed, and a
 * call anywhere inside an `override` function (the nearest enclosing named
 * function, looking through lambdas, anonymous functions, and local classes)
 * is not reported. The finding sits on the first line of the call expression
 * (the receiver's first line), with Go's message.
 *
 * The View proof comes from the resolved callee: it must be a member function
 * (not an extension) of `android.view.View` or of a subtype of it, including
 * local and anonymous subclasses. K2 rejects most of the shapes the Go rule
 * targets, because the three callbacks are protected: `child.onDraw(canvas)`
 * on a `View`-typed child from inside a View subclass is
 * `INVISIBLE_REFERENCE`, so that file is not authoritative and Go keeps its
 * findings there. The shapes that compile are a receiver whose type is the
 * calling View subclass itself (`this.onDraw(c)`, `other.onDraw(c)` with
 * `other: MyView`), and a subclass that widens a callback to `public`.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Precision: Go proves the receiver is a View by the simple name `View` or
 *   its source hierarchy, and when it cannot type the receiver it falls back to
 *   "the enclosing class extends View". This checker does not report a project
 *   class named `View` (not android.view.View), or a non-View receiver that
 *   Go cannot type (`x!!`, `x.let { it }`, `run { x }`, `map[k]?.`) inside a
 *   View subclass: neither calls a View callback.
 * - Recall: resolution sees receivers Go misses: `this@Outer` from an inner
 *   class (Go's fallback reads only the nearest class), an object expression
 *   extending View (Go's fallback needs a class or object declaration), a View
 *   receiver whose chain starts with a name Go treats as a known non-View
 *   builder root (`Tab`, `Preference`, ...), and receivers Go cannot type
 *   outside a View subclass (a local inferred from a generic call, a type
 *   parameter bounded by a View subclass).
 */
internal object WrongCall : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "WrongCall"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(WrongCall)
    }

    private val callbacks = setOf(
        Name.identifier("onDraw"),
        Name.identifier("onMeasure"),
        Name.identifier("onLayout"),
    )
    private val view = ClassId(FqName("android.view"), Name.identifier("View"))
    private val qualifiedTypes = setOf(KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return
        if (callee.name !in callbacks) return
        val receiver = expression.explicitReceiver ?: return
        // Go skips only the receiver spelled `super`; a qualified
        // `super<View>` or labeled `super@Outer` is still checked.
        if (receiver is FirSuperReceiverExpression && receiver.source?.text?.toString() == "super") return
        if (callee.receiverParameterSymbol != null) return
        val owner = callee.getContainingClassSymbol() ?: return
        if (!isView(owner)) return
        val enclosing = context.containingDeclarations.filterIsInstance<FirNamedFunctionSymbol>().lastOrNull()
        if (enclosing != null && enclosing.isOverride) return
        val source = expression.source ?: return
        report(
            qualifiedCall(source)?.let { sourceOf(it, source) } ?: source,
            "Suspicious method call; should probably call draw/measure/layout instead of ${callee.name}.",
        )
    }

    // The containing class comes from the symbol's lookup tag, which is bound to
    // local and anonymous classes; only class ids already in hand are compared.
    context(context: CheckerContext)
    private fun isView(symbol: FirClassLikeSymbol<*>): Boolean =
        symbol.classId == view ||
            lookupSuperTypes(symbol, lookupInterfaces = false, deep = true, useSiteSession = context.session)
                .any { it.lookupTag.classId == view }

    // The qualified expression (`r.f()` / `r?.f()`) whose selector is this
    // call. K2 gives a dot call the whole qualified expression as its source;
    // a safe call keeps the selector call expression, so step up to its parent.
    private fun qualifiedCall(source: KtSourceElement): LighterASTNode? {
        val tree = source.treeStructure
        val node = source.lighterASTNode
        if (node.tokenType in qualifiedTypes) return node
        if (node.tokenType != KtNodeTypes.CALL_EXPRESSION) return null
        val parent = tree.getParent(node) ?: return null
        if (parent.tokenType !in qualifiedTypes) return null
        val parts = significantChildren(parent, tree)
        return parent.takeIf { parts.size > 1 && parts.last() == node }
    }

    // A source element for [node], a node in [anchor]'s tree, keeping the
    // anchor's offset shift between tree offsets and file offsets.
    private fun sourceOf(node: LighterASTNode, anchor: KtSourceElement): KtSourceElement {
        if (node == anchor.lighterASTNode) return anchor
        val shift = anchor.startOffset - anchor.lighterASTNode.startOffset
        return node.toKtLightSourceElement(
            anchor.treeStructure,
            startOffset = node.startOffset + shift,
            endOffset = node.endOffset + shift,
        )
    }

    private fun significantChildren(
        node: LighterASTNode,
        tree: FlyweightCapableTreeStructure<LighterASTNode>,
    ): List<LighterASTNode> =
        node.getChildren(tree).filter {
            it.tokenType != KtTokens.WHITE_SPACE &&
                it.tokenType !in KtTokens.COMMENTS &&
                it.tokenType != KtTokens.DOT &&
                it.tokenType != KtTokens.SAFE_ACCESS
        }
}
