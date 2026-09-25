package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// JavaObjectInputStream cases a single-file golden cannot express: a class
// named ObjectInputStream declared in another file of the same package.
class JavaObjectInputStreamCrossFileTest {

    private fun findings(sources: Map<String, String>): List<Pair<String, Int>> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("JavaObjectInputStream")),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "JavaObjectInputStream" }.map { it.file to it.line }
    }

    // Go reports the bare call: the java.io star import satisfies its file
    // gate and the call is spelled ObjectInputStream. FIR is correct to drop
    // it, because the same-package demo.ObjectInputStream wins over the star
    // import, so the call does not construct a java.io.ObjectInputStream. The
    // fully qualified call in the same file still does.
    @Test fun samePackageClassWinsOverStarImport() {
        val sources = mapOf(
            "Shadow.kt" to """
                package demo

                class ObjectInputStream(val s: String)
            """.trimIndent(),
            "StarUser.kt" to """
                package demo

                import java.io.*

                fun open(): Any = ObjectInputStream("x")

                fun jdk(input: InputStream): Any = java.io.ObjectInputStream(input)
            """.trimIndent(),
        )
        assertEquals(listOf("StarUser.kt" to 7), findings(sources))
    }
}
