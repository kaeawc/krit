package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// UnsafeCallOnNullableType cases that need request facts (test files, scan paths, options) or the message.
class UnsafeCallOnNullableTypeFilesTest {

    private fun source(pkg: String) = """
        package $pkg

        fun read(value: String?): Int = value!!.length
    """.trimIndent()

    private fun diagnostics(
        sources: Map<String, String>,
        testFiles: Set<String> = emptySet(),
        scanPaths: Map<String, String> = emptyMap(),
        config: Map<String, Any?> = emptyMap(),
    ): List<KritFirProbe.Diag> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(
                enabledRuleIds = setOf("UnsafeCallOnNullableType"),
                ruleConfigs = if (config.isEmpty()) emptyMap() else mapOf("UnsafeCallOnNullableType" to config),
            ),
            testFiles = testFiles,
            scanPaths = scanPaths,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "UnsafeCallOnNullableType" }
    }

    private fun filesWithFindings(
        sources: Map<String, String>,
        testFiles: Set<String> = emptySet(),
        scanPaths: Map<String, String> = emptyMap(),
    ) = diagnostics(sources, testFiles, scanPaths).map { it.file.substringAfterLast('/') }.toSet()

    @Test fun skipsTestFilesAndTestSupportDirectories() {
        val sources = mapOf(
            "Plain.kt" to source("plain"),
            "Listed.kt" to source("listed"),
            "Fake.kt" to source("fake"),
            "Testing.kt" to source("testing"),
            "Utils.kt" to source("utils"),
        )
        val scanPaths = mapOf(
            "Fake.kt" to "app/src/main/fakes/Fake.kt",
            "Testing.kt" to "feature-testing/src/main/kotlin/Testing.kt",
            "Utils.kt" to "lib/test-utils/Utils.kt",
        )
        assertEquals(setOf("Plain.kt"), filesWithFindings(sources, testFiles = setOf("Listed.kt"), scanPaths = scanPaths))
    }

    @Test fun reportsOnProductionPathsThatLookLikeTestNames() {
        val sources = mapOf("Contest.kt" to source("contest"))
        assertEquals(setOf("Contest.kt"), filesWithFindings(sources, scanPaths = mapOf("Contest.kt" to "app/src/main/kotlin/Contest.kt")))
    }

    private val previewSource = """
        package preview

        annotation class DevicePreview

        @DevicePreview
        fun screen(value: String?): Int = value!!.length

        fun other(value: String?): Int = value!!.length
    """.trimIndent()

    @Test fun customPreviewAnnotationsFollowTheWildcardOption() {
        // `DevicePreview` ends with Preview, so the wildcard (on by default) treats screen() as scaffolding.
        assertEquals(1, diagnostics(mapOf("P.kt" to previewSource)).size)
        assertEquals(2, diagnostics(mapOf("P.kt" to previewSource), config = mapOf("customPreviewWildcard" to false)).size)
    }

    @Test fun customPreviewPrefixesNameExtraPreviewAnnotations() {
        // Without the wildcard, a prefix still matches `<prefix>Preview`, so screen() is scaffolding again.
        val off = mapOf<String, Any?>("customPreviewWildcard" to false)
        assertEquals(1, diagnostics(mapOf("P.kt" to previewSource), config = off + ("customPreviewPrefixes" to listOf("Device"))).size)
        assertEquals(2, diagnostics(mapOf("P.kt" to previewSource), config = off + ("customPreviewPrefixes" to listOf("Other"))).size)
    }

    private fun message(code: String): String {
        val diags = diagnostics(mapOf("M.kt" to code))
        assertEquals(1, diags.size, diags.toString())
        return diags.single().message
    }

    private val detekt =
        "Calling !! on a nullable type will throw a NullPointerException at runtime in case the value is null. It should be avoided."

    @Test fun plainMessage() {
        val text = message("package m\nfun f(v: String?): Int = v!!.length\n")
        assertEquals(
            "$detekt Prefer requireNotNull(x) { \"reason\" } or checkNotNull(x) { \"reason\" }; use a safe call (?.) only when null is a valid case.",
            text.removePrefix("[UnsafeCallOnNullableType] "),
        )
    }

    @Test fun argumentMessage() {
        val text = message("package m\nfun take(s: String) {}\nfun f(v: String?) { take(v!!) }\n")
        assertTrue(text.endsWith("Use requireNotNull(x) { \"reason\" } on a call argument to document the precondition."), text)
    }

    @Test fun columnReadMessage() {
        val text = message("package m\nclass C { fun getString(i: Int): String? = null }\nfun f(cursor: C): String = cursor.getString(0)!!\n")
        assertTrue(text.contains("on a value read from a column or cursor"), text)
    }

    @Test fun superMessage() {
        val text = message(
            "package m\nopen class A { open fun name(): String? = null }\nclass B : A() { override fun name(): String = super.name()!! }\n",
        )
        assertTrue(text.contains("to document why a platform-type call result cannot be null."), text)
    }

    private val model = """
        package model

        class Holder(val name: String?)
    """.trimIndent()

    private fun use(pkg: String) = """
        package $pkg

        import model.Holder

        fun length(holder: Holder): Int {
            if (holder.name != null) {
                return holder.name!!.length
            }
            return 0
        }
    """.trimIndent()

    // The check compile may read a dependency from source, where its property looks stable. A build of the using
    // module cannot smart-cast a property of another module, so the `!!` is needed there.
    @Test fun smartCastOnAPropertyOfAnotherModuleIsNotStable() {
        val sources = mapOf(
            "libraries/model/src/main/kotlin/Model.kt" to model,
            "features/screen/src/main/kotlin/Use.kt" to use("screen"),
        )
        assertEquals(setOf("Use.kt"), filesWithFindings(sources))
    }

    @Test fun smartCastOnAPropertyOfTheSameModuleIsStable() {
        val sources = mapOf(
            "libraries/model/src/main/kotlin/Model.kt" to model,
            "libraries/model/src/main/kotlin/Use.kt" to use("screen"),
        )
        assertEquals(emptySet(), filesWithFindings(sources))
    }
}
