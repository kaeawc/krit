package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtFakeSourceElementKind
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
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.references.FirNamedReference
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.CallableId
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
 * written `next`: a function called by the name `next`, whatever declares it
 * (so the constructor of a class named `next` and a function imported under
 * the alias `next` count, and a `next` imported under another name does not),
 * or the invocation of a value, callable reference, or object named `next`.
 * This matches Go on purpose instead of requiring an iterator owner: a
 * cursor's `next()` that is not an `Iterator` member
 * (`java.sql.ResultSet.next()`, a reader's or tokenizer's `next()`) advances
 * the state the iterator reads just as `items.next()` does, and resolution
 * cannot tell it from a linked-list node's side-effect-free `next()`.
 *
 * FIR rewrites every `for (x in xs)` into `iterator()`, `hasNext()`, and
 * `next()` calls. That generated `next()` is not a call in the source and
 * advances the loop's own fresh iterator, so it does not count (Go sees no
 * call either). The one exception is a loop over an iterator held in `this`
 * or a value (`for (x in this)`, `for (x in items)`): `Iterator<T>.iterator()`
 * returns that iterator, so the loop really does call its `next()`.
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
 *   type argument (`Comparable<Iterator<Int>>`), and a `hasNext()` with a
 *   context parameter. None of those is an iterator's `hasNext()`, so none is
 *   reported here.
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
 *   `next()`, and `(items::next)()`, which calls `items.next()`.
 * - Go only reads `call_expression`s, so it misses the infix call
 *   `stride next 1`, the same call as `stride.next(1)`, which it reports.
 * - Go sees no call in a for-loop over an iterator (`for (x in this)`,
 *   `for (x in items)`), which calls that iterator's `next()`.
 */
internal object IteratorHasNextCallsNextMethod :
    FirDeclarationChecker<FirNamedFunction>(MppCheckerKind.Common), FirRule {
    override val ruleId = "IteratorHasNextCallsNextMethod"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val simpleFunctionCheckers = setOf(IteratorHasNextCallsNextMethod)
    }

    private val hasNext = Name.identifier("hasNext")
    private val next = Name.identifier("next")
    private val kotlinIterator = ClassId(FqName("kotlin.collections"), Name.identifier("Iterator"))
    private val iteratorClassIds = setOf(
        kotlinIterator,
        ClassId(FqName("java.util"), Name.identifier("Iterator")),
    )
    private val iteratorOperator = CallableId(FqName("kotlin.collections"), Name.identifier("iterator"))

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
    // classes, looking for a call written next: a function called as next,
    // whatever declares it, or the invocation of a value named next.
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

    // Go reads the call's name as written, so this does too: the callee
    // reference keeps the written name, which is `next` for a function
    // imported under the alias next and for the constructor of a class named
    // next, and not `next` for a next() imported under another name.
    private fun isNextCall(call: FirFunctionCall): Boolean {
        // FIR rewrites `for (x in xs)` into iterator(), hasNext(), and next()
        // calls on a fresh local iterator. That next() is not in the source and
        // advances the loop's own iterator, so it does not count, unless the
        // loop runs over an existing iterator: then iterator() returns that
        // iterator and the loop calls its next().
        if (call.source?.kind == KtFakeSourceElementKind.DesugaredForLoop) return advancesHeldIterator(call)
        if (call is FirImplicitInvokeCall) {
            return when (val value = call.explicitReceiver) {
                // A function-typed value or a callable reference named next.
                is FirQualifiedAccessExpression -> (value.calleeReference as? FirNamedReference)?.name == next
                // An object named next, invoked through a qualified name.
                is FirResolvedQualifier -> value.relativeClassFqName?.shortName() == next
                else -> false
            }
        }
        return call.calleeReference.name == next
    }

    // The iterator() call of a for-loop over an iterator held in `this` or in a
    // value (`for (x in this)`, `for (x in items)`): the stdlib
    // `operator fun <T> Iterator<T>.iterator() = this`, which hands the loop
    // that iterator, so the loop calls its next(). A loop over an iterator a
    // call returns (`for (x in list.iterator())`) advances that fresh
    // iterator instead, like a loop over the list, and Go reports neither.
    private fun advancesHeldIterator(call: FirFunctionCall): Boolean {
        val symbol = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return false
        if (symbol.callableId != iteratorOperator || symbol.resolvedReceiverType?.classId != kotlinIterator) return false
        var subject = call.explicitReceiver
        while (subject is FirSmartCastExpression) subject = subject.originalExpression
        return subject is FirThisReceiverExpression || subject is FirPropertyAccessExpression
    }
}
