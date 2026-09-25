package dev.jasonpearson.krit.fir

import dev.jasonpearson.krit.fir.checkers.protocol.ProtocolProbe
import dev.jasonpearson.krit.fir.oracle.OracleClassChecker
import dev.jasonpearson.krit.fir.oracle.OracleExpressionChecker
import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.FileRef
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class FirRuleProtocolTest {
    @TempDir lateinit var tmp: Path

    @Test fun testOnlyRuleIsDiscoveredSelectedAndConfiguredEndToEnd() {
        assertTrue(FirRuleDiscovery.rules.any { it === ProtocolProbe })
        val testCodeSource = java.io.File(ProtocolProbe::class.java.protectionDomain.codeSource.location.toURI())
        val mainCodeSource = java.io.File(FirRuleDiscovery::class.java.protectionDomain.codeSource.location.toURI())
        assertTrue(testCodeSource.isDirectory)
        assertFalse(testCodeSource == mainCodeSource)
        val excluded = mergeFirRules(FirRuleDiscovery.enabled(FirRuleCompileContext(setOf("InjectDispatcher"))))
        assertFalse(ProtocolProbe in excluded.first.functionCallCheckers)
        val file = tmp.resolve("Probe.kt").toFile().apply {
            writeText("fun protocolProbe() {}\nfun use() { protocolProbe() }\n")
        }
        val session = AnalysisSession(listOf(tmp.toString()), listOf(java.io.File(kotlin.Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath))
        val result = session.check(1, listOf(FileRef(file.absolutePath)), setOf("ProtocolProbe", "UnknownRule"),
            mapOf("ProtocolProbe" to mapOf("tag" to "hello")))
        assertEquals(listOf("ProtocolProbe"), result.rules)
        assertTrue(result.findings.any { it.rule == "ProtocolProbe" && it.message == "configured: hello" }, result.toString())
        assertTrue("\"rule\":\"ProtocolProbe\"" in buildCheckResponseForTest(result))
    }

    @Test fun checkRequestParsesNestedRuleConfigs() {
        val request = parseRequest("""{"id":3,"command":"check","rules":["ProtocolProbe"],"ruleConfigs":{"ProtocolProbe":{"tag":"hello","limit":2,"strict":true}}}""")
        assertEquals(listOf("ProtocolProbe"), request.rules)
        assertEquals("hello", request.ruleConfigs["ProtocolProbe"]?.get("tag"))
        assertEquals(2L, request.ruleConfigs["ProtocolProbe"]?.get("limit"))
        assertEquals(true, request.ruleConfigs["ProtocolProbe"]?.get("strict"))
    }

    @Test fun oracleContextHasNoRuleCheckersButKeepsOracleCheckers() {
        FirRuleContext.begin(FirRuleCompileContext(noneEnabled = true))
        try {
            val rules = FirRuleDiscovery.enabled()
            assertTrue(rules.isEmpty())
            val merged = mergeFirRules(rules,
                baseExpressions = listOf(object : org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers() {
                    override val functionCallCheckers = setOf(OracleExpressionChecker)
                }),
                baseDeclarations = listOf(object : org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers() {
                    override val classCheckers = setOf(OracleClassChecker)
                }))
            assertFalse(ProtocolProbe in merged.first.functionCallCheckers)
            assertTrue(OracleExpressionChecker in merged.first.functionCallCheckers)
            assertTrue(OracleClassChecker in merged.second.classCheckers)
        } finally { FirRuleContext.end() }
        val file = tmp.resolve("OracleProbe.kt").toFile().apply {
            writeText("fun protocolProbe() {}\nfun use() { protocolProbe() }\n")
        }
        ProtocolProbe.checks.set(0)
        val stdlib = java.io.File(kotlin.Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
        val outcome = AnalysisSession(listOf(tmp.toString()), listOf(stdlib)).analyzeFull(listOf(file.absolutePath))
        assertTrue(outcome.result.files.isNotEmpty())
        assertEquals(0, ProtocolProbe.checks.get(), "oracle compile invoked a FIR rule checker")
    }

    private fun buildCheckResponseForTest(result: dev.jasonpearson.krit.fir.runner.BatchResult): String =
        dev.jasonpearson.krit.fir.buildCheckResponse(result)
}
