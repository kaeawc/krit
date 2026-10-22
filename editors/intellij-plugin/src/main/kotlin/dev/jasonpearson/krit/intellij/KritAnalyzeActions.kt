package dev.jasonpearson.krit.intellij

import com.intellij.openapi.actionSystem.ActionUpdateThread
import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.CommonDataKeys
import com.intellij.openapi.components.service
import com.intellij.openapi.project.DumbAware
import com.intellij.openapi.vfs.VirtualFile

// "Krit: Analyze File" — scans the file(s) under the caret or selection
// immediately instead of waiting for the document-change debounce. A
// missing krit binary surfaces the standard notification through the
// project service's scan path.
class KritAnalyzeFileAction : AnAction(), DumbAware {
    override fun getActionUpdateThread(): ActionUpdateThread = ActionUpdateThread.BGT

    override fun update(e: AnActionEvent) {
        val visible = e.project != null && supportedFiles(e).isNotEmpty()
        e.presentation.isEnabledAndVisible = visible
    }

    override fun actionPerformed(e: AnActionEvent) {
        val project = e.project ?: return
        val paths = supportedFiles(e).map { it.path }
        project.service<KritProjectService>().analyzeFiles(paths)
    }

    private fun supportedFiles(e: AnActionEvent): List<VirtualFile> {
        val selected = e.getData(CommonDataKeys.VIRTUAL_FILE_ARRAY)?.toList()
            ?: listOfNotNull(e.getData(CommonDataKeys.VIRTUAL_FILE))
        return selected.filter { !it.isDirectory && KritFileFilter.isSupported(it.name) }
    }
}

// "Krit: Analyze Project" — forces an immediate full project scan.
class KritAnalyzeProjectAction : AnAction(), DumbAware {
    override fun getActionUpdateThread(): ActionUpdateThread = ActionUpdateThread.BGT

    override fun update(e: AnActionEvent) {
        e.presentation.isEnabledAndVisible = e.project != null
    }

    override fun actionPerformed(e: AnActionEvent) {
        val project = e.project ?: return
        project.service<KritProjectService>().analyzeNow()
    }
}
