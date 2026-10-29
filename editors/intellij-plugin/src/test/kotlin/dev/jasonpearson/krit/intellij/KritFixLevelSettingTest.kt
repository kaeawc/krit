package dev.jasonpearson.krit.intellij

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class KritFixLevelSettingTest {
    @Test
    fun `fixes at or below the maximum are allowed`() {
        assertTrue(KritFixLabels.isWithin("cosmetic", "idiomatic"))
        assertTrue(KritFixLabels.isWithin("idiomatic", "idiomatic"))
        assertFalse(KritFixLabels.isWithin("semantic", "idiomatic"))
        assertTrue(KritFixLabels.isWithin("semantic", "semantic"))
        assertFalse(KritFixLabels.isWithin("idiomatic", "cosmetic"))
    }

    @Test
    fun `an unstated fix level is allowed and an unknown maximum means idiomatic`() {
        assertTrue(KritFixLabels.isWithin(null, "cosmetic"))
        assertTrue(KritFixLabels.isWithin("", "cosmetic"))
        assertFalse(KritFixLabels.isWithin("semantic", "bogus"))
        assertTrue(KritFixLabels.isWithin("IDIOMATIC", ""))
    }

    @Test
    fun `the fix runs at the finding's level, or the maximum when it has none`() {
        assertEquals("cosmetic", KritFixLabels.levelToApply("cosmetic", "semantic"))
        assertEquals("semantic", KritFixLabels.levelToApply(null, "semantic"))
        assertEquals("idiomatic", KritFixLabels.levelToApply(null, "bogus"))
    }

    private fun fixable(level: String?) = KritFinding(
        file = "X.kt",
        line = 1,
        column = 1,
        ruleSet = "style",
        rule = "R",
        severity = "warning",
        message = "m",
        fixable = true,
        fixLevel = level,
    )

    @Test
    fun `intentions offer the autofix only within the maximum`() {
        val semanticUnderIdiomatic = KritIntentions.forFinding(fixable("semantic"), maxFixLevel = "idiomatic").map { it.text }
        assertFalse(semanticUnderIdiomatic.any { it.startsWith("Apply Krit") }, "$semanticUnderIdiomatic")

        val semanticUnderSemantic = KritIntentions.forFinding(fixable("semantic"), maxFixLevel = "semantic").map { it.text }
        assertTrue(KritFixLabels.applyFixTitle("semantic") in semanticUnderSemantic, "$semanticUnderSemantic")

        val unstated = KritIntentions.forFinding(fixable(null), maxFixLevel = "cosmetic").map { it.text }
        assertTrue(KritFixLabels.applyFixTitle("cosmetic") in unstated, "$unstated")
    }

    @Test
    fun `quick fixes follow the same rule`() {
        val names = KritQuickFixes.forFinding(fixable("semantic"), maxFixLevel = "idiomatic").map { it.familyName }
        assertFalse(names.any { it.startsWith("Apply Krit") }, "$names")
        val allowed = KritQuickFixes.forFinding(fixable("cosmetic"), maxFixLevel = "idiomatic").map { it.familyName }
        assertTrue(KritFixLabels.applyFixTitle("cosmetic") in allowed, "$allowed")
    }
}
