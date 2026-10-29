package dev.jasonpearson.krit.fir

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.BatchResult
import dev.jasonpearson.krit.fir.runner.FileRef
import java.io.File
import java.lang.management.ManagementFactory
import java.nio.file.Files

/**
 * `canary` request: compiles one built-in source in a throwaway session and
 * answers with the usual check response. The Go client sends it to a daemon
 * launched from a Leyden AOT cache (#768) before trusting the daemon; a JVM
 * that registers its checkers but reports nothing fails to return the known
 * [CANARY_RULE] finding. The session covers only this source, so the cost
 * does not scale with the project the daemon serves.
 *
 * With `"oracle":true` (sent for the oracle-backend daemon) the response also
 * carries `oracleCanary`: the oracle facts the same session collected for the
 * canary source. The oracle collectors are checkers of the same
 * [KritFirCheckers] extension, but they also gate on static collector state,
 * so a JVM serving oracle facts proves that path too.
 */
internal const val CANARY_RULE = "DefaultLocale"
internal const val CANARY_FILE = "Canary.kt"
// Exactly one DefaultLocale finding: String.format without a Locale.
internal const val CANARY_SOURCE = "package krit.canary\n\nfun canary(): String = String.format(\"%d\", 1)\n"
internal const val CANARY_PACKAGE = "krit.canary"

internal fun handleCanary(id: Long, oracleFacts: Boolean = false): String = withCanarySession { session, file ->
    val check = buildCheckResponse(session.check(id, listOf(FileRef(file.path)), setOf(CANARY_RULE)))
    if (oracleFacts) check.dropLast(1) + ",\"oracleCanary\":" + canaryOracleFacts(session, file) + "}" else check
}

/** Whether a canary request asks for the oracle facts too. */
internal fun canaryWantsOracleFacts(request: String): Boolean =
    Regex(""""oracle"\s*:\s*true""").containsMatchIn(request)

private fun canaryOracleFacts(session: AnalysisSession, file: File): String = try {
    val result = session.analyze(listOf(file.path))
    val payload = result.files[file.path] ?: result.files.entries.firstOrNull { File(it.key).name == CANARY_FILE }?.value
    """{"packageName":${jsonStr(payload?.packageName.orEmpty())},"expressions":${payload?.expressions?.size ?: 0}}"""
} catch (t: Throwable) {
    """{"error":${jsonStr(t.message ?: t.javaClass.name)}}"""
}

private fun <T> withCanarySession(body: (AnalysisSession, File) -> T): T {
    val dir = Files.createTempDirectory("krit-fir-canary-").toFile()
    try {
        val file = File(dir, CANARY_FILE)
        file.writeText(CANARY_SOURCE)
        val session = AnalysisSession(listOf(dir.path), emptyList())
        try {
            return body(session, file)
        } finally {
            session.dispose()
        }
    } finally {
        dir.deleteRecursively()
    }
}

internal fun canaryReported(result: BatchResult): Boolean =
    result.findings.any { it.rule == CANARY_RULE && File(it.path).name == CANARY_FILE }

/**
 * Self-check for a check that rides on the oracle's compilation (#739): the
 * Go client cannot send that JVM a canary first when it is a one-shot
 * process, so a JVM launched from a Leyden AOT cache runs the canary itself,
 * once, before it shares a check. A JVM whose canary fails does not share,
 * and the Go pass checks on its own (behind its own canary); a passing one
 * marks the shared response with `"aotCanary":true`, which the Go side
 * requires from an AOT launch.
 */
internal object AotSelfCheck {
    val launchedFromCache: Boolean by lazy {
        try {
            ManagementFactory.getRuntimeMXBean().inputArguments.any { it.startsWith("-XX:AOTCache=") }
        } catch (_: Throwable) {
            false
        }
    }

    val canaryPassed: Boolean by lazy {
        try {
            withCanarySession { session, file -> canaryReported(session.check(0, listOf(FileRef(file.path)), setOf(CANARY_RULE))) }
        } catch (t: Throwable) {
            System.err.println("krit-fir: AOT self-check failed: ${t.message}")
            false
        }
    }

    /** Runs the canary now when the JVM needs it, so it never runs mid-compile. */
    fun prepare() {
        if (launchedFromCache) canaryPassed
    }
}

/**
 * The shared check response [json] as it may be returned: unchanged for a
 * JVM launched without an AOT cache, marked when the canary passed, and null
 * (not shared) when it failed.
 */
internal fun markSharedCheck(
    json: String,
    launchedFromCache: Boolean = AotSelfCheck.launchedFromCache,
    canaryPassed: () -> Boolean = { AotSelfCheck.canaryPassed },
): String? = when {
    !launchedFromCache -> json
    canaryPassed() -> json.dropLast(1) + ",\"aotCanary\":true}"
    else -> null
}
