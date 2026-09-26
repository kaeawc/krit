package dev.jasonpearson.krit.fir.checkers.coroutines

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.descriptors.Modality
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirPropertyAccessor
import org.jetbrains.kotlin.fir.declarations.FirVariable
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirCallableReferenceAccess
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirWhileLoop
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds

// Flags a `for` loop over a `java.util.Collections.synchronized*` wrapper that
// is not protected by the wrapper's own monitor: its iterator is not
// thread-safe, so iteration must hold that monitor.
//
// Mirrors the Go CollectionsSynchronizedListIteration rule, which reports a
// `for` statement (on its `for` line) whose text contains
// `Collections.synchronizedList`, `Collections.synchronizedSet`, or
// `Collections.synchronizedMap`. An arbitrary enclosing synchronized call
// does not establish that the wrapper's monitor is held.
//
// The checker reads what the loop's code does with the wrapper instead of its
// text:
// - It reports when a call resolved to `java.util.Collections.synchronized*`
//   appears in the iterable expression, as Go's text match of the loop header
//   does, unless the wrapper there is only the receiver of an operation that
//   uses no iterator of it: a member java.util.Collections runs under the
//   wrapper's own lock (`size`, `contains`, `get`, `toArray`, `toString`) or a
//   stdlib call built only on such members (`first()` of a List, `indices`,
//   `getValue`, `toList`). Anything else (`joinToString`, `count { }`, `sum`,
//   `forEach`, ...) walks the wrapper's iterator and is reported. That
//   includes the forms Go's text match misses: a static or aliased import
//   (`synchronizedList(xs)`), an import alias of `Collections`, a qualifier
//   split by a line break or comment, and the other wrappers
//   (`synchronizedCollection`, `synchronizedNavigableMap`, ...), which the
//   message's `Collections.synchronized*` names too.
// - It reports a loop whose body iterates such a call (directly, through a
//   view, or through a local declared in the body) outside a synchronized
//   call, as Go's match of the whole statement does, but not a body that only
//   creates, passes, or reads the wrapper under its lock, and not an outer
//   loop whose inner loop reports the wrapper itself.
// - It also reports a loop over a `val` initialized with such a call (member,
//   top-level, or local), iterated directly or through a lazy view or adapter
//   (`keys`, `values`, `entries`, `withIndex()`, `asSequence()`,
//   `iterator()`): that is the wrapper being iterated, which Go cannot see
//   from the loop's text. That loop is left alone when the code around it
//   holds the wrapper's monitor, directly or through a private function
//   whose every caller in the file acquires that monitor.
// - It does not report a lookalike `Collections` class: the loop does not
//   iterate a `java.util.Collections` wrapper there.
internal object CollectionsSynchronizedListIteration : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "CollectionsSynchronizedListIteration"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(CollectionsSynchronizedListIteration)
    }

    private const val MESSAGE =
        "Iterating over a Collections.synchronized* wrapper without external synchronization. The iterator is not thread-safe."

    private val collections = ClassId(FqName("java.util"), Name.identifier("Collections"))
    private val lazyId = CallableId(FqName("kotlin"), Name.identifier("lazy"))
    private val wrapperFactories = names(
        "synchronizedCollection",
        "synchronizedList",
        "synchronizedSet",
        "synchronizedSortedSet",
        "synchronizedNavigableSet",
        "synchronizedMap",
        "synchronizedSortedMap",
        "synchronizedNavigableMap",
    )

    private val iteratorName = Name.identifier("iterator")
    private const val SYNCHRONIZED = "synchronized"

    private val kotlinPackage = FqName("kotlin")
    // Packages whose collection members a wrapper inherits.
    private val memberPackages = setOf(FqName("kotlin.collections"), FqName("java.util"), FqName("java.lang"))
    private val extensionPackages = setOf(FqName("kotlin.collections"), FqName("kotlin.sequences"))

    // Members of the wrapper that java.util.Collections does not run under
    // its lock: they hand out an iterator or a stream of the wrapper.
    private val iteratorMembers =
        names("iterator", "listIterator", "descendingIterator", "spliterator", "stream", "parallelStream")
    // Members that return a view synchronized on the wrapper's lock, which
    // iterates the wrapper's backing collection.
    private val viewMembers = names(
        "keys", "values", "entries", "keySet", "entrySet", "subList", "headSet", "tailSet", "subSet",
        "headMap", "tailMap", "subMap", "descendingSet", "descendingMap", "navigableKeySet",
        "descendingKeySet", "reversed",
    )
    // Stdlib extensions that return their receiver.
    private val identityFunctions = names("also", "apply", "takeIf", "takeUnless")
    private val identityCollectionFunctions = names("orEmpty")
    private val toStringName = Name.identifier("toString")
    // Stdlib adapters whose result iterates the receiver lazily.
    private val adapterFunctions = names("withIndex", "asSequence", "asIterable", "asReversed")

    private val iterable = StandardClassIds.Iterable
    private val collection = StandardClassIds.Collection
    private val list = StandardClassIds.List
    private val mutableList = StandardClassIds.MutableList
    private val map = StandardClassIds.Map
    private val mutableCollection = StandardClassIds.MutableCollection
    private val mutableMap = StandardClassIds.MutableMap

    // Stdlib extensions that only call members java.util.Collections runs
    // under the wrapper's lock (`size`, `get`, `containsKey`, `toArray`,
    // `add`, `put`), keyed to the receiver types where that holds.
    private val guardedExtensions: Map<Name, Set<ClassId>> = mapOf(
        "isNotEmpty" to setOf(collection, map),
        "isNullOrEmpty" to setOf(collection, map),
        "indices" to setOf(collection),
        "lastIndex" to setOf(list),
        "getValue" to setOf(map),
        "getOrElse" to setOf(list, map),
        "getOrNull" to setOf(list),
        "getOrPut" to setOf(mutableMap),
        "set" to setOf(mutableMap),
        "contains" to setOf(iterable, collection, map),
        "containsKey" to setOf(map),
        "containsValue" to setOf(map),
        "toTypedArray" to setOf(collection),
        "toList" to setOf(iterable, collection, list),
        "sorted" to setOf(iterable),
        "sortedDescending" to setOf(iterable),
        "sortedWith" to setOf(iterable),
        "sortedBy" to setOf(iterable),
        "sortedByDescending" to setOf(iterable),
        "reversed" to setOf(iterable),
        "shuffled" to setOf(iterable),
        "toMutableList" to setOf(iterable, collection),
        "first" to setOf(list),
        "last" to setOf(list),
        "firstOrNull" to setOf(list),
        "lastOrNull" to setOf(list),
        "count" to setOf(collection),
        "plusAssign" to setOf(mutableCollection),
        "minusAssign" to setOf(mutableCollection),
    ).mapKeys { Name.identifier(it.key) }
    // The predicate overloads of these walk the iterator.
    private val noArgumentExtensions = names("first", "last", "firstOrNull", "lastOrNull", "count")
    // These copy a Collection through toArray(), except that one with a
    // single element takes it from get(0) on a List but from iterator()
    // otherwise.
    private val listOnlyExtensions =
        names("toList", "sorted", "sortedDescending", "sortedWith", "sortedBy", "sortedByDescending", "reversed")

    private fun names(vararg names: String): Set<Name> = names.mapTo(HashSet(), Name::identifier)

    // What an operation on the wrapper does with it.
    private enum class Use {
        // Reads it under its own lock; no iterator.
        Guarded,
        // Returns a view or the wrapper itself, still guarded by its lock.
        LockedView,
        // Returns an adapter that iterates it lazily.
        LazyView,
        // Walks its iterator.
        Iterates,
        // Something else: a project call, a scope function, an argument.
        Other,
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        // FIR rewrites `for (x in xs)` into `val it = xs.iterator()` and a
        // while loop; that iterator() call carries the loop's desugared source
        // and xs as its receiver.
        val source = expression.source ?: return
        if (source.kind != KtFakeSourceElementKind.DesugaredForLoop) return
        if (expression.calleeReference.name != iteratorName) return
        val iterable = expression.explicitReceiver ?: return
        val loop = enclosingFor(source) ?: return
        val path = context.containingElements

        val inline = containsWrapperCall(iterable) || iteratesWrapperInBody(expression, path)
        val held = !inline && iteratesWrapperValue(iterable)
        if (!inline && !held) return
        val wrapper = if (held) heldWrapperSymbol(iterable) else null
        if (wrapper != null && lockHeld(path, wrapper)) return
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

    // The value [expression] passes on, through smart casts, casts, and `!!`.
    private fun unwrapValue(expression: FirExpression): FirExpression {
        var current = expression
        while (true) {
            current = when {
                current is FirSmartCastExpression -> current.originalExpression
                current is FirCheckNotNullCall -> current.argumentList.arguments.firstOrNull() ?: return current
                current is FirTypeOperatorCall &&
                    (current.operation == FirOperation.AS || current.operation == FirOperation.SAFE_AS) ->
                    current.argumentList.arguments.firstOrNull() ?: return current
                else -> return current
            }
        }
    }

    private fun use(access: FirQualifiedAccessExpression): Use {
        val symbol = access.calleeReference.toResolvedCallableSymbol() ?: return Use.Other
        val callableId = symbol.callableId ?: return Use.Other
        val name = callableId.callableName
        val owner = callableId.classId
        if (owner != null) {
            // java.util.Collections wraps every member in the wrapper's lock
            // except the ones handing out an iterator, stream, or view.
            if (owner == StandardClassIds.Any) return Use.Guarded
            if (owner.packageFqName !in memberPackages) return Use.Other
            return when (name) {
                in iteratorMembers -> Use.Iterates
                in viewMembers -> Use.LockedView
                else -> Use.Guarded
            }
        }
        val packageName = callableId.packageName
        if (packageName == kotlinPackage) {
            return when (name) {
                in identityFunctions -> Use.LockedView
                toStringName -> Use.Guarded
                else -> Use.Other
            }
        }
        if (packageName !in extensionPackages) return Use.Other
        return when {
            name in identityCollectionFunctions -> Use.LockedView
            name in adapterFunctions -> Use.LazyView
            isGuardedExtension(access, symbol, name) -> Use.Guarded
            else -> Use.Iterates
        }
    }

    private fun isGuardedExtension(access: FirQualifiedAccessExpression, symbol: FirCallableSymbol<*>, name: Name): Boolean {
        val receivers = guardedExtensions[name] ?: return false
        if (name in noArgumentExtensions && (access as? FirFunctionCall)?.argumentList?.arguments?.isNotEmpty() == true) {
            return false
        }
        val receiver = symbol.resolvedReceiverType?.lowerBoundIfFlexible()?.classId ?: return false
        if (receiver !in receivers) return false
        if (name !in listOnlyExtensions) return true
        val actual = access.explicitReceiver?.resolvedType?.lowerBoundIfFlexible()?.classId
        return actual == list || actual == mutableList
    }

    // The wrapper call [expression] reads under the wrapper's lock: the
    // wrapper itself, or a view or identity call on it.
    private fun lockedWrapper(expression: FirExpression): FirFunctionCall? {
        val value = unwrapValue(expression)
        if (isWrapperCall(value)) return value as FirFunctionCall
        if (value is FirQualifiedAccessExpression && use(value) == Use.LockedView) {
            return value.explicitReceiver?.let(::lockedWrapper)
        }
        return null
    }

    // A wrapper call in the iterable, as Go's text match of the loop header
    // finds one, except as the receiver of an operation that reads it under
    // its own lock (`0 until wrapper.size`, `wrapper.toList()`): the loop
    // does not iterate the wrapper through that value.
    private fun containsWrapperCall(iterable: FirExpression): Boolean {
        var found = false
        iterable.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (isWrapperCall(element)) {
                    found = true
                    return
                }
                if (element is FirQualifiedAccessExpression && use(element) == Use.Guarded) {
                    val wrapper = element.explicitReceiver?.let(::lockedWrapper)
                    if (wrapper != null) {
                        wrapper.argumentList.accept(this)
                        (element as? FirFunctionCall)?.argumentList?.accept(this)
                        return
                    }
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    // The loop's body iterates a wrapper call written in the loop, as Go's
    // text match of the whole statement sees it: an operation that walks the
    // wrapper's iterator, on the call, on a view of it, or on a local the
    // body initializes with it.
    // An inner loop over such a value counts only when it does not report
    // the wrapper itself.
    private fun iteratesWrapperInBody(iteratorCall: FirFunctionCall, path: List<FirElement>): Boolean {
        val index = path.indexOfLast { it === iteratorCall }
        val variable = path.getOrNull(index - 1) as? FirVariable ?: return false
        val block = path.getOrNull(index - 2) as? FirBlock ?: return false
        val position = block.statements.indexOfFirst { it === variable }
        if (position < 0) return false
        val whileLoop = block.statements.drop(position + 1).firstOrNull { it is FirWhileLoop } as? FirWhileLoop
            ?: return false

        val locals = HashSet<FirBasedSymbol<*>?>()
        whileLoop.block.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirProperty && element.isLocal) {
                    val initializer = element.initializer
                    if (initializer != null && isWrapperCall(unwrapValue(initializer))) locals += element.symbol
                }
                element.acceptChildren(this)
            }
        })

        fun holdsWrapperInBody(expression: FirExpression): Boolean {
            val value = unwrapValue(expression)
            if (isWrapperCall(value)) return true
            if (value !is FirQualifiedAccessExpression) return false
            if (value.explicitReceiver == null && locals.contains(value.calleeReference.toResolvedCallableSymbol())) return true
            val use = use(value)
            if (use != Use.LockedView && use != Use.LazyView) return false
            return value.explicitReceiver?.let(::holdsWrapperInBody) ?: false
        }

        var found = false
        whileLoop.block.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirQualifiedAccessExpression && iteratesInBody(element, ::holdsWrapperInBody)) {
                    found = true
                } else {
                    element.acceptChildren(this)
                }
            }
        })
        return found
    }

    private fun iteratesInBody(
        access: FirQualifiedAccessExpression,
        holdsWrapper: (FirExpression) -> Boolean,
    ): Boolean {
        val receiver = access.explicitReceiver ?: return false
        if (!holdsWrapper(receiver)) return false
        val source = access.source
        if (access is FirFunctionCall && source?.kind == KtFakeSourceElementKind.DesugaredForLoop &&
            access.calleeReference.name == iteratorName
        ) {
            // An inner loop over a held wrapper checks its own monitor. A
            // mutable local is not tracked as held, so its outer loop reports.
            return !containsWrapperCall(receiver) && !iteratesWrapperValue(receiver)
        }
        return use(access) == Use.Iterates
    }

    // The iterable is a `val` holding a wrapper, directly or through a view or
    // adapter that iterates it lazily.
    private fun iteratesWrapperValue(iterable: FirExpression): Boolean = heldWrapperSymbol(iterable) != null

    private fun heldWrapperSymbol(iterable: FirExpression): FirPropertySymbol? {
        var current = unwrapValue(iterable)
        while (current is FirQualifiedAccessExpression) {
            val symbol = current.calleeReference.toResolvedCallableSymbol()
            val adapts = when (use(current)) {
                Use.LockedView, Use.LazyView -> true
                Use.Iterates -> symbol?.name in iteratorMembers
                else -> false
            }
            if (!adapts) return (symbol as? FirPropertySymbol)?.takeIf(::holdsWrapper)
            current = unwrapValue(current.explicitReceiver ?: return null)
        }
        return null
    }

    // A read-only property whose value is the wrapper its initializer
    // creates: a `val` that is not open and has no explicit getter.
    @OptIn(SymbolInternals::class)
    private fun holdsWrapper(symbol: FirPropertySymbol): Boolean {
        if (symbol.isVar) return false
        if (symbol.resolvedStatus.modality == Modality.OPEN || symbol.resolvedStatus.modality == Modality.ABSTRACT) {
            return false
        }
        if (symbol.getterSymbol?.isDefault == false && !symbol.hasDelegate) return false
        val initializer = symbol.resolvedInitializer
        if (initializer != null && isWrapperCall(unwrapValue(initializer))) return true
        val delegate = symbol.delegate ?: return false
        var lazy = false
        var wrapper = false
        delegate.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirFunctionCall) {
                    if (element.calleeReference.toResolvedCallableSymbol()?.callableId == lazyId) lazy = true
                    if (isWrapperCall(element)) wrapper = true
                }
                element.acceptChildren(this)
            }
        })
        return lazy && wrapper
    }

    // Only the wrapper's own monitor protects iteration. A method annotation
    // or a separate lock does not; a private helper is safe only if every
    // caller holds the wrapper's monitor.
    context(context: CheckerContext)
    private fun lockHeld(path: List<FirElement>, wrapper: FirPropertySymbol): Boolean {
        if (lockHeldAround(path, wrapper)) return true
        val function = enclosingFunction(path) ?: return false
        return callersHoldLock(function, path.firstOrNull() as? FirFile ?: return false, wrapper)
    }

    private fun lockHeldAround(path: List<FirElement>, wrapper: FirPropertySymbol): Boolean {
        for (i in path.indices.reversed()) {
            when (val element = path[i]) {
                is FirFunctionCall -> {
                    val child = path.getOrNull(i + 1)
                    if (child != null && child !== element.argumentList.arguments.firstOrNull() &&
                        isWrapperSynchronizedCall(element, wrapper)
                    ) return true
                }
                is FirNamedFunction, is FirPropertyAccessor -> return false
                else -> {}
            }
        }
        return false
    }

    private fun enclosingFunction(path: List<FirElement>): FirNamedFunction? {
        for (i in path.indices.reversed()) {
            when (val element = path[i]) {
                is FirNamedFunction -> return element
                is FirPropertyAccessor -> return null
                else -> {}
            }
        }
        return null
    }

    private fun isWrapperSynchronizedCall(call: FirFunctionCall, wrapper: FirPropertySymbol): Boolean {
        val callableId = call.calleeReference.toResolvedCallableSymbol()?.callableId ?: return false
        if (callableId.classId != null || callableId.packageName != FqName("kotlin") ||
            callableId.callableName.asString() != SYNCHRONIZED
        ) return false
        val lock = unwrapValue(call.argumentList.arguments.firstOrNull() ?: return false)
        return (lock as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedCallableSymbol() == wrapper
    }

    // Every call of the private or local [function] in [file] holds a lock,
    // and there is at least one; a function reference may run anywhere.
    private fun callersHoldLock(function: FirNamedFunction, file: FirFile, wrapper: FirPropertySymbol): Boolean {
        val visibility = function.symbol.resolvedStatus.visibility
        if (visibility != Visibilities.Private && visibility != Visibilities.Local) return false
        val target = function.symbol
        var guarded = 0
        var unguarded = false
        file.accept(object : FirVisitorVoid() {
            val stack = ArrayList<FirElement>()

            override fun visitElement(element: FirElement) {
                if (unguarded) return
                stack += element
                when (element) {
                    is FirCallableReferenceAccess ->
                        if (element.calleeReference.toResolvedCallableSymbol() == target) unguarded = true
                    is FirFunctionCall -> if (element.calleeReference.toResolvedCallableSymbol() == target) {
                        val locked = lockHeldAround(stack, wrapper)
                        if (locked) guarded++ else unguarded = true
                    }
                    else -> {}
                }
                element.acceptChildren(this)
                stack.removeAt(stack.lastIndex)
            }
        })
        return guarded > 0 && !unguarded
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

}
