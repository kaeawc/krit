package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.CheckRequest
import dev.jasonpearson.krit.fir.incremental.testStdlib
import dev.jasonpearson.krit.fir.runModuleFacts
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * Modules of a `--module-facts-file` request compile on several threads of one JVM (#1656) and give what one
 * thread gives: the same facts and the same check for every module, a broken module included.
 */
class ModuleFactsThreadsTest {
    @TempDir lateinit var tmp: Path

    private fun module(name: String, code: String): Pair<ModuleSpec, String> {
        val root = tmp.resolve("$name/src/main/kotlin").toFile().apply { mkdirs() }
        val file = root.resolve("$name.kt").apply { writeText(code) }
        return ModuleSpec(id = ":$name:main", sourceRoots = listOf(root.path), classpath = listOf(testStdlib)) to file.absolutePath
    }

    private fun run(modules: List<ModuleSpec>, files: List<String>, threads: Int): Map<String, String> {
        val out = tmp.resolve("facts-$threads").toFile().apply { mkdirs() }
        val check = AnalysisSession.ModuleCheckIndex(
            CheckRequest(id = 1, command = "check", files = files.map { FileRef(it) }, rules = listOf("PrintlnInProduction")),
        )
        runModuleFacts(modules, out, "", check, threads)
        // Response timings differ from run to run; everything else must not.
        return out.listFiles()!!.associate { it.name to it.readText().replace(Regex("\"totalMs\":\\d+"), "") }
    }

    @Test fun threadedModulesGiveWhatOneThreadGives() {
        val specs = (1..6).map { i ->
            if (i == 4) module("m$i", "package m$i\n\nfun broken(): Int = missing$i + 1\n")
            else module("m$i", "package m$i\n\nfun f$i() { println(\"m$i\") }\nclass C$i(val v: Int)\n")
        }
        val modules = specs.map { it.first }
        val files = specs.map { it.second }
        val one = run(modules, files, threads = 1)
        val three = run(modules, files, threads = 3)
        assertEquals(12, one.size, "a facts file and a check file per module: ${one.keys}")
        assertEquals(one, three)
        assertTrue(one.getValue("_m4_main.check.json").contains("missing4"), "the broken module is reported: ${one["_m4_main.check.json"]}")
        assertTrue(one.getValue("_m1_main.check.json").contains("PrintlnInProduction"), one["_m1_main.check.json"])
    }
}
