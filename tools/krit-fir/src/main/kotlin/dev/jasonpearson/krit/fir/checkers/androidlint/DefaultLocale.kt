package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.checkers.followableInitializer
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.qualifiedCall
import dev.jasonpearson.krit.fir.support.unwrapArgument
import dev.jasonpearson.krit.fir.support.unwrapSmartCasts
import org.jetbrains.kotlin.descriptors.Modality
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.utils.isConst
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirFunctionCallOrigin
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirStringConcatenationCall
import org.jetbrains.kotlin.fir.expressions.FirThrowExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds

// Flags the JDK / Kotlin stdlib calls that format or case-convert text with
// the JVM's default locale:
// - `String.format(pattern, args...)`: the stdlib `String.Companion.format`
//   overload without a Locale, or the JDK's static
//   `java.lang.String.format(String, Object...)`. The overloads that take a
//   Locale first are never reported. As in Lint's LocaleDetector (and Go), the
//   pattern must fold to text (literals, templates, `+`, `const`s, final `val`s
//   of the same file) with a
//   locale-specific conversion (d, e, E, f, g, G, t, T), and the call must not
//   sit under a throw or be an argument of a single-letter android.util.Log
//   call. Like Go, the parts of the pattern that do not fold are skipped
//   (Lint gives up on them), and a `%tY`-style date-time conversion counts
//   (Lint reads its suffix letter as the conversion), since both format with
//   the locale;
// - kotlin.text's `capitalize()` / `decapitalize()` without a Locale, which
//   Lint reports too;
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

    private fun caseMessage(method: String) =
        "Implicitly using the default locale is a common source of bugs: Use `$method(Locale)` instead. " +
            "For strings meant to be internal use `Locale.ROOT`, otherwise `Locale.getDefault()`."
    private const val FORMAT_MESSAGE =
        "Implicitly using the default locale is a common source of bugs: Use `String.format(Locale, ...)` instead"

    private val kotlinText = FqName("kotlin.text")
    private val javaString = classIdOf("java.lang", "String")
    private val javaLocale = classIdOf("java.util", "Locale")
    private val stringCompanion = StandardClassIds.String.createNestedClassId(Name.identifier("Companion"))
    private val format = Name.identifier("format")
    private val caseNames = setOf(Name.identifier("toLowerCase"), Name.identifier("toUpperCase"))
    private val capitalizeNames = setOf(Name.identifier("capitalize"), Name.identifier("decapitalize"))
    private val plus = Name.identifier("plus")
    private val androidLog = classIdOf("android.util", "Log")

    // Conversions whose output depends on the locale, as in Lint's isLocaleSpecific.
    private val localeConversions = setOf("d", "e", "E", "f", "g", "G", "t", "T")

    // Lint's StringFormatDetector specifier pattern; group 6 is the conversion.
    private val specifier = Regex("""%(\d+\$)?([-+#, 0(<]*)?(\d+)?(\.\d+)?([tT])?([a-zA-Z%])""")
    private val icuUCharacter = setOf(
        classIdOf("android.icu.lang", "UCharacter"),
        classIdOf("com.ibm.icu.lang", "UCharacter"),
    )


    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return
        val callableId = callee.callableId ?: return
        val message = when (callee.name) {
            format -> FORMAT_MESSAGE.takeIf { isDefaultLocaleFormat(callee) }
            in caseNames -> caseMessage(callee.name.asString()).takeIf { isDefaultLocaleCaseConversion(callee) }
            in capitalizeNames -> caseMessage(callee.name.asString()).takeIf { isDefaultLocaleCapitalize(callee) }
            else -> null
        } ?: return
        // Owner proof: the kotlin.text top-level extension, a java.lang.String
        // member, or an ICU UCharacter static.
        val stdlib = callableId.packageName == kotlinText && callableId.className == null
        val jdk = callableId.classId == javaString
        val icu = callableId.classId in icuUCharacter
        if (!stdlib && !jdk && !icu) return
        if (callee.name == format && !formatReportable(expression, callee)) return
        val source = expression.source ?: return
        report(qualifiedCall(source)?.let { lightSourceOf(it, source) } ?: source, message)
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

    // kotlin.text's `String.capitalize()` / `decapitalize()` without a Locale.
    private fun isDefaultLocaleCapitalize(callee: FirFunctionSymbol<*>): Boolean =
        callee.callableId?.classId == null && callee.valueParameterSymbols.isEmpty() &&
            callee.resolvedReceiverType?.lowerBoundIfFlexible()?.classId == StandardClassIds.String

    // Lint's LocaleDetector.checkFormat, as Go applies it: the pattern folds to
    // a string with a locale-specific conversion (d, e, E, f, g, G, t, T, or a
    // `t`/`T` date-time prefix), and the call is neither under a throw nor an
    // argument of a single-letter android.util.Log call.
    context(context: CheckerContext)
    private fun formatReportable(call: FirFunctionCall, callee: FirFunctionSymbol<*>): Boolean {
        val patternParameter = callee.valueParameterSymbols.firstOrNull() ?: return false
        val argument = call.resolvedArgumentMapping?.entries?.firstOrNull { it.value.symbol == patternParameter }?.key
            ?: return false
        val pattern = constantText(argument, 0) ?: return false
        if (specifier.findAll(pattern).none { it.groupValues[5].isNotEmpty() || it.groupValues[6] in localeConversions }) {
            return false
        }
        return !inThrowOrLogArgument(call)
    }

    // The text of a string expression: literals, templates, `+`, and final
    // `val`s with an initializer. Parts that are not constant are dropped, as
    // in Go's lintEvalString; null when no part is constant.
    context(context: CheckerContext)
    private fun constantText(expression: FirExpression?, depth: Int): String? {
        if (expression == null || depth > 6) return null
        val unwrapped = unwrapArgument(expression)
        return when (val e = unwrapSmartCasts(unwrapped)) {
            is FirLiteralExpression -> e.value as? String
            is FirStringConcatenationCall -> joinKnown(e.argumentList.arguments.map { constantText(it, depth) })
            is FirFunctionCall -> {
                val callee = e.calleeReference.toResolvedCallableSymbol()
                if (callee?.name != plus || callee.callableId?.classId != StandardClassIds.String) return null
                joinKnown(listOf(constantText(e.explicitReceiver, depth), constantText(e.argumentList.arguments.singleOrNull(), depth)))
            }
            // A `const` wherever it is declared, or a final `val` of this file:
            // a slice compile reads other files' declarations without their
            // initializers.
            is FirPropertyAccessExpression -> {
                val symbol = e.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol ?: return null
                if (!symbol.isConst && (!symbol.isVal || symbol.resolvedStatus.modality != Modality.FINAL)) return null
                constantText(symbol.followableInitializer(), depth + 1)
            }
            else -> null
        }
    }

    private fun joinKnown(parts: List<String?>): String? =
        if (parts.all { it == null }) null else parts.filterNotNull().joinToString("")

    // Lint: a throw anywhere above the call, or a single-letter android.util.Log
    // call as the nearest call the format call is an argument of. A call it is
    // the receiver of (`String.format(...).trim()`) and an operator or infix
    // call (`"a" + String.format(...)`) are not that call, as in UAST.
    context(context: CheckerContext)
    private fun inThrowOrLogArgument(call: FirFunctionCall): Boolean {
        val path = context.containingElements
        var child: FirElement = call
        var logChecked = false
        for (i in path.indices.reversed()) {
            val parent = path[i]
            if (parent === call) continue
            when (parent) {
                is FirThrowExpression -> return true
                is FirFunctionCall -> if (!logChecked && parent.origin == FirFunctionCallOrigin.Regular &&
                    child !== parent.explicitReceiver
                ) {
                    logChecked = true
                    if (isLogCall(parent)) return true
                }
            }
            child = parent
        }
        return false
    }

    private fun isLogCall(call: FirFunctionCall): Boolean {
        val callee = call.calleeReference.toResolvedCallableSymbol() ?: return false
        return callee.name.asString().length == 1 && callee.callableId?.classId == androidLog
    }
}
