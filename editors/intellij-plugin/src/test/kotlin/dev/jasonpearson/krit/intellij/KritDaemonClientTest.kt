package dev.jasonpearson.krit.intellij

import java.io.BufferedReader
import java.io.IOException
import java.io.InputStreamReader
import java.net.StandardProtocolFamily
import java.net.UnixDomainSocketAddress
import java.nio.ByteBuffer
import java.nio.channels.Channels
import java.nio.channels.ServerSocketChannel
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import java.nio.file.Path
import java.util.concurrent.atomic.AtomicReference
import kotlin.test.AfterTest
import kotlin.test.BeforeTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class KritDaemonClientTest {
    private lateinit var socketDir: Path
    private lateinit var socketPath: Path
    private var server: MockDaemonServer? = null

    @BeforeTest
    fun setUp() {
        socketDir = Files.createTempDirectory("krit-daemon-test")
        socketPath = socketDir.resolve("daemon.sock")
    }

    @AfterTest
    fun tearDown() {
        server?.close()
        runCatching { Files.deleteIfExists(socketPath) }
        runCatching { Files.deleteIfExists(socketDir) }
    }

    @Test
    fun `available returns false when socket file is missing`() {
        // No server running, no socket file: caller should fall back.
        assertFalse(KritDaemonClient(socketPath).available())
    }

    @Test
    fun `available returns true when daemon accepts connections`() {
        server = MockDaemonServer.start(socketPath) { _, _ -> okResponse(emptyColumnsPayload()) }
        assertTrue(KritDaemonClient(socketPath).available())
    }

    @Test
    fun `analyzeBuffer decodes a single-finding response from the wire`() {
        // Captures the verb the client actually sent so a mistake in
        // KritDaemonClient (wrong verb, missing newline, etc.) would
        // surface as a verb mismatch here rather than a silent fall-back.
        val seenVerb = AtomicReference<String>()
        server = MockDaemonServer.start(socketPath) { verb, _ ->
            seenVerb.set(verb)
            okResponse(singleFindingColumnsPayload())
        }
        val findings = KritDaemonClient(socketPath).analyzeBuffer("/repo/X.kt", "fun a() {}\n")
        assertEquals(KritDaemonProtocol.VERB_ANALYZE_BUFFER, seenVerb.get())
        assertEquals(1, findings?.size)
        val f = findings!!.single()
        assertEquals("/repo/X.kt", f.file)
        assertEquals("warning", f.severity)
        assertEquals("rule.id", f.rule)
    }

    @Test
    fun `analyzeBuffer returns null when daemon reports error`() {
        // ok=false is the daemon's "I tried and failed" signal; client
        // returns null so the caller falls back to the CLI without
        // surfacing a false-positive empty-findings result.
        server = MockDaemonServer.start(socketPath) { _, _ ->
            """{"ok":false,"error":"oracle init failed"}""" + "\n"
        }
        assertNull(KritDaemonClient(socketPath).analyzeBuffer("/x.kt", ""))
    }

    @Test
    fun `analyzeBuffer returns null when socket is absent`() {
        // Confirms the IOException-to-null translation: no daemon running,
        // dial fails, caller falls back gracefully.
        assertNull(KritDaemonClient(socketPath).analyzeBuffer("/x.kt", ""))
    }

    private fun emptyColumnsPayload(): String =
        """{"findings":{},"cache_hit":false}"""

    private fun singleFindingColumnsPayload(): String =
        """{"findings":{"files":["/repo/X.kt"],"ruleSets":["rs"],"rules":["rule.id"],"messages":["m"],""" +
            """"fileIdx":[0],"line":[1],"col":[1],"ruleSetIdx":[0],"ruleIdx":[0],"severityID":[1],""" +
            """"messageIdx":[0],"confidence":[80],"n":1},"cache_hit":false}"""

    // Daemon writes one JSON object per response terminated by \n. Strip
    // any embedded newlines so tests can author pretty-printed JSON in
    // helpers without breaking the line-delimited contract.
    private fun okResponse(dataJson: String): String =
        """{"ok":true,"data":${dataJson.replace("\n", "").replace(Regex("\\s+"), " ")}}""" + "\n"
}

private class MockDaemonServer private constructor(
    private val server: ServerSocketChannel,
    private val thread: Thread,
) : AutoCloseable {
    @Volatile
    private var running = true

    override fun close() {
        running = false
        runCatching { server.close() }
        thread.interrupt()
        thread.join(2_000)
    }

    companion object {
        fun start(
            socketPath: Path,
            respond: (verb: String, requestLine: String) -> String,
        ): MockDaemonServer {
            val server = ServerSocketChannel.open(StandardProtocolFamily.UNIX)
            server.bind(UnixDomainSocketAddress.of(socketPath))
            val thread = Thread {
                while (true) {
                    val conn = try {
                        server.accept() ?: continue
                    } catch (_: IOException) {
                        return@Thread
                    }
                    try {
                        val reader = BufferedReader(
                            InputStreamReader(Channels.newInputStream(conn), StandardCharsets.UTF_8),
                        )
                        val requestLine = reader.readLine() ?: continue
                        // Cheap verb extraction — doesn't need a JSON parser
                        // for the test fixture. Pattern: {"verb":"…"}.
                        val verb = Regex("\"verb\"\\s*:\\s*\"([^\"]+)\"")
                            .find(requestLine)?.groupValues?.getOrNull(1)
                            .orEmpty()
                        val response = respond(verb, requestLine)
                        val bytes = response.toByteArray(StandardCharsets.UTF_8)
                        var buf = ByteBuffer.wrap(bytes)
                        while (buf.hasRemaining()) {
                            conn.write(buf)
                        }
                    } finally {
                        runCatching { conn.close() }
                    }
                }
            }
            thread.isDaemon = true
            thread.start()
            return MockDaemonServer(server, thread)
        }
    }
}
