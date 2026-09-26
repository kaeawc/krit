package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// RuntimeExecUnsafeShape cases a single-file golden cannot express: a
// project class named Runtime brought in by a star import.
class RuntimeExecUnsafeShapeCrossFileTest {

    private fun findings(sources: Map<String, String>): List<Pair<String, Int>> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("RuntimeExecUnsafeShape")),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "RuntimeExecUnsafeShape" }.map { it.file to it.line }
    }

    // Divergence (precision): the star import `com.example.proc.*` takes
    // priority over the default `java.lang.*` import, so `Runtime` on lines 6
    // and 7 is com.example.proc.Runtime and exec is its member exec(String).
    // Go resolves the name `Runtime` to java.lang.Runtime and reports both
    // lines (interpolated, computed). FIR drops them: the message
    // ("Runtime.exec(String) uses ...") is false, the call is not
    // java.lang.Runtime.exec. The qualified java.lang call on line 8 still
    // reports.
    @Test fun starImportedRuntimeLookalike() {
        val sources = mapOf(
            "Runtime.kt" to """
                package com.example.proc

                class Runtime {
                    fun exec(command: String) {}

                    companion object {
                        fun getRuntime(): Runtime = Runtime()
                    }
                }
            """.trimIndent(),
            "User.kt" to """
                package demo

                import com.example.proc.*

                fun run(userPath: String) {
                    Runtime.getRuntime().exec("ls ${'$'}userPath")
                    Runtime.getRuntime().exec("ls " + userPath)
                    java.lang.Runtime.getRuntime().exec("ls " + userPath)
                }
            """.trimIndent(),
        )
        assertEquals(listOf("User.kt" to 8), findings(sources))
    }
}
