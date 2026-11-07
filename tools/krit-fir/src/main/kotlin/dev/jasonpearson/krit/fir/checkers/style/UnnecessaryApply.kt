package dev.jasonpearson.krit.fir.checkers.style

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.significantChildren
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirAnonymousInitializer
import org.jetbrains.kotlin.fir.declarations.FirFunction
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirCallableReferenceAccess
import org.jetbrains.kotlin.fir.expressions.FirCatch
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirImplicitInvokeCall
import org.jetbrains.kotlin.fir.expressions.FirLoop
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirStatement
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirTryExpression
import org.jetbrains.kotlin.fir.expressions.FirWhenBranch
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.isProperlyExhaustive
import org.jetbrains.kotlin.fir.expressions.impl.FirElseIfTrueCondition
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.isUnit
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags `receiver.apply { statement }` where `apply` adds nothing, with detekt 1.23.8's
// UnnecessaryApply semantics:
// - the callee is written `apply` and resolves to kotlin.apply, with an explicit receiver
//   (`r.apply { }`, `r?.apply { }`) and a trailing lambda;
// - the lambda holds exactly one statement: an assignment, `this`, a reference, a call or
//   an index access, or a dot-qualified chain of those (a `?.` chain, an operator, a
//   parenthesized expression and a declaration do not count);
// - exactly one name reference reads the apply receiver implicitly (an unqualified member,
//   or `this`), counted over the whole statement or, for an assignment, only its target.
//   Nested lambdas count too. A member read through the implicit receiver of an inner
//   scope (`with`, `run`, an outer class) does not;
// - the value of the apply call is not used: it is a statement of a block, and no
//   enclosing expression reads the value.
//
// The message is detekt's and the finding sits on the `apply` call name. Go has no type
// information, so it reports only the subset where `this` is the single receiver reference.
internal object UnnecessaryApply : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UnnecessaryApply"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(UnnecessaryApply)
    }

    private val applyId = CallableId(FqName("kotlin"), Name.identifier("apply"))
    private const val OMIT_MESSAGE = "apply expression can be omitted"
    private const val LET_MESSAGE = "apply can be replaced with let or an if"

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference
        if (callee.name.asString() != "apply") return
        if (callee.toResolvedCallableSymbol()?.callableId != applyId) return
        if (expression.explicitReceiver == null) return
        val lambda = (expression.argumentList.arguments.singleOrNull() as? FirAnonymousFunctionExpression)
            ?.takeIf { it.isTrailingLambda } ?: return
        val function = lambda.anonymousFunction
        val statement = singleStatement(function) ?: return
        val scan = scanRange(statement) ?: return
        if (countImplicitReceiverReferences(statement, function, scan) != 1) return

        val path = context.containingElements.filter { it !== expression }
        val outer = path.lastOrNull()
        val safe = outer is FirSafeCallExpression && outer.selector === expression
        val top: FirElement = if (safe) outer else expression
        val ancestors = if (safe) path.dropLast(1) else path
        if (isUsedAsStatementValue(ancestors, top, directChild = true)) return

        report(callee.source ?: expression.source, if (safe) LET_MESSAGE else OMIT_MESSAGE)
    }

    // The lambda's only statement, without the implicit return K2 wraps around the last one.
    private fun singleStatement(function: FirAnonymousFunction): FirStatement? {
        val statements = function.body?.statements.orEmpty().filterNot { isSyntheticUnit(it) }
        val only = statements.singleOrNull() ?: return null
        return if (only is FirReturnExpression && isImplicitReturn(only)) only.result else only
    }

    private fun isImplicitReturn(expression: FirReturnExpression): Boolean =
        expression.source?.kind is KtFakeSourceElementKind.ImplicitReturn

    // K2 appends a Unit after an indexed assignment, and may wrap the last statement in a Unit return.
    private fun isSyntheticUnit(statement: FirStatement): Boolean =
        statement.source?.kind is KtFakeSourceElementKind.ImplicitUnit ||
            (statement is FirReturnExpression && statement.source?.kind is KtFakeSourceElementKind &&
                statement.result.source?.kind is KtFakeSourceElementKind && statement.result.resolvedType.isUnit)

    // The offsets whose name references count: the target of an assignment, else the whole
    // statement. Null when the statement is not a shape detekt accepts.
    private fun scanRange(statement: FirStatement): IntRange? {
        val source = statement.source ?: return null
        val parent = source.treeStructure.getParent(source.lighterASTNode)
        if (parent?.tokenType == KtNodeTypes.PARENTHESIZED) return null
        return when (source.elementType) {
            KtNodeTypes.CALL_EXPRESSION,
            KtNodeTypes.DOT_QUALIFIED_EXPRESSION,
            KtNodeTypes.REFERENCE_EXPRESSION,
            KtNodeTypes.THIS_EXPRESSION,
            KtNodeTypes.ARRAY_ACCESS_EXPRESSION,
            -> source.startOffset until source.endOffset
            KtNodeTypes.BINARY_EXPRESSION -> assignmentTarget(source)?.let { it.startOffset until it.endOffset }
            else -> null
        }
    }

    private fun assignmentTarget(source: KtSourceElement): KtSourceElement? {
        val children = significantChildren(source, source.lighterASTNode)
        val operation = children.firstOrNull { it.tokenType == KtNodeTypes.OPERATION_REFERENCE } ?: return null
        val token = significantChildren(source, operation).singleOrNull()?.tokenType ?: return null
        if (token !in KtTokens.ALL_ASSIGNMENTS) return null
        val target = children.firstOrNull() ?: return null
        return if (target === operation) null else lightSourceOf(target, source)
    }

    private fun countImplicitReceiverReferences(
        statement: FirStatement,
        function: FirAnonymousFunction,
        scan: IntRange,
    ): Int {
        val seen = HashSet<IntRange>()
        val skipped = HashSet<FirElement>()
        statement.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                when (element) {
                    // Detekt resolves no implicit receiver for `::member`.
                    is FirCallableReferenceAccess -> return
                    is FirThisReceiverExpression ->
                        if (element !in skipped && !element.isImplicit && boundToLambda(element, function)) record(element.source)
                    is FirQualifiedAccessExpression -> {
                        if (element !in skipped) count(element)
                        // Detekt does not count the variable or `this` that an implicit `invoke` call goes through.
                        if (element is FirImplicitInvokeCall) element.explicitReceiver?.let { skipped.add(it) }
                    }
                    else -> Unit
                }
                element.acceptChildren(this)
            }

            private fun count(access: FirQualifiedAccessExpression) {
                val receivers = listOfNotNull(access.dispatchReceiver, access.extensionReceiver)
                val implicit = receivers.any { it is FirThisReceiverExpression && it.isImplicit && boundToLambda(it, function) }
                if (!implicit) return
                val named = access.calleeReference.source?.takeIf { it.kind is KtRealSourceElementKind }
                record(named ?: access.source)
            }

            private fun record(source: KtSourceElement?) {
                if (source == null) return
                val range = source.startOffset until source.endOffset
                if (range.first >= scan.first && range.last <= scan.last) seen.add(range)
            }
        })
        return seen.size
    }

    private fun boundToLambda(receiver: FirThisReceiverExpression, function: FirAnonymousFunction): Boolean {
        val bound = receiver.calleeReference.boundSymbol ?: return false
        val owner: Any = bound
        return owner === function.symbol || owner === function.receiverParameter?.symbol
    }

    // True when the code around [child] reads the value of the expression. [ancestors] ends
    // at the parent of [child]. [directChild] is true for the apply call itself, which must be
    // a statement of a braced block (detekt checks the parent PSI node there).
    private fun isUsedAsStatementValue(ancestors: List<FirElement>, child: FirElement, directChild: Boolean): Boolean {
        val parent = ancestors.lastOrNull() ?: return true
        val above = ancestors.dropLast(1)
        return when (parent) {
            is FirBlock -> blockValueUsed(parent, above, child, directChild)
            is FirReturnExpression -> returnValueUsed(parent, above)
            is FirWhenBranch -> true
            else -> true
        }
    }

    private fun blockValueUsed(block: FirBlock, above: List<FirElement>, child: FirElement, directChild: Boolean): Boolean {
        val statements = block.statements
        val last = statements.lastOrNull { !isSyntheticUnit(it) }
        if (last !== child) return false
        if (directChild && block.source?.elementType != KtNodeTypes.BLOCK) return true
        val owner = above.lastOrNull() ?: return true
        val beyond = above.dropLast(1)
        return when (owner) {
            is FirAnonymousFunction -> owner.isLambda && !owner.returnTypeRef.coneType.isUnit
            is FirLoop -> false
            is FirBlock -> blockValueUsed(owner, beyond, block, directChild = false)
            is FirWhenBranch -> {
                val whenExpression = beyond.lastOrNull() as? FirWhenExpression ?: return true
                if (!canBeValue(whenExpression)) false else isUsedAsStatementValue(beyond.dropLast(1), whenExpression, directChild = false)
            }
            is FirCatch -> {
                val tryExpression = beyond.lastOrNull() as? FirTryExpression ?: return true
                isUsedAsStatementValue(beyond.dropLast(1), tryExpression, directChild = false)
            }
            is FirTryExpression ->
                if (owner.finallyBlock === block) false else isUsedAsStatementValue(beyond, owner, directChild = false)
            is FirFunction, is FirAnonymousInitializer -> false
            else -> true
        }
    }

    // The implicit return around the last statement of a lambda, or `return x`.
    private fun returnValueUsed(returnExpression: FirReturnExpression, above: List<FirElement>): Boolean {
        if (!isImplicitReturn(returnExpression)) return true
        val block = above.lastOrNull() as? FirBlock ?: return true
        val function = above.getOrNull(above.size - 2) as? FirAnonymousFunction ?: return true
        if (block !== function.body) return true
        return !function.returnTypeRef.coneType.isUnit
    }

    // A `when` can produce a value when it has an `else` or covers every case.
    private fun canBeValue(whenExpression: FirWhenExpression): Boolean =
        whenExpression.isProperlyExhaustive || whenExpression.branches.any { it.condition is FirElseIfTrueCondition }
}
