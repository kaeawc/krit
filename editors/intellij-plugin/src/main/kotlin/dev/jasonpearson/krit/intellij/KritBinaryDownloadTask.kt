package dev.jasonpearson.krit.intellij

import com.intellij.notification.NotificationAction
import com.intellij.notification.NotificationGroupManager
import com.intellij.notification.NotificationType
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.options.ShowSettingsUtil
import com.intellij.openapi.progress.ProgressIndicator
import com.intellij.openapi.progress.ProgressManager
import com.intellij.openapi.progress.Task
import com.intellij.openapi.project.Project
import com.intellij.openapi.project.ProjectManager
import com.intellij.util.io.HttpRequests
import java.io.File
import java.util.concurrent.atomic.AtomicBoolean

/**
 * Runs the GitHub Releases download the user asked for from the
 * "Krit binary not found" notification. Nothing is downloaded without that
 * click. Requests go through the IDE's HTTP client, so its proxy settings apply.
 */
object KritBinaryDownloadTask {
    private val running = AtomicBoolean(false)

    fun start(project: Project) {
        if (!running.compareAndSet(false, true)) return
        try {
            ProgressManager.getInstance().run(object : Task.Backgroundable(project, "Downloading krit", true) {
                override fun run(indicator: ProgressIndicator) {
                    indicator.isIndeterminate = true
                    val installer = KritBinaryInstaller(IdeHttp(indicator), KritBinaryDownloads.installRoot())
                    finished(project, installer.install())
                }

                override fun onFinished() {
                    running.set(false)
                }
            })
        } catch (t: Throwable) {
            running.set(false)
            throw t
        }
    }

    private fun finished(project: Project, result: KritBinaryInstaller.Result) {
        when (result) {
            is KritBinaryInstaller.Result.Installed -> {
                notify(
                    project,
                    "Krit ${result.version} installed",
                    "Downloaded to ${result.binary.path}. A krit binary set in settings, KRIT_BINARY, or on PATH " +
                        "still takes precedence.",
                    NotificationType.INFORMATION,
                )
                ApplicationManager.getApplication().invokeLater {
                    for (open in ProjectManager.getInstance().openProjects) {
                        if (open.isDisposed) continue
                        val service = open.getServiceIfCreated(KritProjectService::class.java) ?: continue
                        service.binaryChanged()
                        service.analyzeNow()
                    }
                }
            }
            is KritBinaryInstaller.Result.Failed ->
                notify(project, "Krit download failed", result.message, NotificationType.ERROR, settingsAction = true)
        }
    }

    internal fun notify(
        project: Project,
        title: String,
        content: String,
        type: NotificationType,
        settingsAction: Boolean = false,
        download: Boolean = false,
    ) {
        ApplicationManager.getApplication().invokeLater {
            if (project.isDisposed) return@invokeLater
            val notification = NotificationGroupManager.getInstance()
                .getNotificationGroup("Krit")
                .createNotification(title, content, type)
            if (download) {
                notification.addAction(
                    NotificationAction.createSimpleExpiring("Download krit from GitHub") { start(project) },
                )
            }
            if (settingsAction) {
                notification.addAction(NotificationAction.createSimple("Open Krit settings") {
                    ShowSettingsUtil.getInstance().showSettingsDialog(project, KritSettingsConfigurable::class.java)
                })
            }
            notification.notify(project)
        }
    }

    private class IdeHttp(private val indicator: ProgressIndicator) : KritHttp {
        override fun getText(url: String): String =
            HttpRequests.request(url)
                .productNameAsUserAgent()
                .accept("application/vnd.github+json")
                .readString(indicator)

        override fun download(url: String, target: File) {
            indicator.text = "Downloading ${url.substringAfterLast('/')}"
            HttpRequests.request(url).productNameAsUserAgent().saveToFile(target, indicator)
        }
    }
}

/** Missing-binary notification text. Free of IntelliJ classes. */
object KritBinaryMessages {
    const val NOT_FOUND =
        "Set its path in Settings → Tools → Krit → Krit binary, set KRIT_BINARY, or install krit on PATH. " +
            "Or download a krit release from GitHub; the plugin checks it against the release checksums " +
            "and installs it under the IDE's system directory."

    fun configuredMissing(resolution: KritBinaryResolver.Resolution.ConfiguredMissing): String {
        val where = when (resolution.source) {
            KritBinaryResolver.Source.PROJECT_SETTINGS -> "Settings → Tools → Krit → Project → Krit binary"
            KritBinaryResolver.Source.SETTINGS -> "Settings → Tools → Krit → Krit binary"
            KritBinaryResolver.Source.SYSTEM_PROPERTY -> "-Dkrit.binary"
            else -> "KRIT_BINARY"
        }
        return "${resolution.path} (from $where) is missing or not executable. Fix or clear that setting."
    }
}
