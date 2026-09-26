package dev.jasonpearson.krit.fir.checkers.security

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.scanSqlOutsideStrings
import dev.jasonpearson.krit.fir.support.significantChildren
import dev.jasonpearson.krit.fir.support.splitSqlConcatOperands
import dev.jasonpearson.krit.fir.support.sqlInterpolationUsesOnlySchemaConstants
import dev.jasonpearson.krit.fir.support.sqlLastIdentifierSegment
import dev.jasonpearson.krit.fir.support.sqlSchemaConstantName
import dev.jasonpearson.krit.fir.support.unwrapLightParens
import org.jetbrains.kotlin.KtNodeTypes
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
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds

/**
 * Port of the Go RuntimeExecUnsafeShape rule: a call to
 * `java.lang.Runtime.exec(String)` whose single command string is built with
 * string interpolation or with non-static concatenation, reported on the
 * command argument.
 *
 * Mirrored from Go:
 * - only the one-argument `exec(String)` overload counts; `exec(String[])`,
 *   `exec(String, String[])`, and the other overloads are left alone;
 * - the argument's shape is read from its source text exactly as Go reads it
 *   (parentheses around the argument unwrapped): it must be a string literal,
 *   contain an interpolation anywhere inside it, or be a top-level `+`
 *   concatenation; a plain variable or call (`exec(cmd)`) is not reported;
 * - an argument with interpolation is reported as interpolated unless every
 *   `$name` / `${name...}` in its text names a constant-looking identifier
 *   (`TABLE_`/`COLUMN_` prefix, `_TABLE`/`_COLUMN`/`_KEY` suffix, or an
 *   upper-case name with an underscore; the last dotted segment counts);
 * - a concatenation (or a single operand) is reported as computed when an
 *   operand is neither `null`, a constant-looking name, nor text starting
 *   with a quote that holds no `$` (so a chain such as
 *   `"ls %1\$s".format(dir)` is computed, and `"ls %s".format(dir)` is not).
 *
 * Deliberate differences from Go, pinned by goldens and listed in the PR:
 * - Recall: Go requires the syntactic receiver `Runtime.getRuntime()` (or
 *   `java.lang.Runtime.getRuntime()`, or an import alias of it) and resolves
 *   `Runtime` by name. FIR reads the resolved call, so it also reports exec on
 *   a stored Runtime, on an implicit Runtime receiver
 *   (`with(Runtime.getRuntime()) { exec(..) }`, an extension on Runtime), on a
 *   parenthesized or `!!` receiver, and through a typealias or a statically
 *   imported getRuntime.
 * - Recall: Go treats a parenthesized operand that starts with a string
 *   literal (`"ls " + ("-la " + dir)`) as static text, and treats an argument
 *   whose interpolations all name constants as static even when it also
 *   concatenates a non-static operand (`"${BASE_DIR}/ls " + dir`). FIR
 *   classifies the parenthesized group's own operands, and falls through to
 *   the concatenation check after the constant-only interpolation exemption.
 * - Precision: Go reports exec(String[]) when the array argument's text
 *   contains an interpolation or a top-level `+` (`listOf("ls", "$dir")
 *   .toTypedArray()`, `base + dir` on an array), reports a command made only
 *   of complete literals as computed when a literal holds a `$` that is not a
 *   non-constant template (`"echo \$HOME"`, `"price $5"`), and reports exec
 *   on a star-imported project class named Runtime. None passes a computed
 *   String command to java.lang.Runtime.exec(String).
 */
internal object RuntimeExecUnsafeShape : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "RuntimeExecUnsafeShape"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(RuntimeExecUnsafeShape)
    }

    private const val INTERPOLATED_MESSAGE =
        "Runtime.exec(String) uses an interpolated command string. Pass a String array or ProcessBuilder argument list instead."
    private const val COMPUTED_MESSAGE =
        "Runtime.exec(String) uses a computed command string. Pass a String array or ProcessBuilder argument list instead."

    private val runtimeClassId = ClassId(FqName("java.lang"), Name.identifier("Runtime"))
    private val execId = CallableId(runtimeClassId, Name.identifier("exec"))

    private enum class Shape { STATIC, INTERPOLATED, COMPUTED }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return
        if (callee.callableId != execId) return
        val parameter = callee.valueParameterSymbols.singleOrNull() ?: return
        if (parameter.resolvedReturnType.lowerBoundIfFlexible().classId != StandardClassIds.String) return
        if (expression.argumentList.arguments.size != 1) return

        val source = expression.source ?: return
        val argument = commandArgument(source) ?: return
        val text = lightText(source, argument).trim()
        val interpolated = containsTemplateEntry(source, argument)
        if (!looksLikeStringCommand(text, interpolated)) return

        val message = when (shape(text, interpolated)) {
            Shape.STATIC -> return
            Shape.INTERPOLATED -> INTERPOLATED_MESSAGE
            Shape.COMPUTED -> COMPUTED_MESSAGE
        }
        report(lightSourceOf(argument, source), message)
    }

    // The single value argument's expression in the call's syntax tree, with
    // enclosing parentheses removed (Go's flatUnwrapParenExpr).
    private fun commandArgument(source: KtSourceElement): LighterASTNode? {
        val root = source.lighterASTNode
        val call = when (root.tokenType) {
            KtNodeTypes.CALL_EXPRESSION -> root
            KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION ->
                significantChildren(source, root).lastOrNull()?.takeIf { it.tokenType == KtNodeTypes.CALL_EXPRESSION }
            else -> null
        } ?: return null
        val arguments = significantChildren(source, call)
            .firstOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT_LIST } ?: return null
        val argument = significantChildren(source, arguments)
            .singleOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT } ?: return null
        val value = significantChildren(source, argument).singleOrNull() ?: return null
        return unwrapLightParens(source, value)
    }

    // Go's flatContainsStringInterpolation: a `$name` or `${...}` entry
    // anywhere inside the argument, nested calls and lambdas included.
    private fun containsTemplateEntry(source: KtSourceElement, node: LighterASTNode): Boolean {
        if (node.tokenType == KtNodeTypes.SHORT_STRING_TEMPLATE_ENTRY ||
            node.tokenType == KtNodeTypes.LONG_STRING_TEMPLATE_ENTRY
        ) {
            return true
        }
        return lightChildren(source, node).any { containsTemplateEntry(source, it) }
    }

    // Go's runtimeExecArgumentLooksStringCommand.
    private fun looksLikeStringCommand(text: String, interpolated: Boolean): Boolean {
        if (text.isEmpty()) return false
        if (text.startsWith("arrayOf(") || text.startsWith("new String[]") || text.startsWith("String[]")) return false
        return text.startsWith("\"") || interpolated || splitSqlConcatOperands(text).size > 1
    }

    // Go's argumentIsUntrustedShape, with the two recall fixes described on
    // the object: a constant-only interpolation falls through to the
    // concatenation check, and a parenthesized operand is classified by its
    // own operands.
    private fun shape(text: String, interpolated: Boolean): Shape {
        if (text.isEmpty() || text == "null") return Shape.STATIC
        if (interpolated && !sqlInterpolationUsesOnlySchemaConstants(text)) return Shape.INTERPOLATED
        val operands = splitSqlConcatOperands(text)
        // Go calls any constant-only interpolation static; only a
        // concatenation beside it can make the command computed.
        if (operands.size <= 1 && interpolated) return Shape.STATIC
        val verdict = operandsVerdict(operands.ifEmpty { listOf(text) })
        return if (verdict.goStatic || verdict.constant) Shape.STATIC else Shape.COMPUTED
    }

    // How Go and the code itself see an operand (or a list of `+` operands).
    // [goStatic] is Go's sqlStaticOperand: Go reports the command as computed
    // when an operand is not goStatic. [constant] says the operand really is
    // constant text: a complete string literal, `null`, or a constant-looking
    // name. The two differ on a literal holding a `$` that is not a
    // non-constant template (`"echo \$HOME"`, `"price $5"`), which is constant
    // but not goStatic, and on a call chain on a literal without a `$`
    // (`"ls %s".format(dir)`), which is goStatic but computed. The command is
    // reported when Go would report it and it is not all constant text: when
    // Go reports only because of a constant literal's `$`, its message is
    // false; when a chain beside that literal splices in data, it is true.
    private data class Verdict(val goStatic: Boolean, val constant: Boolean)

    private fun operandsVerdict(operands: List<String>): Verdict {
        val verdicts = operands.map(::operandVerdict)
        return Verdict(verdicts.all { it.goStatic }, verdicts.all { it.constant })
    }

    // Go's sqlStaticOperand, plus whether the operand is constant text. A
    // parenthesized group is classified by its own operands.
    private fun operandVerdict(operand: String): Verdict {
        var text = operand.trim()
        var group = false
        while (text.startsWith("(") && text.endsWith(")")) {
            val inner = text.substring(1, text.length - 1).trim()
            if (inner.isEmpty()) break
            if (closingParen(text) == text.length - 1) group = true
            text = inner
        }
        if (group) {
            val inner = splitSqlConcatOperands(text)
            if (inner.size > 1) return operandsVerdict(inner)
        }
        if (text == "null") return Verdict(goStatic = true, constant = true)
        if (text.startsWith("\"")) {
            return Verdict(goStatic = !text.contains('$'), constant = isSingleStringLiteral(text))
        }
        val constant = sqlSchemaConstantName(sqlLastIdentifierSegment(text))
        return Verdict(constant, constant)
    }

    // Whether [text] is exactly one string literal: the literal that opens at
    // index 0 closes at the last character. A call chain on a literal
    // (`"ls %1\$s".format(dir)`) is not.
    private fun isSingleStringLiteral(text: String): Boolean =
        text.startsWith("\"") && stringLiteralEnd(text, 0) == text.length

    // The index just past the string literal (plain or raw) that opens at
    // [start], or -1 when it does not close. `${...}` template blocks are
    // skipped, including string and char literals nested inside them.
    private fun stringLiteralEnd(text: String, start: Int): Int {
        val raw = text.startsWith("\"\"\"", start)
        var i = start + if (raw) 3 else 1
        while (i < text.length) {
            val ch = text[i]
            when {
                raw && text.startsWith("\"\"\"", i) -> {
                    // A raw string closes on its last run of quotes.
                    i += 3
                    while (i < text.length && text[i] == '"') i++
                    return i
                }
                !raw && ch == '\\' -> i += 2
                !raw && ch == '"' -> return i + 1
                ch == '$' && text.startsWith("{", i + 1) -> {
                    i = templateBlockEnd(text, i + 2)
                    if (i < 0) return -1
                }
                else -> i++
            }
        }
        return -1
    }

    // The index just past the `}` that closes a `${` block whose body starts
    // at [start], or -1.
    private fun templateBlockEnd(text: String, start: Int): Int {
        var depth = 0
        var i = start
        while (i < text.length) {
            when (text[i]) {
                '"' -> {
                    i = stringLiteralEnd(text, i)
                    if (i < 0) return -1
                    continue
                }
                '\'' -> {
                    i++
                    while (i < text.length && text[i] != '\'') i += if (text[i] == '\\') 2 else 1
                }
                '{' -> depth++
                '}' -> if (depth == 0) return i + 1 else depth--
            }
            i++
        }
        return -1
    }

    // The index of the `)` closing the `(` that starts [text], or -1.
    private fun closingParen(text: String): Int {
        var closing = -1
        scanSqlOutsideStrings(text) { i, ch, depth ->
            if (ch == ')' && depth == 0) {
                closing = i
                true
            } else {
                false
            }
        }
        return closing
    }
}
