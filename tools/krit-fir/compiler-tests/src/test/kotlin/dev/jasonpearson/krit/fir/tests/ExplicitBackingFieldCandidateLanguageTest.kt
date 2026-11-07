package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// Explicit backing fields are stable from Kotlin 2.4. The checker asks the compiler session whether the
// feature exists, so a module on an older language version is not told to use it.
class ExplicitBackingFieldCandidateLanguageTest {

    private val source = """
        package test

        import kotlinx.coroutines.flow.MutableStateFlow
        import kotlinx.coroutines.flow.StateFlow

        class ViewModel {
            private val _state = MutableStateFlow(0)
            val state: StateFlow<Int> get() = _state
        }
    """.trimIndent()

    private fun candidates(languageVersion: String?): Int {
        val result = KritFirProbe.compile(
            mapOf("Main.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("ExplicitBackingFieldCandidate")),
        ) { settings -> languageVersion?.let { settings.languageVersion = it } }
        assertTrue(result.clean, result.problems())
        return result.diags.count { it.name == "ExplicitBackingFieldCandidate" }
    }

    @Test fun reportsOnTheDefaultLanguageVersion() {
        assertEquals(1, candidates(null))
    }

    @Test fun reportsOnLanguageVersion24() {
        assertEquals(1, candidates("2.4"))
    }

    @Test fun staysSilentBeforeLanguageVersion24() {
        assertEquals(0, candidates("2.3"))
    }
}
