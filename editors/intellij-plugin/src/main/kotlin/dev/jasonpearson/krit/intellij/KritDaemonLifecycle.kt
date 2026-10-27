package dev.jasonpearson.krit.intellij

/**
 * Pure state for the daemon the plugin starts on project open (see
 * KritProjectService). The plugin stops a daemon on project close only when
 * it started that daemon itself, never one the user or the CLI started.
 * Kept free of IntelliJ classes so it is unit-testable without the platform.
 *
 * [T] is whatever the caller needs to stop the daemon later (the krit binary
 * that started it).
 */
class KritDaemonOwnership<T : Any> {
    private var starting = false
    private var owned: T? = null
    private var released = false

    /** True when the caller should go on to start a daemon. */
    @Synchronized
    fun beginStart(): Boolean {
        if (released || starting || owned != null) return false
        starting = true
        return true
    }

    /**
     * Records the outcome of a start begun with [beginStart]: [startedWith] is
     * non-null when this start launched a new daemon. Returns true when the
     * caller must stop that daemon right away because the project closed
     * while it was starting.
     */
    @Synchronized
    fun finishStart(startedWith: T?): Boolean {
        starting = false
        if (startedWith == null) return false
        if (released) return true
        owned = startedWith
        return false
    }

    /**
     * Called once on project close. Returns what the plugin-started daemon
     * needs to be stopped with, or null when the plugin didn't start one.
     */
    @Synchronized
    fun release(): T? {
        released = true
        val stop = owned
        owned = null
        return stop
    }
}

object KritDaemonStart {
    enum class Result { STARTED, ALREADY_RUNNING, FAILED }

    /**
     * Classifies `krit daemon start` (internal/cli/daemoncmd/start.go): it
     * exits 0 both when it launched a daemon and when one was already running
     * for the repo, and says which on stdout.
     */
    fun classify(exitCode: Int, stdout: String): Result = when {
        exitCode != 0 -> Result.FAILED
        stdout.contains("already running") -> Result.ALREADY_RUNNING
        else -> Result.STARTED
    }
}
