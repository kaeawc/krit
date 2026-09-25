package dev.jasonpearson.krit.fir

import dev.jasonpearson.krit.fir.checkers.protocol.ProtocolProbe
import dev.jasonpearson.krit.fir.checkers.protocol.TypeProtocolProbe
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
        assertFalse(ProtocolProbe in excluded.expression.functionCallCheckers)
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

    @Test fun typeCheckerContributionRunsInARealCompile() {
        val merged = mergeFirRules(FirRuleDiscovery.enabled(FirRuleCompileContext(setOf("TypeProtocolProbe"))))
        assertTrue(TypeProtocolProbe in merged.type.resolvedTypeRefCheckers)
        val file = tmp.resolve("TypeProbe.kt").toFile().apply {
            writeText("class ProtocolProbeType\nval probe: ProtocolProbeType? = null\n")
        }
        val stdlib = java.io.File(kotlin.Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
        val result = AnalysisSession(listOf(tmp.toString()), listOf(stdlib))
            .check(2, listOf(FileRef(file.absolutePath)), setOf("TypeProtocolProbe"), emptyMap())
        assertTrue(result.findings.any { it.rule == "TypeProtocolProbe" && it.line == 2 }, result.toString())
    }

    @Test fun wireRequestRuleConfigsReachFirRuleConfig() {
        val file = tmp.resolve("WireProbe.kt").toFile().apply {
            writeText("fun protocolProbe() {}\nfun use() { protocolProbe() }\n")
        }
        val stdlib = java.io.File(kotlin.Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
        val session = AnalysisSession(listOf(tmp.toString()), listOf(stdlib))
        // Same shape internal/firchecks marshals for a check request.
        val line = """{"id":5,"command":"check","files":[{"path":${jsonStr(file.absolutePath)}}],""" +
            """"rules":["ProtocolProbe"],"ruleConfigs":{"ProtocolProbe":{"tag":"from go\nconfig"}}}"""
        val result = handleRequestLine(line, session, System.currentTimeMillis())
        val response = (result as RequestResult.Response).json
        assertTrue("""configured: from go\nconfig""" in response, response)
    }

    @Test fun checkRequestParsesNestedRuleConfigs() {
        val request = parseRequest("""{"id":3,"command":"check","rules":["ProtocolProbe"],"ruleConfigs":{"ProtocolProbe":{"tag":"hello","limit":2,"strict":true}}}""")
        assertEquals(listOf("ProtocolProbe"), request.rules)
        assertEquals("hello", request.ruleConfigs["ProtocolProbe"]?.get("tag"))
        assertEquals(2L, request.ruleConfigs["ProtocolProbe"]?.get("limit"))
        assertEquals(true, request.ruleConfigs["ProtocolProbe"]?.get("strict"))
    }

    @Test fun wireRequestTestFilesReachFirRuleIsTestFile() {
        // The request, not the path, decides: `src/test` below is NOT listed,
        // so it is checked like production code, while Listed.kt is a test file.
        val listed = tmp.resolve("Listed.kt").toFile().apply {
            writeText("fun protocolProbe() {}\nfun use() { protocolProbe() }\n")
        }
        val unlisted = tmp.resolve("src/test/Unlisted.kt").toFile().apply {
            parentFile.mkdirs()
            writeText("fun use2() { protocolProbe() }\n")
        }
        val stdlib = java.io.File(kotlin.Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
        val session = AnalysisSession(listOf(tmp.toString()), listOf(stdlib))
        // Same shape internal/firchecks marshals: testFiles is spelled as in
        // files, and an option named testFiles must not be read as the field.
        val line = """{"id":6,"command":"check","files":[{"path":${jsonStr(listed.absolutePath)}},""" +
            """{"path":${jsonStr(unlisted.absolutePath)}}],"rules":["ProtocolProbe"],""" +
            """"testFiles":[${jsonStr(listed.absolutePath)}],""" +
            """"ruleConfigs":{"ProtocolProbe":{"tag":"t","testFiles":[${jsonStr(unlisted.absolutePath)}]}}}"""
        val response = (handleRequestLine(line, session, System.currentTimeMillis()) as RequestResult.Response).json
        assertEquals(1, Regex("""configured: t \(test file\)""").findAll(response).count(), response)
        assertEquals(1, Regex("""configured: t"""").findAll(response).count(), "the unlisted src/test file is not a test file: $response")
    }

    @Test fun checkRequestParsesTestFilesAtTopLevelOnly() {
        val request = parseRequest(
            """{"id":4,"command":"check","files":[{"path":"/p/src/[id]/A.kt"},{"path":"/p/B.kt"}],""" +
                """"ruleConfigs":{"R":{"testFiles":["/p/B.kt"]}},"testFiles":["/p/src/[id]/A.kt"]}""",
        )
        assertEquals(setOf("/p/src/[id]/A.kt"), request.testFiles)
        assertEquals(listOf("/p/src/[id]/A.kt", "/p/B.kt"), request.files.map { it.path })
        assertEquals(emptySet(), parseRequest("""{"id":5,"command":"check","files":[]}""").testFiles)
    }

    @Test fun wireRequestScanPathsReachFirRuleScanPath() {
        val spelled = tmp.resolve("samples/proj/src/A.kt").toFile().apply {
            parentFile.mkdirs()
            writeText("fun protocolProbe() {}\nfun use() { protocolProbe() }\n")
        }
        val unspelled = tmp.resolve("B.kt").toFile().apply { writeText("fun use2() { protocolProbe() }\n") }
        val stdlib = java.io.File(kotlin.Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
        val session = AnalysisSession(listOf(tmp.toString()), listOf(stdlib))
        // Same shape internal/firchecks marshals: scanPaths is keyed by the
        // path as spelled in files, and only lists files whose scan spelling
        // differs.
        val line = """{"id":9,"command":"check","files":[{"path":${jsonStr(spelled.absolutePath)}},""" +
            """{"path":${jsonStr(unspelled.absolutePath)}}],"rules":["ProtocolProbe"],""" +
            """"scanPaths":{${jsonStr(spelled.absolutePath)}:"samples/proj/src/A.kt"},""" +
            """"ruleConfigs":{"ProtocolProbe":{"tag":"t","showScanPath":true}}}"""
        val response = (handleRequestLine(line, session, System.currentTimeMillis()) as RequestResult.Response).json
        assertTrue("configured: t scan=samples/proj/src/A.kt" in response, response)
        assertTrue("configured: t scan=${unspelled.absolutePath}" in response, "no scan spelling: as requested: $response")
    }

    @Test fun checkRequestParsesScanPathsAtTopLevelOnly() {
        val request = parseRequest(
            """{"id":7,"command":"check","files":[{"path":"/p/samples/[id]/A.kt"},{"path":"/p/B.kt"}],""" +
                """"scanPaths":{"/p/samples/[id]/A.kt":"samples/[id]/A.kt"},""" +
                """"ruleConfigs":{"R":{"scanPaths":{"/p/B.kt":"B.kt"}}}}""",
        )
        assertEquals(mapOf("/p/samples/[id]/A.kt" to "samples/[id]/A.kt"), request.scanPaths)
        assertEquals(listOf("/p/samples/[id]/A.kt", "/p/B.kt"), request.files.map { it.path })
        assertEquals("check", request.command)
        assertEquals(emptyMap(), parseRequest("""{"id":8,"command":"check","files":[]}""").scanPaths)
    }

    @Test fun compileContextMatchesTestFilesByRequestOrCanonicalSpelling() {
        val real = tmp.resolve("real").toFile().apply { mkdirs() }
        val file = real.resolve("T.kt").apply { writeText("") }
        val link = tmp.resolve("link")
        java.nio.file.Files.createSymbolicLink(link, real.toPath())
        val context = FirRuleCompileContext(testFiles = setOf(link.resolve("T.kt").toString()))
        assertTrue(context.isTestFile(link.resolve("T.kt").toString()))
        assertTrue(context.isTestFile(file.absolutePath), "canonical spelling of a listed file")
        assertFalse(context.isTestFile(real.resolve("Other.kt").absolutePath))
        assertFalse(context.isTestFile(null))
        assertFalse(FirRuleCompileContext().isTestFile(file.absolutePath))
        // No request context (oracle compile, direct compiler runs): nothing is a test file.
        assertFalse(ProtocolProbe.isTestFile(file.absolutePath))
        FirRuleContext.begin(context)
        try {
            assertTrue(ProtocolProbe.isTestFile(file.absolutePath))
        } finally { FirRuleContext.end() }
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
            assertFalse(ProtocolProbe in merged.expression.functionCallCheckers)
            assertTrue(OracleExpressionChecker in merged.expression.functionCallCheckers)
            assertTrue(OracleClassChecker in merged.declaration.classCheckers)
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
