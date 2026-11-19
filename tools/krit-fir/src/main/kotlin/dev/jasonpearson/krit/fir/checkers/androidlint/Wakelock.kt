package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.inheritsFrom
import dev.jasonpearson.krit.fir.support.isOrInheritsFrom
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.qualifiedCall
import dev.jasonpearson.krit.fir.support.unwrapArgument
import dev.jasonpearson.krit.fir.support.unwrapSmartCasts
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirFileChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousInitializer
import org.jetbrains.kotlin.fir.declarations.FirConstructor
import org.jetbrains.kotlin.fir.declarations.FirDeclaration
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirPropertyAccessor
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.toClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirValueParameterSymbol
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Flags a WakeLock `acquire()` that nothing in the file releases: no call
 * written `release` on the same receiver, anywhere in the file. Android Lint
 * never reports a lock for lack of a release in the acquiring method (a lock
 * acquired in `onResume` and released in `onPause` is the intended use); it
 * reports an acquire only when the project has no `release()` at all. This
 * checker reads that at file scope, by receiver identity.
 *
 * Like the Go rule, the acquire is a call written `acquire` with an explicit
 * receiver (`wl.acquire()`, `wl?.acquire()`, `holder.wl.acquire()`)
 * that is a WakeLock: its type is a class whose simple name is `WakeLock`
 * (`android.os.PowerManager.WakeLock`, or a class of the same name in any
 * package, as Go matches the receiver type by that simple name), or the
 * receiver is a variable whose declaration inside the same function is
 * initialized by a call written `newWakeLock` (Go's fallback when it cannot
 * type the receiver). The acquire must sit in a named function, looking
 * through lambdas, anonymous functions, and local classes and objects (their
 * initializers, constructors, and accessors, not their member functions,
 * which are named functions of their own). An acquire outside any named
 * function (a property initializer, an init block, a property accessor) is
 * not reported, as in Go. A lock the caller owns is not reported either: a
 * receiver reached only through a parameter of a function or lambda, or the
 * receiver of an extension (`fun start(lock: WakeLock) { lock.acquire() }`),
 * is released, if at all, by whoever passed it in.
 *
 * The acquire counts as released when the file (any function, lambda,
 * property, or init block, before or after the acquire) has a call written
 * `release` whose receiver is a WakeLock by the same test and is the same
 * object as the acquire's receiver: the same variable reached through the
 * same receiver chain (`wl` and `this.wl` in a class are the same, also from
 * another member function, while `holder.wl` is not), or an alias of it. The
 * finding sits on the acquire call's first line, which is the receiver's
 * first line, with Go's message. `acquire(timeout)` is never reported, as it
 * releases itself.
 *
 * Like Lint, which does not analyze test sources, it skips test files (an
 * `acquire()` there is usually a mock verification).
 *
 * As in Lint, a release on a WakeLock receiver in a method named `onDestroy`
 * of an `android.app.Activity` subclass is also reported, on the release:
 * the lock should be released in `onPause`. Lambdas and local functions sit
 * inside the method they belong to.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Go matches a release by the last identifier of its receiver as written,
 *   so `second.wl.release()` clears `first.wl.acquire()`, and Go reads no
 *   name through parentheses or `!!` (it reports `(wl).acquire()` even when
 *   `wl` is released, and cannot type `wl!!`). This checker matches by
 *   receiver identity, through parentheses and `!!`.
 * - A receiver is also the variable it aliases: a local variable is the
 *   variable it holds where it is read (its last assignment before the read
 *   in source order, else its initializer: `val lock = wakeLock`,
 *   `val lock = wakeLock ?: return`), and the parameter of a `let`, `also`,
 *   `takeIf`, or `takeUnless` lambda, like the receiver of an `apply`,
 *   `run`, or `with` lambda, is that call's receiver (looking through
 *   `also`, `apply`, `takeIf`, and `takeUnless` calls, which return their
 *   receiver). So `wakeLock?.let { it.release() }` releases `wakeLock`,
 *   which Go does not see, and `other.wakeLock.let { it.release() }` does
 *   not. A call result is not an alias of a local, as each call may return a
 *   different lock.
 * - A release on an implicit receiver (`with(lock) { release() }`) counts,
 *   and `this` is the declaration or lambda receiver it is bound to. Go
 *   needs a written receiver and reads no name from `this`.
 * - Resolution types receivers Go's source inference cannot: a
 *   `PowerManager.WakeLock` annotation (Go reads it as `PowerManager`), a
 *   property or local initialized from `newWakeLock`, a call result, a type
 *   alias, a smart cast, `!!`, and a subtype of a class named `WakeLock`.
 * - Go's `newWakeLock` fallback matches any earlier declaration with the
 *   receiver's name in the function, even one the receiver does not refer to
 *   (a shadowed or out-of-scope local). This checker takes the fallback only
 *   for the declaration the receiver resolves to.
 * - Go reads a parameter only by the receiver's first name, so it still
 *   reports a lock reached through a lambda parameter (`locks.forEach {
 *   it.acquire() }` when it can type `it`).
 */
internal object Wakelock : FirFileChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "Wakelock"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val fileCheckers = setOf(Wakelock)
    }

    private const val MESSAGE = "WakeLock acquired without release. Ensure WakeLock.release() is called."
    private const val ON_DESTROY_MESSAGE = "Wakelocks should be released in `onPause`, not `onDestroy`"
    private val onDestroy = Name.identifier("onDestroy")
    private val activity = classIdOf("android.app", "Activity")
    private val acquire = Name.identifier("acquire")
    private val release = Name.identifier("release")
    private val newWakeLock = Name.identifier("newWakeLock")
    private val wakeLock = Name.identifier("WakeLock")
    private val kotlinPackage = FqName("kotlin")

    // Scope functions whose lambda parameter is the receiver.
    private val lambdaScopeFunctions = setOf("let", "also", "takeIf", "takeUnless").map(Name::identifier).toSet()

    // Scope functions whose lambda receiver is the call's receiver.
    private val receiverScopeFunctions = setOf("apply", "run", "with").map(Name::identifier).toSet()
    private val with = Name.identifier("with")

    // Scope functions that return their receiver (or null).
    private val returningScopeFunctions = setOf("also", "apply", "takeIf", "takeUnless").map(Name::identifier).toSet()

    private const val MAX_ALIAS_STEPS = 32

    // What the file's scope-function lambdas bind (see [check]) and the
    // assignments to its local vars.
    private class Bindings(
        val scopes: Map<FirBasedSymbol<*>, FirExpression>,
        val assignments: Map<FirBasedSymbol<*>, List<FirVariableAssignment>>,
    )

    // The root of a path through a `this` bound to something other than a
    // class: an extension receiver or a lambda receiver. It is the caller's
    // object unless a scope function binds it to a variable.
    private data class ForeignThis(val bound: FirBasedSymbol<*>)

    // A call written `acquire` or `release` on a WakeLock receiver: the call,
    // its source (for a written receiver, the qualified expression), which is
    // the finding's anchor, the receiver, the nearest named function (null
    // outside one), and the properties other than locals that function
    // assigns.
    private class Candidate(
        val call: FirFunctionCall,
        val anchor: KtSourceElement,
        val receiver: FirExpression,
        val function: FirNamedFunction?,
        val reassigned: Set<FirBasedSymbol<*>>,
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirFile) {
        // Lint does not analyze test sources, where acquire() is a mock verification.
        if (isTestFile(declaration.sourceFile?.path)) return
        val session = context.session
        val acquires = ArrayList<Candidate>()
        val releases = ArrayList<Candidate>()
        // What a scope function's lambda binds, by symbol: the parameter of a
        // let / also / takeIf / takeUnless lambda, and the receiver of an
        // apply / run / with lambda, are that call's receiver.
        val aliases = HashMap<FirBasedSymbol<*>, FirExpression>()
        // The assignments to each local var after its declaration.
        val assignments = HashMap<FirBasedSymbol<*>, MutableList<FirVariableAssignment>>()
        // The properties other than locals that each named function assigns
        // (the null key: code outside any named function).
        val reassigned = HashMap<FirNamedFunction?, MutableSet<FirBasedSymbol<*>>>()
        val functions = ArrayDeque<FirNamedFunction>()
        // The enclosing methods as Lint sees them: lambdas and local
        // functions sit inside the method they belong to.
        val methods = ArrayDeque<FirDeclaration>()
        val lateReleases = ArrayList<Candidate>()
        declaration.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                val method = element is FirConstructor || element is FirPropertyAccessor ||
                    element is FirAnonymousInitializer ||
                    (element is FirNamedFunction && element.status.visibility != Visibilities.Local)
                if (method) methods.addLast(element as FirDeclaration)
                if (element is FirNamedFunction) {
                    functions.addLast(element)
                    element.acceptChildren(this)
                    functions.removeLast()
                } else {
                    visitOther(element)
                }
                if (method) methods.removeLast()
            }

            private fun visitOther(element: FirElement) {
                val function = functions.lastOrNull()
                if (element is FirVariableAssignment) {
                    val target = element.lValue as? FirQualifiedAccessExpression
                    val symbol = target?.calleeReference?.toResolvedCallableSymbol() as? FirPropertySymbol
                    if (symbol != null && symbol.isLocal) {
                        assignments.getOrPut(symbol) { ArrayList() } += element
                    } else if (symbol != null) {
                        reassigned.getOrPut(function) { HashSet() } += symbol
                    }
                }
                if (element is FirFunctionCall) {
                    val name = element.calleeReference.name
                    recordScopeLambda(element, aliases)
                    if (name == acquire || name == release) {
                        val assigned = reassigned.getOrPut(function) { HashSet() }
                        val candidate = candidate(element, function, assigned)
                        if (candidate != null && isWakeLockReceiver(candidate, session)) {
                            if (name == release) {
                                releases += candidate
                                if (inActivityOnDestroy(methods.lastOrNull(), session)) lateReleases += candidate
                            } else if (isReportable(function) && element.argumentList.arguments.isEmpty()) {
                                // acquire(timeout) releases itself when the timeout expires.
                                acquires += candidate
                            }
                        }
                    }
                }
                element.acceptChildren(this)
            }
        })
        for (late in lateReleases) report(late.anchor, ON_DESTROY_MESSAGE)
        if (acquires.isEmpty()) return
        val bindings = Bindings(aliases, assignments)
        val released = releases.flatMapTo(HashSet()) { identities(it.receiver, bindings, it.reassigned) }
        for (acquired in acquires) {
            val identities = identities(acquired.receiver, bindings, acquired.reassigned)
            if (identities.isNotEmpty() && identities.none(::isOwned)) continue
            if (identities.none { it in released }) report(acquired.anchor, MESSAGE)
        }
    }

    // Lint's onDestroy check: an Activity's onDestroy runs too late to
    // release a lock the user no longer needs.
    private fun inActivityOnDestroy(method: FirDeclaration?, session: FirSession): Boolean {
        if (method !is FirNamedFunction || method.name != onDestroy) return false
        val owner = method.symbol.getContainingClassSymbol() ?: return false
        return isOrInheritsFrom(owner, activity, session, lookupInterfaces = false)
    }

    private fun isReportable(function: FirNamedFunction?): Boolean =
        function != null && function.source?.kind is KtRealSourceElementKind

    // A path whose root is not a parameter or a non-class `this`: a local,
    // a property, a class's `this`, an object, or a top-level declaration.
    private fun isOwned(path: List<Any>): Boolean {
        val root = path.firstOrNull() ?: return false
        return root !is FirValueParameterSymbol && root !is ForeignThis
    }

    private fun recordScopeLambda(call: FirFunctionCall, aliases: MutableMap<FirBasedSymbol<*>, FirExpression>) {
        val name = call.calleeReference.name
        if (name !in lambdaScopeFunctions && name !in receiverScopeFunctions) return
        if (!isStdlib(call)) return
        val arguments = call.argumentList.arguments.map(::unwrapArgument)
        // `with(lock) { ... }` takes its receiver as the first argument, and
        // `run { ... }` in a class runs on the implicit `this`.
        val subject = call.explicitReceiver
            ?: arguments.firstOrNull()?.takeIf { name == with }
            ?: call.extensionReceiver
            ?: return
        for (argument in arguments) {
            val lambda = (argument as? FirAnonymousFunctionExpression)?.anonymousFunction ?: continue
            if (name in lambdaScopeFunctions) {
                for (parameter in lambda.valueParameters) aliases[parameter.symbol] = subject
            } else {
                // `this` is bound to the lambda's receiver parameter.
                val receiver = lambda.receiverParameter ?: continue
                aliases[receiver.symbol] = subject
                aliases[lambda.symbol] = subject
            }
        }
    }

    private fun isStdlib(call: FirFunctionCall): Boolean =
        call.calleeReference.toResolvedCallableSymbol()?.callableId?.packageName == kotlinPackage

    // Go takes an acquire or release only with a written receiver. One on an
    // implicit receiver (`with(lock) { release() }`, `acquire()` in a
    // WakeLock extension) also counts here.
    private fun candidate(call: FirFunctionCall, function: FirNamedFunction?, reassigned: Set<FirBasedSymbol<*>>): Candidate? {
        val source = call.source ?: return null
        // A safe call's selector carries the whole `r?.f()` as a desugared
        // source; any other fake source is not a call in the code.
        if (source.kind !is KtRealSourceElementKind && source.kind != KtFakeSourceElementKind.DesugaredSafeCallExpression) {
            return null
        }
        val explicit = call.explicitReceiver
        if (explicit == null) {
            val receiver = listOfNotNull(call.dispatchReceiver, call.extensionReceiver)
                .firstOrNull { it is FirThisReceiverExpression && it.isImplicit }
                ?: return null
            return Candidate(call, source, receiver, function, reassigned)
        }
        val qualified = qualifiedCall(source) ?: return null
        return Candidate(call, lightSourceOf(qualified, source), explicit, function, reassigned)
    }

    // The identities a receiver goes by, each the variable it reads as a path
    // from a root (a local, a parameter, a `this`, a qualifier) through the
    // properties and calls of its receiver chain: `lock` in a class and
    // `this.lock` are both [this@Class, lock], and `other.lock` is
    // [other, lock]. A local variable also goes by the variable it holds
    // where it is read (its last assignment before the read in source order,
    // else its initializer: `val lock = wakeLock`, `val lock = wakeLock ?:
    // return`), and the parameter of a let / also / takeIf / takeUnless
    // lambda, like the receiver of an apply / run / with lambda, by that
    // call's receiver. A call result is not an alias of a local, as each call
    // may return a different lock. Empty for a receiver with no such path (a
    // cast, an index, a literal), which never matches. [reassigned] holds the
    // properties the reading function assigns.
    private fun identities(
        expression: FirExpression,
        bindings: Bindings,
        reassigned: Set<FirBasedSymbol<*>>,
        depth: Int = 0,
    ): Set<List<Any>> {
        if (depth > MAX_ALIAS_STEPS) return emptySet()
        val next = depth + 1
        return when (val current = unwrapSmartCasts(expression, throughNotNull = true)) {
            is FirSafeCallExpression ->
                (current.selector as? FirExpression)?.let { identities(it, bindings, reassigned, next) }.orEmpty()
            is FirThisReceiverExpression -> {
                val bound = current.calleeReference.boundSymbol ?: return emptySet()
                val subject = bindings.scopes[bound]
                val root: Any = if (bound is FirClassLikeSymbol<*>) bound else ForeignThis(bound)
                setOf(listOf(root)) + subject?.let { identities(it, bindings, reassigned, next) }.orEmpty()
            }
            is FirResolvedQualifier -> {
                val symbol = current.qualifierSymbol ?: return emptySet()
                val companion = (symbol as? FirRegularClassSymbol)?.resolvedCompanionObjectSymbol
                setOf(listOf<Any>(if (current.resolvedToCompanionObject && companion != null) companion else symbol))
            }
            is FirFunctionCall -> {
                val symbol = current.calleeReference.toResolvedCallableSymbol() ?: return emptySet()
                when {
                    // also / apply / takeIf / takeUnless return their receiver.
                    current.calleeReference.name in returningScopeFunctions && isStdlib(current) ->
                        current.explicitReceiver?.let { identities(it, bindings, reassigned, next) }.orEmpty()
                    // Only a call written as one; an operator (an index, `+`) has no path.
                    !isWrittenCall(current) -> emptySet()
                    else -> chain(current, symbol, bindings, reassigned, next)
                }
            }
            is FirQualifiedAccessExpression -> {
                when (val symbol = current.calleeReference.toResolvedCallableSymbol()) {
                    null -> emptySet()
                    is FirPropertySymbol -> if (symbol.isLocal) {
                        // A local holds the value its alias had when assigned,
                        // so a property the function assigns may have changed
                        // since: `val old = this.lock; this.lock = new` does
                        // not make `old` the new lock.
                        val held = valueAt(symbol, current, bindings)
                            ?.let(::variableOf)?.let { identities(it, bindings, reassigned, next) }.orEmpty()
                            .filterTo(HashSet()) { path -> path.none { it in reassigned } }
                        setOf(listOf<Any>(symbol)) + held
                    } else {
                        chain(current, symbol, bindings, reassigned, next)
                    }
                    is FirValueParameterSymbol -> setOf(listOf<Any>(symbol)) +
                        bindings.scopes[symbol]?.let { identities(it, bindings, reassigned, next) }.orEmpty()
                    else -> chain(current, symbol, bindings, reassigned, next)
                }
            }
            else -> emptySet()
        }
    }

    // A member reached through its receiver: each identity of the receiver
    // (explicit, else the implicit dispatch or extension receiver) followed by
    // the member; the member alone when it has no receiver (a top-level
    // property or function).
    private fun chain(
        access: FirQualifiedAccessExpression,
        symbol: FirBasedSymbol<*>,
        bindings: Bindings,
        reassigned: Set<FirBasedSymbol<*>>,
        depth: Int,
    ): Set<List<Any>> {
        val receiver = access.explicitReceiver ?: access.dispatchReceiver ?: access.extensionReceiver
            ?: return setOf(listOf<Any>(symbol))
        return identities(receiver, bindings, reassigned, depth).mapTo(HashSet()) { it + symbol }
    }

    private fun isWrittenCall(call: FirFunctionCall): Boolean {
        val source = call.source ?: return false
        if (source.kind !is KtRealSourceElementKind && source.kind != KtFakeSourceElementKind.DesugaredSafeCallExpression) {
            return false
        }
        return source.lighterASTNode.tokenType == KtNodeTypes.CALL_EXPRESSION || qualifiedCall(source) != null
    }

    // The value a local variable holds where [read] reads it, going by source
    // order: the last assignment that starts before the read, else the
    // initializer.
    private fun valueAt(symbol: FirPropertySymbol, read: FirExpression, bindings: Bindings): FirExpression? {
        val offset = read.source?.startOffset ?: return null
        val assignment = bindings.assignments[symbol].orEmpty()
            .filter { (it.source?.startOffset ?: Int.MAX_VALUE) < offset }
            .maxByOrNull { it.source?.startOffset ?: Int.MIN_VALUE }
        return assignment?.rValue ?: symbol.resolvedInitializer
    }

    // The variable a value reads, through an elvis's left side, safe calls,
    // and also / apply / takeIf / takeUnless calls, which return their
    // receiver; null for any other value.
    private fun variableOf(value: FirExpression): FirExpression? {
        var target = if (value is FirElvisExpression) value.lhs else value
        repeat(MAX_ALIAS_STEPS) {
            target = unwrapSmartCasts(target, throughNotNull = true)
            val current = target
            when {
                current is FirSafeCallExpression -> target = current.selector as? FirExpression ?: return null
                current is FirFunctionCall && current.calleeReference.name in returningScopeFunctions && isStdlib(current) ->
                    target = current.explicitReceiver ?: return null
                else -> return target.takeIf { it is FirQualifiedAccessExpression && it !is FirFunctionCall }
            }
        }
        return null
    }

    private fun isWakeLockReceiver(candidate: Candidate, session: FirSession): Boolean {
        // The receiver's type as used here, smart casts included.
        val receiver = candidate.receiver
        val type = receiver.resolvedType.fullyExpandedType(session).lowerBoundIfFlexible()
        val symbol = type.toClassLikeSymbol(session)
        if (symbol != null && isWakeLockClass(symbol, session)) return true
        return declaredFromNewWakeLock(unwrapSmartCasts(receiver, throughNotNull = true), candidate)
    }

    // The class itself or a supertype named WakeLock, read from the class's
    // own lookup tags, which are bound to local and anonymous classes, so no
    // class id is resolved from a symbol that may be local.
    private fun isWakeLockClass(symbol: FirClassLikeSymbol<*>, session: FirSession): Boolean =
        symbol.classId.shortClassName == wakeLock ||
            inheritsFrom(symbol, session, lookupInterfaces = true) { it.shortClassName == wakeLock }

    // Go's fallback when it cannot type the receiver: a variable declared in
    // the call's nearest named function, before the call, whose initializer
    // is a call written newWakeLock. Only the declaration the receiver
    // resolves to counts.
    private fun declaredFromNewWakeLock(receiver: FirExpression, candidate: Candidate): Boolean {
        val access = receiver as? FirQualifiedAccessExpression ?: return false
        if (access is FirFunctionCall) return false
        val property = access.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol ?: return false
        // Source offsets alone do not prove the property is in this file.
        if (!property.isLocal) return false
        val declared = property.source ?: return false
        val function = candidate.function?.source ?: return false
        if (declared.startOffset < function.startOffset || declared.endOffset > function.endOffset) return false
        if (declared.startOffset >= candidate.anchor.startOffset) return false
        var initializer = property.resolvedInitializer ?: return false
        if (initializer is FirSafeCallExpression) initializer = initializer.selector as? FirExpression ?: return false
        return initializer is FirFunctionCall && initializer.calleeReference.name == newWakeLock
    }
}
