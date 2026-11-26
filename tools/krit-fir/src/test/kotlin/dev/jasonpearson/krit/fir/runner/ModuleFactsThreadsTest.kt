package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.CheckRequest
import dev.jasonpearson.krit.fir.incremental.compileKotlinLibrary
import dev.jasonpearson.krit.fir.incremental.testStdlib
import dev.jasonpearson.krit.fir.runConcurrently
import dev.jasonpearson.krit.fir.runModuleFacts
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import java.util.Collections
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * Modules of a `--module-facts-file` request compile on several threads of one JVM (#1656) and give what one
 * thread gives: the same facts and the same check for every module, whatever its settings, plugins and friends.
 */
class ModuleFactsThreadsTest {
    @TempDir lateinit var tmp: Path

    private val serializationPlugin = File(requireNotNull(System.getProperty("krit.fir.test.serializationPlugin"))).absolutePath
    private val serializationRuntime = File(requireNotNull(System.getProperty("krit.fir.test.serializationRuntime"))).absolutePath

    private fun module(name: String, code: String, kind: String = "main"): Pair<ModuleSpec, String> {
        val root = tmp.resolve("$name/src/$kind/kotlin").toFile().apply { mkdirs() }
        val file = root.resolve("$name.kt").apply { writeText(code) }
        return ModuleSpec(id = ":$name:$kind", kind = kind, sourceRoots = listOf(root.path), classpath = listOf(testStdlib)) to
            file.absolutePath
    }

    private fun run(modules: List<ModuleSpec>, files: List<String>, label: String, threads: Int): Map<String, String> {
        val out = tmp.resolve("facts-$label").toFile().apply { mkdirs() }
        val check = AnalysisSession.ModuleCheckIndex(
            CheckRequest(id = 1, command = "check", files = files.map { FileRef(it) }, rules = listOf("PrintlnInProduction")),
        )
        runModuleFacts(modules, out, "", check, threads)
        // Response timings differ from run to run; everything else must not.
        return out.listFiles()!!.associate { it.name to it.readText().replace(Regex("\"totalMs\":\\d+"), "") }
    }

    @Test fun threadedModulesGiveWhatOneThreadGives() {
        val guard = "package g\n\nfun f(x: Any): Int = when (x) {\n    is String if x.isNotEmpty() -> 1\n    else -> 0\n}\n"
        val pinned = listOf("-language-version=2.1", "-api-version=2.1")
        val serializable = "package s\n\nimport kotlinx.serialization.KSerializer\nimport kotlinx.serialization.Serializable\n\n" +
            "@Serializable\ndata class Foo(val x: Int)\n\nfun fooSerializer(): KSerializer<Foo> = Foo.serializer()\n"
        // A built main module whose internal declaration only its friends see.
        val mainOut = tmp.resolve("lib-classes").toFile().apply { mkdirs() }
        val mainSrc = tmp.resolve("lib-src/Lib.kt").toFile().apply { parentFile.mkdirs(); writeText("package lib\n\ninternal fun secret() = 1\n") }
        compileKotlinLibrary(listOf(mainSrc.path), mainOut)
        val usesSecret = "package t\n\nfun probe() = lib.secret()\n"
        val specs = listOf(
            module("plain", "package p\n\nfun f() { println(\"p\") }\nclass C(val v: Int)\n"),
            module("broken", "package b\n\nfun broken(): Int = missing + 1\n"),
            module("plugin", serializable).let { (m, f) ->
                m.copy(classpath = m.classpath + serializationRuntime, compilerPluginClasspath = listOf(serializationPlugin)) to f
            },
            module("noPlugin", serializable).let { (m, f) -> m.copy(classpath = m.classpath + serializationRuntime) to f },
            module("older", guard).let { (m, f) -> m.copy(compilerArgs = pinned) to f },
            module("modern", guard),
            module("flagged", guard).let { (m, f) -> m.copy(compilerArgs = pinned + "-Xwhen-guards") to f },
            module("friend", usesSecret, kind = "test").let { (m, f) ->
                m.copy(classpath = m.classpath + mainOut.path, friends = listOf(mainOut.path)) to f
            },
            module("stranger", usesSecret, kind = "test").let { (m, f) -> m.copy(classpath = m.classpath + mainOut.path) to f },
        )
        val modules = specs.map { it.first }
        val files = specs.map { it.second }
        val one = run(modules, files, "one", threads = 1)
        assertEquals(2 * modules.size, one.size, "a facts file and a check file per module: ${one.keys}")
        // Each case must give its own verdict, or the comparison below proves nothing about isolation.
        val failing = one.filterKeys { it.endsWith(".check.json") }.filterValues { !it.contains("\"errorFiles\":{}") }.keys
        assertEquals(
            setOf("_broken_main.check.json", "_noPlugin_main.check.json", "_older_main.check.json", "_stranger_test.check.json"),
            failing,
            one.filterKeys { it.endsWith(".check.json") }.toString(),
        )
        assertTrue(one.getValue("_plain_main.check.json").contains("PrintlnInProduction"), one["_plain_main.check.json"])
        assertEquals(one, run(modules, files, "three", threads = 3))
        assertEquals(one, run(modules.reversed(), files, "reversed", threads = 4))
    }

    @Test fun anErrorStopsTheModulesNotYetStarted() {
        val started = Collections.synchronizedList(ArrayList<Int>())
        val release = CountDownLatch(1)
        val began = System.nanoTime()
        val failure = assertFailsWith<OutOfMemoryError> {
            runConcurrently((1..20).toList(), threads = 2) { i ->
                started += i
                when (i) {
                    1 -> throw OutOfMemoryError("heap")
                    // Still compiling when the error arrives: the caller must not wait for it.
                    2 -> release.await(30, TimeUnit.SECONDS)
                }
            }
        }
        val waitedMs = (System.nanoTime() - began) / 1_000_000
        release.countDown()
        assertEquals("heap", failure.message)
        assertFalse(started.any { it > 2 }, "queued items ran after the error: $started")
        assertTrue(waitedMs < 10_000, "the error waited ${waitedMs}ms for a running item")
    }
}
