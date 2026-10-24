package dev.jasonpearson.krit.intellij

import com.intellij.codeInsight.daemon.DaemonCodeAnalyzer
import com.intellij.notification.NotificationAction
import com.intellij.notification.NotificationGroupManager
import com.intellij.notification.NotificationType
import com.intellij.openapi.Disposable
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.diagnostic.Logger
import com.intellij.openapi.editor.EditorFactory
import com.intellij.openapi.editor.event.BulkAwareDocumentListener
import com.intellij.openapi.editor.event.DocumentEvent
import com.intellij.openapi.application.ReadAction
import com.intellij.openapi.fileEditor.FileDocumentManager
import com.intellij.openapi.project.Project
import com.intellij.openapi.roots.ProjectFileIndex
import com.intellij.openapi.vfs.LocalFileSystem
import com.intellij.openapi.vfs.VirtualFile
import com.intellij.openapi.wm.WindowManager
import java.io.File
import java.nio.file.Path
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.Executors
import java.util.concurrent.ScheduledFuture
import java.util.concurrent.ScheduledExecutorService
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean

class KritProjectService(private val project: Project) : Disposable {
    private val log = Logger.getInstance(KritProjectService::class.java)
    private val running = AtomicBoolean(false)
    private val rerunAfterCurrent = AtomicBoolean(false)
    private val scanLock = Any()
    private val executor: ScheduledExecutorService =
        Executors.newSingleThreadScheduledExecutor { runnable ->
            Thread(runnable, "krit-project-runner").apply { isDaemon = true }
        }
    private var pendingScan: ScheduledFuture<*>? = null
    private val pendingFiles = ConcurrentHashMap.newKeySet<String>()
    // Bumped on every document change that schedules a scan. A running scan
    // captures the value it started under and abandons itself (killing the
    // krit process / dropping the daemon reply) once a newer edit arrives.
    private val scanGeneration = KritScanGeneration()

    // Set when a project-wide scan was cancelled before publishing, so the
    // next scan does a full refresh instead of only the edited files.
    @Volatile
    private var projectRescanPending = false
    private val documentListener = object : BulkAwareDocumentListener {
        override fun documentChanged(event: DocumentEvent) {
            val file = FileDocumentManager.getInstance().getFile(event.document) ?: return
            if (shouldTriggerScan(file)) {
                scanGeneration.bump()
                pendingFiles.add(file.path)
                scheduleScan(KritScanDebounce.effectiveMs(KritSettingsState.getSafely()?.state?.debounceMs))
            }
        }
    }

    @Volatile
    private var findingsByFile: Map<String, List<KritFinding>> = emptyMap()

    @Volatile
    private var lastReportJson: String = ""

    @Volatile
    var state: KritState = KritState.Initializing
        private set

    private val missingBinaryNotified = AtomicBoolean(false)

    // Scans shell out to the krit binary; unit tests never run one.
    private val scansEnabled = !ApplicationManager.getApplication().isUnitTestMode

    init {
        EditorFactory.getInstance().eventMulticaster.addDocumentListener(documentListener, this)
        // Light-fixture tests seed findings directly; an initial scan would
        // race with the seed and overwrite it (no krit binary in tests).
        if (scansEnabled) {
            scheduleScan(0)
        }
    }

    fun findingsFor(path: String): List<KritFinding> {
        return findingsByFile[canonicalPath(path)].orEmpty()
    }

    internal fun seedFindingsForTest(path: String, findings: List<KritFinding>) {
        findingsByFile = findingsByFile + (canonicalPath(path) to findings)
    }

    // Light-fixture tests share one project, and so this service, across
    // test methods; clear seeded findings so one test's seed can't leak.
    internal fun clearFindingsForTest() {
        findingsByFile = emptyMap()
    }

    // Entry points for the Analyze Project / Analyze File actions. Both
    // skip the document-change debounce and re-arm the missing-binary
    // notification so an explicit user request is never met with silence.
    fun analyzeNow() {
        missingBinaryNotified.set(false)
        // A non-empty pending set would turn the run into a changed-file
        // refresh; the full scan below covers those files anyway.
        pendingFiles.clear()
        scheduleScan(0)
    }

    fun analyzeFiles(paths: Collection<String>) {
        if (paths.isEmpty()) return
        missingBinaryNotified.set(false)
        pendingFiles.addAll(paths)
        scheduleScan(0)
    }

    fun applyFix(fixLevel: String?, findingId: String) {
        executor.execute {
            if (project.isDisposed || !running.compareAndSet(false, true)) {
                rerunAfterCurrent.set(true)
                return@execute
            }
            try {
                if (KritRunner.fixFinding(project, fixLevel, findingId)) {
                    refreshFindings()
                }
            } catch (t: Throwable) {
                log.warn("krit per-finding fix runner failed", t)
            } finally {
                running.set(false)
                scheduleRequestedRerun()
            }
        }
    }

    fun applySuggestion(findingId: String, suggestionId: String) {
        val cachedReport = lastReportJson
        executor.execute {
            if (project.isDisposed || !running.compareAndSet(false, true)) {
                rerunAfterCurrent.set(true)
                return@execute
            }
            try {
                if (KritRunner.applySuggestion(project, findingId, suggestionId, cachedReport)) {
                    refreshFindings()
                }
            } catch (t: Throwable) {
                log.warn("krit apply-suggestion runner failed", t)
            } finally {
                running.set(false)
                scheduleRequestedRerun()
            }
        }
    }

    private fun runIfIdle() {
        if (project.isDisposed || !running.compareAndSet(false, true)) {
            rerunAfterCurrent.set(true)
            return
        }
        try {
            val generation = scanGeneration.current()
            val isCancelled = { scanGeneration.isSuperseded(generation) }
            val targets = drainPendingFiles()
            val full = targets.isEmpty() || projectRescanPending
            if (full) {
                if (!refreshFindings(isCancelled)) {
                    // Superseded: keep the edited files queued for the scan
                    // that replaces this one and remember to redo the project.
                    projectRescanPending = true
                    pendingFiles.addAll(targets)
                    return
                }
                projectRescanPending = false
            }
            if (targets.isNotEmpty() && !refreshChangedFiles(targets, isCancelled)) {
                pendingFiles.addAll(targets)
            }
        } catch (t: Throwable) {
            log.warn("krit project runner failed", t)
        } finally {
            running.set(false)
            scheduleRequestedRerun()
        }
    }

    private fun drainPendingFiles(): Set<String> {
        if (pendingFiles.isEmpty()) return emptySet()
        val snapshot = HashSet(pendingFiles)
        pendingFiles.removeAll(snapshot)
        return snapshot
    }

    // Returns false when the scan was superseded (or cancelled) and nothing was
    // published; the status is left at Scanning for the superseding scan.
    private fun refreshFindings(isCancelled: () -> Boolean = { false }): Boolean {
        transition(KritState.Scanning)
        val outcome = KritRunner.analyzeProject(project, isCancelled)
        if (outcome is KritRunner.AnalyzeOutcome.Cancelled || isCancelled()) return false
        when (outcome) {
            is KritRunner.AnalyzeOutcome.Success -> {
                findingsByFile = outcome.report.findings.groupBy {
                    canonicalPath(it.file, project.basePath)
                }
                lastReportJson = outcome.rawJson
                transition(KritState.Idle(outcome.report.findings.size))
            }
            is KritRunner.AnalyzeOutcome.MissingBinary -> {
                findingsByFile = emptyMap()
                lastReportJson = ""
                transition(KritState.MissingBinary)
                notifyMissingBinaryOnce()
            }
            is KritRunner.AnalyzeOutcome.Failure -> {
                transition(KritState.Error(outcome.message))
            }
            is KritRunner.AnalyzeOutcome.Cancelled -> return false
        }
        restartHighlighting()
        return true
    }

    // Returns false when the scan was superseded and nothing was published.
    private fun refreshChangedFiles(paths: Set<String>, isCancelled: () -> Boolean = { false }): Boolean {
        // Per-file rescans replace findings for each changed file in the
        // running map; other files' findings are untouched. Errors and
        // missing-binary on any one file bubble up to the state surface
        // the same way a project scan would so the user still sees the
        // failure mode in the status bar.
        transition(KritState.Scanning)
        val updated = findingsByFile.toMutableMap()
        // Several changed files go to the daemon in one analyze-buffers
        // request. Files it didn't answer for (no daemon, a per-buffer error)
        // fall through to the one-by-one path below, which still ends in the
        // CLI when no daemon is running.
        val batched = if (paths.size > 1) {
            KritRunner.analyzeFilesWithDaemon(project, paths.associateWith { readLiveDocument(it) }, isCancelled)
                .orEmpty()
        } else {
            emptyMap()
        }
        for (path in paths) {
            val canonical = canonicalPath(path, project.basePath)
            val outcome = batched[path]
                ?: KritRunner.analyzeFile(project, path, readLiveDocument(path), isCancelled)
            if (outcome is KritRunner.AnalyzeOutcome.Cancelled || isCancelled()) return false
            when (outcome) {
                is KritRunner.AnalyzeOutcome.Success -> {
                    val grouped = outcome.report.findings.groupBy {
                        canonicalPath(it.file, project.basePath)
                    }
                    updated[canonical] = grouped[canonical] ?: emptyList()
                }
                is KritRunner.AnalyzeOutcome.MissingBinary -> {
                    transition(KritState.MissingBinary)
                    notifyMissingBinaryOnce()
                    restartHighlighting()
                    return true
                }
                is KritRunner.AnalyzeOutcome.Failure -> {
                    transition(KritState.Error(outcome.message))
                    restartHighlighting()
                    return true
                }
                is KritRunner.AnalyzeOutcome.Cancelled -> return false
            }
        }
        findingsByFile = updated
        transition(KritState.Idle(updated.values.sumOf { it.size }))
        restartHighlighting()
        return true
    }

    // Returns the in-editor buffer text for path when one is open in the
    // IDE, or null when no document is loaded for this file. The daemon
    // analyse-buffer path consumes this so the user sees diagnostics for
    // what's on screen rather than the on-disk snapshot; the CLI
    // fallback ignores it and reads from disk via krit's path arg.
    private fun readLiveDocument(path: String): String? {
        return ReadAction.compute<String?, RuntimeException> {
            val file = LocalFileSystem.getInstance().findFileByPath(path) ?: return@compute null
            FileDocumentManager.getInstance().getDocument(file)?.text
        }
    }

    private fun transition(next: KritState) {
        state = next
        ApplicationManager.getApplication().invokeLater {
            if (project.isDisposed) return@invokeLater
            WindowManager.getInstance().getStatusBar(project)?.updateWidget(KritStatusBarWidget.ID)
        }
    }

    private fun notifyMissingBinaryOnce() {
        if (!missingBinaryNotified.compareAndSet(false, true)) return
        ApplicationManager.getApplication().invokeLater {
            if (project.isDisposed) return@invokeLater
            NotificationGroupManager.getInstance()
                .getNotificationGroup("Krit")
                .createNotification(
                    "Krit binary not found",
                    "Set -Dkrit.binary or KRIT_BINARY, or install krit on PATH.",
                    NotificationType.WARNING,
                )
                .notify(project)
        }
    }

    private fun scheduleScan(delayMs: Long) {
        synchronized(scanLock) {
            pendingScan?.cancel(false)
            pendingScan = executor.schedule(::runIfIdle, delayMs, TimeUnit.MILLISECONDS)
        }
    }

    private fun scheduleRequestedRerun() {
        if (rerunAfterCurrent.getAndSet(false)) {
            scheduleScan(0)
        }
    }

    private fun restartHighlighting() {
        ApplicationManager.getApplication().invokeLater {
            if (!project.isDisposed) {
                DaemonCodeAnalyzer.getInstance(project).settingsChanged()
            }
        }
    }

    // One daemon client (and so one open socket) per daemon socket path for
    // the life of the project. A changed socket path (the daemon moved to its
    // fallback socket) replaces the client.
    private var daemonClient: KritDaemonClient? = null
    private var daemonClientSocket: Path? = null

    @Synchronized
    internal fun daemonClientFor(socketPath: Path): KritDaemonClient? {
        if (disposed) return null
        if (daemonClientSocket != socketPath) {
            daemonClient?.close()
            daemonClient = KritDaemonClient(
                socketPath,
                expectedBinaryHash = { KritBinaryResolver.find(project)?.let { KritBinaryHash.of(it) } },
                onBinaryMismatch = { _, _ -> notifyBinaryMismatchOnce() },
            )
            daemonClientSocket = socketPath
        }
        return daemonClient
    }

    // The daemon was started by a different krit binary than this project
    // resolves (an upgrade mid-session, or a daemon from another install).
    // Scans already fall back to the CLI; offer to restart the daemon. The
    // client reports each mismatch once, until the daemon matches again.
    private fun notifyBinaryMismatchOnce() {
        ApplicationManager.getApplication().invokeLater {
            if (project.isDisposed) return@invokeLater
            NotificationGroupManager.getInstance()
                .getNotificationGroup("Krit")
                .createNotification(
                    "Krit daemon runs a different krit binary",
                    "The running daemon was started by another krit binary than this project uses, " +
                        "so Krit is using the CLI instead. Restart the daemon to use it again.",
                    NotificationType.WARNING,
                )
                .addAction(NotificationAction.createSimpleExpiring("Restart daemon") { restartDaemon() })
                .notify(project)
        }
    }

    private fun restartDaemon() {
        ApplicationManager.getApplication().executeOnPooledThread {
            if (project.isDisposed) return@executeOnPooledThread
            val restarted = KritRunner.restartDaemon(project)
            synchronized(this) { daemonClient?.resetBinaryCheck() }
            if (!restarted) {
                ApplicationManager.getApplication().invokeLater {
                    if (project.isDisposed) return@invokeLater
                    NotificationGroupManager.getInstance()
                        .getNotificationGroup("Krit")
                        .createNotification(
                            "Krit daemon restart failed",
                            "Run `krit daemon restart` in the project directory; see the IDE log for details.",
                            NotificationType.ERROR,
                        )
                        .notify(project)
                }
            }
        }
    }

    @Volatile
    private var disposed = false

    override fun dispose() {
        synchronized(scanLock) {
            pendingScan?.cancel(false)
            pendingScan = null
        }
        executor.shutdownNow()
        synchronized(this) {
            disposed = true
            daemonClient?.close()
            daemonClient = null
            daemonClientSocket = null
        }
    }

    private fun shouldTriggerScan(file: VirtualFile): Boolean {
        if (!scansEnabled) {
            return false
        }
        if (!isSupportedSourceFile(file)) {
            return false
        }
        val index = ProjectFileIndex.getInstance(project)
        if (!index.isInSourceContent(file)) {
            return false
        }
        if (index.isInLibrary(file) || isGeneratedOrBuildPath(file)) {
            return false
        }
        return true
    }

    private fun isSupportedSourceFile(file: VirtualFile): Boolean =
        KritFileFilter.isSupported(file.name)

    private fun isGeneratedOrBuildPath(file: VirtualFile): Boolean {
        val path = file.path
        return GENERATED_OR_BUILD_PATH_PARTS.any { path.contains(it) }
    }

    private fun canonicalPath(path: String, basePath: String? = null): String {
        return try {
            val file = File(path)
            if (file.isAbsolute || basePath == null) {
                file.canonicalPath
            } else {
                File(basePath, path).canonicalPath
            }
        } catch (_: Exception) {
            path
        }
    }

    companion object {
        private val GENERATED_OR_BUILD_PATH_PARTS = listOf(
            "/.gradle/",
            "/.idea/",
            "/build/",
            "/generated/",
            "/.kotlin/",
        )
    }
}
