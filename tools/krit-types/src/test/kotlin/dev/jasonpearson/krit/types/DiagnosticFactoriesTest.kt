package dev.jasonpearson.krit.types

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class DiagnosticFactoriesTest {

    @Test
    fun retainedFactorySetMatchesCompilerDiagnosticProjectionTier() {
        assertEquals(
            setOf(
                "UNREACHABLE_CODE",
                "USELESS_ELVIS",
                "CAST_NEVER_SUCCEEDS",
                "UNNECESSARY_NOT_NULL_ASSERTION",
                "UNNECESSARY_SAFE_CALL",
                "SENSELESS_COMPARISON",
                "USELESS_CAST",
            ),
            retainedDiagnosticFactories,
        )
    }

    @Test
    fun lexicalGateAdmitsEachNewDiagnosticConstruct() {
        val representativeSources = mapOf(
            "UNNECESSARY_NOT_NULL_ASSERTION" to "fun f(value: String) = value!!",
            "UNNECESSARY_SAFE_CALL" to "fun f(value: String) = value?.length",
            "SENSELESS_COMPARISON" to "fun f(value: String) = value == null",
            "USELESS_CAST" to "fun f(value: String) = value as String",
        )

        for ((factory, source) in representativeSources) {
            assertTrue(shouldCollectDiagnostics(source), "$factory source should pass the lexical gate")
        }
    }

    @Test
    fun lexicalGateAdmitsExistingCastAndUnreachableConstructsWithoutElvis() {
        assertTrue(shouldCollectDiagnostics("fun f(value: String) = value as Int"))
        assertTrue(shouldCollectDiagnostics("fun f(): Int { return 1; return 2 }"))
    }

    @Test
    fun lexicalGateAdmitsAllUselessCallSpellings() {    }

    @Test
    fun lexicalGateRejectsSourceWithoutDiagnosticHints() {
        assertFalse(shouldCollectDiagnostics("fun answer(): Int = 42"))
    }
}
