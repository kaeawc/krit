package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class UnnecessaryNotNullOperatorTest {
    private val rule = FirRuleCompileContext(enabledRuleIds = setOf("UnnecessaryNotNullOperator"))

    @Test fun errorTypesDoNotReport() {
        val result = KritFirProbe.compile(
            mapOf("ErrorAssertion.kt" to "fun errorAssertion(value: MissingAssertionType) = value!!"),
            rule,
        )
        assertFalse(result.clean)
        assertFalse(result.crashed)
        assertTrue(result.diags.isEmpty(), result.diags.toString())
    }

    @Test fun messagesAndSameLineCountsMatchGo() {
        val diagnostics = KritFirProbe.diagnose(
            mapOf("AssertionMessages.kt" to
                "fun assertionMessages(value: String) { println(value!!); println(value.length!!) }"),
            rule,
        )
        assertEquals(2, diagnostics.size)
        assertTrue(diagnostics.all { it.line == 1 && it.name == "UnnecessaryNotNullOperator" })
        assertTrue(diagnostics.any {
            "Unnecessary not-null assertion (!!) on non-nullable 'value'." in it.message
        })
        assertTrue(diagnostics.any {
            "Unnecessary not-null assertion (!!): 'value.length' is already non-null here " +
                "(non-null type or smart cast)." in it.message
        })
    }
}
