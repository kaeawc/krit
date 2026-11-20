package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class HardcodedCryptoTestFilesTest {

    private fun source(pkg: String) = """
        package $pkg

        import javax.crypto.spec.IvParameterSpec
        import javax.crypto.spec.SecretKeySpec

        fun configureCrypto() {
            IvParameterSpec(byteArrayOf(1, 2, 3, 4))
            SecretKeySpec(byteArrayOf(1, 2, 3, 4), "AES")
        }
    """.trimIndent()

    @Test fun skipsRequestedTestSourceSetsButKeepsProductionFindings() {
        val sources = mapOf(
            "app/src/main/kotlin/ProductionCrypto.kt" to source("production"),
            "app/src/test/kotlin/UnitCrypto.kt" to source("unit"),
            "app/src/androidTest/kotlin/InstrumentedCrypto.kt" to source("instrumented"),
        )
        val testFiles = setOf(
            "app/src/test/kotlin/UnitCrypto.kt",
            "app/src/androidTest/kotlin/InstrumentedCrypto.kt",
        )
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("HardcodedSecretKey", "StaticIv")),
            testFiles = testFiles,
        )

        assertTrue(result.clean, result.problems())
        assertEquals(
            mapOf("ProductionCrypto.kt" to 2),
            result.diags.filter { it.name in setOf("HardcodedSecretKey", "StaticIv") }
                .groupingBy { it.file }
                .eachCount(),
        )
    }
}
