package dev.jasonpearson.krit.fir

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * #810: the daemon's `analyzeWithCheck` answers the oracle's `analyze` and the
 * rule `check` from one compilation, through the same `handleRequestLine` entry
 * point both daemon loops use.
 */
class AnalyzeWithCheckDispatchTest {
    @TempDir lateinit var tmp: Path

    private val stdlib = File(Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath

    private fun write(rel: String, text: String): File =
        tmp.resolve(rel).toFile().apply { parentFile.mkdirs(); writeText(text) }

    private fun project(): Pair<String, List<String>> {
        val root = tmp.resolve("src/main/kotlin").toString()
        val files = listOf(
            write("src/main/kotlin/p/Decl.kt", "package p\nopen class Base { fun protocolProbe() {} }\n"),
            write("src/main/kotlin/p/Use.kt", "package p\nfun protocolProbe() {}\nclass Leaf : Base()\nfun use(l: Leaf): Int { protocolProbe(); l.protocolProbe(); return 1 }\n"),
        )
        return root to files.map { it.path }
    }

    private fun checkBlock(paths: List<String>, sourceDirs: List<String>, id: Int = 1) =
        """{"id":$id,"command":"check","files":[${paths.joinToString(",") { "{\"path\":\"$it\"}" }}],""" +
            """"sourceDirs":[${sourceDirs.joinToString(",") { "\"$it\"" }}],"classpath":["$stdlib"],"rules":["ProtocolProbe"],""" +
            """"ruleConfigs":{"ProtocolProbe":{"files":"opt-named-like-a-field","classpath":"x"}}}"""

    private fun request(id: Int, check: String?, files: List<String> = emptyList()): String {
        val params = buildList {
            if (check != null) add("\"sharedCheck\":$check")
            add("\"files\":[${files.joinToString(",") { "\"$it\"" }}]")
        }.joinToString(",")
        return """{"id":$id,"method":"analyzeWithCheck","params":{$params}}"""
    }

    private fun respond(session: AnalysisSession, line: String): String =
        (handleRequestLine(line, session, startTime = 0L) as RequestResult.Response).json

    @Test fun oneRequestAnswersTheOracleAndTheCheck() {
        val (root, paths) = project()
        val session = AnalysisSession(listOf(root), listOf(stdlib))
        try {
            val response = respond(session, request(5, checkBlock(paths, listOf(root))))
            val oracleOnly = respond(session, """{"id":5,"method":"analyzeAll"}""")
            val checkOnly = respond(session, checkBlock(paths, listOf(root)))

            assertTrue(response.startsWith(oracleOnly.dropLast(1) + ",\"check\":"), response.take(300))
            val check = response.substring(response.indexOf(",\"check\":") + ",\"check\":".length).dropLast(1)
            // The check is the one a `check` request for the same files gives.
            assertEquals(normalizedFindings(checkOnly), normalizedFindings(check))
            assertTrue("\"ProtocolProbe\"" in check, check)
            assertTrue(check.startsWith("{\"id\":1,"), check.take(80))
        } finally {
            session.dispose()
        }
    }

    @Test fun anotherCompileContextLeavesTheCheckNull() {
        val (root, paths) = project()
        val session = AnalysisSession(listOf(root), listOf(stdlib))
        try {
            val elsewhere = tmp.resolve("elsewhere").toString()
            val response = respond(session, request(6, checkBlock(paths, listOf(elsewhere))))
            assertTrue(response.endsWith(",\"check\":null}"), response.takeLast(80))
            assertEquals(respond(session, """{"id":6,"method":"analyzeAll"}"""), response.removeSuffix(",\"check\":null}") + "}")
        } finally {
            session.dispose()
        }
    }

    @Test fun withoutACheckItIsThePlainAnalyze() {
        val (root, _) = project()
        val session = AnalysisSession(listOf(root), listOf(stdlib))
        try {
            val response = respond(session, request(7, null))
            assertEquals(respond(session, """{"id":7,"method":"analyzeAll"}"""), response)
            assertFalse("\"check\"" in response)
        } finally {
            session.dispose()
        }
    }

    @Test fun theEmbeddedCheckDoesNotLeakIntoTheOuterRequest() {
        val (root, paths) = project()
        val parsed = parseRequest(request(8, checkBlock(paths, listOf(root)), files = listOf(paths[0])))
        assertEquals("analyzeWithCheck", parsed.command)
        assertEquals(listOf(paths[0]), parsed.files.map { it.path })
        assertTrue(parsed.rules.isEmpty() && parsed.sourceDirs.isEmpty() && parsed.classpath.isEmpty() && parsed.ruleConfigs.isEmpty())
        val check = assertNotNull(parsed.sharedCheck)
        assertEquals(1L, check.id)
        assertEquals(paths, check.files.map { it.path })
        assertEquals(listOf("ProtocolProbe"), check.rules)
        assertEquals(listOf(root), check.sourceDirs)
        assertEquals(listOf(stdlib), check.classpath)
        assertNull(check.sharedCheck)
        assertNull(parseRequest("""{"id":9,"method":"analyzeAll","params":{"files":[]}}""").sharedCheck)
    }

    @Test fun anOldJarStyleRequestWithoutTheCommandIsRefused() {
        // A jar without this command answers "Unknown command", which is how the
        // Go client detects it; here that is pinned for a command we do not know.
        val session = AnalysisSession(emptyList(), emptyList())
        try {
            val response = respond(session, """{"id":10,"method":"analyzeWithCheckV2","params":{}}""")
            assertTrue("Unknown command: analyzeWithCheckV2" in response, response)
        } finally {
            session.dispose()
        }
    }

    // The findings array, ordered, from a check response.
    private fun normalizedFindings(json: String): List<String> {
        val body = json.substringAfter("\"findings\":[").substringBefore("],\"rules\"")
        return Regex("""\{[^{}]*\}""").findAll(body).map { it.value }.sorted().toList()
    }
}
