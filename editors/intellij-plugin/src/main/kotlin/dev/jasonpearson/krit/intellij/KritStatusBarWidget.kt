package dev.jasonpearson.krit.intellij

import com.intellij.openapi.components.service
import com.intellij.openapi.project.DumbAware
import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.MessageType
import com.intellij.openapi.ui.popup.Balloon
import com.intellij.openapi.ui.popup.JBPopupFactory
import com.intellij.openapi.wm.StatusBar
import com.intellij.openapi.wm.StatusBarWidget
import com.intellij.openapi.wm.StatusBarWidgetFactory
import com.intellij.ui.awt.RelativePoint
import com.intellij.util.Consumer
import java.awt.event.MouseEvent

class KritStatusBarWidgetFactory : StatusBarWidgetFactory {
    override fun getId(): String = KritStatusBarWidget.ID

    override fun getDisplayName(): String = "Krit"

    override fun isAvailable(project: Project): Boolean = true

    override fun createWidget(project: Project): StatusBarWidget = KritStatusBarWidget(project)

    override fun disposeWidget(widget: StatusBarWidget) {
        widget.dispose()
    }

    override fun canBeEnabledOn(statusBar: StatusBar): Boolean = true
}

class KritStatusBarWidget(private val project: Project) :
    StatusBarWidget, StatusBarWidget.TextPresentation, DumbAware {

    override fun ID(): String = ID

    override fun getPresentation(): StatusBarWidget.WidgetPresentation = this

    override fun install(statusBar: StatusBar) = Unit

    override fun dispose() = Unit

    override fun getText(): String = KritStatusText.render(project.service<KritProjectService>().state)

    override fun getTooltipText(): String =
        KritStatusText.tooltip(project.service<KritProjectService>().state)

    override fun getAlignment(): Float = 0.5f

    override fun getClickConsumer(): Consumer<MouseEvent> = Consumer { event ->
        val state = project.service<KritProjectService>().state
        val type = when (state) {
            is KritState.MissingBinary, is KritState.Error -> MessageType.WARNING
            else -> MessageType.INFO
        }
        JBPopupFactory.getInstance()
            .createHtmlTextBalloonBuilder(KritStatusText.tooltip(state), type, null)
            .setFadeoutTime(5_000)
            .createBalloon()
            .show(RelativePoint(event.component, event.point), Balloon.Position.above)
    }

    companion object {
        const val ID: String = "Krit"
    }
}
