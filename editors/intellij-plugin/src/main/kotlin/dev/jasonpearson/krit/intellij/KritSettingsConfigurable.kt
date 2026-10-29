package dev.jasonpearson.krit.intellij

import com.intellij.openapi.fileChooser.FileChooserDescriptorFactory
import com.intellij.openapi.options.Configurable
import com.intellij.openapi.project.ProjectManager
import com.intellij.openapi.ui.ComboBox
import com.intellij.openapi.ui.TextFieldWithBrowseButton
import com.intellij.ui.components.JBCheckBox
import com.intellij.ui.components.JBTextField
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
        val f = form ?: return
        val before = KritSettingsState.get().state.copy()
        val next = f.save()
        KritSettingsState.get().update(next)
        val configChanged = before.configPath != next.configPath
        val binaryChanged = before.binaryPath != next.binaryPath
        if (configChanged || binaryChanged || before.fixLevel != next.fixLevel) {
            for (project in ProjectManager.getInstance().openProjects) {
                if (project.isDisposed) continue
                val service = project.getServiceIfCreated(KritProjectService::class.java) ?: continue
                if (binaryChanged) service.binaryChanged()
                if (configChanged || before.fixLevel != next.fixLevel) service.settingsChanged(configChanged)
            }
        }
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
        private val fixLevelCombo = ComboBox(KritFixLabels.FIX_LEVELS.toTypedArray()).apply {
            toolTipText = "Most invasive krit autofix the quick fixes apply (krit --fix-level): cosmetic, " +
                "idiomatic, or semantic. Fixes above it are not offered. Rule suggestions are always offered."
        }
        private val configField = TextFieldWithBrowseButton().apply {
            addBrowseFolderListener(
                "Select Krit Config File",
                "Path to a krit.yml override (leave blank to auto-detect).",
                null,
                FileChooserDescriptorFactory.createSingleFileDescriptor(),
            )
            toolTipText = "Passed to krit as --config instead of the project's krit.yml / .krit.yml. " +
                "A relative path resolves against each project's root. While set, scans use the krit " +
                "CLI instead of the daemon, which only reads the project's own krit.yml, and files with " +
                "unsaved edits show no Krit findings until they are saved."
        }

        private val debounceField = JBTextField().apply {
            toolTipText = "Delay after the last edit before Krit rescans (minimum " +
                "${KritScanDebounce.MIN_MS} ms, default ${KritScanDebounce.DEFAULT_MS} ms). " +
                "Lower is more responsive but uses more CPU; higher is lighter on large projects."
        }

        private val startDaemonBox = JBCheckBox("Start a krit daemon for this project").apply {
            toolTipText = "When a project opens and no krit daemon is running for it, start one so " +
                "per-file scans skip process startup. Krit stops it again when the project closes; " +
                "a daemon you started yourself is left running."
        }

        val panel: JPanel = FormBuilder.createFormBuilder()
            .addLabeledComponent("Krit binary:", binaryField)
            .addLabeledComponent("Maximum fix level:", fixLevelCombo)
            .addLabeledComponent("Config file:", configField)
            .addLabeledComponent("Scan debounce (ms):", debounceField)
            .addComponent(startDaemonBox)
            .addComponentFillVertically(JPanel(), 0)
            .panel

        fun load(state: KritSettingsState.Snapshot) {
            binaryField.text = state.binaryPath
            fixLevelCombo.selectedItem = state.fixLevel.ifBlank { KritSettingsState.DEFAULT_FIX_LEVEL }
            configField.text = state.configPath
            debounceField.text = KritScanDebounce.clamp(state.debounceMs).toString()
            startDaemonBox.isSelected = state.startDaemon
        }

        fun save(): KritSettingsState.Snapshot = KritSettingsState.Snapshot(
            binaryPath = binaryField.text.trim(),
            fixLevel = (fixLevelCombo.selectedItem as? String) ?: KritSettingsState.DEFAULT_FIX_LEVEL,
            configPath = configField.text.trim(),
            debounceMs = KritScanDebounce.fromInput(debounceField.text),
            startDaemon = startDaemonBox.isSelected,
        )

        fun isModified(state: KritSettingsState.Snapshot): Boolean = save() != state
    }
}
