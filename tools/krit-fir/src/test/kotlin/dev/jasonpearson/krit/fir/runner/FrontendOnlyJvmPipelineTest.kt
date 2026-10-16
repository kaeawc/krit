package dev.jasonpearson.krit.fir.runner

import org.jetbrains.kotlin.cli.common.ExitCode
import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSeverity
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSourceLocation
import org.jetbrains.kotlin.cli.common.messages.MessageCollector
import org.jetbrains.kotlin.config.Services
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/** #737: krit's compiles stop after the K2 frontend but keep its diagnostics and exit codes. */
class FrontendOnlyJvmPipelineTest {
    @TempDir lateinit var tmp: Path

    private class Recorder : MessageCollector {
        val messages = mutableListOf<Pair<CompilerMessageSeverity, String>>()
        override fun clear() = messages.clear()
        override fun hasErrors() = messages.any { it.first.isError }
        override fun report(severity: CompilerMessageSeverity, message: String, location: CompilerMessageSourceLocation?) {
            messages += severity to message
        }
    }

    private fun compile(source: String): Triple<ExitCode, Recorder, File> {
        val src = tmp.resolve("Sample.kt").toFile().apply { writeText(source) }
        val out = tmp.resolve("out").toFile().apply { mkdirs() }
        val args = K2JVMCompilerArguments().apply {
            freeArgs = listOf(src.path)
            classpath = effectiveClasspath(emptyList()).joinToString(File.pathSeparator)
            destination = out.path
            noStdlib = true
            noReflect = true
            reportAllWarnings = true
        }
        val recorder = Recorder()
        return Triple(FrontendOnlyJvmPipeline().execute(args, Services.EMPTY, recorder), recorder, out)
    }

    @Test
    fun cleanSourceSucceedsWithoutWritingClassFiles() {
        val (exit, recorder, out) = compile("fun answer(): Int = listOf(1, 2).sum()\n")
        assertEquals(ExitCode.OK, exit, recorder.messages.toString())
        assertTrue(out.walkTopDown().none { it.isFile }, "backend output written: ${out.walkTopDown().toList()}")
    }

    @Test
    fun frontendErrorsAndWarningsStillReport() {
        val (exit, recorder, _) = compile("fun broken(): Int = \"text\"\nfun warn(x: String) = x!!\n")
        assertEquals(ExitCode.COMPILATION_ERROR, exit)
        assertTrue(recorder.messages.any { it.first.isError && "mismatch" in it.second.lowercase() }, recorder.messages.toString())
        assertTrue(recorder.messages.any { it.first == CompilerMessageSeverity.WARNING }, recorder.messages.toString())
    }
}
