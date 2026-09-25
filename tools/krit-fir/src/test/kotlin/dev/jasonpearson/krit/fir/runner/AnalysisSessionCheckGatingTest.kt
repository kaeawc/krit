package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.buildCheckResponse
import dev.jasonpearson.krit.fir.plugins.PayloadParsers
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageLocation
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSeverity
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * The check response tells Go which requested files have an authoritative
 * checker verdict. Files the compiler could not analyze cleanly, and files
 * outside the JVM compilation, must be reported so Go keeps its own findings
 * for them.
 */
class AnalysisSessionCheckGatingTest {
    @TempDir lateinit var tmp: Path

    private val stdlib = File(Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath

    @Test fun collectorRecordsFirstErrorPerRequestedFileAndGlobalErrors() {
        val path = tmp.resolve("A.kt").toString()
        val other = tmp.resolve("Other.kt").toString()
        val collector = FindingCollector(mapOf(File(path).canonicalPath to path))
        val at = CompilerMessageLocation.create(path, 3, 5, null)
        collector.report(CompilerMessageSeverity.ERROR, "Unresolved reference 'missing'.", at)
        collector.report(CompilerMessageSeverity.ERROR, "second error", at)
        collector.report(CompilerMessageSeverity.ERROR, "error elsewhere", CompilerMessageLocation.create(other, 1, 1, null))
        collector.report(CompilerMessageSeverity.ERROR, "[ProtocolProbe] a rule reported at error severity", at)
        collector.report(CompilerMessageSeverity.ERROR, "no location", null)
        collector.report(CompilerMessageSeverity.EXCEPTION, "boom", null)

        assertEquals(mapOf(path to "Unresolved reference 'missing'."), collector.errorFiles)
        assertEquals(listOf("no location"), collector.globalErrors)
        assertEquals(listOf("boom"), collector.exceptions)
        assertEquals(listOf("ProtocolProbe"), collector.findings.map { it.rule })
    }

    @Test fun unresolvedReferenceGatesOnlyTheFileThatHasIt() {
        val src = tmp.resolve("src/main/kotlin/p").toFile().apply { mkdirs() }
        // Declared in a source-dir file that is not requested: resolving it
        // needs the whole-module compilation.
        File(src, "Decl.kt").writeText("package p\nclass Declared { fun protocolProbe() {} }\n")
        val clean = File(src, "Clean.kt").apply {
            writeText("package p\nfun protocolProbe() {}\nfun use(d: Declared) { protocolProbe(); d.protocolProbe() }\n")
        }
        val broken = File(src, "Broken.kt").apply {
            writeText("package p\nfun broken() { protocolProbe(); missingFunction() }\n")
        }
        val session = AnalysisSession(listOf(tmp.resolve("src/main/kotlin").toString()), listOf(stdlib))

        val result = session.check(1, listOf(FileRef(clean.path), FileRef(broken.path)), setOf("ProtocolProbe"))

        assertEquals(setOf(broken.path), result.errorFiles.keys, result.toString())
        assertTrue(result.errorFiles.getValue(broken.path).contains("missingFunction"), result.errorFiles.toString())
        assertTrue(result.crashed.isEmpty(), result.crashed.toString())
        assertEquals(1, result.succeeded)
        assertEquals(2, result.findings.count { it.path == clean.path && it.rule == "ProtocolProbe" }, result.toString())

        val response = buildCheckResponse(result)
        val errorFiles = PayloadParsers.extractObjectBlock(response, "errorFiles")
        assertTrue(errorFiles != null && broken.path in errorFiles, response)
    }

    @Test fun requestedFileFromANonJvmSourceSetIsExcludedNotCompiled() {
        val common = write("src/commonMain/kotlin/r/Common.kt", "package r\n\nexpect fun label(): String\nfun protocolProbe() {}\n")
        val jvm = write("src/jvmMain/kotlin/r/Jvm.kt", "package r\n\nactual fun label(): String = \"jvm\"\nfun useJvm() { protocolProbe() }\n")
        // A second actual for the same expect: compiled on the JVM it would
        // collide with jvmMain's actual and gate every file.
        val js = write("src/jsMain/kotlin/r/Js.kt", "package r\n\nactual fun label(): String = \"js\"\nfun useJs() { protocolProbe() }\n")
        val session = AnalysisSession(
            listOf(tmp.resolve("src/commonMain/kotlin").toString(), tmp.resolve("src/jvmMain/kotlin").toString()),
            listOf(stdlib),
        )

        val result = session.check(1, listOf(FileRef(common), FileRef(jvm), FileRef(js)), setOf("ProtocolProbe"))

        assertEquals(mapOf(js to AnalysisSession.NOT_IN_JVM_COMPILATION), result.errorFiles, result.toString())
        assertEquals(1, result.skipped)
        assertEquals(2, result.succeeded)
        assertEquals(listOf(jvm), result.findings.filter { it.rule == "ProtocolProbe" }.map { it.path }, result.toString())
    }

    @Test fun withoutSourceDirsEveryRequestedFileIsCompiled() {
        val file = write("src/jsMain/kotlin/r/Loose.kt", "package r\n\nfun protocolProbe() {}\nfun use() { protocolProbe() }\n")
        val result = AnalysisSession(emptyList(), listOf(stdlib)).check(1, listOf(FileRef(file)), setOf("ProtocolProbe"))
        assertTrue(result.errorFiles.isEmpty(), result.toString())
        assertEquals(listOf(file), result.findings.map { it.path })
    }

    private fun write(relative: String, text: String): String {
        val file = tmp.resolve(relative).toFile()
        file.parentFile.mkdirs()
        file.writeText(text)
        return file.path
    }
}
