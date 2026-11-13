package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.oracle.calleeAnnotationFqns
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.unwrapArgument
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.text
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirUnusedCheckerBase
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirImplicitInvokeCall
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirStatement
import org.jetbrains.kotlin.fir.expressions.FirTryExpression
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid

// Flags a call statement whose result is discarded although the result is what the call is for: a callee
// annotated @CheckReturnValue or @CheckResult, or a stdlib call that builds a lazy Sequence, Flow or Stream
// (or a function value) and does nothing until the result is used.
//
// Go reads the callee's annotations and return type from the oracle and decides "discarded" from the parse
// tree around the call, then drops findings with a set of text heuristics (builder callbacks, test setup
// names, emitter receivers, Result handlers). This checker takes the same two pieces of evidence from the
// compiler. "Discarded" is the compiler's own unused-value analysis (the statement position of a block, an
// `if`/`when`/`try` branch or a loop body whose value nobody reads), so a last statement of a lambda counts as
// used, like Go. The heuristics that only exist to correct a guessed receiver or return type are not needed
// when the callee and type are resolved.
//
// Kept from Go because they express intent: the in-place mutator names, the `ignoreFunctionCall` option,
// assertion-library calls, `verify(...)` chains, a `subscribe` with handlers, `subscribeWith`, a lambda with a
// non-local `return`, a scope function, a collection mutator, and every option of the rule. A `Unit` or
// `Nothing` result is skipped.
internal object IgnoredReturnValue : FirRule {
    override val ruleId = "IgnoredReturnValue"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val basicDeclarationCheckers = setOf(Checker())
    }

    // The defaults of config/default-krit.yml, used when the check request carries no options.
    private val defaultAnnotations = listOf("CheckResult", "*.CheckResult", "CheckReturnValue", "*.CheckReturnValue")
    private val defaultIgnoreAnnotations = listOf("CanIgnoreReturnValue", "*.CanIgnoreReturnValue")
    private val defaultTypes = listOf(
        "kotlin.Function*", "kotlin.sequences.Sequence", "kotlinx.coroutines.flow.*Flow", "java.util.stream.*Stream",
    )

    // Always must-use, whatever the configured types are.
    private val alwaysMustUse = setOf(
        "kotlin.sequences.Sequence",
        "kotlinx.coroutines.flow.Flow",
        "kotlinx.coroutines.flow.StateFlow",
        "kotlinx.coroutines.flow.SharedFlow",
        "kotlinx.coroutines.flow.MutableStateFlow",
        "kotlinx.coroutines.flow.MutableSharedFlow",
    )
    private val builtinTypePatterns = defaultTypes

    private val checkAnnotations = setOf(
        "CheckReturnValue",
        "CheckResult",
        "com.google.errorprone.annotations.CheckReturnValue",
        "androidx.annotation.CheckResult",
        "javax.annotation.CheckReturnValue",
    )
    private val ignoreAnnotations = setOf("CanIgnoreReturnValue", "com.google.errorprone.annotations.CanIgnoreReturnValue")

    private val inPlaceMutations = setOf("clear", "dispose", "fill", "reverse", "shuffle", "sort")
    private val scopeFunctions = setOf("let", "run", "with", "apply", "also", "use")
    private val verificationCalls = setOf("verify", "coVerify", "verifyBlocking")
    private val testDisposableCalls = setOf("test", "offer", "subscribe")
    private val assertionPrefixes = listOf(
        "kotlin.test.", "org.junit.", "junit.framework.", "com.google.common.truth.", "org.assertj.",
        "org.hamcrest.", "app.cash.turbine.",
    )

    // Stdlib and platform calls whose result is routinely ignored on purpose.
    private val collectionMutators = setOf(
        "remove", "removeAt", "removeFirst", "removeLast", "removeAll", "removeIf", "retainAll", "emit", "send", "add",
        "addAll", "addFirst", "addLast", "put", "putAll", "putIfAbsent", "set", "clear", "offer", "offerFirst",
        "offerLast", "poll", "pollFirst", "pollLast", "pop", "push", "getOrPut", "compute", "computeIfAbsent",
        "computeIfPresent", "replace", "replaceAll", "trySend", "tryEmit", "tryReceive",
    )
    private val purePackages = listOf("kotlin.", "kotlinx.coroutines.", "java.util.")

    // The compiler's unused-value walk, asked to report each discarded call to [evaluate]. A block that the
    // compiler coerces to Unit (a callback lambda) marks every statement of it as unused, its last one too.
    // Go reads the last statement of a lambda as the lambda's value, so that one is kept out, along with the
    // last statements of the `if`, `when` and `try` branches it ends with.
    private class Checker : FirUnusedCheckerBase() {
        context(context: CheckerContext)
        override fun isEnabled(): Boolean = true

        context(context: CheckerContext, reporter: DiagnosticReporter)
        protected override fun createVisitor(): UsageVisitorBase = object : UsageVisitorBase(context, reporter) {
            private val tailBlocks = HashSet<FirBlock>()
            private val tails = HashSet<FirStatement>()

            override fun visitAnonymousFunction(anonymousFunction: FirAnonymousFunction, data: UsageState) {
                if (anonymousFunction.isLambda) anonymousFunction.body?.let { tailBlocks += it }
                super.visitAnonymousFunction(anonymousFunction, data)
            }

            override fun visitBlock(block: FirBlock, data: UsageState) {
                if (block in tailBlocks) block.statements.lastOrNull()?.let { tails += it }
                super.visitBlock(block, data)
            }

            override fun visitWhenExpression(whenExpression: FirWhenExpression, data: UsageState) {
                if (whenExpression in tails) whenExpression.branches.mapTo(tailBlocks) { it.result }
                super.visitWhenExpression(whenExpression, data)
            }

            override fun visitTryExpression(tryExpression: FirTryExpression, data: UsageState) {
                if (tryExpression in tails) {
                    tailBlocks += tryExpression.tryBlock
                    tryExpression.catches.mapTo(tailBlocks) { it.block }
                }
                super.visitTryExpression(tryExpression, data)
            }

            override fun checkExpression(expression: FirExpression, state: UsageState) {
                val discarded = state == UsageState.Unused || (state == UsageState.UnusedFromCoercion && expression !in tails)
                if (!discarded) return
                val call = expression as? FirFunctionCall
                    ?: (expression as? FirSafeCallExpression)?.selector as? FirFunctionCall
                    ?: return
                evaluate(call, expression)
            }
        }
    }

    private class Call(
        val call: FirFunctionCall,
        val statement: FirExpression,
        val name: String,
        val target: String,
        val annotations: List<String>,
        val type: ConeClassLikeType?,
    ) {
        val typeFqn: String? get() = type?.lookupTag?.classId?.asFqNameString()
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    private fun evaluate(call: FirFunctionCall, statement: FirExpression) {
        if (call is FirImplicitInvokeCall) return
        val name = call.calleeReference.name.asString()
        if (name in inPlaceMutations || name in stringList("ignoreFunctionCall", emptyList())) return
        val callee = call.calleeReference.toResolvedCallableSymbol() ?: return
        val target = callee.callableId?.asSingleFqName()?.asString().orEmpty()
        if (isAssertion(target, name)) return
        val resolved = call.resolvedType.fullyExpandedType().lowerBoundIfFlexible()
        if (isUnitOrNothing(resolved)) return
        val type = resolved as? ConeClassLikeType
        if (inVerification(statement) || returnsFromEnclosingFunction(statement)) return
        if (name == "subscribeWith" && call.argumentList.arguments.isNotEmpty()) return
        val info = Call(call, statement, name, target, calleeAnnotationFqns(callee, context.session), type)
        if (isSubscribeWithHandlers(info)) return
        reportEvidence(info)
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    private fun reportEvidence(info: Call) {
        val ignoreAnnotationList = stringList("ignoreReturnValueAnnotations", defaultIgnoreAnnotations)
        val checkAnnotationList = stringList("returnValueAnnotations", defaultAnnotations)
        if (hasAnnotation(info.annotations, ignoreAnnotations, ignoreAnnotationList)) return
        val annotated = info.annotations.firstOrNull { hasAnnotation(listOf(it), checkAnnotations, checkAnnotationList) }
        val anchor = anchor(info)
        if (annotated != null) {
            if (isInTestFile() && info.name in testDisposableCalls) return
            report(anchor, annotatedMessage(info.name, annotated))
            return
        }
        if (info.name in scopeFunctions) return
        val type = info.type ?: return
        if (!typeMatches(type) || functionTypeFromInvoke(info) || !isKnownPure(info)) return
        report(anchor, "Return value of '${info.name}' is ignored. '${info.name}' is a pure stdlib function that returns ${info.typeFqn}.")
    }

    // Go reports on the callee name, except that a call whose chain starts on an earlier line keeps that line.
    private fun anchor(info: Call): KtSourceElement? {
        val name = info.call.calleeReference.source ?: return info.statement.source
        val start = info.statement.source ?: return name
        val before = start.text?.take(name.startOffset - start.startOffset) ?: return name
        return if ('\n' !in before) name else rootReceiver(info.statement)?.source ?: start
    }

    // The leftmost expression of a call chain: the one that starts the statement.
    private fun rootReceiver(statement: FirExpression): FirExpression? {
        var current: FirExpression = statement
        while (true) {
            current = when (current) {
                is FirSafeCallExpression -> current.receiver
                is FirQualifiedAccessExpression -> current.explicitReceiver ?: return current
                else -> return current
            }
        }
    }

    private fun annotatedMessage(name: String, annotation: String): String {
        val simple = annotation.substringAfterLast('.')
        val message = "Return value of '$name' is ignored. The function is annotated with @$simple ($annotation)."
        return if (name == "offer" && annotation.startsWith("io.reactivex.")) {
            "$message Check the result, or call onNext() when delivery does not need to be confirmed."
        } else {
            message
        }
    }

    // ---- Evidence ----

    private fun hasAnnotation(annotations: List<String>, builtin: Set<String>, patterns: List<String>): Boolean =
        annotations.any { annotation ->
            annotation in builtin ||
                (annotation.contains('.') && annotation.substringAfterLast('.') in builtin) ||
                patterns.any { matchesAnnotationPattern(it, annotation) }
        }

    private fun matchesAnnotationPattern(pattern: String, annotation: String): Boolean = when {
        pattern == annotation -> true
        pattern.startsWith("*") -> annotation.endsWith(pattern.substring(1))
        else -> false
    }

    private fun isUnitOrNothing(type: ConeKotlinType): Boolean {
        val id = (type as? ConeClassLikeType)?.lookupTag?.classId?.asFqNameString()
        return id == "kotlin.Unit" || id == "kotlin.Nothing"
    }

    private fun typeMatches(type: ConeClassLikeType): Boolean {
        val fqn = type.lookupTag.classId.asFqNameString()
        val restrict = config()["restrictToConfig"] as? Boolean ?: true
        val configured = stringList("returnValueTypes", defaultTypes)
        if (isFunctionType(type)) return !restrict || matchesType("kotlin.Function", configured)
        val patterns = if (restrict) configured else builtinTypePatterns + configured
        return matchesType(fqn, patterns)
    }

    private fun isFunctionType(type: ConeClassLikeType): Boolean {
        val classId = type.lookupTag.classId
        return classId.asFqNameString().startsWith("kotlin.Function") || classId.shortClassName.asString().startsWith("Function")
    }

    private fun matchesType(fqn: String, patterns: List<String>): Boolean {
        if (fqn in alwaysMustUse) return true
        return patterns.any { raw ->
            val pattern = raw.trim()
            val star = pattern.indexOf('*')
            pattern == fqn ||
                (star >= 0 && fqn.length >= star + (pattern.length - star - 1) &&
                    fqn.startsWith(pattern.substring(0, star)) && fqn.endsWith(pattern.substring(star + 1)))
        }
    }

    // An `invoke` on a function value yields the function's result, not a function.
    private fun functionTypeFromInvoke(info: Call): Boolean {
        val type = info.type ?: return false
        if (!isFunctionType(type)) return false
        return info.name == "invoke" || info.call.explicitReceiver != null
    }

    // A project function that returns a Flow can register state eagerly, so only stdlib targets count.
    private fun isKnownPure(info: Call): Boolean {
        if (info.name in collectionMutators) return false
        val target = info.target
        if (target.startsWith("kotlin.Function") || target.startsWith("kotlin.coroutines.SuspendFunction")) return false
        return purePackages.any { target.startsWith(it) }
    }

    private fun isAssertion(target: String, name: String): Boolean =
        assertionPrefixes.any { target.startsWith(it) } ||
            (name.length > "assert".length && name.startsWith("assert") && name["assert".length] in 'A'..'Z')

    // `subscribe(onNext)` returns a Disposable that fire-and-forget callers drop on purpose.
    private fun isSubscribeWithHandlers(info: Call): Boolean {
        if (info.name != "subscribe" || info.typeFqn?.substringAfterLast('.') != "Disposable") return false
        var found = false
        info.call.argumentList.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirAnonymousFunctionExpression) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found || info.call.argumentList.arguments.any { unwrapArgument(it) is FirAnonymousFunctionExpression }
    }

    // `verify(mock).call()` checks that the call happened; its result is not the point.
    private fun inVerification(statement: FirExpression): Boolean = anyCall(statement) { it.calleeReference.name.asString() in verificationCalls }

    // A lambda that returns from the enclosing function means the call may never complete. A named function
    // inside the call owns its returns, so the walk does not enter it.
    private fun returnsFromEnclosingFunction(statement: FirExpression): Boolean {
        var found = false
        statement.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (!found) element.acceptChildren(this)
            }

            override fun visitNamedFunction(namedFunction: FirNamedFunction) {}

            override fun visitReturnExpression(returnExpression: FirReturnExpression) {
                if (returnExpression.target.labeledElement is FirNamedFunction) found = true
                visitElement(returnExpression)
            }
        })
        return found
    }

    private fun anyCall(root: FirElement, predicate: (FirFunctionCall) -> Boolean): Boolean {
        var found = false
        root.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (!found) element.acceptChildren(this)
            }

            override fun visitFunctionCall(functionCall: FirFunctionCall) {
                if (predicate(functionCall)) found = true else visitElement(functionCall)
            }
        })
        return found
    }

    private fun stringList(key: String, default: List<String>): List<String> =
        (config()[key] as? List<*>)?.mapNotNull { it as? String } ?: default
}
