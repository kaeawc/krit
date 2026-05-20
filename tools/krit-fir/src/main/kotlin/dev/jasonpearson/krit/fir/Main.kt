package dev.jasonpearson.krit.fir

import dev.jasonpearson.krit.api.KritFile
import dev.jasonpearson.krit.fir.oracle.FileOffsetTable
import dev.jasonpearson.krit.fir.oracle.OracleResponse
import dev.jasonpearson.krit.fir.plugins.FirOracleResolver
import dev.jasonpearson.krit.fir.plugins.KtFileParser
import dev.jasonpearson.krit.fir.plugins.PluginResponse
import dev.jasonpearson.krit.fir.plugins.PluginRuleRegistry
import dev.jasonpearson.krit.fir.plugins.PluginRuleRunner
import dev.jasonpearson.krit.fir.plugins.ProjectPayloads
import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.BatchResult
import java.io.File as JavaFile
import dev.jasonpearson.krit.fir.runner.FileRef
import dev.jasonpearson.krit.fir.runner.Finding
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

    if (daemon) {
        System.err.println("krit-fir daemon starting...")
        val session = AnalysisSession(emptyList(), emptyList())
        val startTime = System.currentTimeMillis()
        if (port >= 0) {
            runDaemonTcp(port, session, startTime)
        } else {
            runDaemonStdio(session, startTime)
        }
        // Analysis API and kotlinc start non-daemon threads; force
        // exit after clean shutdown.
        exitProcess(0)
    }

    // One-shot CLI:
    //   krit-fir --sources DIR[,DIR...] --output FILE
    //            [--files LIST_FILE] [--classpath JAR[:JAR...]]
    // Mirrors krit-types' one-shot surface so `oracle.InvokeWithFiles`
    // can drive either backend with the same arg vector.
    val sources = extractCliValue(args, "--sources")?.split(",")?.map { it.trim() }?.filter { it.isNotEmpty() }
    val output = extractCliValue(args, "--output", "-o")
    if (sources.isNullOrEmpty() || output.isNullOrBlank()) {
        printOneShotUsage()
        exitProcess(2)
    }
    val classpath = extractCliValue(args, "--classpath", "-cp")
        ?.split(java.io.File.pathSeparator)
        ?.map { it.trim() }
        ?.filter { it.isNotEmpty() }
        .orEmpty()
    runOneShot(
        sources = sources,
        outputPath = output,
        filesListPath = extractCliValue(args, "--files"),
        classpath = classpath,
    )
    exitProcess(0)
}

// `internal` so unit tests in the same module can verify the
// arg-vector parser without driving a JVM subprocess.
internal fun extractCliValue(args: Array<String>, vararg flags: String): String? {
    for ((i, arg) in args.withIndex()) {
        if (arg in flags && i + 1 < args.size) return args[i + 1]
    }
    return null
}

private fun printOneShotUsage() {
    System.err.println(
        """
        |Usage:
        |  krit-fir --daemon [--port N]
        |  krit-fir --sources DIR[,DIR...] --output FILE
        |           [--files LIST_FILE] [--classpath JAR[${java.io.File.pathSeparatorChar}JAR...]]
        """.trimMargin(),
    )
}

private fun runOneShot(
    sources: List<String>,
    outputPath: String,
    filesListPath: String?,
    classpath: List<String>,
) {
    val session = AnalysisSession(sources, classpath)
    val files = if (filesListPath.isNullOrBlank()) {
        emptyList()
    } else {
        // `--files` is a newline-delimited list of absolute paths to
        // restrict analysis to. Same shape krit-types accepts.
        java.io.File(filesListPath).readLines().map { it.trim() }.filter { it.isNotEmpty() }
    }
    val result = session.analyze(files)
    java.io.File(outputPath).writeText(dev.jasonpearson.krit.fir.oracle.OracleResponse.buildOneShot(result))
    System.err.println("Wrote $outputPath")
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
            "analyze", "analyzeAll", "analyzeFiles", "analyzeWithDeps" -> {
                val needsRebuild =
                    request.sourceDirs != session.sourceDirs || request.classpath != session.classpath
                val activeSession = if (needsRebuild) {
                    session.dispose()
                    AnalysisSession(request.sourceDirs, request.classpath)
                } else {
                    session
                }
                val analyzeFiles = if (request.command == "analyzeAll") {
                    emptyList()
                } else {
                    request.files.map { it.path }
                }
                val response = if (request.command == "analyzeWithDeps") {
                    val outcome = activeSession.analyzeFull(analyzeFiles)
                    OracleResponse.buildAnalyzeWithDeps(request.id, outcome.result, outcome.cacheDeps)
                } else {
                    val result = activeSession.analyze(analyzeFiles)
                    OracleResponse.buildAnalyze(request.id, result)
                }
                if (needsRebuild) {
                    RequestResult.SessionRebuilt(response, activeSession)
                } else {
                    RequestResult.Response(response)
                }
            }
            "listPlugins" -> {
                val response = try {
                    PluginRuleRegistry.load(request.pluginJars)
                    PluginResponse.buildListPlugins(
                        id = request.id,
                        descriptors = PluginRuleRegistry.descriptorsForJars(request.pluginJars),
                        diagnostics = PluginRuleRegistry.diagnosticsForJars(request.pluginJars),
                    )
                } catch (t: Throwable) {
                    """{"id":${request.id},"error":"${escJson(t.message ?: "listPlugins failed")}"}"""
                }
                RequestResult.Response(response)
            }
            "analyzeFile" -> {
                val needsRebuild =
                    request.sourceDirs != session.sourceDirs || request.classpath != session.classpath
                val activeSession = if (needsRebuild) {
                    session.dispose()
                    AnalysisSession(request.sourceDirs, request.classpath)
                } else {
                    session
                }
                val response = handleAnalyzeFile(request, activeSession)
                if (needsRebuild) {
                    RequestResult.SessionRebuilt(response, activeSession)
                } else {
                    RequestResult.Response(response)
                }
            }
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
    // Plugin-rule jar paths, matching krit-types' `"jars"` array in
    // `listPlugins` / `analyzeFile` requests.
    val pluginJars: List<String> = emptyList(),
    // analyzeFile (plugin-rule execution) payload.
    val path: String? = null,
    val source: String? = null,
    val ruleIds: List<String>? = null,
    // Project-scope facts shipped on analyzeFile. Each field is null
    // when the corresponding top-level JSON key is absent; the Go
    // side only sends a key when it has facts to ship, so rules that
    // declared the matching `NEEDS_*` capability see the data when
    // it's available and null when the project doesn't have any
    // (e.g. NEEDS_MANIFEST on a pure-Kotlin library).
    val projectPayloads: ProjectPayloads = ProjectPayloads.EMPTY,
)

fun parseRequest(json: String): CheckRequest {
    val id = extractLong(json, "id") ?: throw IllegalArgumentException("Missing 'id' field")
    val command = extractString(json, "command") ?: throw IllegalArgumentException("Missing 'command' field")
    val sourceDirs = extractStringArray(json, "sourceDirs") ?: emptyList()
    val classpath = extractStringArray(json, "classpath") ?: emptyList()
    val rules = extractStringArray(json, "rules") ?: emptyList()
    val pluginJars = extractStringArray(json, "jars") ?: emptyList()
    val ruleIds = extractStringArray(json, "ruleIds")
    val path = extractString(json, "path")
    val source = extractString(json, "source")
    val files = extractFileRefs(json)
    val payloads = if (command == "analyzeFile") ProjectPayloads.parse(json) else ProjectPayloads.EMPTY
    return CheckRequest(id, command, files, sourceDirs, classpath, rules, pluginJars, path, source, ruleIds, payloads)
}

internal fun handleAnalyzeFile(request: CheckRequest, session: AnalysisSession): String {
    val path = request.path
    if (path.isNullOrBlank()) {
        return """{"id":${request.id},"error":"analyzeFile requires path"}"""
    }
    return try {
        PluginRuleRegistry.load(request.pluginJars)
        val text = request.source ?: try {
            JavaFile(path).readText()
        } catch (t: Throwable) {
            return """{"id":${request.id},"error":"analyzeFile could not read source for ${escJson(path)}: ${escJson(t.message ?: "io error")}"}"""
        }
        if (text.isBlank()) {
            return """{"id":${request.id},"error":"analyzeFile could not resolve source for ${escJson(path)}"}"""
        }

        // Run the K2 oracle pass against this file so the resolver
        // has FIR data to answer queries from. The pass populates
        // `expressions` keyed by `"line:col"` for every resolved
        // function call AND `lambdaSuspendByLineCol` for every
        // visited lambda — the two maps the FirOracleResolver looks
        // up. If the K2 pass crashes we still want plugin rules to
        // run; fall back to empty maps and a resolver that returns
        // null/false.
        val (expressions, lambdaSuspendByKey) = try {
            val canonical = JavaFile(path).canonicalPath
            val result = session.analyze(listOf(path))
            val filePayload = result.files[canonical] ?: result.files[path]
            (filePayload?.expressions ?: emptyMap<String, dev.jasonpearson.krit.fir.oracle.ExpressionPayload>()) to
                (filePayload?.lambdaSuspendByLineCol ?: emptyMap<String, Boolean>())
        } catch (t: Throwable) {
            System.err.println("analyzeFile oracle pass failed: ${t.message}")
            emptyMap<String, dev.jasonpearson.krit.fir.oracle.ExpressionPayload>() to emptyMap<String, Boolean>()
        }
        val offsets = FileOffsetTable(text)
        val resolver = FirOracleResolver(expressions, offsets, lambdaSuspendByKey)

        // Parse PSI off the request payload (not off disk) so an
        // in-flight edit shipped via `request.source` is reflected
        // in the KtFile the rule sees. The parser's Disposable holds
        // the IntelliJ infrastructure for the request lifetime.
        val parsed = KtFileParser.parse(text, pathHint = path)
        try {
            val file = KritFile(path = path, text = text, ktFile = parsed.ktFile)
            val outcome = PluginRuleRunner.run(
                file = file,
                ruleIds = request.ruleIds,
                ruleConfigs = null,
                resolver = resolver,
                projectPayloads = request.projectPayloads,
            )
            PluginResponse.buildAnalyzeFile(request.id, outcome.findings, outcome.errors)
        } finally {
            parsed.close()
        }
    } catch (t: Throwable) {
        """{"id":${request.id},"error":"${escJson(t.message ?: "analyzeFile failed")}"}"""
    }
}

// ── Response building ─────────────────────────────────────────────────────────

fun buildCheckResponse(result: BatchResult): String {
    val findingsJson = result.findings.joinToString(",") { f ->
        """{"path":${jsonStr(f.path)},"line":${f.line},"col":${f.col},"rule":${jsonStr(f.rule)},"severity":${jsonStr(f.severity)},"message":${jsonStr(f.message)},"confidence":${f.confidence}}"""
    }

    val crashedJson = result.crashed.entries.joinToString(",", "{", "}") { (k, v) ->
        "${jsonStr(k)}:${jsonStr(v)}"
    }

    return """{"id":${result.id},"succeeded":${result.succeeded},"skipped":${result.skipped},"findings":[$findingsJson],"crashed":$crashedJson}"""
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
