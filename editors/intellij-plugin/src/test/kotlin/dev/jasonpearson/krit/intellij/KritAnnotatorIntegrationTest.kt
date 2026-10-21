package dev.jasonpearson.krit.intellij

import com.intellij.lang.annotation.HighlightSeverity
import com.intellij.openapi.components.service
import com.intellij.testFramework.fixtures.BasePlatformTestCase

// End-to-end smoke test for the annotator pipeline: a finding seeded into
// KritProjectService must surface as an editor highlight on the right range
// with the right severity, message, and quick fixes. The krit binary is
// never run -- KritProjectService skips its scans in unit-test mode and the
// findings are seeded directly. Runs through the JUnit vintage engine.
class KritAnnotatorIntegrationTest : BasePlatformTestCase() {
    // "fun bad() = " is 12 chars, so `println` starts at 0-based offset 12
    // (krit's 1-based column 13) and spans 7 chars. <caret> is stripped by
    // configureByText and leaves the caret on the finding.
    private val source = "fun bad() = <caret>println(\"x\")\n"
    private val tokenStart = 12
    private val tokenEnd = 19

    private fun finding(severity: String = "warning", fixable: Boolean = false) = KritFinding(
        file = "Foo.kt",
        line = 1,
        column = 13,
        ruleSet = "style",
        rule = "PrintlnInProduction",
        severity = severity,
        message = "println in production code",
        fixable = fixable,
        fixLevel = if (fixable) "idiomatic" else null,
    )

    private fun configureWithSeed(finding: KritFinding) {
        myFixture.configureByText("Foo.kt", source)
        val path = myFixture.file.virtualFile.path
        project.service<KritProjectService>().seedFindingsForTest(path, listOf(finding))
    }

    // The inspection and the external annotator both render findings, so
    // the same message can legitimately appear more than once. Assert on the
    // set of matching highlights rather than an exact count.
    private fun kritHighlights(finding: KritFinding) =
        myFixture.doHighlighting().filter { it.description == finding.displayMessage }

    fun `test annotator renders warning finding from project service`() {
        val finding = finding()
        configureWithSeed(finding)

        val highlights = kritHighlights(finding)

        assertTrue("expected a krit highlight for ${finding.displayMessage}", highlights.isNotEmpty())
        for (info in highlights) {
            assertEquals(HighlightSeverity.WARNING, info.severity)
            assertEquals(tokenStart, info.startOffset)
            assertEquals(tokenEnd, info.endOffset)
            assertEquals("println", info.text)
        }
    }

    fun `test annotator renders error severity`() {
        val finding = finding(severity = "error")
        configureWithSeed(finding)

        val highlights = kritHighlights(finding)

        assertTrue("expected a krit highlight for ${finding.displayMessage}", highlights.isNotEmpty())
        assertTrue(highlights.all { it.severity == HighlightSeverity.ERROR })
    }

    fun `test file without seeded findings has no krit highlights`() {
        myFixture.configureByText("Foo.kt", source)

        val krit = myFixture.doHighlighting().filter { it.description?.startsWith("Krit ") == true }

        assertTrue("unexpected krit highlights: $krit", krit.isEmpty())
    }

    fun `test suppress intention is offered at the finding`() {
        val finding = finding()
        configureWithSeed(finding)
        myFixture.doHighlighting()

        val texts = myFixture.availableIntentions.map { it.text }

        assertTrue("expected suppress intention in $texts", KritSuppress.titleFor(finding.rule) in texts)
    }

    fun `test apply fix intention is offered for fixable finding`() {
        configureWithSeed(finding(fixable = true))
        myFixture.doHighlighting()

        val texts = myFixture.availableIntentions.map { it.text }

        assertTrue(
            "expected apply-fix intention in $texts",
            KritFixLabels.applyFixTitle("idiomatic") in texts,
        )
    }
}
