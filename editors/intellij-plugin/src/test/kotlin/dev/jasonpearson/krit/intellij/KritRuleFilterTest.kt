package dev.jasonpearson.krit.intellij

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

class KritRuleFilterTest {
    private fun finding(rule: String, severity: String = "warning", confidence: Double = 0.0) = KritFinding(
        file = "X.kt",
        line = 1,
        column = 1,
        ruleSet = "style",
        rule = rule,
        severity = severity,
        message = "m",
        confidence = confidence,
    )

    private fun setting(enabled: Boolean = true, level: KritLevel? = KritLevel.WARNING, customized: Boolean = false) =
        KritRuleFilter.RuleSetting(enabled, level, customized)

    @Test
    fun `short names are prefixed rule ids`() {
        assertEquals("Krit_MagicNumber", KritRuleFilter.shortNameFor("MagicNumber"))
    }

    @Test
    fun `default level follows the rule's registry severity`() {
        assertEquals(KritLevel.ERROR, KritRuleFilter.defaultLevel("error"))
        assertEquals(KritLevel.WARNING, KritRuleFilter.defaultLevel("warning"))
        assertEquals(KritLevel.INFORMATION, KritRuleFilter.defaultLevel("INFO"))
        assertEquals(KritLevel.WARNING, KritRuleFilter.defaultLevel(""))
    }

    @Test
    fun `a rule without a per-rule inspection shows krit's severity`() {
        assertEquals(KritLevel.ERROR, KritRuleFilter.levelFor(finding("A", "error"), null))
        assertEquals(KritLevel.WEAK_WARNING, KritRuleFilter.levelFor(finding("A", confidence = 0.3), null))
    }

    @Test
    fun `a disabled rule inspection hides the finding`() {
        assertNull(KritRuleFilter.levelFor(finding("A"), setting(enabled = false)))
        assertNull(KritRuleFilter.levelFor(finding("A"), setting(enabled = false, level = KritLevel.ERROR, customized = true)))
    }

    @Test
    fun `an untouched level keeps krit's reported severity`() {
        // krit.yml raised this rule to error; the profile still has the
        // registry default (warning), so krit's severity shows.
        assertEquals(KritLevel.ERROR, KritRuleFilter.levelFor(finding("A", "error"), setting()))
        assertEquals(KritLevel.INFORMATION, KritRuleFilter.levelFor(finding("A", "info"), setting()))
    }

    @Test
    fun `a level changed in the profile overrides krit's severity`() {
        val custom = setting(level = KritLevel.ERROR, customized = true)
        assertEquals(KritLevel.ERROR, KritRuleFilter.levelFor(finding("A", "warning"), custom))
        assertEquals(KritLevel.ERROR, KritRuleFilter.levelFor(finding("A", "info", confidence = 0.2), custom))
        val lowered = setting(level = KritLevel.WEAK_WARNING, customized = true)
        assertEquals(KritLevel.WEAK_WARNING, KritRuleFilter.levelFor(finding("A", "error"), lowered))
    }

    @Test
    fun `apply filters per rule and looks each rule up once`() {
        val lookups = mutableListOf<String>()
        val shown = KritRuleFilter.apply(
            listOf(finding("Off"), finding("Raised"), finding("Off"), finding("Unknown", "info"), finding("Raised")),
        ) { rule ->
            lookups += rule
            when (rule) {
                "Off" -> setting(enabled = false)
                "Raised" -> setting(level = KritLevel.ERROR, customized = true)
                else -> null
            }
        }

        assertEquals(listOf("Raised", "Unknown", "Raised"), shown.map { it.finding.rule })
        assertEquals(listOf(KritLevel.ERROR, KritLevel.INFORMATION, KritLevel.ERROR), shown.map { it.level })
        assertEquals(listOf("Off", "Raised", "Unknown"), lookups)
    }
}
