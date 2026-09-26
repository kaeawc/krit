package dev.jasonpearson.krit.fir.checkers.androidlint

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
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
import org.jetbrains.kotlin.lexer.KtTokens
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
 *   receiver: `r.setSeed(1L)` dispatches to SecureRandom.setSeed;
 * - the finding sits on the first line of the call expression, which for a
 *   qualified call is the receiver's (or the package qualifier's) first line.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Recall: resolution sees java.util.Random through an import alias, a
 *   typealias, a `java.util.*` star import, and in a file that also imports
 *   `kotlin.random.Random`; Go needs the spelling `Random` plus an explicit
 *   `java.util.Random` import, or the literal `java.util.Random` qualifier.
 * - Recall: a setSeed receiver is proved by its type, so a SecureRandom
 *   parameter, a `SecureRandom.getInstance(...)` result, a member chain
 *   (`this.rng`), a nullable property, a subclass, and an implicit receiver
 *   (`with(rng) { setSeed(1L) }`) are reported. Go only accepts a
 *   SecureRandom constructor call or a name declared by a property whose type
 *   is spelled `SecureRandom` or whose initializer constructs one. A negative
 *   literal seed (`-1L`) and a qualified `java.lang.System.nanoTime()` are
 *   fixed / time-based seeds Go does not recognize.
 * - Precision: Go matches the receiver name against every property in the
 *   file, so a `java.util.Random` local that shares a SecureRandom property's
 *   name is reported; a class named `Random` / `SecureRandom` or a local
 *   `System` object shadowing the JDK one is reported by Go from spelling
 *   alone. None of them is the JDK class, so FIR does not report them.
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
    private val systemClassId = ClassId(FqName("java.lang"), Name.identifier("System"))
    private val timeCalls = setOf(
        CallableId(systemClassId, Name.identifier("currentTimeMillis")),
        CallableId(systemClassId, Name.identifier("nanoTime")),
    )
    private val signCalls = listOf(StandardClassIds.Long, StandardClassIds.Int).flatMapTo(HashSet()) { owner ->
        listOf("unaryMinus", "unaryPlus").map { CallableId(owner, Name.identifier(it)) }
    }
    private val qualifiedTypes = setOf(KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION)

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
                // A member of java.util.Random or a subtype: SecureRandom
                // overrides setSeed, so its own member is resolved on a
                // SecureRandom receiver, and a Random-typed receiver still
                // counts when its declaration constructs a SecureRandom.
                val owner = callee.getContainingClassSymbol() ?: return
                if (!isSubclassOf(owner, secureRandomClassId) && !(
                        isSubclassOf(owner, randomClassId) && receiverConstructsSecureRandom(expression.explicitReceiver)
                        )
                ) {
                    return
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

    // The receiver is a variable whose declared initializer constructs a
    // java.security.SecureRandom (or a subclass), whatever its declared type.
    context(context: CheckerContext)
    private fun receiverConstructsSecureRandom(receiver: FirExpression?): Boolean {
        var current = receiver
        while (current is FirSmartCastExpression) current = current.originalExpression
        val access = current as? FirPropertyAccessExpression ?: return false
        val property = access.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol ?: return false
        val initializer = property.resolvedInitializer as? FirFunctionCall ?: return false
        val constructor = initializer.calleeReference.toResolvedCallableSymbol() as? FirConstructorSymbol
            ?: return false
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

    // The qualified expression (`r.f()` / `r?.f()`) whose selector is this
    // call. K2 gives a dot call the whole qualified expression as its source;
    // a safe call keeps the selector call expression, so step up to its parent.
    private fun qualifiedCall(source: KtSourceElement): LighterASTNode? {
        val node = source.lighterASTNode
        if (node.tokenType in qualifiedTypes) return node
        if (node.tokenType != KtNodeTypes.CALL_EXPRESSION) return null
        val parent = source.treeStructure.getParent(node) ?: return null
        if (parent.tokenType !in qualifiedTypes) return null
        val parts = lightChildren(source, parent).filter {
            it.tokenType != KtTokens.WHITE_SPACE &&
                it.tokenType !in KtTokens.COMMENTS &&
                it.tokenType != KtTokens.DOT &&
                it.tokenType != KtTokens.SAFE_ACCESS
        }
        return parent.takeIf { parts.size > 1 && parts.last() == node }
    }
}
