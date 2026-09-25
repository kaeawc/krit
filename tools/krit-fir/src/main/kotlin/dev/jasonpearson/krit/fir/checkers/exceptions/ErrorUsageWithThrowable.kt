package dev.jasonpearson.krit.fir.checkers.exceptions

import com.intellij.lang.LighterASTNode
import com.intellij.util.diff.FlyweightCapableTreeStructure
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.fir.types.isNothingOrNullableNothing
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds
import org.jetbrains.kotlin.util.getChildren

// Flags `error(throwable)`: the stdlib `kotlin.error(message: Any)` stringifies
// its argument into an IllegalStateException message, so passing a Throwable
// drops the original exception as the cause. Use `throw` instead, or pass a
// message string.
//
// Mirrors the Go ErrorUsageWithThrowable rule, which fires on a direct,
// unqualified `error(...)` call (callee is the bare identifier `error`) whose
// first argument is a Throwable. The Go rule decides "is a Throwable" from
// source type inference with a name-based fallback; FIR uses the argument's
// resolved (smart-cast aware) type instead, and requires the call to resolve to
// `kotlin.error`, so a local `error(...)` function or member is not flagged.
internal object ErrorUsageWithThrowable : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ErrorUsageWithThrowable"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(ErrorUsageWithThrowable)
    }
    private val kotlinError = CallableId(FqName("kotlin"), Name.identifier("error"))
    private val nullableThrowable = StandardClassIds.Throwable.constructClassLikeType(emptyArray(), isMarkedNullable = true)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId != kotlinError) return
        // Go only matches the bare `error(...)` call form: `kotlin.error(e)` or a
        // receiver-qualified call has a navigation expression as its callee.
        if (expression.explicitReceiver != null) return
        val source = expression.source ?: return
        val argumentSource = directErrorCallFirstArgument(source) ?: return

        val argument = unwrapArgument(expression.argumentList.arguments.singleOrNull() ?: return)
        if (!isThrowable(argument)) return

        val argText = argumentSource.trim()
        if (argText.isEmpty() || argText.startsWith("\"")) return
        report(source, "error($argText) passes a Throwable. Use throw instead, or pass the message string.")
    }

    private fun unwrapArgument(argument: FirExpression): FirExpression =
        if (argument is FirWrappedArgumentExpression) argument.expression else argument

    context(context: CheckerContext)
    private fun isThrowable(argument: FirExpression): Boolean {
        val type = runCatching { argument.resolvedType }.getOrNull() ?: return false
        // `error(null)`, `error(TODO())`: Nothing is a subtype of everything but
        // is not a Throwable being passed along.
        if (type.isNothingOrNullableNothing) return false
        return type.isSubtypeOf(nullableThrowable, context.session)
    }

    // Returns the source text of the first value argument's expression when the
    // call is the direct form `error(...)` — a CALL_EXPRESSION whose callee is
    // the plain identifier `error` — or null otherwise. The text matches what
    // the Go rule interpolates: the argument expression without any
    // `name =` label.
    private fun directErrorCallFirstArgument(source: KtSourceElement): String? {
        if (source.elementType != KtNodeTypes.CALL_EXPRESSION) return null
        val tree = source.treeStructure
        val children = source.lighterASTNode.getChildren(tree).filter { it.isMeaningful() }
        val calleeNode = children.firstOrNull() ?: return null
        if (calleeNode.tokenType != KtNodeTypes.REFERENCE_EXPRESSION) return null
        if (tree.toString(calleeNode).toString() != "error") return null
        val argumentList = children.firstOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT_LIST } ?: return null
        val firstArgument = argumentList.getChildren(tree)
            .firstOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT } ?: return null
        val expressionNode = valueArgumentExpression(firstArgument, tree) ?: return null
        return tree.toString(expressionNode).toString()
    }

    private fun valueArgumentExpression(
        argument: LighterASTNode,
        tree: FlyweightCapableTreeStructure<LighterASTNode>,
    ): LighterASTNode? = argument.getChildren(tree).firstOrNull {
        it.isMeaningful() &&
            it.tokenType != KtNodeTypes.VALUE_ARGUMENT_NAME &&
            it.tokenType != KtTokens.EQ &&
            it.tokenType != KtTokens.MUL
    }

    private fun LighterASTNode.isMeaningful(): Boolean =
        tokenType != KtTokens.WHITE_SPACE && tokenType !in KtTokens.COMMENTS
}
