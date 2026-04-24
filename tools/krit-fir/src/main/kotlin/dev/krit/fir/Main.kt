package dev.krit.fir

import dev.krit.fir.runner.AnalysisSession
import dev.krit.fir.runner.BatchResult
import dev.krit.fir.runner.FileRef
import dev.krit.fir.runner.Finding
import java.io.BufferedReader
import java.io.InputStreamReader
import java.io.PrintWriter
import java.net.ServerSocket
import java.net.SocketTimeoutException
import kotlin.system.exitProcess

fun main(args: Array<String>) {
    val daemon = args.contains("--daemon")
    val portIdx = args.indexOf("--port")
    val port = if (portIdx >= 0 && portIdx + 1 < args.size) args[portIdx + 1].toIntOrNull() ?: -1 else -1

    if (!daemon) {
        System.err.println("Usage: krit-fir --daemon [--port N]")
        exitProcess(2)
    }

    System.err.println("krit-fir daemon starting...")
    var session = AnalysisSession(emptyList(), emptyList())
    val startTime = System.currentTimeMillis()

    if (port >= 0) {
        runDaemonTcp(port, session, startTime)
    } else {
        runDaemonStdio(session, startTime)
    }

    // Analysis API and kotlinc start non-daemon threads; force exit after clean shutdown.
    exitProcess(0)
}

// ── Daemon modes ──────────────────────────────────────────────────────────────

fun runDaemonStdio(initialSession: AnalysisSession, startTime: Long) {
    var session = initialSession
    System.err.println("Session ready. Waiting for requests on stdin...")
    println("""{"ready":true}""")
    System.out.flush()

    val reader = BufferedReader(InputStreamReader(System.`in`))
    while (true) {
        val line = reader.readLine() ?: break
        val trimmed = line.trim()
        if (trimmed.isEmpty()) continue

        val result = handleRequestLine(trimmed, session, startTime)
        when (result) {
            is RequestResult.Response -> { println(result.json); System.out.flush() }
            is RequestResult.ParseError -> { println(result.json); System.out.flush() }
            is RequestResult.SessionRebuilt -> {
                session = result.newSession
                println(result.json)
                System.out.flush()
            }
            is RequestResult.Shutdown -> {
                println(result.json)
                System.out.flush()
                session.dispose()
                System.err.println("Daemon shutting down.")
                return
            }
        }
    }
    session.dispose()
    System.err.println("Daemon exiting (stdin closed).")
}

fun runDaemonTcp(port: Int, initialSession: AnalysisSession, startTime: Long) {
    var session = initialSession
    val serverSocket = ServerSocket(port)
    val actualPort = serverSocket.localPort
    System.err.println("Session ready. TCP server listening on port $actualPort")
    println("""{"ready":true,"port":$actualPort}""")
    System.out.flush()

    serverSocket.soTimeout = 30 * 60 * 1000 // 30-minute idle timeout

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
                val line = reader.readLine() ?: break
                val trimmed = line.trim()
                if (trimmed.isEmpty()) continue

                val result = handleRequestLine(trimmed, session, startTime)
                when (result) {
                    is RequestResult.Response -> writer.println(result.json)
                    is RequestResult.ParseError -> writer.println(result.json)
                    is RequestResult.SessionRebuilt -> { session = result.newSession; writer.println(result.json) }
                    is RequestResult.Shutdown -> { writer.println(result.json); shutdownRequested = true; break }
                }
            }
            client.close()
            if (shutdownRequested) { System.err.println("Daemon shutting down."); break }
        } catch (e: Exception) {
            System.err.println("Error handling client: ${e.message}")
            try { client.close() } catch (_: Exception) {}
        }
    }
    serverSocket.close()
    session.dispose()
    System.err.println("Daemon exited.")
}

// ── Request dispatch ──────────────────────────────────────────────────────────

sealed class RequestResult {
    data class Response(val json: String) : RequestResult()
    data class ParseError(val json: String) : RequestResult()
    data class SessionRebuilt(val json: String, val newSession: AnalysisSession) : RequestResult()
    data class Shutdown(val json: String) : RequestResult()
}

fun handleRequestLine(trimmed: String, session: AnalysisSession, startTime: Long): RequestResult {
    val request = try {
        parseRequest(trimmed)
    } catch (e: Exception) {
        System.err.println("Failed to parse request: ${e.message}")
        return RequestResult.ParseError("""{"id":null,"error":"Parse error: ${escJson(e.message ?: "unknown")}"}""")
    }

    return try {
        when (request.command) {
            "check" -> {
                val needsRebuild =
                    request.sourceDirs != session.sourceDirs || request.classpath != session.classpath
                val activeSession = if (needsRebuild) {
                    session.dispose()
                    AnalysisSession(request.sourceDirs, request.classpath)
                } else {
                    session
                }
                val result = activeSession.check(request.id, request.files, request.rules.toSet())
                val response = buildCheckResponse(result)
                if (needsRebuild) {
                    RequestResult.SessionRebuilt(response, activeSession)
                } else {
                    RequestResult.Response(response)
                }
            }
            "rebuild" -> {
                val start = System.currentTimeMillis()
                session.dispose()
                val newSession = AnalysisSession(request.sourceDirs, request.classpath)
                val elapsed = System.currentTimeMillis() - start
                RequestResult.SessionRebuilt(
                    """{"id":${request.id},"result":{"ok":true,"sessionRebuildMs":$elapsed}}""",
                    newSession,
                )
            }
            "ping" -> {
                val uptime = System.currentTimeMillis() - startTime
                RequestResult.Response("""{"id":${request.id},"result":{"ok":true,"uptime":$uptime}}""")
            }
            "shutdown" -> RequestResult.Shutdown("""{"id":${request.id},"result":{"ok":true}}""")
            else -> RequestResult.Response("""{"id":${request.id},"error":"Unknown command: ${escJson(request.command)}"}""")
        }
    } catch (e: Exception) {
        System.err.println("Error handling ${request.command}: ${e.message}")
        RequestResult.Response("""{"id":${request.id},"error":"${escJson(e.message ?: "unknown")}"}""")
    }
}

// ── Request model ─────────────────────────────────────────────────────────────

data class CheckRequest(
    val id: Long,
    val command: String,
    val files: List<FileRef> = emptyList(),
    val sourceDirs: List<String> = emptyList(),
    val classpath: List<String> = emptyList(),
    val rules: List<String> = emptyList(),
)

fun parseRequest(json: String): CheckRequest {
    val id = extractLong(json, "id") ?: throw IllegalArgumentException("Missing 'id' field")
    val command = extractString(json, "command") ?: throw IllegalArgumentException("Missing 'command' field")
    val sourceDirs = extractStringArray(json, "sourceDirs") ?: emptyList()
    val classpath = extractStringArray(json, "classpath") ?: emptyList()
    val rules = extractStringArray(json, "rules") ?: emptyList()
    val files = extractFileRefs(json)
    return CheckRequest(id, command, files, sourceDirs, classpath, rules)
}

// ── Response building ─────────────────────────────────────────────────────────

fun buildCheckResponse(result: BatchResult): String {
    val findingsJson = result.findings.joinToString(",\n    ", prefix = "\n    ", postfix = "\n  ") { f ->
        """{"path":${jsonStr(f.path)},"line":${f.line},"col":${f.col},"rule":${jsonStr(f.rule)},"severity":${jsonStr(f.severity)},"message":${jsonStr(f.message)},"confidence":${f.confidence}}"""
    }.let { if (result.findings.isEmpty()) "" else it }

    val crashedJson = result.crashed.entries.joinToString(",", "{", "}") { (k, v) ->
        "${jsonStr(k)}:${jsonStr(v)}"
    }

    return """{
  "id":${result.id},
  "succeeded":${result.succeeded},
  "skipped":${result.skipped},
  "findings":[${findingsJson}],
  "crashed":$crashedJson
}"""
}

// ── Minimal JSON parsing (no external deps) ───────────────────────────────────

fun extractLong(json: String, key: String): Long? =
    Regex(""""$key"\s*:\s*(\d+)""").find(json)?.groupValues?.get(1)?.toLongOrNull()

fun extractString(json: String, key: String): String? =
    Regex(""""$key"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"""").find(json)
        ?.groupValues?.get(1)?.replace("\\\"", "\"")?.replace("\\\\", "\\")

fun extractStringArray(json: String, key: String): List<String>? {
    val arrayPat = Regex(""""$key"\s*:\s*\[([^\]]*)]""")
    val arrayBody = arrayPat.find(json)?.groupValues?.get(1) ?: return null
    if (arrayBody.isBlank()) return emptyList()
    return Regex(""""([^"\\]*(?:\\.[^"\\]*)*)"""").findAll(arrayBody).map {
        it.groupValues[1].replace("\\\"", "\"").replace("\\\\", "\\")
    }.toList()
}

fun extractFileRefs(json: String): List<FileRef> {
    val filesIdx = json.indexOf("\"files\"")
    if (filesIdx < 0) return emptyList()
    val arrStart = json.indexOf('[', filesIdx)
    if (arrStart < 0) return emptyList()
    var depth = 0
    var arrEnd = arrStart
    for (i in arrStart until json.length) {
        when (json[i]) {
            '[' -> depth++
            ']' -> { depth--; if (depth == 0) { arrEnd = i; break } }
        }
    }
    val arrBody = json.substring(arrStart + 1, arrEnd)
    val objPat = Regex("""\{([^}]*)}""")
    return objPat.findAll(arrBody).map { m ->
        val obj = m.value
        val path = extractString(obj, "path") ?: ""
        val hash = extractString(obj, "contentHash") ?: ""
        FileRef(path, hash)
    }.toList()
}

fun escJson(s: String): String = s.replace("\\", "\\\\").replace("\"", "\\\"")
fun jsonStr(s: String): String = "\"${escJson(s)}\""
