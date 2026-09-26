package dev.jasonpearson.krit.fir.checkers.androidlint

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirDeclarationChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.toClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirValueParameterSymbol
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Flags a WakeLock `acquire()` whose enclosing named function never calls
 * `release()` on the same receiver.
 *
 * Like the Go rule, the acquire is a call written `acquire` with an explicit
 * receiver (`wl.acquire()`, `wl?.acquire()`, `holder.wl.acquire(timeout)`)
 * that is a WakeLock: its type is a class whose simple name is `WakeLock`
 * (`android.os.PowerManager.WakeLock`, or a class of the same name in any
 * package, as Go matches the receiver type by that simple name), or the
 * receiver is a variable whose declaration inside the same function is
 * initialized by a call written `newWakeLock` (Go's fallback when it cannot
 * type the receiver). The function is the nearest enclosing named function,
 * looking through lambdas, anonymous functions, and local classes and
 * objects (their initializers, constructors, and accessors, not their member
 * functions, which are named functions of their own). An acquire outside any
 * named function (a property initializer, an init block, a property
 * accessor) is not reported, as in Go.
 *
 * The acquire counts as released when that function's whole body (lambdas,
 * local functions, and local classes included, before or after the acquire)
 * has a call written `release` whose receiver is a WakeLock by the same test
 * (the variable fallback is read in the release's own nearest named
 * function) and names the same thing as the acquire's receiver. Like Go, the
 * name is the last identifier of the receiver as written (`wl`, `this.wl`,
 * `holder.wl`, and `getLock()` name `wl`, `wl`, `wl`, and `getLock`), so a
 * release through another chain with the same last segment counts. The
 * finding sits on the acquire call's first line, which is the receiver's
 * first line, with Go's message.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Go reads no name through parentheses or `!!`, so it reports
 *   `(wl).acquire()` even when `wl` is released, and cannot type `wl!!`.
 *   This checker reads the name through both.
 * - A receiver also names the variables it aliases: a local variable names
 *   the variable it holds where it is read (its last assignment before the
 *   read in source order, else its initializer: `val lock = wakeLock`,
 *   `val lock = wakeLock ?: return`), and the parameter of a `let`, `also`,
 *   `takeIf`, or `takeUnless` lambda, like the receiver of an `apply`,
 *   `run`, or `with` lambda, names that call's receiver (looking through
 *   `also`, `apply`, `takeIf`, and `takeUnless` calls, which return their
 *   receiver). So `wakeLock?.let { it.release() }` releases `wakeLock`,
 *   which Go does not see. A call result is not an alias, as each call may
 *   return a different lock (Go's name match does not relate `val a = f()`
 *   to `f()` either).
 * - A release on an implicit receiver (`with(lock) { release() }`,
 *   `release()` in a WakeLock extension) counts, and `this` names the
 *   declaration or lambda it is bound to. Go needs a written receiver for a
 *   release and reads no name from `this`.
 * - Resolution types receivers Go's source inference cannot: a
 *   `PowerManager.WakeLock` annotation (Go reads it as `PowerManager`), a
 *   property or local initialized from `newWakeLock`, a call result, a type
 *   alias, a smart cast, `!!`, and a subtype of a class named `WakeLock`.
 * - Go's `newWakeLock` fallback matches any earlier declaration with the
 *   receiver's name in the function, even one the receiver does not refer to
 *   (a shadowed or out-of-scope local). This checker takes the fallback only
 *   for the declaration the receiver resolves to.
 */
internal object Wakelock : FirDeclarationChecker<FirNamedFunction>(MppCheckerKind.Common), FirRule {
    override val ruleId = "Wakelock"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val simpleFunctionCheckers = setOf(Wakelock)
    }

    private const val MESSAGE = "WakeLock acquired without release. Ensure WakeLock.release() is called."
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

    private val qualifiedTypes = setOf(KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION)
    private const val MAX_ALIAS_STEPS = 32

    // What the function's scope-function lambdas bind (see [check]) and the
    // assignments to its local vars.
    private class Bindings(
        val scopes: Map<FirBasedSymbol<*>, FirExpression>,
        val assignments: Map<FirBasedSymbol<*>, List<FirVariableAssignment>>,
    )

    // A call written `acquire` or `release` on a WakeLock receiver: the call,
    // its source (for a written receiver, the qualified expression, which is
    // the finding's anchor), the written receiver's light-tree node, the
    // receiver, and the named function whose body Go searches for it.
    private class Candidate(
        val call: FirFunctionCall,
        val anchor: KtSourceElement,
        val written: LighterASTNode?,
        val receiver: FirExpression,
        val function: FirNamedFunction,
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirNamedFunction) {
        if (declaration.source?.kind !is KtRealSourceElementKind) return
        val session = context.session
        val acquires = ArrayList<Candidate>()
        val releases = ArrayList<Candidate>()
        // What a scope function's lambda binds, by symbol: the parameter of a
        // let / also / takeIf / takeUnless lambda, and the receiver of an
        // apply / run / with lambda, are that call's receiver.
        val aliases = HashMap<FirBasedSymbol<*>, FirExpression>()
        // The assignments to each local var after its declaration.
        val assignments = HashMap<FirBasedSymbol<*>, MutableList<FirVariableAssignment>>()
        val functions = ArrayDeque<FirNamedFunction>()
        declaration.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirNamedFunction) {
                    functions.addLast(element)
                    element.acceptChildren(this)
                    functions.removeLast()
                    return
                }
                if (element is FirVariableAssignment) {
                    val target = element.lValue as? FirQualifiedAccessExpression
                    val symbol = target?.calleeReference?.toResolvedCallableSymbol() as? FirPropertySymbol
                    if (symbol != null && symbol.isLocal) assignments.getOrPut(symbol) { ArrayList() } += element
                }
                if (element is FirFunctionCall) {
                    val name = element.calleeReference.name
                    recordScopeLambda(element, aliases)
                    if (name == acquire || name == release) {
                        val candidate = candidate(element, functions.last(), implicit = name == release)
                        if (candidate != null && isWakeLockReceiver(candidate, session)) {
                            // An acquire inside a nested named function belongs
                            // to that function's own check; a release anywhere
                            // in the body counts, as Go walks the whole body.
                            if (name == release) {
                                releases += candidate
                            } else if (candidate.function === declaration) {
                                acquires += candidate
                            }
                        }
                    }
                }
                element.acceptChildren(this)
            }
        })
        if (acquires.isEmpty()) return
        val bindings = Bindings(aliases, assignments)
        val released = releases.flatMapTo(HashSet()) { names(it, bindings) }
        for (acquired in acquires) {
            if (names(acquired, bindings).none { it in released }) report(acquired.anchor, MESSAGE)
        }
    }

    private fun recordScopeLambda(call: FirFunctionCall, aliases: MutableMap<FirBasedSymbol<*>, FirExpression>) {
        val name = call.calleeReference.name
        if (name !in lambdaScopeFunctions && name !in receiverScopeFunctions) return
        if (!isStdlib(call)) return
        val arguments = call.argumentList.arguments.map(::unwrapArgument)
        // `with(lock) { ... }` takes its receiver as the first argument.
        val subject = call.explicitReceiver ?: arguments.firstOrNull()?.takeIf { name == with } ?: return
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

    // Go takes an acquire or release only with a written receiver. A release
    // on an implicit receiver (`with(lock) { release() }`, `release()` in a
    // WakeLock extension) also counts here, when [implicit] is set.
    private fun candidate(call: FirFunctionCall, function: FirNamedFunction, implicit: Boolean): Candidate? {
        val source = call.source ?: return null
        // A safe call's selector carries the whole `r?.f()` as a desugared
        // source; any other fake source is not a call in the code.
        if (source.kind !is KtRealSourceElementKind && source.kind != KtFakeSourceElementKind.DesugaredSafeCallExpression) {
            return null
        }
        val explicit = call.explicitReceiver
        if (explicit == null) {
            if (!implicit) return null
            val receiver = listOfNotNull(call.dispatchReceiver, call.extensionReceiver)
                .firstOrNull { it is FirThisReceiverExpression && it.isImplicit }
                ?: return null
            return Candidate(call, source, null, receiver, function)
        }
        val qualified = qualifiedCall(source) ?: return null
        val written = significantChildren(source, qualified).firstOrNull() ?: return null
        return Candidate(call, lightSourceOf(qualified, source), written, explicit, function)
    }

    // The qualified expression (`r.f()` / `r?.f()`) whose selector is this
    // call: the call's own source (a dot call's, or a safe call's desugared
    // one), or the parent of a selector call expression.
    private fun qualifiedCall(source: KtSourceElement): LighterASTNode? {
        val node = source.lighterASTNode
        if (node.tokenType in qualifiedTypes) return node
        if (node.tokenType != KtNodeTypes.CALL_EXPRESSION) return null
        val parent = source.treeStructure.getParent(node) ?: return null
        if (parent.tokenType !in qualifiedTypes) return null
        val parts = significantChildren(source, parent)
        return parent.takeIf { parts.size > 1 && parts.last() == node }
    }

    // The names a receiver goes by: its own last identifier as Go reads it,
    // then the variables it may alias, and for `this`, the declaration it is
    // bound to. Empty names never match.
    private fun names(candidate: Candidate, bindings: Bindings): Set<String> {
        val out = HashSet<String>()
        candidate.written?.let { referenceName(candidate.anchor, it) }?.takeIf { it.isNotEmpty() }?.let(out::add)
        val queue = ArrayDeque(listOf(candidate.receiver))
        val seen = HashSet<FirExpression>()
        var steps = 0
        while (queue.isNotEmpty() && steps++ < MAX_ALIAS_STEPS) {
            val current = queue.removeFirst()
            thisName(current, bindings)?.let(out::add)
            for (target in aliasTargets(current, bindings)) {
                if (!seen.add(target)) continue
                val source = target.source ?: continue
                referenceName(source, source.lighterASTNode).takeIf { it.isNotEmpty() }?.let(out::add)
                queue += target
            }
        }
        return out
    }

    // A `this` bound to a declaration (a class, or a function's extension
    // receiver) is named by that declaration; one bound to a scope lambda is
    // an alias instead.
    private fun thisName(expression: FirExpression, bindings: Bindings): String? {
        val receiver = unwrap(expression) as? FirThisReceiverExpression ?: return null
        val bound = receiver.calleeReference.boundSymbol ?: return null
        if (bound in bindings.scopes) return null
        return "this@" + System.identityHashCode(bound)
    }

    // The variable an expression aliases: the value of a local variable where
    // it is read (its last assignment before the read, else its initializer;
    // the left side of an elvis), or the receiver of the scope function whose
    // lambda parameter or receiver it is. Only a variable counts, not a call,
    // whose every evaluation may return a different lock.
    private fun aliasTargets(expression: FirExpression, bindings: Bindings): List<FirExpression> {
        val access = unwrap(expression) as? FirQualifiedAccessExpression ?: return emptyList()
        if (access is FirFunctionCall) return emptyList()
        val value = if (access is FirThisReceiverExpression) {
            access.calleeReference.boundSymbol?.let(bindings.scopes::get)
        } else {
            when (val symbol = access.calleeReference.toResolvedCallableSymbol()) {
                is FirPropertySymbol -> if (symbol.isLocal) valueAt(symbol, access, bindings) else null
                is FirValueParameterSymbol -> bindings.scopes[symbol]
                else -> null
            }
        }
        return listOfNotNull(value?.let(::variableOf))
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
            target = unwrap(target)
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

    private fun unwrapArgument(argument: FirExpression): FirExpression =
        if (argument is FirWrappedArgumentExpression) unwrapArgument(argument.expression) else argument

    // The last identifier of a receiver as Go reads it: a name, the selector
    // of a qualified expression, or a call's callee. This checker also reads
    // through `!!` and parentheses, where Go reads no name. Empty otherwise
    // (`this`, an index, a cast, a literal), which never matches a release.
    private fun referenceName(anchor: KtSourceElement, node: LighterASTNode): String {
        val children = significantChildren(anchor, node)
        return when (node.tokenType) {
            KtNodeTypes.REFERENCE_EXPRESSION -> anchor.treeStructure.toString(node).toString()
            KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION ->
                children.lastOrNull()?.takeIf { children.size > 1 }?.let { referenceName(anchor, it) }.orEmpty()
            KtNodeTypes.CALL_EXPRESSION -> children.firstOrNull()
                ?.takeIf { it.tokenType == KtNodeTypes.REFERENCE_EXPRESSION || it.tokenType == KtNodeTypes.CALL_EXPRESSION }
                ?.let { referenceName(anchor, it) }
                .orEmpty()
            KtNodeTypes.POSTFIX_EXPRESSION -> {
                val notNull = lightChildren(anchor, node).any { child ->
                    child.tokenType == KtNodeTypes.OPERATION_REFERENCE &&
                        lightChildren(anchor, child).any { it.tokenType == KtTokens.EXCLEXCL }
                }
                if (notNull) children.firstOrNull()?.let { referenceName(anchor, it) }.orEmpty() else ""
            }
            KtNodeTypes.PARENTHESIZED -> children.firstOrNull()?.let { referenceName(anchor, it) }.orEmpty()
            else -> ""
        }
    }

    private fun significantChildren(anchor: KtSourceElement, node: LighterASTNode): List<LighterASTNode> =
        lightChildren(anchor, node).filter {
            it.tokenType != KtTokens.WHITE_SPACE &&
                it.tokenType !in KtTokens.COMMENTS &&
                it.tokenType != KtTokens.DOT &&
                it.tokenType != KtTokens.SAFE_ACCESS &&
                it.tokenType != KtTokens.LPAR &&
                it.tokenType != KtTokens.RPAR &&
                it.tokenType != KtNodeTypes.OPERATION_REFERENCE
        }

    private fun isWakeLockReceiver(candidate: Candidate, session: FirSession): Boolean {
        // The receiver's type as used here, smart casts included.
        val receiver = candidate.receiver
        val type = receiver.resolvedType.fullyExpandedType(session).lowerBoundIfFlexible()
        val symbol = type.toClassLikeSymbol(session)
        if (symbol != null && isWakeLockClass(symbol, session)) return true
        return declaredFromNewWakeLock(unwrap(receiver), candidate)
    }

    // The class itself or a supertype named WakeLock, read from the class's
    // own lookup tags, which are bound to local and anonymous classes, so no
    // class id is resolved from a symbol that may be local.
    private fun isWakeLockClass(symbol: FirClassLikeSymbol<*>, session: FirSession): Boolean =
        symbol.classId.shortClassName == wakeLock ||
            lookupSuperTypes(symbol, lookupInterfaces = true, deep = true, useSiteSession = session)
                .any { it.lookupTag.classId.shortClassName == wakeLock }

    // Go's fallback when it cannot type the receiver: a variable declared in
    // the call's nearest named function, before the call, whose initializer
    // is a call written newWakeLock. Only the declaration the receiver
    // resolves to counts.
    private fun declaredFromNewWakeLock(receiver: FirExpression, candidate: Candidate): Boolean {
        val access = receiver as? FirQualifiedAccessExpression ?: return false
        if (access is FirFunctionCall) return false
        val property = access.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol ?: return false
        val declared = property.source ?: return false
        val function = candidate.function.source ?: return false
        if (declared.startOffset < function.startOffset || declared.endOffset > function.endOffset) return false
        if (declared.startOffset >= candidate.anchor.startOffset) return false
        var initializer = property.resolvedInitializer ?: return false
        if (initializer is FirSafeCallExpression) initializer = initializer.selector as? FirExpression ?: return false
        return initializer is FirFunctionCall && initializer.calleeReference.name == newWakeLock
    }

    private tailrec fun unwrap(expression: FirExpression): FirExpression = when (expression) {
        is FirCheckedSafeCallSubject -> unwrap(expression.originalReceiverRef.value)
        is FirSmartCastExpression -> unwrap(expression.originalExpression)
        is FirCheckNotNullCall -> unwrap(expression.argumentList.arguments.first())
        else -> expression
    }
}
