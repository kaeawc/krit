package dev.jasonpearson.krit.fir.support

import com.intellij.lang.LighterASTNode
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirThrowExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.types.isMarkedNullable
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.StandardClassIds

// Shared shape test for the rules that replace `throw X(...)` with `check()`, `error()` or `require()`.

/**
 * Whether [throwExpression] throws a freshly constructed exception of [exceptionClassIds] that carries no cause: a
 * constructor with no argument or with one non-null `String` message. The constructor is the thrown call itself or the
 * innermost receiver of a chain on it (`X("m").also { ... }`). The class is read from the resolved constructor, so an
 * import alias, a type alias or a qualified name counts and a look-alike class does not.
 */
context(context: CheckerContext)
fun throwsPlainException(throwExpression: FirThrowExpression, exceptionClassIds: Set<ClassId>): Boolean {
    val source = throwExpression.source ?: return false
    if (source.kind !is KtRealSourceElementKind) return false
    val constructor = constructorCallOf(throwExpression.exception) ?: return false
    val classId = constructor.resolvedType.expandedClassType()?.lookupTag?.classId ?: return false
    if (classId !in exceptionClassIds) return false
    val arguments = constructor.argumentList.arguments
    if (arguments.isEmpty()) return true
    val message = arguments.singleOrNull() ?: return false
    return isNonNullString(unwrapArgument(message))
}

context(context: CheckerContext)
private fun isNonNullString(argument: FirExpression): Boolean {
    val type = argument.resolvedType.expandedBound()
    return !type.isMarkedNullable && type.expandedClassType()?.lookupTag?.classId == StandardClassIds.String
}

// The constructor call a thrown expression starts with, looking through the receivers of a call chain.
private fun constructorCallOf(thrown: FirExpression): FirFunctionCall? {
    var current: FirExpression? = unwrapSmartCasts(thrown)
    while (current != null) {
        current = when (current) {
            is FirFunctionCall ->
                if (current.calleeReference.toResolvedCallableSymbol() is FirConstructorSymbol) return current
                else unwrapSmartCasts(current.explicitReceiver)
            is FirPropertyAccessExpression -> unwrapSmartCasts(current.explicitReceiver)
            else -> null
        }
    }
    return null
}

/**
 * Whether the throw is the whole body of an `if` branch or of a `for`, `while` or `do` loop, with or without braces.
 * Other statements in the braces, or a `when` entry, do not count.
 */
fun isDirectControlBody(source: KtSourceElement): Boolean {
    val parent = source.treeStructure.getParent(source.lighterASTNode) ?: return false
    val body = if (parent.tokenType == KtNodeTypes.BLOCK) {
        if (!isOnlyStatementOf(source, parent, source.lighterASTNode)) return false
        source.treeStructure.getParent(parent) ?: return false
    } else {
        parent
    }
    return body.tokenType in controlBodyTypes
}

/** Whether the throw is the only statement of a lambda (`{ throw X() }`), which `check()` cannot replace. */
fun isOnlyStatementInLambda(source: KtSourceElement): Boolean {
    val block = source.treeStructure.getParent(source.lighterASTNode) ?: return false
    if (block.tokenType != KtNodeTypes.BLOCK) return false
    val literal = source.treeStructure.getParent(block) ?: return false
    return literal.tokenType == KtNodeTypes.FUNCTION_LITERAL && isOnlyStatementOf(source, block, source.lighterASTNode)
}

private fun isOnlyStatementOf(source: KtSourceElement, block: LighterASTNode, statement: LighterASTNode): Boolean =
    significantChildren(source, block, blockPunctuation).singleOrNull() == statement

private val controlBodyTypes = setOf(KtNodeTypes.THEN, KtNodeTypes.ELSE, KtNodeTypes.BODY)
// A `;` separates statements without being one, as in detekt's KtBlockExpression.statements.
private val blockPunctuation = setOf(KtTokens.LBRACE, KtTokens.RBRACE, KtTokens.SEMICOLON)
