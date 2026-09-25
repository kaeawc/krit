package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// PrintlnInProduction cases that need more than one source file or a source
// path: cross-file shadowing of the built-in, and the Go rule's path
// exemptions (test files, sample and demo directories).
class PrintlnInProductionFilesTest {

    private fun findings(
        sources: Map<String, String>,
        testFiles: Set<String> = emptySet(),
        scanPaths: Map<String, String> = emptyMap(),
    ): Map<String, Int> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("PrintlnInProduction")),
            testFiles = testFiles,
            scanPaths = scanPaths,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "PrintlnInProduction" }.groupingBy { it.file }.eachCount()
    }

    private fun printer(pkg: String) = """
        package $pkg

        class Service {
            fun load() {
                println("loading")
            }
        }
    """.trimIndent()

    // Go reports each of these calls: the calling file declares and imports
    // nothing named println, so Go takes the call for the built-in. Each
    // resolves to a user function in another file and prints nothing to the
    // console, so FIR does not report it.
    @Test fun printlnDeclaredInAnotherFileIsNotTheBuiltIn() {
        val sources = mapOf(
            "Declarations.kt" to """
                package shadow

                fun println(label: String): String = label
            """.trimIndent(),
            "SamePackage.kt" to """
                package shadow

                fun samePackage(): String = println("same-package declaration")
            """.trimIndent(),
            "StarImport.kt" to """
                package other

                import shadow.*

                fun starImported(): String = println("star-imported declaration")
            """.trimIndent(),
            "Base.kt" to """
                package base

                open class Base {
                    fun println(label: String): String = label
                }
            """.trimIndent(),
            "Inherited.kt" to """
                package inherited

                import base.Base

                class Child : Base() {
                    fun emit(): String = println("inherited member")
                }
            """.trimIndent(),
        )
        assertEquals(emptyMap(), findings(sources))
    }

    // Go reports this call: the calling file declares and imports nothing
    // named println. Inside `with(log)` it resolves to the Log member declared
    // in another file, a user function that prints nothing, so FIR does not
    // report it. (Declared in the same file, Go would skip it too.)
    @Test fun printlnMemberOfWithReceiverDeclaredInAnotherFile() {
        val sources = mapOf(
            "Log.kt" to """
                package logging

                interface Log {
                    fun println(s: String)
                }
            """.trimIndent(),
            "WithReceiver.kt" to """
                package caller

                import logging.Log

                fun emit(log: Log) {
                    with(log) {
                        println("x")
                    }
                }
            """.trimIndent(),
        )
        assertEquals(emptyMap(), findings(sources))
    }

    // Go skips every bare println in a file that imports a user println. The
    // imported function takes an Int, so println("x") falls through to
    // kotlin.io.println and is console output, which FIR reports. The Int
    // call resolves to the import and is not reported by either.
    @Test fun importedPrintlnThatDoesNotFitFallsThroughToTheBuiltIn() {
        val sources = mapOf(
            "Counter.kt" to """
                package counter

                fun println(value: Int): Int = value
            """.trimIndent(),
            "Imported.kt" to """
                package caller

                import counter.println

                fun emit(): Int {
                    println("x")
                    return println(1)
                }
            """.trimIndent(),
        )
        assertEquals(mapOf("Imported.kt" to 1), findings(sources))
    }

    @Test fun sampleAndDemoDirectoriesAreSkipped() {
        // Findings are keyed by file name, so every file name is distinct.
        val sources = mapOf(
            "app/src/main/kotlin/AppService.kt" to printer("app"),
            "samples/SamplesService.kt" to printer("samples"),
            "features/sample/SampleService.kt" to printer("sample"),
            "Demos/DemosService.kt" to printer("demos"),
            "features/demo/src/main/kotlin/DemoService.kt" to printer("demo"),
            // Only a whole directory name counts.
            "demonstration/DemonstrationService.kt" to printer("demonstration"),
        )
        assertEquals(
            mapOf("AppService.kt" to 1, "DemonstrationService.kt" to 1),
            findings(sources),
        )
    }

    // Go tests the scan's own spelling of the path, which the request carries
    // (FirRule.scanPath). `krit samples/proj` scans `samples/proj/src/X.kt`:
    // no `/samples/` in it, so Go reports the file, even though its absolute
    // path holds `/samples/`; a `/sample/` below the scan root still counts.
    // When Go scanned the absolute path itself (no scan spelling in the
    // request), the checkout's `/samples/` directory counts, as it does in Go.
    @Test fun markersAreMatchedOnTheScanSpelling() {
        val sources = mapOf(
            "samples/proj/src/X.kt" to printer("x"),
            "samples/proj/app/sample/Y.kt" to printer("y"),
        )
        val relative = mapOf(
            "samples/proj/src/X.kt" to "samples/proj/src/X.kt",
            "samples/proj/app/sample/Y.kt" to "samples/proj/app/sample/Y.kt",
        )
        assertEquals(mapOf("X.kt" to 1), findings(sources, scanPaths = relative))
        assertEquals(emptyMap(), findings(sources))
    }

    @Test fun requestListedTestFileIsSkipped() {
        val sources = mapOf("Listed.kt" to printer("listed"), "Unlisted.kt" to printer("unlisted"))
        assertEquals(mapOf("Unlisted.kt" to 1), findings(sources, testFiles = setOf("Listed.kt")))
    }
}
