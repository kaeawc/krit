package dev.jasonpearson.krit.fir.checkers.security

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.significantChildren
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
 * - a concatenation is reported as computed when an operand is neither a
 *   string literal, `null`, nor a constant-looking name.
 *
 * Deliberate differences from Go, pinned by goldens and listed in the PR:
 * - Recall: Go requires the syntactic receiver `Runtime.getRuntime()` (or
 *   `java.lang.Runtime.getRuntime()`, or an import alias of it) and resolves
 *   `Runtime` by name. FIR reads the resolved call, so it also reports exec on
 *   a stored Runtime, on an implicit Runtime receiver
 *   (`with(Runtime.getRuntime()) { exec(..) }`, an extension on Runtime), on a
 *   parenthesized receiver, and through a typealias or a statically imported
 *   getRuntime.
 * - Recall: Go treats a parenthesized operand that starts with a string
 *   literal (`"ls " + ("-la " + dir)`) as static text, and treats an argument
 *   whose interpolations all name constants as static even when it also
 *   concatenates a non-static operand (`"${BASE_DIR}/ls " + dir`). FIR
 *   classifies the parenthesized group's own operands, and falls through to
 *   the concatenation check after the constant-only interpolation exemption.
 * - Precision: Go reports exec(String[]) when the array argument's text
 *   contains an interpolation or a top-level `+` (`listOf("ls", "$dir")
 *   .toTypedArray()`), and reports a plain literal holding an escaped `\$`
 *   (`"echo \$HOME"`) as computed. Neither passes a computed String command.
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
        return text.startsWith("\"") || interpolated || splitConcatOperands(text).size > 1
    }

    // Go's argumentIsUntrustedShape, with the two recall fixes described on
    // the object: a constant-only interpolation falls through to the
    // concatenation check, and a parenthesized operand is classified by its
    // own operands.
    private fun shape(text: String, interpolated: Boolean): Shape {
        if (text.isEmpty() || text == "null") return Shape.STATIC
        if (interpolated && !interpolationUsesOnlyConstants(text)) return Shape.INTERPOLATED
        val operands = splitConcatOperands(text)
        if (operands.size > 1) {
            return if (operands.all(::staticOperand)) Shape.STATIC else Shape.COMPUTED
        }
        // Go calls any constant-only interpolation static; only a
        // concatenation beside it (above) can make the command computed.
        if (interpolated) return Shape.STATIC
        return if (staticOperand(text)) Shape.STATIC else Shape.COMPUTED
    }

    // Go's sqlStaticOperand. A `$` inside a literal operand is never an
    // interpolation here (an argument with a non-constant interpolation was
    // already reported), so a literal is static whatever it holds; Go calls a
    // literal with an escaped `\$` computed.
    private fun staticOperand(operand: String): Boolean {
        var text = operand.trim()
        var group = false
        while (text.startsWith("(") && text.endsWith(")")) {
            val inner = text.substring(1, text.length - 1).trim()
            if (inner.isEmpty()) break
            if (closingParen(text) == text.length - 1) group = true
            text = inner
        }
        if (group) {
            val inner = splitConcatOperands(text)
            if (inner.size > 1) return inner.all(::staticOperand)
        }
        if (text == "null") return true
        if (text.startsWith("\"")) return true
        return constantName(lastIdentifierSegment(text))
    }

    // Go's splitSQLConcatOperands: split on `+` outside string literals and
    // parentheses. Returns an empty list when there is no top-level `+`.
    private fun splitConcatOperands(text: String): List<String> {
        val out = mutableListOf<String>()
        var start = 0
        scanOutsideStrings(text) { i, ch, depth ->
            if (ch == '+' && depth == 0) {
                out += text.substring(start, i).trim()
                start = i + 1
            }
            false
        }
        if (out.isEmpty()) return emptyList()
        out += text.substring(start).trim()
        return out
    }

    // The index of the `)` closing the `(` that starts [text], or -1.
    private fun closingParen(text: String): Int {
        var closing = -1
        scanOutsideStrings(text) { i, ch, depth ->
            if (ch == ')' && depth == 0) {
                closing = i
                true
            } else {
                false
            }
        }
        return closing
    }

    // Walks [text] the way Go's splitSQLConcatOperands does, calling [visit]
    // with each character outside string literals and the parenthesis depth
    // after it; stops when [visit] returns true.
    private inline fun scanOutsideStrings(text: String, visit: (Int, Char, Int) -> Boolean) {
        var depth = 0
        var inString = false
        var raw = false
        var escaped = false
        var i = 0
        while (i < text.length) {
            val ch = text[i]
            if (inString) {
                if (raw) {
                    if (text.startsWith("\"\"\"", i)) {
                        inString = false
                        raw = false
                        i += 2
                    }
                } else if (escaped) {
                    escaped = false
                } else if (ch == '\\') {
                    escaped = true
                } else if (ch == '"') {
                    inString = false
                }
                i++
                continue
            }
            when (ch) {
                '"' -> {
                    inString = true
                    if (text.startsWith("\"\"\"", i)) {
                        raw = true
                        i += 2
                    }
                }
                '(' -> depth++
                ')' -> if (depth > 0) depth--
            }
            if (ch != '"' && visit(i, ch, depth)) return
            i++
        }
    }

    private val interpolationIdentifier = Regex("""\$\{?\s*([A-Za-z_][A-Za-z0-9_.]*)""")

    // Go's sqlInterpolationUsesOnlyStaticSchemaConstants, over the argument's
    // source text.
    private fun interpolationUsesOnlyConstants(text: String): Boolean {
        val matches = interpolationIdentifier.findAll(text).toList()
        if (matches.isEmpty()) return false
        return matches.all { constantName(lastIdentifierSegment(it.groupValues[1])) }
    }

    // Go's sqlLastIdentifierSegment.
    private fun lastIdentifierSegment(value: String): String {
        var text = value.trim().removeSuffix(")")
        val dot = text.lastIndexOf('.')
        if (dot >= 0) text = text.substring(dot + 1)
        return text.trim('`', ' ')
    }

    // Go's sqlSchemaConstantName.
    private fun constantName(name: String): Boolean {
        if (name.isEmpty()) return false
        if (name.startsWith("TABLE_") || name.startsWith("COLUMN_")) return true
        if (name.endsWith("_TABLE") || name.endsWith("_COLUMN") || name.endsWith("_KEY")) return true
        return name.uppercase() == name && name.contains('_')
    }
}
