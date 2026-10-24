package dev.jasonpearson.krit.intellij

import com.intellij.openapi.components.PersistentStateComponent
import com.intellij.openapi.components.Service
import com.intellij.openapi.components.State
import com.intellij.openapi.components.Storage
import com.intellij.openapi.components.StoragePathMacros
import com.intellij.openapi.project.Project

// Per-project override of the application-level Krit settings, for people
// who work on projects that pin different krit versions. A blank value means
// "use the application setting". Stored in the workspace file, not a shared
// .idea file, because a binary path is specific to one machine.
@State(name = "KritProjectSettings", storages = [Storage(StoragePathMacros.WORKSPACE_FILE)])
@Service(Service.Level.PROJECT)
class KritProjectSettingsState : PersistentStateComponent<KritProjectSettingsState.Snapshot> {
    data class Snapshot(
        var binaryPath: String = "",
    )

    @Volatile
    private var snapshot: Snapshot = Snapshot()

    override fun getState(): Snapshot = snapshot

    override fun loadState(state: Snapshot) {
        this.snapshot = state
    }

    fun update(next: Snapshot) {
        snapshot = next
    }

    companion object {
        fun get(project: Project): KritProjectSettingsState = project.getService(KritProjectSettingsState::class.java)

        // The project's binary override, or null when unset, the project is
        // gone, or the service can't be reached (platform-free unit tests).
        fun binaryPathFor(project: Project?): String? {
            if (project == null || project.isDisposed) return null
            val state = runCatching { project.getService(KritProjectSettingsState::class.java) }.getOrNull()
            return state?.state?.binaryPath?.takeIf { it.isNotBlank() }
        }
    }
}
