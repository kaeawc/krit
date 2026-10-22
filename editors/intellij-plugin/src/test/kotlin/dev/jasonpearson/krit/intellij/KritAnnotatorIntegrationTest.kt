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
    // XML is part of the core platform, so the annotator registered for
    // language="XML" runs in the light fixture without depending on the
    // bundled Kotlin plugin loading in the test sandbox. `LinearLayout`
    // starts at 0-based offset 1 (krit's 1-based column 2) and spans 12
    // chars. <caret> is stripped by configureByText.
    private val fileName = "main.xml"
    private val source = "<<caret>LinearLayout/>\n"
    private val tokenStart = 1
    private val tokenEnd = 13

    override fun setUp() {
        super.setUp()
        project.service<KritProjectService>().clearFindingsForTest()
    }

    private fun finding(severity: String = "warning", fixable: Boolean = false) = KritFinding(
        file = fileName,
        line = 1,
        column = 2,
        ruleSet = "android",
        rule = "UselessParent",
        severity = severity,
        message = "This LinearLayout is useless",
        fixable = fixable,
        fixLevel = if (fixable) "idiomatic" else null,
    )

    private fun configureWithSeed(finding: KritFinding) {
        myFixture.configureByText(fileName, source)
        val path = myFixture.file.virtualFile.path
        project.service<KritProjectService>().seedFindingsForTest(path, listOf(finding))
    }

    // The inspection and the external annotator both render findings, so
    // the same message can legitimately appear more than once. Assert on the
    // set of matching highlights rather than an exact count.
    private fun kritHighlights(finding: KritFinding): List<com.intellij.codeInsight.daemon.impl.HighlightInfo> {
        val all = myFixture.doHighlighting()
        lastDiagnostics = "language=${myFixture.file.language.id} " +
            "path=${myFixture.file.virtualFile.path} " +
            "seeded=${project.service<KritProjectService>().findingsFor(myFixture.file.virtualFile.path).size} " +
            "highlights=${all.map { "${it.severity}:${it.description}" }}"
        return all.filter { it.description == finding.displayMessage }
    }

    private var lastDiagnostics = ""

    fun `test annotator renders warning finding from project service`() {
        val finding = finding()
        configureWithSeed(finding)

        val highlights = kritHighlights(finding)

        assertTrue("expected a krit highlight for ${finding.displayMessage}; $lastDiagnostics", highlights.isNotEmpty())
        for (info in highlights) {
            assertEquals(HighlightSeverity.WARNING, info.severity)
            assertEquals(tokenStart, info.startOffset)
            assertEquals(tokenEnd, info.endOffset)
            assertEquals("LinearLayout", info.text)
        }
    }

    fun `test annotator renders error severity`() {
        val finding = finding(severity = "error")
        configureWithSeed(finding)

        val highlights = kritHighlights(finding)

        assertTrue("expected a krit highlight for ${finding.displayMessage}; $lastDiagnostics", highlights.isNotEmpty())
        assertTrue(highlights.all { it.severity == HighlightSeverity.ERROR })
    }

    fun `test file without seeded findings has no krit highlights`() {
        myFixture.configureByText(fileName, source)

        val krit = myFixture.doHighlighting().filter { it.description?.startsWith("Krit ") == true }

        assertTrue("unexpected krit highlights: $krit", krit.isEmpty())
    }

    // Suppression writes @Suppress / @SuppressWarnings on a Kotlin or Java
    // declaration; an XML file has no such owner, so the intention must not
    // be offered there even though the finding itself is highlighted.
    fun `test suppress intention is not offered without a declaration owner`() {
        val finding = finding()
        configureWithSeed(finding)
        myFixture.doHighlighting()

        val texts = myFixture.availableIntentions.map { it.text }

        assertFalse("unexpected suppress intention in $texts", KritSuppress.titleFor(finding.rule) in texts)
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
