package dev.krit.types

import com.intellij.openapi.Disposable
import com.intellij.openapi.util.Disposer
import com.intellij.psi.PsiElement
import org.jetbrains.kotlin.analysis.api.KaExperimentalApi
import org.jetbrains.kotlin.analysis.api.analyze
import org.jetbrains.kotlin.analysis.api.projectStructure.KaSourceModule
import org.jetbrains.kotlin.analysis.api.resolution.KaCall
import org.jetbrains.kotlin.analysis.api.resolution.KaCallableMemberCall
import org.jetbrains.kotlin.analysis.api.resolution.singleFunctionCallOrNull
import org.jetbrains.kotlin.analysis.api.resolution.singleVariableAccessCall
import org.jetbrains.kotlin.analysis.api.resolution.symbol
import org.jetbrains.kotlin.analysis.api.standalone.buildStandaloneAnalysisAPISession
import org.jetbrains.kotlin.analysis.api.symbols.*
import org.jetbrains.kotlin.analysis.api.types.*
import org.jetbrains.kotlin.analysis.project.structure.builder.buildKtLibraryModule
import org.jetbrains.kotlin.analysis.project.structure.builder.buildKtSdkModule
import org.jetbrains.kotlin.analysis.project.structure.builder.buildKtSourceModule
import org.jetbrains.kotlin.platform.jvm.JvmPlatforms
import org.jetbrains.kotlin.psi.*
import java.io.BufferedReader
import java.io.File
import java.io.InputStreamReader
import java.io.PrintWriter
import java.net.ServerSocket
import java.net.SocketTimeoutException
import java.nio.file.FileSystems
import java.nio.file.PathMatcher
import java.nio.file.Paths
import kotlin.io.path.Path
import kotlin.system.exitProcess

fun main(args: Array<String>) {
    val parsed = parseArgs(args) ?: run {
        printUsage()
        exitProcess(2)
    }

    if (parsed.daemon) {
        runDaemon(parsed)
    } else {
        runOneShot(parsed)
    }

    // Force clean JVM exit. The Kotlin Analysis API + IntelliJ platform
    // start non-daemon background threads (Disposer registry, AppExecutorUtil,
    // project environments) that prevent the JVM from exiting naturally after
    // main() returns, even after Disposer.dispose() on the root disposable.
    // Without this call, the JVM can hang for minutes after the output file
    // has been written, because the non-daemon threads keep it alive.
    exitProcess(0)
}

fun runOneShot(parsed: ParsedArgs) {
    val disposable = Disposer.newDisposable("krit-types")
    val perf = KotlinPerf(parsed.timingsOut != null)
    try {
        val json = analyzeAndExport(disposable, parsed, perf)
        if (parsed.output != null) {
            perf.track("kotlinOutputWrite") {
                File(parsed.output).writeText(json)
            }
            System.err.println("Wrote ${parsed.output}")
        } else {
            perf.track("kotlinStdoutWrite") {
                println(json)
            }
        }
    } finally {
        if (parsed.timingsOut != null) {
            try {
                File(parsed.timingsOut).writeText(perf.toJson())
            } catch (e: Exception) {
                System.err.println("Failed to write --timings-out ${parsed.timingsOut}: ${e.message}")
            }
        }
        Disposer.dispose(disposable)
    }
}

fun runDaemon(parsed: ParsedArgs) {
    System.err.println("krit-types daemon starting...")
    var session = DaemonSession.build(parsed)
    val startTime = System.currentTimeMillis()

    if (parsed.port >= 0) {
        runDaemonTcp(parsed, session, startTime)
    } else {
        runDaemonStdio(parsed, session, startTime)
    }
}

fun runDaemonStdio(parsed: ParsedArgs, initialSession: DaemonSession, startTime: Long) {
    var session = initialSession
    System.err.println("Session ready. Waiting for requests on stdin...")
    // Signal ready to the Go client (it reads this line from stdout)
    println("""{"ready":true}""")
    System.out.flush()

    val reader = BufferedReader(InputStreamReader(System.`in`))
    while (true) {
        val line = reader.readLine() ?: break // EOF
        val trimmed = line.trim()
        if (trimmed.isEmpty()) continue

        val result = handleRequestLine(trimmed, session, parsed, startTime, ::println)
        when (result) {
            is RequestResult.Response -> { println(result.json); System.out.flush() }
            is RequestResult.ParseError -> { println(result.json); System.out.flush() }
            is RequestResult.SessionRebuilt -> { session = result.newSession; println(result.json); System.out.flush() }
            is RequestResult.Shutdown -> { println(result.json); System.out.flush(); session.dispose(); System.err.println("Daemon shutting down."); return }
        }
    }

    session.dispose()
    System.err.println("Daemon exiting (stdin closed).")
}

fun runDaemonTcp(parsed: ParsedArgs, initialSession: DaemonSession, startTime: Long) {
    var session = initialSession
    val serverSocket = ServerSocket(parsed.port)
    val actualPort = serverSocket.localPort
    System.err.println("Session ready. TCP server listening on port $actualPort")
    // Signal ready to the Go client with the actual port
    println("""{"ready":true,"port":$actualPort}""")
    System.out.flush()

    // Idle timeout: 30 minutes with no client connection
    serverSocket.soTimeout = 30 * 60 * 1000

    while (true) {
        val client = try {
            serverSocket.accept()
        } catch (_: SocketTimeoutException) {
            System.err.println("Daemon idle timeout, shutting down.")
            break
        }

        System.err.println("Client connected: ${client.remoteSocketAddress}")
        try {
            val reader = BufferedReader(InputStreamReader(client.getInputStream()))
            val writer = PrintWriter(client.getOutputStream(), true)

            var shutdownRequested = false
            while (true) {
                val line = reader.readLine() ?: break // client disconnected
                val trimmed = line.trim()
                if (trimmed.isEmpty()) continue

                val result = handleRequestLine(trimmed, session, parsed, startTime) { /* no stdout output in TCP mode */ }
                when (result) {
                    is RequestResult.Response -> writer.println(result.json)
                    is RequestResult.ParseError -> writer.println(result.json)
                    is RequestResult.SessionRebuilt -> { session = result.newSession; writer.println(result.json) }
                    is RequestResult.Shutdown -> { writer.println(result.json); shutdownRequested = true; break }
                }
            }

            client.close()
            System.err.println("Client disconnected.")

            if (shutdownRequested) {
                System.err.println("Daemon shutting down (shutdown requested).")
                break
            }
        } catch (e: Exception) {
            System.err.println("Error handling client: ${e.message}")
            try { client.close() } catch (_: Exception) {}
        }
    }

    serverSocket.close()
    session.dispose()
    System.err.println("Daemon exited.")
}

sealed class RequestResult {
    data class Response(val json: String) : RequestResult()
    data class ParseError(val json: String) : RequestResult()
    data class SessionRebuilt(val json: String, val newSession: DaemonSession) : RequestResult()
    data class Shutdown(val json: String) : RequestResult()
}

fun handleRequestLine(
    trimmed: String,
    session: DaemonSession,
    parsed: ParsedArgs,
    startTime: Long,
    stdoutWriter: (String) -> Unit
): RequestResult {
    val request = try {
        parseRequest(trimmed)
    } catch (e: Exception) {
        System.err.println("Failed to parse request: ${e.message}")
        return RequestResult.ParseError("""{"id":null,"error":"Parse error: ${escJsonStr(e.message ?: "unknown")}"}""")
    }

    return try {
        when (request.method) {
            "analyze" -> RequestResult.Response(session.handleAnalyze(request))
            "analyzeAll" -> RequestResult.Response(session.handleAnalyzeAll(request))
            "analyzeWithDeps" -> RequestResult.Response(session.handleAnalyzeWithDeps(request))
            "rebuild" -> {
                val start = System.currentTimeMillis()
                session.dispose()
                val newSession = DaemonSession.build(parsed)
                val elapsed = System.currentTimeMillis() - start
                RequestResult.SessionRebuilt(
                    """{"id":${request.id},"result":{"ok":true,"sessionRebuildMs":$elapsed}}""",
                    newSession
                )
            }
            "ping" -> {
                val uptime = System.currentTimeMillis() - startTime
                RequestResult.Response("""{"id":${request.id},"result":{"ok":true,"uptime":$uptime}}""")
            }
            "checkpoint" -> RequestResult.Response(handleCheckpoint(request))
            "shutdown" -> RequestResult.Shutdown("""{"id":${request.id},"result":{"ok":true}}""")
            else -> RequestResult.Response("""{"id":${request.id},"error":"Unknown method: ${escJsonStr(request.method)}"}""")
        }
    } catch (e: Exception) {
        System.err.println("Error handling ${request.method}: ${e.message}")
        RequestResult.Response("""{"id":${request.id},"error":"${escJsonStr(e.message ?: "unknown")}"}""")
    }
}

/**
 * Handle the "checkpoint" JSON-RPC method.  Attempts to create a CRaC checkpoint
 * via reflection so there is no compile-time dependency on jdk.crac.  On non-CRaC
 * JDKs this returns an error response and the Go side silently ignores it.
 */
fun handleCheckpoint(request: DaemonRequest): String {
    return try {
        val coreClass = Class.forName("jdk.crac.Core")
        val checkpointMethod = coreClass.getMethod("checkpointRestore")
        System.err.println("CRaC: Creating checkpoint...")
        checkpointMethod.invoke(null)
        // If we reach here, we were restored from a checkpoint.
        System.err.println("CRaC: Restored from checkpoint")
        """{"id":${request.id},"result":{"ok":true,"restored":true}}"""
    } catch (_: ClassNotFoundException) {
        // Not a CRaC-enabled JDK — expected on most installations.
        """{"id":${request.id},"error":"CRaC not available"}"""
    } catch (e: Exception) {
        val cause = e.cause?.message ?: e.message ?: "unknown"
        System.err.println("CRaC: Checkpoint failed: $cause")
        """{"id":${request.id},"error":"CRaC checkpoint failed: ${escJsonStr(cause)}"}"""
    }
}

// --- Daemon session: holds the Analysis API session and file tracking ---

class DaemonSession(
    private var disposable: Disposable,
    val sourceModule: KaSourceModule,
    val args: ParsedArgs
) {
    // file path -> last analyzed mtime
    private val fileTimestamps = mutableMapOf<String, Long>()

    companion object {
        fun build(args: ParsedArgs): DaemonSession {
            val disposable = Disposer.newDisposable("krit-types-daemon")
            val sourceModule = buildSession(disposable, args)
            return DaemonSession(disposable, sourceModule, args)
        }
    }

    fun dispose() {
        Disposer.dispose(disposable)
    }

    @OptIn(KaExperimentalApi::class)
    fun handleAnalyze(request: DaemonRequest): String {
        val requestedFiles = request.files
        if (requestedFiles.isNullOrEmpty()) {
            return """{"id":${request.id},"result":{"files":{},"dependencies":{}}}"""
        }

        val ktFiles = sourceModule.psiRoots.filterIsInstance<KtFile>()
        val filesToAnalyze = mutableListOf<KtFile>()
        val errors = mutableMapOf<String, String>()

        for (requestedPath in requestedFiles) {
            val resolvedPath = File(requestedPath).canonicalPath
            val ktFile = ktFiles.find { file ->
                file.virtualFilePath == resolvedPath ||
                    file.virtualFilePath.endsWith(requestedPath) ||
                    file.virtualFilePath == requestedPath
            }
            if (ktFile != null) {
                // Check mtime: only re-analyze if changed
                val fileOnDisk = File(ktFile.virtualFilePath)
                val currentMtime = if (fileOnDisk.exists()) fileOnDisk.lastModified() else 0L
                val lastMtime = fileTimestamps[ktFile.virtualFilePath]
                if (lastMtime == null || currentMtime != lastMtime) {
                    filesToAnalyze.add(ktFile)
                    fileTimestamps[ktFile.virtualFilePath] = currentMtime
                }
            } else {
                errors[requestedPath] = "File not found in source module"
            }
        }

        val files = mutableMapOf<String, FileResult>()
        val deps = mutableMapOf<String, ClassResult>()
        val callFilter = request.callFilter ?: args.callFilter

        for (ktFile in filesToAnalyze) {
            try {
                analyzeKtFile(ktFile, files, deps, args.expressions, callFilter = callFilter)
            } catch (e: Exception) {
                errors[ktFile.virtualFilePath] = e.message ?: "Analysis failed"
                System.err.println("Error analyzing ${ktFile.virtualFilePath}: ${e.message}")
            }
        }

        return buildDaemonResponse(request.id, files, deps, errors)
    }

    // handleAnalyzeWithDeps is the cache-aware variant of handleAnalyze. It
    // mirrors handleAnalyze's file-selection + mtime-skip logic exactly but
    // passes a DepTracker into analyzeKtFile so the response envelope also
    // carries the per-file dependency closure + crash markers the Go-side
    // cache layer needs to write fresh entries. The response uses a flat
    // envelope (result / errors / cacheDeps as siblings) rather than the
    // errors-nested-inside-result shape buildDaemonResponse emits for the
    // legacy analyze/analyzeAll methods — cleaner JSON for the new caller.
    @OptIn(KaExperimentalApi::class)
    fun handleAnalyzeWithDeps(request: DaemonRequest): String {
        val requestedFiles = request.files
        val errors = mutableMapOf<String, String>()
        val files = mutableMapOf<String, FileResult>()
        val deps = mutableMapOf<String, ClassResult>()
        val tracker = DepTracker()
        val perf = KotlinPerf(request.timings)
        val callFilter = request.callFilter ?: args.callFilter
        perf.recordCallFilterSummary(callFilter)

        if (requestedFiles.isNullOrEmpty()) {
            return buildDaemonResponseWithDeps(request.id, files, deps, errors, tracker, perf)
        }

        val ktFiles = perf.track("kotlinDaemonPsiRoots") {
            sourceModule.psiRoots.filterIsInstance<KtFile>()
        }
        perf.addInstant(
            "kotlinDaemonRequestSummary",
            mapOf(
                "requested" to requestedFiles.size.toLong(),
                "sessionFiles" to ktFiles.size.toLong()
            )
        )
        val filesToAnalyze = mutableListOf<KtFile>()

        // NO mtime skipping here (intentional difference from handleAnalyze).
        // The Go-side cache layer already did content-hash + closure-fingerprint
        // comparison to identify which files are misses; every file in
        // requestedFiles genuinely needs re-analysis. Skipping any of them
        // would return an empty FileResult, which the Go side would
        // misinterpret as "jar skipped this file" and write a poison entry
        // over the existing real cache entry — silent data corruption.
        perf.track("kotlinDaemonMatchRequestedFiles") {
            for (requestedPath in requestedFiles) {
                val resolvedPath = File(requestedPath).canonicalPath
                val ktFile = ktFiles.find { file ->
                    file.virtualFilePath == resolvedPath ||
                        file.virtualFilePath.endsWith(requestedPath) ||
                        file.virtualFilePath == requestedPath
                }
                if (ktFile != null) {
                    filesToAnalyze.add(ktFile)
                    // Update the timestamp so the legacy handleAnalyze path's
                    // mtime skip logic stays consistent if it's called later on
                    // the same session.
                    val fileOnDisk = File(ktFile.virtualFilePath)
                    if (fileOnDisk.exists()) {
                        fileTimestamps[ktFile.virtualFilePath] = fileOnDisk.lastModified()
                    }
                } else {
                    errors[requestedPath] = "File not found in source module"
                }
            }
        }
        perf.addInstant(
            "kotlinDaemonMatchSummary",
            mapOf(
                "matched" to filesToAnalyze.size.toLong(),
                "missing" to errors.size.toLong()
            )
        )

        var processed = 0
        var skipped = 0
        perf.track("kotlinDaemonAnalyzeFiles") {
            for (ktFile in filesToAnalyze) {
                try {
                    val ok = analyzeKtFile(ktFile, files, deps, args.expressions, tracker, perf, callFilter)
                    if (ok) processed++ else skipped++
                } catch (e: Exception) {
                    skipped++
                    errors[ktFile.virtualFilePath] = e.message ?: "Analysis failed"
                    System.err.println("Error analyzing ${ktFile.virtualFilePath}: ${e.message}")
                }
            }
        }
        perf.addInstant(
            "kotlinDaemonAnalyzeSummary",
            mapOf(
                "files" to filesToAnalyze.size.toLong(),
                "processed" to processed.toLong(),
                "skipped" to skipped.toLong(),
                "outputFiles" to files.size.toLong(),
                "dependencyTypes" to deps.size.toLong()
            )
        )

        return buildDaemonResponseWithDeps(request.id, files, deps, errors, tracker, perf)
    }

    @OptIn(KaExperimentalApi::class)
    fun handleAnalyzeAll(request: DaemonRequest): String {
        val allKtFiles = sourceModule.psiRoots.filterIsInstance<KtFile>()
        val ktFiles = filterExcludedKtFiles(allKtFiles, args.exclude)
        val files = mutableMapOf<String, FileResult>()
        val deps = mutableMapOf<String, ClassResult>()
        val errors = mutableMapOf<String, String>()
        val callFilter = request.callFilter ?: args.callFilter

        System.err.println("Analyzing ${ktFiles.size} files...")

        for (ktFile in ktFiles) {
            try {
                analyzeKtFile(ktFile, files, deps, args.expressions, callFilter = callFilter)
                val fileOnDisk = File(ktFile.virtualFilePath)
                if (fileOnDisk.exists()) {
                    fileTimestamps[ktFile.virtualFilePath] = fileOnDisk.lastModified()
                }
            } catch (e: Exception) {
                errors[ktFile.virtualFilePath] = e.message ?: "Analysis failed"
                System.err.println("Error analyzing ${ktFile.virtualFilePath}: ${e.message}")
            }
        }

        return buildDaemonResponse(request.id, files, deps, errors)
    }
}

// --- Request parsing (minimal JSON-RPC) ---

data class DaemonRequest(
    val id: Long,
    val method: String,
    val files: List<String>? = null,
    val timings: Boolean = false,
    val callFilter: CallFilter? = null
)

fun parseRequest(json: String): DaemonRequest {
    // Minimal JSON parsing without external dependency
    val id = extractJsonLong(json, "id") ?: throw IllegalArgumentException("Missing 'id' field")
    val method = extractJsonString(json, "method") ?: throw IllegalArgumentException("Missing 'method' field")
    val files = extractJsonStringArray(json, "files")
    val timings = extractJsonBoolean(json, "timings") ?: false
    val callFilterNames = extractJsonStringArray(json, "callFilterCalleeNames")
    val callFilter = callFilterNames?.let { CallFilter(enabled = true, calleeNames = it.toSet()) }
    return DaemonRequest(id, method, files, timings, callFilter)
}

fun extractJsonLong(json: String, key: String): Long? {
    val pattern = Regex(""""$key"\s*:\s*(\d+)""")
    return pattern.find(json)?.groupValues?.get(1)?.toLongOrNull()
}

fun extractJsonString(json: String, key: String): String? {
    val pattern = Regex(""""$key"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"""")
    return pattern.find(json)?.groupValues?.get(1)?.replace("\\\"", "\"")?.replace("\\\\", "\\")
}

fun extractJsonStringArray(json: String, key: String): List<String>? {
    val pattern = Regex(""""$key"\s*:\s*\[([^\]]*)\]""")
    val match = pattern.find(json) ?: return null
    val content = match.groupValues[1].trim()
    if (content.isEmpty()) return emptyList()
    return content.split(",").map { it.trim().removeSurrounding("\"") }
}

fun extractJsonBoolean(json: String, key: String): Boolean? {
    val pattern = Regex(""""$key"\s*:\s*(true|false)""")
    return pattern.find(json)?.groupValues?.get(1)?.toBooleanStrictOrNull()
}

// escJsonStr returns the INTERIOR of a JSON string literal (no surrounding
// quotes) with all control characters and metacharacters escaped per
// RFC 8259. Handles all 0x00-0x1F bytes — Kotlin source strings from the
// kotlin/kotlin repo legitimately contain literal tab / CR / bell / etc.
// inside test-fixture string literals, and emitting those raw produces
// JSON that strict parsers (Python's json module, Go's encoding/json)
// reject with "invalid control character in string literal" at decode
// time.
fun escJsonStr(s: String): String {
    val sb = StringBuilder(s.length + 16)
    for (c in s) {
        when (c) {
            '\\' -> sb.append("\\\\")
            '"' -> sb.append("\\\"")
            '\b' -> sb.append("\\b")
            '\u000C' -> sb.append("\\f")
            '\n' -> sb.append("\\n")
            '\r' -> sb.append("\\r")
            '\t' -> sb.append("\\t")
            else -> {
                if (c.code < 0x20) {
                    sb.append("\\u").append("%04x".format(c.code))
                } else {
                    sb.append(c)
                }
            }
        }
    }
    return sb.toString()
}

// --- Perf timing sidecar ---

data class PerfEntry(
    val name: String,
    val durationMs: Long,
    val metrics: Map<String, Long> = emptyMap(),
    val attributes: Map<String, String> = emptyMap(),
    val children: List<PerfEntry> = emptyList()
)

data class FilePerf(
    val path: String,
    val totalNs: Long,
    val analysisSessionNs: Long,
    val declarationsNs: Long,
    val importDepsNs: Long,
    val callCollectNs: Long,
    val callResolveNs: Long,
    val declarations: Long,
    val calls: Long,
    val expressions: Long,
    val ok: Boolean
)

data class CounterSummary(
    var count: Long = 0,
    var durationNs: Long = 0,
    var maxNs: Long = 0
)

data class SlowCallSite(
    val path: String,
    val line: Int,
    val col: Int,
    val callee: String,
    val durationNs: Long,
    val status: String
)

data class CallFilter(
    val enabled: Boolean,
    val calleeNames: Set<String>,
    val targetFqns: Set<String> = emptySet()
) {
    fun shouldResolve(callee: String): Boolean {
        if (!enabled) return true
        return calleeNames.contains(callee)
    }
}

fun loadCallFilter(path: String?): CallFilter? {
    if (path == null) return null
    return try {
        val json = File(path).readText()
        val names = extractJsonStringArray(json, "calleeNames") ?: emptyList()
        val fqns = extractJsonStringArray(json, "targetFqns") ?: emptyList()
        CallFilter(enabled = true, calleeNames = names.toSet(), targetFqns = fqns.toSet())
    } catch (e: Exception) {
        System.err.println("Failed to read --call-filter $path: ${e.message}")
        null
    }
}

fun KotlinPerf.recordCallFilterSummary(filter: CallFilter?) {
    if (filter == null) return
    addInstant(
        "kotlinCallFilterSummary",
        mapOf(
            "enabled" to (if (filter.enabled) 1L else 0L),
            "calleeNames" to filter.calleeNames.size.toLong(),
            "targetFqns" to filter.targetFqns.size.toLong()
        )
    )
}

class KotlinPerf(val enabled: Boolean = false) {
    private val entries = mutableListOf<PerfEntry>()
    private val fileTimings = mutableListOf<FilePerf>()
    private val phaseTotals = linkedMapOf<String, Long>()
    private val counters = linkedMapOf<String, CounterSummary>()
    private val callLatencyBuckets = linkedMapOf(
        "lt1ms" to 0L,
        "1_5ms" to 0L,
        "5_20ms" to 0L,
        "20_100ms" to 0L,
        "gte100ms" to 0L
    )
    private val slowCallSites = mutableListOf<SlowCallSite>()

    fun <T> track(name: String, block: () -> T): T {
        if (!enabled) return block()
        val start = System.nanoTime()
        try {
            return block()
        } finally {
            add(name, System.nanoTime() - start)
        }
    }

    fun add(name: String, durationNs: Long, metrics: Map<String, Long> = emptyMap(), attributes: Map<String, String> = emptyMap()) {
        if (!enabled) return
        entries.add(PerfEntry(name, durationNs / 1_000_000, metrics, attributes))
    }

    fun addInstant(name: String, metrics: Map<String, Long> = emptyMap(), attributes: Map<String, String> = emptyMap()) {
        if (!enabled) return
        entries.add(PerfEntry(name, 0, metrics, attributes))
    }

    fun count(name: String, durationNs: Long = 0) {
        if (!enabled) return
        val c = counters.getOrPut(name) { CounterSummary() }
        c.count++
        c.durationNs += durationNs
        if (durationNs > c.maxNs) c.maxNs = durationNs
    }

    fun addPhaseTotal(name: String, durationNs: Long) {
        if (!enabled) return
        phaseTotals[name] = (phaseTotals[name] ?: 0L) + durationNs
    }

    fun recordCallSite(path: String, line: Int, col: Int, callee: String, durationNs: Long, status: String) {
        if (!enabled) return
        val bucket = when {
            durationNs < 1_000_000L -> "lt1ms"
            durationNs < 5_000_000L -> "1_5ms"
            durationNs < 20_000_000L -> "5_20ms"
            durationNs < 100_000_000L -> "20_100ms"
            else -> "gte100ms"
        }
        callLatencyBuckets[bucket] = (callLatencyBuckets[bucket] ?: 0L) + 1

        if (slowCallSites.size < 25) {
            slowCallSites.add(SlowCallSite(path, line, col, callee.take(160), durationNs, status))
            return
        }
        var minIdx = 0
        var minNs = slowCallSites[0].durationNs
        for (i in 1 until slowCallSites.size) {
            if (slowCallSites[i].durationNs < minNs) {
                minIdx = i
                minNs = slowCallSites[i].durationNs
            }
        }
        if (durationNs > minNs) {
            slowCallSites[minIdx] = SlowCallSite(path, line, col, callee.take(160), durationNs, status)
        }
    }

    fun recordFile(file: FilePerf) {
        if (!enabled) return
        fileTimings.add(file)
        phaseTotals["kotlinFileAnalysisSession"] = (phaseTotals["kotlinFileAnalysisSession"] ?: 0L) + file.analysisSessionNs
        phaseTotals["kotlinFileDeclarations"] = (phaseTotals["kotlinFileDeclarations"] ?: 0L) + file.declarationsNs
        phaseTotals["kotlinFileImportDeps"] = (phaseTotals["kotlinFileImportDeps"] ?: 0L) + file.importDepsNs
        phaseTotals["kotlinFileCallCollect"] = (phaseTotals["kotlinFileCallCollect"] ?: 0L) + file.callCollectNs
        phaseTotals["kotlinFileCallResolve"] = (phaseTotals["kotlinFileCallResolve"] ?: 0L) + file.callResolveNs
    }

    fun toJson(): String {
        if (!enabled) return "[]"
        val all = mutableListOf<PerfEntry>()
        all.addAll(entries)

        val fileCount = fileTimings.size.toLong()
        if (fileCount > 0) {
            for ((name, ns) in phaseTotals) {
                all.add(PerfEntry(name, ns / 1_000_000, mapOf("files" to fileCount)))
            }

            val slow = fileTimings.sortedByDescending { it.totalNs }.take(25).map { f ->
                PerfEntry(
                    f.path,
                    f.totalNs / 1_000_000,
                    metrics = mapOf(
                        "analysisSessionMs" to f.analysisSessionNs / 1_000_000,
                        "declarationsMs" to f.declarationsNs / 1_000_000,
                        "importDepsMs" to f.importDepsNs / 1_000_000,
                        "callCollectMs" to f.callCollectNs / 1_000_000,
                        "callResolveMs" to f.callResolveNs / 1_000_000,
                        "declarations" to f.declarations,
                        "calls" to f.calls,
                        "expressions" to f.expressions
                    ),
                    attributes = mapOf("ok" to f.ok.toString())
                )
            }
            all.add(PerfEntry("kotlinSlowFilesTop25", slow.sumOf { it.durationMs }, children = slow))
        }

        if (counters.isNotEmpty()) {
            for ((name, c) in counters) {
                all.add(
                    PerfEntry(
                        name,
                        c.durationNs / 1_000_000,
                        metrics = mapOf(
                            "count" to c.count,
                            "maxMs" to c.maxNs / 1_000_000
                        )
                    )
                )
            }
        }
        val histogramCount = callLatencyBuckets.values.sum()
        if (histogramCount > 0) {
            all.add(PerfEntry("kotlinCallResolveLatencyHistogram", 0, metrics = callLatencyBuckets))
        }
        if (slowCallSites.isNotEmpty()) {
            val slow = slowCallSites.sortedByDescending { it.durationNs }.map { s ->
                PerfEntry(
                    "${s.path}:${s.line}:${s.col}",
                    s.durationNs / 1_000_000,
                    metrics = mapOf(
                        "line" to s.line.toLong(),
                        "col" to s.col.toLong()
                    ),
                    attributes = mapOf(
                        "file" to s.path,
                        "callee" to s.callee,
                        "status" to s.status
                    )
                )
            }
            all.add(PerfEntry("kotlinCallResolveSlowSitesTop25", slow.sumOf { it.durationMs }, children = slow))
        }

        return buildString {
            append("[")
            all.forEachIndexed { i, entry ->
                if (i > 0) append(",")
                appendPerfEntry(this, entry)
            }
            append("]")
        }
    }
}

fun appendPerfEntry(sb: StringBuilder, entry: PerfEntry) {
    sb.append("{")
    sb.append(""""name":${esc(entry.name)},"durationMs":${entry.durationMs}""")
    if (entry.metrics.isNotEmpty()) {
        sb.append(""","metrics":{""")
        entry.metrics.entries.forEachIndexed { i, (k, v) ->
            if (i > 0) sb.append(",")
            sb.append("${esc(k)}:$v")
        }
        sb.append("}")
    }
    if (entry.attributes.isNotEmpty()) {
        sb.append(""","attributes":{""")
        entry.attributes.entries.forEachIndexed { i, (k, v) ->
            if (i > 0) sb.append(",")
            sb.append("${esc(k)}:${esc(v)}")
        }
        sb.append("}")
    }
    if (entry.children.isNotEmpty()) {
        sb.append(""","children":[""")
        entry.children.forEachIndexed { i, child ->
            if (i > 0) sb.append(",")
            appendPerfEntry(sb, child)
        }
        sb.append("]")
    }
    sb.append("}")
}

// --- Build session (shared between one-shot and daemon) ---

@OptIn(KaExperimentalApi::class)
fun buildSession(disposable: Disposable, args: ParsedArgs): KaSourceModule {
    val platform = JvmPlatforms.defaultJvmPlatform
    lateinit var sourceModule: KaSourceModule

    buildStandaloneAnalysisAPISession(disposable) {
        buildKtModuleProvider {
            this.platform = platform

            val jdk = args.jdkHome?.let { jdkHome ->
                buildKtSdkModule {
                    addBinaryRootsFromJdkHome(Path(jdkHome), isJre = false)
                    this.platform = platform
                    libraryName = "jdk"
                }
            } ?: run {
                buildKtSdkModule {
                    addBinaryRootsFromJdkHome(Path(System.getProperty("java.home")), isJre = true)
                    this.platform = platform
                    libraryName = "jdk"
                }
            }

            val dependencies = buildKtLibraryModule {
                this.platform = platform
                addBinaryRoots(args.classpath.map { Path(it) })
                libraryName = "dependencies"
            }

            sourceModule = buildKtSourceModule {
                addSourceRoots(args.sourceDirs.map { Path(it) })
                this.platform = platform
                moduleName = "source"
                addRegularDependency(jdk)
                addRegularDependency(dependencies)
            }

            addModule(sourceModule)
        }
    }
    return sourceModule
}

/**
 * Filter a list of KtFiles down to the ones NOT matching any exclude glob.
 *
 * Glob semantics follow `java.nio.file.FileSystem.getPathMatcher("glob:...")`.
 * On the default (unix) filesystem that is GNU-ish: `**` matches across
 * directory separators, `*` matches within a single segment, `?` a single
 * character. We match against the file's absolute path string.
 *
 * Logs a single stderr line summarising the patterns and the drop count, so
 * mega-repos like jetbrains/kotlin make their `compiler/testData/...` skip
 * visible in the run output.
 */
@OptIn(KaExperimentalApi::class)
fun filterExcludedKtFiles(ktFiles: List<KtFile>, excludeGlobs: List<String>): List<KtFile> {
    if (excludeGlobs.isEmpty()) {
        System.err.println("Excluding 0 patterns; skipped 0 files.")
        return ktFiles
    }
    val fs = FileSystems.getDefault()
    val matchers: List<PathMatcher> = excludeGlobs.map { fs.getPathMatcher("glob:$it") }
    val kept = ArrayList<KtFile>(ktFiles.size)
    var skipped = 0
    for (f in ktFiles) {
        val path = try { Paths.get(f.virtualFilePath) } catch (_: Exception) { null }
        val excluded = path != null && matchers.any { it.matches(path) }
        if (excluded) skipped++ else kept.add(f)
    }
    System.err.println("Excluding ${excludeGlobs.size} patterns; skipped $skipped files.")
    return kept
}

// --- Daemon JSON response building ---

fun buildDaemonResponse(
    id: Long,
    files: Map<String, FileResult>,
    deps: Map<String, ClassResult>,
    errors: Map<String, String>
): String {
    val sb = StringBuilder()
    sb.append("""{"id":$id,"result":""")
    sb.append(buildJsonCompact(files, deps))
    if (errors.isNotEmpty()) {
        // Insert errors before the closing brace of result
        sb.setLength(sb.length - 1) // remove trailing }
        sb.append(""","errors":{""")
        errors.entries.forEachIndexed { i, (path, msg) ->
            if (i > 0) sb.append(",")
            sb.append("${esc(path)}:${esc(msg)}")
        }
        sb.append("}}")
    }
    sb.append("}")
    return sb.toString()
}

// buildDaemonResponseWithDeps is the cache-aware variant of
// buildDaemonResponse. It emits a FLAT envelope where `result`, `errors`,
// and `cacheDeps` are siblings — unlike the legacy builder which nests
// `errors` inside `result`. The Go-side client (Daemon.AnalyzeWithDeps)
// parses this flat shape into separate (*OracleData, *CacheDepsFile)
// return values.
//
// The cacheDeps field is always emitted, possibly with empty `files`
// and `crashed` maps — its presence is how the Go side detects that
// the daemon speaks the new protocol version.
fun buildDaemonResponseWithDeps(
    id: Long,
    files: Map<String, FileResult>,
    deps: Map<String, ClassResult>,
    errors: Map<String, String>,
    tracker: DepTracker,
    perf: KotlinPerf? = null
): String {
    val sb = StringBuilder()
    val cacheDepsJson = perf?.track("kotlinDaemonCacheDepsJsonBuild") {
        buildCacheDepsJson(tracker)
    } ?: buildCacheDepsJson(tracker)
    sb.append("""{"id":$id,"result":""")
    sb.append(buildJsonCompact(files, deps))
    if (errors.isNotEmpty()) {
        sb.append(""","errors":{""")
        errors.entries.forEachIndexed { i, (path, msg) ->
            if (i > 0) sb.append(",")
            sb.append("${esc(path)}:${esc(msg)}")
        }
        sb.append("}")
    }
    sb.append(""","cacheDeps":""")
    sb.append(cacheDepsJson)
    if (perf != null && perf.enabled) {
        sb.append(""","timings":""")
        sb.append(perf.toJson())
    }
    sb.append("}")
    return sb.toString()
}

fun buildJsonCompact(files: Map<String, FileResult>, deps: Map<String, ClassResult>): String {
    val sb = StringBuilder()
    sb.append("{")
    sb.append(""""version":1,""")
    sb.append(""""kotlinVersion":"${KotlinVersion.CURRENT}",""")

    sb.append(""""files":{""")
    files.entries.forEachIndexed { i, (path, file) ->
        if (i > 0) sb.append(",")
        sb.append("${esc(path)}:{")
        sb.append(""""package":${esc(file.packageName)},""")
        sb.append(""""declarations":[""")
        file.declarations.forEachIndexed { j, cls ->
            if (j > 0) sb.append(",")
            appendClassCompact(sb, cls)
        }
        sb.append("],")
        sb.append(""""expressions":{""")
        file.expressions.entries.forEachIndexed { j, (key, expr) ->
            if (j > 0) sb.append(",")
            sb.append("${esc(key)}:{\"type\":${esc(expr.type)},\"nullable\":${expr.nullable}")
            if (expr.callTarget != null) sb.append(",\"callTarget\":${esc(expr.callTarget)}")
            sb.append("}")
        }
        sb.append("}")
        if (file.diagnostics.isNotEmpty()) {
            sb.append(",")
            sb.append(""""diagnostics":[""")
            file.diagnostics.forEachIndexed { j, d ->
                if (j > 0) sb.append(",")
                sb.append("{\"factoryName\":${esc(d.factoryName)},\"severity\":${esc(d.severity)},\"message\":${esc(d.message)},\"line\":${d.line},\"col\":${d.col}}")
            }
            sb.append("]")
        }
        sb.append("}")
    }
    sb.append("},")

    sb.append(""""dependencies":{""")
    deps.entries.forEachIndexed { i, (fqn, cls) ->
        if (i > 0) sb.append(",")
        sb.append("${esc(fqn)}:")
        appendClassCompact(sb, cls)
    }
    sb.append("}}")
    return sb.toString()
}

fun appendClassCompact(sb: StringBuilder, c: ClassResult) {
    sb.append("{")
    sb.append(""""fqn":${esc(c.fqn)},""")
    sb.append(""""kind":${esc(c.kind)},""")
    sb.append(""""supertypes":[${c.supertypes.joinToString(",") { esc(it) }}],""")
    sb.append(""""isSealed":${c.isSealed},"isData":${c.isData},"isOpen":${c.isOpen},"isAbstract":${c.isAbstract},""")
    sb.append(""""visibility":${esc(c.visibility)},""")
    if (c.annotations.isNotEmpty()) sb.append(""""annotations":[${c.annotations.joinToString(",") { esc(it) }}],""")
    if (c.typeParameters.isNotEmpty()) sb.append(""""typeParameters":[${c.typeParameters.joinToString(",") { esc(it) }}],""")
    sb.append(""""members":[""")
    c.members.forEachIndexed { i, m ->
        if (i > 0) sb.append(",")
        sb.append("{\"name\":${esc(m.name)},\"kind\":${esc(m.kind)},\"returnType\":${esc(m.returnType)},\"nullable\":${m.nullable},\"visibility\":${esc(m.visibility)}")
        if (m.isOverride) sb.append(",\"isOverride\":true")
        if (m.isAbstract) sb.append(",\"isAbstract\":true")
        if (m.params.isNotEmpty()) {
            sb.append(",\"params\":[")
            m.params.forEachIndexed { j, p ->
                if (j > 0) sb.append(",")
                sb.append("{\"name\":${esc(p.name)},\"type\":${esc(p.type)},\"nullable\":${p.nullable}}")
            }
            sb.append("]")
        }
        if (m.annotations.isNotEmpty()) {
            sb.append(",\"annotations\":[${m.annotations.joinToString(",") { esc(it) }}]")
        }
        sb.append("}")
    }
    sb.append("]}")
}

// --- Analyze a single KtFile (shared between one-shot and daemon) ---
//
// Error handling: the Kotlin Analysis API is not stable against every
// possible Kotlin source pattern. Specific files in large repos (e.g.
// kotlin/kotlin) trigger internal FIR builder bugs like
// "FirPropertyImpl with Source origin was instantiated without a source
// element" when a function contains `arr[i]++` or similar
// array-increment patterns. These bubble up as
// KotlinIllegalArgumentExceptionWithAttachments from expression type
// queries.
//
// We can't fix those upstream, so we catch them at three granularities:
//
//   1. Inside the per-expression loop — if one `.expressionType` call
//      crashes, we skip that expression and continue with the next.
//      Most files with a crash-triggering pattern still produce ~99% of
//      their expression data this way.
//   2. Around the class-declaration loop — if `extractClass` crashes,
//      we skip that class and still emit data for other classes in the
//      same file.
//   3. Around the whole `analyze {}` block — if the KaSession itself
//      throws (e.g. session build fails), we skip the whole file and
//      log the path + exception class so the next file can run.
//
// `analyzeKtFile` returns `true` if the file was processed (even partially,
// even if zero declarations/expressions were extracted), `false` if the
// whole file had to be skipped.

// DepTracker collects, per analyzed source file, the set of source-origin
// KtFile paths the analysis touched AND the set of library dependency
// ClassResults recorded via collectDependencySupertypes while analyzing it.
// See analyzeKtFile for how entries get populated.
class DepTracker {
    // path -> set of absolute source file paths whose PSI was touched
    val depPathsByFile: MutableMap<String, LinkedHashSet<String>> = mutableMapOf()
    // path -> per-file ClassResult fragments (deps uniquely observed while
    // analyzing this file). Not necessarily all dependency types in the full
    // run: collectDependencySupertypes short-circuits on global dedup via the
    // shared `deps` map. For cache correctness we collect into a per-file
    // fragment instead so each cache entry is self-contained.
    val perFileDeps: MutableMap<String, MutableMap<String, ClassResult>> = mutableMapOf()
    // path -> short description of the crash (class name + first message line)
    // for files where analyzeKtFile's outer catch fired. Poison-entry markers
    // written for these files let subsequent runs skip the JVM entirely for
    // content-hash-unchanged files that deterministically crash krit-types.
    val crashedFiles: MutableMap<String, String> = mutableMapOf()

    fun recordDepPath(forFile: String, depPath: String) {
        if (depPath == forFile) return
        depPathsByFile.getOrPut(forFile) { LinkedHashSet() }.add(depPath)
    }

    fun recordCrash(forFile: String, error: String) {
        crashedFiles[forFile] = error
    }
}

/**
 * Extract a source-file path from a KaSymbol, if the symbol originates from
 * the source module (as opposed to a .jar/.class binary dep or generated
 * synthetic symbol). Returns null for symbols with no PSI, no containing
 * file, or a containing file outside the source module (library jars).
 *
 * This is the primary dep-closure tracking hook: every time the analysis
 * resolves a type reference to a source-origin class, we can ask the
 * resulting symbol for its source file via `symbol.psi?.containingFile`.
 * The Analysis API surfaces the PSI for source-origin symbols via the
 * standard KaSymbol.psi accessor — no reflection, no wrappers needed.
 */
fun sourceFilePathOf(symbol: KaSymbol?): String? {
    if (symbol == null) return null
    if (symbol.origin != KaSymbolOrigin.SOURCE) return null
    val psi: PsiElement = symbol.psi ?: return null
    val containing = psi.containingFile ?: return null
    val vf = containing.virtualFile ?: return null
    return vf.path
}

@OptIn(KaExperimentalApi::class)
fun analyzeKtFile(
    ktFile: KtFile,
    files: MutableMap<String, FileResult>,
    deps: MutableMap<String, ClassResult>,
    includeExpressions: Boolean,
    depTracker: DepTracker? = null,
    perf: KotlinPerf? = null,
    callFilter: CallFilter? = null
): Boolean {
    val path = ktFile.virtualFilePath
    val fileStart = System.nanoTime()
    var analysisSessionNs = 0L
    var declarationsNs = 0L
    var importDepsNs = 0L
    var callCollectNs = 0L
    var callResolveNs = 0L
    var declarationCount = 0L
    var callCount = 0L
    var expressionCount = 0L
    // Initialize this file's tracker bucket even if we end up recording 0
    // deps — downstream cache writer treats "no entry" as "not analyzed",
    // not "zero deps".
    if (depTracker != null) {
        depTracker.depPathsByFile.getOrPut(path) { LinkedHashSet() }
        depTracker.perFileDeps.getOrPut(path) { mutableMapOf() }
    }
    try {
        val sessionStart = System.nanoTime()
        try {
            analyze(ktFile) {
            val pkg = ktFile.packageFqName.asString()
            val declarations = mutableListOf<ClassResult>()

            val declarationsStart = System.nanoTime()
            for (decl in ktFile.declarations) {
                when (decl) {
                    is KtClassOrObject -> {
                        try {
                            val symbolLookupStart = System.nanoTime()
                            val symbol = decl.symbol as? KaNamedClassSymbol
                            perf?.addPhaseTotal("kotlinDeclarations.symbolLookup", System.nanoTime() - symbolLookupStart)
                            if (symbol == null) continue
                            val extractClassStart = System.nanoTime()
                            val result = extractClass(symbol, perf)
                            perf?.addPhaseTotal("kotlinDeclarations.extractClass", System.nanoTime() - extractClassStart)
                            declarations.add(result)
                            val depStart = System.nanoTime()
                            collectDependencySupertypes(symbol, deps, depTracker, path, perf)
                            perf?.addPhaseTotal("kotlinDeclarations.collectDependencySupertypes", System.nanoTime() - depStart)
                            // Record same-package source siblings as direct
                            // deps: two files in the same package share an
                            // implicit namespace and a top-level change in
                            // one can silently affect the other even without
                            // an import. We approximate "same package source"
                            // via supertype source origins, which the session
                            // resolver will have already walked.
                            val sourceOriginsStart = System.nanoTime()
                            recordSupertypeSourceOrigins(symbol, depTracker, path)
                            perf?.addPhaseTotal("kotlinDeclarations.recordSupertypeSourceOrigins", System.nanoTime() - sourceOriginsStart)
                        } catch (t: Throwable) {
                            // Skip this class but keep going. Preserves any
                            // earlier classes in this file and lets
                            // expression extraction still run.
                            System.err.println(
                                "krit-types: skipping class in $path: " +
                                    "${t.javaClass.simpleName}: ${t.message?.lineSequence()?.firstOrNull() ?: "(no message)"}"
                            )
                        }
                    }
                }
            }
            declarationsNs += System.nanoTime() - declarationsStart
            declarationCount = declarations.size.toLong()

            // Walk imports so same-package and cross-package direct
            // references show up in the dep closure. Import directives
            // resolved via the Analysis API session to a source-origin
            // KaClassLikeSymbol contribute that symbol's containing file
            // path as a dep. This is the primary "direct dependency" signal.
            if (depTracker != null) {
                val importStart = System.nanoTime()
                for (import in ktFile.importDirectives) {
                    val importedFqName = import.importedFqName ?: continue
                    try {
                        val classId = org.jetbrains.kotlin.name.ClassId.topLevel(importedFqName)
                        val sym = findClass(classId) ?: continue
                        sourceFilePathOf(sym)?.let { depTracker.recordDepPath(path, it) }
                    } catch (_: Throwable) {}
                }
                importDepsNs += System.nanoTime() - importStart
            }

            val expressions = mutableMapOf<String, ExpressionResult>()
            if (includeExpressions) {
                val document = ktFile.viewProvider.document
                if (document != null) {
                    // Single-loop walker: only collect KtCallExpressions and use
                    // resolveToCall() against the symbol graph to extract a
                    // fully-qualified call target. The previous three-loop walker
                    // (call / dot-qualified / reference) called
                    // KtExpression.expressionType on every node, which profiled
                    // at 69.6% CPU / 82.8% allocations on Signal-Android because
                    // each call re-enters the FIR body-resolve tower. resolveToCall
                    // shares the lazy-resolve work at call-site granularity and
                    // produces more precise call targets (containing class FQN
                    // instead of just the lexical receiver text).
                    //
                    // The dotExprs and refExprs loops were dropped entirely: their
                    // ExpressionResult output had zero production callers
                    // (Oracle.LookupExpression has no rules consuming it).
                    val callExprs = mutableListOf<KtCallExpression>()
                    val callCollectStart = System.nanoTime()
                    ktFile.accept(object : KtTreeVisitorVoid() {
                        override fun visitCallExpression(expression: KtCallExpression) {
                            super.visitCallExpression(expression)
                            callExprs.add(expression)
                        }
                    })
                    callCollectNs += System.nanoTime() - callCollectStart
                    callCount = callExprs.size.toLong()

                    val callResolveStart = System.nanoTime()
                    for (expr in callExprs) {
                        // Per-expression try-catch: the FIR lazy resolver can
                        // crash while building the containing function's FIR
                        // (see FirPropertyImpl bug above). Catching here lets
                        // us skip the one bad expression and continue. Same
                        // resilience as the old expressionType path.
                        var line = 0
                        var col = 0
                        var callee = ""
                        var status = "unknown"
                        val siteStart = System.nanoTime()
                        try {
                            val offset = expr.textRange.startOffset
                            line = document.getLineNumber(offset) + 1
                            col = offset - document.getLineStartOffset(line - 1) + 1
                            val key = "$line:$col"
                            if (key in expressions) {
                                perf?.count("kotlinCallResolveDuplicate")
                                status = "duplicate"
                                continue
                            }
                            callee = expr.calleeExpression?.text ?: ""
                            perf?.count("kotlinCallResolveAttempt")
                            if (callFilter != null && !callFilter.shouldResolve(callee)) {
                                perf?.count("kotlinCallResolveSkippedByFilter")
                                status = "skipped-filter"
                                continue
                            }
                            perf?.count("kotlinCallResolveAttempted")

                            // Primary: resolve against the symbol graph for a
                            // fully-qualified callable FQN. This is the only
                            // value the downstream rules consume (coroutines
                            // knownSuspendFQNs, deprecation LookupAnnotations).
                            var callTarget: String? = null
                            var fallbackReason = "unresolved"
                            val resolveStart = System.nanoTime()
                            val callInfo = expr.resolveToCall()
                            val resolveNs = System.nanoTime() - resolveStart
                            perf?.count("kotlinCallResolveResolveToCall", resolveNs)
                            if (callInfo != null) {
                                perf?.count("kotlinCallResolveNonNull")
                                val memberCall: KaCallableMemberCall<*, *>? =
                                    callInfo.singleFunctionCallOrNull()
                                        ?: callInfo.singleVariableAccessCall()
                                if (memberCall != null) {
                                    perf?.count("kotlinCallResolveMemberCall")
                                    val symbol = memberCall.partiallyAppliedSymbol.symbol
                                    val cid = symbol.callableId
                                    if (cid != null) {
                                        perf?.count("kotlinCallResolveCallableId")
                                        val fqn = cid.asSingleFqName().asString()
                                        if (fqn.isNotEmpty()) {
                                            callTarget = fqn
                                            status = "resolved"
                                            perf?.count("kotlinCallResolveResolved")
                                        } else {
                                            fallbackReason = "empty-callable-id"
                                        }
                                    } else {
                                        perf?.count("kotlinCallResolveNoCallableId")
                                        fallbackReason = "no-callable-id"
                                    }
                                } else {
                                    perf?.count("kotlinCallResolveNoMember")
                                    fallbackReason = "no-member-call"
                                }
                            } else {
                                perf?.count("kotlinCallResolveNull")
                                fallbackReason = "resolve-null"
                            }
                            // Fallback: lexical callee text. Preserves parity
                            // with the old code for calls the symbol graph
                            // can't resolve (missing classpath, generated
                            // sources, unresolved references) so downstream
                            // oracle lookups still have something to match on.
                            // Cheap: just a PSI text read, no type resolution.
                            if (callTarget == null) {
                                callTarget = callee
                                if (callTarget.isNullOrEmpty()) {
                                    perf?.count("kotlinCallResolveNoFallback")
                                    status = "empty-$fallbackReason"
                                    continue
                                }
                                perf?.count("kotlinCallResolveLexicalFallback")
                                perf?.count("kotlinCallResolveFallback")
                                status = "fallback-$fallbackReason"
                            }

                            // type="" and nullable=false preserve the
                            // ExpressionResult shape for schema stability. The
                            // type/nullable fields had no production consumers;
                            // only callTarget is read downstream.
                            expressions[key] = ExpressionResult(
                                type = "",
                                nullable = false,
                                callTarget = callTarget
                            )
                        } catch (_: Throwable) {
                            perf?.count("kotlinCallResolveException")
                            status = "exception"
                            // Silent: logging per-expression crashes would
                            // swamp stderr on affected repos. The per-file
                            // summary counts how many files were touched.
                        } finally {
                            perf?.recordCallSite(path, line, col, callee, System.nanoTime() - siteStart, status)
                        }
                    }
                    callResolveNs += System.nanoTime() - callResolveStart
                }
            }
            expressionCount = expressions.size.toLong()

        // TODO: Collect compiler diagnostics for unreachable code detection.
        // When implemented, add diagnostic collection here and pass to FileResult:
        //
        //   val diagnostics = mutableListOf<DiagnosticResult>()
        //   for (diagnostic in ktFile.collectDiagnostics(KaDiagnosticCheckerFilter.ONLY_COMMON_CHECKERS)) {
        //       when (diagnostic) {
        //           is KaDiagnosticWithPsi<*> -> {
        //               val offset = diagnostic.psi.textRange.startOffset
        //               val line = document?.getLineNumber(offset)?.plus(1) ?: continue
        //               val col = document?.let { offset - it.getLineStartOffset(line - 1) + 1 } ?: 1
        //               diagnostics.add(DiagnosticResult(
        //                   factoryName = diagnostic.factoryName,
        //                   severity = diagnostic.severity.name,
        //                   message = diagnostic.defaultMessage,
        //                   line = line,
        //                   col = col
        //               ))
        //           }
        //       }
        //   }
        //
        // Then change FileResult construction below to:
        //   files[path] = FileResult(pkg, declarations, expressions, diagnostics)
        //
        // Key diagnostic factory names consumed by Go rules:
        //   - UNREACHABLE_CODE — unreachable code after return/throw/etc.
        //   - USELESS_ELVIS — useless elvis operator (right side is dead code)

            // Always emit a FileResult, even if the file contributes zero
            // declarations and zero expressions. The empty entry tells the
            // cache layer "this file was analyzed and produced nothing" so
            // a subsequent warm run can serve it from the cache instead of
            // relaunching the JVM. Without this, legitimately-empty files
            // (package-only, comments-only, object initializers at file
            // scope) become permanent cache misses and pin the warm-no-edit
            // wall time at the JVM+session cold-start cost (~4 s).
            files[path] = FileResult(pkg, declarations, expressions)
            }
        } finally {
            analysisSessionNs += System.nanoTime() - sessionStart
        }
        perf?.recordFile(
            FilePerf(
                path,
                System.nanoTime() - fileStart,
                analysisSessionNs,
                declarationsNs,
                importDepsNs,
                callCollectNs,
                callResolveNs,
                declarationCount,
                callCount,
                expressionCount,
                true
            )
        )
        return true
    } catch (t: Throwable) {
        // Per-file fallback: a crash that escaped the inner per-expression /
        // per-class handlers (e.g. session-level corruption, OOM after partial
        // FIR state, Analysis API throwing from its own internal checks). Log
        // one line of context and let the caller continue with the next file.
        // Record the crash in the DepTracker so the Go cache layer can write
        // a poison-entry marker; subsequent runs with the same content hash
        // skip the JVM entirely for this file.
        val firstMsg = t.message?.lineSequence()?.firstOrNull() ?: "(no message)"
        System.err.println(
            "krit-types: skipping $path: ${t.javaClass.simpleName}: $firstMsg"
        )
        depTracker?.recordCrash(path, "${t.javaClass.simpleName}: $firstMsg")
        perf?.recordFile(
            FilePerf(
                path,
                System.nanoTime() - fileStart,
                analysisSessionNs,
                declarationsNs,
                importDepsNs,
                callCollectNs,
                callResolveNs,
                declarationCount,
                callCount,
                expressionCount,
                false
            )
        )
        return false
    }
}

data class ParsedArgs(
    val sourceDirs: List<String>,
    val classpath: List<String>,
    val jdkHome: String?,
    val output: String?,
    val expressions: Boolean = true,
    val daemon: Boolean = false,
    val port: Int = -1,  // -1 = stdin/stdout mode, 0 = auto-assign TCP, >0 = specific port
    val exclude: List<String> = DEFAULT_EXCLUDE_GLOBS,
    val filesList: String? = null,      // --files LISTFILE: restrict analyze to these paths
    val cacheDepsOut: String? = null,   // --cache-deps-out PATH: emit per-file dep-closure JSON
    val timingsOut: String? = null,     // --timings-out PATH: emit perf.TimingEntry-compatible JSON
    val callFilter: CallFilter? = null  // --call-filter JSON: narrow resolveToCall by lexical callee
)

val DEFAULT_EXCLUDE_GLOBS: List<String> = listOf("**/testData/**", "**/test-resources/**")

fun parseArgs(args: Array<String>): ParsedArgs? {
    var sources: List<String> = emptyList()
    var classpath: List<String> = emptyList()
    var jdkHome: String? = null
    var output: String? = null
    var expressions = true
    var daemon = false
    var port = -1
    var exclude: List<String> = DEFAULT_EXCLUDE_GLOBS
    var filesList: String? = null
    var cacheDepsOut: String? = null
    var timingsOut: String? = null
    var callFilterPath: String? = null

    var i = 0
    while (i < args.size) {
        when (args[i]) {
            "--sources" -> { i++; if (i >= args.size) return null; sources = args[i].split(",").map { it.trim() } }
            "--classpath" -> { i++; if (i >= args.size) return null; classpath = args[i].split(File.pathSeparator).map { it.trim() } }
            "--jdk-home" -> { i++; if (i >= args.size) return null; jdkHome = args[i] }
            "--output", "-o" -> { i++; if (i >= args.size) return null; output = args[i] }
            "--no-expressions" -> { expressions = false }
            "--daemon" -> { daemon = true }
            "--port" -> { i++; if (i >= args.size) return null; port = args[i].toIntOrNull() ?: run { System.err.println("Error: --port requires an integer"); return null } }
            "--exclude" -> {
                i++
                if (i >= args.size) return null
                // Empty string means no exclusion; otherwise split on comma and trim.
                exclude = if (args[i].isEmpty()) {
                    emptyList()
                } else {
                    args[i].split(",").map { it.trim() }.filter { it.isNotEmpty() }
                }
            }
            "--files" -> { i++; if (i >= args.size) return null; filesList = args[i] }
            "--cache-deps-out" -> { i++; if (i >= args.size) return null; cacheDepsOut = args[i] }
            "--timings-out" -> { i++; if (i >= args.size) return null; timingsOut = args[i] }
            "--call-filter" -> { i++; if (i >= args.size) return null; callFilterPath = args[i] }
            "--help", "-h" -> return null
            else -> { System.err.println("Unknown argument: ${args[i]}"); return null }
        }
        i++
    }
    if (sources.isEmpty()) { System.err.println("Error: --sources is required"); return null }
    return ParsedArgs(sources, classpath, jdkHome, output, expressions, daemon, port, exclude, filesList, cacheDepsOut, timingsOut, loadCallFilter(callFilterPath))
}

fun printUsage() {
    System.err.println("""
        |Usage: krit-types [options]
        |
        |Options:
        |  --sources DIR[,DIR]     Kotlin source directories (required)
        |  --classpath JAR[:JAR]   Classpath JARs (optional)
        |  --jdk-home PATH         JDK home directory (optional)
        |  --output FILE           Output file (default: stdout)
        |  --no-expressions        Skip expression-level type export
        |  --daemon                Run in daemon mode (JSON-RPC over stdin/stdout)
        |  --port N                TCP port for daemon (-1=stdin/stdout, 0=auto-assign, >0=specific port)
        |  --exclude GLOB[,GLOB]   Skip files whose paths match any glob (default: **/testData/**,**/test-resources/**; pass "" to disable)
        |  --files LISTFILE        Restrict analysis to absolute paths in LISTFILE (one per line)
        |  --cache-deps-out PATH   Emit per-file dep-closure + per-file-deps JSON alongside --output
        |  --timings-out PATH      Emit perf timing JSON sidecar
        |  --call-filter PATH      JSON callee filter for call-target resolution
        |  --help                  Show this help
    """.trimMargin())
}

@OptIn(KaExperimentalApi::class)
fun analyzeAndExport(disposable: Disposable, args: ParsedArgs, perf: KotlinPerf = KotlinPerf()): String {
    perf.recordCallFilterSummary(args.callFilter)
    val sourceModule = perf.track("kotlinBuildSession") {
        buildSession(disposable, args)
    }

    val files = mutableMapOf<String, FileResult>()
    val deps = mutableMapOf<String, ClassResult>()

    val allKtFiles = perf.track("kotlinPsiRoots") {
        sourceModule.psiRoots.filterIsInstance<KtFile>()
    }
    perf.addInstant("kotlinPsiRootSummary", mapOf("ktFiles" to allKtFiles.size.toLong()))
    val excludedKtFiles = perf.track("kotlinExcludeFilter") {
        filterExcludedKtFiles(allKtFiles, args.exclude)
    }
    perf.addInstant(
        "kotlinExcludeSummary",
        mapOf(
            "before" to allKtFiles.size.toLong(),
            "after" to excludedKtFiles.size.toLong(),
            "patterns" to args.exclude.size.toLong()
        )
    )

    // --files LISTFILE: if set, restrict to the intersection of the source
    // module's KtFiles and the paths in the list. This is the cache-miss
    // re-analysis path used by the Go cache layer.
    val ktFiles: List<KtFile> = if (args.filesList != null) {
        val wanted = HashSet<String>()
        perf.track("kotlinFilesListRead") {
            try {
                File(args.filesList).forEachLine { line ->
                    val t = line.trim()
                    if (t.isNotEmpty()) wanted.add(t)
                }
            } catch (e: Exception) {
                System.err.println("Failed to read --files list ${args.filesList}: ${e.message}")
            }
        }
        val restricted = perf.track("kotlinFilesRestriction") {
            excludedKtFiles.filter { wanted.contains(it.virtualFilePath) }
        }
        perf.addInstant(
            "kotlinFilesRestrictionSummary",
            mapOf(
                "restricted" to restricted.size.toLong(),
                "available" to excludedKtFiles.size.toLong(),
                "requested" to wanted.size.toLong()
            )
        )
        System.err.println("--files: restricting to ${restricted.size} of ${excludedKtFiles.size} files (${wanted.size} requested)")
        restricted
    } else {
        excludedKtFiles
    }

    val total = ktFiles.size
    System.err.println("Analyzing $total files...")

    // Build a DepTracker only when the caller wants the per-file dep
    // closure emitted — avoids paying any cost in the default one-shot path
    // when no cache is in use.
    val tracker: DepTracker? = if (args.cacheDepsOut != null) DepTracker() else null

    // Progress + skip tracking. We log every 5k files so the caller sees
    // forward movement even on large repos, and we print a final summary
    // with succeeded/skipped counts so any per-file FIR crashes are
    // visible in the run output.
    val progressStep = (total / 20).coerceAtLeast(1000)
    var processed = 0
    var skipped = 0
    perf.track("kotlinAnalyzeFiles") {
        for ((i, ktFile) in ktFiles.withIndex()) {
            val ok = analyzeKtFile(ktFile, files, deps, args.expressions, tracker, perf, args.callFilter)
            if (ok) processed++ else skipped++
            if ((i + 1) % progressStep == 0) {
                System.err.println("  ... ${i + 1}/$total (${processed} processed, ${skipped} skipped)")
            }
        }
    }
    perf.addInstant(
        "kotlinAnalyzeSummary",
        mapOf(
            "files" to total.toLong(),
            "processed" to processed.toLong(),
            "skipped" to skipped.toLong(),
            "outputFiles" to files.size.toLong(),
            "dependencyTypes" to deps.size.toLong()
        )
    )
    if (skipped > 0) {
        System.err.println("Analyzed $processed files, skipped $skipped files due to Analysis API errors.")
    } else {
        System.err.println("Analyzed $processed files.")
    }

    // Emit the cache-deps JSON if requested. Keys are source file paths;
    // values are { depPaths: [...], perFileDeps: { fqn: ClassResult, ... } }.
    // The Go side uses depPaths to compute the closure fingerprint, and
    // perFileDeps to make each cache entry a self-contained slice of the
    // dependencies map so cold-start assembly can union without a second
    // pass through the JVM.
    if (tracker != null && args.cacheDepsOut != null) {
        val cacheDepsJson = perf.track("kotlinCacheDepsJsonBuild") {
            buildCacheDepsJson(tracker)
        }
        perf.track("kotlinCacheDepsWrite") {
            File(args.cacheDepsOut).writeText(cacheDepsJson)
        }
        System.err.println("Wrote ${args.cacheDepsOut}")
    }

    return perf.track("kotlinOracleJsonBuild") {
        buildJson(files, deps)
    }
}

// buildCacheDepsJson serializes a DepTracker's recorded per-file
// dependency closures and crash markers into the compact JSON shape
// the Go-side cache layer expects in its `--cache-deps-out` consumer.
//
// Shape:
//   {"version":1,
//    "approximation":"symbol-resolved-sources",
//    "files":{"<path>":{"depPaths":[...],"perFileDeps":{<fqn>:<ClassResult>}}},
//    "crashed":{"<path>":"<error first line>"}}
//
// Shared between the one-shot analyzeAndExport path (--cache-deps-out
// flag) and the forthcoming daemon handleAnalyzeWithDeps path. Kept
// byte-identical to the pre-refactor inlined builder so existing
// Go-side LoadCacheDeps consumers remain unchanged.
fun buildCacheDepsJson(tracker: DepTracker): String {
    val sb = StringBuilder()
    sb.append("{")
    sb.append(""""version":1,""")
    sb.append(""""approximation":"symbol-resolved-sources",""")
    sb.append(""""files":{""")
    var first = true
    for ((filePath, depPaths) in tracker.depPathsByFile) {
        if (!first) sb.append(",") else first = false
        sb.append(esc(filePath)).append(":{")
        sb.append(""""depPaths":[""")
        depPaths.forEachIndexed { j, p ->
            if (j > 0) sb.append(",")
            sb.append(esc(p))
        }
        sb.append("],")
        sb.append(""""perFileDeps":{""")
        val perFile = tracker.perFileDeps[filePath] ?: emptyMap()
        perFile.entries.forEachIndexed { j, (fqn, cls) ->
            if (j > 0) sb.append(",")
            sb.append(esc(fqn)).append(":")
            appendClassCompact(sb, cls)
        }
        sb.append("}}")
    }
    sb.append("},")
    // Crashed files: poison-entry markers for files that deterministically
    // fail analyzeKtFile. Go-side cache writer emits a CacheEntry with
    // Crashed=true for each, and subsequent runs classify them as hits
    // so the JVM never re-analyzes content it already knows crashes.
    sb.append(""""crashed":{""")
    var firstCrash = true
    for ((filePath, err) in tracker.crashedFiles) {
        if (!firstCrash) sb.append(",") else firstCrash = false
        sb.append(esc(filePath)).append(":").append(esc(err))
    }
    sb.append("}}")
    return sb.toString()
}

@OptIn(KaExperimentalApi::class)
fun org.jetbrains.kotlin.analysis.api.KaSession.extractClass(symbol: KaNamedClassSymbol, perf: KotlinPerf? = null): ClassResult {
    val fqn = symbol.classId?.asFqNameString() ?: symbol.name.asString()
    val kind = when {
        symbol.classKind == KaClassKind.INTERFACE && symbol.modality == KaSymbolModality.SEALED -> "sealed interface"
        symbol.classKind == KaClassKind.CLASS && symbol.modality == KaSymbolModality.SEALED -> "sealed class"
        symbol.classKind == KaClassKind.INTERFACE -> "interface"
        symbol.classKind == KaClassKind.ENUM_CLASS -> "enum"
        symbol.classKind == KaClassKind.OBJECT -> "object"
        symbol.classKind == KaClassKind.COMPANION_OBJECT -> "companion object"
        else -> "class"
    }

    val superTypesStart = System.nanoTime()
    val supertypes = symbol.superTypes.mapNotNull { type ->
        (type as? KaClassType)?.classId?.asFqNameString()
    }.filter { it != "kotlin.Any" }
    perf?.addPhaseTotal("kotlinExtractClass.superTypes", System.nanoTime() - superTypesStart)

    val members = mutableListOf<MemberResult>()

    val memberScopeStart = System.nanoTime()
    val memberDecls = symbol.memberScope.declarations.toList()
    perf?.addPhaseTotal("kotlinExtractClass.memberScope", System.nanoTime() - memberScopeStart)

    for (decl in memberDecls) {
        when (decl) {
            is KaNamedFunctionSymbol -> {
                if (decl.origin != KaSymbolOrigin.INTERSECTION_OVERRIDE &&
                    decl.origin != KaSymbolOrigin.SUBSTITUTION_OVERRIDE) {
                    val memberStart = System.nanoTime()
                    members.add(extractFunction(decl, perf))
                    perf?.addPhaseTotal("kotlinExtractClass.memberFunctions", System.nanoTime() - memberStart)
                }
            }
            is KaPropertySymbol -> {
                if (decl.origin != KaSymbolOrigin.INTERSECTION_OVERRIDE &&
                    decl.origin != KaSymbolOrigin.SUBSTITUTION_OVERRIDE) {
                    val memberStart = System.nanoTime()
                    members.add(extractProperty(decl, perf))
                    perf?.addPhaseTotal("kotlinExtractClass.memberProperties", System.nanoTime() - memberStart)
                }
            }
            is KaEnumEntrySymbol -> {
                members.add(MemberResult(
                    name = decl.name.asString(),
                    kind = "enum_entry",
                    returnType = "",
                    nullable = false,
                    visibility = "public"
                ))
            }
            else -> {}
        }
    }

    val annotationsStart = System.nanoTime()
    val annotations = symbol.annotations.mapNotNull { it.classId?.asFqNameString() }
    perf?.addPhaseTotal("kotlinExtractClass.annotations", System.nanoTime() - annotationsStart)

    return ClassResult(
        fqn = fqn,
        kind = kind,
        supertypes = supertypes,
        isSealed = symbol.modality == KaSymbolModality.SEALED,
        isData = symbol.isData,
        isOpen = symbol.modality == KaSymbolModality.OPEN,
        isAbstract = symbol.modality == KaSymbolModality.ABSTRACT,
        visibility = symbol.visibility.name.lowercase(),
        typeParameters = symbol.typeParameters.map { it.name.asString() },
        members = members,
        annotations = annotations
    )
}

@OptIn(KaExperimentalApi::class)
fun org.jetbrains.kotlin.analysis.api.KaSession.extractFunction(symbol: KaNamedFunctionSymbol, perf: KotlinPerf? = null): MemberResult {
    val signatureStart = System.nanoTime()
    val returnType = symbol.returnType.renderType()
    val returnNullable = symbol.returnType.isMarkedNullable
    val params = symbol.valueParameters.map { param ->
        val paramType = param.returnType
        ParamResult(
            name = param.name.asString(),
            type = paramType.renderType(),
            nullable = paramType.isMarkedNullable
        )
    }
    perf?.addPhaseTotal("kotlinExtractFunction.signature", System.nanoTime() - signatureStart)
    val annotationsStart = System.nanoTime()
    val annotations = symbol.annotations.mapNotNull { it.classId?.asFqNameString() }
    perf?.addPhaseTotal("kotlinExtractFunction.annotations", System.nanoTime() - annotationsStart)

    return MemberResult(
        name = symbol.name.asString(),
        kind = "function",
        returnType = returnType,
        nullable = returnNullable,
        visibility = symbol.visibility.name.lowercase(),
        isOverride = symbol.isOverride,
        isAbstract = symbol.modality == KaSymbolModality.ABSTRACT,
        params = params,
        annotations = annotations
    )
}

@OptIn(KaExperimentalApi::class)
fun org.jetbrains.kotlin.analysis.api.KaSession.extractProperty(symbol: KaPropertySymbol, perf: KotlinPerf? = null): MemberResult {
    val annotationsStart = System.nanoTime()
    val annotations = symbol.annotations.mapNotNull { it.classId?.asFqNameString() }
    perf?.addPhaseTotal("kotlinExtractProperty.annotations", System.nanoTime() - annotationsStart)
    val signatureStart = System.nanoTime()
    val returnType = symbol.returnType
    val renderedReturnType = returnType.renderType()
    val returnNullable = returnType.isMarkedNullable
    perf?.addPhaseTotal("kotlinExtractProperty.signature", System.nanoTime() - signatureStart)
    return MemberResult(
        name = symbol.name.asString(),
        kind = "property",
        returnType = renderedReturnType,
        nullable = returnNullable,
        visibility = symbol.visibility.name.lowercase(),
        isOverride = symbol.isOverride,
        isAbstract = symbol.modality == KaSymbolModality.ABSTRACT,
        annotations = annotations
    )
}

@OptIn(KaExperimentalApi::class)
fun org.jetbrains.kotlin.analysis.api.KaSession.renderType(type: KaType): String {
    return when (type) {
        is KaClassType -> type.classId.asFqNameString()
        else -> type.toString()
    }
}

fun KaType.renderType(): String {
    return when (this) {
        is KaClassType -> classId.asFqNameString()
        else -> toString()
    }
}

@OptIn(KaExperimentalApi::class)
fun org.jetbrains.kotlin.analysis.api.KaSession.collectDependencySupertypes(
    symbol: KaNamedClassSymbol,
    deps: MutableMap<String, ClassResult>,
    depTracker: DepTracker? = null,
    forFile: String? = null,
    perf: KotlinPerf? = null
) {
    for (supertype in symbol.superTypes) {
        val superClassType = supertype as? KaClassType ?: continue
        val fqn = superClassType.classId.asFqNameString()
        if (fqn == "kotlin.Any") continue

        val superSymbol = superClassType.symbol as? KaNamedClassSymbol ?: continue

        // For source-origin supertypes, record the path in the tracker so
        // the cache can invalidate when that source file changes.
        if (superSymbol.origin == KaSymbolOrigin.SOURCE) {
            if (depTracker != null && forFile != null) {
                sourceFilePathOf(superSymbol)?.let { depTracker.recordDepPath(forFile, it) }
            }
            continue
        }

        // Library dependency: extract and record both globally (shared deps
        // map for the whole run) and per-file (so each cache entry is
        // self-contained).
        val cls = deps.getOrPut(fqn) { extractClass(superSymbol, perf) }
        if (depTracker != null && forFile != null) {
            depTracker.perFileDeps.getOrPut(forFile) { mutableMapOf() }[fqn] = cls
        }
    }
}

/**
 * Walk a source-origin symbol's supertypes and record any source-origin
 * ancestors in the tracker. This catches the common case where File A
 * declares `class Foo : Bar` and Bar lives in another source file — the
 * usual collectDependencySupertypes path short-circuits on source origin,
 * so we separately record the path here for closure fingerprinting.
 */
@OptIn(KaExperimentalApi::class)
fun org.jetbrains.kotlin.analysis.api.KaSession.recordSupertypeSourceOrigins(
    symbol: KaNamedClassSymbol,
    depTracker: DepTracker?,
    forFile: String
) {
    if (depTracker == null) return
    for (supertype in symbol.superTypes) {
        val superClassType = supertype as? KaClassType ?: continue
        val superSymbol = superClassType.symbol ?: continue
        sourceFilePathOf(superSymbol)?.let { depTracker.recordDepPath(forFile, it) }
    }
    // Also walk property/function return types so cross-file type
    // references (field types, return types) show up in the closure.
    for (decl in symbol.memberScope.declarations) {
        try {
            when (decl) {
                is KaPropertySymbol -> {
                    val t = decl.returnType as? KaClassType
                    val sym = t?.symbol
                    sourceFilePathOf(sym)?.let { depTracker.recordDepPath(forFile, it) }
                }
                is KaNamedFunctionSymbol -> {
                    val rt = decl.returnType as? KaClassType
                    sourceFilePathOf(rt?.symbol)?.let { depTracker.recordDepPath(forFile, it) }
                    for (p in decl.valueParameters) {
                        val pt = p.returnType as? KaClassType
                        sourceFilePathOf(pt?.symbol)?.let { depTracker.recordDepPath(forFile, it) }
                    }
                }
                else -> {}
            }
        } catch (_: Throwable) {
            // FIR lazy resolve can crash on some members; skip and continue.
        }
    }
}

// --- JSON output ---

data class ExpressionResult(val type: String, val nullable: Boolean, val callTarget: String? = null)

// DiagnosticResult holds a compiler diagnostic for JSON export.
// TODO: Populate in analyzeKtFile once KaDiagnosticCheckerFilter is available.
data class DiagnosticResult(
    val factoryName: String,
    val severity: String,
    val message: String,
    val line: Int,
    val col: Int
)

data class FileResult(
    val packageName: String,
    val declarations: List<ClassResult>,
    val expressions: Map<String, ExpressionResult> = emptyMap(),
    val diagnostics: List<DiagnosticResult> = emptyList()
)

data class ClassResult(
    val fqn: String, val kind: String, val supertypes: List<String>,
    val isSealed: Boolean = false, val isData: Boolean = false,
    val isOpen: Boolean = false, val isAbstract: Boolean = false,
    val visibility: String = "public", val typeParameters: List<String> = emptyList(),
    val members: List<MemberResult> = emptyList(),
    val annotations: List<String> = emptyList()
)

data class MemberResult(
    val name: String, val kind: String, val returnType: String,
    val nullable: Boolean = false, val visibility: String = "public",
    val isOverride: Boolean = false, val isAbstract: Boolean = false,
    val params: List<ParamResult> = emptyList(),
    val annotations: List<String> = emptyList()
)

data class ParamResult(val name: String, val type: String, val nullable: Boolean = false)

fun buildJson(files: Map<String, FileResult>, deps: Map<String, ClassResult>): String {
    val sb = StringBuilder()
    sb.appendLine("{")
    sb.appendLine("""  "version": 1,""")
    sb.appendLine("""  "kotlinVersion": "${KotlinVersion.CURRENT}",""")

    sb.appendLine("""  "files": {""")
    files.entries.forEachIndexed { i, (path, file) ->
        sb.appendLine("    ${esc(path)}: {")
        sb.appendLine("      \"package\": ${esc(file.packageName)},")
        sb.appendLine("      \"declarations\": [")
        file.declarations.forEachIndexed { j, cls -> appendClass(sb, cls, "        "); if (j < file.declarations.size - 1) sb.appendLine(",") else sb.appendLine() }
        sb.appendLine("      ],")
        sb.appendLine("      \"expressions\": {")
        file.expressions.entries.forEachIndexed { j, (key, expr) ->
            sb.append("        ${esc(key)}: {\"type\": ${esc(expr.type)}, \"nullable\": ${expr.nullable}")
            if (expr.callTarget != null) sb.append(", \"callTarget\": ${esc(expr.callTarget)}")
            sb.append("}")
            if (j < file.expressions.size - 1) sb.appendLine(",") else sb.appendLine()
        }
        if (file.diagnostics.isNotEmpty()) {
            sb.appendLine("      },")
            sb.appendLine("      \"diagnostics\": [")
            file.diagnostics.forEachIndexed { j, d ->
                sb.append("        {\"factoryName\": ${esc(d.factoryName)}, \"severity\": ${esc(d.severity)}, \"message\": ${esc(d.message)}, \"line\": ${d.line}, \"col\": ${d.col}}")
                if (j < file.diagnostics.size - 1) sb.appendLine(",") else sb.appendLine()
            }
            sb.appendLine("      ]")
        } else {
            sb.appendLine("      }")
        }
        sb.append("    }"); if (i < files.size - 1) sb.appendLine(",") else sb.appendLine()
    }
    sb.appendLine("  },")

    sb.appendLine("""  "dependencies": {""")
    deps.entries.forEachIndexed { i, (fqn, cls) ->
        sb.append("    ${esc(fqn)}: "); appendClass(sb, cls, "    "); if (i < deps.size - 1) sb.appendLine(",") else sb.appendLine()
    }
    sb.appendLine("  }")
    sb.appendLine("}")
    return sb.toString()
}

fun appendClass(sb: StringBuilder, c: ClassResult, ind: String) {
    sb.appendLine("{")
    sb.appendLine("$ind  \"fqn\": ${esc(c.fqn)},")
    sb.appendLine("$ind  \"kind\": ${esc(c.kind)},")
    sb.appendLine("$ind  \"supertypes\": [${c.supertypes.joinToString(", ") { esc(it) }}],")
    sb.appendLine("$ind  \"isSealed\": ${c.isSealed}, \"isData\": ${c.isData}, \"isOpen\": ${c.isOpen}, \"isAbstract\": ${c.isAbstract},")
    sb.appendLine("$ind  \"visibility\": ${esc(c.visibility)},")
    if (c.annotations.isNotEmpty()) sb.appendLine("$ind  \"annotations\": [${c.annotations.joinToString(", ") { esc(it) }}],")
    if (c.typeParameters.isNotEmpty()) sb.appendLine("$ind  \"typeParameters\": [${c.typeParameters.joinToString(", ") { esc(it) }}],")
    sb.appendLine("$ind  \"members\": [")
    c.members.forEachIndexed { i, m ->
        sb.append("$ind    {\"name\": ${esc(m.name)}, \"kind\": ${esc(m.kind)}, \"returnType\": ${esc(m.returnType)}, \"nullable\": ${m.nullable}, \"visibility\": ${esc(m.visibility)}")
        if (m.isOverride) sb.append(", \"isOverride\": true")
        if (m.isAbstract) sb.append(", \"isAbstract\": true")
        if (m.params.isNotEmpty()) {
            sb.append(", \"params\": [")
            m.params.forEachIndexed { j, p -> sb.append("{\"name\": ${esc(p.name)}, \"type\": ${esc(p.type)}, \"nullable\": ${p.nullable}}"); if (j < m.params.size - 1) sb.append(", ") }
            sb.append("]")
        }
        if (m.annotations.isNotEmpty()) {
            sb.append(", \"annotations\": [${m.annotations.joinToString(", ") { esc(it) }}]")
        }
        sb.append("}"); if (i < c.members.size - 1) sb.appendLine(",") else sb.appendLine()
    }
    sb.appendLine("$ind  ]")
    sb.append("$ind}")
}

// esc quotes a value for emission as a JSON string literal (adds surrounding
// quotes). Delegates to escJsonStr for full RFC-8259-compliant escaping of
// control characters — the earlier version only escaped backslash + quote,
// which produced invalid JSON whenever a Kotlin source string contained a
// literal tab / CR / bell / etc.
fun esc(s: String): String = "\"${escJsonStr(s)}\""
