package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.checkers.followableInitializer
import dev.jasonpearson.krit.fir.checkers.initializerInCheckedFile
import dev.jasonpearson.krit.fir.checkers.security.assignedLocalValues
import dev.jasonpearson.krit.fir.checkers.security.isLocalVar
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.assignedAccess
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.expandedClassType
import dev.jasonpearson.krit.fir.support.isOrInheritsFrom
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.pathTo
import dev.jasonpearson.krit.fir.support.qualifiedCall
import dev.jasonpearson.krit.fir.support.unwrapArgument
import dev.jasonpearson.krit.fir.support.unwrapSmartCasts
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirLoop
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFieldSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFileSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds

/**
 * Flags two insecure-randomness shapes, with the Go rule's two messages:
 * - a `java.util.Random(...)` constructor call, with any arguments;
 * - a `SecureRandom.setSeed(seed)` call whose single argument is a fixed
 *   seed or `System.currentTimeMillis()` / `System.nanoTime()`.
 *
 * Like the Go rule:
 * - only a constructor call counts: a superclass delegation
 *   (`object : Random() {}`, `class R : Random()`) and a constructor reference
 *   are not reported, and a subclass constructor (`ThreadLocalRandom`, a
 *   project subclass) is not `java.util.Random`;
 * - `SecureRandom(seed)` is not reported here: that constructor belongs to the
 *   HardcodedRandomSeed rule;
 * - a fixed seed is what Lint's ConstantEvaluator folds, as Go's constant
 *   check reads it: a numeric literal, a constant, a val or a never-reassigned
 *   var holding one, arithmetic and conversions of constants, and an array
 *   built from constants (`byteArrayOf(1, 2)`, `ByteArray(16)`) that no other
 *   use may fill before setSeed reads it. A parameter, a generated seed, or a
 *   reassigned variable is not fixed;
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
 *   `SecureRandom` or whose initializer constructs one. A qualified
 *   `java.lang.System.nanoTime()`, a qualified constant (`Seeds.FIXED`,
 *   `java.lang.Long.MIN_VALUE`), and infix bit operators on constants
 *   (`SEED or 1L`) are time-based / fixed seeds Go does not recognize.
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

    private val randomClassId = classIdOf("java.util", "Random")
    private val secureRandomClassId = classIdOf("java.security", "SecureRandom")
    private val setSeed = Name.identifier("setSeed")
    private val randomSetSeed = CallableId(randomClassId, setSeed)
    private val secureRandomSetSeed = CallableId(secureRandomClassId, setSeed)
    private val systemClassId = classIdOf("java.lang", "System")
    private val timeCalls = setOf(
        CallableId(systemClassId, Name.identifier("currentTimeMillis")),
        CallableId(systemClassId, Name.identifier("nanoTime")),
    )
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        when (val callee = expression.calleeReference.toResolvedCallableSymbol()) {
            is FirConstructorSymbol -> {
                // The return type names the constructed class; for a typealias
                // constructor it is expanded to the aliased class.
                val constructed = callee.resolvedReturnType.expandedClassType() ?: return
                if (constructed.lookupTag.classId != randomClassId) return
                report(expression.source, RANDOM_MESSAGE)
            }
            is FirNamedFunctionSymbol -> {
                if (callee.name != setSeed || callee.receiverParameterSymbol != null) return
                val argument = expression.argumentList.arguments.singleOrNull() ?: return
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
                if (!isTimeSeed(argument) && !ConstantSeed().isConstant(argument, depth = 0)) return
                val source = expression.source ?: return
                report(qualifiedCall(source)?.let { lightSourceOf(it, source) } ?: source, SET_SEED_MESSAGE)
            }
            else -> return
        }
    }

    // A no-argument System.currentTimeMillis() / System.nanoTime() call.
    private fun isTimeSeed(argument: FirExpression): Boolean {
        val call = unwrapArgument(argument) as? FirFunctionCall ?: return false
        return call.argumentList.arguments.isEmpty() && call.calleeReference.toResolvedCallableSymbol()?.callableId in timeCalls
    }

    // Go's secureRandomIsConstantSeed, after Lint's ConstantEvaluator: a numeric or char literal, a constant, a val
    // or never-reassigned var whose initializer is one, arithmetic, bit operators and conversions of constants, and
    // an array built from constants (`byteArrayOf(1, 2)`, `ByteArray(16)`) that nothing may fill before the read.
    private class ConstantSeed {
        private val following = HashSet<FirCallableSymbol<*>>()

        context(context: CheckerContext)
        fun isConstant(expression: FirExpression, depth: Int): Boolean {
            if (depth > MAX_DEPTH) return false
            return when (val value = unwrapSmartCasts(unwrapArgument(expression))) {
                is FirLiteralExpression -> value.value.let { it is Number || it is Char }
                is FirFunctionCall -> callIsConstant(value, depth)
                is FirQualifiedAccessExpression -> readIsConstant(value, depth)
                else -> false
            }
        }

        context(context: CheckerContext)
        private fun callIsConstant(call: FirFunctionCall, depth: Int): Boolean {
            val callee = call.calleeReference.toResolvedCallableSymbol() ?: return false
            val arguments = call.argumentList.arguments.flatMap { (it as? FirVarargArgumentsExpression)?.arguments ?: listOf(it) }
            if (callee is FirConstructorSymbol) {
                // `ByteArray(16)` is zero-filled; an init lambda decides the contents.
                val built = callee.resolvedReturnType.expandedClassType()?.lookupTag?.classId
                return built in sizedArrays && arguments.size == 1 && isConstant(arguments[0], depth + 1)
            }
            val callableId = callee.callableId ?: return false
            if (callableId in arrayBuilders) return arguments.all { isConstant(it, depth + 1) }
            if (callableId.classId !in numericClassIds || callableId.callableName.asString() !in foldedOperators) return false
            val receiver = call.explicitReceiver ?: call.dispatchReceiver ?: return false
            return isConstant(receiver, depth + 1) && arguments.all { isConstant(it, depth + 1) }
        }

        @OptIn(SymbolInternals::class)
        context(context: CheckerContext)
        private fun readIsConstant(read: FirQualifiedAccessExpression, depth: Int): Boolean {
            val symbol = read.calleeReference.toResolvedCallableSymbol()?.unwrapFakeOverrides() ?: return false
            if (!following.add(symbol)) return false
            try {
                return when (symbol) {
                    is FirPropertySymbol -> propertyIsConstant(symbol, read, depth)
                    // A Java constant such as java.lang.Long.MAX_VALUE.
                    is FirFieldSymbol -> symbol.fir.isVal && symbol.fir.initializer?.let { isConstant(it, depth + 1) } == true
                    else -> false
                }
            } finally {
                following.remove(symbol)
            }
        }

        context(context: CheckerContext)
        private fun propertyIsConstant(symbol: FirPropertySymbol, read: FirQualifiedAccessExpression, depth: Int): Boolean {
            if (symbol.hasDelegate) return false
            if (symbol.getterSymbol?.isDefault == false) return false
            // A const keeps its value in a class file; another property is followed only in the file under check.
            val initializer = symbol.followableInitializer() ?: return false
            if (!symbol.isVal && isReassigned(symbol)) return false
            if (!isConstant(initializer, depth + 1)) return false
            return !buildsArray(initializer) || !mayBeFilled(symbol, read)
        }

        // Go's lintNameReassigned: a local var assigned anywhere in its function, a member var anywhere in the file.
        @OptIn(SymbolInternals::class)
        context(context: CheckerContext)
        private fun isReassigned(symbol: FirPropertySymbol): Boolean {
            if (symbol.isLocal) {
                val assigned = assignedLocalValues() ?: return true
                return assigned[symbol].orEmpty().isNotEmpty()
            }
            val file = context.containingFileSymbol?.fir ?: return true
            var found = false
            file.accept(object : FirVisitorVoid() {
                override fun visitElement(element: FirElement) {
                    if (found) return
                    if (element is FirVariableAssignment &&
                        assignedAccess(element)?.calleeReference?.toResolvedCallableSymbol()?.unwrapFakeOverrides() == symbol
                    ) {
                        found = true
                        return
                    }
                    element.acceptChildren(this)
                }
            })
            return found
        }

        context(context: CheckerContext)
        private fun buildsArray(initializer: FirExpression): Boolean {
            val call = unwrapArgument(initializer) as? FirFunctionCall ?: return false
            return when (val callee = call.calleeReference.toResolvedCallableSymbol()) {
                is FirConstructorSymbol -> callee.resolvedReturnType.expandedClassType()?.lookupTag?.classId in sizedArrays
                else -> callee?.callableId in arrayBuilders
            }
        }

        // Go's secureRandomArrayMayBeFilled: another use of the array may write it before [read] reads it
        // (`rng.nextBytes(seed)`). For a local, only a use that starts before the read or shares a loop with it
        // counts; setSeed arguments never count, because setSeed only reads the array.
        @OptIn(SymbolInternals::class)
        context(context: CheckerContext)
        private fun mayBeFilled(symbol: FirPropertySymbol, read: FirQualifiedAccessExpression): Boolean {
            val readStart = read.source?.startOffset ?: return true
            val rootSymbol = if (symbol.isLocal) {
                context.containingDeclarations.firstOrNull { it !is FirClassLikeSymbol<*> && it !is FirFileSymbol }
            } else {
                context.containingFileSymbol
            }
            val root: FirElement = rootSymbol?.fir ?: return true
            val loops = if (symbol.isLocal) {
                (pathTo(read) ?: return true).filterIsInstance<FirLoop>().mapNotNull { it.source }
            } else {
                emptyList()
            }
            var filled = false
            root.accept(object : FirVisitorVoid() {
                private val seedArguments = HashSet<FirElement>()

                override fun visitElement(element: FirElement) {
                    if (filled) return
                    if (element is FirFunctionCall && element.calleeReference.name == setSeed) {
                        element.argumentList.arguments.singleOrNull()?.let { seedArguments += unwrapArgument(it) }
                    }
                    if (element is FirQualifiedAccessExpression && element !== read && element !in seedArguments &&
                        element.calleeReference.toResolvedCallableSymbol()?.unwrapFakeOverrides() == symbol
                    ) {
                        val start = element.source?.startOffset
                        if (!symbol.isLocal || start == null || start < readStart ||
                            loops.any { start >= it.startOffset && start < it.endOffset }
                        ) {
                            filled = true
                            return
                        }
                    }
                    element.acceptChildren(this)
                }
            })
            return filled
        }
    }

    private const val MAX_DEPTH = 8
    private val numericClassIds = setOf(
        StandardClassIds.Long,
        StandardClassIds.Int,
        StandardClassIds.Short,
        StandardClassIds.Byte,
        StandardClassIds.Char,
        StandardClassIds.Double,
        StandardClassIds.Float,
    )
    private val foldedOperators = setOf(
        "plus", "minus", "times", "div", "rem", "unaryMinus", "unaryPlus", "inv", "or", "and", "xor", "shl", "shr",
        "ushr", "toLong", "toInt", "toShort", "toByte", "toChar", "toDouble", "toFloat",
    )
    private val arrayBuilders = listOf(
        "byteArrayOf", "shortArrayOf", "intArrayOf", "longArrayOf", "charArrayOf", "floatArrayOf", "doubleArrayOf",
        "booleanArrayOf", "arrayOf",
    ).mapTo(HashSet()) { CallableId(FqName("kotlin"), Name.identifier(it)) }
    private val sizedArrays = listOf(
        "ByteArray", "ShortArray", "IntArray", "LongArray", "CharArray", "FloatArray", "DoubleArray", "BooleanArray",
    ).mapTo(HashSet()) { classIdOf("kotlin", it) }

    // The explicit receiver is a variable whose declared initializer constructs
    // a java.security.SecureRandom (or a subclass), whatever its declared type,
    // seen through a smart cast, a safe call (`r?.setSeed(1L)`) or a not-null
    // assertion (`r!!.setSeed(1L)`). A local var must not be assigned anything
    // else before the call.
    context(context: CheckerContext)
    private fun receiverHoldsSecureRandom(call: FirFunctionCall): Boolean {
        val receiver = unwrapSmartCasts(call.explicitReceiver, throughNotNull = true)
        val access = receiver as? FirPropertyAccessExpression ?: return false
        val property = access.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol ?: return false
        // A property of another file shows no initializer, as in a class file.
        if (!constructsSecureRandom(property.initializerInCheckedFile())) return false
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
        isOrInheritsFrom(symbol, classId, context.session, lookupInterfaces = false)
}
