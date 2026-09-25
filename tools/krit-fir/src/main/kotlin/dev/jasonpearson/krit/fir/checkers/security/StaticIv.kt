package dev.jasonpearson.krit.fir.checkers.security

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.utils.isLateInit
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirCallableReferenceAccess
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirIntegerLiteralOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirSpreadArgumentExpression
import org.jetbrains.kotlin.fir.expressions.FirStringConcatenationCall
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.types.type
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds

/**
 * Flags `javax.crypto.spec.IvParameterSpec` and `GCMParameterSpec`
 * constructions whose IV bytes (the first argument of IvParameterSpec, the
 * second of GCMParameterSpec) are written inline as literals. Mirrors the Go
 * StaticIv rule on Kotlin code. The IV argument counts as literal bytes when
 * it is one of Go's literal byte sources:
 *  - `byteArrayOf(...)` (`kotlin.byteArrayOf`) whose elements are all integer
 *    literals, optionally signed; an empty call does not count, as in Go;
 *  - `"<literal>".toByteArray(...)` or `.encodeToByteArray(...)`, or
 *    `"<hex>".hexToByteArray(...)` from `kotlin.text`, on a literal string;
 *  - a Base64 decode of a literal string or of literal bytes: `decode` on any
 *    class named `Base64` (android.util, kotlin.io.encoding, BouncyCastle, a
 *    project codec), or `java.util.Base64.Decoder.decode`.
 * A literal string is a string literal (raw or not), a template whose entries
 * are literals, `const val`s, or properties (val or var, final or open, not
 * lateinit) initialized with a literal string, or a `kotlin.text` call
 * (`trimIndent()`, `replace(" ", "")`) on a literal string with literal
 * arguments.
 *
 * The argument may wrap the source in `!!`, `as`, `?.`, or an Elvis whose left
 * side is literal, and may chain calls on it (`.copyOf(16)`,
 * `+ "x".toByteArray()`, `.also { log(it) }`) as long as no call feeds in
 * runtime bytes: a byte, byte array, or byte collection argument that is not
 * itself literal, a `Random` argument, or a lambda or callable reference that
 * overwrites the bytes (`nextBytes(it)`, `it[0] = random[0]`) or returns them
 * mixed with runtime bytes (`let { xor(it, random) }`). Go takes chains only on
 * a source it matches by text (a string literal followed by `.toByteArray(`,
 * or `Base64.decode(` / `Base64.getDecoder().decode(`); on the other sources
 * FIR takes only kotlin.collections calls without lambdas. The finding is
 * reported on the constructor call, which starts on the line Go reports.
 *
 * Deliberate differences from Go, pinned by golden and probe tests. Go matches
 * the constructor by its simple name plus the file's imports and declarations,
 * and the argument by its source text; FIR reads the resolved call instead:
 *  - Go false negatives FIR reports: an import alias or typealias of the spec
 *    class, and a bare spec call in a file that declares an unrelated nested
 *    class of the same name (StaticIvAlias); `byteArrayOf` elements Go's
 *    number parser rejects, an annotated argument, a copy chained on
 *    `byteArrayOf`, decoder forms Go's text match misses, a decode of literal
 *    bytes, a parenthesized string concatenation, and a String transform
 *    before toByteArray (StaticIvGoMisses).
 *  - Go false positives FIR skips, where the IV is not literal bytes: a string
 *    template with a runtime entry; a chain that mixes in runtime or random
 *    bytes, or whose lambda overwrites them; a decode whose data argument is
 *    not a literal but whose text holds a quote; a literal source nested in
 *    another call's argument (StaticIvPrecision); a same-file
 *    `String.toByteArray` lookalike (StaticIvLookalike); a same-package spec
 *    class that wins over a star import (StaticIvTest).
 */
internal object StaticIv : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "StaticIv"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(StaticIv)
    }

    private const val MESSAGE =
        "IV parameter spec is built from literal bytes. Generate a fresh random IV for each encryption operation."

    private val specPackage = FqName("javax.crypto.spec")
    private val ivParameterSpec = ClassId(specPackage, Name.identifier("IvParameterSpec"))
    private val gcmParameterSpec = ClassId(specPackage, Name.identifier("GCMParameterSpec"))

    private val kotlinPackage = FqName("kotlin")
    private val kotlinText = FqName("kotlin.text")
    private val kotlinCollections = FqName("kotlin.collections")
    private val byteArrayOf = CallableId(kotlinPackage, Name.identifier("byteArrayOf"))
    private val stringBytes = setOf(
        CallableId(kotlinText, Name.identifier("toByteArray")),
        CallableId(kotlinText, Name.identifier("encodeToByteArray")),
        CallableId(kotlinText, Name.identifier("hexToByteArray")),
    )
    private val regex = ClassId(kotlinText, Name.identifier("Regex"))

    private val byteArrays = setOf(
        ClassId(kotlinPackage, Name.identifier("ByteArray")),
        ClassId(kotlinPackage, Name.identifier("UByteArray")),
    )

    private val bytes = setOf(StandardClassIds.Byte, StandardClassIds.UByte)

    private val randoms = setOf(
        ClassId(FqName("java.util"), Name.identifier("Random")),
        ClassId(FqName("java.security"), Name.identifier("SecureRandom")),
        ClassId(FqName("kotlin.random"), Name.identifier("Random")),
        ClassId(FqName("kotlin.random"), FqName("Random.Default"), false),
    )

    private val decode = Name.identifier("decode")
    private val base64 = Name.identifier("Base64")
    private val getDecoder = Name.identifier("getDecoder")
    private val javaDecoder = ClassId(FqName("java.util"), FqName("Base64.Decoder"), false)

    // Scope functions whose result is their receiver, so a lambda passed to
    // them can change the IV only by mutating it.
    private val receiverReturning = setOf("also", "apply", "takeIf", "takeUnless")
        .map { CallableId(kotlinPackage, Name.identifier(it)) }
        .toSet()

    // How Go treats a literal source: TEXT when Go matches it by text and so
    // accepts any chain on it; STRICT when Go misses it, where FIR accepts only
    // kotlin.collections calls without lambdas so it adds no false positive.
    private enum class Root { TEXT, STRICT }

    private fun Root.and(other: Root): Root = if (this == Root.TEXT && other == Root.TEXT) Root.TEXT else Root.STRICT

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirConstructorSymbol ?: return
        // The call's type is the constructed class, also through a typealias.
        val constructed = expression.resolvedType.fullyExpandedType().classId ?: return
        val ivIndex = when (constructed) {
            ivParameterSpec -> 0
            gcmParameterSpec -> 1
            else -> return
        }
        // Java constructors take no named arguments, so the IV is positional.
        if (callee.valueParameterSymbols.size <= ivIndex) return
        val ivArgument = expression.argumentList.arguments.getOrNull(ivIndex) ?: return
        if (literalBytes(ivArgument, emptySet()) == null) return
        report(expression.source, MESSAGE)
    }

    // Strips argument wrappers and value-preserving wrappers: `!!`, `as`,
    // `as?`, and a safe call (its selector, whose receiver is the checked
    // subject standing for the original receiver).
    private fun unwrap(expression: FirExpression): FirExpression {
        var current = expression
        while (true) {
            current = when (current) {
                is FirSpreadArgumentExpression -> return current
                is FirWrappedArgumentExpression -> current.expression
                is FirSmartCastExpression -> current.originalExpression
                is FirCheckNotNullCall -> current.argumentList.arguments.singleOrNull() ?: return current
                is FirTypeOperatorCall -> {
                    if (current.operation != FirOperation.AS && current.operation != FirOperation.SAFE_AS) return current
                    current.argumentList.arguments.singleOrNull() ?: return current
                }
                is FirSafeCallExpression -> current.selector as? FirExpression ?: return current
                is FirCheckedSafeCallSubject -> current.originalReceiverRef.value
                else -> return current
            }
        }
    }

    // A byte array built only from inline literals: a literal byte source, or a
    // chain of calls on one that feeds in no other bytes. [self] holds the
    // parameters and receivers of enclosing chain lambdas, which stand for the
    // literal bytes the lambda was called on. Returns how Go treats the chain,
    // or null when the bytes are not literal.
    private fun literalBytes(expression: FirExpression, self: Set<FirBasedSymbol<*>>): Root? {
        val value = unwrap(expression)
        if (isSelfReference(value, self)) return Root.TEXT
        // `x ?: y` evaluates to `x` whenever `x` is non-null, and a literal
        // source only yields null in a platform type.
        if (value is FirElvisExpression) return literalBytes(value.lhs, self)
        if (value !is FirFunctionCall) return null
        literalSource(value, self)?.let { return it }
        val receiver = value.explicitReceiver ?: return null
        if (receiver is FirResolvedQualifier) return null
        val root = literalBytes(receiver, self) ?: return null
        val callee = value.calleeReference.toResolvedCallableSymbol()?.unwrapFakeOverrides()
        if (root == Root.STRICT && callee?.callableId?.packageName != kotlinCollections) return null
        val keepsLiteral = callArguments(value).all { argument ->
            val arg = unwrap(argument)
            when (arg) {
                is FirAnonymousFunctionExpression ->
                    root == Root.TEXT && lambdaKeepsBytes(value, arg.anonymousFunction, self)
                is FirCallableReferenceAccess -> root == Root.TEXT && !isMutator(arg.calleeReference.toResolvedCallableSymbol()?.unwrapFakeOverrides()?.callableId)
                else -> keepsBytesLiteral(arg, self)
            }
        }
        return if (keepsLiteral) root else null
    }

    private fun literalSource(call: FirFunctionCall, self: Set<FirBasedSymbol<*>>): Root? {
        val callee = call.calleeReference.toResolvedCallableSymbol()?.unwrapFakeOverrides() ?: return null
        val callableId = callee.callableId ?: return null
        return when {
            callableId == byteArrayOf -> {
                val elements = callArguments(call)
                if (elements.isNotEmpty() && elements.all { isIntegerLiteral(it) }) Root.STRICT else null
            }
            callableId in stringBytes -> {
                val receiver = call.explicitReceiver?.let(::unwrap) ?: return null
                if (!isLiteralString(receiver)) return null
                // Go needs the argument text to start with the string literal
                // followed by `.toByteArray(`; a transform or a parenthesized
                // concatenation in between is a source Go misses.
                val plain = receiver is FirLiteralExpression ||
                    (receiver is FirStringConcatenationCall && receiver.source?.elementType == KtNodeTypes.STRING_TEMPLATE)
                if (plain) Root.TEXT else Root.STRICT
            }
            callableId.callableName == decode && isBase64Owner(callableId.classId) -> {
                val data = call.argumentList.arguments.firstOrNull() ?: return null
                // Go matches `Base64.decode(` or `Base64.getDecoder().decode(`
                // with a quote anywhere in the argument text.
                val goForm = if (goDecodeForm(call)) Root.TEXT else Root.STRICT
                when {
                    isLiteralString(data) -> goForm
                    else -> literalBytes(data, self)?.and(goForm)
                }
            }
            else -> null
        }
    }

    // Any class named Base64 (Go matches `Base64.decode(`), or the JDK decoder.
    private fun isBase64Owner(owner: ClassId?): Boolean =
        owner != null && (owner.shortClassName == base64 || owner == javaDecoder)

    private fun goDecodeForm(call: FirFunctionCall): Boolean {
        val receiver = call.explicitReceiver?.let(::unwrap) ?: return false
        return when (receiver) {
            is FirResolvedQualifier -> receiver.classId?.shortClassName == base64
            is FirFunctionCall -> {
                val owner = receiver.explicitReceiver?.let(::unwrap)
                receiver.calleeReference.name == getDecoder &&
                    owner is FirResolvedQualifier && owner.classId?.shortClassName == base64
            }
            else -> false
        }
    }

    // The call's value arguments, with a vararg group flattened.
    private fun callArguments(call: FirFunctionCall): List<FirExpression> =
        call.argumentList.arguments.flatMap { argument ->
            if (argument is FirVarargArgumentsExpression) argument.arguments else listOf(argument)
        }

    private fun keepsBytesLiteral(argument: FirExpression, self: Set<FirBasedSymbol<*>>): Boolean {
        val value = unwrap(argument)
        if (value is FirSpreadArgumentExpression) return literalBytes(value.expression, self) != null
        if (value is FirAnonymousFunctionExpression || value is FirCallableReferenceAccess) return false
        if (isSelfReference(value, self)) return true
        val type = value.resolvedType.lowerBoundIfFlexible()
        return when (type.classId) {
            in byteArrays -> literalBytes(value, self) != null
            in bytes -> isIntegerLiteral(value)
            // A random source feeds runtime bytes into the call.
            in randoms -> false
            // A collection or array of bytes (`+ listOf(b)`) feeds in bytes this
            // check cannot prove literal.
            else -> type.typeArguments.none { it.type?.lowerBoundIfFlexible()?.classId in bytes }
        }
    }

    // A lambda passed to a call in the chain. Go reports these chains, so FIR
    // does too unless the lambda provably brings in runtime bytes: it
    // overwrites the literal bytes, or (when the call returns the lambda's
    // result) it returns them mixed with runtime bytes.
    private fun lambdaKeepsBytes(
        call: FirFunctionCall,
        lambda: FirAnonymousFunction,
        outer: Set<FirBasedSymbol<*>>,
    ): Boolean {
        val self = buildSet {
            addAll(outer)
            lambda.valueParameters.forEach { add(it.symbol) }
            lambda.receiverParameter?.let { add(it.symbol) }
        }
        val body = lambda.body ?: return true
        if (mutatesSelf(body, self)) return false
        val callableId = call.calleeReference.toResolvedCallableSymbol()?.unwrapFakeOverrides()?.callableId
        if (callableId in receiverReturning) return true
        val last = body.statements.lastOrNull() as? FirExpression ?: return true
        val result = unwrap(if (last is FirReturnExpression) last.result else last)
        if (literalBytes(result, self) != null) return true
        return !mixesRuntimeBytes(result, self)
    }

    // True when [result] is a call that takes the lambda's literal bytes along
    // with bytes that are not literal (`xor(it, random)`, `it + random`).
    private fun mixesRuntimeBytes(result: FirExpression, self: Set<FirBasedSymbol<*>>): Boolean {
        if (result !is FirFunctionCall) return false
        val receiver = result.explicitReceiver?.let(::unwrap)?.takeUnless { it is FirResolvedQualifier }
        val inputs = (listOfNotNull(receiver) + callArguments(result)).filterNot {
            val value = unwrap(it)
            value is FirAnonymousFunctionExpression || value is FirCallableReferenceAccess
        }
        if (inputs.none { isSelfReference(unwrap(it), self) }) return false
        return inputs.any { !keepsBytesLiteral(it, self) }
    }

    // Calls that write into a byte array. [targetIndex] is the argument that
    // receives the bytes when the array is not the call's receiver.
    private data class Mutator(val randomizes: Boolean, val targetIndex: Int?)

    private fun mutator(callableId: CallableId?): Mutator? {
        if (callableId == null) return null
        val name = callableId.callableName.asString()
        val owner = callableId.classId
        return when {
            name == "nextBytes" && owner in randoms -> Mutator(randomizes = true, targetIndex = 0)
            name == "set" && owner in byteArrays -> Mutator(randomizes = false, targetIndex = null)
            name == "fill" && callableId.packageName == kotlinCollections && owner == null ->
                Mutator(randomizes = false, targetIndex = null)
            name == "fill" && owner == ClassId(FqName("java.util"), Name.identifier("Arrays")) ->
                Mutator(randomizes = false, targetIndex = 0)
            name == "shuffle" && callableId.packageName == kotlinCollections && owner == null ->
                Mutator(randomizes = true, targetIndex = null)
            name == "copyInto" && callableId.packageName == kotlinCollections && owner == null ->
                Mutator(randomizes = false, targetIndex = 0)
            name == "arraycopy" && owner == ClassId(FqName("java.lang"), Name.identifier("System")) ->
                Mutator(randomizes = false, targetIndex = 2)
            else -> null
        }
    }

    private fun isMutator(callableId: CallableId?): Boolean = mutator(callableId) != null

    // True when the lambda body writes runtime or random bytes into the
    // literal bytes: `rng.nextBytes(it)`, `it[0] = random[0]`,
    // `random.copyInto(it)`. Writing literals (`this[0] = 1`) keeps them literal.
    private fun mutatesSelf(body: FirElement, self: Set<FirBasedSymbol<*>>): Boolean {
        var found = false
        body.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirFunctionCall && writesRuntimeBytes(element, self)) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    private fun writesRuntimeBytes(call: FirFunctionCall, self: Set<FirBasedSymbol<*>>): Boolean {
        val callableId = call.calleeReference.toResolvedCallableSymbol()?.unwrapFakeOverrides()?.callableId
        val mutator = mutator(callableId) ?: return false
        val arguments = callArguments(call)
        val explicitReceiver = call.explicitReceiver?.let(::unwrap)
        val receivers = listOfNotNull(explicitReceiver, call.dispatchReceiver, call.extensionReceiver)
        val target = if (mutator.targetIndex == null) {
            receivers.firstOrNull { isSelfReference(unwrap(it), self) }
        } else {
            arguments.getOrNull(mutator.targetIndex)?.takeIf { isSelfReference(unwrap(it), self) }
        }
        if (target == null) return false
        if (mutator.randomizes) return true
        val inputs = arguments.filterIndexed { index, _ -> index != mutator.targetIndex } +
            listOfNotNull(explicitReceiver?.takeIf { mutator.targetIndex != null && it !is FirResolvedQualifier })
        return inputs.any { !isLiteralValue(it, self) }
    }

    private fun isLiteralValue(expression: FirExpression, self: Set<FirBasedSymbol<*>>): Boolean {
        val value = unwrap(expression)
        return value is FirLiteralExpression || isIntegerLiteral(value) || isSelfReference(value, self) ||
            literalBytes(value, self) != null
    }

    private fun isSelfReference(expression: FirExpression, self: Set<FirBasedSymbol<*>>): Boolean {
        if (self.isEmpty()) return false
        val symbol: FirBasedSymbol<*>? = when (expression) {
            is FirThisReceiverExpression -> expression.calleeReference.boundSymbol
            is FirQualifiedAccessExpression -> expression.calleeReference.toResolvedCallableSymbol()
            else -> null
        }
        return symbol != null && symbol in self
    }

    // An integer literal, optionally signed: K2 keeps `-1` as a unary minus
    // on the literal until it folds it.
    private fun isIntegerLiteral(expression: FirExpression): Boolean {
        val value = unwrap(expression)
        if (value is FirLiteralExpression) return value.value is Number
        if (value is FirIntegerLiteralOperatorCall) {
            val name = value.calleeReference.name.asString()
            if (name != "unaryMinus" && name != "unaryPlus") return false
            val receiver = value.explicitReceiver ?: return false
            return receiver is FirLiteralExpression && receiver.value is Number
        }
        return false
    }

    // A string fixed in the source: a literal, a template of literal entries,
    // or a kotlin.text transform of one with literal arguments.
    private fun isLiteralString(expression: FirExpression, seen: PropertyVerdicts = PropertyVerdicts()): Boolean {
        val value = unwrap(expression)
        return when (value) {
            is FirLiteralExpression -> value.value is String
            is FirStringConcatenationCall -> value.argumentList.arguments.all { entry ->
                val part = unwrap(entry)
                part is FirLiteralExpression || isLiteralProperty(part, seen) || isLiteralString(part, seen)
            }
            is FirFunctionCall -> {
                val callee = value.calleeReference.toResolvedCallableSymbol()?.unwrapFakeOverrides()
                if (callee?.callableId?.packageName != kotlinText) return false
                val receiver = value.explicitReceiver ?: return false
                isLiteralString(receiver, seen) && callArguments(value).all { isLiteralArgument(it, seen) }
            }
            else -> false
        }
    }

    private fun isLiteralArgument(argument: FirExpression, seen: PropertyVerdicts): Boolean {
        val value = unwrap(argument)
        if (value is FirLiteralExpression || isIntegerLiteral(value)) return true
        if (isLiteralProperty(value, seen) || isLiteralString(value, seen)) return true
        // `Regex("\\s")`
        if (value is FirFunctionCall && value.calleeReference.toResolvedCallableSymbol() is FirConstructorSymbol) {
            return value.resolvedType.classId == regex && callArguments(value).all { isLiteralArgument(it, seen) }
        }
        return false
    }

    // A `const val`, or a property with a default getter and no delegate
    // whose initializer is a literal string. A var or an open val counts too:
    // the literal it is initialized with is still written in the source, and
    // Go reports a template over one. A lateinit var has no initializer.
    private fun isLiteralProperty(expression: FirExpression, seen: PropertyVerdicts): Boolean {
        if (expression !is FirPropertyAccessExpression) return false
        val symbol = expression.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol ?: return false
        if (symbol.resolvedStatus.isConst) return true
        if (symbol.isLateInit || symbol.hasDelegate) return false
        if (symbol.resolvedStatus.isExpect) return false
        if (symbol.getterSymbol?.isDefault == false) return false
        val initializer = symbol.resolvedInitializer ?: return false
        return seen.property(symbol) { isLiteralString(initializer, seen) }
    }

    // One literal-string check's property verdicts. Each property is walked
    // once and its verdict reused: a shared initializer (`val B = "$A$A"`)
    // is not walked again, which would make the check exponential in the
    // chain length, and a property read again while its own initializer is
    // being walked (a cycle) is not literal on that path.
    private class PropertyVerdicts {
        private val verdicts = HashMap<FirPropertySymbol, Boolean?>()

        fun property(symbol: FirPropertySymbol, compute: () -> Boolean): Boolean {
            if (symbol in verdicts) return verdicts[symbol] == true
            verdicts[symbol] = null
            return compute().also { verdicts[symbol] = it }
        }
    }
}
