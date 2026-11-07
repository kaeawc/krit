package dev.jasonpearson.krit.fir.checkers.style

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.lightText
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
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirComponentCall
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirFunctionTypeConversionExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirAnonymousFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.isUnit
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Port of detekt 1.23's UnnecessaryLet, which needs type resolution. The
// decision table is the one in the Go fallback (style_unnecessary_let.go):
//
//   more than one use of the lambda parameter       -> no finding
//   safe call (`a?.let`):
//     no trailing lambda                            -> no finding
//     parameter unused, call result unused          -> "can be replaced with a simple if"
//     parameter used once, body is `p.member...`    -> "can be omitted"
//   other receiver (`a.let`, bare `let`):
//     parameter unused (or no trailing lambda), or the body is `p.member...`,
//     or the call is not part of a call chain       -> "can be omitted"
//
// What resolution decides, and the Go fallback only guesses:
// - the call is kotlin.let called by that name (a member or local `let`, or an
//   import alias, is not);
// - the parameter's uses are counted by symbol, so an `it` that belongs to a
//   nested lambda, a shadowing local, or a named argument label is not a use,
//   and the destructured names of a `(a, b) ->` parameter count together;
// - a statement `a?.let { }` is "used" as a lambda's last statement unless the
//   lambda returns Unit.
// The shape of the body and of the call chain is read from the syntax tree, as
// detekt reads the PSI.
internal object UnnecessaryLet : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UnnecessaryLet"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(UnnecessaryLet)
    }

    private const val OMITTED = "let expression can be omitted"
    private const val SIMPLE_IF = "let expression can be replaced with a simple if"
    private val letId = CallableId(FqName("kotlin"), Name.identifier("let"))
    private val qualified = setOf(KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val source = expression.source ?: return
        // A safe call's selector carries the safe-call expression as a desugared source.
        if (source.kind !is KtRealSourceElementKind && source.kind != KtFakeSourceElementKind.DesugaredSafeCallExpression) return
        val callee = expression.calleeReference
        val symbol = callee.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return
        if (symbol.callableId != letId || callee.name != letId.callableName) return
        val call = callNode(source) ?: return
        val tree = source.treeStructure
        val parent = tree.getParent(call)
        val safe = parent?.tokenType == KtNodeTypes.SAFE_ACCESS_EXPRESSION

        val lambdaNode = trailingLambda(source, call)
        val lambda = trailingLambdaOf(expression)
        val count = if (lambda == null) 0 else countReferences(lambda)
        if (count > 1) return

        val names = lambdaNode?.let { parameterNames(source, it) } ?: listOf("it")
        val body = lambdaNode?.let { singleStatement(source, it) }
        val canReplace = body != null && rootMatches(source, body, names)

        val message = when {
            safe -> when {
                lambdaNode == null -> null
                count == 0 && !receiverUsed(source, call, parent!!, context) -> SIMPLE_IF
                count == 1 && canReplace -> OMITTED
                else -> null
            }
            count == 0 || canReplace || !inCallChain(source, call, parent) -> OMITTED
            else -> null
        } ?: return
        report(lightSourceOf(call, source), message)
    }

    // The CALL_EXPRESSION node of the let call: the whole source for a bare or
    // safe call, the selector of the qualified source for a dot call.
    private fun callNode(source: KtSourceElement): LighterASTNode? {
        val node = source.lighterASTNode
        return when (node.tokenType) {
            KtNodeTypes.CALL_EXPRESSION -> node
            KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION ->
                significantChildren(source, node, accessTokens).lastOrNull()
                    ?.takeIf { it.tokenType == KtNodeTypes.CALL_EXPRESSION }
            else -> null
        }
    }

    // The FUNCTION_LITERAL of the call's trailing lambda argument, null when the
    // lambda is a parenthesized argument or the argument is not a lambda.
    private fun trailingLambda(source: KtSourceElement, call: LighterASTNode): LighterASTNode? {
        var node = lightChildren(source, call).firstOrNull { it.tokenType == KtNodeTypes.LAMBDA_ARGUMENT } ?: return null
        while (true) {
            if (node.tokenType == KtNodeTypes.LAMBDA_EXPRESSION) {
                return lightChildren(source, node).firstOrNull { it.tokenType == KtNodeTypes.FUNCTION_LITERAL }
            }
            node = significantChildren(source, node).lastOrNull {
                it.tokenType in wrappers
            } ?: return null
        }
    }

    private val wrappers = setOf(
        KtNodeTypes.LAMBDA_ARGUMENT,
        KtNodeTypes.LABELED_EXPRESSION,
        KtNodeTypes.ANNOTATED_EXPRESSION,
        KtNodeTypes.LAMBDA_EXPRESSION,
    )

    private fun trailingLambdaOf(call: FirFunctionCall): FirAnonymousFunctionExpression? {
        var argument = call.argumentList.arguments.lastOrNull() ?: return null
        if (argument is FirFunctionTypeConversionExpression) argument = argument.expression
        val lambda = argument as? FirAnonymousFunctionExpression ?: return null
        return if (lambda.isTrailingLambda) lambda else null
    }

    // The names the lambda parameter goes by: `it`, the declared name, or the
    // destructured names. Detekt compares them with the root of the body.
    private fun parameterNames(source: KtSourceElement, literal: LighterASTNode): List<String> {
        val list = lightChildren(source, literal).firstOrNull { it.tokenType == KtNodeTypes.VALUE_PARAMETER_LIST }
        val param = list?.let { lightChildren(source, it).firstOrNull { c -> c.tokenType == KtNodeTypes.VALUE_PARAMETER } }
            ?: return listOf("it")
        val destructuring = lightChildren(source, param).firstOrNull { it.tokenType == KtNodeTypes.DESTRUCTURING_DECLARATION }
        if (destructuring != null) {
            return lightChildren(source, destructuring)
                .filter { it.tokenType == KtNodeTypes.DESTRUCTURING_DECLARATION_ENTRY }
                .mapNotNull { entry ->
                    lightChildren(source, entry).firstOrNull { it.tokenType == KtTokens.IDENTIFIER }
                        ?.let { lightText(source, it) }
                }
        }
        val identifier = lightChildren(source, param).firstOrNull { it.tokenType == KtTokens.IDENTIFIER }
        return listOf(identifier?.let { lightText(source, it) } ?: "it")
    }

    private fun statements(source: KtSourceElement, literal: LighterASTNode): List<LighterASTNode> {
        val block = lightChildren(source, literal).firstOrNull { it.tokenType == KtNodeTypes.BLOCK } ?: return emptyList()
        return significantChildren(source, block, blockPunctuation)
    }

    private fun singleStatement(source: KtSourceElement, literal: LighterASTNode): LighterASTNode? =
        statements(source, literal).singleOrNull()

    private fun isQualified(node: LighterASTNode) = node.tokenType in qualified

    // The leftmost receiver of a dot or safe-call chain has the parameter's name.
    private fun rootMatches(source: KtSourceElement, body: LighterASTNode, names: List<String>): Boolean {
        if (!isQualified(body)) return false
        var root = body
        while (isQualified(root)) {
            root = significantChildren(source, root, accessTokens).firstOrNull() ?: return false
        }
        return lightText(source, root) in names
    }

    private val accessTokens = setOf(KtTokens.DOT, KtTokens.SAFE_ACCESS)
    private val blockPunctuation = setOf(KtTokens.SEMICOLON, KtTokens.LBRACE, KtTokens.RBRACE)

    // The let call is in a longer chain: its qualified expression is a receiver,
    // or its receiver is itself qualified.
    private fun inCallChain(source: KtSourceElement, call: LighterASTNode, parent: LighterASTNode?): Boolean {
        if (parent == null || !isQualified(parent)) return false
        val tree = source.treeStructure
        if (tree.getParent(parent)?.let { isQualified(it) } == true) return true
        val receiver = significantChildren(source, parent, accessTokens).firstOrNull() ?: return false
        return receiver != call && isQualified(receiver)
    }

    // Detekt's receiverIsUsed: a call that is not a statement is used; a statement
    // is used when it is the value of an enclosing expression.
    private fun receiverUsed(
        source: KtSourceElement,
        call: LighterASTNode,
        qualifiedParent: LighterASTNode,
        context: CheckerContext,
    ): Boolean {
        val tree = source.treeStructure
        val block = tree.getParent(qualifiedParent) ?: return true
        if (block.tokenType != KtNodeTypes.BLOCK) return true
        return statementUsed(source, qualifiedParent, block, context)
    }

    // Whether a statement of [block] is used as a value.
    private fun statementUsed(
        source: KtSourceElement,
        statement: LighterASTNode,
        block: LighterASTNode,
        context: CheckerContext,
    ): Boolean {
        val tree = source.treeStructure
        var stmt = statement
        var current = block
        while (true) {
            if (significantChildren(source, current, blockPunctuation).lastOrNull() != stmt) return false
            val owner = tree.getParent(current) ?: return false
            stmt = when (owner.tokenType) {
                KtNodeTypes.FUNCTION_LITERAL -> return !enclosingLambdaReturnsUnit(context)
                KtNodeTypes.THEN, KtNodeTypes.ELSE -> tree.getParent(owner) ?: return false
                KtNodeTypes.WHEN_ENTRY -> tree.getParent(owner) ?: return false
                KtNodeTypes.TRY -> owner
                KtNodeTypes.CATCH -> tree.getParent(owner) ?: return false
                else -> return false
            }
            if (stmt.tokenType != KtNodeTypes.IF && stmt.tokenType != KtNodeTypes.WHEN && stmt.tokenType != KtNodeTypes.TRY) {
                return false
            }
            val up = tree.getParent(stmt) ?: return true
            if (up.tokenType != KtNodeTypes.BLOCK) return true
            current = up
        }
    }

    // A lambda that returns Unit does not use its last statement as a value.
    private fun enclosingLambdaReturnsUnit(context: CheckerContext): Boolean {
        val symbol: FirBasedSymbol<*> = context.containingDeclarations.lastOrNull() ?: return false
        val lambda = symbol as? FirAnonymousFunctionSymbol ?: return false
        return lambda.resolvedReturnType.isUnit
    }

    // The number of uses of the lambda parameter (or its destructured names) in the body.
    private fun countReferences(lambda: FirAnonymousFunctionExpression): Int {
        val function = lambda.anonymousFunction
        val param = function.valueParameters.singleOrNull() ?: return 0
        val body = function.body ?: return 0
        val targets: Set<FirBasedSymbol<*>> = if (param.name.isSpecial) {
            body.statements.mapNotNull { destructured(it, param.symbol) }.toSet()
        } else {
            setOf(param.symbol)
        }
        var count = 0
        body.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirQualifiedAccessExpression) {
                    val symbol = element.calleeReference.toResolvedCallableSymbol()
                    if (symbol != null && symbol in targets) count++
                }
                element.acceptChildren(this)
            }
        })
        return count
    }

    // The property a destructuring parameter declares: `val a = <destruct>.component1()`.
    private fun destructured(statement: FirElement, param: FirBasedSymbol<*>): FirBasedSymbol<*>? {
        val property = statement as? FirProperty ?: return null
        val component = property.initializer as? FirComponentCall ?: return null
        val receiver = component.explicitReceiver as? FirQualifiedAccessExpression ?: return null
        return property.symbol.takeIf { receiver.calleeReference.toResolvedCallableSymbol() == param }
    }
}
