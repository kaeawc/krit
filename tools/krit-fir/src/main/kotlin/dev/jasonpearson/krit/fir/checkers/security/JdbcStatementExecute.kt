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
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.utils.isConst
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirStringConcatenationCall
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.expressions.arguments
import org.jetbrains.kotlin.fir.references.FirSuperReference
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.references.toResolvedNamedFunctionSymbol
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.fir.types.isNothingOrNullableNothing
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds
import org.jetbrains.kotlin.types.ConstantValueKind

/**
 * Port of the Go JdbcStatementExecute rule: an `execute`, `executeQuery`,
 * `executeUpdate`, or `executeLargeUpdate` call on a `java.sql.Statement`
 * whose SQL (the first positional argument) is interpolated or computed,
 * reported on that argument with Go's message for its shape.
 *
 * Mirrored from Go:
 * - the call has an explicit receiver; an unqualified call (`with(stmt) {
 *   execute(sql) }`) is not reported, as Go needs a receiver to type;
 * - `PreparedStatement` (and `CallableStatement`) receivers are skipped: Go
 *   accepts only a receiver it proves is exactly a `Statement`, and the
 *   String overloads throw on a prepared statement instead of running the SQL;
 * - the SQL argument's shape is Go's, read from the argument's source text:
 *   a string template anywhere in it is "interpolation" unless every
 *   interpolated name's last segment is a schema-constant name
 *   (`TABLE_*`, `COLUMN_*`, `*_TABLE`, `*_COLUMN`, `*_KEY`, or upper case with
 *   an underscore); otherwise a top-level `+` concatenation, or a single
 *   operand, is "computed" unless every operand is `null`, a string literal
 *   without `$`, or a schema-constant name.
 *
 * Deliberate differences from Go, pinned by goldens and listed in the PR:
 * - Recall: FIR reads the receiver's resolved type. Go proves it only for a
 *   bare name resolved to `Statement`, a bare name declared in the enclosing
 *   function with a `conn.createStatement()` initializer, or
 *   `conn.createStatement()` on a bare name resolved to `Connection`. FIR
 *   also reports a `Statement` from `use { it.execute(..) }`, a property
 *   chain (`this.stmt`, `holder.stmt`), a call chain
 *   (`DriverManager.getConnection(url).createStatement()`), a cast or smart
 *   cast, a function returning `Statement`, and a `Statement` subtype.
 * - Precision: a "computed" SQL argument that is a compile-time or local
 *   constant (a `const val`, a local `val` initialized with a constant, a
 *   literal with an escaped `\$`, or a `+` of those) is not built from
 *   non-static data, so it is not reported. A receiver that is not a
 *   `Statement` (a lookalike Go binds to an earlier `createStatement()`
 *   declaration of the same name in another scope) and a project extension
 *   function named like the JDBC methods are not reported either.
 */
internal object JdbcStatementExecute : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "JdbcStatementExecute"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(JdbcStatementExecute)
    }

    private const val INTERPOLATED_MESSAGE =
        "JDBC Statement SQL uses string interpolation. Use PreparedStatement with bind parameters."
    private const val COMPUTED_MESSAGE =
        "JDBC Statement SQL is built with non-static concatenation. Use PreparedStatement with bind parameters."

    private val executeMethods = setOf("execute", "executeQuery", "executeUpdate", "executeLargeUpdate")
        .map { Name.identifier(it) }.toSet()

    private val javaSql = FqName("java.sql")
    private val statement = ClassId(javaSql, Name.identifier("Statement"))
    private val nullablePreparedStatement = ClassId(javaSql, Name.identifier("PreparedStatement"))
        .constructClassLikeType(emptyArray(), isMarkedNullable = true)

    private val stringPlus = CallableId(StandardClassIds.String, Name.identifier("plus"))
    private const val MAX_DEPTH = 16

    private enum class Shape { STATIC, INTERPOLATED, COMPUTED }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedNamedFunctionSymbol() ?: return
        if (callee.name !in executeMethods) return
        val receiver = expression.explicitReceiver ?: return
        if ((receiver as? FirQualifiedAccessExpression)?.calleeReference is FirSuperReference) return
        // A member of Statement or of a Statement subtype; an extension named
        // like the JDBC method is a project helper, not the driver call.
        if (callee.resolvedReceiverType != null) return
        val owner = callee.getContainingClassSymbol() as? FirClassSymbol<*> ?: return
        if (!isStatementClass(owner)) return
        if (isPreparedStatement(expression.dispatchReceiver?.resolvedType)) return

        val anchor = expression.calleeReference.source ?: return
        if (anchor.kind !is KtRealSourceElementKind) return
        val argument = firstPositionalArgument(anchor) ?: return
        val message = when (goShape(anchor, argument)) {
            Shape.STATIC -> return
            Shape.INTERPOLATED -> INTERPOLATED_MESSAGE
            Shape.COMPUTED -> {
                val argumentSource = lightSourceOf(argument, anchor)
                val value = firArgumentAt(expression, argumentSource)
                if (value != null && isStaticValue(value, depth = 0)) return
                COMPUTED_MESSAGE
            }
        }
        report(lightSourceOf(argument, anchor), message)
    }

    context(context: CheckerContext)
    private fun isStatementClass(owner: FirClassSymbol<*>): Boolean =
        owner.classId == statement ||
            lookupSuperTypes(owner, lookupInterfaces = true, deep = true, useSiteSession = context.session)
                .any { it.lookupTag.classId == statement }

    context(context: CheckerContext)
    private fun isPreparedStatement(type: ConeKotlinType?): Boolean {
        if (type == null || type.isNothingOrNullableNothing) return false
        return type.isSubtypeOf(nullablePreparedStatement, context.session)
    }

    // Go's first positional argument: the first VALUE_ARGUMENT without a name,
    // in the argument list of the call whose callee is [calleeSource]. A
    // trailing lambda is outside the list, as in tree-sitter.
    private fun firstPositionalArgument(calleeSource: KtSourceElement): LighterASTNode? {
        val tree = calleeSource.treeStructure
        val call = tree.getParent(calleeSource.lighterASTNode) ?: return null
        if (call.tokenType != KtNodeTypes.CALL_EXPRESSION) return null
        val list = lightChildren(calleeSource, call)
            .firstOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT_LIST } ?: return null
        for (argument in lightChildren(calleeSource, list)) {
            if (argument.tokenType != KtNodeTypes.VALUE_ARGUMENT) continue
            val parts = significantChildren(calleeSource, argument)
            if (parts.any { it.tokenType == KtNodeTypes.VALUE_ARGUMENT_NAME }) continue
            return parts.lastOrNull { it.tokenType != KtTokens.MUL && it.tokenType != KtTokens.EQ } ?: return null
        }
        return null
    }

    // The resolved argument whose source lies within [argumentSource].
    private fun firArgumentAt(expression: FirFunctionCall, argumentSource: KtSourceElement): FirExpression? =
        expression.argumentList.arguments.asSequence()
            .map { if (it is FirWrappedArgumentExpression) it.expression else it }
            .firstOrNull { value ->
                val source = value.source ?: return@firstOrNull false
                source.startOffset >= argumentSource.startOffset && source.endOffset <= argumentSource.endOffset
            }

    // Whether [expression] is a String (or null) value fixed at compile time
    // or by a local val holding one.
    private fun isStaticValue(expression: FirExpression, depth: Int): Boolean {
        if (depth > MAX_DEPTH) return false
        return when (expression) {
            is FirLiteralExpression -> expression.kind == ConstantValueKind.String || expression.kind == ConstantValueKind.Null
            is FirSmartCastExpression -> isStaticValue(expression.originalExpression, depth + 1)
            is FirStringConcatenationCall -> expression.argumentList.arguments.all { isStaticValue(it, depth + 1) }
            is FirPropertyAccessExpression -> {
                val property = expression.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol
                    ?: return false
                when {
                    property.isConst -> true
                    property.isLocal && property.isVal && !property.hasDelegate ->
                        property.resolvedInitializer?.let { isStaticValue(it, depth + 1) } == true
                    else -> false
                }
            }
            is FirFunctionCall -> {
                val symbol = expression.calleeReference.toResolvedCallableSymbol()
                symbol?.callableId == stringPlus &&
                    expression.explicitReceiver?.let { isStaticValue(it, depth + 1) } == true &&
                    expression.argumentList.arguments.all { isStaticValue(it, depth + 1) }
            }
            else -> false
        }
    }

    // Go's argumentIsUntrustedShape on the argument's source text.
    private fun goShape(anchor: KtSourceElement, argument: LighterASTNode): Shape {
        val inner = unwrapLightParens(anchor, argument)
        val text = lightText(anchor, inner).trim()
        if (text.isEmpty() || text == "null") return Shape.STATIC
        if (containsTemplateEntry(anchor, inner)) {
            return if (interpolationUsesOnlySchemaConstants(text)) Shape.STATIC else Shape.INTERPOLATED
        }
        val operands = splitConcatOperands(text)
        if (operands.size > 1) {
            return if (operands.all(::staticOperand)) Shape.STATIC else Shape.COMPUTED
        }
        return if (staticOperand(text)) Shape.STATIC else Shape.COMPUTED
    }

    private val templateEntries = setOf(KtNodeTypes.SHORT_STRING_TEMPLATE_ENTRY, KtNodeTypes.LONG_STRING_TEMPLATE_ENTRY)

    private fun containsTemplateEntry(anchor: KtSourceElement, node: LighterASTNode): Boolean {
        if (node.tokenType in templateEntries) return true
        return lightChildren(anchor, node).any { containsTemplateEntry(anchor, it) }
    }

    // Go's sqlStaticOperand.
    private fun staticOperand(operand: String): Boolean {
        var text = operand.trim()
        while (text.startsWith("(") && text.endsWith(")")) {
            val inner = text.substring(1, text.length - 1).trim()
            if (inner.isEmpty()) break
            text = inner
        }
        if (text == "null") return true
        if (text.startsWith("\"")) return !text.contains('$')
        return schemaConstantName(lastIdentifierSegment(text))
    }

    // Go's splitSQLConcatOperands: split on `+` outside strings and outside
    // parentheses; a single operand yields an empty list.
    private fun splitConcatOperands(text: String): List<String> {
        val out = mutableListOf<String>()
        var start = 0
        var depth = 0
        var inString = false
        var raw = false
        var escaped = false
        var i = 0
        while (i < text.length) {
            val ch = text[i]
            if (inString) {
                if (raw) {
                    if (i + 2 < text.length && text.startsWith("\"\"\"", i)) {
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
                    if (i + 2 < text.length && text.startsWith("\"\"\"", i)) {
                        raw = true
                        i += 2
                    }
                }
                '(' -> depth++
                ')' -> if (depth > 0) depth--
                '+' -> if (depth == 0) {
                    out += text.substring(start, i).trim()
                    start = i + 1
                }
            }
            i++
        }
        if (out.isEmpty()) return emptyList()
        out += text.substring(start).trim()
        return out
    }

    private val interpolatedName = Regex("""\$\{?[\t\n\u000C\r ]*([A-Za-z_][A-Za-z0-9_.]*)""")

    // Go's sqlInterpolationUsesOnlyStaticSchemaConstants.
    private fun interpolationUsesOnlySchemaConstants(text: String): Boolean {
        val matches = interpolatedName.findAll(text).toList()
        if (matches.isEmpty()) return false
        return matches.all { schemaConstantName(lastIdentifierSegment(it.groupValues[1])) }
    }

    // Go's sqlLastIdentifierSegment.
    private fun lastIdentifierSegment(value: String): String {
        var text = value.trim().removeSuffix(")")
        val dot = text.lastIndexOf('.')
        if (dot >= 0) text = text.substring(dot + 1)
        return text.trim('`', ' ')
    }

    // Go's sqlSchemaConstantName.
    private fun schemaConstantName(name: String): Boolean {
        if (name.isEmpty()) return false
        if (name.startsWith("TABLE_") || name.startsWith("COLUMN_")) return true
        if (name.endsWith("_TABLE") || name.endsWith("_COLUMN") || name.endsWith("_KEY")) return true
        return name.map { it.uppercaseChar() }.joinToString("") == name && name.contains('_')
    }
}
