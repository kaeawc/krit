package dev.jasonpearson.krit.intellij

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class KritDocsLinksTest {
    private val finding = KritFinding(
        file = "/p/A.kt",
        line = 3,
        column = 1,
        ruleSet = "style",
        rule = "MagicNumber",
        severity = "warning",
        message = "Use a <named> constant & not 42",
    )

    @Test
    fun `reads docsUrl from the list-rules JSON`() {
        val rules = KritRuleList.parse(
            """{"rules": [
                {"id": "MagicNumber", "category": "style", "severity": "warning",
                 "docsUrl": "https://kaeawc.github.io/krit/rules/style/magic-number/"},
                {"id": "Older", "category": "style", "severity": "warning"}
            ]}""",
        )!!

        assertEquals("https://kaeawc.github.io/krit/rules/style/magic-number/", rules[0].docsUrl)
        assertNull(rules[1].docsUrl)
    }

    @Test
    fun `links only rules with an http docs URL`() {
        val links = KritDocsLinks.byRule(
            listOf(
                KritRuleInfo(id = "A", docsUrl = "https://docs.test/a"),
                KritRuleInfo(id = "B", docsUrl = "http://docs.test/b"),
                KritRuleInfo(id = "C", docsUrl = "javascript:alert(1)"),
                KritRuleInfo(id = "D", docsUrl = " "),
                KritRuleInfo(id = "E"),
            ),
        )

        assertEquals(mapOf("A" to "https://docs.test/a", "B" to "http://docs.test/b"), links)
    }

    @Test
    fun `tooltip shows the escaped message and a docs link`() {
        val tooltip = KritDocsLinks.tooltip(finding, "https://docs.test/rules?id=a&b=\"c\"")!!

        assertTrue(tooltip.startsWith("<html>Krit style/MagicNumber: Use a &lt;named&gt; constant &amp; not 42<br>"))
        assertTrue(tooltip.contains("<a href=\"https://docs.test/rules?id=a&amp;b=&quot;c&quot;\">MagicNumber documentation</a>"))
    }

    @Test
    fun `no docs link keeps the default tooltip`() {
        assertNull(KritDocsLinks.tooltip(finding, null))
    }

    @Test
    fun `the inspection description links the docs`() {
        val withDocs = KritRuleInspection(KritRuleInfo(id = "A", category = "style", severity = "warning", docsUrl = "https://docs.test/a"))
        val without = KritRuleInspection(KritRuleInfo(id = "B", category = "style", severity = "warning"))

        assertTrue(withDocs.staticDescription.contains("<a href=\"https://docs.test/a\">Rule documentation</a>"))
        assertTrue(!without.staticDescription.contains("<a "))
    }
}
