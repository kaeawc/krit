package dev.jasonpearson.krit.intellij

import java.io.File
import java.nio.file.Files
import java.nio.file.Path
import kotlin.test.AfterTest
import kotlin.test.BeforeTest
import kotlin.test.Test
import java.nio.file.attribute.PosixFilePermissions
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class KritDaemonSocketPathTest {
    private lateinit var base: Path
    private lateinit var shortDir: Path

    @BeforeTest
    fun setUp() {
        base = Files.createTempDirectory("krit-sockpath").toRealPath()
        shortDir = Files.createTempDirectory("krit-")
    }

    @AfterTest
    fun tearDown() {
        base.toFile().deleteRecursively()
        shortDir.toFile().deleteRecursively()
    }

    private fun longRoot(): Path {
        var p = base
        while (p.resolve(".krit").resolve("daemon.sock").toString().length <= 100) {
            p = p.resolve("a-deeply-nested-directory-name")
        }
        Files.createDirectories(p.resolve(".krit"))
        return p
    }

    private fun fallbackSocket(root: Path): Path {
        val sock = shortDir.resolve(KritDaemonClient.fallbackSocketName(root))
        Files.createFile(sock)
        return sock
    }

    private fun writePointer(root: Path, content: String) {
        Files.writeString(root.resolve(".krit").resolve("daemon.sock.path"), content)
    }

    @Test
    fun `long project path resolves via pointer file`() {
        val root = longRoot()
        val sock = fallbackSocket(root)
        writePointer(root, sock.toString() + "\n")
        assertEquals(sock, KritDaemonClient.socketPathFor(root.toFile()))
    }

    @Test
    fun `missing pointer file falls back to conventional socket`() {
        val root = longRoot()
        assertEquals(root.resolve(".krit/daemon.sock"), KritDaemonClient.socketPathFor(root.toFile()))
    }

    @Test
    fun `invalid pointer content is ignored`() {
        val root = longRoot()
        val conventional = root.resolve(".krit/daemon.sock")
        val sock = fallbackSocket(root)
        val wrongName = shortDir.resolve("0123456789abcdef.sock").also { Files.createFile(it) }
        val badContents = listOf(
            "",
            "relative/krit/path.sock",
            "/etc/passwd",
            wrongName.toString(),
            sock.toString() + "x",
        )
        for (content in badContents) {
            writePointer(root, content)
            assertEquals(conventional, KritDaemonClient.socketPathFor(root.toFile()), "content: '$content'")
        }
    }

    @Test
    fun `pointer to nonexistent socket is ignored`() {
        val root = longRoot()
        val sock = fallbackSocket(root)
        writePointer(root, sock.toString())
        Files.delete(sock)
        assertEquals(root.resolve(".krit/daemon.sock"), KritDaemonClient.socketPathFor(root.toFile()))
    }

    @Test
    fun `pointer is ignored when conventional path is short`() {
        val root = base.resolve("p").also { Files.createDirectories(it.resolve(".krit")) }
        val sock = fallbackSocket(root)
        writePointer(root, sock.toString())
        assertEquals(root.resolve(".krit/daemon.sock"), KritDaemonClient.socketPathFor(File(root.toString())))
    }

    @Test
    fun `pointer into group-writable directory is ignored`() {
        val root = longRoot()
        val sock = fallbackSocket(root)
        writePointer(root, sock.toString())
        assertEquals(sock, KritDaemonClient.socketPathFor(root.toFile()))
        Files.setPosixFilePermissions(shortDir, PosixFilePermissions.fromString("rwxrwx---"))
        assertEquals(root.resolve(".krit/daemon.sock"), KritDaemonClient.socketPathFor(root.toFile()))
        Files.setPosixFilePermissions(shortDir, PosixFilePermissions.fromString("rwx---rwx"))
        assertEquals(root.resolve(".krit/daemon.sock"), KritDaemonClient.socketPathFor(root.toFile()))
    }

    @Test
    fun `socket location owned by another user is not private`() {
        val sock = shortDir.resolve("x.sock").also { Files.createFile(it) }
        assertTrue(KritDaemonClient.isSocketLocationPrivate(sock))
        assertFalse(KritDaemonClient.isSocketLocationPrivate(sock, "someone-else"))
    }

    @Test
    fun `peer check rejects mismatched user and accepts matching user`() {
        val sock = Path.of("/nonexistent/krit/0123456789abcdef.sock")
        val neverSafe: (Path) -> Boolean = { false }
        assertFalse(KritDaemonClient.isPeerTrusted(sock, "mallory", "alice", neverSafe))
        assertTrue(KritDaemonClient.isPeerTrusted(sock, "alice", "alice", neverSafe))
    }

    @Test
    fun `peer check falls back to filesystem checks when credentials are unavailable`() {
        val sock = Path.of("/nonexistent/krit/0123456789abcdef.sock")
        assertFalse(KritDaemonClient.isPeerTrusted(sock, null, "alice") { false })
        assertTrue(KritDaemonClient.isPeerTrusted(sock, null, "alice") { true })
    }

    @Test
    fun `daemon client connects to a listener owned by the current user`() {
        val sock = shortDir.resolve("live.sock")
        java.nio.channels.ServerSocketChannel.open(java.net.StandardProtocolFamily.UNIX).use { server ->
            server.bind(java.net.UnixDomainSocketAddress.of(sock))
            assertTrue(KritDaemonClient(sock).available())
        }
    }

    @Test
    fun `socket path bytes counts utf-8 bytes not chars`() {
        assertEquals(3, KritDaemonClient.socketPathBytes("a\u00e9"))
        assertEquals(5, KritDaemonClient.socketPathBytes("\u00e9\u00e9a"))
    }

    @Test
    fun `socket path limit is measured in utf-8 bytes`() {
        // Multibyte directory: at most 100 chars but more than 100 bytes.
        val suffix = "/.krit/daemon.sock"
        val k = 100 - (base.toString().length + 1 + suffix.length)
        val root = try {
            base.resolve("\u00e9".repeat(k)).also { Files.createDirectories(it.resolve(".krit")) }
        } catch (_: java.nio.file.InvalidPathException) {
            return // file system encoding cannot represent non-ASCII names
        }
        val conventional = root.resolve(".krit/daemon.sock")
        assertTrue(conventional.toString().length <= 100)
        assertTrue(conventional.toString().toByteArray(Charsets.UTF_8).size > 100)
        val sock = fallbackSocket(root)
        writePointer(root, sock.toString())
        assertEquals(sock, KritDaemonClient.socketPathFor(root.toFile()))
    }
}
