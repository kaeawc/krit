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
        """{"id":$id,"files":[${paths.joinToString(",") { "{\"path\":\"$it\"}" }}],""" +
            """"sourceDirs":[${sourceDirs.joinToString(",") { "\"$it\"" }}],"classpath":["$stdlib"],"rules":["ProtocolProbe"],""" +
            """"ruleConfigs":{"ProtocolProbe":{"files":"opt-named-like-a-field","classpath":"x"}}}"""

    private fun request(id: Int, check: String?, files: List<String> = emptyList()): String {
        val params = buildList {
            if (check != null) add("\"kritSharedCheck\":$check")
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
            val checkOnly = respond(session, checkBlock(paths, listOf(root)).replaceFirst("{", "{\"command\":\"check\","))

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

    @Test fun anUnknownMethodIsRefusedWithUnknownCommand() {
        // The refusal shape the Go client keys on: "Unknown command: <method>".
        val session = AnalysisSession(emptyList(), emptyList())
        try {
            val response = respond(session, """{"id":10,"method":"analyzeWithCheckV2","params":{}}""")
            assertTrue("Unknown command: analyzeWithCheckV2" in response, response)
        } finally {
            session.dispose()
        }
    }

    @Test fun theSharedCheckCarriesTheSameWarningsAsAStandaloneCheck() {
        val (root, paths) = project()
        val args = listOf("-language-version=9.9")
        val session = AnalysisSession(listOf(root), listOf(stdlib), "", args)
        try {
            val block = checkBlock(paths, listOf(root)).replaceFirst("{", "{\"kotlinArgs\":[\"-language-version=9.9\"],")
            val standalone = respond(session, block.replaceFirst("{", "{\"command\":\"check\","))
            val shared = respond(session, request(14, block))
            val standaloneWarning = standalone.substringAfter("\"warning\":", "")
            assertTrue(standaloneWarning.isNotEmpty() && "languageVersion 9.9" in standaloneWarning, standalone.takeLast(200))
            val check = shared.substring(shared.indexOf(",\"check\":") + ",\"check\":".length)
            assertTrue(check.startsWith("{\"id\":1,"), check.take(80))
            // The embedded check ends with the same warning a standalone check carries.
            val sharedWarning = check.substringAfter("\"warning\":", "").substringBefore("}")
            assertEquals(standaloneWarning.substringBefore("}"), sharedWarning)
        } finally {
            session.dispose()
        }
    }

    @Test fun theKeyTokenInsideArraysOrDeeperMapKeysIsNotLifted() {
        val (root, paths) = project()
        val check = checkBlock(paths, listOf(root))
        // The real key sits after callFilter* in the sorted wire order; the token
        // as an array element or a nested map key must not be taken for it.
        val line = """{"id":11,"method":"analyzeWithCheck","params":{"callFilterCalleeNames":["a","kritSharedCheck"],""" +
            """"callFilterLexicalHintsByCallee":{"kritSharedCheck":["x"]},"kritSharedCheck":$check,"files":[]}}"""
        val parsed = parseRequest(line)
        assertEquals("analyzeWithCheck", parsed.command)
        assertEquals(1L, assertNotNull(parsed.sharedCheck).id)
        assertEquals(paths, parsed.sharedCheck!!.files.map { it.path })

        // Without the real key nothing is lifted, however the token is spelled.
        val none = parseRequest(
            """{"id":12,"method":"analyzeAll","params":{"callFilterCalleeNames":["a","kritSharedCheck"],""" +
                """"callFilterLexicalHintsByCallee":{"kritSharedCheck":["x"]}}}""",
        )
        assertNull(none.sharedCheck)
        assertEquals(12L, none.id)
    }

    @Test fun aMalformedSharedCheckIsAnErrorNotALeak() {
        val line = """{"id":13,"method":"analyzeWithCheck","params":{"kritSharedCheck":"nope"}}"""
        val result = handleRequestLine(line, AnalysisSession(emptyList(), emptyList()), startTime = 0L)
        assertTrue(result is RequestResult.ParseError && "Parse error" in result.json, result.toString())
    }

    // The findings array, ordered, from a check response.
    private fun normalizedFindings(json: String): List<String> {
        val body = json.substringAfter("\"findings\":[").substringBefore("],\"rules\"")
        return Regex("""\{[^{}]*\}""").findAll(body).map { it.value }.sorted().toList()
    }
}
