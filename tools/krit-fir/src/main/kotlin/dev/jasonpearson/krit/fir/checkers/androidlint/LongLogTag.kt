package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.expressions.unwrapSmartcastExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirBackingFieldSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.text

// Flags an android.util.Log level call (`v`, `d`, `i`, `w`, `e`, `wtf`) whose
// tag is longer than 23 characters, the limit older Android releases enforce
// in Log.isLoggable. Like the Go rule, the tag is known when the first
// argument is:
// - a string literal without interpolation, raw strings included; or
// - a reference to a property (member, top-level, companion, object, or
//   local; `val`, `const val`, or `var`) initialized with such a literal.
// Anything else (a parameter, a call, a concatenation, an interpolated
// template) is left alone. `Log.println` and `Log.isLoggable` are not
// checked, as in Go.
//
// The finding sits on the first line of the call expression (the receiver's
// line for `Log.d(...)`), with Go's message; the tag in the message is the
// literal's source spelling between the quotes, as Go reads it.
//
// Deliberate differences from Go, each pinned in the golden data:
// - Recall: the call is identified by resolution, so a fully qualified
//   `android.util.Log.d(...)`, an import alias or typealias of Log, and a
//   statically imported `d(...)` are reported (LongLogTagRecall). Go needs
//   the receiver to be spelled `Log`. A tag property initialized with a
//   parenthesized literal (LongLogTagRecall) or declared in another file
//   (LongLogTagCrossFileTest) is resolved too; Go reads only bare literals in
//   the calling file.
// - Precision: a project or local class or object named Log is not
//   android.util.Log and has no tag limit (LongLogTagLookalike); Go only
//   filters it out when the Kotlin oracle is running. Go finds a referenced
//   tag by name alone, taking the first property of that name anywhere in
//   the file whose initializer is a literal; FIR reads the property the
//   reference resolves to, so a parameter, a shadowing local, a same-named
//   property in another class, or a property with a custom getter does not
//   borrow another declaration's literal (LongLogTagDivergence), and a tag
//   Go misses because an earlier same-named property is short is reported
//   (LongLogTagRecall). A custom getter other than `get() = field` makes the
//   value unknown. The limit is
//   counted in characters of the runtime value (UTF-16, as Android counts
//   it); Go counts UTF-8 bytes of the source spelling, so a short non-ASCII
//   tag or a tag spelled with escape sequences can exceed 23 for Go only
//   (LongLogTagDivergence).
internal object LongLogTag : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "LongLogTag"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(LongLogTag)
    }

    private const val MAX_TAG_LENGTH = 23

    private val logClassId = ClassId(FqName("android.util"), Name.identifier("Log"))
    private val levelNames = setOf("v", "d", "i", "w", "e", "wtf").map(Name::identifier).toSet()

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return
        val callableId = callee.callableId
        if (callableId.classId != logClassId || callableId.callableName !in levelNames) return
        // Every level overload takes the tag first.
        val tagParameter = callee.valueParameterSymbols.firstOrNull() ?: return
        val argument = expression.resolvedArgumentMapping
            ?.entries
            ?.firstOrNull { it.value.symbol == tagParameter }
            ?.key
            ?: return
        val literal = tagLiteral(argument) ?: return
        val value = literal.value as? String ?: return
        if (value.length <= MAX_TAG_LENGTH) return
        report(expression.source, "Log tag \"${spelling(literal, value)}\" exceeds the 23 character limit.")
    }

    // The literal that gives the tag its value: the argument itself, or the
    // literal initializer of the property it reads. FIR drops parentheses
    // around either one; an interpolated template is not a literal.
    private fun tagLiteral(argument: FirExpression): FirLiteralExpression? {
        val expression = unwrap(argument).unwrapSmartcastExpression()
        if (expression is FirLiteralExpression) return expression
        if (expression !is FirPropertyAccessExpression) return null
        val symbol = expression.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol ?: return null
        if (symbol.hasDelegate) return null
        val getter = symbol.getterSymbol
        if (getter != null && !getter.isDefault && !returnsField(getter)) return null
        return symbol.resolvedInitializer as? FirLiteralExpression
    }

    // A custom getter that only returns the backing field (`get() = field`)
    // still yields the initializer's value, so it keeps the finding.
    @OptIn(SymbolInternals::class)
    private fun returnsField(getter: FirFunctionSymbol<*>): Boolean {
        val statement = getter.fir.body?.statements?.singleOrNull()
        val result = if (statement is FirReturnExpression) statement.result else statement
        val access = result as? FirPropertyAccessExpression ?: return false
        return access.calleeReference.toResolvedCallableSymbol() is FirBackingFieldSymbol
    }

    private fun unwrap(argument: FirExpression): FirExpression =
        if (argument is FirWrappedArgumentExpression) argument.expression else argument

    // The literal's source text between its quotes (escape sequences
    // undecoded), which is how Go prints the tag; the runtime value when the
    // literal has no source, such as a constant from a library.
    private fun spelling(literal: FirLiteralExpression, value: String): String {
        val text = literal.source?.text?.toString()?.trim() ?: return value
        return when {
            text.length >= 6 && text.startsWith(RAW_QUOTE) && text.endsWith(RAW_QUOTE) ->
                text.substring(RAW_QUOTE.length, text.length - RAW_QUOTE.length)
            text.length >= 2 && text.startsWith(QUOTE) && text.endsWith(QUOTE) ->
                text.substring(QUOTE.length, text.length - QUOTE.length)
            else -> value
        }
    }

    private const val QUOTE = "\""
    private const val RAW_QUOTE = "\"\"\""
}
