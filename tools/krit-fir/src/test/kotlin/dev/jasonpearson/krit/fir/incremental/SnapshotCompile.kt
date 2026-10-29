@file:OptIn(org.jetbrains.kotlin.compiler.plugin.ExperimentalCompilerApi::class)

package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.BatchResult
import dev.jasonpearson.krit.fir.runner.BundledStdlib
import dev.jasonpearson.krit.fir.runner.KotlinCompilerSettings
import dev.jasonpearson.krit.fir.runner.resolveJvmTarget
import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.cli.common.messages.MessageCollector
import org.jetbrains.kotlin.cli.jvm.K2JVMCompiler
import org.jetbrains.kotlin.config.Services
import java.io.File

// Test plumbing shared by the incremental tests. The snapshot and the slice compile themselves are production code
// (`AnalysisSession.check` builds the snapshot, `AnalysisSession.sliceCheck` compiles a slice); this file only
// compiles a stub library and reduces a `BatchResult` to the verdict of one file.

internal fun canon(p: String): String = try { File(p).canonicalPath } catch (_: Exception) { File(p).absolutePath }

/** Compiles [sources] with the backend into the class directory [out], without krit's plugin. */
internal fun compileKotlinLibrary(sources: List<String>, out: File, javaSourceRoots: List<String> = emptyList()) {
    val args = K2JVMCompilerArguments().apply {
        freeArgs = sources
        classpath = BundledStdlib.effectiveClasspath(emptyList()).joinToString(File.pathSeparator)
        jvmTarget = resolveJvmTarget("")
        KotlinCompilerSettings.parse(emptyList()).applyTo(this)
        destination = out.absolutePath
        noStdlib = true
        noReflect = true
        moduleName = "stubs"
        if (javaSourceRoots.isNotEmpty()) this.javaSourceRoots = javaSourceRoots.toTypedArray()
    }
    K2JVMCompiler().exec(MessageCollector.NONE, Services.EMPTY, args)
}

/** The stdlib jar the unit tests compile against. */
internal val testStdlib: String = File(Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath

internal fun File.writeSource(relative: String, text: String): File =
    File(this, relative).also { it.parentFile.mkdirs(); it.writeText(text) }

/** Every `.kt` under [src], canonical paths, sorted. */
internal fun kotlinFilesIn(src: File): List<String> =
    src.walkTopDown().filter { it.isFile && it.extension == "kt" }.map { canon(it.path) }.sorted().toList()

/** A recording session that builds the snapshot under [snapshots] (never the user's cache dir). */
internal fun recordingSession(src: File, snapshots: File): AnalysisSession =
    AnalysisSession(listOf(src.path), listOf(testStdlib), incrementalRecording = true, incrementalPlanning = false)
        .also { it.snapshotRootOverride = snapshots }

/** A session that records nothing: the reference, as the check runs without the feature. */
internal fun plainSession(src: File): AnalysisSession =
    AnalysisSession(listOf(src.path), listOf(testStdlib), incrementalRecording = false, incrementalPlanning = false)

/** Every file under [dir], relative paths. */
internal fun filesUnder(dir: File): Set<String> =
    dir.walkTopDown().filter { it.isFile }.map { it.relativeTo(dir).path }.toSet()

/** One file's verdict as Go consumes it. */
internal data class FileVerdict(
    val findings: List<String>,
    val error: Boolean,
    val errorMessage: String?,
    val ruleErrors: Set<String>,
) {
    /** What must be equal between a full check and a slice check of the same file. */
    fun key() = Triple(findings, error, ruleErrors.sorted())
}

/** [path] must be spelled as in the request that produced this result. */
internal fun BatchResult.verdictOf(path: String): FileVerdict = FileVerdict(
    findings = findings.filter { it.path == path }.map { "${it.rule}@${it.line}:${it.col} ${it.message}" }.sorted(),
    error = path in errorFiles,
    errorMessage = errorFiles[path],
    ruleErrors = ruleErrors.filterValues { path in it }.keys,
)
