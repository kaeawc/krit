package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.isOrInheritsFrom
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirSuperReceiverExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirAnonymousFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirAnonymousInitializerSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertyAccessorSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirValueParameterSymbol
import org.jetbrains.kotlin.name.Name

/**
 * Flags a call of a View's `onDraw`, `onMeasure`, or `onLayout`, where the
 * caller almost always meant `draw`, `measure`, or `layout`, as Android
 * Lint's WrongCall does.
 *
 * The callee must be a member function (not an extension) of
 * `android.view.View` or of a subtype of it, including local and anonymous
 * subclasses, called on any receiver: explicit (`this.onDraw(c)`,
 * `other.onDraw(c)`), implicit (`onDraw(c)` in a View subclass or in
 * `with(view) { ... }`), or `super`. A `super` call is reported unless the
 * nearest enclosing method has the callback's name (`super.onDraw(c)` in
 * `onDraw` is the normal chain-up; in `onMeasure` it is not). As in Lint,
 * lambdas and local functions are not methods, an init block belongs to the
 * constructor, and a super call in a property initializer, which has no
 * method, is not reported. The finding sits on the callee name, with Lint's
 * message.
 *
 * Like Lint, which does not analyze test sources, it skips test files.
 *
 * K2 rejects `child.onDraw(canvas)` on a `View`-typed child from inside a
 * View subclass (the callbacks are protected, `INVISIBLE_REFERENCE`), so that
 * file is not authoritative and Go keeps its findings there.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Precision: Go proves the receiver is a View by the simple name `View` or
 *   its source hierarchy, and when it cannot type the receiver it falls back to
 *   "the enclosing class extends View". It also matches the callee by name
 *   alone. This checker does not report a project class named `View` (not
 *   android.view.View), a non-View receiver that Go cannot type (`x!!`,
 *   `x.let { it }`, `run { x }`, `map[k]?.`) inside a View subclass, or a
 *   View receiver whose call resolves to something other than a View member:
 *   a project extension named `onDraw` (including one with the callback's
 *   exact signature, which K2 picks over the invisible protected member), a
 *   function-typed property named like a callback, or an interface method a
 *   View subclass inherits. None of them calls a View callback.
 * - Recall: resolution sees receivers Go misses: `this@Outer` and
 *   `super@Outer` from an inner class (Go's fallback reads only the nearest
 *   class), an object expression extending View (Go's fallback needs a class
 *   or object declaration), a View receiver whose chain starts with a name Go
 *   treats as a known non-View builder root (`Tab`, `Preference`, ...), an
 *   implicit receiver inside a lambda (Go skips a bare call in a lambda, which
 *   may have a receiver of its own), and receivers Go cannot type outside a
 *   View subclass (a local inferred from a generic call or holding an object
 *   expression, a type parameter with a View-subclass or intersection bound,
 *   a scope-function `this` / `it`, a cast, a parenthesized receiver split
 *   over lines, a `?.let { it }` chain).
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
    private val view = classIdOf("android.view", "View")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return
        if (callee.name !in callbacks) return
        // Lint does not analyze test sources.
        if (isInTestFile()) return
        if (callee.receiverParameterSymbol != null) return
        val owner = callee.getContainingClassSymbol() ?: return
        if (!isView(owner)) return
        if (expression.explicitReceiver is FirSuperReceiverExpression && !superCallReported(callee.name)) return
        val source = expression.calleeReference.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        val name = callee.name.asString()
        val intended = name[2].lowercaseChar() + name.substring(3)
        report(source, "Suspicious method call; should probably call \"`$intended`\" rather than \"`$name`\"")
    }

    // Lint reports a super call unless its nearest enclosing method has the
    // callback's name; lambdas, local functions, local variables, and
    // parameter defaults sit inside the method they belong to.
    context(context: CheckerContext)
    private fun superCallReported(name: Name): Boolean {
        for (symbol in context.containingDeclarations.asReversed()) {
            when (symbol) {
                is FirAnonymousFunctionSymbol, is FirValueParameterSymbol -> continue
                is FirNamedFunctionSymbol ->
                    if (symbol.rawStatus.visibility == Visibilities.Local) continue else return symbol.name != name
                is FirConstructorSymbol, is FirPropertyAccessorSymbol, is FirAnonymousInitializerSymbol -> return true
                is FirPropertySymbol -> if (symbol.isLocal) continue else return false
                else -> return false
            }
        }
        return false
    }

    // The containing class comes from the symbol's lookup tag, which is bound to
    // local and anonymous classes; only class ids already in hand are compared.
    context(context: CheckerContext)
    private fun isView(symbol: FirClassLikeSymbol<*>): Boolean =
        isOrInheritsFrom(symbol, view, context.session, lookupInterfaces = false)
}
