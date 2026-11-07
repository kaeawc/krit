package dev.jasonpearson.krit.fir.runner

import org.junit.jupiter.api.Assumptions.assumeTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals

/**
 * `kotlin.test.assertNotNull` has a contract that smart casts its argument; JUnit's does not. A helper of the same
 * name imported from a module whose build output is missing, or inherited from a missing base class, leaves the name
 * to `kotlin.test`, so the compiler's redundant `!!` and `?.` verdicts after it describe its guess.
 */
class AssertNotNullContractDiagnosticsTest {

    @TempDir
    lateinit var tmp: Path

    private val models = """
        class Res(val name: String?)
        class Box(val items: List<String>?)
    """.trimIndent()

    private val uses = """
        fun bang(res: Res?) { val prefix = res?.let { it.name }; assertNotNull(prefix); println(prefix!!) }
        fun safe(b: Box?) { assertNotNull(b); println(b?.items?.size) }
    """.trimIndent()

    private fun file(pkg: String, header: String, body: String = uses) = "package $pkg\n$header\n$models\n$body"

    @Test
    fun contractVerdictsFollowTheFunctionThatResolves() {
        val sources = mapOf(
            // kotlin.test resolves: its contract makes the `!!` and `?.` redundant.
            "KotlinTest" to file("kt", "import kotlin.test.assertNotNull"),
            "KotlinTestStar" to file("kts", "import kotlin.test.*"),
            // JUnit resolves: no contract, nothing is redundant.
            "Junit" to file("ju", "import org.junit.Assert.assertNotNull\nimport kotlin.test.*"),
            "JunitOnly" to file("juo", "import org.junit.Assert.*"),
            // The helper's module is missing: the name is not kotlin.test's to take.
            "MissingImport" to file("mi", "import com.acme.testing.assertNotNull\nimport kotlin.test.*"),
            "MissingAlias" to file("ma", "import com.acme.testing.verifyPresent as assertNotNull\nimport kotlin.test.*"),
            "MissingBase" to file(
                "mb", "import com.acme.missing.Base\nimport kotlin.test.*",
                "class T : Base() {\n" + uses.lines().joinToString("\n") { "    $it" } + "\n}",
            ),
        )
        val dir = tmp.toFile()
        val paths = sources.mapValues { (name, text) -> File(dir, "app/$name.kt").also { it.parentFile.mkdirs(); it.writeText(text) } }
        val classpath = listOfNotNull(jar("kotlin-stdlib-2", "kotlin-stdlib"), jar("kotlin-test-2", "kotlin-test"), jar("junit-4.13.2", "junit"))
        assumeTrue(classpath.size == 3, "kotlin-stdlib, kotlin-test and junit jars not in the Gradle cache")
        val result = AnalysisSession(sourceDirs = listOf(dir.absolutePath), classpath = classpath).analyze(emptyList())
        val reported = paths.mapValues { (_, f) ->
            result.files[f.absolutePath]?.diagnostics.orEmpty().map { it.factoryName }.filter { it.startsWith("UNNECESSARY") }.sorted()
        }
        val both = listOf("UNNECESSARY_NOT_NULL_ASSERTION", "UNNECESSARY_SAFE_CALL")
        assertEquals(
            mapOf(
                "KotlinTest" to both, "KotlinTestStar" to both, "Junit" to emptyList(), "JunitOnly" to emptyList(),
                "MissingImport" to emptyList(), "MissingAlias" to emptyList(), "MissingBase" to emptyList(),
            ),
            reported,
        )
    }

    /**
     * Truth's `isNotNull()` has no contract, so the first `!!` on a nullable property is required, while a second `!!`
     * on the same stable property is redundant because the first one smart casts it. A contract helper smart casts a
     * stable member of a local `var`, until the `var` is reassigned.
     */
    @Test
    fun redundantVerdictsStartAfterTheFirstProof() {
        val source = """
            package shapes
            import kotlin.contracts.ExperimentalContracts
            import kotlin.contracts.contract

            class Subject<T>(val actual: T?) { fun isNotNull() {}; fun isTrue() {} }
            fun <T> assertThat(actual: T?): Subject<T> = Subject(actual)

            @OptIn(ExperimentalContracts::class)
            fun <T : Any> assertNotNull(actual: T?, message: String? = null): T {
                contract { returns() implies (actual != null) }
                return actual!!
            }

            class Style(val bold: Boolean?, val italic: Boolean?)
            class Tag(val style: Style?)
            class State(val items: List<String>?)

            fun truth(tag: Tag) {
                assertThat(tag.style).isNotNull()
                assertThat(tag.style!!.bold).isTrue()
                assertThat(tag.style!!.italic).isTrue()
            }

            fun reassigned(next: () -> State) {
                var state = next()
                assertNotNull(state.items)
                println(state.items?.size)
                state = next()
                println(state.items?.size)
                assertNotNull(state.items)
                println(state.items?.size)
            }
        """.trimIndent()
        val dir = tmp.toFile()
        val file = File(dir, "shapes/Shapes.kt").also { it.parentFile.mkdirs(); it.writeText(source) }
        val classpath = listOfNotNull(jar("kotlin-stdlib-2", "kotlin-stdlib"))
        assumeTrue(classpath.isNotEmpty(), "kotlin-stdlib jar not in the Gradle cache")
        val result = AnalysisSession(sourceDirs = listOf(dir.absolutePath), classpath = classpath).analyze(emptyList())
        val lines = source.lines()
        fun lineOf(text: String, nth: Int = 0) = lines.withIndex().filter { text in it.value }[nth].index + 1
        val reported = result.files[file.absolutePath]?.diagnostics.orEmpty()
            .filter { it.factoryName.startsWith("UNNECESSARY") }.map { it.factoryName to it.line }.sortedBy { it.second }
        assertEquals(
            listOf(
                "UNNECESSARY_NOT_NULL_ASSERTION" to lineOf("tag.style!!.italic"),
                "UNNECESSARY_SAFE_CALL" to lineOf("println(state.items?.size)", 0),
                "UNNECESSARY_SAFE_CALL" to lineOf("println(state.items?.size)", 2),
            ),
            reported,
        )
    }

    private fun jar(prefix: String, artifact: String): String? {
        val root = File(System.getProperty("user.home") ?: return null, ".gradle/caches/modules-2/files-2.1")
        return root.walkTopDown().maxDepth(4).filter { it.isDirectory && it.name == artifact }
            .flatMap { it.walkTopDown() }
            .filter { it.isFile && it.name.startsWith(prefix) && it.name.endsWith(".jar") && !it.name.contains("sources") }
            .sortedByDescending { it.path }.firstOrNull()?.absolutePath
    }
}
