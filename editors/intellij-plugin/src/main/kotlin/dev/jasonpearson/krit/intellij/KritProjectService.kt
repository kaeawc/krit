package dev.jasonpearson.krit.intellij

import com.intellij.codeInsight.daemon.DaemonCodeAnalyzer
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
import com.intellij.codeInspection.InspectionProfile
import com.intellij.profile.ProfileChangeAdapter
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
    // False while every Krit inspection is disabled in the active inspection
    // profile. Automatic scans pause then; explicit Analyze actions still run.
    @Volatile
    private var inspectionsEnabled = true
    private val explicitRequest = AtomicBoolean(false)

    private val documentListener = object : BulkAwareDocumentListener {
        override fun documentChanged(event: DocumentEvent) {
            val file = FileDocumentManager.getInstance().getFile(event.document) ?: return
            if (!inspectionsEnabled) {
                if (shouldTriggerScan(file)) dropFindingsWhilePaused(file.path)
                return
            }
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
            project.messageBus.connect(this).subscribe(
                ProfileChangeAdapter.TOPIC,
                object : ProfileChangeAdapter {
                    override fun profileChanged(profile: InspectionProfile) = onInspectionProfileChanged()
                    override fun profileActivated(oldProfile: InspectionProfile?, profile: InspectionProfile?) =
                        onInspectionProfileChanged()
                },
            )
            inspectionsEnabled = KritInspectionToggle.anyEnabled(project)
            if (inspectionsEnabled) {
                scheduleScan(0)
            } else {
                projectRescanPending = true
            }
        }
    }

    // While scans are paused nothing refreshes an edited file, so findings
    // from an explicit Analyze run would go stale under the user's edits.
    // Supersede any scan still running and drop the edited file's findings on
    // the scan thread, so the drop lands after anything that scan publishes.
    private fun dropFindingsWhilePaused(path: String) {
        scanGeneration.bump()
        try {
            executor.execute {
                val canonical = canonicalPath(path, project.basePath)
                val dropped = findingsByFile.containsKey(canonical)
                if (dropped) {
                    findingsByFile = findingsByFile - canonical
                }
                // A scan the bump cancelled left the widget at Scanning, and
                // no automatic scan will replace it while paused.
                if (dropped || state is KritState.Scanning) {
                    transition(KritState.Idle(findingsByFile.values.sumOf { it.size }))
                }
                if (dropped) restartHighlighting()
            }
        } catch (_: java.util.concurrent.RejectedExecutionException) {
            // Disposed concurrently.
        }
    }

    // Pauses automatic scans (and clears the findings they would have kept
    // fresh) when the last Krit inspection is disabled, and runs a full
    // refresh as soon as one is enabled again, instead of waiting for the
    // next edit.
    private fun onInspectionProfileChanged() {
        if (project.isDisposed) return
        val enabled = KritInspectionToggle.anyEnabled(project)
        val wasEnabled = inspectionsEnabled
        inspectionsEnabled = enabled
        if (!enabled) {
            synchronized(scanLock) {
                pendingScan?.cancel(false)
                pendingScan = null
            }
            // No scan will refresh these while paused, so drop them rather
            // than leave warnings on code the user goes on to edit. The
            // external annotator reads this map regardless of the toggles.
            if (wasEnabled) {
                // Supersede a scan that is already running so it stops at its
                // next cancellation check, and clear on the scan thread so the
                // clear lands after anything that scan still publishes.
                scanGeneration.bump()
                try {
                    executor.execute {
                        findingsByFile = emptyMap()
                        lastReportJson = ""
                        // A scan cancelled by the pause left the widget at
                        // Scanning; nothing replaces it while paused.
                        if (state is KritState.Scanning || state is KritState.Idle) {
                            transition(KritState.Idle(0))
                        }
                        restartHighlighting()
                    }
                } catch (_: java.util.concurrent.RejectedExecutionException) {
                    // Disposed concurrently; nothing left to clear.
                }
            }
            return
        }
        if (!wasEnabled) {
            projectRescanPending = true
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
        explicitRequest.set(true)
        // A non-empty pending set would turn the run into a changed-file
        // refresh; the full scan below covers those files anyway.
        pendingFiles.clear()
        scheduleScan(0)
    }

    fun analyzeFiles(paths: Collection<String>) {
        if (paths.isEmpty()) return
        missingBinaryNotified.set(false)
        explicitRequest.set(true)
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
        // An explicit Analyze action runs even with the inspections off; an
        // automatic scan does not.
        if (!inspectionsEnabled && !explicitRequest.get()) return
        if (project.isDisposed || !running.compareAndSet(false, true)) {
            rerunAfterCurrent.set(true)
            return
        }
        explicitRequest.set(false)
        try {
            val generation = scanGeneration.current()
            val isCancelled = { scanGeneration.isSuperseded(generation) }
            val targets = drainPendingFiles()
            // A project refresh left pending while the inspections were off
            // waits for them to come back on; an explicit Analyze File in the
            // meantime scans only its files.
            val full = targets.isEmpty() || (projectRescanPending && inspectionsEnabled)
            if (full) {
                // The edited files get their own buffer scan just below.
                if (!refreshFindings(isCancelled, skipBuffers = targets)) {
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
    //
    // The project scan reads files from disk, so it then re-analyzes the
    // editor buffers that have unsaved changes and lets their findings win,
    // except for paths in skipBuffers, which the caller scans itself.
    private fun refreshFindings(
        isCancelled: () -> Boolean = { false },
        skipBuffers: Set<String> = emptySet(),
    ): Boolean {
        transition(KritState.Scanning)
        val outcome = KritRunner.analyzeProject(project, isCancelled)
        if (outcome is KritRunner.AnalyzeOutcome.Cancelled || isCancelled()) return false
        when (outcome) {
            is KritRunner.AnalyzeOutcome.Success -> {
                val onDisk = outcome.report.findings.groupBy {
                    canonicalPath(it.file, project.basePath)
                }
                val merged = overlayUnsavedBuffers(onDisk, skipBuffers, isCancelled) ?: return false
                findingsByFile = merged
                lastReportJson = outcome.rawJson
                transition(KritState.Idle(merged.values.sumOf { it.size }))
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

    // Analyzes every unsaved, scannable editor buffer through the daemon and
    // lays the results over onDisk (for the per-file rules the daemon ran;
    // see KritUnsavedBuffers.overlay). Returns null when cancelled. Without a
    // running daemon the buffers can't be analyzed (the CLI reads only files,
    // and saving them for the scan would change the user's files), so onDisk
    // is returned as is.
    private fun overlayUnsavedBuffers(
        onDisk: Map<String, List<KritFinding>>,
        skipBuffers: Set<String>,
        isCancelled: () -> Boolean,
    ): Map<String, List<KritFinding>>? {
        val skip = skipBuffers.mapTo(HashSet()) { canonicalPath(it, project.basePath) }
        val buffers = unsavedBuffers().filterKeys { canonicalPath(it, project.basePath) !in skip }
        if (buffers.isEmpty()) return onDisk
        val scan = KritRunner.scanBuffersWithDaemon(project, buffers, isCancelled) ?: return onDisk
        val byCanonical = scan.outcomes.mapKeys { (path, _) -> canonicalPath(path, project.basePath) }
        return KritUnsavedBuffers.overlay(onDisk, byCanonical, scan.evaluatedRules) {
            canonicalPath(it, project.basePath)
        }
    }

    // Path to text for each open document with unsaved changes that a scan
    // would cover.
    private fun unsavedBuffers(): Map<String, String?> {
        if (project.isDisposed) return emptyMap()
        return ReadAction.compute<Map<String, String?>, RuntimeException> {
            val documents = FileDocumentManager.getInstance()
            documents.unsavedDocuments.mapNotNull { document ->
                val file = documents.getFile(document) ?: return@mapNotNull null
                if (!file.isInLocalFileSystem || !shouldTriggerScan(file)) return@mapNotNull null
                file.path to document.text
            }.toMap()
        }
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
            daemonClient = KritDaemonClient(socketPath)
            daemonClientSocket = socketPath
        }
        return daemonClient
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
