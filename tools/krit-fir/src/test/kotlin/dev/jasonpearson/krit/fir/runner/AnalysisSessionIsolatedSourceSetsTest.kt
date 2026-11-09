package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.CheckRequest
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/** Source sets of one module compiled together redeclare each other; compiled one by one they do not. */
class AnalysisSessionIsolatedSourceSetsTest {
    @TempDir lateinit var tmp: Path

    private val rules = setOf("PrintlnInProduction")

    private fun write(set: String, name: String, code: String): String =
        tmp.resolve("src/$set/kotlin/$name.kt").toFile().apply { parentFile.mkdirs(); writeText(code) }.absolutePath

    private fun root(set: String) = tmp.resolve("src/$set/kotlin").toString()

    private val mainFile get() = write("main", "Shared", "package app\ninternal fun shared() = 1\nclass Helper")
    private val testFile get() = write("test", "Fake", "package app\nclass Fake\nfun useMain() = shared() + Helper().hashCode()")
    private val androidTestFile get() = write("androidTest", "Fake", "package app\nclass Fake")

    private fun isolated(set: String, files: List<String>): BatchResult {
        val spec = ModuleSpec(id = ":m:$set", sourceRoots = listOf(root(set)), contextSourceRoots = listOf(root("main")))
        val index = AnalysisSession.ModuleCheckIndex(CheckRequest(id = 1, command = "check", files = files.map { FileRef(it) }, rules = rules.toList()))
        val session = AnalysisSession(emptyList(), emptyList())
        try { return requireNotNull(session.analyzeModuleFacts(spec, index).second) } finally { session.dispose() }
    }

    @Test fun mergedSourceSetsReportRedeclaration() {
        val files = listOf(mainFile, testFile, androidTestFile)
        val session = AnalysisSession(listOf("main", "test", "androidTest").map(::root), emptyList())
        try {
            val result = session.check(1, files.map { FileRef(it) }, rules)
            assertTrue(result.errorFiles.isNotEmpty(), "union compile should report the duplicate Fake: $result")
        } finally { session.dispose() }
    }

    @Test fun isolatedSourceSetsDoNotReportRedeclaration() {
        mainFile
        val test = testFile
        val androidTest = androidTestFile
        // The test set sees main as context, including its internal declarations; androidTest never sees test.
        for ((set, file) in listOf("test" to test, "androidTest" to androidTest)) {
            val result = isolated(set, listOf(file))
            assertEquals(emptyMap(), result.errorFiles, "$set: $result")
            assertEquals(1, result.succeeded, "$set: $result")
        }
    }
}
