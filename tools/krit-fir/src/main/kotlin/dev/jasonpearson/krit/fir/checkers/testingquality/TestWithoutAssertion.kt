package dev.jasonpearson.krit.fir.checkers.testingquality

import dev.jasonpearson.krit.fir.FirRule
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirFileChecker
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid

internal object TestWithoutAssertion : FirFileChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "TestWithoutAssertion"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val fileCheckers = setOf(TestWithoutAssertion)
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirFile) {
        if (config()["allowNoAssertionTests"] == true ||
            TestWithoutAssertionGoExemptions.exemptFile(declaration)
        ) return
        val patterns = configuredPatterns()
        val fileText = TestWithoutAssertionGoSupport.sourceText(declaration.source)
        val uiEvidence = hasUiEvidence(fileText)
        val helpers = TestWithoutAssertionGoExemptions.goHelperNames(declaration, patterns, uiEvidence)
        val owners = ArrayList<FirRegularClass>()
        declaration.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirRegularClass) owners += element
                if (element is FirNamedFunction && TestWithoutAssertionGoSupport.isTest(element) &&
                    !TestWithoutAssertionGoExemptions.goExempts(element, owners, helpers, patterns, uiEvidence)
                ) {
                    TestWithoutAssertionTestBody.checkTest(element, owners, declaration, patterns)
                }
                element.acceptChildren(this)
                if (element is FirRegularClass) owners.removeAt(owners.lastIndex)
            }
        })
    }

    private fun configuredPatterns(): List<String> =
        (config()["assertionMethodPatterns"] as? List<*>)?.filterIsInstance<String>().orEmpty()

}

private fun hasUiEvidence(fileText: String): Boolean = listOf(
    "androidx.compose.ui.test", "composeTestRule", "androidx.test.espresso", "onView(", "withId(", "withText(",
).any { it in fileText }
