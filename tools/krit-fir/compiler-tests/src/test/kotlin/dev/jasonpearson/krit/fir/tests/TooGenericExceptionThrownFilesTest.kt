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

    // The reported lines per file.
    private fun lines(
        sources: Map<String, String>,
        options: Map<String, Any?>? = null,
    ): Map<String, List<Int>> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(
                enabledRuleIds = setOf(rule),
                ruleConfigs = options?.let { mapOf(rule to it) }.orEmpty(),
            ),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == rule }.groupBy({ it.file }, { it.line }).mapValues { it.value.sorted() }
    }

    private val configuredNames = mapOf(
        "exceptionNames" to listOf(
            "Exception",
            "IllegalStateException",
            "ApiException",
            "NoSuchElementException",
            "ConcurrentModificationException",
            "IOException",
        ),
    )

    private val apiException = """
        package configured.more

        class ApiException(message: String) : IllegalStateException(message)
    """.trimIndent()

    // `exceptionNames` replaces the default list: RuntimeException is no longer
    // reported. A configured extra name counts for a library class of that
    // name however it is written: Go's resolver cannot resolve it and reports
    // it. That includes a kotlin.* alias of a java.util class
    // (NoSuchElementException, ConcurrentModificationException) and a fully
    // qualified java.io.IOException. A project class with a configured name is
    // not reported (Go finds it in its class index).
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
                fun g(): Nothing = throw NoSuchElementException("kotlin alias to java.util")
                fun h(): Nothing = throw ConcurrentModificationException("kotlin alias to java.util")
                fun i(): Nothing = throw java.io.IOException("qualified")
                fun j(): Nothing = throw java.util.NoSuchElementException("qualified")
            """.trimIndent(),
            "More.kt" to apiException,
        )
        assertEquals(mapOf("Configured.kt" to listOf(4, 5, 6, 9, 10, 11, 12)), lines(sources, configuredNames))
    }

    // A star-imported library class with a configured name is reported, as in
    // Go. A star-imported or same-package project class is not (Go finds it in
    // its class index). An explicitly imported library class is reported too:
    // Go resolves the import to an FQN outside its table and misses it, but the
    // configured class is thrown (a listed divergence).
    @Test fun configuredNamesMatchLibraryClassesHoweverImported() {
        val sources = mapOf(
            "Star.kt" to """
                package star

                import java.io.*
                import configured.more.*

                fun io(): Nothing = throw IOException("star-imported library class")
                fun api(): Nothing = throw ApiException("star-imported project class")
            """.trimIndent(),
            "Explicit.kt" to """
                package explicit

                import java.io.IOException

                fun io(): Nothing = throw IOException("explicitly imported library class")
            """.trimIndent(),
            "SamePackage.kt" to """
                package configured.more

                fun api(): Nothing = throw ApiException("same-package project class")
            """.trimIndent(),
            "More.kt" to apiException,
        )
        assertEquals(mapOf("Star.kt" to listOf(6), "Explicit.kt" to listOf(5)), lines(sources, configuredNames))
    }

    // Go's class index holds Kotlin declarations only, so a configured name
    // that resolves to a Java class counts even when that class is compiled
    // from source (here the Java stub layer, a Java source root like a
    // project's own Java code): Go reports the throw. Only a class declared in
    // a Kotlin source of the compilation is a project class.
    @Test fun javaSourceClassesWithAConfiguredNameAreReported() {
        val sources = mapOf(
            "JavaSource.kt" to """
                package javasource

                import android.database.*

                fun sql(): Nothing = throw SQLException("Java source class")
                fun qualified(): Nothing = throw android.database.SQLException("qualified")
            """.trimIndent(),
        )
        val options = mapOf("exceptionNames" to listOf("SQLException"))
        assertEquals(mapOf("JavaSource.kt" to listOf(5, 6)), lines(sources, options))
    }

    // An empty list falls back to the defaults, as in Go.
    @Test fun emptyExceptionNamesUseTheDefaults() {
        val sources = mapOf("Empty.kt" to thrower("empty"))
        assertEquals(mapOf("Empty.kt" to 1), findings(sources, options = mapOf("exceptionNames" to emptyList<String>())))
    }

    // A project class named Exception in another file of the same package, or
    // imported from another package, is not java.lang.Exception. Go skips only
    // the explicitly imported one (its import table resolves it to
    // shadow.Exception). Divergence (precision): Go reports the same-package
    // throw, because its resolver honors a project class only in the same file
    // and otherwise falls back to java.lang.Exception.
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

    // Divergence (precision): Go resolves a name with no explicit import and no
    // same-file class to java.lang, so it reports each of these throws. None
    // constructs a generic exception: a star import of a project class named
    // Exception beats Kotlin's default imports, a nested class of a supertype
    // is in scope in the subclass, and a same-package type alias named
    // Exception expands to IllegalStateException.
    @Test fun starImportsSupertypeNestedClassesAndSamePackageAliasesAreNotGeneric() {
        val sources = mapOf(
            "Declared.kt" to """
                package shadow

                class Exception(message: String) : Throwable(message)
            """.trimIndent(),
            "Star.kt" to """
                package star

                import shadow.*

                fun starImported(): Nothing = throw Exception("star-imported project class")
            """.trimIndent(),
            "Base.kt" to """
                package base

                open class Base {
                    class Error(message: String) : Throwable(message)
                }

                typealias Exception = IllegalStateException
            """.trimIndent(),
            "Derived.kt" to """
                package base

                class Derived : Base() {
                    fun nested(): Nothing = throw Error("nested class of the supertype")
                    fun aliased(): Nothing = throw Exception("same-package type alias")
                }
            """.trimIndent(),
        )
        assertEquals(emptyMap(), findings(sources))
    }
}
