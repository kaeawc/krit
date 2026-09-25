package dev.jasonpearson.krit.fir.checkers.security

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirCallableReferenceAccess
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirIntegerLiteralOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirSpreadArgumentExpression
import org.jetbrains.kotlin.fir.expressions.FirStringConcatenationCall
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.types.type
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
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
 *    `"<hex>".hexToByteArray(...)` from `kotlin.text`, on a string literal
 *    (raw or not; a template counts only when every entry is a literal or a
 *    `const val`);
 *  - a Base64 decode of such a literal: `android.util.Base64.decode`,
 *    `java.util.Base64.Decoder.decode`, or `kotlin.io.encoding.Base64.decode`;
 * or when it is a chain of calls on one of those sources (`.copyOf(16)`,
 * `+ "x".toByteArray()`) in which no call takes a lambda, a callable
 * reference, or a byte, byte array, or byte collection argument that is not
 * itself literal. The finding is reported on the constructor call, which
 * starts on the line Go reports.
 *
 * Deliberate differences from Go, pinned by golden and probe tests. Go matches
 * the constructor by its simple name plus the file's imports and declarations,
 * and the argument by its source text; FIR reads the resolved call instead:
 *  - Go false negatives FIR reports: an import alias or typealias of the spec
 *    class, and a bare spec call in a file that declares an unrelated nested
 *    class of the same name (StaticIvAlias); `byteArrayOf` elements Go's
 *    number parser rejects (`-0x10`, `0b1`, a parenthesized literal, a
 *    comment, a trailing comma), an annotated argument, a chain on
 *    `byteArrayOf` (`byteArrayOf(1).copyOf(16)`), and decoder forms Go's text
 *    match misses (`Base64.Default.decode`, `Base64.getUrlDecoder().decode`, a
 *    decoder held in a local) (StaticIvGoMisses).
 *  - Go false positives FIR skips, where the IV is not literal bytes: a string
 *    template with a runtime entry (`"$x".toByteArray()`); a chain that mixes
 *    in runtime bytes or runs a lambda, which can replace or overwrite them
 *    (`"a".toByteArray() + random`, `.let { ... }`, `.also { ... }`); a decode
 *    whose data argument is not a literal but whose text holds a quote
 *    (`Base64.decode(prefs.getString("iv", ""), 0)`); a literal source nested
 *    in another call's argument (`xor(Base64.decode("a", 0), random)`); a
 *    Base64 lookalike (StaticIvPrecision); a same-file `String.toByteArray`
 *    lookalike (StaticIvLookalike); a same-package spec class or Base64 that
 *    wins over a star import (StaticIvTest).
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

    private val kotlinText = FqName("kotlin.text")
    private val byteArrayOf = CallableId(FqName("kotlin"), Name.identifier("byteArrayOf"))
    private val stringBytes = setOf(
        CallableId(kotlinText, Name.identifier("toByteArray")),
        CallableId(kotlinText, Name.identifier("encodeToByteArray")),
        CallableId(kotlinText, Name.identifier("hexToByteArray")),
    )

    private val byteArrays = setOf(
        ClassId(FqName("kotlin"), Name.identifier("ByteArray")),
        ClassId(FqName("kotlin"), Name.identifier("UByteArray")),
    )

    private val bytes = setOf(StandardClassIds.Byte, StandardClassIds.UByte)

    private val decode = Name.identifier("decode")
    private val base64Owners = setOf(
        ClassId(FqName("android.util"), Name.identifier("Base64")),
        ClassId(FqName("java.util"), FqName("Base64.Decoder"), false),
        ClassId(FqName("kotlin.io.encoding"), Name.identifier("Base64")),
    )

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
        if (!isLiteralBytes(ivArgument)) return
        report(expression.source, MESSAGE)
    }

    private fun unwrap(expression: FirExpression): FirExpression {
        var current = expression
        while (true) {
            current = when (current) {
                is FirSpreadArgumentExpression -> return current
                is FirWrappedArgumentExpression -> current.expression
                is FirSmartCastExpression -> current.originalExpression
                else -> return current
            }
        }
    }

    // A byte array built only from inline literals: a literal byte source, or a
    // chain of calls on one that feeds in no other bytes and runs no lambda.
    private fun isLiteralBytes(expression: FirExpression): Boolean {
        val value = unwrap(expression)
        if (value !is FirFunctionCall) return false
        if (isLiteralSource(value)) return true
        val receiver = value.explicitReceiver ?: return false
        if (receiver is FirResolvedQualifier) return false
        if (!isLiteralBytes(receiver)) return false
        return callArguments(value).all(::keepsBytesLiteral)
    }

    private fun isLiteralSource(call: FirFunctionCall): Boolean {
        val callee = call.calleeReference.toResolvedCallableSymbol()?.unwrapFakeOverrides() ?: return false
        val callableId = callee.callableId ?: return false
        return when {
            callableId == byteArrayOf -> {
                val elements = callArguments(call)
                elements.isNotEmpty() && elements.all { isIntegerLiteral(it) }
            }
            callableId in stringBytes -> call.explicitReceiver?.let(::isLiteralString) == true
            callableId.callableName == decode && callableId.classId in base64Owners -> {
                val data = call.argumentList.arguments.firstOrNull() ?: return false
                isLiteralString(data) || isLiteralBytes(data)
            }
            else -> false
        }
    }

    // The call's value arguments, with a vararg group flattened.
    private fun callArguments(call: FirFunctionCall): List<FirExpression> =
        call.argumentList.arguments.flatMap { argument ->
            if (argument is FirVarargArgumentsExpression) argument.arguments else listOf(argument)
        }

    private fun keepsBytesLiteral(argument: FirExpression): Boolean {
        val value = unwrap(argument)
        if (value is FirSpreadArgumentExpression) return isLiteralBytes(value.expression)
        if (value is FirAnonymousFunctionExpression || value is FirCallableReferenceAccess) return false
        val type = value.resolvedType.lowerBoundIfFlexible()
        return when (type.classId) {
            in byteArrays -> isLiteralBytes(value)
            in bytes -> isIntegerLiteral(value)
            // A collection or array of bytes (`+ listOf(b)`) feeds in bytes this
            // check cannot prove literal.
            else -> type.typeArguments.none { it.type?.lowerBoundIfFlexible()?.classId in bytes }
        }
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

    private fun isLiteralString(expression: FirExpression): Boolean {
        val value = unwrap(expression)
        return when (value) {
            is FirLiteralExpression -> value.value is String
            is FirStringConcatenationCall -> value.argumentList.arguments.all { entry ->
                val part = unwrap(entry)
                part is FirLiteralExpression || isConstReference(part)
            }
            else -> false
        }
    }

    private fun isConstReference(expression: FirExpression): Boolean {
        if (expression !is FirPropertyAccessExpression) return false
        val symbol = expression.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol ?: return false
        return symbol.resolvedStatus.isConst
    }
}
