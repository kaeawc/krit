package dev.jasonpearson.krit.fir.checkers.testingquality

import dev.jasonpearson.krit.fir.support.GoTestAssertionNames
import dev.jasonpearson.krit.fir.support.lightSourceOf
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.lexer.KtTokens

internal object RunBlockingInTestIntent {
    private val intentions = listOf("intentional", "deliberate")
    private val reasons = listOf("dispatcher", "thread", "confinement", "reentrant", "locking")

    fun exempt(syntax: RunBlockingInTestSyntax): Boolean = threadIntent(syntax) || documentedIntent(syntax)

    private fun threadIntent(syntax: RunBlockingInTestSyntax): Boolean {
        val text = syntax.text(syntax.call)
        val dispatcher = "Dispatchers.Default" in text || "Dispatchers.IO" in text
        val primitive = "CountDownLatch" in text || "Semaphore" in text
        val scope = "CoroutineScope(" in text
        return "isDispatchNeeded" in text || primitive && (dispatcher || scope) || dispatcher && scope ||
            syntax.descendants(syntax.call).any { threadCall(syntax, it) }
    }

    private fun threadCall(syntax: RunBlockingInTestSyntax, node: com.intellij.lang.LighterASTNode): Boolean {
        val name = syntax.name(node)
        val parent = syntax.parents(node).firstOrNull()?.takeIf {
            it.tokenType == KtNodeTypes.DOT_QUALIFIED_EXPRESSION || it.tokenType == KtNodeTypes.SAFE_ACCESS_EXPRESSION
        }
        val names = parent?.let { GoTestAssertionNames.identifiers(lightSourceOf(it, syntax.source)) }.orEmpty()
        return name == "currentThread" && "Thread" in names ||
            name in setOf("isSameInstanceAs", "isNotSameInstanceAs") &&
            "thread" in syntax.text(parent ?: node).lowercase()
    }

    private fun documentedIntent(syntax: RunBlockingInTestSyntax): Boolean =
        syntax.descendants(syntax.enclosing).any {
            it.tokenType in KtTokens.COMMENTS && beforeCallLine(syntax, it.startOffset) &&
                documentsIntent(syntax.text(it))
        }

    private fun beforeCallLine(syntax: RunBlockingInTestSyntax, offset: Int): Boolean {
        val start = syntax.call.startOffset
        if (offset <= start) return true
        val between = syntax.text(syntax.file).substring(start, offset)
        return '\n' !in between
    }

    private fun documentsIntent(text: String): Boolean {
        val lower = text.lowercase()
        return "runblocking" in lower && intentions.any { it in lower } && reasons.any { it in lower }
    }
}
