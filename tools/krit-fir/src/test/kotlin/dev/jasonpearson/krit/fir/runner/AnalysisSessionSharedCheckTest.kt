package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.CheckRequest
import dev.jasonpearson.krit.fir.buildCheckResponse
import dev.jasonpearson.krit.fir.oracle.OracleResponse
import dev.jasonpearson.krit.fir.runOneShot
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
 * #739: one compilation serves both the oracle and the rule checkers. Each
 * side must get exactly what its own compile would have given it.
 */
class AnalysisSessionSharedCheckTest {
    @TempDir lateinit var tmp: Path

    private val stdlib = File(Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath

    private fun write(rel: String, text: String): File =
        tmp.resolve(rel).toFile().apply { parentFile.mkdirs(); writeText(text) }

    private fun project(): Pair<String, List<File>> {
        val root = tmp.resolve("src/main/kotlin").toString()
        val files = listOf(
            write("src/main/kotlin/p/Decl.kt", "package p\nopen class Base { fun protocolProbe() {} }\n"),
            write("src/main/kotlin/p/Use.kt", "package p\nfun protocolProbe() {}\nclass Leaf : Base()\nfun use(l: Leaf): Int { protocolProbe(); l.protocolProbe(); return 1 }\n"),
            write("src/main/kotlin/p/Broken.kt", "package p\nfun broken() { protocolProbe(); missingFunction() }\n"),
        )
        return root to files
    }

    private fun request(files: List<String>, rules: List<String> = listOf("ProtocolProbe")) =
        CheckRequest(id = 7, command = "check", files = files.map { FileRef(it) }, rules = rules)

    // Findings arrive in compiler report order; compare them as a set.
    private fun normalized(result: BatchResult) = result.copy(findings = result.findings.sortedWith(
        compareBy({ it.path }, { it.line }, { it.col }, { it.rule }),
    ))

    @Test fun sharedCompileMatchesSeparateCompiles() {
        val (root, files) = project()
        val paths = files.map { it.path }

        val separate = AnalysisSession(listOf(root), listOf(stdlib))
        val oracleOnly = separate.analyzeFull(paths)
        val checkOnly = separate.check(7, paths.map { FileRef(it) }, setOf("ProtocolProbe"))

        val (shared, checked) = AnalysisSession(listOf(root), listOf(stdlib)).analyzeWithCheck(paths, request(paths))

        assertNotNull(checked, "the check should share the oracle's compilation")
        assertEquals(normalized(checkOnly), normalized(checked))
        assertEquals(OracleResponse.buildOneShot(oracleOnly.result), OracleResponse.buildOneShot(shared.result))
        assertEquals(OracleResponse.buildCacheDeps(oracleOnly.cacheDeps), OracleResponse.buildCacheDeps(shared.cacheDeps))
        // Sanity: the fixture exercises findings, a gated file, and oracle facts.
        assertTrue(checked.findings.any { it.rule == "ProtocolProbe" }, checked.toString())
        assertTrue(files[2].path in checked.errorFiles, checked.errorFiles.toString())
        assertFalse(OracleResponse.buildOneShot(shared.result).contains("ProtocolProbe"), "rule diagnostics leaked into oracle facts")
    }

    @Test fun oracleKeepsItsSpellingAndCheckKeepsTheRequestSpelling() {
        val (root, files) = project()
        val oracleSpelling = files.map { it.path }
        // Same files, spelled differently by the check request.
        val checkSpelling = files.map { File(it.parentFile, "./" + it.name).path }

        val (shared, checked) = AnalysisSession(listOf(root), listOf(stdlib)).analyzeWithCheck(oracleSpelling, request(checkSpelling))
        val alone = AnalysisSession(listOf(root), listOf(stdlib)).analyzeFull(oracleSpelling)

        assertNotNull(checked)
        assertEquals(OracleResponse.buildOneShot(alone.result), OracleResponse.buildOneShot(shared.result))
        assertTrue(checked.findings.isNotEmpty() && checked.findings.all { it.path in checkSpelling }, checked.findings.toString())
        assertTrue(checked.errorFiles.keys.all { it in checkSpelling }, checked.errorFiles.toString())
    }

    @Test fun differentSourcesOrContextDoNotShareTheCompile() {
        val (root, files) = project()
        val paths = files.map { it.path }
        val outside = write("loose/Loose.kt", "package q\nfun loose() {}\n").path
        val session = AnalysisSession(listOf(root), listOf(stdlib))

        // The check compiles a file the oracle's compilation does not have.
        val (outcome, checked) = session.analyzeWithCheck(paths, request(paths + outside))
        assertNull(checked)
        assertEquals(OracleResponse.buildOneShot(session.analyzeFull(paths).result), OracleResponse.buildOneShot(outcome.result))

        // The request names another compile context.
        val other = request(paths).copy(classpath = listOf(stdlib, outside))
        assertNull(session.analyzeWithCheck(paths, other).second)

        // Nothing to compile for the check.
        assertNull(session.analyzeWithCheck(paths, request(listOf(tmp.resolve("build.gradle.kts").toString()))).second)
    }

    @Test fun oneShotWritesTheCheckResponseOnlyWhenShared() {
        val (root, files) = project()
        val paths = files.map { it.path }
        val output = tmp.resolve("types.json").toFile()
        val checkOut = tmp.resolve("check.json").toFile()
        val requestFile = tmp.resolve("request.json").toFile()
        requestFile.writeText(
            """{"id":3,"command":"check","files":[${paths.joinToString(",") { "{\"path\":\"$it\"}" }}],""" +
                """"sourceDirs":["$root"],"rules":["ProtocolProbe"]}""",
        )

        runOneShot(listOf(root), output.path, null, listOf(stdlib), cacheDepsOutPath = null,
            checkRequestPath = requestFile.path, checkOutPath = checkOut.path)

        assertTrue(output.length() > 0)
        val expected = buildCheckResponse(AnalysisSession(listOf(root), listOf(stdlib)).check(3, paths.map { FileRef(it) }, setOf("ProtocolProbe")))
        assertEquals(expected.length, checkOut.readText().length, checkOut.readText())
        assertTrue("\"ProtocolProbe\"" in checkOut.readText())

        // A request for another compilation leaves --check-out unwritten.
        checkOut.delete()
        requestFile.writeText("""{"id":4,"command":"check","files":[{"path":"${paths[0]}"}],"sourceDirs":["${tmp.resolve("elsewhere")}"]}""")
        runOneShot(listOf(root), output.path, null, listOf(stdlib), cacheDepsOutPath = null,
            checkRequestPath = requestFile.path, checkOutPath = checkOut.path)
        assertFalse(checkOut.exists())
    }
}
