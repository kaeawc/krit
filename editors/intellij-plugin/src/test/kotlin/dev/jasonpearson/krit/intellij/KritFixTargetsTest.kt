package dev.jasonpearson.krit.intellij

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

class KritFixTargetsTest {
    private fun finding(file: String, suggestions: List<KritSuggestedFix> = emptyList()) = KritFinding(
        file = file,
        line = 3,
        column = 5,
        ruleSet = "style",
        rule = "MagicNumber",
        severity = "warning",
        message = "m",
        suggestedFixes = suggestions,
    )

    @Test
    fun `fix targets the finding's own file`() {
        val f = finding("/repo/src/A.kt")
        assertEquals(setOf("/repo/src/A.kt"), KritFixTargets.forFix(listOf(f), f.findingId, "/repo"))
    }

    @Test
    fun `relative finding paths resolve against the project`() {
        val f = finding("src/A.kt")
        assertEquals(setOf("/repo/src/A.kt"), KritFixTargets.forFix(listOf(f), f.findingId, "/repo"))
    }

    @Test
    fun `unknown finding falls back to a project scan`() {
        assertNull(KritFixTargets.forFix(listOf(finding("/repo/A.kt")), "Other:/repo/A.kt:1:1", "/repo"))
        assertNull(KritFixTargets.forFix(listOf(finding("A.kt")), "MagicNumber:A.kt:3:5", null))
    }

    @Test
    fun `suggestion targets every edited file`() {
        val suggestion = KritSuggestedFix(
            id = "s1",
            edits = listOf(
                KritSuggestedEdit(targetFile = "/repo/src/A.kt"),
                KritSuggestedEdit(targetFile = "src/B.kt"),
                KritSuggestedEdit(targetFile = "/repo/src/A.kt"),
            ),
        )
        val f = finding("/repo/src/A.kt", listOf(suggestion))
        assertEquals(
            setOf("/repo/src/A.kt", "/repo/src/B.kt"),
            KritFixTargets.forSuggestion(listOf(f), f.findingId, "s1", "/repo"),
        )
    }

    @Test
    fun `suggestion without known targets falls back to a project scan`() {
        val blankTarget = KritSuggestedFix(id = "s1", edits = listOf(KritSuggestedEdit(targetFile = "")))
        val noEdits = KritSuggestedFix(id = "s2")
        val f = finding("/repo/A.kt", listOf(blankTarget, noEdits))
        assertNull(KritFixTargets.forSuggestion(listOf(f), f.findingId, "s1", "/repo"))
        assertNull(KritFixTargets.forSuggestion(listOf(f), f.findingId, "s2", "/repo"))
        assertNull(KritFixTargets.forSuggestion(listOf(f), f.findingId, "missing", "/repo"))
        assertNull(KritFixTargets.forSuggestion(emptyList(), f.findingId, "s1", "/repo"))
    }

    @Test
    fun `suggestion paths are normalized`() {
        val suggestion = KritSuggestedFix(id = "s1", edits = listOf(KritSuggestedEdit(targetFile = "src/../B.kt")))
        val f = finding("/repo/A.kt", listOf(suggestion))
        assertEquals(setOf("/repo/B.kt"), KritFixTargets.forSuggestion(listOf(f), f.findingId, "s1", "/repo"))
    }

    @Test
    fun `per-file rescan stands in only for rules it re-evaluates`() {
        val perFile = setOf("MagicNumber", "UnnecessaryApply")
        assertEquals(true, KritFixTargets.perFileRescanCovers("MagicNumber", perFile))
        // DeadCode is project-only: its fixed finding would survive a
        // per-file rescan, so a project scan must follow the fix.
        assertEquals(false, KritFixTargets.perFileRescanCovers("DeadCode", perFile))
        assertEquals(false, KritFixTargets.perFileRescanCovers("MagicNumber", null))
        assertEquals(false, KritFixTargets.perFileRescanCovers("MagicNumber", emptySet()))
        assertEquals(false, KritFixTargets.perFileRescanCovers(null, perFile))
    }

    @Test
    fun `ruleOf finds the fixed finding's rule`() {
        val f = finding("/repo/A.kt")
        assertEquals("MagicNumber", KritFixTargets.ruleOf(listOf(f), f.findingId))
        assertNull(KritFixTargets.ruleOf(listOf(f), "Other:/repo/A.kt:1:1"))
    }
}
