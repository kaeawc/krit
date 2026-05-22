package dev.jasonpearson.krit.intellij

import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.components.PersistentStateComponent
import com.intellij.openapi.components.Service
import com.intellij.openapi.components.State
import com.intellij.openapi.components.Storage
import com.intellij.openapi.components.service

@State(name = "KritSettings", storages = [Storage("krit.xml")])
@Service(Service.Level.APP)
class KritSettingsState : PersistentStateComponent<KritSettingsState.Snapshot> {
    data class Snapshot(
        var binaryPath: String = "",
        var fixLevel: String = DEFAULT_FIX_LEVEL,
        var configPath: String = "",
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
        const val DEFAULT_FIX_LEVEL = "idiomatic"

        fun get(): KritSettingsState = ApplicationManager.getApplication().service()

        // Null when the IntelliJ Application isn't initialised — e.g. unit
        // tests that don't bring up the platform. Lets KritBinaryResolver
        // call this safely from its no-arg production default while leaving
        // its unit tests platform-free.
        fun getSafely(): KritSettingsState? {
            val app = ApplicationManager.getApplication() ?: return null
            return runCatching { app.service<KritSettingsState>() }.getOrNull()
        }
    }
}
