package dev.jasonpearson.krit.fir

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.FileRef
import java.io.File
import java.nio.file.Files

/**
 * `canary` request: compiles one built-in source in a throwaway session and
 * answers with the usual check response. The Go client sends it to a daemon
 * launched from a Leyden AOT cache (#768) before trusting the daemon; a JVM
 * that registers its checkers but reports nothing fails to return the known
 * [CANARY_RULE] finding. The session covers only this source, so the cost
 * does not scale with the project the daemon serves.
 */
internal const val CANARY_RULE = "DefaultLocale"
internal const val CANARY_FILE = "Canary.kt"
// Exactly one DefaultLocale finding: String.format without a Locale.
internal const val CANARY_SOURCE = "package krit.canary\n\nfun canary(): String = String.format(\"%d\", 1)\n"

internal fun handleCanary(id: Long): String {
    val dir = Files.createTempDirectory("krit-fir-canary-").toFile()
    try {
        val file = File(dir, CANARY_FILE)
        file.writeText(CANARY_SOURCE)
        val session = AnalysisSession(listOf(dir.path), emptyList())
        try {
            return buildCheckResponse(session.check(id, listOf(FileRef(file.path)), setOf(CANARY_RULE)))
        } finally {
            session.dispose()
        }
    } finally {
        dir.deleteRecursively()
    }
}
