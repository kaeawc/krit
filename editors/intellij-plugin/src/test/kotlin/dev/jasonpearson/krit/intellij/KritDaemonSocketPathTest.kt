package dev.jasonpearson.krit.intellij

import java.io.File
import java.nio.file.Files
import java.nio.file.Path
import kotlin.test.AfterTest
import kotlin.test.BeforeTest
import kotlin.test.Test
import kotlin.test.assertEquals

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
}
