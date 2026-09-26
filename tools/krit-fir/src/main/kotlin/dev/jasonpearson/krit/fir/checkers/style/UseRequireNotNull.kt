package dev.jasonpearson.krit.fir.checkers.style

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
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
import org.jetbrains.kotlin.fir.expressions.FirEqualityOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.types.canBeNull
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags `require(x != null)` (or `require(null != x)`, optionally with a
// trailing message lambda): `requireNotNull(x)` states the intent and returns
// the value as non-null.
//
// Mirrors the Go UseRequireNotNull rule, which matches the call's written
// shape: a call named `require` with exactly one value argument in the
// parentheses (a trailing lambda is not a value argument) whose expression,
// after unwrapping parentheses, is `a != b` with one operand spelled exactly
// `null`. Go skips the call when source inference resolves the other operand
// as a known non-nullable type. FIR keeps the written-shape match, except that
// a parenthesized `(null)` still counts as `null` (Go misses it; UseCheckNotNull
// reports the same shape), and decides the rest semantically:
// - the call must resolve to `kotlin.require`, so a local, member or
//   same-package `require` lookalike is not flagged (Go reports any callee
//   named `require`), while an import alias of `kotlin.require` is (Go
//   matches the written name only);
// - the operand is skipped when its type cannot be null, for any expression
//   (Go can only resolve a plain name, so it reports `obj.nonNull != null`)
//   and not when it can (a type parameter with a nullable bound, which Go
//   resolves as non-nullable).
// The golden data pins both directions.
internal object UseRequireNotNull : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UseRequireNotNull"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(UseRequireNotNull)
    }

    private val kotlinRequire = CallableId(FqName("kotlin"), Name.identifier("require"))
    private val VALUE = Name.identifier("value")

    private const val MESSAGE = "Use 'requireNotNull(x)' instead of 'require(x != null)'."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId != kotlinRequire) return
        val source = expression.source ?: return
        val operandIndex = writtenNonNullOperandIndex(source) ?: return

        val condition = expression.resolvedArgumentMapping
            ?.entries
            ?.firstOrNull { it.value.name == VALUE }
            ?.key
            ?.let(::unwrapArgument)
            ?: expression.argumentList.arguments.firstOrNull()?.let(::unwrapArgument)
            ?: return
        val equality = condition as? FirEqualityOperatorCall ?: return
        if (equality.operation != FirOperation.NOT_EQ) return
        val operand = equality.argumentList.arguments.getOrNull(operandIndex) ?: return
        if (isNonNullable(operand)) return

        report(source, MESSAGE)
    }

    private fun unwrapArgument(argument: FirExpression): FirExpression =
        if (argument is FirWrappedArgumentExpression) argument.expression else argument

    context(context: CheckerContext)
    private fun isNonNullable(operand: FirExpression): Boolean {
        val type = runCatching { operand.resolvedType }.getOrNull() ?: return false
        return !type.canBeNull(context.session)
    }

    // Matches the call's written shape the way Go does, and returns the index
    // (0 = left, 1 = right) of the operand compared with `null`: the call's
    // parentheses hold exactly one value argument whose expression, after
    // unwrapping parentheses, is a `!=` comparison with one side spelled
    // `null`. Returns null when the shape does not match.
    private fun writtenNonNullOperandIndex(source: KtSourceElement): Int? {
        val call = callExpressionNode(source, source.lighterASTNode) ?: return null
        val argumentList = significantChildren(source, call)
            .firstOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT_LIST } ?: return null
        val argument = lightChildren(source, argumentList)
            .filter { it.tokenType == KtNodeTypes.VALUE_ARGUMENT }
            .singleOrNull() ?: return null
        val value = significantChildren(source, argument).firstOrNull {
            it.tokenType != KtNodeTypes.VALUE_ARGUMENT_NAME &&
                it.tokenType != KtTokens.EQ &&
                it.tokenType != KtTokens.MUL
        }?.let { unwrapLightParens(source, it) } ?: return null
        if (value.tokenType != KtNodeTypes.BINARY_EXPRESSION) return null
        // Comments are skipped too: `x != /* c */ null` still compares x with
        // null. Go reads the operator as the second child and the operand as
        // the last, so it also reports a comment after the operator, and
        // misses one before it (golden UseRequireNotNullComments).
        val parts = significantChildren(source, value)
        if (parts.size != 3) return null
        val (left, operator, right) = parts
        if (operator.tokenType != KtNodeTypes.OPERATION_REFERENCE || lightText(source, operator) != "!=") return null
        // A parenthesized `(null)` is still the null literal, as
        // UseCheckNotNull reads it; Go compares the raw text and misses it
        // (golden UseRequireNotNullParenthesizedNull).
        return when {
            isNullLiteral(source, left) -> 1
            isNullLiteral(source, right) -> 0
            else -> null
        }
    }

    private fun isNullLiteral(source: KtSourceElement, node: LighterASTNode): Boolean =
        lightText(source, unwrapLightParens(source, node)) == "null"

    // The CALL_EXPRESSION of a call's source: the source itself, or the
    // selector of a qualified call (`kotlin.require(...)`).
    private fun callExpressionNode(source: KtSourceElement, node: LighterASTNode): LighterASTNode? = when (node.tokenType) {
        KtNodeTypes.CALL_EXPRESSION -> node
        KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION ->
            significantChildren(source, node).lastOrNull()?.let { callExpressionNode(source, it) }
        else -> null
    }
}
