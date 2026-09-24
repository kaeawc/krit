package dev.jasonpearson.krit.fir.tests

import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSeverity
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSourceLocation
import org.jetbrains.kotlin.cli.common.messages.MessageCollector
import org.jetbrains.kotlin.cli.jvm.K2JVMCompiler
import org.jetbrains.kotlin.config.Services
import java.io.File

// Compiles in-memory Kotlin sources (plus the shared stubs) with the krit-fir
// plugin and returns the diagnostics it emitted. Shared by AbstractDiagnosticTest
// (golden marker tests) and the property tests, so both exercise one compile path.
object KritFirProbe {

    data class Diag(val file: String, val line: Int, val name: String)

    private val stubsDir: File get() = File("src/test/data/stubs")

    // Compiles all [sources] (keyed by filename) together and returns every krit
    // plugin diagnostic, tagged with its factory name (the [NAME] render prefix),
    // file name, and 1-based line.
    fun diagnose(sources: Map<String, String>): List<Diag> {
        val pluginJar = requireNotNull(locatePluginJar()) {
            "krit-fir plugin JAR not found. Set 'krit.fir.plugin.jar' or run `./gradlew :jar`."
        }
        val tmpDir = kotlin.io.path.createTempDirectory("krit-fir-probe").toFile()
        try {
            sources.forEach { (name, body) -> tmpDir.resolve(name).writeText(body) }
            if (stubsDir.isDirectory) {
                stubsDir.listFiles { f -> f.extension == "kt" }?.forEach { stub ->
                    stub.copyTo(tmpDir.resolve(stub.name), overwrite = true)
                }
            }
            val outDir = tmpDir.resolve("out").apply { mkdirs() }
            val diags = mutableListOf<Diag>()
            val collector = object : MessageCollector {
                override fun clear() {}
                override fun hasErrors() = false
                override fun report(
                    severity: CompilerMessageSeverity,
                    message: String,
                    location: CompilerMessageSourceLocation?,
                ) {
                    if (location == null || severity !in reportable) return
                    val match = pluginDiagnosticRe.find(message) ?: return
                    diags.add(Diag(File(location.path).name, location.line, match.groupValues[1]))
                }
            }
            val stdlibJar = System.getProperty("kotlin.stdlib.jar")?.let { File(it).takeIf { f -> f.exists() } }
            K2JVMCompiler().exec(
                collector,
                Services.EMPTY,
                K2JVMCompilerArguments().apply {
                    freeArgs = listOf(tmpDir.absolutePath)
                    destination = outDir.absolutePath
                    noStdlib = true
                    noReflect = true
                    if (stdlibJar != null) classpath = stdlibJar.absolutePath
                    pluginClasspaths = arrayOf(pluginJar.absolutePath)
                },
            )
            return diags
        } finally {
            tmpDir.deleteRecursively()
        }
    }

    // Diagnostics for one source file (named "Main.kt"), the common single-file case.
    fun diagnose(source: String): List<Diag> = diagnose(mapOf("Main.kt" to source))

    private fun locatePluginJar(): File? {
        System.getProperty("krit.fir.plugin.jar")?.let { return File(it).takeIf { f -> f.exists() } }
        return File("../build/libs")
            .takeIf { it.isDirectory }
            ?.listFiles { f -> f.name.startsWith("krit-fir") && f.name.endsWith(".jar") }
            ?.firstOrNull()
    }

    private val reportable = setOf(
        CompilerMessageSeverity.WARNING,
        CompilerMessageSeverity.STRONG_WARNING,
        CompilerMessageSeverity.ERROR,
    )
    private val pluginDiagnosticRe = Regex("""\[([A-Z_]+)]""")
}
