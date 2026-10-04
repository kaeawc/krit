package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.checkers.security.assignedLocalValues
import dev.jasonpearson.krit.fir.checkers.security.isLocalVar
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.qualifiedCall
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.argument
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds

/**
 * Flags two insecure-randomness shapes, with the Go rule's two messages:
 * - a `java.util.Random(...)` constructor call, with any arguments;
 * - a `SecureRandom.setSeed(seed)` call whose single argument is a fixed
 *   integer literal or `System.currentTimeMillis()` / `System.nanoTime()`.
 *
 * Like the Go rule:
 * - only a constructor call counts: a superclass delegation
 *   (`object : Random() {}`, `class R : Random()`) and a constructor reference
 *   are not reported, and a subclass constructor (`ThreadLocalRandom`, a
 *   project subclass) is not `java.util.Random`;
 * - `SecureRandom(seed)` is not reported here: that constructor belongs to the
 *   TrulyRandom rule;
 * - `setSeed(bytes)` and a seed from a variable or parameter are not reported;
 * - a `java.util.Random`-typed variable whose declaration constructs a
 *   SecureRandom (`val r: Random = SecureRandom()`) counts as a SecureRandom
 *   receiver, also through a safe call (`r?.setSeed(1L)`): `r.setSeed(1L)`
 *   dispatches to SecureRandom.setSeed;
 * - a project overload or override of setSeed (`Sub().setSeed(1L)` where
 *   `Sub` overrides it) is not reported: only the JDK's own setSeed is known
 *   to seed the generator;
 * - the finding sits on the first line of the call expression, which for a
 *   qualified call is the receiver's (or the package qualifier's) first line.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Recall: resolution sees java.util.Random through an import alias, a
 *   typealias, a `java.util.*` star import, and in a file that also imports
 *   `kotlin.random.Random`; Go needs the spelling `Random` plus an explicit
 *   `java.util.Random` import, or the literal `java.util.Random` qualifier.
 * - Recall: a setSeed receiver is proved by its type, so a SecureRandom
 *   parameter, a `SecureRandom.getInstance(...)` result, a lazy delegate, a
 *   member chain (`holder.rng`, `anon.inner`), a nullable property, a
 *   not-null assertion (`rng!!.setSeed(1L)`), a smart cast from
 *   java.util.Random (`if (r is SecureRandom) r.setSeed(1L)`), a subclass
 *   that does not override setSeed, `super.setSeed` / `this.setSeed` in a
 *   subclass, and an implicit receiver (`with(rng) { setSeed(1L) }`, a
 *   subclass's own `init { setSeed(1L) }`) are reported. A Random-typed
 *   variable whose initializer constructs a SecureRandom through a typealias
 *   or a local subclass is reported too. Each is still a deterministic seed on
 *   a SecureRandom, which is what the message says and what the rule is for.
 *   Go only accepts an explicit receiver that is a SecureRandom constructor
 *   call or a name declared by a property whose type is spelled
 *   `SecureRandom` or whose initializer constructs one. A negative literal
 *   seed (`-1L`) and a qualified `java.lang.System.nanoTime()` are fixed /
 *   time-based seeds Go does not recognize.
 * - Precision: Go matches the receiver name against every property in the
 *   file, so a `java.util.Random` local that shares a SecureRandom property's
 *   name is reported; a class named `Random` / `SecureRandom`, a local lambda
 *   named `Random`, or a local `System` object shadowing the JDK one is
 *   reported by Go from spelling alone. None of them is the JDK class, so FIR
 *   does not report them. A local var reassigned to a non-SecureRandom value
 *   before the call no longer holds the SecureRandom its declaration
 *   constructed, so FIR does not report it either.
 */
internal object SecureRandom : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "SecureRandom"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(SecureRandom)
    }

    private const val RANDOM_MESSAGE =
        "Using java.util.Random. Use java.security.SecureRandom for security-sensitive operations."
    private const val SET_SEED_MESSAGE =
        "Calling SecureRandom.setSeed() with a fixed or time-based seed makes output predictable. " +
            "Use the default SecureRandom seeding."

    private val randomClassId = ClassId(FqName("java.util"), Name.identifier("Random"))
    private val secureRandomClassId = ClassId(FqName("java.security"), Name.identifier("SecureRandom"))
    private val setSeed = Name.identifier("setSeed")
    private val randomSetSeed = CallableId(randomClassId, setSeed)
    private val secureRandomSetSeed = CallableId(secureRandomClassId, setSeed)
    private val systemClassId = ClassId(FqName("java.lang"), Name.identifier("System"))
    private val timeCalls = setOf(
        CallableId(systemClassId, Name.identifier("currentTimeMillis")),
        CallableId(systemClassId, Name.identifier("nanoTime")),
    )
    private val signCalls = listOf(StandardClassIds.Long, StandardClassIds.Int).flatMapTo(HashSet()) { owner ->
        listOf("unaryMinus", "unaryPlus").map { CallableId(owner, Name.identifier(it)) }
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        when (val callee = expression.calleeReference.toResolvedCallableSymbol()) {
            is FirConstructorSymbol -> {
                // The return type names the constructed class; for a typealias
                // constructor it is expanded to the aliased class.
                val constructed = callee.resolvedReturnType.fullyExpandedType().lowerBoundIfFlexible()
                    as? ConeClassLikeType ?: return
                if (constructed.lookupTag.classId != randomClassId) return
                report(expression.source, RANDOM_MESSAGE)
            }
            is FirNamedFunctionSymbol -> {
                if (callee.name != setSeed || callee.receiverParameterSymbol != null) return
                val argument = expression.argumentList.arguments.singleOrNull() ?: return
                if (!isDeterministicSeed(argument)) return
                // Only the JDK's own setSeed: SecureRandom overrides it, so its
                // member is resolved on a SecureRandom receiver, and Random's is
                // resolved on a Random-typed variable, which counts when it holds
                // a SecureRandom. A project overload or override is not known to
                // seed the generator.
                when (callee.unwrapFakeOverrides().callableId) {
                    secureRandomSetSeed -> Unit
                    randomSetSeed -> if (!receiverHoldsSecureRandom(expression)) return
                    else -> return
                }
                val source = expression.source ?: return
                report(qualifiedCall(source)?.let { lightSourceOf(it, source) } ?: source, SET_SEED_MESSAGE)
            }
            else -> return
        }
    }

    // A fixed integer literal, optionally signed (K2 keeps `-1L` as a
    // `unaryMinus` call on the literal), or a no-argument
    // System.currentTimeMillis() / System.nanoTime() call.
    private fun isDeterministicSeed(argument: FirExpression): Boolean = when (argument) {
        is FirLiteralExpression -> isIntegerLiteral(argument)
        is FirFunctionCall -> {
            val callableId = argument.calleeReference.toResolvedCallableSymbol()?.callableId
            argument.argumentList.arguments.isEmpty() &&
                (
                    callableId in timeCalls ||
                        callableId in signCalls && (argument.explicitReceiver as? FirLiteralExpression)?.let(::isIntegerLiteral) == true
                    )
        }
        else -> false
    }

    private fun isIntegerLiteral(literal: FirLiteralExpression): Boolean =
        literal.value.let { it is Long || it is Int || it is Short || it is Byte }

    // The explicit receiver is a variable whose declared initializer constructs
    // a java.security.SecureRandom (or a subclass), whatever its declared type,
    // seen through a smart cast, a safe call (`r?.setSeed(1L)`) or a not-null
    // assertion (`r!!.setSeed(1L)`). A local var must not be assigned anything
    // else before the call.
    context(context: CheckerContext)
    private fun receiverHoldsSecureRandom(call: FirFunctionCall): Boolean {
        var current = call.explicitReceiver
        while (true) {
            current = when (current) {
                is FirSmartCastExpression -> current.originalExpression
                is FirCheckedSafeCallSubject -> current.originalReceiverRef.value
                is FirCheckNotNullCall -> current.argument
                else -> break
            }
        }
        val access = current as? FirPropertyAccessExpression ?: return false
        val property = access.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol ?: return false
        if (!constructsSecureRandom(property.resolvedInitializer)) return false
        if (!isLocalVar(property)) return true
        val callStart = call.source?.startOffset ?: return false
        val assigned = assignedLocalValues() ?: return false
        return assigned[property].orEmpty().none { value ->
            val start = value.source?.startOffset ?: return@none true
            start < callStart && !constructsSecureRandom(value)
        }
    }

    context(context: CheckerContext)
    private fun constructsSecureRandom(expression: FirExpression?): Boolean {
        val call = expression as? FirFunctionCall ?: return false
        val constructor = call.calleeReference.toResolvedCallableSymbol() as? FirConstructorSymbol ?: return false
        val constructed = constructor.getContainingClassSymbol() ?: return false
        return isSubclassOf(constructed, secureRandomClassId)
    }

    // The containing class comes from the symbol's lookup tag, which is bound to
    // local and anonymous classes; only class ids already in hand are compared.
    context(context: CheckerContext)
    private fun isSubclassOf(symbol: FirClassLikeSymbol<*>, classId: ClassId): Boolean =
        symbol.classId == classId ||
            lookupSuperTypes(symbol, lookupInterfaces = false, deep = true, useSiteSession = context.session)
                .any { it.lookupTag.classId == classId }
}
