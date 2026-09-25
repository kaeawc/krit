package dev.jasonpearson.krit.types

import kotlin.test.Test
import kotlin.test.assertEquals

class DiagnosticFactoriesTest {

    // Must stay in step with OracleDiagnosticMessageCollector's factory list in
    // krit-fir: both backends project the same compiler verdicts. Every analyzed
    // file is collected — DEPRECATION can fire on any reference, so there is no
    // lexical pre-gate; tests/parity covers that end to end across both backends.
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
                "DEPRECATION",
            ),
            retainedDiagnosticFactories,
        )
    }
}
