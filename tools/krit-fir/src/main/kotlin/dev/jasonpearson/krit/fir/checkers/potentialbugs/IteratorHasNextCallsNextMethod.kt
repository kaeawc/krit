package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirDeclarationChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirImplicitInvokeCall
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Port of the Go IteratorHasNextCallsNextMethod rule: an iterator's
 * `hasNext()` whose body calls `next()`, reported once per function on its
 * first line (its modifier list, else `fun`), like Go.
 *
 * The function is the `hasNext()` a class, object, interface, enum entry,
 * local class, or anonymous object declares directly as a member, with no
 * value parameters and no receiver, and with a body, where that declaring
 * class is an iterator: `kotlin.collections.Iterator` or `java.util.Iterator`
 * is in its supertype closure (so `MutableIterator`, `ListIterator`,
 * `MutableListIterator`, `IntIterator`, user interfaces and classes extending
 * them, type aliases and import aliases of them all count). This is the same
 * iterator test `IteratorNotThrowingNoSuchElementException` makes for `next()`.
 *
 * The body calls `next()`, like Go, when any call anywhere in it (lambdas,
 * local functions, and local classes included, as Go walks the whole body) is
 * named `next`: a call that resolves to a function named `next`, whatever
 * declares it, or the invocation of a function-typed value named `next`. This
 * matches Go on purpose instead of requiring an iterator owner: a cursor's
 * `next()` that is not an `Iterator` member (`java.sql.ResultSet.next()`, a
 * reader's or tokenizer's `next()`) advances the state the iterator reads just
 * as `items.next()` does, and resolution cannot tell it from a linked-list
 * node's side-effect-free `next()`.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`IteratorHasNextCallsNextMethod*.kt`) or in
 * `IteratorHasNextCallsNextMethodTest`:
 * - Go takes any function named `hasNext` whose nearest enclosing class
 *   declaration mentions `Iterator`, `MutableIterator`, or `ListIterator`
 *   anywhere in a supertype list in its body. So it reports a local function
 *   named `hasNext`, a `hasNext` overload with parameters or a receiver, a
 *   companion's `hasNext`, a member of an anonymous object nested in an
 *   iterator, the `hasNext` of a class that merely contains an iterator class,
 *   and the `hasNext` of a class whose supertype only mentions `Iterator` as a
 *   type argument (`Comparable<Iterator<Int>>`). None of those is an
 *   iterator's `hasNext()`, so none is reported here.
 * - Go cannot see declarations in other files, so it reports an iterator of a
 *   same-package `Iterator` lookalike declared in another file (no finding
 *   here: it is not an iterator).
 * - Resolution sees iterators Go misses: an anonymous object outside any class
 *   (in a top-level function or property; Go only looks through class and
 *   object declarations), `MutableListIterator`, `IntIterator`, a type alias,
 *   an import alias, and a user interface, abstract class, or open class
 *   extending `Iterator`.
 * - Go reads no call name through parentheses, so it misses `(next)()`, the
 *   same invocation of a function-typed value named `next` it reports as
 *   `next()`.
 */
internal object IteratorHasNextCallsNextMethod :
    FirDeclarationChecker<FirNamedFunction>(MppCheckerKind.Common), FirRule {
    override val ruleId = "IteratorHasNextCallsNextMethod"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val simpleFunctionCheckers = setOf(IteratorHasNextCallsNextMethod)
    }

    private val hasNext = Name.identifier("hasNext")
    private val next = Name.identifier("next")
    private val iteratorClassIds = setOf(
        ClassId(FqName("kotlin.collections"), Name.identifier("Iterator")),
        ClassId(FqName("java.util"), Name.identifier("Iterator")),
    )

    private const val MESSAGE = "hasNext() should not call next(). This modifies the iterator state."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirNamedFunction) {
        if (declaration.name != hasNext) return
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        if (declaration.valueParameters.isNotEmpty() || declaration.receiverParameter != null) return
        if (declaration.contextParameters.isNotEmpty()) return
        val body = declaration.body ?: return
        // The declaring class must be the nearest container: a local function
        // named hasNext, even inside an iterator, is not the iterator's hasNext().
        val owner = context.containingDeclarations.lastOrNull { it != declaration.symbol } as? FirClassSymbol<*> ?: return
        if (!isIterator(owner, context.session)) return
        if (!callsNext(body)) return
        report(source, MESSAGE)
    }

    // The class itself or its supertypes, read from the class's own lookup
    // tags, which are bound to local and anonymous classes, so no class id is
    // resolved from a symbol that may be local.
    private fun isIterator(symbol: FirClassSymbol<*>, session: FirSession): Boolean =
        symbol.classId in iteratorClassIds ||
            lookupSuperTypes(symbol, lookupInterfaces = true, deep = true, useSiteSession = session)
                .any { it.classId in iteratorClassIds }

    // Walks the whole body, like Go, into lambdas, local functions, and local
    // classes, looking for a call named next: a function named next, whatever
    // declares it, or the invocation of a function-typed value named next.
    private fun callsNext(body: FirElement): Boolean {
        var found = false
        body.accept(object : FirVisitorVoid() {
            // Checked here rather than in visitFunctionCall: the generated
            // visitors send an implicit invoke call (`next()` on a
            // function-typed value) to visitElement, not visitFunctionCall.
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirFunctionCall && isNextCall(element)) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    private fun isNextCall(call: FirFunctionCall): Boolean {
        if (call is FirImplicitInvokeCall) {
            val value = call.explicitReceiver as? FirQualifiedAccessExpression ?: return false
            return value.calleeReference.toResolvedCallableSymbol()?.name == next
        }
        return (call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol)?.name == next
    }
}
