package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.jetbrains.kotlin.cli.common.ExitCode
import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.cli.common.messages.MessageCollector
import org.jetbrains.kotlin.cli.jvm.K2JVMCompiler
import org.jetbrains.kotlin.config.Services
import org.junit.jupiter.api.Test
import java.io.File
import kotlin.test.assertEquals

// WorldWriteableFiles cases a single-file golden cannot express: per-line
// finding counts (the golden test compares lines, not counts), project
// declarations named like the constant that live in another file and package,
// and properties from a binary dependency.
class WorldWriteableFilesTest {

    private val rule = FirRuleCompileContext(enabledRuleIds = setOf("WorldWriteableFiles"))

    // file -> line -> count
    private fun findings(sources: Map<String, String>): Map<String, Map<Int, Int>> =
        KritFirProbe.diagnose(sources, rule)
            .filter { it.name == "WorldWriteableFiles" }
            .groupBy { it.file }
            .mapValues { (_, diags) -> diags.groupingBy { it.line }.eachCount() }

    // Every WorldWriteableFiles golden, counted: each line reports exactly as
    // many findings as it has markers. The golden test compares only the set
    // of reported lines, while Go (the `// go-lines:` header) counts each
    // identifier, so a name reported twice (an assigned name that is also
    // read, a parameter's name and its default) or once where Go reports
    // twice would otherwise go unnoticed.
    @Test
    fun goldenFindingCountsMatchTheirMarkers() {
        val marker = Regex("""<!WorldWriteableFiles!>(.*?)<!>""")
        val goldens = File("src/test/data/diagnostic/androidlint")
            .listFiles { file -> file.name.startsWith("WorldWriteableFiles") && file.extension == "kt" }
            .orEmpty()
            .sortedBy { it.name }
        check(goldens.isNotEmpty()) { "no WorldWriteableFiles goldens found" }
        for (golden in goldens) {
            val lines = golden.readLines()
            val expected = lines.withIndex()
                .associate { (index, line) -> index + 1 to marker.findAll(line).count() }
                .filterValues { it > 0 }
            val source = lines.joinToString("\n") { marker.replace(it, "$1") }
            val actual = findings(mapOf(golden.name to source))[golden.name].orEmpty()
            assertEquals(expected, actual, "per-line finding counts in ${golden.name}")
        }
    }

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

    // A property from a binary dependency: a non-const property has no
    // initializer FIR can read, so its value is unknown and each read (and
    // import) is reported, as Go reports it by name. A binary const keeps its
    // value, so one that is not world-writeable is dropped, like a project
    // const (the lookalike divergence pinned in WorldWriteableFilesLookalike.kt).
    @Test
    fun binaryDependencyProperties() {
        val library = """
            package lib

            object BinaryModes {
                val MODE_WORLD_WRITEABLE: Int = compute()
                const val MODE_WORLD_WRITABLE = 0

                private fun compute(): Int = 2
            }
        """.trimIndent()
        val use = """
            package use

            import android.content.Context
            import lib.BinaryModes
            import lib.BinaryModes.MODE_WORLD_WRITEABLE
            import java.io.FileOutputStream

            fun unknown(context: Context): FileOutputStream = context.openFileOutput("a.txt", BinaryModes.MODE_WORLD_WRITEABLE)

            fun private(context: Context): FileOutputStream = context.openFileOutput("b.txt", BinaryModes.MODE_WORLD_WRITABLE)
        """.trimIndent()
        val libDir = kotlin.io.path.createTempDirectory("krit-fir-binary-lib").toFile()
        try {
            val srcDir = libDir.resolve("src").apply { mkdirs() }
            srcDir.resolve("BinaryModes.kt").writeText(library)
            val outDir = libDir.resolve("out").apply { mkdirs() }
            val stdlib = System.getProperty("kotlin.stdlib.jar")
            val exit = K2JVMCompiler().exec(
                MessageCollector.NONE,
                Services.EMPTY,
                K2JVMCompilerArguments().apply {
                    freeArgs = listOf(srcDir.absolutePath)
                    destination = outDir.absolutePath
                    noStdlib = true
                    noReflect = true
                    if (stdlib != null) classpath = stdlib
                },
            )
            assertEquals(ExitCode.OK, exit, "library compile")
            val result = KritFirProbe.compile(mapOf("Use.kt" to use), rule) { args ->
                args.classpath = listOfNotNull(args.classpath, outDir.absolutePath).joinToString(File.pathSeparator)
            }
            check(result.clean) { result.problems() }
            val counts = result.diags.filter { it.name == "WorldWriteableFiles" }
                .groupBy { it.file }
                .mapValues { (_, diags) -> diags.groupingBy { it.line }.eachCount() }
            assertEquals(mapOf("Use.kt" to mapOf(5 to 1, 8 to 1)), counts)
        } finally {
            libDir.deleteRecursively()
        }
    }
}
