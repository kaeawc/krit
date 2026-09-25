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
import org.jetbrains.kotlin.fir.declarations.utils.isLateInit
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirStringConcatenationCall
import org.jetbrains.kotlin.fir.expressions.FirTryExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFieldSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.CallableId
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
 *    hardcoded value (see [isHardcoded]) or the bytes of one. The decoder is
 *    matched by its spelling, as Go matches its text, so
 *    `android.util.Base64`, `java.util.Base64`, and
 *    `kotlin.io.encoding.Base64` all count.
 * The byte helpers (`byteArrayOf`, `toByteArray`, `decode`, ...) are matched by
 * name, as Go matches them: the argument feeds the real SecretKeySpec's
 * `byte[]` parameter, so a same-named local helper still builds the key from
 * the literal. A constant string is a string literal, or a template whose
 * entries are all hardcoded. The finding is reported on the constructor call,
 * which starts on the line Go reports.
 *
 * Deliberate differences from Go, pinned by golden tests:
 *  - Recall (Go misses, FIR reports): the constructor is resolved, so an
 *    import alias, a typealias, and a file that also declares an unrelated
 *    class named `SecretKeySpec` still report (HardcodedSecretKeyResolution);
 *    Go needs the call spelled `SecretKeySpec` and skips a bare call in any
 *    file that declares a `SecretKeySpec`. Literal lists Go's comma-split
 *    parser rejects still count: a negative hex or a binary literal, a
 *    parenthesized element, a comment, a trailing comma; so do the string and
 *    Base64 conversions with whitespace or a line break before the dot
 *    (HardcodedSecretKeyLiteralForms). A Base64 decode of a bare hardcoded
 *    read with no quote in the argument (`decode(KEY_B64)`, also of a var or
 *    an open val with a hardcoded initializer, and of a local var nothing in
 *    its function reassigns) reports (HardcodedSecretKeyConstants,
 *    HardcodedSecretKeyLocalVars).
 *  - Precision (Go reports, FIR does not): a `SecretKeySpec` that resolves to
 *    another class, such as an import alias of a lookalike under a
 *    `javax.crypto.spec.*` import (HardcodedSecretKeyLookalike); a string
 *    template that interpolates a runtime value (`"$pin".toByteArray()`)
 *    or a local var reassigned anywhere in its function
 *    (HardcodedSecretKeyLocalVars); and a Base64 decode of a runtime value in an
 *    argument that merely holds a quote somewhere: a lookup key or a partial
 *    literal (`decode(prefs["key"])`, `decode(System.getenv("K"))`,
 *    `decode("c2Vj" + pin)`) (HardcodedSecretKeyNegative,
 *    HardcodedSecretKeyObfuscated, HardcodedSecretKeyFallbacks). A hardcoded
 *    fallback still reports (`decode(prefs["key"] ?: "c2Vj...")`), as in Go.
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
        if (!isHardcodedKey(key, Walk { assignedLocals() })) return
        report(expression.source, MESSAGE)
    }

    private fun isHardcodedKey(key: FirExpression, walk: Walk): Boolean =
        isLiteralByteArrayOf(key) || isStringBytes(key, walk) || containsLiteralBase64Decode(key, walk)

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
    private fun isStringBytes(expression: FirExpression, walk: Walk): Boolean {
        if (expression is FirPropertyAccessExpression && expression.calleeReference.name.asString() == "bytes") {
            return expression.explicitReceiver?.let { isLiteralReceiver(it, walk) } == true
        }
        val start = expression.source?.startOffset ?: return false
        var startsWithString = false
        var converts = false
        expression.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirExpression && isStringTemplate(element) && element.source?.startOffset == start) {
                    startsWithString = true
                }
                if (element is FirFunctionCall && isLiteralStringConversion(element, walk)) {
                    converts = true
                }
                element.acceptChildren(this)
            }
        })
        return startsWithString && converts
    }

    private fun isLiteralStringConversion(call: FirFunctionCall, walk: Walk): Boolean =
        call.calleeReference.name.asString() in stringToBytes &&
            call.explicitReceiver?.let { isLiteralReceiver(it, walk) } == true

    // A hardcoded string written directly before the dot, as in Go's `".name`.
    private fun isLiteralReceiver(receiver: FirExpression, walk: Walk): Boolean =
        isConstantString(receiver, walk) && receiver.source?.text?.endsWith("\"") == true

    // Any `<...>Base64.decode(x)` or `<...>Base64.getDecoder().decode(x)` inside
    // the expression whose input `x` is a hardcoded value or its bytes.
    private fun containsLiteralBase64Decode(expression: FirExpression, walk: Walk): Boolean {
        var found = false
        expression.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirFunctionCall && isLiteralBase64Decode(element, walk)) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    private fun isLiteralBase64Decode(call: FirFunctionCall, walk: Walk): Boolean {
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
        return isHardcoded(input, walk) || isStringBytes(input, walk)
    }

    // A value whose content is fixed by the source, on at least one path. Go
    // accepts any decode argument whose key text holds a quote anywhere; FIR
    // drops the ones whose value comes from elsewhere, such as a lookup key
    // (`prefs["key"]`, `System.getenv("K")`) or a partial literal
    // (`"c2Vj" + pin`). Hardcoded means:
    //  - a non-null literal, or a template whose entries are all hardcoded;
    //  - a read of a const val, of a property (a val or var, local or member,
    //    final or open, not lateinit) whose initializer (or literal getter,
    //    or `lazy { }` result) is hardcoded, of a Java field with a
    //    compile-time constant initializer, or of a standard charset;
    //  - a call whose receiver and arguments are all hardcoded, whatever its
    //    result type (`StringBuilder("..").reverse().toString()`), or a String
    //    or StringBuilder constructor or a charArrayOf call whose arguments are
    //    all hardcoded. A call on a class qualifier or with no receiver does
    //    not count on its literal arguments alone (`System.getenv("K")`,
    //    `loadKey("name")`);
    //  - a fallback with a hardcoded branch: an if/when/try branch, either
    //    side of an elvis, a lookup default (`getOrDefault(k, "..")`,
    //    `getString(k, "..")`), a fallback lambda (`ifEmpty { ".." }`), or a
    //    scope function's result, since the key is the literal whenever that
    //    branch runs;
    //  - any of these under a cast, a smart cast, or `!!`.
    private fun isHardcoded(expression: FirExpression, walk: Walk): Boolean {
        if (walk.depth >= MAX_DEPTH) return false
        walk.depth++
        try {
            return when (val value = strip(expression)) {
                // A null default is a missing value, not a key.
                is FirLiteralExpression -> value.kind != ConstantValueKind.Null
                is FirStringConcatenationCall -> value.argumentList.arguments.all { isHardcoded(it, walk) }
                is FirPropertyAccessExpression -> isFixedRead(value, walk)
                is FirWhenExpression -> value.branches.any { branch -> isHardcodedResult(branch.result, walk) }
                is FirElvisExpression -> isHardcoded(value.lhs, walk) || isHardcoded(value.rhs, walk)
                is FirTryExpression ->
                    isHardcodedResult(value.tryBlock, walk) || value.catches.any { isHardcodedResult(it.block, walk) }
                is FirFunctionCall -> isHardcodedCall(value, walk)
                else -> false
            }
        } finally {
            walk.depth--
        }
    }

    private fun isHardcodedCall(call: FirFunctionCall, walk: Walk): Boolean {
        val arguments = flatArguments(call)
        if (isLiteralBuilder(call)) return arguments.isNotEmpty() && arguments.all { isHardcoded(it, walk) }
        val receiver = call.explicitReceiver
        val receiverHardcoded = receiver != null && isHardcoded(receiver, walk)
        val fallback = when (call.calleeReference.name.asString()) {
            in lookupDefaults -> arguments.size == 2 && isHardcoded(arguments[1], walk)
            in fallbackLambdas -> receiverHardcoded || isHardcodedLambdaResult(arguments.lastOrNull(), walk)
            in resultScopes -> isHardcodedLambdaResult(arguments.lastOrNull(), walk)
            in receiverScopes -> receiverHardcoded
            else -> false
        }
        return fallback || (receiverHardcoded && arguments.all { isHardcoded(it, walk) })
    }

    // The default value of a lookup (the second argument).
    private val lookupDefaults = setOf("getOrDefault", "getString")

    // The receiver, or the lambda's result when the receiver is empty or the
    // key is missing.
    private val fallbackLambdas = setOf("ifEmpty", "ifBlank", "getOrElse")

    // The lambda's result.
    private val resultScopes = setOf("run", "let", "with")

    // The receiver itself (or null).
    private val receiverScopes = setOf("also", "apply", "takeIf", "takeUnless")

    private val stringBuilderClassIds = setOf(
        ClassId(FqName("java.lang"), Name.identifier("StringBuilder")),
        ClassId(FqName("java.lang"), Name.identifier("StringBuffer")),
        ClassId(FqName("kotlin"), Name.identifier("String")),
    )
    private val stringFunctionId = CallableId(FqName("kotlin.text"), Name.identifier("String"))
    private val charArrayOfId = CallableId(FqName("kotlin"), Name.identifier("charArrayOf"))

    // `String(..)`, `StringBuilder(..)`, `StringBuffer(..)`, or
    // `charArrayOf(..)`: the result holds exactly the characters (or bytes) of
    // its arguments.
    private fun isLiteralBuilder(call: FirFunctionCall): Boolean {
        if (call.explicitReceiver != null) return false
        return when (val symbol = call.calleeReference.toResolvedCallableSymbol()) {
            is FirConstructorSymbol -> symbol.containingClassLookupTag()?.classId in stringBuilderClassIds
            is FirNamedFunctionSymbol -> symbol.callableId == stringFunctionId || symbol.callableId == charArrayOfId
            else -> false
        }
    }

    private fun isHardcodedLambdaResult(argument: FirExpression?, walk: Walk): Boolean {
        val lambda = argument?.let(::strip) as? FirAnonymousFunctionExpression ?: return false
        val body = lambda.anonymousFunction.body ?: return false
        return isHardcodedResult(body, walk)
    }

    // The value a block produces is its last statement.
    private fun isHardcodedResult(block: FirBlock, walk: Walk): Boolean {
        val result = when (val last = block.statements.lastOrNull()) {
            is FirReturnExpression -> last.result
            is FirExpression -> last
            else -> return false
        }
        return isHardcoded(result, walk)
    }

    private fun flatArguments(call: FirFunctionCall): List<FirExpression> =
        call.argumentList.arguments.flatMap { argument ->
            val value = unwrap(argument)
            if (value is FirVarargArgumentsExpression) value.arguments.map(::unwrap) else listOf(value)
        }

    private val charsetOwners = setOf(
        ClassId(FqName("kotlin.text"), Name.identifier("Charsets")),
        ClassId(FqName("java.nio.charset"), Name.identifier("StandardCharsets")),
    )
    private val lazyId = CallableId(FqName("kotlin"), Name.identifier("lazy"))

    // A read whose value the source fixes: its declared initializer (or
    // getter, or lazy result) is hardcoded. A member or top-level var and an
    // open val count too: the hardcoded initializer is still a secret written
    // in the source (the default), and Go reports a template over one. A
    // local var counts only when nothing in its function assigns it again;
    // otherwise its value at the read need not be the initializer. A
    // lateinit var has no initializer.
    @OptIn(SymbolInternals::class)
    private fun isFixedRead(access: FirPropertyAccessExpression, walk: Walk): Boolean =
        when (val symbol = access.calleeReference.toResolvedCallableSymbol()) {
            is FirPropertySymbol -> when {
                symbol.isConst -> true
                symbol.callableId?.classId in charsetOwners -> true
                symbol.isLateInit -> false
                isReassignedLocalVar(symbol, walk::assignedLocals) -> false
                else -> walk.property(symbol) {
                    when {
                        symbol.hasDelegate -> isHardcodedLazy(symbol.delegate, walk)
                        symbol.getterSymbol?.isDefault == false ->
                            symbol.getterSymbol?.fir?.body?.let { isHardcodedResult(it, walk) } == true
                        else -> symbol.resolvedInitializer?.let { isHardcoded(it, walk) } == true
                    }
                }
            }
            // A Java field: only a compile-time constant (a final field with a
            // constant initializer, in source or in the class file).
            is FirFieldSymbol -> symbol.isVal && symbol.hasConstantInitializer
            else -> false
        }

    // `by lazy { hardcoded }`.
    private fun isHardcodedLazy(delegate: FirExpression?, walk: Walk): Boolean {
        val call = delegate as? FirFunctionCall ?: return false
        if (call.calleeReference.toResolvedCallableSymbol()?.callableId != lazyId) return false
        return isHardcodedLambdaResult(flatArguments(call).lastOrNull(), walk)
    }

    // Wrappers that keep the value: a named argument, a cast, a smart cast, `!!`.
    private fun strip(expression: FirExpression): FirExpression {
        var current = expression
        while (true) {
            current = when (current) {
                is FirWrappedArgumentExpression -> current.expression
                is FirSmartCastExpression -> current.originalExpression
                is FirCheckNotNullCall -> current.argumentList.arguments.firstOrNull() ?: return current
                is FirTypeOperatorCall -> {
                    if (current.operation != FirOperation.AS && current.operation != FirOperation.SAFE_AS) return current
                    current.argumentList.arguments.firstOrNull() ?: return current
                }
                else -> return current
            }
        }
    }

    // Bounds the recursion depth (the stack) through chains of initializers.
    private const val MAX_DEPTH = 64

    // One key's evaluation. Each property's verdict is computed once and
    // reused: a shared initializer (`val B = A + A`) is not walked again, and
    // a property read again while its own initializer is being walked (a
    // cycle, `val A: String = if (f) B else B` with B reading A) counts as
    // not hardcoded on that path. Without this, branching initializers make
    // the walk exponential in the chain length.
    private class Walk(assigned: () -> Set<FirBasedSymbol<*>>?) {
        var depth = 0
        private val properties = HashMap<FirPropertySymbol, Boolean?>()
        private val assignedOnce by lazy(LazyThreadSafetyMode.NONE, assigned)

        // The local variables assigned in the enclosing declaration (see
        // assignedLocals), computed on the first local var read.
        fun assignedLocals(): Set<FirBasedSymbol<*>>? = assignedOnce

        fun property(symbol: FirPropertySymbol, compute: () -> Boolean): Boolean {
            if (symbol in properties) return properties[symbol] == true
            properties[symbol] = null
            return compute().also { properties[symbol] = it }
        }
    }

    private fun spelledBase64(receiver: FirExpression): Boolean =
        receiver.source?.text?.toString()?.endsWith("Base64") == true

    private fun isStringTemplate(expression: FirExpression): Boolean =
        (expression is FirLiteralExpression && expression.value is String) || expression is FirStringConcatenationCall

    // A string literal, or a template whose entries are all hardcoded.
    private fun isConstantString(expression: FirExpression, walk: Walk): Boolean = when (expression) {
        is FirLiteralExpression -> expression.value is String
        is FirStringConcatenationCall -> isHardcoded(expression, walk)
        else -> false
    }

    private fun unwrap(argument: FirExpression): FirExpression =
        if (argument is FirWrappedArgumentExpression) argument.expression else argument
}
