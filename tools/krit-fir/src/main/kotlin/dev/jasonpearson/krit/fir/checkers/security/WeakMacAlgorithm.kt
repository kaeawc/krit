package dev.jasonpearson.krit.fir.checkers.security

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.firstArgumentStringContent
import dev.jasonpearson.krit.fir.support.trimGoSpace
import dev.jasonpearson.krit.fir.support.upperCodePoints
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags `Mac.getInstance("<weak HMAC>")` on the JDK's javax.crypto.Mac, where
// the first argument is a plain string literal naming HmacMD2, HmacMD5,
// HmacSHA0, or HmacSHA1 (case-insensitive, surrounding whitespace ignored).
// Every getInstance overload counts; only the first argument is read.
//
// Mirrors the Go rule's argument evidence on top of FIR resolution:
// - the call must resolve to javax.crypto.Mac.getInstance;
// - the first argument is read from the call's syntax tree: it must be a
//   string template (optionally parenthesized) with no `$` entries, and the
//   algorithm is the raw text of its entries, so a literal containing an
//   escape sequence never matches;
// - surrounding whitespace is trimmed with Go's unicode.IsSpace set and the
//   text is uppercased code point by code point, as the Go rule does.
//
// Deliberate differences from Go, pinned by goldens. Go cannot resolve the
// receiver, so it accepts only the spellings `Mac` and `javax.crypto.Mac` and
// guesses what a bare `Mac` means from the file's imports and declarations;
// FIR reads the resolved call instead:
// - Recall (Go misses these true positives): an import alias, typealias, or
//   backticked spelling of javax.crypto.Mac, and a statically imported
//   getInstance (WeakMacAlgorithmSpellings); a bare `Mac` in a file that also
//   declares an unrelated, non-shadowing top-level, nested, or local class or
//   nested object named Mac (WeakMacAlgorithmDeclaredName); a top-level
//   `typealias Mac = javax.crypto.Mac`, which Go counts as a lookalike
//   declaration (WeakMacAlgorithmTypealiasNamedMac); a parenthesized receiver
//   `(Mac)`, an annotated or labeled literal (`@A "HmacMD5"`,
//   `l@ "HmacMD5"`), or a parenthesized literal preceded by a comment, which
//   Go reads as the argument node instead of the literal (WeakMacAlgorithm).
// - Precision (Go reports these, but the call is not javax.crypto.Mac's):
//   a local val, parameter, member property, or companion object named Mac
//   that shadows the import (WeakMacAlgorithmShadowed), and a top-level
//   property named Mac (WeakMacAlgorithmShadowedTopLevel); an explicit import
//   of another class as or named Mac over a `javax.crypto.*` star import
//   (WeakMacAlgorithmAliasedOther); and, in WeakMacAlgorithmTest, a
//   same-package Mac declared in another file over a star import, an explicit
//   import of another Mac where Go's javax.crypto.Mac mention check is met by
//   a comment, a `javax.crypto.MacSpi` import, or `javax.crypto.Mac as JMac`,
//   and a nested Mac inherited from a supertype declared in another file.
internal object WeakMacAlgorithm : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "WeakMacAlgorithm"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(WeakMacAlgorithm)
    }

    private const val MESSAGE =
        "Mac.getInstance uses an HMAC algorithm backed by a weak digest. Use HmacSHA256, HmacSHA384, HmacSHA512, or SHA-3-based alternatives."

    private val macClassId = ClassId(FqName("javax.crypto"), Name.identifier("Mac"))
    private val getInstanceId = CallableId(macClassId, Name.identifier("getInstance"))
    private val weakAlgorithms = setOf("HMACMD2", "HMACMD5", "HMACSHA0", "HMACSHA1")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId != getInstanceId) return

        if (expression.argumentList.arguments.isEmpty()) return
        val algorithm = expression.source?.let(::firstArgumentStringContent) ?: return
        if (upperCodePoints(trimGoSpace(algorithm)) !in weakAlgorithms) return

        report(expression.source, MESSAGE)
    }
}
