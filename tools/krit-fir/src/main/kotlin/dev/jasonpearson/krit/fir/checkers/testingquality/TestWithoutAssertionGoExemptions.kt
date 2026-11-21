package dev.jasonpearson.krit.fir.checkers.testingquality

import dev.jasonpearson.krit.fir.containingScanPath
import dev.jasonpearson.krit.fir.support.GoTestAssertionNames
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid

internal object TestWithoutAssertionGoExemptions {
    private val nonAssertions = setOf("throw", "error", "TODO", "NotImplementedError")

    fun goHelperNames(file: FirFile, patterns: List<String>, uiEvidence: Boolean): Set<String> {
        val candidates = ArrayList<FirNamedFunction>()
        file.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirNamedFunction && !TestWithoutAssertionGoSupport.isTest(element)) candidates += element
                element.acceptChildren(this)
            }
        })
        val helpers = HashSet<String>()
        var previousHelpers: Set<String>
        do {
            previousHelpers = helpers.toSet()
            for (candidate in candidates) {
                val bodySource = candidate.body?.source
                val bodyText = TestWithoutAssertionGoSupport.sourceText(bodySource)
                val textExemption = GoTestAssertionNames.textExempts(bodyText)
                val identifierExemption = GoTestAssertionNames.identifiers(bodySource).any { name ->
                    isExemptHelperIdentifier(name, helpers, patterns, uiEvidence)
                }
                if (textExemption || identifierExemption) helpers += candidate.name.asString()
            }
        } while (previousHelpers != helpers)
        return helpers
    }

    fun goExempts(
        function: FirNamedFunction,
        owners: List<FirRegularClass>,
        helpers: Set<String>,
        patterns: List<String>,
        uiEvidence: Boolean,
    ): Boolean {
        if (!hasSimpleTestAnnotation(function) || hasExpectedException(function)) return true
        val body = function.body ?: return true
        val names = GoTestAssertionNames.identifiers(body.source)
        val functionText = TestWithoutAssertionGoSupport.sourceText(function.source)
        val text = functionText.lowercase().replace('’', '\'')
        val squashed = text.filterNot { it.isWhitespace() || it == '_' || it == '-' }
        return hasAssertionEvidence(names, helpers, patterns, uiEvidence) ||
            GoTestAssertionNames.textExempts(functionText) ||
            hasSmokeEvidence(squashed, text) ||
            hasComposeSmokeEvidence(names, text) ||
            isExceptionFactory(body, owners)
    }

    context(context: CheckerContext)
    fun exemptFile(file: FirFile): Boolean {
        val path = with(TestWithoutAssertion) { containingScanPath() }.orEmpty().replace('\\', '/')
        if (".idea" in path.split('/')) return true
        if (file.imports.any(TestWithoutAssertionGoSupport::isBenchmarkImport)) return true
        val lowerPath = path.lowercase()
        if (!TestWithoutAssertionGoSupport.isBenchmarkPath(lowerPath)) return false
        val text = TestWithoutAssertionGoSupport.sourceText(file.source)
        return "@LargeTest" in text || "AndroidJUnit4" in text
    }

    private fun isExemptHelperIdentifier(
        name: String,
        helpers: Set<String>,
        patterns: List<String>,
        uiEvidence: Boolean,
    ): Boolean = name in helpers || name !in nonAssertions &&
        (GoTestAssertionNames.exempts(name, patterns) ||
            TestWithoutAssertionGoSupport.goUIVisibilityExempts(name, uiEvidence))

    private fun hasSimpleTestAnnotation(function: FirNamedFunction): Boolean =
        function.annotations.any { annotation ->
            TestWithoutAssertionGoSupport.sourceText(annotation.source).removePrefix("@").substringBefore('(')
                .trim().substringAfterLast('.') == "Test"
        }

    private fun hasExpectedException(function: FirNamedFunction): Boolean =
        function.annotations.any { "expected" in TestWithoutAssertionGoSupport.sourceText(it.source) }

    private fun hasAssertionEvidence(
        names: Set<String>,
        helpers: Set<String>,
        patterns: List<String>,
        uiEvidence: Boolean,
    ): Boolean = names.any { name ->
        name in helpers || GoTestAssertionNames.exempts(name, patterns) ||
            TestWithoutAssertionGoSupport.goUIVisibilityExempts(name, uiEvidence)
    }

    private fun hasSmokeEvidence(squashed: String, text: String): Boolean =
        "smoke" in squashed || "noop" in squashed ||
            GoTestAssertionNames.noExceptionMarkers.any { it.replace(" ", "") in squashed } ||
            GoTestAssertionNames.smokePhrases.any { it in text }

    private fun hasComposeSmokeEvidence(names: Set<String>, text: String): Boolean =
        "setContent" in names && "waitForIdle" in names &&
            GoTestAssertionNames.composeSmokeCommentWords.any { it in text }

}

private fun isExceptionFactoryName(name: String): Boolean = name.isNotEmpty() &&
    (name[0] in 'A'..'Z' || name.startsWith("create") || name.startsWith("from") || name.startsWith("parse") ||
        name in setOf("of", "valueOf"))

private fun isExceptionFactory(
    body: FirBlock,
    owners: List<FirRegularClass>,
): Boolean {
    val onlyCall = body.statements.singleOrNull() as? FirFunctionCall ?: return false
    val callName = onlyCall.calleeReference.name.asString()
    if (!isExceptionFactoryName(callName)) return false
    val classText = TestWithoutAssertionGoSupport.sourceText(owners.lastOrNull()?.source)
    return listOf("assertThrows", "assertFailsWith", "assertFails(").any { it in classText }
}
