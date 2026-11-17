package dev.jasonpearson.krit.fir.tests

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

// HandlerLeak messages, which goldens do not compare. Lint 32.4.1 names an
// inner Handler class by its package-qualified name; Go builds the same name
// from the package and the enclosing class declarations.
class HandlerLeakTest {

    private fun messages(source: String): Map<Int, String> =
        KritFirProbe.diagnose(mapOf("Main.kt" to source))
            .filter { it.file == "Main.kt" && it.name == "HandlerLeak" }
            .associate { it.line to it.message.removePrefix("[HandlerLeak] ") }

    @Test
    fun innerClassMessageIsPackageQualified() {
        val source = """
            package hl.messages

            import android.os.Handler

            class Screen {
                inner class Leaky : Handler()

                class Nested {
                    inner class Deeper : Handler()
                }
            }

            fun host(): Any {
                class Local {
                    inner class InLocal : Handler()
                }
                return Local()
            }

            val anonymous = object : Handler() {}
        """.trimIndent()
        assertEquals(
            mapOf(
                6 to "This `Handler` class should be static or leaks might occur (hl.messages.Screen.Leaky)",
                9 to "This `Handler` class should be static or leaks might occur (hl.messages.Screen.Nested.Deeper)",
                15 to "This `Handler` class should be static or leaks might occur (hl.messages.Local.InLocal)",
                20 to "This `Handler` class should be static or leaks might occur (anonymous android.os.Handler)",
            ),
            messages(source),
        )
    }
}
