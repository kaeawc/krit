package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

// WorldWriteableFiles cases a single-file golden cannot express: per-line
// finding counts (golden markers record lines, not counts) and project
// declarations named like the constant that live in another file and package.
class WorldWriteableFilesTest {

    private val rule = FirRuleCompileContext(enabledRuleIds = setOf("WorldWriteableFiles"))

    // file -> line -> count
    private fun findings(sources: Map<String, String>): Map<String, Map<Int, Int>> =
        KritFirProbe.diagnose(sources, rule)
            .filter { it.name == "WorldWriteableFiles" }
            .groupBy { it.file }
            .mapValues { (_, diags) -> diags.groupingBy { it.line }.eachCount() }

    // Go reports each identifier, so two reads on one line are two findings.
    @Test
    fun twoReadsOnOneLineAreTwoFindings() {
        val source = """
            package count

            import android.content.Context
            import android.content.Context.MODE_WORLD_WRITEABLE

            fun both(): Int = Context.MODE_WORLD_WRITEABLE or MODE_WORLD_WRITEABLE
        """.trimIndent()
        assertEquals(mapOf("Count.kt" to mapOf(4 to 1, 6 to 2)), findings(mapOf("Count.kt" to source)))
    }

    // A world-writeable project alias imported from another package is
    // reported on the import and the read, as Go reports both by name; a
    // same-named project constant that is not world-writeable is reported on
    // neither (Go reports both lines of it too, a lookalike divergence pinned
    // in WorldWriteableFilesLookalike.kt).
    @Test
    fun projectDeclarationsInAnotherFile() {
        val modes = """
            package com.example.modes

            import android.content.Context

            object SharedModes {
                const val MODE_WORLD_WRITABLE = Context.MODE_WORLD_WRITEABLE
            }

            object PrivateModes {
                const val MODE_WORLD_WRITEABLE = 0
            }
        """.trimIndent()
        val shared = """
            package use.shared

            import android.content.Context
            import com.example.modes.SharedModes.MODE_WORLD_WRITABLE
            import java.io.FileOutputStream

            fun open(context: Context): FileOutputStream = context.openFileOutput("a.txt", MODE_WORLD_WRITABLE)
        """.trimIndent()
        val closed = """
            package use.closed

            import android.content.Context
            import com.example.modes.PrivateModes.MODE_WORLD_WRITEABLE
            import java.io.FileOutputStream

            fun open(context: Context): FileOutputStream = context.openFileOutput("b.txt", MODE_WORLD_WRITEABLE)
        """.trimIndent()
        val result = findings(mapOf("Modes.kt" to modes, "Shared.kt" to shared, "Private.kt" to closed))
        assertEquals(
            mapOf("Modes.kt" to mapOf(6 to 1), "Shared.kt" to mapOf(4 to 1, 7 to 1)),
            result,
        )
    }
}
