package dev.jasonpearson.krit.fir.checkers.security

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.containingClassLookupTag
import org.jetbrains.kotlin.fir.declarations.utils.isConst
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirStringConcatenationCall
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.isString
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.text
import org.jetbrains.kotlin.types.ConstantValueKind

/**
 * Flags `javax.crypto.spec.SecretKeySpec(<key>, ...)` whose key argument (the
 * first argument, in both constructor overloads) is built from hardcoded
 * bytes. Mirrors the Go HardcodedSecretKey rule on Kotlin code. The key counts
 * as hardcoded in the shapes Go recognizes:
 *  - the argument is `byteArrayOf(...)` or `kotlin.byteArrayOf(...)` (an
 *    unqualified call may be a same-named local function, as in Go) whose
 *    arguments are all integer or char literals, `-1` included; an empty
 *    call does not count;
 *  - the argument starts with a string literal and contains a `toByteArray`,
 *    `encodeToByteArray`, `getBytes`, or `hexToByteArray` call made directly
 *    on a constant string literal (`"k".toByteArray()`), anywhere in it, so
 *    calls chained after it still count (`"k".toByteArray().copyOf(16)`), as
 *    Go matches the text anywhere in the argument; or the argument is a
 *    `.bytes` read on a constant string literal (Go matches `".bytes` only at
 *    the end of the argument);
 *  - the argument contains, anywhere, a `decode(x)` call on a receiver
 *    spelled `...Base64` or on `...Base64.getDecoder()` whose input `x` is a
 *    hardcoded string or the bytes of one. The decoder is matched by its
 *    spelling, as Go matches its text, so `android.util.Base64`,
 *    `java.util.Base64`, and `kotlin.io.encoding.Base64` all count.
 * The byte helpers (`byteArrayOf`, `toByteArray`, `decode`, ...) are matched by
 * name, as Go matches them: the argument feeds the real SecretKeySpec's
 * `byte[]` parameter, so a same-named local helper still builds the key from
 * the literal. A constant string is a string literal, or a template whose
 * entries are all literals or `const val` reads. The finding is reported on
 * the constructor call, which starts on the line Go reports.
 *
 * Deliberate differences from Go, pinned by golden tests:
 *  - Recall (Go misses, FIR reports): the constructor is resolved, so an
 *    import alias, a typealias, and a file that also declares an unrelated
 *    class named `SecretKeySpec` still report (HardcodedSecretKeyResolution);
 *    Go needs the call spelled `SecretKeySpec` and skips a bare call in any
 *    file that declares a `SecretKeySpec`. Literal lists Go's comma-split
 *    parser rejects still count: a negative hex or a binary literal, a
 *    parenthesized element, a comment, a trailing comma; so do the string and
 *    Base64 conversions split across a line break
 *    (HardcodedSecretKeyLiteralForms).
 *  - Precision (Go reports, FIR does not): a `SecretKeySpec` that resolves to
 *    another class, such as an import alias of a lookalike under a
 *    `javax.crypto.spec.*` import (HardcodedSecretKeyLookalike); a string
 *    template that interpolates a runtime value (`"$pin".toByteArray()`); and
 *    a Base64 decode of a runtime value in an argument that merely holds a
 *    quote somewhere: a lookup key, a fallback, or a partial literal
 *    (`decode(prefs["key"] ?: "c2Vj...")`, `decode("c2Vj" + pin)`). Their key
 *    bytes are not hardcoded (HardcodedSecretKeyNegative).
 */
internal object HardcodedSecretKey : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "HardcodedSecretKey"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(HardcodedSecretKey)
    }

    private const val MESSAGE =
        "SecretKeySpec is constructed from hardcoded bytes. Load keys from Android Keystore or a secret manager instead."

    private val secretKeySpecClassId =
        ClassId(FqName("javax.crypto.spec"), Name.identifier("SecretKeySpec"))
    private val kotlinPackage = FqName("kotlin")
    private const val BYTE_ARRAY_OF = "byteArrayOf"
    private val stringToBytes = setOf("toByteArray", "encodeToByteArray", "getBytes", "hexToByteArray")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirConstructorSymbol ?: return
        if (callee.containingClassLookupTag()?.classId != secretKeySpecClassId) return
        val key = expression.argumentList.arguments.firstOrNull()?.let(::unwrap) ?: return
        if (!isHardcodedKey(key)) return
        report(expression.source, MESSAGE)
    }

    private fun isHardcodedKey(key: FirExpression): Boolean =
        isLiteralByteArrayOf(key) || isStringBytes(key) || containsLiteralBase64Decode(key)

    // `byteArrayOf(1, 2)` / `kotlin.byteArrayOf(1, 2)` with only number or char
    // literal arguments.
    private fun isLiteralByteArrayOf(expression: FirExpression): Boolean {
        val call = expression as? FirFunctionCall ?: return false
        if (call.calleeReference.name.asString() != BYTE_ARRAY_OF) return false
        when (val receiver = call.explicitReceiver) {
            null -> Unit
            is FirResolvedQualifier -> if (receiver.classId != null || receiver.packageFqName != kotlinPackage) return false
            else -> return false
        }
        val items = call.argumentList.arguments.flatMap { argument ->
            if (argument is FirVarargArgumentsExpression) argument.arguments else listOf(argument)
        }
        return items.isNotEmpty() && items.all(::isByteLiteral)
    }

    // Go parses each element as an integer (sign, `0x` prefix, `_`, and `L`/`u`
    // suffixes allowed) or a char literal; a float literal does not count.
    private val byteLiteralKinds = setOf(
        ConstantValueKind.Byte,
        ConstantValueKind.Short,
        ConstantValueKind.Int,
        ConstantValueKind.Long,
        ConstantValueKind.UnsignedByte,
        ConstantValueKind.UnsignedShort,
        ConstantValueKind.UnsignedInt,
        ConstantValueKind.UnsignedLong,
        ConstantValueKind.IntegerLiteral,
        ConstantValueKind.UnsignedIntegerLiteral,
        ConstantValueKind.Char,
    )

    private fun isByteLiteral(expression: FirExpression): Boolean =
        expression is FirLiteralExpression && expression.kind in byteLiteralKinds

    // Starts with a string literal and holds a string-to-bytes conversion call
    // made directly on a hardcoded string literal, or is a `.bytes` read on one
    // (Go matches `".bytes` only at the end of the argument).
    private fun isStringBytes(expression: FirExpression): Boolean {
        if (expression is FirPropertyAccessExpression && expression.calleeReference.name.asString() == "bytes") {
            return expression.explicitReceiver?.let(::isLiteralReceiver) == true
        }
        val start = expression.source?.startOffset ?: return false
        var startsWithString = false
        var converts = false
        expression.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirExpression && isStringTemplate(element) && element.source?.startOffset == start) {
                    startsWithString = true
                }
                if (element is FirFunctionCall && isLiteralStringConversion(element)) {
                    converts = true
                }
                element.acceptChildren(this)
            }
        })
        return startsWithString && converts
    }

    private fun isLiteralStringConversion(call: FirFunctionCall): Boolean =
        call.calleeReference.name.asString() in stringToBytes &&
            call.explicitReceiver?.let(::isLiteralReceiver) == true

    // A hardcoded string written directly before the dot, as in Go's `".name`.
    private fun isLiteralReceiver(receiver: FirExpression): Boolean =
        isConstantString(receiver) && receiver.source?.text?.endsWith("\"") == true

    // Any `<...>Base64.decode(x)` or `<...>Base64.getDecoder().decode(x)` inside
    // the expression whose input `x` is a hardcoded string or its bytes.
    private fun containsLiteralBase64Decode(expression: FirExpression): Boolean {
        var found = false
        expression.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirFunctionCall && isLiteralBase64Decode(element)) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    private fun isLiteralBase64Decode(call: FirFunctionCall): Boolean {
        if (call.calleeReference.name.asString() != "decode") return false
        val receiver = call.explicitReceiver ?: return false
        val decoder = when {
            spelledBase64(receiver) -> true
            receiver is FirFunctionCall && receiver.calleeReference.name.asString() == "getDecoder" &&
                receiver.argumentList.arguments.isEmpty() ->
                receiver.explicitReceiver?.let(::spelledBase64) == true
            else -> false
        }
        if (!decoder) return false
        val input = call.argumentList.arguments.firstOrNull()?.let(::unwrap) ?: return false
        return isHardcodedString(input) || isStringBytes(input)
    }

    // A string whose value is fixed by the source: a constant string, a String
    // call made on one with only literal or hardcoded arguments (`"a" + "b"`,
    // `"a-b".replace("-", "")`), or an if/when whose every branch is one. Go
    // accepts any decode argument when the key text holds a quote anywhere;
    // FIR drops the ones whose value can come from elsewhere, such as a lookup
    // key (`prefs["key"]`) or a fallback (`prefs["key"] ?: "default"`).
    private fun isHardcodedString(expression: FirExpression): Boolean = when (expression) {
        is FirFunctionCall -> {
            val receiver = expression.explicitReceiver
            expression.resolvedType.isString && receiver != null && isHardcodedString(receiver) &&
                expression.argumentList.arguments.all { it is FirLiteralExpression || isHardcodedString(it) }
        }
        is FirWhenExpression -> expression.branches.isNotEmpty() && expression.branches.all { branch ->
            (branch.result.statements.lastOrNull() as? FirExpression)?.let(::isHardcodedString) == true
        }
        else -> isConstantString(expression)
    }

    private fun spelledBase64(receiver: FirExpression): Boolean =
        receiver.source?.text?.toString()?.endsWith("Base64") == true

    private fun isStringTemplate(expression: FirExpression): Boolean =
        (expression is FirLiteralExpression && expression.value is String) || expression is FirStringConcatenationCall

    // A string literal, or a template whose entries are all literals or
    // `const val` reads.
    private fun isConstantString(expression: FirExpression): Boolean = when (expression) {
        is FirLiteralExpression -> expression.value is String
        is FirStringConcatenationCall -> expression.argumentList.arguments.all(::isConstantEntry)
        else -> false
    }

    private fun isConstantEntry(expression: FirExpression): Boolean = when (expression) {
        is FirLiteralExpression -> true
        is FirStringConcatenationCall -> expression.argumentList.arguments.all(::isConstantEntry)
        is FirPropertyAccessExpression ->
            (expression.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol)?.isConst == true
        else -> false
    }

    private fun unwrap(argument: FirExpression): FirExpression =
        if (argument is FirWrappedArgumentExpression) argument.expression else argument
}
