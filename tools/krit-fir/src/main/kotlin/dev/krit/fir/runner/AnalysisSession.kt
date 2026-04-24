package dev.krit.fir.runner

import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.cli.jvm.K2JVMCompiler
import org.jetbrains.kotlin.config.Services
import java.io.File
import java.nio.file.Files

data class FileRef(val path: String, val contentHash: String = "")

data class BatchResult(
    val id: Long,
    val succeeded: Int,
    val skipped: Int,
    val findings: List<Finding>,
    val crashed: Map<String, String>,
)

// Holds the current session config. When sourceDirs or classpath change the Go side sends a
// "rebuild" command which disposes this session and creates a new one.
// Analysis runs via K2JVMCompiler with krit-fir registered as a plugin via the fat JAR itself.
class AnalysisSession(val sourceDirs: List<String>, val classpath: List<String>) {

    // Path to the running fat JAR — used to register our FIR plugin with the embedded compiler.
    private val selfJar: String? = resolveSelfJar()

    // Eagerly collect all .kt files from sourceDirs. K2 needs all sources for correct type
    // resolution even when checking a subset.
    private val allSourceFiles: List<String> by lazy {
        sourceDirs.flatMap { dir ->
            File(dir).walkTopDown()
                .filter { it.isFile && it.extension == "kt" }
                .map { it.canonicalPath }
                .toList()
        }
    }

    fun check(id: Long, files: List<FileRef>, enabledRules: Set<String>): BatchResult {
        val requestedCanonical = files.map {
            try { File(it.path).canonicalPath } catch (_: Exception) { it.path }
        }.toSet()

        // The protocol uses checker class names (e.g. "FlowCollectInOnCreate"), but FindingCollector
        // matches against the diagnostic names in [RULE_NAME] format (e.g. "FLOW_COLLECT_IN_ON_CREATE").
        val enabledDiagnostics = if (enabledRules.isEmpty()) emptySet()
            else enabledRules.map { checkerToDiagnostic[it] ?: it }.toSet()

        val collector = FindingCollector(requestedCanonical, enabledDiagnostics)
        val outDir = Files.createTempDirectory("krit-fir-out-").toFile()

        try {
            val args = K2JVMCompilerArguments().apply {
                freeArgs = allSourceFiles.ifEmpty { files.map { it.path } }
                this.classpath = this@AnalysisSession.classpath.joinToString(File.pathSeparator)
                destination = outDir.absolutePath
                noStdlib = true
                noReflect = true
                suppressWarnings = false
                if (selfJar != null) {
                    pluginClasspaths = arrayOf(selfJar)
                }
            }
            K2JVMCompiler().exec(collector, Services.EMPTY, args)
        } finally {
            outDir.deleteRecursively()
        }

        val crashed = collector.crashes.toMap()
        return BatchResult(
            id = id,
            succeeded = files.size - crashed.size,
            skipped = 0,
            findings = collector.findings.toList(),
            crashed = crashed,
        )
    }

    fun dispose() {} // No long-lived JVM resources beyond the lazy source file list.

    companion object {
        // Maps the protocol's checker class name to the [DIAGNOSTIC_NAME] emitted by the renderer.
        internal val checkerToDiagnostic = mapOf(
            "FlowCollectInOnCreate" to "FLOW_COLLECT_IN_ON_CREATE",
            "ComposeRememberWithoutKey" to "COMPOSE_REMEMBER_WITHOUT_KEY",
            "UnsafeCastWhenNullable" to "UNSAFE_CAST_WHEN_NULLABLE",
            "SmokeChecker" to "SMOKE_CLASS",
        )

        private fun resolveSelfJar(): String? = try {
            val location = AnalysisSession::class.java.protectionDomain?.codeSource?.location
            location?.toURI()?.let { File(it) }?.absolutePath?.takeIf { it.endsWith(".jar") }
        } catch (_: Exception) {
            null
        }
    }
}
