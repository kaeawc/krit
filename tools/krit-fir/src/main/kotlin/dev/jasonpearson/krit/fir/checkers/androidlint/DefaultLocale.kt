package dev.jasonpearson.krit.fir.checkers.androidlint

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.qualifiedCall
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds
import org.jetbrains.kotlin.toKtLightSourceElement

// Flags the JDK / Kotlin stdlib calls that format or case-convert text with
// the JVM's default locale:
// - `String.format(pattern, args...)`: the stdlib `String.Companion.format`
//   overload without a Locale, or the JDK's static
//   `java.lang.String.format(String, Object...)`. The overloads that take a
//   Locale first are never reported;
// - the no-argument `String.toLowerCase()` / `toUpperCase()`: the stdlib
//   extensions on kotlin.String and the JDK members of java.lang.String. K2
//   rejects the stdlib ones (DEPRECATION_ERROR since Kotlin 2.1), so they only
//   reach a clean compile under `@Suppress("DEPRECATION_ERROR")`; the JDK
//   members compile when the receiver is typed java.lang.String;
// - ICU's static `UCharacter.toLowerCase(String)` / `toUpperCase(String)`
//   (android.icu.lang and ICU4J's com.ibm.icu.lang), which ICU documents as
//   using the default locale. Their int code-point overloads are
//   locale-independent and the (Locale/ULocale, String) overloads are
//   explicit, so neither is reported.
//
// Like the Go rule, the finding sits on the first line of the call expression
// (the receiver's first line for a qualified call), with Go's messages. The
// modern `lowercase()` / `uppercase()` are never reported, and the
// instance form `"%d".format(x)` is not the static `String.format` call this
// rule targets (ImplicitDefaultLocale covers it).
//
// Deliberate differences from Go, each pinned in the golden data:
// - Precision: Go matches the call by name (`toLowerCase` / `toUpperCase` on
//   any receiver, `format` on the receiver text `String`) and skips it only
//   when an argument mentions the identifier `Locale`. FIR reports only the
//   default-locale JDK/stdlib/ICU overloads, so it does not report a call
//   that passes an explicit Locale held in a variable or parameter
//   (`String.format(locale, ...)`, `s.toLowerCase(locale)`), a Locale passed
//   by name (`String.format(locale = Locale.US, ...)`), an ICU `ULocale`
//   (`UCharacter.toLowerCase(ULocale.ROOT, s)`), a null Locale
//   (`String.format(null, ...)`, which applies no localization), the
//   locale-independent `Char.toLowerCase()` / `Character.toLowerCase(c)` /
//   `UCharacter.toLowerCase(codePoint)` / Guava's ASCII-only
//   `Ascii.toLowerCase(s)`, or a project function or class member of the same
//   name (including a same-package `String.Companion.format` extension and a
//   nested `object String`).
// - Recall: resolution sees the static call however it is spelled
//   (`java.lang.String.format`, `kotlin.String.format`,
//   `String.Companion.format`, a typealias of String, any expression of type
//   String.Companion, an implicit `String` receiver, an import alias of
//   `format` or `toLowerCase`), with named arguments, and with a format
//   argument whose text happens to mention `Locale`
//   (`String.format(patterns.getValue(Locale.US), x)` still formats with the
//   default locale). Go misses those.
internal object DefaultLocale : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "DefaultLocale"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(DefaultLocale)
    }

    private const val CASE_MESSAGE =
        "Implicitly using the default locale. Use lowercase(Locale) or uppercase(Locale) instead."
    private const val FORMAT_MESSAGE =
        "Implicitly using the default locale. Use String.format(Locale, ...) instead."

    private val kotlinText = FqName("kotlin.text")
    private val javaString = ClassId(FqName("java.lang"), Name.identifier("String"))
    private val javaLocale = ClassId(FqName("java.util"), Name.identifier("Locale"))
    private val stringCompanion = StandardClassIds.String.createNestedClassId(Name.identifier("Companion"))
    private val format = Name.identifier("format")
    private val caseNames = setOf(Name.identifier("toLowerCase"), Name.identifier("toUpperCase"))
    private val icuUCharacter = setOf(
        ClassId(FqName("android.icu.lang"), Name.identifier("UCharacter")),
        ClassId(FqName("com.ibm.icu.lang"), Name.identifier("UCharacter")),
    )


    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return
        val callableId = callee.callableId ?: return
        val message = when (callee.name) {
            format -> FORMAT_MESSAGE.takeIf { isDefaultLocaleFormat(callee) }
            in caseNames -> CASE_MESSAGE.takeIf { isDefaultLocaleCaseConversion(callee) }
            else -> null
        } ?: return
        // Owner proof: the kotlin.text top-level extension, a java.lang.String
        // member, or an ICU UCharacter static.
        val stdlib = callableId.packageName == kotlinText && callableId.className == null
        val jdk = callableId.classId == javaString
        val icu = callableId.classId in icuUCharacter
        if (!stdlib && !jdk && !icu) return
        val source = expression.source ?: return
        report(qualifiedCall(source)?.let { sourceOf(it, source) } ?: source, message)
    }

    // `String.Companion.format(format, vararg args)` (stdlib) or the static
    // `java.lang.String.format(String, Object...)`: the first parameter is the
    // pattern, not a Locale.
    private fun isDefaultLocaleFormat(callee: FirFunctionSymbol<*>): Boolean {
        val first = callee.valueParameterSymbols.firstOrNull() ?: return false
        if (first.resolvedReturnType.lowerBoundIfFlexible().classId == javaLocale) return false
        return when (callee.callableId?.classId) {
            javaString -> true
            null -> callee.resolvedReceiverType?.lowerBoundIfFlexible()?.classId == stringCompanion
            else -> false
        }
    }

    // The no-argument conversion of a String: kotlin.text's
    // `String.toLowerCase()` (not the Char one, which uses the invariant
    // Unicode mapping) or java.lang.String's member; or ICU's static
    // `UCharacter.toLowerCase(String)` / `toUpperCase(String)`, whose only
    // parameter is the text.
    private fun isDefaultLocaleCaseConversion(callee: FirFunctionSymbol<*>): Boolean {
        val owner = callee.callableId?.classId
        if (owner in icuUCharacter) {
            val only = callee.valueParameterSymbols.singleOrNull() ?: return false
            val type = only.resolvedReturnType.lowerBoundIfFlexible().classId
            return type == StandardClassIds.String || type == javaString
        }
        if (callee.valueParameterSymbols.isNotEmpty()) return false
        return when (owner) {
            javaString -> true
            null -> callee.resolvedReceiverType?.lowerBoundIfFlexible()?.classId == StandardClassIds.String
            else -> false
        }
    }

    // A source element for [node], a node in [anchor]'s tree, keeping the
    // anchor's offset shift between tree offsets and file offsets.
    private fun sourceOf(node: LighterASTNode, anchor: KtSourceElement): KtSourceElement {
        if (node == anchor.lighterASTNode) return anchor
        val shift = anchor.startOffset - anchor.lighterASTNode.startOffset
        return node.toKtLightSourceElement(
            anchor.treeStructure,
            startOffset = node.startOffset + shift,
            endOffset = node.endOffset + shift,
        )
    }
}
