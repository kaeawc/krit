package dev.jasonpearson.krit.fir.checkers.testingquality

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.qualifiedCall
import org.jetbrains.kotlin.KtNodeTypes

internal object RunBlockingInTestResults {
    fun verification(syntax: RunBlockingInTestSyntax, node: LighterASTNode): Boolean {
        val name = syntax.name(node)
        return RunBlockingInTestExemptions.assertion(name) || harness(syntax, node, name) || ui(syntax, node, name)
    }

    private fun harness(syntax: RunBlockingInTestSyntax, node: LighterASTNode, name: String): Boolean = when (name) {
        "compile" -> ownerSupertypes(syntax).any { it.endsWith("CompilerTest") }
        "compileKotlinAndFail" -> "BaseIncrementalCompilationTest" in ownerSupertypes(syntax)
        "build" -> syntax.imports("com.autonomousapps.kit.GradleBuilder.build")
        "waitFor" -> syntax.imports("androidx.testutils.PollingCheck") || syntax.imports("androidx.testutils.PollingCheck.waitFor")
        "expectError" -> syntax.imports("androidx.compose.testutils.expectError")
        "test", "testWithLifecycle" ->
            (syntax.imports("app.cash.turbine.$name") || "app.cash.turbine.$name" in syntax.fileText) &&
                RunBlockingInTestBridges.hasTurbine(syntax, node)
        "runTest" -> callText(syntax, node).contains("presenter", ignoreCase = true)
        else -> false
    }

    private fun ownerSupertypes(syntax: RunBlockingInTestSyntax): List<String> {
        val owner = syntax.parents(syntax.enclosing).firstOrNull { it.tokenType == KtNodeTypes.CLASS }
            ?: return emptyList()
        val list = lightChildren(syntax.source, owner).firstOrNull { it.tokenType == KtNodeTypes.SUPER_TYPE_LIST }
            ?: return emptyList()
        return lightChildren(syntax.source, list).mapNotNull { entry ->
            val type = syntax.descendants(entry).firstOrNull { it.tokenType == KtNodeTypes.USER_TYPE }
            type?.let { lightChildren(syntax.source, it).lastOrNull { child ->
                child.tokenType == KtNodeTypes.REFERENCE_EXPRESSION
            } }?.let(syntax::text)
        }
    }

    private fun ui(syntax: RunBlockingInTestSyntax, node: LighterASTNode, name: String): Boolean {
        val text = callText(syntax, node)
        val compose = name in setOf("isDisplayed", "doesNotExist") &&
            ("androidx.compose.ui.test" in syntax.fileText || "composeTestRule" in syntax.fileText) &&
            ("onNode" in text || "onAllNodes" in text)
        val espresso = name in setOf("matches", "isDisplayed") &&
            listOf("androidx.test.espresso", "onView(", "withId(", "withText(").any { it in syntax.fileText } &&
            listOf("onView", "withId", "withText").any { it in text }
        return compose || espresso
    }

    private fun callText(syntax: RunBlockingInTestSyntax, node: LighterASTNode): String =
        syntax.text(qualifiedCall(lightSourceOf(node, syntax.source)) ?: node)
}
