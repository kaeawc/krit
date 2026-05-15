package dev.krit.intellij

import com.intellij.codeInsight.daemon.DaemonCodeAnalyzer
import com.intellij.openapi.Disposable
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.diagnostic.Logger
import com.intellij.openapi.project.Project
import java.io.File
import java.util.concurrent.Executors
import java.util.concurrent.ScheduledExecutorService
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean

class KritProjectService(private val project: Project) : Disposable {
    private val log = Logger.getInstance(KritProjectService::class.java)
    private val running = AtomicBoolean(false)
    private val executor: ScheduledExecutorService =
        Executors.newSingleThreadScheduledExecutor { runnable ->
            Thread(runnable, "krit-project-runner").apply { isDaemon = true }
        }

    @Volatile
    private var findingsByFile: Map<String, List<KritFinding>> = emptyMap()

    init {
        executor.scheduleAtFixedRate(::runIfIdle, 0, 5, TimeUnit.SECONDS)
    }

    fun findingsFor(path: String): List<KritFinding> {
        return findingsByFile[canonicalPath(path)].orEmpty()
    }

    fun applyFixes() {
        executor.execute {
            if (project.isDisposed || !running.compareAndSet(false, true)) {
                return@execute
            }
            try {
                if (KritRunner.fixProject(project)) {
                    val report = KritRunner.analyzeProject(project)
                    findingsByFile = report.findings.groupBy { canonicalPath(it.file, project.basePath) }
                    restartHighlighting()
                }
            } catch (t: Throwable) {
                log.warn("krit fix runner failed", t)
            } finally {
                running.set(false)
            }
        }
    }

    private fun runIfIdle() {
        if (project.isDisposed || !running.compareAndSet(false, true)) {
            return
        }
        try {
            val report = KritRunner.analyzeProject(project)
            findingsByFile = report.findings.groupBy { canonicalPath(it.file, project.basePath) }
            restartHighlighting()
        } catch (t: Throwable) {
            log.warn("krit project runner failed", t)
        } finally {
            running.set(false)
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
        executor.shutdownNow()
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
}
