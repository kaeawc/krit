package dev.jasonpearson.krit.intellij

import com.intellij.openapi.fileChooser.FileChooserDescriptorFactory
import com.intellij.openapi.options.Configurable
import com.intellij.openapi.ui.ComboBox
import com.intellij.openapi.ui.TextFieldWithBrowseButton
import com.intellij.util.ui.FormBuilder
import javax.swing.JComponent
import javax.swing.JPanel

class KritSettingsConfigurable : Configurable {
    private var form: Form? = null

    override fun getDisplayName(): String = "Krit"

    override fun createComponent(): JComponent {
        val f = Form()
        form = f
        f.load(KritSettingsState.get().state)
        return f.panel
    }

    override fun isModified(): Boolean = form?.isModified(KritSettingsState.get().state) ?: false

    override fun apply() {
        form?.let { KritSettingsState.get().update(it.save()) }
    }

    override fun reset() {
        form?.load(KritSettingsState.get().state)
    }

    override fun disposeUIResources() {
        form = null
    }

    private class Form {
        private val binaryField = TextFieldWithBrowseButton().apply {
            addBrowseFolderListener(
                "Select Krit Binary",
                "Path to the krit executable (leave blank to use KRIT_BINARY or PATH).",
                null,
                FileChooserDescriptorFactory.createSingleFileDescriptor(),
            )
        }
        private val fixLevelCombo = ComboBox(arrayOf("cosmetic", "idiomatic", "semantic"))
        private val configField = TextFieldWithBrowseButton().apply {
            addBrowseFolderListener(
                "Select Krit Config File",
                "Path to a krit.yml override (leave blank to auto-detect).",
                null,
                FileChooserDescriptorFactory.createSingleFileDescriptor(),
            )
        }

        val panel: JPanel = FormBuilder.createFormBuilder()
            .addLabeledComponent("Krit binary:", binaryField)
            .addLabeledComponent("Default fix level:", fixLevelCombo)
            .addLabeledComponent("Config file:", configField)
            .addComponentFillVertically(JPanel(), 0)
            .panel

        fun load(state: KritSettingsState.Snapshot) {
            binaryField.text = state.binaryPath
            fixLevelCombo.selectedItem = state.fixLevel.ifBlank { KritSettingsState.DEFAULT_FIX_LEVEL }
            configField.text = state.configPath
        }

        fun save(): KritSettingsState.Snapshot = KritSettingsState.Snapshot(
            binaryPath = binaryField.text.trim(),
            fixLevel = (fixLevelCombo.selectedItem as? String) ?: KritSettingsState.DEFAULT_FIX_LEVEL,
            configPath = configField.text.trim(),
        )

        fun isModified(state: KritSettingsState.Snapshot): Boolean = save() != state
    }
}
