package dev.jasonpearson.krit.fir.runner

import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.junit.jupiter.api.Assumptions.assumeTrue
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * Kotlin Multiplatform JVM compilations: the source roots Go hands over
 * (non-JVM target sets already removed) must compile the way kotlinc
 * compiles a common + jvm project, not as one flat module in which every
 * `expect` collides with its `actual`.
 *
 * Expected diagnostics were measured with K2JVMCompiler directly
 * (`-Xmulti-platform -Xexpect-actual-classes -Xcommon-sources=...`).
 */
class AnalysisSessionMultiplatformTest {

    @TempDir
    lateinit var tmp: Path

    private lateinit var stdlibClasspath: List<String>

    @BeforeEach
    fun resolveStdlib() {
        val stdlib = findKotlinStdlib()
        assumeTrue(stdlib != null, "kotlin-stdlib jar not found; set KOTLIN_STDLIB_JAR or populate the Gradle cache")
        stdlibClasspath = listOfNotNull(stdlib)
    }

    // The KMP divergence reproduction. A flat compile loses lines 30 and 31
    // (the call through `expect fun platformLabel()` is ambiguous); kotlinc's
    // common + jvm compile reports all five.
    @Test
    fun commonAndJvmActualRetainExpectDependentWarnings() {
        val common = write("src/commonMain/kotlin/repro/Common.kt", COMMON_KT)
        write("src/jvmMain/kotlin/repro/Platform.kt", JVM_PLATFORM_KT)

        val result = session("commonMain", "jvmMain").analyze(emptyList())

        assertEquals(
            listOf(
                12 to "UNNECESSARY_SAFE_CALL",
                18 to "USELESS_ELVIS",
                25 to "UNNECESSARY_SAFE_CALL",
                30 to "UNNECESSARY_SAFE_CALL",
                31 to "USELESS_ELVIS",
            ),
            diagnostics(result.files[common]?.diagnostics),
        )
    }

    // An intermediate source set that declares an `expect` whose `actual` is
    // in jvmMain must be common: compiled as platform, kotlinc reports
    // "expect and corresponding actual are declared in the same module" and
    // every call through it is ambiguous.
    @Test
    fun intermediateSetDeclaringExpectIsCompiledAsCommon() {
        write("src/commonMain/kotlin/r/Common.kt", "package r\n\nfun commonValue(): Int = 1\n")
        val concurrent = write(
            "src/concurrentMain/kotlin/r/Concurrent.kt",
            """
            package r

            expect fun lockLabel(): String

            fun concurrentSafe(): Int = lockLabel()?.length ?: 0
            fun concurrentElvis(): String = lockLabel() ?: "none"
            """.trimIndent() + "\n",
        )
        val jvm = write(
            "src/jvmMain/kotlin/r/Jvm.kt",
            """
            package r

            import java.io.File

            actual fun lockLabel(): String = File(".").name

            fun jvmSafe(): Int = lockLabel()?.length ?: 0
            """.trimIndent() + "\n",
        )

        val result = session("commonMain", "concurrentMain", "jvmMain").analyze(emptyList())

        assertEquals(
            listOf(5 to "UNNECESSARY_SAFE_CALL", 6 to "USELESS_ELVIS"),
            diagnostics(result.files[concurrent]?.diagnostics),
        )
        assertEquals(listOf(7 to "UNNECESSARY_SAFE_CALL"), diagnostics(result.files[jvm]?.diagnostics))
    }

    // An intermediate source set that holds an `actual` must stay platform:
    // compiled as common, kotlinc puts the `actual` beside commonMain's
    // `expect` and breaks every caller, including commonMain's own.
    @Test
    fun intermediateSetDeclaringActualIsCompiledAsPlatform() {
        val common = write(
            "src/commonMain/kotlin/r/Common.kt",
            "package r\n\nexpect fun label(): String\n\nfun commonSafe(): Int = label()?.length ?: 0\n",
        )
        val shared = write(
            "src/jvmAndroidMain/kotlin/r/JvmAndroid.kt",
            """
            package r

            import java.io.File

            actual fun label(): String = File(".").name

            fun sharedSafe(): Int = label()?.length ?: 0
            """.trimIndent() + "\n",
        )
        val jvm = write("src/jvmMain/kotlin/r/Jvm.kt", "package r\n\nfun jvmSafe(): Int = label()?.length ?: 0\n")

        val result = session("commonMain", "jvmAndroidMain", "jvmMain").analyze(emptyList())

        assertEquals(listOf(5 to "UNNECESSARY_SAFE_CALL"), diagnostics(result.files[common]?.diagnostics))
        assertEquals(listOf(7 to "UNNECESSARY_SAFE_CALL"), diagnostics(result.files[shared]?.diagnostics))
        assertEquals(listOf(3 to "UNNECESSARY_SAFE_CALL"), diagnostics(result.files[jvm]?.diagnostics))
    }

    @Test
    fun commonSourcesFollowTheMeasuredClassification() {
        val common = write("src/commonMain/kotlin/r/Common.kt", "package r\nexpect fun a(): String\n")
        val commonTest = write("src/commonTest/kotlin/r/CommonTest.kt", "package r\nfun t() = expect(a())\nfun expect(s: String) = s\n")
        val expectOnly = write("src/concurrentMain/kotlin/r/C.kt", "package r\n@Suppress(\"x\") public expect class Lock()\n")
        val actualOnly = write("src/jvmAndroidMain/kotlin/r/J.kt", "package r\nactual fun a(): String = \"\"\n")
        val mixed1 = write("src/jvmCommonMain/kotlin/r/M1.kt", "package r\nexpect fun b(): String\n")
        val mixed2 = write("src/jvmCommonMain/kotlin/r/M2.kt", "package r\nactual class Lock actual constructor()\n")
        val jvm = write("src/jvmMain/kotlin/r/Jvm.kt", "package r\nactual fun b(): String = \"\"\n")
        // Words inside comments and strings are not modifiers.
        val commentOnly = write(
            "src/desktopMain/kotlin/r/D.kt",
            "package r\n// expect fun x()\n/* outer /* expect class Y */ still comment */\nval s = \"expect fun z()\"\nval r = \"\"\"\nexpect val q\n\"\"\"\n",
        )
        val main = write("src/main/kotlin/r/Main.kt", "package r\nexpect fun m(): String\n")
        val flat = write("other/kotlin/r/Flat.kt", "package r\nexpect fun f(): String\n")
        val all = listOf(common, commonTest, expectOnly, actualOnly, mixed1, mixed2, jvm, commentOnly, main, flat)

        val dirs = listOf(
            "src/commonMain", "src/commonTest", "src/concurrentMain", "src/jvmAndroidMain",
            "src/jvmCommonMain", "src/jvmMain", "src/desktopMain", "src/main", "other",
        ).map { tmp.resolve("$it/kotlin").toFile().path }

        assertEquals(listOf(common, commonTest, expectOnly), MultiplatformSources.commonSources(dirs, all))
    }

    // Plain JVM/Android projects have no commonMain/commonTest root: the
    // compiler arguments stay exactly as before the multiplatform support.
    @Test
    fun plainProjectKeepsFlatCompilerArguments() {
        val main = write("src/main/kotlin/p/Main.kt", "package p\n// expect fun looksLikeKmp()\nfun plain(): String { val s: String = \"ok\"; return s ?: \"x\" }\n")
        val concurrent = write("src/concurrentMain/kotlin/p/C.kt", "package p\nexpect fun c(): String\n")
        val dirs = listOf(main, concurrent).map { File(it).parentFile.parentFile.path }

        val args = K2JVMCompilerArguments()
        MultiplatformSources.configure(args, dirs, listOf(main, concurrent))
        val untouched = K2JVMCompilerArguments()
        assertFalse(args.multiPlatform)
        assertFalse(args.expectActualClasses)
        assertNull(args.commonSources)
        assertEquals(untouched.multiPlatform, args.multiPlatform)
        assertEquals(untouched.expectActualClasses, args.expectActualClasses)

        val plainOnly = AnalysisSession(listOf(File(main).parentFile.parentFile.path), stdlibClasspath)
        assertEquals(
            listOf(3 to "USELESS_ELVIS"),
            diagnostics(plainOnly.analyze(emptyList()).files[main]?.diagnostics),
        )
    }

    @Test
    fun commonSourcesUseTheFreeArgumentSpelling() {
        val common = write("src/commonMain/kotlin/r/Common.kt", "package r\n")
        val jvm = write("src/jvmMain/kotlin/r/Jvm.kt", "package r\n")
        val viaDotDot = tmp.toFile().path + "/src/jvmMain/../commonMain/kotlin/r/Common.kt"
        val dirs = listOf("src/commonMain/kotlin", "src/jvmMain/kotlin").map { tmp.resolve(it).toFile().path }

        assertEquals(listOf(viaDotDot), MultiplatformSources.commonSources(dirs, listOf(viaDotDot, jvm)))
        assertEquals(listOf(common), MultiplatformSources.commonSources(dirs, listOf(common, jvm)))
    }

    @Test
    fun sourceSetNameReadsTheGradleLayout() {
        assertEquals("commonMain", MultiplatformSources.sourceSetName("/p/src/commonMain/kotlin"))
        assertEquals("main", MultiplatformSources.sourceSetName("/p/src/main/java"))
        assertNull(MultiplatformSources.sourceSetName("/p/commonMain/kotlin"))
        assertTrue(MultiplatformSources.stripCommentsAndStrings("a // expect fun b\nc").startsWith("a \nc"))
    }

    private fun session(vararg sourceSets: String) =
        AnalysisSession(sourceSets.map { tmp.resolve("src/$it/kotlin").toFile().path }, stdlibClasspath)

    private fun diagnostics(list: List<dev.jasonpearson.krit.fir.oracle.DiagnosticPayload>?) =
        list.orEmpty().map { it.line to it.factoryName }.sortedBy { it.first }

    private fun write(relative: String, text: String): String {
        val file = tmp.resolve(relative).toFile()
        file.parentFile.mkdirs()
        file.writeText(text)
        // The session reports each walked file as its source root, as passed,
        // plus the relative path; tmp is already absolute.
        return file.path
    }

    private fun findKotlinStdlib(): String? {
        System.getenv("KOTLIN_STDLIB_JAR")?.let { override ->
            if (File(override).isFile) return override
        }
        val home = System.getProperty("user.home") ?: return null
        val cacheRoot = File(home, ".gradle/caches/modules-2/files-2.1/org.jetbrains.kotlin/kotlin-stdlib")
        if (!cacheRoot.isDirectory) return null
        return cacheRoot.walkTopDown()
            .filter { it.isFile && it.name.startsWith("kotlin-stdlib-") && it.name.endsWith(".jar") }
            .filter { !it.name.contains("sources") && !it.name.contains("javadoc") }
            .toList()
            .maxByOrNull { it.name }
            ?.absolutePath
    }

    private companion object {
        val COMMON_KT = """
            package repro

            expect fun platformLabel(): String

            expect class PlatformBox {
                fun text(): String
            }

            // Expected UNNECESSARY_SAFE_CALL: the receiver is declared non-null.
            fun commonSafeCall(): Int {
                val value: String = "ready"
                return value?.length ?: 0
            }

            // Expected USELESS_ELVIS: the left operand is declared non-null.
            fun commonElvis(): String {
                val value: String = "ready"
                return value ?: "fallback"
            }

            // Resolution of trim() depends on kotlin-stdlib being on the oracle classpath.
            // Expected UNNECESSARY_SAFE_CALL with stdlib present.
            fun commonStdlibCase(): Int {
                val trimmed = " ready ".trim()
                return trimmed?.length ?: 0
            }

            // These depend on resolution of the expect declaration. A correctly scoped
            // common+jvm compilation should report UNNECESSARY_SAFE_CALL and USELESS_ELVIS.
            fun expectResultSafeCall(): Int = platformLabel()?.length ?: 0
            fun expectResultElvis(): String = platformLabel() ?: "fallback"
        """.trimIndent() + "\n"

        val JVM_PLATFORM_KT = """
            package repro

            import java.io.File

            actual fun platformLabel(): String = File(".").absolutePath

            actual class PlatformBox {
                actual fun text(): String = File(".").name
            }
        """.trimIndent() + "\n"
    }
}
