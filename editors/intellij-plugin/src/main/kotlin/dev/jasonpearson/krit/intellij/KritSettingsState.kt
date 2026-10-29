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
        // Maximum fix level the quick fixes apply (krit --fix-level). Named
        // fixLevel for compatibility with persisted state.
        var fixLevel: String = DEFAULT_FIX_LEVEL,
        // krit config file passed as --config instead of the project's
        // krit.yml / .krit.yml; relative paths resolve against the project
        // root (see KritConfigOverride).
        var configPath: String = "",
        // Persisted state written before this field existed deserialises with
        // the constructor default, so upgrades keep the 1s behaviour.
        var debounceMs: Int = KritScanDebounce.DEFAULT_MS,
        // Start a krit daemon on project open when none is running (and stop
        // it on close). On by default; older persisted state gets the default.
        var startDaemon: Boolean = true,
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

        /** The "Maximum fix level" setting, or the default without the platform. */
        fun maxFixLevel(): String = getSafely()?.state?.fixLevel?.takeIf { it.isNotBlank() } ?: DEFAULT_FIX_LEVEL

        /** The "Config file" setting resolved for a project at [basePath]; see [KritConfigOverride]. */
        fun configOverride(basePath: String?): String? =
            KritConfigOverride.resolve(getSafely()?.state?.configPath, basePath)

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
