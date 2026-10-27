package dev.jasonpearson.krit.intellij

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class KritRuleListTest {
    @Test
    fun `parses the list-rules JSON document`() {
        val json = """
            {"rules": [
              {"id": "MagicNumber", "category": "style", "severity": "warning", "defaultActive": true,
               "fixable": false, "precision": "ast-backed", "maturity": "stable", "description": "No magic."},
              {"id": "UnusedImport", "category": "style", "severity": "info", "defaultActive": true,
               "fixable": true, "fixLevel": "cosmetic"},
              {"id": "CustomRule", "category": "custom", "severity": "error", "defaultActive": true,
               "fixable": false, "plugin": true}
            ]}
        """.trimIndent()

        val rules = KritRuleList.parse(json)!!

        assertEquals(listOf("MagicNumber", "UnusedImport", "CustomRule"), rules.map { it.id })
        assertEquals("style", rules[0].category)
        assertEquals("warning", rules[0].severity)
        assertEquals("No magic.", rules[0].description)
        assertTrue(rules[1].fixable)
        assertEquals("cosmetic", rules[1].fixLevel)
        assertTrue(rules[2].plugin)
        assertFalse(rules[0].plugin)
    }

    @Test
    fun `text table from an older krit is not a rule list`() {
        val text = "Available rules:\n  A  MagicNumber    [style] warning\n"
        assertNull(KritRuleList.parse(text))
        assertNull(KritRuleList.parse(""))
        assertNull(KritRuleList.parse("{}"))
        assertNull(KritRuleList.parse("[1, 2]"))
    }

    @Test
    fun `drops entries without an id and repeated ids, and defaults the category`() {
        val json = """{"rules": [
            {"id": "", "category": "style"},
            {"category": "style"},
            {"id": "A", "category": ""},
            {"id": "A", "category": "later"},
            {"id": "B"}
        ]}"""

        val rules = KritRuleList.parse(json)!!

        assertEquals(listOf("A", "B"), rules.map { it.id })
        assertEquals("other", rules[0].category)
        assertEquals("other", rules[1].category)
        assertEquals("", rules[1].severity)
    }

    @Test
    fun `round trips through toJson`() {
        val rules = listOf(KritRuleInfo(id = "A", category = "style", severity = "warning", fixable = true, fixLevel = "idiomatic"))
        assertEquals(rules, KritRuleList.parse(KritRuleList.toJson(rules)))
    }
}
