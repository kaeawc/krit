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
import java.util.concurrent.atomic.AtomicInteger
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

    @Test
    fun `consecutive calls reuse one connection`() {
        server = MockDaemonServer.start(socketPath, keepAlive = true) { _, _ -> okResponse(singleFindingColumnsPayload()) }
        KritDaemonClient(socketPath).use { client ->
            repeat(5) { assertEquals(1, client.analyzeBuffer("/repo/X.kt", "fun a() {}\n")?.size) }
        }
        assertEquals(1, server!!.accepted.get())
    }

    @Test
    fun `a connection the daemon closed is redialed and the call retried`() {
        // The default mock closes each connection after one reply, as a
        // restarted or idle-exited daemon would; every later call must still
        // succeed on a fresh dial instead of failing on the dead socket.
        server = MockDaemonServer.start(socketPath) { _, _ -> okResponse(singleFindingColumnsPayload()) }
        KritDaemonClient(socketPath).use { client ->
            repeat(3) { assertEquals(1, client.analyzeBuffer("/repo/X.kt", "")?.size) }
        }
        assertEquals(3, server!!.accepted.get())
    }

    @Test
    fun `close drops the connection and a later call dials again`() {
        server = MockDaemonServer.start(socketPath, keepAlive = true) { _, _ -> okResponse(emptyColumnsPayload()) }
        val client = KritDaemonClient(socketPath)
        assertTrue(client.analyzeBuffer("/x.kt", "") != null)
        client.close()
        assertTrue(client.analyzeBuffer("/x.kt", "") != null)
        client.close()
        assertEquals(2, server!!.accepted.get())
    }

    @Test
    fun `analyzeBuffers sends one request and keeps per-buffer errors local`() {
        val requests = AtomicReference<String>()
        server = MockDaemonServer.start(socketPath) { verb, line ->
            requests.set("$verb $line")
            okResponse(
                """{"results":[""" +
                    """{"findings":${singleFindingColumns()},"cache_hit":false},""" +
                    """{"cache_hit":false,"error":"parse failed"}""" +
                    """]}""",
            )
        }
        val results = KritDaemonClient(socketPath).use { client ->
            client.analyzeBuffers(listOf("/repo/X.kt" to "fun a() {}\n", "/repo/Y.kt" to "fun ("))
        }
        assertTrue(requests.get().startsWith(KritDaemonProtocol.VERB_ANALYZE_BUFFERS + " "))
        assertTrue(requests.get().contains("/repo/Y.kt"))
        assertEquals(2, results?.size)
        val first = results!![0] as KritDaemonClient.BufferResult.Findings
        assertEquals("rule.id", first.findings.single().rule)
        assertEquals(KritDaemonClient.BufferResult.Error("parse failed"), results[1])
        assertEquals(1, server!!.accepted.get())
    }

    @Test
    fun `analyzeBuffers returns null when the reply does not match the request`() {
        server = MockDaemonServer.start(socketPath) { _, _ -> okResponse("""{"results":[]}""") }
        val results = KritDaemonClient(socketPath).use { it.analyzeBuffers(listOf("/x.kt" to "")) }
        assertNull(results)
    }

    @Test
    fun `a daemon from another krit binary is skipped after the handshake`() {
        val verbs = java.util.Collections.synchronizedList(mutableListOf<String>())
        server = MockDaemonServer.start(socketPath, keepAlive = true) { verb, _ ->
            verbs.add(verb)
            if (verb == KritDaemonProtocol.VERB_STATUS) okResponse("""{"binary_hash":"daemon-hash"}""")
            else okResponse(singleFindingColumnsPayload())
        }
        val mismatches = AtomicInteger()
        val client = KritDaemonClient(
            socketPath,
            expectedBinaryHash = { "plugin-hash" },
            onBinaryMismatch = { daemon, plugin ->
                assertEquals("daemon-hash", daemon)
                assertEquals("plugin-hash", plugin)
                mismatches.incrementAndGet()
            },
        )
        client.use {
            assertNull(it.analyzeBuffer("/x.kt", ""))
            // Later calls fall back without dialing or re-reporting.
            assertNull(it.analyzeBuffer("/x.kt", ""))
            assertTrue(it.binaryMismatch)
        }
        assertEquals(1, mismatches.get())
        assertEquals(listOf(KritDaemonProtocol.VERB_STATUS), verbs.toList())
        assertEquals(1, server!!.accepted.get())
    }

    @Test
    fun `a matching binary hash checks once per connection`() {
        val verbs = java.util.Collections.synchronizedList(mutableListOf<String>())
        server = MockDaemonServer.start(socketPath, keepAlive = true) { verb, _ ->
            verbs.add(verb)
            if (verb == KritDaemonProtocol.VERB_STATUS) okResponse("""{"binary_hash":"same"}""")
            else okResponse(singleFindingColumnsPayload())
        }
        KritDaemonClient(socketPath, expectedBinaryHash = { "same" }).use { client ->
            repeat(3) { assertEquals(1, client.analyzeBuffer("/x.kt", "")?.size) }
        }
        assertEquals(
            listOf(KritDaemonProtocol.VERB_STATUS) + List(3) { KritDaemonProtocol.VERB_ANALYZE_BUFFER },
            verbs.toList(),
        )
    }

    @Test
    fun `no expected hash skips the handshake`() {
        val verbs = java.util.Collections.synchronizedList(mutableListOf<String>())
        server = MockDaemonServer.start(socketPath, keepAlive = true) { verb, _ ->
            verbs.add(verb)
            okResponse(singleFindingColumnsPayload())
        }
        KritDaemonClient(socketPath).use { assertEquals(1, it.analyzeBuffer("/x.kt", "")?.size) }
        assertEquals(listOf(KritDaemonProtocol.VERB_ANALYZE_BUFFER), verbs.toList())
    }

    private fun emptyColumnsPayload(): String =
        """{"findings":{},"cache_hit":false}"""

    private fun singleFindingColumnsPayload(): String =
        """{"findings":${singleFindingColumns()},"cache_hit":false}"""

    private fun singleFindingColumns(): String =
        """{"files":["/repo/X.kt"],"ruleSets":["rs"],"rules":["rule.id"],"messages":["m"],""" +
            """"fileIdx":[0],"line":[1],"col":[1],"ruleSetIdx":[0],"ruleIdx":[0],"severityID":[1],""" +
            """"messageIdx":[0],"confidence":[80],"n":1}"""

    // Daemon writes one JSON object per response terminated by \n. Strip
    // any embedded newlines so tests can author pretty-printed JSON in
    // helpers without breaking the line-delimited contract.
    private fun okResponse(dataJson: String): String =
        """{"ok":true,"data":${dataJson.replace("\n", "").replace(Regex("\\s+"), " ")}}""" + "\n"
}

private class MockDaemonServer private constructor(
    private val server: ServerSocketChannel,
    private val thread: Thread,
    val accepted: AtomicInteger,
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
        // keepAlive serves requests on a connection until the client closes
        // it, like the real daemon; otherwise each connection is closed after
        // one reply. accepted counts the connections dialed.
        fun start(
            socketPath: Path,
            keepAlive: Boolean = false,
            respond: (verb: String, requestLine: String) -> String,
        ): MockDaemonServer {
            val server = ServerSocketChannel.open(StandardProtocolFamily.UNIX)
            server.bind(UnixDomainSocketAddress.of(socketPath))
            val accepted = AtomicInteger()
            val thread = Thread {
                while (true) {
                    val conn = try {
                        server.accept() ?: continue
                    } catch (_: IOException) {
                        return@Thread
                    }
                    accepted.incrementAndGet()
                    try {
                        val reader = BufferedReader(
                            InputStreamReader(Channels.newInputStream(conn), StandardCharsets.UTF_8),
                        )
                        do {
                            val requestLine = reader.readLine() ?: break
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
                        } while (keepAlive)
                    } catch (_: IOException) {
                        // Client went away mid-request; wait for the next one.
                    } finally {
                        runCatching { conn.close() }
                    }
                }
            }
            thread.isDaemon = true
            thread.start()
            return MockDaemonServer(server, thread, accepted)
        }
    }
}
