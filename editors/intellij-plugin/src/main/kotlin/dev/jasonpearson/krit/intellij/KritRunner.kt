package dev.jasonpearson.krit.intellij

import com.intellij.openapi.diagnostic.Logger
import com.intellij.openapi.project.Project
import java.io.File
import java.util.concurrent.TimeUnit

object KritRunner {
    private val log = Logger.getInstance(KritRunner::class.java)

    data class AnalyzeResult(val report: KritReport, val rawJson: String) {
        companion object {
            val EMPTY = AnalyzeResult(KritReport(), "")
        }
    }

    fun analyzeProject(project: Project): AnalyzeResult {
        val projectDir = project.baseDir() ?: return AnalyzeResult.EMPTY
        val output = File.createTempFile("krit-intellij-", ".json")
        return try {
            val command = listOf(
                kritBinary(),
                "--format=json",
                "-o",
                output.absolutePath,
                "-q",
                projectDir.absolutePath,
            )
            // krit exits non-zero when it reports findings; trust the output
            // file as long as it exists rather than the exit code.
            val ok = runKrit(projectDir, command, ANALYZE_TIMEOUT_SECONDS, "project analysis") { exit, _ ->
                exit == 0 || output.isFile
            }
            if (!ok) return AnalyzeResult.EMPTY
            val raw = output.readText()
            AnalyzeResult(KritJsonParser.parse(raw), raw)
        } catch (t: Throwable) {
            log.warn("krit project analysis failed for ${projectDir.path}", t)
            AnalyzeResult.EMPTY
        } finally {
            output.delete()
        }
    }

    fun fixProject(project: Project, fixLevel: String?): Boolean {
        val projectDir = project.baseDir() ?: return false
        val level = KritFixLabels.normalizeFixLevel(fixLevel)
        val command = listOf(
            kritBinary(),
            "--fix",
            "--fix-level",
            level,
            "-q",
            projectDir.absolutePath,
        )
        return runKrit(projectDir, command, FIX_TIMEOUT_SECONDS, "fix") { exit, _ -> exit == 0 }
    }

    fun applySuggestion(
        project: Project,
        findingId: String,
        suggestionId: String,
        reportJson: String,
    ): Boolean {
        val projectDir = project.baseDir() ?: return false
        if (reportJson.isBlank()) {
            log.warn("krit apply-suggestion skipped: no cached report for ${projectDir.path}")
            return false
        }
        val reportFile = File.createTempFile("krit-intellij-suggest-", ".json")
        return try {
            reportFile.writeText(reportJson)
            val command = listOf(
                kritBinary(),
                "apply-suggestion",
                "--finding",
                findingId,
                "--suggestion",
                suggestionId,
                "--base",
                projectDir.absolutePath,
                reportFile.absolutePath,
            )
            runKrit(projectDir, command, APPLY_SUGGESTION_TIMEOUT_SECONDS, "apply-suggestion") { exit, _ ->
                exit == 0
            }
        } finally {
            reportFile.delete()
        }
    }

    private fun runKrit(
        projectDir: File,
        command: List<String>,
        timeoutSeconds: Long,
        label: String,
        isSuccess: (exit: Int, stderr: String) -> Boolean,
    ): Boolean {
        return try {
            val process = ProcessBuilder(command)
                .directory(projectDir)
                .redirectError(ProcessBuilder.Redirect.PIPE)
                .redirectOutput(ProcessBuilder.Redirect.PIPE)
                .start()

            if (!process.waitFor(timeoutSeconds, TimeUnit.SECONDS)) {
                process.destroyForcibly()
                log.warn("krit $label timed out for ${projectDir.path}")
                return false
            }

            val stderr = process.errorStream.bufferedReader().readText()
            val exit = process.exitValue()
            if (!isSuccess(exit, stderr)) {
                log.warn("krit $label failed for ${projectDir.path} with exit $exit: $stderr")
                return false
            }
            true
        } catch (t: Throwable) {
            log.warn("krit $label failed for ${projectDir.path}", t)
            false
        }
    }

    private fun kritBinary(): String {
        return System.getProperty("krit.binary")
            ?: System.getenv("KRIT_BINARY")
            ?: "krit"
    }

    private fun Project.baseDir(): File? {
        val base = basePath ?: return null
        return File(base)
    }

    private const val ANALYZE_TIMEOUT_SECONDS = 120L
    private const val FIX_TIMEOUT_SECONDS = 120L
    private const val APPLY_SUGGESTION_TIMEOUT_SECONDS = 60L
}
