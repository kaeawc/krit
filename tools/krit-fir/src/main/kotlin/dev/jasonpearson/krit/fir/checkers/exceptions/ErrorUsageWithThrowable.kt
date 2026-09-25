package dev.jasonpearson.krit.fir.checkers.exceptions

import com.intellij.lang.LighterASTNode
import com.intellij.util.diff.FlyweightCapableTreeStructure
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.contracts.description.LogicOperationKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirBooleanOperatorExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirJump
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirStatement
import org.jetbrains.kotlin.fir.expressions.FirThrowExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.FirWhenBranch
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.FirWhenSubjectExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.references.toResolvedVariableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.fir.types.isNothingOrNullableNothing
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
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
// `kotlin.error`, so a local, member, imported or same-package `error(...)`
// is not flagged, and neither is a non-Throwable property read off an exception
// (`e.stackTrace`). Arguments Go cannot type (a typealias, a bounded type
// parameter, `!!`, Elvis, `when`, a Java getter) are deliberate recall
// additions. The golden data pins both directions.
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

        // A string literal never reaches here: String is not a Throwable.
        val argText = argumentSource.trim()
        if (argText.isEmpty()) return
        report(source, "error($argText) passes a Throwable. Use throw instead, or pass the message string.")
    }

    private fun unwrapArgument(argument: FirExpression): FirExpression =
        if (argument is FirWrappedArgumentExpression) argument.expression else argument

    context(context: CheckerContext)
    private fun isThrowable(argument: FirExpression): Boolean {
        val type = runCatching { argument.resolvedType }.getOrNull() ?: return false
        if (isThrowableType(type)) return true
        // An unstable smart cast on a `var` property keeps the declared type as
        // the expression's resolved type. The `is` check still proves the value
        // is a Throwable, and the Go rule reports it, so test the smart-cast
        // types too.
        if (argument is FirSmartCastExpression) {
            return argument.upperTypesFromSmartCast.any { isThrowableType(it) }
        }
        return capturedVarIsCheckedThrowable(argument)
    }

    // A local `var` captured and reassigned by a closure gets no smart cast at
    // all: FIR types `error(v)`'s argument as the declared type, with no flow
    // information. The Go rule still narrows `v` from an enclosing `is` check,
    // and the value passed is the Throwable that check proved, so mirror the Go
    // narrowing forms for that one case: a positive `if (v is T)` (also inside
    // an `&&` chain) or `when (v) { is T -> }` branch around the call, or an
    // earlier `if (v !is T) return` in an enclosing block. The variable is
    // matched by symbol, so a shadowing declaration is not confused with it.
    // An assignment to the variable between the check and the call voids the
    // proof: the value passed is then no longer the one the check tested.
    context(context: CheckerContext)
    private fun capturedVarIsCheckedThrowable(argument: FirExpression): Boolean {
        val symbol = localVarSymbol(argument) ?: return false
        val path = context.containingElements
        for (i in path.indices.reversed()) {
            val parent = path[i]
            val child = path.getOrNull(i + 1) ?: continue
            when (parent) {
                is FirWhenBranch -> {
                    if (child !== parent.result) continue
                    val whenExpression = path.getOrNull(i - 1) as? FirWhenExpression ?: continue
                    if (conditionProvesThrowable(parent.condition, whenExpression, symbol) &&
                        !reassignedBelow(path, i, symbol)
                    ) return true
                }
                is FirBlock -> {
                    val index = parent.statements.indexOfFirst { it === child }
                    if (index <= 0) continue
                    val guard = parent.statements.subList(0, index).indexOfLast { earlyExitProvesThrowable(it, symbol) }
                    if (guard >= 0 &&
                        parent.statements.subList(guard + 1, index).none { assigns(it, symbol) } &&
                        !reassignedBelow(path, i, symbol)
                    ) return true
                }
            }
        }
        return false
    }

    // Whether a statement that runs before the call, in a block nested below
    // path level [level], assigns [symbol].
    private fun reassignedBelow(path: List<FirElement>, level: Int, symbol: FirPropertySymbol): Boolean {
        for (k in level + 1 until path.size - 1) {
            val block = path[k] as? FirBlock ?: continue
            val index = block.statements.indexOfFirst { it === path[k + 1] }
            if (index > 0 && block.statements.subList(0, index).any { assigns(it, symbol) }) return true
        }
        return false
    }

    // Whether [element] contains an assignment to [symbol], including one in a
    // nested lambda (conservative: it may run before the call).
    private fun assigns(element: FirElement, symbol: FirPropertySymbol): Boolean {
        var found = false
        element.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (!found) element.acceptChildren(this)
            }

            override fun visitVariableAssignment(variableAssignment: FirVariableAssignment) {
                if ((variableAssignment.lValue as? FirPropertyAccessExpression)?.calleeReference?.toResolvedVariableSymbol() == symbol) {
                    found = true
                } else {
                    variableAssignment.acceptChildren(this)
                }
            }
        })
        return found
    }

    private fun localVarSymbol(expression: FirExpression): FirPropertySymbol? {
        val access = expression as? FirPropertyAccessExpression ?: return null
        if (access.explicitReceiver != null) return null
        val symbol = access.calleeReference.toResolvedVariableSymbol() as? FirPropertySymbol ?: return null
        return symbol.takeIf { it.isLocal && it.isVar }
    }

    // The symbol an `is` check tests: a plain local access, or the subject of
    // `when (v) { ... }`.
    private fun testedSymbol(expression: FirExpression, whenExpression: FirWhenExpression?): FirPropertySymbol? {
        if (expression is FirWhenSubjectExpression) {
            val initializer = whenExpression?.subjectVariable?.initializer ?: return null
            return testedSymbol(initializer, null)
        }
        val access = expression as? FirPropertyAccessExpression ?: return null
        if (access.explicitReceiver != null) return null
        return access.calleeReference.toResolvedVariableSymbol() as? FirPropertySymbol
    }

    context(context: CheckerContext)
    private fun conditionProvesThrowable(
        condition: FirExpression,
        whenExpression: FirWhenExpression,
        symbol: FirPropertySymbol,
    ): Boolean = when (condition) {
        is FirBooleanOperatorExpression ->
            condition.kind == LogicOperationKind.AND &&
                (conditionProvesThrowable(condition.leftOperand, whenExpression, symbol) ||
                    conditionProvesThrowable(condition.rightOperand, whenExpression, symbol))
        is FirTypeOperatorCall -> isCheckOn(condition, FirOperation.IS, whenExpression, symbol)
        else -> false
    }

    // `if (v !is T) return` (or throw, break, continue) before the call.
    context(context: CheckerContext)
    private fun earlyExitProvesThrowable(statement: FirStatement, symbol: FirPropertySymbol): Boolean {
        val whenExpression = statement as? FirWhenExpression ?: return false
        val branch = whenExpression.branches.singleOrNull() ?: return false
        val check = branch.condition as? FirTypeOperatorCall ?: return false
        if (!isCheckOn(check, FirOperation.NOT_IS, whenExpression, symbol)) return false
        val exits = branch.result.statements.lastOrNull() ?: return false
        return exits is FirJump<*> || exits is FirThrowExpression
    }

    context(context: CheckerContext)
    private fun isCheckOn(
        check: FirTypeOperatorCall,
        operation: FirOperation,
        whenExpression: FirWhenExpression,
        symbol: FirPropertySymbol,
    ): Boolean {
        if (check.operation != operation) return false
        val tested = check.argumentList.arguments.singleOrNull() ?: return false
        if (testedSymbol(tested, whenExpression) != symbol) return false
        return isThrowableType(check.conversionTypeRef.coneType)
    }

    context(context: CheckerContext)
    private fun isThrowableType(type: ConeKotlinType): Boolean {
        // `error(TODO())`: Nothing is a subtype of everything but is not a
        // Throwable being passed along.
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
