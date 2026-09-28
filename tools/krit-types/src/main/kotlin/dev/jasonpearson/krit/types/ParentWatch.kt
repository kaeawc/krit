package dev.jasonpearson.krit.types

import kotlin.concurrent.thread
import kotlin.system.exitProcess

internal const val PARENT_POLL_MILLIS = 2_000L
internal const val EPHEMERAL_IDLE_MILLIS = 2 * 60 * 1_000
internal const val PERSISTENT_IDLE_MILLIS = 30 * 60 * 1_000

internal fun daemonIdleMillis(parentPid: Long?): Int =
    if (parentPid == null) PERSISTENT_IDLE_MILLIS else EPHEMERAL_IDLE_MILLIS

internal class ParentWatch(
    private val isAlive: () -> Boolean,
    private val onDeath: () -> Unit,
    private val pollMillis: Long = PARENT_POLL_MILLIS,
) {
    fun start(): Thread = thread(name = "krit-types-parent-watch", isDaemon = true) {
        while (true) {
            Thread.sleep(pollMillis)
            if (!isAlive()) {
                onDeath()
                return@thread
            }
        }
    }
}

internal fun startParentWatch(parentPid: Long?) {
    if (parentPid == null) return
    ParentWatch(
        isAlive = { ProcessHandle.of(parentPid).map { it.isAlive }.orElse(false) },
        onDeath = { System.err.println("krit-types parent $parentPid exited; stopping daemon"); exitProcess(0) },
    ).start()
}
