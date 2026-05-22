package dev.jasonpearson.krit.intellij

import com.intellij.openapi.diagnostic.Logger
import com.intellij.openapi.project.Project
import java.io.File
import java.util.concurrent.TimeUnit

object KritRunner {
    private val log = Logger.getInstance(KritRunner::class.java)

    sealed class AnalyzeOutcome {
        data class Success(val report: KritReport, val rawJson: String) : AnalyzeOutcome()
        object MissingBinary : AnalyzeOutcome()
        data class Failure(val message: String) : AnalyzeOutcome()
    }

    fun analyzeProject(project: Project): AnalyzeOutcome {
        val projectDir = project.baseDir() ?: return AnalyzeOutcome.Failure("project has no base path")
        val binary = KritBinaryResolver.find() ?: return AnalyzeOutcome.MissingBinary
        val output = File.createTempFile("krit-intellij-", ".json")
        return try {
            val command = listOf(
                binary.absolutePath,
                "--format=json",
                "-o",
                output.absolutePath,
                "-q",
                projectDir.absolutePath,
            )
            // krit exits non-zero when it reports findings; trust the output
            // file as long as it exists rather than the exit code.
            val result = runKrit(projectDir, command, ANALYZE_TIMEOUT_SECONDS, "project analysis") { exit, _ ->
                exit == 0 || output.isFile
            }
            when (result) {
                is RunOutcome.Ok -> {
                    val raw = output.readText()
                    AnalyzeOutcome.Success(KritJsonParser.parse(raw), raw)
                }
                is RunOutcome.MissingBinary -> AnalyzeOutcome.MissingBinary
                is RunOutcome.Failed -> AnalyzeOutcome.Failure(result.message)
            }
        } catch (t: Throwable) {
            log.warn("krit project analysis failed for ${projectDir.path}", t)
            AnalyzeOutcome.Failure(t.message ?: "unknown error")
        } finally {
            output.delete()
        }
    }

    fun fixFinding(project: Project, fixLevel: String?, findingId: String): Boolean {
        if (findingId.isBlank()) return false
        val projectDir = project.baseDir() ?: return false
        val binary = KritBinaryResolver.find() ?: return false
        val level = KritFixLabels.normalizeFixLevel(fixLevel)
        val command = listOf(
            binary.absolutePath,
            "--fix",
            "--fix-level",
            level,
            "--finding-id",
            findingId,
            "-q",
            projectDir.absolutePath,
        )
        return runKrit(projectDir, command, FIX_TIMEOUT_SECONDS, "fix") { exit, _ -> exit == 0 } is RunOutcome.Ok
    }

    fun applySuggestion(
        project: Project,
        findingId: String,
        suggestionId: String,
        reportJson: String,
    ): Boolean {
        val projectDir = project.baseDir() ?: return false
        val binary = KritBinaryResolver.find() ?: return false
        if (reportJson.isBlank()) {
            log.warn("krit apply-suggestion skipped: no cached report for ${projectDir.path}")
            return false
        }
        val reportFile = File.createTempFile("krit-intellij-suggest-", ".json")
        return try {
            reportFile.writeText(reportJson)
            val command = listOf(
                binary.absolutePath,
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
            } is RunOutcome.Ok
        } finally {
            reportFile.delete()
        }
    }

    private sealed class RunOutcome {
        object Ok : RunOutcome()
        object MissingBinary : RunOutcome()
        data class Failed(val message: String) : RunOutcome()
    }

    private fun runKrit(
        projectDir: File,
        command: List<String>,
        timeoutSeconds: Long,
        label: String,
        isSuccess: (exit: Int, stderr: String) -> Boolean,
    ): RunOutcome {
        return try {
            val process = ProcessBuilder(command)
                .directory(projectDir)
                .redirectError(ProcessBuilder.Redirect.PIPE)
                .redirectOutput(ProcessBuilder.Redirect.PIPE)
                .start()

            if (!process.waitFor(timeoutSeconds, TimeUnit.SECONDS)) {
                process.destroyForcibly()
                val msg = "krit $label timed out after ${timeoutSeconds}s"
                log.warn("$msg for ${projectDir.path}")
                return RunOutcome.Failed(msg)
            }

            val stderr = process.errorStream.bufferedReader().readText()
            val exit = process.exitValue()
            if (!isSuccess(exit, stderr)) {
                val msg = "krit $label exited $exit: ${stderr.lineSequence().firstOrNull().orEmpty()}"
                log.warn("$msg for ${projectDir.path}")
                return RunOutcome.Failed(msg)
            }
            RunOutcome.Ok
        } catch (t: java.io.IOException) {
            // IOException at start() typically means the binary disappeared
            // between resolver lookup and exec. Treat it as MissingBinary
            // so the status surface stays consistent.
            log.warn("krit $label process start failed for ${projectDir.path}", t)
            RunOutcome.MissingBinary
        } catch (t: Throwable) {
            log.warn("krit $label failed for ${projectDir.path}", t)
            RunOutcome.Failed(t.message ?: "unknown error")
        }
    }

    private fun Project.baseDir(): File? {
        val base = basePath ?: return null
        return File(base)
    }

    private const val ANALYZE_TIMEOUT_SECONDS = 120L
    private const val FIX_TIMEOUT_SECONDS = 120L
    private const val APPLY_SUGGESTION_TIMEOUT_SECONDS = 60L
}
