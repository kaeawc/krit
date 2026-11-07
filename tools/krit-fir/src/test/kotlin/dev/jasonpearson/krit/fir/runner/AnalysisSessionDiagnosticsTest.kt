package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.oracle.DiagnosticPayload
import org.junit.jupiter.api.Assumptions.assumeTrue
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * End-to-end coverage for the diagnostic projection in
 * [`AnalysisSession.analyze`]: the retained K2 warning factories round-trip
 * through the [`OracleDiagnosticMessageCollector`] into each
 * [`FilePayload.diagnostics`] list.
 */
class AnalysisSessionDiagnosticsTest {

    @TempDir
    lateinit var tmp: Path

    private lateinit var stdlibClasspath: List<String>

    @BeforeEach
    fun resolveStdlib() {
        val stdlib = findKotlinStdlib()
        assumeTrue(
            stdlib != null,
            "kotlin-stdlib jar not found in Gradle cache; K2 needs it on the classpath to resolve " +
                "built-in types and emit warning-level diagnostics. Set KOTLIN_STDLIB_JAR or " +
                "populate the Gradle cache by running `./gradlew :test` once at the repo root.",
        )
        stdlibClasspath = listOfNotNull(stdlib)
    }

    @Test
    fun uselessElvisOnNonNullableLhsIsRecorded() {
        val path = writeKt(
            "Elvis.kt",
            """
            package com.acme.elvis

            fun caller(): String {
                val s: String = "hi"
                return s ?: "fallback"
            }
            """.trimIndent(),
        )

        val diagnostics = diagnosticsFor(path)
        assertTrue(
            diagnostics.any { it.factoryName == "USELESS_ELVIS" },
            "expected USELESS_ELVIS, got ${diagnostics.map { it.factoryName }}",
        )
    }

    @Test
    fun castNeverSucceedsIsRecorded() {
        val path = writeKt(
            "Cast.kt",
            """
            package com.acme.cast

            fun caller(input: String): Int {
                return (input as Int)
            }
            """.trimIndent(),
        )

        val diagnostics = diagnosticsFor(path)
        assertTrue(
            diagnostics.any { it.factoryName == "CAST_NEVER_SUCCEEDS" },
            "expected CAST_NEVER_SUCCEEDS, got ${diagnostics.map { it.factoryName }}",
        )
    }

    @Test
    fun unreachableCodeIsRecorded() {
        // K2's UNREACHABLE_CODE checker lives in the experimental
        // checker set. KritFirCheckers registers it with the plugin's
        // control-flow checkers so the projection collects it on the
        // default analyze path.
        val path = writeKt(
            "Unreachable.kt",
            """
            package com.acme.unreachable

            fun caller(): Int {
                return 1
                return 2
            }
            """.trimIndent(),
        )

        val diagnostics = diagnosticsFor(path)
        assertTrue(
            diagnostics.any { it.factoryName == "UNREACHABLE_CODE" },
            "expected UNREACHABLE_CODE, got ${diagnostics.map { it.factoryName }}",
        )
        // Belt-and-suspenders: ensure factoryName ↔ message pairing
        // isn't crossed (a prefix-match miss would otherwise label
        // an unrelated warning with the wrong factory).
        diagnostics.filter { it.factoryName == "UNREACHABLE_CODE" }.forEach {
            assertTrue("Unreachable" in it.message, "factoryName/message mismatch: $it")
        }
    }

    @Test
    fun cleanFileEmitsNoDiagnostics() {
        val path = writeKt(
            "Clean.kt",
            """
            package com.acme.clean

            fun greet(name: String): String = "hi, " + name
            """.trimIndent(),
        )

        val diagnostics = diagnosticsFor(path)
        assertEquals(emptyList(), diagnostics)
    }

    @Test
    fun recordedDiagnosticCarriesLineColAndByteRange() {
        val path = writeKt(
            "Position.kt",
            """
            package com.acme.position

            fun caller(): String {
                val s: String = "hi"
                return s ?: "fallback"
            }
            """.trimIndent(),
        )

        val diagnostic = diagnosticsFor(path).firstOrNull { it.factoryName == "USELESS_ELVIS" }
        assertNotNull(diagnostic)
        assertEquals(5, diagnostic.line, "elvis is on line 5: $diagnostic")
        assertTrue(diagnostic.col > 0, "col must be 1-based: $diagnostic")
        assertTrue(
            diagnostic.endByte >= diagnostic.startByte,
            "byte range must be non-decreasing: $diagnostic",
        )
        assertEquals("WARNING", diagnostic.severity)
    }

    @Test
    fun deprecationIsRecordedForEveryReferenceKindAndOnlyTheDeprecatedOverload() {
        writeKt(
            "Lib.kt",
            """
            package com.acme.dep

            @Deprecated("use newFn") fun oldFn() {}
            fun overloaded(i: Int) {}
            @Deprecated("use the Int overload") fun overloaded(s: String) {}
            @Deprecated("use NewType") class OldType
            class Holder {
                @Deprecated("use newProp") val oldProp: Int = 1
                val newProp: Int = 2
            }
            open class Base { @Deprecated("gone") open fun m() {} }
            class Child : Base()
            @Deprecated("use String") typealias OldAlias = String
            """.trimIndent(),
        )
        // Deliberately free of the tokens the old lexical gate keyed on.
        val use = writeKt(
            "Use.kt",
            """
            package com.acme.dep

            import java.util.Date

            fun useAll(h: Holder, c: Child, t: OldType, a: OldAlias) {
                oldFn()
                overloaded(1)
                overloaded("s")
                println(h.oldProp)
                println(h.newProp)
                c.m()
                println(Date().year)
            }
            """.trimIndent(),
        )

        val deprecations = diagnosticsFor(use).filter { it.factoryName == "DEPRECATION" }
        // Line 5: the OldType and OldAlias parameter types. Then oldFn (6), the
        // String overload (8), oldProp (9), the inherited m (11), and the
        // Java-deprecated Date.year getter (12). Not overloaded(1) (7) or
        // newProp (10).
        assertEquals(
            listOf(5, 5, 6, 8, 9, 11, 12),
            deprecations.map { it.line }.sorted(),
            "got ${deprecations.map { "${it.line}:${it.col} ${it.message}" }}",
        )
        assertTrue(deprecations.all { it.severity == "WARNING" })
    }

    @Test
    fun deprecationMessageContainingBracketedTextIsStillRecorded() {
        // The @Deprecated message is embedded in the compiler message; an
        // unanchored "[NAME]" check would mistake it for a krit plugin
        // diagnostic and drop it.
        val path = writeKt(
            "Bracketed.kt",
            """
            package com.acme.bracketed

            @Deprecated("[OLD] use g") fun f() {}
            fun g() {}
            fun caller() { f() }
            """.trimIndent(),
        )

        val deprecations = diagnosticsFor(path).filter { it.factoryName == "DEPRECATION" }
        assertEquals(listOf(5), deprecations.map { it.line }, "got $deprecations")
    }

    @Test
    fun otherDeprecationWordedWarningsAreNotRecordedAsDeprecation() {
        // DEPRECATED_IDENTITY_EQUALS renders "Identity equality … is
        // deprecated." — a different factory the DEPRECATION pattern must not
        // claim.
        val path = writeKt(
            "Identity.kt",
            """
            package com.acme.identity

            fun same(a: Int, b: Int) = a === b
            """.trimIndent(),
        )

        assertEquals(emptyList(), diagnosticsFor(path).filter { it.factoryName == "DEPRECATION" })
    }

    @Test
    fun smartCastOnAnotherFilesPropertyNamesTheDeclaringFile() {
        val model = writeKt(
            "lib/Model.kt",
            """
            package com.acme.lib

            class Profile(val name: String)
            open class Account(val profile: Profile?)
            """.trimIndent(),
        )
        val use = writeKt(
            "app/Use.kt",
            """
            package com.acme.app

            import com.acme.lib.Account
            import com.acme.lib.Profile

            class Admin : Account(null)
            class Holder(val account: Account, val own: Profile?)

            fun safeCall(a: Account): String? = if (a.profile != null) a.profile?.name else null
            fun assertion(a: Account): String { checkNotNull(a.profile); return a.profile!!.name }
            fun elvis(a: Account): Profile? = if (a.profile != null) a.profile ?: Profile("x") else null
            fun comparison(a: Account): Boolean = a.profile != null && a.profile != null
            fun inherited(a: Admin): String? = if (a.profile != null) (a.profile)?.name else null
            fun chain(h: Holder): String? = if (h.account.profile != null) h.account.profile?.name else null
            fun sameFile(h: Holder): String? = if (h.own != null) h.own?.name else null
            fun local(p: Profile?): String? = if (p != null) p?.name else null
            fun cast(a: Account): Profile? = if (a.profile != null) a.profile as Profile else null
            """.trimIndent(),
        )

        val byLine = diagnosticsFor(use)
            .filter { it.factoryName in setOf("UNNECESSARY_SAFE_CALL", "UNNECESSARY_NOT_NULL_ASSERTION", "USELESS_ELVIS", "SENSELESS_COMPARISON", "USELESS_CAST") }
            .groupBy { it.line }
        val modelFile = File(model).canonicalPath
        fun declarations(line: Int) = byLine[line].orEmpty().also { assertTrue(it.isNotEmpty(), "no diagnostic on line $line: $byLine") }
            .map { d -> d.smartCastDeclarations.map { File(it).canonicalPath } }
        for (line in (9..14) + 17) {
            assertEquals(listOf(listOf(modelFile)), declarations(line), "line $line")
        }
        // A property of the diagnosed file itself and a local need no other module's stability.
        assertEquals(listOf(emptyList()), declarations(15))
        assertEquals(listOf(emptyList()), declarations(16))
    }

    @Test
    fun contractSmartCastOnAnotherFilesPropertyNamesTheDeclaringFile() {
        val model = writeKt(
            "lib/Model.kt",
            """
            package com.acme.lib

            class Profile(val name: String)
            class Account(val profile: Profile?)
            val topLevel: Profile? = null
            """.trimIndent(),
        )
        val use = writeKt(
            "app/Use.kt",
            """
            package com.acme.app

            import com.acme.lib.Account
            import com.acme.lib.topLevel
            import kotlin.contracts.ExperimentalContracts
            import kotlin.contracts.contract

            @OptIn(ExperimentalContracts::class)
            fun <T : Any> assertNotNull(actual: T?, message: String? = null): T {
                contract { returns() implies (actual != null) }
                return actual ?: error(message.orEmpty())
            }

            fun viaContract(a: Account): String {
                assertNotNull(a.profile)
                return a.profile!!.name
            }

            fun viaTopLevel(): String {
                assertNotNull(topLevel)
                return topLevel!!.name
            }
            """.trimIndent(),
        )

        val byLine = diagnosticsFor(use).filter { it.factoryName == "UNNECESSARY_NOT_NULL_ASSERTION" }.groupBy { it.line }
        val modelFile = File(model).canonicalPath
        for (line in listOf(16, 21)) {
            val found = byLine[line].orEmpty()
            assertTrue(found.isNotEmpty(), "no diagnostic on line $line: $byLine")
            assertEquals(listOf(listOf(modelFile)), found.map { d -> d.smartCastDeclarations.map { File(it).canonicalPath } }, "line $line")
        }
    }

    @Test
    fun localInitializedFromSmartCastNamesTheDeclaringFile() {
        val model = writeKt(
            "lib/Model.kt",
            """
            package com.acme.lib

            class Profile(val name: String?)
            class Account(val profile: Profile?)
            """.trimIndent(),
        )
        val use = writeKt(
            "app/Use.kt",
            """
            package com.acme.app

            import com.acme.lib.Account
            import com.acme.lib.Profile

            fun viaLocal(a: Account): String? {
                if (a.profile != null) {
                    val p = a.profile
                    return p?.name
                }
                return null
            }

            fun explicitType(a: Account): String? {
                if (a.profile != null) {
                    val p: Profile? = a.profile
                    return p?.name
                }
                return null
            }
            """.trimIndent(),
        )

        val byLine = diagnosticsFor(use).filter { it.factoryName == "UNNECESSARY_SAFE_CALL" }.groupBy { it.line }
        // The inferred local is non-null only through the smart cast of another file's property.
        assertEquals(listOf(listOf(File(model).canonicalPath)), byLine[9].orEmpty().map { d -> d.smartCastDeclarations.map { File(it).canonicalPath } })
        // An explicit nullable type keeps the safe call necessary, so there is no verdict at all.
        assertEquals(emptyList(), byLine[17].orEmpty())
    }

    @Test
    fun smartCastTypeOfAnotherFilesPropertyNamesTheDeclaringFile() {
        val model = writeKt(
            "lib/Model.kt",
            """
            package com.acme.lib

            class Profile(val name: String)
            class Account(val profile: Profile?)
            """.trimIndent(),
        )
        val use = writeKt(
            "app/Use.kt",
            """
            package com.acme.app

            import com.acme.lib.Account
            import com.acme.lib.Profile

            fun take(p: Profile): String = p.name
            fun other(a: Account): String = if (a.profile != null) take(a.profile) else ""
            fun local(p: Profile?): String = if (p != null) take(p) else ""
            """.trimIndent(),
        )

        val expressions = AnalysisSession(
            sourceDirs = listOf(tmp.toFile().absolutePath),
            classpath = stdlibClasspath,
        ).analyze(emptyList()).files[use]?.expressions.orEmpty()
        val smartCasts = expressions.values.filter { it.type == "com.acme.lib.Profile" && !it.nullable }
        val tagged = smartCasts.filter { it.smartCastDeclarations.isNotEmpty() }
        assertEquals(listOf(listOf(File(model).canonicalPath)), tagged.map { e -> e.smartCastDeclarations.map { File(it).canonicalPath } })
        // The local's smart cast relies on no other file.
        assertTrue(smartCasts.size > tagged.size, "expected an untagged local smart cast: $smartCasts")
    }

    @Test
    fun safeCallOnErrorTypedReceiverIsNotRecorded() {
        val use = writeKt(
            "app/Use.kt",
            """
            package com.acme.app

            import com.acme.missing.Wrapper
            import com.acme.missing.Node

            class Row(val wrapper: Wrapper) {
                val node = wrapper.inner
                val label: String? = node.name
            }

            fun viaParam(w: Wrapper): Int? = w.inner.parent?.size
            fun viaInferred(r: Row): Int? = r.node.parent?.size
            fun viaDeclared(n: Node): Int? = n.parent?.size
            fun viaStdlibExtension(w: Wrapper): Int? = w.inner.root?.name?.length
            fun viaChain(w: Wrapper): Int? = w.inner.root.parent?.name?.length
            fun viaLocal(w: Wrapper): Int? {
                val n = w.inner
                return n?.size
            }
            fun genuine(s: String): Int? = s?.length
            """.trimIndent(),
        )
        val diagnostics = diagnosticsFor(use).filter { it.factoryName == "UNNECESSARY_SAFE_CALL" }
        // Only the receiver that is genuinely non-null keeps its verdict.
        assertEquals(listOf(20), diagnostics.map { it.line })
    }

    private fun diagnosticsFor(path: String): List<DiagnosticPayload> {
        val result = AnalysisSession(
            sourceDirs = listOf(tmp.toFile().absolutePath),
            classpath = stdlibClasspath,
        ).analyze(emptyList())
        return result.files[path]?.diagnostics.orEmpty()
    }

    private fun writeKt(name: String, source: String): String {
        val file = tmp.resolve(name).toFile()
        file.parentFile.mkdirs()
        file.writeText(source)
        return file.absolutePath
    }

    private fun findKotlinStdlib(): String? {
        System.getenv("KOTLIN_STDLIB_JAR")?.let { override ->
            if (File(override).isFile) return override
        }
        val home = System.getProperty("user.home") ?: return null
        val cacheRoot = File(home, ".gradle/caches/modules-2/files-2.1/org.jetbrains.kotlin/kotlin-stdlib")
        if (!cacheRoot.isDirectory) return null
        val matches = cacheRoot.walkTopDown()
            .filter { it.isFile && it.name.startsWith("kotlin-stdlib-") && it.name.endsWith(".jar") }
            .filter { !it.name.contains("sources") && !it.name.contains("javadoc") }
            .toList()
            .sortedByDescending { it.name }
        return matches.firstOrNull()?.absolutePath
    }
}
