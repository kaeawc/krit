package dev.jasonpearson.krit.intellij

import com.intellij.openapi.diagnostic.Logger
import com.intellij.notification.NotificationGroupManager
import com.intellij.notification.NotificationType
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.application.ReadAction
import com.intellij.openapi.module.ModuleManager
import com.intellij.openapi.project.Project
import com.intellij.openapi.roots.ModuleRootManager
import com.intellij.openapi.roots.ProjectRootManager
import java.io.File
import java.nio.file.Files
import java.util.concurrent.ExecutionException
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException

object KritRunner {
    private val log = Logger.getInstance(KritRunner::class.java)

    sealed class AnalyzeOutcome {
        data class Success(val report: KritReport, val rawJson: String) : AnalyzeOutcome()
        object MissingBinary : AnalyzeOutcome()
        data class Failure(val message: String) : AnalyzeOutcome()
        // The caller's isCancelled check fired (a newer scan superseded this
        // one); any in-flight work was torn down and no result is available.
        object Cancelled : AnalyzeOutcome()
    }

    fun analyzeProject(project: Project, isCancelled: () -> Boolean = NEVER_CANCELLED): AnalyzeOutcome =
        runAnalyze(project, target = null, label = "project analysis", isCancelled = isCancelled)

    // Single-file scan. When a daemon socket is reachable at
    // <repo>/.krit/daemon.sock, sends an analyze-buffer request directly
    // over the Unix socket — no ProcessBuilder.start() on the hot path.
    // Falls back to the CLI invocation if the socket is absent or the
    // RPC fails for any reason, so the plugin behaves identically when
    // no daemon is running.
    //
    // liveContent, when non-null, is the in-editor buffer text — the
    // daemon analyses exactly what the user sees rather than the
    // on-disk snapshot. CLI fallback reads from disk because krit's
    // positional-path argument expects a real file.
    fun analyzeFile(
        project: Project,
        filePath: String,
        liveContent: String? = null,
        isCancelled: () -> Boolean = NEVER_CANCELLED,
    ): AnalyzeOutcome {
        val projectDir = project.baseDir() ?: return AnalyzeOutcome.Failure("project has no base path")
        val daemonOutcome = tryDaemonAnalyzeBuffer(project, projectDir, filePath, liveContent, isCancelled)
        if (daemonOutcome != null) {
            return daemonOutcome
        }
        return runAnalyze(project, target = filePath, label = "file analysis", isCancelled = isCancelled)
    }

    private fun tryDaemonAnalyzeBuffer(
        project: Project,
        projectDir: File,
        filePath: String,
        liveContent: String?,
        isCancelled: () -> Boolean,
    ): AnalyzeOutcome? {
        if (isCancelled()) return AnalyzeOutcome.Cancelled
        val socketPath = KritDaemonClient.socketPathFor(projectDir)
        if (!Files.exists(socketPath)) return null
        val content = liveContent ?: try {
            File(filePath).readText()
        } catch (_: java.io.IOException) {
            return null
        }
        // The daemon protocol has no cancel verb, so a superseded call is not
        // aborted server-side. Run the blocking socket call on a helper thread
        // and stop waiting (interrupting it, which closes the channel) once
        // cancelled; the late result is dropped.
        val client = daemonClientFor(project, socketPath) ?: return null
        val call = daemonCalls.submit<List<KritFinding>?> {
            client.analyzeBuffer(filePath, content)
        }
        var result: List<KritFinding>? = null
        val waited = try {
            KritScanWait.await(Long.MAX_VALUE, KritScanWait.POLL_INTERVAL_MS, isCancelled) { ms ->
                try {
                    result = call.get(ms, TimeUnit.MILLISECONDS)
                    true
                } catch (_: TimeoutException) {
                    false
                }
            }
        } catch (t: ExecutionException) {
            log.debug("krit daemon analyze-buffer threw, falling back to CLI", t)
            return null
        } catch (_: InterruptedException) {
            Thread.currentThread().interrupt()
            call.cancel(true)
            return AnalyzeOutcome.Cancelled
        }
        if (waited == KritWaitResult.Cancelled) {
            call.cancel(true)
            return AnalyzeOutcome.Cancelled
        }
        val findings = result ?: return null
        // rawJson is intentionally empty: the daemon path doesn't carry
        // suggested-fix metadata for apply-suggestion. Per-file scans
        // refresh after each apply-fix anyway, so lastReportJson is
        // never consumed for daemon-served updates.
        return AnalyzeOutcome.Success(KritReport(findings), "")
    }

    // Batched form of the daemon path in analyzeFile, for a change window
    // that touched several files: one analyze-buffers request instead of one
    // round trip per file. liveContents maps each path to its in-editor text
    // (null reads the file from disk). Returns an outcome per path the daemon
    // analyzed; paths it could not (a per-buffer error, an unreadable file)
    // are left out so the caller can retry them one by one. Returns null when
    // no daemon answered, and a map holding only Cancelled when a newer scan
    // superseded this one.
    fun analyzeFilesWithDaemon(
        project: Project,
        liveContents: Map<String, String?>,
        isCancelled: () -> Boolean = NEVER_CANCELLED,
    ): Map<String, AnalyzeOutcome>? {
        if (liveContents.isEmpty()) return emptyMap()
        if (isCancelled()) return liveContents.mapValues { AnalyzeOutcome.Cancelled }
        val projectDir = project.baseDir() ?: return null
        val socketPath = KritDaemonClient.socketPathFor(projectDir)
        if (!Files.exists(socketPath)) return null
        val entries = liveContents.mapNotNull { (path, live) ->
            val content = live ?: try {
                File(path).readText()
            } catch (_: java.io.IOException) {
                null
            }
            content?.let { path to it }
        }
        if (entries.isEmpty()) return emptyMap()
        val client = daemonClientFor(project, socketPath) ?: return null
        // Same cancellation model as tryDaemonAnalyzeBuffer.
        val call = daemonCalls.submit<List<KritDaemonClient.BufferResult>?> { client.analyzeBuffers(entries) }
        var results: List<KritDaemonClient.BufferResult>? = null
        val waited = try {
            KritScanWait.await(Long.MAX_VALUE, KritScanWait.POLL_INTERVAL_MS, isCancelled) { ms ->
                try {
                    results = call.get(ms, TimeUnit.MILLISECONDS)
                    true
                } catch (_: TimeoutException) {
                    false
                }
            }
        } catch (t: ExecutionException) {
            log.debug("krit daemon analyze-buffers threw, falling back", t)
            return null
        } catch (_: InterruptedException) {
            Thread.currentThread().interrupt()
            call.cancel(true)
            return liveContents.mapValues { AnalyzeOutcome.Cancelled }
        }
        if (waited == KritWaitResult.Cancelled) {
            call.cancel(true)
            return liveContents.mapValues { AnalyzeOutcome.Cancelled }
        }
        val batch = results ?: return null
        val outcomes = LinkedHashMap<String, AnalyzeOutcome>()
        entries.forEachIndexed { i, (path, _) ->
            when (val result = batch[i]) {
                is KritDaemonClient.BufferResult.Findings ->
                    outcomes[path] = AnalyzeOutcome.Success(KritReport(result.findings), "")
                is KritDaemonClient.BufferResult.Error ->
                    log.debug("krit daemon could not analyze $path: ${result.message}")
            }
        }
        return outcomes
    }

    // The project's shared daemon client, or null when the project is gone.
    private fun daemonClientFor(project: Project, socketPath: java.nio.file.Path): KritDaemonClient? {
        if (project.isDisposed) return null
        return project.getService(KritProjectService::class.java)?.daemonClientFor(socketPath)
    }

    private fun runAnalyze(
        project: Project,
        target: String?,
        label: String,
        isCancelled: () -> Boolean = NEVER_CANCELLED,
    ): AnalyzeOutcome {
        val projectDir = project.baseDir() ?: return AnalyzeOutcome.Failure("project has no base path")
        val binary = KritBinaryResolver.find(project) ?: return AnalyzeOutcome.MissingBinary
        val scanPath = target ?: projectDir.absolutePath
        val classpath = KritClasspathResolver.resolve(project)
        val output = File.createTempFile("krit-intellij-", ".json")
        return try {
            val command = listOf(
                binary.absolutePath,
                "--format=json",
                "-o",
                output.absolutePath,
                "-q",
                scanPath,
            )
            // krit exits non-zero when it reports findings; trust the output
            // file as long as it exists rather than the exit code.
            val result = runKrit(project, projectDir, command, classpath, ANALYZE_TIMEOUT_SECONDS, label, isCancelled) { exit, _ ->
                exit == 0 || (exit == 1 && output.isFile)
            }
            when (result) {
                is RunOutcome.Ok -> {
                    val raw = output.readText()
                    AnalyzeOutcome.Success(KritJsonParser.parse(raw), raw)
                }
                is RunOutcome.MissingBinary -> AnalyzeOutcome.MissingBinary
                is RunOutcome.Failed -> AnalyzeOutcome.Failure(result.message)
                is RunOutcome.Cancelled -> AnalyzeOutcome.Cancelled
            }
        } catch (t: Throwable) {
            log.warn("krit $label failed for ${projectDir.path}", t)
            AnalyzeOutcome.Failure(t.message ?: "unknown error")
        } finally {
            output.delete()
        }
    }

    fun fixFinding(project: Project, fixLevel: String?, findingId: String): Boolean {
        if (findingId.isBlank()) return false
        val projectDir = project.baseDir() ?: return false
        val binary = KritBinaryResolver.find(project) ?: return false
        val classpath = KritClasspathResolver.resolve(project)
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
        return runKrit(project, projectDir, command, classpath, FIX_TIMEOUT_SECONDS, "fix") { exit, _ -> exit == 0 } is RunOutcome.Ok
    }

    fun applySuggestion(
        project: Project,
        findingId: String,
        suggestionId: String,
        reportJson: String,
    ): Boolean {
        val projectDir = project.baseDir() ?: return false
        val binary = KritBinaryResolver.find(project) ?: return false
        val classpath = KritClasspathResolver.resolve(project)
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
            runKrit(project, projectDir, command, classpath, APPLY_SUGGESTION_TIMEOUT_SECONDS, "apply-suggestion") { exit, _ ->
                exit == 0
            } is RunOutcome.Ok
        } finally {
            reportFile.delete()
        }
    }

    private sealed class RunOutcome {
        object Ok : RunOutcome()
        object MissingBinary : RunOutcome()
        object Cancelled : RunOutcome()
        data class Failed(val message: String) : RunOutcome()
    }

    private fun runKrit(
        project: Project,
        projectDir: File,
        command: List<String>,
        classpath: List<File>,
        timeoutSeconds: Long,
        label: String,
        isCancelled: () -> Boolean = NEVER_CANCELLED,
        isSuccess: (exit: Int, stderr: String) -> Boolean,
    ): RunOutcome {
        if (isCancelled()) return RunOutcome.Cancelled
        return try {
            val builder = ProcessBuilder(command)
                .directory(projectDir)
                .redirectError(ProcessBuilder.Redirect.PIPE)
                .redirectOutput(ProcessBuilder.Redirect.PIPE)
            if (classpath.isNotEmpty()) {
                // Krit's Go side reads CLASSPATH via splitEnvClasspath in
                // internal/cli/scan/oracle_classpath.go and threads it into
                // the FIR/KAA oracle. Setting it here is what makes
                // type-aware rules in the IDE behave the same as
                // krit-gradle-plugin runs.
                builder.environment()["CLASSPATH"] = KritClasspathResolver.toClasspathString(classpath)
            }
            val javaHome = ReadAction.compute<String?, RuntimeException> {
                ProjectRootManager.getInstance(project).projectSdk?.homePath
                    ?: ModuleManager.getInstance(project).modules.firstNotNullOfOrNull { module ->
                        ModuleRootManager.getInstance(module).sdk?.homePath
                    }
            }
            if (!javaHome.isNullOrBlank()) builder.environment()["JAVA_HOME"] = javaHome
            val process = builder.start()

            val waited = KritScanWait.await(
                TimeUnit.SECONDS.toMillis(timeoutSeconds),
                KritScanWait.POLL_INTERVAL_MS,
                isCancelled,
            ) { ms -> process.waitFor(ms, TimeUnit.MILLISECONDS) }
            if (waited == KritWaitResult.Cancelled) {
                // Superseded by a newer scan: kill and reap the process so no
                // zombie krit is left behind, and report no result.
                process.destroyForcibly()
                process.waitFor()
                log.debug("krit $label cancelled for ${projectDir.path}")
                return RunOutcome.Cancelled
            }
            if (waited == KritWaitResult.TimedOut) {
                process.destroyForcibly()
                val msg = "krit $label timed out after ${timeoutSeconds}s"
                log.warn("$msg for ${projectDir.path}")
                return RunOutcome.Failed(msg)
            }

            val stderr = process.errorStream.bufferedReader().readText()
            val exit = process.exitValue()
            if (exit == 2) {
                val msg = stderr.trim().ifBlank { "krit preflight failed (exit code 2)" }
                ApplicationManager.getApplication().invokeLater {
                    if (!project.isDisposed) {
                        NotificationGroupManager.getInstance().getNotificationGroup("Krit")
                            .createNotification("Krit configuration error", msg, NotificationType.ERROR)
                            .notify(project)
                    }
                }
                return RunOutcome.Failed(msg)
            }
            if (!isSuccess(exit, stderr)) {
                val msg = "krit $label exited $exit: ${stderr.lineSequence().firstOrNull().orEmpty()}"
                log.warn("$msg for ${projectDir.path}")
                return RunOutcome.Failed(msg)
            }
            RunOutcome.Ok
        } catch (t: java.io.IOException) {
            // IOException at start() typically means the binary disappeared
            // between resolver lookup and exec. Treat it as MissingBinary
            // so the status surface stays consistent. The user-facing
            // notification comes from notifyMissingBinaryOnce, so keep this
            // at debug to avoid repeated warn lines.
            log.debug("krit $label process start failed for ${projectDir.path}", t)
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

    private val NEVER_CANCELLED: () -> Boolean = { false }

    private val daemonCalls = Executors.newCachedThreadPool { runnable ->
        Thread(runnable, "krit-daemon-call").apply { isDaemon = true }
    }

    private const val ANALYZE_TIMEOUT_SECONDS = 120L
    private const val FIX_TIMEOUT_SECONDS = 120L
    private const val APPLY_SUGGESTION_TIMEOUT_SECONDS = 60L
}
