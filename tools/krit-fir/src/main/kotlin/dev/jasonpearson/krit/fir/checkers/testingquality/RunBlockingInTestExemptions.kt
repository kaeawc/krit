package dev.jasonpearson.krit.fir.checkers.testingquality

import dev.jasonpearson.krit.fir.support.GoTestAssertionNames
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.selectorCallExpression
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.utils.isCompanion
import org.jetbrains.kotlin.text

internal object RunBlockingInTestExemptions {
    private val androidPaths = listOf(
        "/src/androidtest/", "/src/androidinstrumentedtest/", "/src/androidcommontest/", "/src/commonjvmandroidtest/",
    )
    private val coroutineMarkers = listOf(
        "kotlinx.coroutines.test", "runTest", "TestScope", "TestDispatcher", "TestCoroutineScheduler",
        "runCurrent", "advanceUntilIdle", "MainDispatcherRule",
    )
    private val schedulerMarkers = listOf("TestCoroutineScheduler", "testScheduler", "MainCoroutineRule", "TestDispatcher")
    private val schedulerSteps = listOf("runCurrent(", "advanceUntilIdle(", "advanceTimeBy(")
    private val callbacks = setOf("runOnIdle", "runOnUiThread", "runOnActivity")

    fun exempt(
        syntax: RunBlockingInTestSyntax,
        path: String,
        markers: List<String>,
        containingClasses: List<FirRegularClass>,
    ): Boolean =
        androidPaths.any { it in path.lowercase() } || boundary(syntax) ||
            realDispatcherOwner(markers, containingClasses) ||
            RunBlockingInTestIntent.exempt(syntax) || RunBlockingInTestBridges.exempt(syntax) || bridgeScope(syntax)

    private fun boundary(syntax: RunBlockingInTestSyntax): Boolean {
        val ancestors = syntax.ancestors.takeWhile { it != syntax.enclosing }
        return ancestors.any {
            val name = syntax.name(it)
            name in callbacks || assertion(name)
        } || ancestors.any { node ->
            if (node.tokenType != KtNodeTypes.DOT_QUALIFIED_EXPRESSION &&
                node.tokenType != KtNodeTypes.SAFE_ACCESS_EXPRESSION
            ) return@any false
            val selector = selectorCallExpression(lightSourceOf(node, syntax.source)) ?: return@any false
            selector != syntax.call && assertion(syntax.name(selector))
        }
    }

    fun assertion(name: String): Boolean =
        name in GoTestAssertionNames.assertionCallNames || name in GoTestAssertionNames.verifyCallNames ||
            name.startsWith("assert") || name.startsWith("verify") || name.startsWith("validate") ||
            RunBlockingInTestNames.assertionToken(name)

    private fun realDispatcherOwner(markers: List<String>, containingClasses: List<FirRegularClass>): Boolean {
        val owner = containingClasses.lastOrNull { declaration ->
            declaration.classKind != ClassKind.OBJECT ||
                !declaration.isCompanion && !declaration.name.isSpecial
        } ?: return false
        val text = owner.source?.text?.toString() ?: return false
        return "Dispatchers.Default" in text || "Dispatchers.IO" in text ||
            markers.any { it.isNotEmpty() && it in text }
    }

    private fun bridgeScope(syntax: RunBlockingInTestSyntax): Boolean {
        if (syntax.insideRunTest() || syntax.wholeBody()) return false
        val noCoroutineTests = coroutineMarkers.none { it in syntax.fileText }
        val manualScheduler = schedulerMarkers.any { it in syntax.fileText } &&
            schedulerSteps.any { it in syntax.fileText } && !RunBlockingInTestBridges.isSuspend(syntax)
        return noCoroutineTests || manualScheduler
    }
}
