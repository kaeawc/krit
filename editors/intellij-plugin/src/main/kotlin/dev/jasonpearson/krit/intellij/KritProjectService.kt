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
import com.intellij.openapi.fileEditor.FileDocumentManager
import com.intellij.openapi.project.Project
import com.intellij.openapi.roots.ProjectFileIndex
import com.intellij.openapi.vfs.VirtualFile
import com.intellij.openapi.wm.WindowManager
import java.io.File
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
    private val documentListener = object : BulkAwareDocumentListener {
        override fun documentChanged(event: DocumentEvent) {
            val file = FileDocumentManager.getInstance().getFile(event.document) ?: return
            if (shouldTriggerScan(file)) {
                scheduleScan(CHANGE_DEBOUNCE_MS)
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

    init {
        EditorFactory.getInstance().eventMulticaster.addDocumentListener(documentListener, this)
        scheduleScan(0)
    }

    fun findingsFor(path: String): List<KritFinding> {
        return findingsByFile[canonicalPath(path)].orEmpty()
    }

    fun applyFixes(fixLevel: String?) {
        executor.execute {
            if (project.isDisposed || !running.compareAndSet(false, true)) {
                rerunAfterCurrent.set(true)
                return@execute
            }
            try {
                if (KritRunner.fixProject(project, fixLevel)) {
                    refreshFindings()
                }
            } catch (t: Throwable) {
                log.warn("krit fix runner failed", t)
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
            refreshFindings()
        } catch (t: Throwable) {
            log.warn("krit project runner failed", t)
        } finally {
            running.set(false)
            scheduleRequestedRerun()
        }
    }

    private fun refreshFindings() {
        transition(KritState.Scanning)
        when (val outcome = KritRunner.analyzeProject(project)) {
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
        }
        restartHighlighting()
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

    override fun dispose() {
        synchronized(scanLock) {
            pendingScan?.cancel(false)
            pendingScan = null
        }
        executor.shutdownNow()
    }

    private fun shouldTriggerScan(file: VirtualFile): Boolean {
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

    private fun isSupportedSourceFile(file: VirtualFile): Boolean {
        return when (file.extension?.lowercase()) {
            "kt", "kts", "java", "xml", "gradle" -> true
            else -> file.name.endsWith(".gradle.kts")
        }
    }

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
        private const val CHANGE_DEBOUNCE_MS = 1_000L
        private val GENERATED_OR_BUILD_PATH_PARTS = listOf(
            "/.gradle/",
            "/.idea/",
            "/build/",
            "/generated/",
            "/.kotlin/",
        )
    }
}
