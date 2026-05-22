package dev.jasonpearson.krit.intellij

import com.google.gson.Gson
import com.google.gson.JsonObject
import com.intellij.openapi.diagnostic.Logger
import java.io.BufferedReader
import java.io.IOException
import java.io.InputStreamReader
import java.net.StandardProtocolFamily
import java.net.UnixDomainSocketAddress
import java.nio.ByteBuffer
import java.nio.channels.Channels
import java.nio.channels.SocketChannel
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import java.nio.file.Path

// Direct Unix-socket client for the krit daemon. Per-call dial — the
// Go-side client (internal/daemon/client.go) uses the same pattern and
// it's plenty fast on Unix sockets. Reusing a long-lived connection is
// a future optimisation if profiling shows the dial cost matters.
//
// Resilience model: any IOException becomes a null return so callers
// can fall back to the CLI invocation path. We do NOT crash the IDE on
// a daemon hiccup.
class KritDaemonClient(private val socketPath: Path) {
    private val log = Logger.getInstance(KritDaemonClient::class.java)
    private val gson = Gson()

    fun available(): Boolean {
        if (!Files.exists(socketPath)) return false
        return try {
            openChannel().use { /* dial only */ }
            true
        } catch (_: IOException) {
            false
        }
    }

    fun analyzeBuffer(filePath: String, content: String): List<KritFinding>? {
        val args = gson.toJsonTree(KritDaemonProtocol.AnalyzeBufferArgs(filePath, content)).asJsonObject
        val data = call(KritDaemonProtocol.VERB_ANALYZE_BUFFER, args) ?: return null
        val payload = gson.fromJson(data, KritDaemonProtocol.AnalyzeBufferData::class.java)
            ?: return emptyList()
        return KritColumnarFindingsDecoder.decode(payload.findings)
    }

    private fun call(verb: String, args: JsonObject?): JsonObject? {
        return try {
            openChannel().use { channel ->
                writeRequest(channel, verb, args)
                readResponse(channel)
            }
        } catch (t: IOException) {
            log.debug("krit daemon call failed for $verb: ${t.message}")
            null
        }
    }

    private fun openChannel(): SocketChannel {
        val channel = SocketChannel.open(StandardProtocolFamily.UNIX)
        try {
            channel.connect(UnixDomainSocketAddress.of(socketPath))
        } catch (t: IOException) {
            channel.close()
            throw t
        }
        return channel
    }

    private fun writeRequest(channel: SocketChannel, verb: String, args: JsonObject?) {
        val request = JsonObject().apply {
            addProperty("verb", verb)
            if (args != null) {
                add("args", args)
            }
        }
        val body = (gson.toJson(request) + "\n").toByteArray(StandardCharsets.UTF_8)
        var buf = ByteBuffer.wrap(body)
        while (buf.hasRemaining()) {
            channel.write(buf)
        }
    }

    private fun readResponse(channel: SocketChannel): JsonObject? {
        // The daemon writes one JSON object per response terminated by \n,
        // but a single analyze-buffer reply can run to tens of KB on a
        // chatty file — read a full line, not a fixed-size buffer.
        val reader = BufferedReader(InputStreamReader(Channels.newInputStream(channel), StandardCharsets.UTF_8))
        val line = reader.readLine() ?: return null
        val response = gson.fromJson(line, KritDaemonProtocol.Response::class.java) ?: return null
        if (!response.ok) {
            log.debug("krit daemon error: ${response.error}")
            return null
        }
        return response.data
    }

    companion object {
        fun socketPathFor(projectDir: java.io.File): Path =
            projectDir.toPath().resolve(".krit").resolve("daemon.sock")
    }
}
