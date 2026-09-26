package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertContains
import kotlin.test.assertEquals

class UnnecessaryFilterSuggestionTest {
    @Test
    fun explicitLabelSurvivesReplacement() {
        val source = "fun labeled(xs: List<Int>) = xs.filter pos@{ return@pos it > 0 }.first()"
        val diagnostics = KritFirProbe.diagnose(
            mapOf("Main.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("UnnecessaryFilter")),
        ).filter { it.name == "UnnecessaryFilter" }
        assertEquals(1, diagnostics.size)
        assertContains(diagnostics.single().message, ".first pos@{ return@pos it > 0 }")
    }

    @Test
    fun implicitFilterReturnIsNotSuggested() {
        val source = "fun labeled(xs: List<Int>) = xs.filter { return@filter it > 0 }.first()"
        val diagnostics = KritFirProbe.diagnose(
            mapOf("Main.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("UnnecessaryFilter")),
        ).filter { it.name == "UnnecessaryFilter" }
        assertEquals(0, diagnostics.size)
    }
}
