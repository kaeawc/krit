package dev.jasonpearson.krit.intellij

import com.intellij.openapi.fileChooser.FileChooserDescriptorFactory
import com.intellij.openapi.options.Configurable
import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.TextFieldWithBrowseButton
import com.intellij.ui.components.JBTextField
import com.intellij.util.ui.FormBuilder
import javax.swing.JComponent
import javax.swing.JPanel

// Settings → Tools → Krit → Project: overrides that apply to this project
// only. A blank field inherits the application value, which is shown as the
// field's placeholder.
class KritProjectSettingsConfigurable(private val project: Project) : Configurable {
    private var form: Form? = null

    override fun getDisplayName(): String = "Project"

    override fun createComponent(): JComponent {
        val f = Form()
        form = f
        f.load(KritProjectSettingsState.get(project).state, appBinaryPath())
        return f.panel
    }

    override fun isModified(): Boolean = form?.isModified(KritProjectSettingsState.get(project).state) ?: false

    override fun apply() {
        form?.let { KritProjectSettingsState.get(project).update(it.save()) }
    }

    override fun reset() {
        form?.load(KritProjectSettingsState.get(project).state, appBinaryPath())
    }

    override fun disposeUIResources() {
        form = null
    }

    private fun appBinaryPath(): String = KritSettingsState.getSafely()?.state?.binaryPath.orEmpty()

    private class Form {
        private val binaryField = TextFieldWithBrowseButton().apply {
            addBrowseFolderListener(
                "Select Krit Binary for This Project",
                "Path to the krit executable for this project (leave blank to use the application setting).",
                null,
                FileChooserDescriptorFactory.createSingleFileDescriptor(),
            )
        }

        val panel: JPanel = FormBuilder.createFormBuilder()
            .addLabeledComponent("Krit binary:", binaryField)
            .addComponentFillVertically(JPanel(), 0)
            .panel

        fun load(state: KritProjectSettingsState.Snapshot, appBinaryPath: String) {
            binaryField.text = state.binaryPath
            (binaryField.textField as? JBTextField)?.emptyText?.text =
                appBinaryPath.ifBlank { "Application setting, -Dkrit.binary, KRIT_BINARY or PATH" }
        }

        fun save(): KritProjectSettingsState.Snapshot =
            KritProjectSettingsState.Snapshot(binaryPath = binaryField.text.trim())

        fun isModified(state: KritProjectSettingsState.Snapshot): Boolean = save() != state
    }
}
