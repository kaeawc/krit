package dev.jasonpearson.krit.fir.checkers.testingquality

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.support.identifierAnchor
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.selectorCallExpression
import dev.jasonpearson.krit.fir.support.significantChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.lexer.KtTokens

internal class RunBlockingInTestSyntax private constructor(
    val source: KtSourceElement,
    val call: LighterASTNode,
    val enclosing: LighterASTNode,
) {
    val ancestors = parents(call)
    val file = ancestors.last()
    val body = significantChildren(source, enclosing).last()
    val fileText = text(file)

    fun text(node: LighterASTNode): String = lightText(source, node)

    fun parents(node: LighterASTNode): List<LighterASTNode> =
        generateSequence(source.treeStructure.getParent(node)) { source.treeStructure.getParent(it) }.toList()

    fun descendants(node: LighterASTNode): Sequence<LighterASTNode> = sequence {
        yield(node)
        lightChildren(source, node).forEach { yieldAll(descendants(it)) }
    }

    fun name(node: LighterASTNode): String {
        if (node.tokenType != KtNodeTypes.CALL_EXPRESSION) return ""
        val callee = significantChildren(source, node).firstOrNull() ?: return ""
        return text(callee).substringAfterLast('.').trim('`')
    }

    fun goTestAnnotationSpelling(): Boolean = descendants(enclosing).any { node ->
        node.tokenType == KtNodeTypes.ANNOTATION_ENTRY &&
            text(node).substringAfter('@').substringBefore('(').substringAfterLast('.').trim('`') == "Test"
    }

    fun wholeBody(): Boolean {
        val statement = if (body.tokenType == KtNodeTypes.BLOCK) {
            significantChildren(source, body, setOf(KtTokens.LBRACE, KtTokens.RBRACE)).singleOrNull()
        } else body
        if (statement?.tokenType == KtNodeTypes.RETURN) return descendants(statement).any { it == call }
        val called = statement?.let { selectorCallExpression(lightSourceOf(it, source)) }
        return called == call
    }

    fun insideRunTest(): Boolean = ancestors.any {
        name(it) == "runTest" && coroutineRunTest(it)
    }

    private fun coroutineRunTest(node: LighterASTNode): Boolean {
        val parent = parents(node).firstOrNull()
        if (parent?.tokenType == KtNodeTypes.DOT_QUALIFIED_EXPRESSION) {
            val callee = descendants(parent).takeWhile { it != node }.filter { it.tokenType == KtTokens.IDENTIFIER }
                .map(::text).toList() + name(node)
            return callee.joinToString(".") == "kotlinx.coroutines.test.runTest"
        }
        return imports("kotlinx.coroutines.test.runTest")
    }

    fun imports(fqn: String): Boolean = descendants(file).any {
        it.tokenType == KtNodeTypes.IMPORT_DIRECTIVE && matchesImport(it, fqn)
    }

    private fun matchesImport(node: LighterASTNode, fqn: String): Boolean {
        val imported = descendants(node).takeWhile { it.tokenType != KtTokens.AS_KEYWORD }
            .filter { it.tokenType == KtTokens.IDENTIFIER || it.tokenType == KtTokens.MUL }
            .map(::text).joinToString(".")
        return imported == fqn || imported == fqn.substringBeforeLast('.') + ".*"
    }

    companion object {
        fun create(source: KtSourceElement): RunBlockingInTestSyntax? {
            val call = selectorCallExpression(source) ?: return null
            val enclosing = generateSequence(source.treeStructure.getParent(call)) {
                source.treeStructure.getParent(it)
            }.firstOrNull {
                it.tokenType == KtNodeTypes.FUN && identifierAnchor(lightSourceOf(it, source)) != null
            } ?: return null
            return RunBlockingInTestSyntax(source, call, enclosing)
        }
    }
}
