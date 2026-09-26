package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// TooGenericExceptionThrown cases that need more than one source file, a scan
// path, or rule options: the Go rule's path exemptions (test files, Gradle
// Kotlin scripts), `exceptionNames`, and project classes declared in another
// file.
class TooGenericExceptionThrownFilesTest {

    private val rule = "TooGenericExceptionThrown"

    private fun findings(
        sources: Map<String, String>,
        testFiles: Set<String> = emptySet(),
        scanPaths: Map<String, String> = emptyMap(),
        options: Map<String, Any?>? = null,
    ): Map<String, Int> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(
                enabledRuleIds = setOf(rule),
                ruleConfigs = options?.let { mapOf(rule to it) }.orEmpty(),
            ),
            testFiles = testFiles,
            scanPaths = scanPaths,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == rule }.groupingBy { it.file }.eachCount()
    }

    private fun thrower(pkg: String) = """
        package $pkg

        fun fail(): Nothing = throw RuntimeException("boom")
    """.trimIndent()

    @Test fun requestListedTestFileIsSkipped() {
        val sources = mapOf("Listed.kt" to thrower("listed"), "Unlisted.kt" to thrower("unlisted"))
        assertEquals(mapOf("Unlisted.kt" to 1), findings(sources, testFiles = setOf("Listed.kt")))
    }

    // Go skips a scan path ending in `.gradle.kts`.
    @Test fun gradleKotlinScriptIsSkipped() {
        val sources = mapOf("Build.kt" to thrower("build"), "App.kt" to thrower("app"))
        val scanPaths = mapOf("Build.kt" to "app/build.gradle.kts", "App.kt" to "app/src/main/kotlin/App.kt")
        assertEquals(mapOf("App.kt" to 1), findings(sources, scanPaths = scanPaths))
    }

    // `exceptionNames` replaces the default list: RuntimeException is no longer
    // reported, and a configured java.lang or kotlin class is (Go leaves the
    // name unresolved and reports it). A project class with a configured name
    // is not.
    @Test fun exceptionNamesReplaceTheDefaults() {
        val sources = mapOf(
            "Configured.kt" to """
                package configured

                fun a(): Nothing = throw RuntimeException("not listed")
                fun b(): Nothing = throw Exception("listed")
                fun c(): Nothing = throw IllegalStateException("listed")
                fun d(): Nothing = throw kotlin.IllegalStateException("listed")
                fun e(): Nothing = throw IllegalArgumentException("not listed")
                fun f(): Nothing = throw configured.more.ApiException("project class")
            """.trimIndent(),
            "More.kt" to """
                package configured.more

                class ApiException(message: String) : IllegalStateException(message)
            """.trimIndent(),
        )
        val options = mapOf("exceptionNames" to listOf("Exception", "IllegalStateException", "ApiException"))
        assertEquals(mapOf("Configured.kt" to 3), findings(sources, options = options))
    }

    // An empty list falls back to the defaults, as in Go.
    @Test fun emptyExceptionNamesUseTheDefaults() {
        val sources = mapOf("Empty.kt" to thrower("empty"))
        assertEquals(mapOf("Empty.kt" to 1), findings(sources, options = mapOf("exceptionNames" to emptyList<String>())))
    }

    // A project class named Exception in another file of the same package, or
    // imported from another package, is not java.lang.Exception. Go resolves
    // both through its class index and import table and skips them too.
    @Test fun projectClassesInOtherFilesAreNotGeneric() {
        val sources = mapOf(
            "Declared.kt" to """
                package shadow

                class Exception(message: String) : Throwable(message)
            """.trimIndent(),
            "SamePackage.kt" to """
                package shadow

                fun samePackage(): Nothing = throw Exception("same package")
            """.trimIndent(),
            "Imported.kt" to """
                package other

                import shadow.Exception

                fun imported(): Nothing = throw Exception("imported")
            """.trimIndent(),
        )
        assertEquals(emptyMap(), findings(sources))
    }
}
