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
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirThrowExpression
import org.jetbrains.kotlin.fir.expressions.FirTryExpression
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeDefinitelyNotNullType
import org.jetbrains.kotlin.fir.types.ConeIntersectionType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.ConeTypeParameterType
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Port of the Go IteratorNotThrowingNoSuchElementException rule: an iterator's
 * `next()` whose body never throws `NoSuchElementException`, reported on the
 * function's first line (its modifier list, else `fun`), like Go.
 *
 * The function is the `next()` a class, object, interface, enum entry, local
 * class, or anonymous object declares directly as a member, with no value
 * parameters and no receiver, and with a body, where that declaring class is
 * an iterator: `kotlin.collections.Iterator` or `java.util.Iterator` is in its
 * supertype closure (so `MutableIterator`, `ListIterator`,
 * `MutableListIterator`, user interfaces and abstract classes extending them,
 * type aliases and import aliases of them all count).
 *
 * The body satisfies the contract, like Go, when any `throw` anywhere in it
 * (lambdas, local functions, and local classes included, as Go walks the whole
 * body) throws `NoSuchElementException` (`kotlin.NoSuchElementException` is a
 * type alias of `java.util.NoSuchElementException`): the type of the thrown
 * expression, or of a value it can evaluate to (an `if`/`when` branch, either
 * side of an elvis, a `try` or `catch` block), is it or a subclass of it
 * (named, local, or anonymous), or the thrown expression contains a call to
 * its constructor (`throw if (done) NoSuchElementException() else ...`).
 * Delegating to another iterator's `next()` or to a helper that throws is not a
 * throw in the body, so Go and this checker both report it.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`IteratorNotThrowingNoSuchElementException*.kt`) or in
 * `IteratorNotThrowingNoSuchElementExceptionTest`:
 * - Go takes any function named `next` whose nearest enclosing class
 *   declaration mentions `Iterator`, `MutableIterator`, or `ListIterator`
 *   anywhere in a supertype list in its body. So it reports a local function
 *   named `next`, a `next` overload with parameters or a receiver, a
 *   companion's `next`, a member of an anonymous object nested in an
 *   iterator, the `next` of a class that merely contains an iterator class,
 *   and the `next` of a class whose supertype only mentions `Iterator` as a
 *   type argument (`Comparable<Iterator<Int>>`). None of those is an
 *   iterator's `next()`, so none is reported here.
 * - Go reads the thrown exception by the name of the call inside `throw`, so
 *   it reports `throw e` where `e` is a `NoSuchElementException`, a throw of a
 *   named, local, or anonymous subclass of it (also from one branch of an
 *   `if`), a call through an import alias of it, a call to a same-file factory
 *   function that returns one, and a reflective `newInstance()` of it. Go also
 *   rejects every call named `NoSuchElementException` in a file that declares
 *   anything with that name (a nested class, a factory function, a lookalike
 *   class), even when the call constructs the real exception. All of them
 *   throw a `NoSuchElementException`, so none is reported here.
 * - Go cannot see declarations in other files, so it reports an iterator of a
 *   same-package `Iterator` lookalike declared in another file (no finding
 *   here: it has no `NoSuchElementException` contract), and accepts a throw of
 *   a `NoSuchElementException` lookalike declared in another file of the same
 *   package or imported from another package, explicitly or with a star import
 *   (reported here: it is not `java.util.NoSuchElementException`).
 * - Resolution sees iterators Go misses: an anonymous object outside any class
 *   (in a top-level function or property; Go only looks through class and
 *   object declarations), `MutableListIterator`, a type alias, an import
 *   alias, a user interface, abstract class, or open class extending
 *   `Iterator`, and a qualified `kotlin.collections.Iterator` in a file that
 *   declares its own `Iterator`.
 */
internal object IteratorNotThrowingNoSuchElementException :
    FirDeclarationChecker<FirNamedFunction>(MppCheckerKind.Common), FirRule {
    override val ruleId = "IteratorNotThrowingNoSuchElementException"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val simpleFunctionCheckers = setOf(IteratorNotThrowingNoSuchElementException)
    }

    private val next = Name.identifier("next")
    private val iteratorClassIds = setOf(
        ClassId(FqName("kotlin.collections"), Name.identifier("Iterator")),
        ClassId(FqName("java.util"), Name.identifier("Iterator")),
    )
    private val noSuchElementException = ClassId(FqName("java.util"), Name.identifier("NoSuchElementException"))

    private const val MESSAGE =
        "Iterator's next() method should throw NoSuchElementException when there are no more elements."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirNamedFunction) {
        if (declaration.name != next) return
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        if (declaration.valueParameters.isNotEmpty() || declaration.receiverParameter != null) return
        if (declaration.contextParameters.isNotEmpty()) return
        val body = declaration.body ?: return
        // The declaring class must be the nearest container: a local function
        // named next, even inside an iterator, is not the iterator's next().
        val owner = context.containingDeclarations.lastOrNull { it != declaration.symbol } as? FirClassSymbol<*> ?: return
        if (!isIterator(owner, context.session)) return
        if (throwsNoSuchElementException(body, context.session)) return
        report(source, MESSAGE)
    }

    // Supertypes come from the class's own lookup tags, which are bound to
    // local and anonymous classes, so no class id is resolved from a symbol
    // that may be local.
    private fun isIterator(owner: FirClassSymbol<*>, session: FirSession): Boolean =
        lookupSuperTypes(owner, lookupInterfaces = true, deep = true, useSiteSession = session)
            .any { it.classId in iteratorClassIds }

    private fun throwsNoSuchElementException(body: FirElement, session: FirSession): Boolean {
        val finder = ThrowFinder(session)
        body.accept(finder)
        return finder.found
    }

    // Walks the whole body, like Go, into lambdas, local functions, and local
    // classes, looking for a throw of NoSuchElementException.
    private class ThrowFinder(private val session: FirSession) : FirVisitorVoid() {
        var found = false

        override fun visitElement(element: FirElement) {
            if (!found) element.acceptChildren(this)
        }

        override fun visitThrowExpression(throwExpression: FirThrowExpression) {
            if (found) return
            val exception = throwExpression.exception
            val results = mutableListOf<FirExpression>()
            collectResults(exception, results, depth = 0)
            if (results.any { isNoSuchElementType(it.resolvedType, session, subtypes = true, depth = 0) } ||
                constructsNoSuchElementException(exception)
            ) {
                found = true
                return
            }
            throwExpression.acceptChildren(this)
        }

        // The thrown expression and every value it can evaluate to: the
        // branches of `if`/`when`, both sides of an elvis, and the `try` and
        // `catch` blocks, so `throw if (strict) MissingElement() else ...`
        // counts a subclass thrown from one branch.
        private fun collectResults(expression: FirExpression, into: MutableList<FirExpression>, depth: Int) {
            into += expression
            if (depth > MAX_TYPE_DEPTH) return
            when (expression) {
                is FirWhenExpression -> expression.branches.forEach { collectResults(it.result, into, depth + 1) }
                is FirElvisExpression -> {
                    collectResults(expression.lhs, into, depth + 1)
                    collectResults(expression.rhs, into, depth + 1)
                }
                is FirTryExpression -> {
                    collectResults(expression.tryBlock, into, depth + 1)
                    expression.catches.forEach { collectResults(it.block, into, depth + 1) }
                }
                is FirBlock -> (expression.statements.lastOrNull() as? FirExpression)?.let {
                    collectResults(it, into, depth + 1)
                }
                else -> Unit
            }
        }

        // Go counts a throw whose thrown expression contains a call to
        // NoSuchElementException's constructor, wherever it sits in it.
        private fun constructsNoSuchElementException(exception: FirExpression): Boolean {
            var constructs = false
            exception.accept(object : FirVisitorVoid() {
                override fun visitElement(element: FirElement) {
                    if (!constructs) element.acceptChildren(this)
                }

                override fun visitFunctionCall(functionCall: FirFunctionCall) {
                    if (constructs) return
                    if (functionCall.calleeReference.toResolvedCallableSymbol() is FirConstructorSymbol &&
                        isNoSuchElementType(functionCall.resolvedType, session, subtypes = false, depth = 0)
                    ) {
                        constructs = true
                        return
                    }
                    functionCall.acceptChildren(this)
                }
            })
            return constructs
        }
    }

    private fun isNoSuchElementType(type: ConeKotlinType, session: FirSession, subtypes: Boolean, depth: Int): Boolean {
        if (depth > MAX_TYPE_DEPTH) return false
        return when (val bound = type.fullyExpandedType(session).lowerBoundIfFlexible()) {
            is ConeDefinitelyNotNullType -> isNoSuchElementType(bound.original, session, subtypes, depth + 1)
            is ConeIntersectionType -> bound.intersectedTypes.any { isNoSuchElementType(it, session, subtypes, depth + 1) }
            is ConeTypeParameterType -> subtypes && bound.lookupTag.typeParameterSymbol.resolvedBounds.any {
                isNoSuchElementType(it.coneType, session, subtypes, depth + 1)
            }
            // The lookup tag of a local class or an anonymous object
            // (`throw object : NoSuchElementException() {}`) is bound to its
            // symbol, so no class id is resolved from a symbol that may be local.
            is ConeClassLikeType -> bound.classId == noSuchElementException || (
                subtypes && bound.lookupTag.toClassSymbol(session)?.let { symbol ->
                    lookupSuperTypes(symbol, lookupInterfaces = false, deep = true, useSiteSession = session)
                        .any { it.classId == noSuchElementException }
                } == true
                )
            else -> false
        }
    }

    private const val MAX_TYPE_DEPTH = 16
}
