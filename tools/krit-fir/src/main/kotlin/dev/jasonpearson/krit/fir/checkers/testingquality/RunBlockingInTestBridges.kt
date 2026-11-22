package dev.jasonpearson.krit.fir.checkers.testingquality

import dev.jasonpearson.krit.fir.support.GoTestAssertionNames
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.modifierKeywordAnchor
import dev.jasonpearson.krit.fir.support.qualifiedCall
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.lexer.KtTokens

internal object RunBlockingInTestBridges {
    fun exempt(syntax: RunBlockingInTestSyntax): Boolean =
        rxBridge(syntax) || rxScheduler(syntax) || assertedResult(syntax) || turbine(syntax)

    fun isSuspend(syntax: RunBlockingInTestSyntax): Boolean =
        modifierKeywordAnchor(lightSourceOf(syntax.enclosing, syntax.source), KtTokens.SUSPEND_KEYWORD) != null

    private fun rxBridge(syntax: RunBlockingInTestSyntax): Boolean {
        val imported = listOf("io.reactivex", "io.reactivex.rxjava3").any { pkg ->
            syntax.imports("$pkg.observers.TestObserver") || syntax.imports("$pkg.subscribers.TestSubscriber")
        }
        val text = syntax.text(syntax.body)
        return imported && syntax.body.tokenType == KtNodeTypes.BLOCK &&
            ("TestObserver" in text || "TestSubscriber" in text) && ("subscribe(" in text || ".subscribe" in text)
    }

    private fun rxScheduler(syntax: RunBlockingInTestSyntax): Boolean {
        val imported = listOf("io.reactivex.schedulers.TestScheduler", "io.reactivex.rxjava3.schedulers.TestScheduler")
            .any(syntax::imports)
        return imported && !isSuspend(syntax) && syntax.descendants(syntax.enclosing).any {
            it.tokenType == KtTokens.IDENTIFIER && syntax.text(it) == "TestScheduler"
        }
    }

    private fun assertedResult(syntax: RunBlockingInTestSyntax): Boolean {
        val property = syntax.ancestors.takeWhile { it != syntax.enclosing }
            .firstOrNull { it.tokenType == KtNodeTypes.PROPERTY } ?: return false
        val assigned = lightChildren(syntax.source, property).firstOrNull { it.tokenType == KtTokens.IDENTIFIER }
            ?.let(syntax::text) ?: return false
        return syntax.descendants(syntax.body).any { node ->
            RunBlockingInTestResults.verification(syntax, node) && references(syntax, node, assigned)
        }
    }

    private fun references(
        syntax: RunBlockingInTestSyntax,
        node: com.intellij.lang.LighterASTNode,
        name: String,
    ): Boolean {
        val root = qualifiedCall(lightSourceOf(node, syntax.source)) ?: node
        return syntax.descendants(root).any { it.tokenType == KtTokens.IDENTIFIER && syntax.text(it) == name }
    }

    private fun turbine(syntax: RunBlockingInTestSyntax): Boolean = syntax.descendants(syntax.call).any {
        it != syntax.call && syntax.name(it) in GoTestAssertionNames.turbineAssertionCalls
    }

    fun hasTurbine(syntax: RunBlockingInTestSyntax, node: com.intellij.lang.LighterASTNode): Boolean =
        syntax.descendants(node).any { syntax.name(it) in GoTestAssertionNames.turbineAssertionCalls }
}
