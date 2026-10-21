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
import java.nio.file.LinkOption
import java.nio.file.Path
import java.nio.file.Paths

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
            // Nothing is written until the listener is confirmed to be ours
            // (mirrors dialVerified in internal/daemon/client.go).
            if (!isPeerTrusted(socketPath, peerUser(channel), currentUser())) {
                throw IOException("krit daemon socket $socketPath is not owned by the current user")
            }
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
        // Longest unix socket path the daemon binds directly; beyond this it
        // falls back to a short per-user socket (mirrors internal/daemon).
        private const val MAX_SOCKET_PATH = 100
        private const val POINTER_MAX_SIZE = 4096L

        private fun currentUser(): String = System.getProperty("user.name") ?: ""

        /** Path length in UTF-8 bytes, as Go's len() sees it. */
        internal fun socketPathBytes(path: String): Int = path.toByteArray(StandardCharsets.UTF_8).size

        /**
         * User of the process listening on [channel] via SO_PEERCRED (JDK 17+,
         * Linux), or null when the option is unavailable on this JDK/platform.
         */
        private fun peerUser(channel: SocketChannel): String? {
            return try {
                channel.getOption(jdk.net.ExtendedSocketOptions.SO_PEERCRED)?.user()?.name
            } catch (_: UnsupportedOperationException) {
                null
            } catch (_: IOException) {
                null
            } catch (_: LinkageError) {
                null
            }
        }

        /**
         * Decides whether the listener may receive data. A known peer user must
         * equal [currentUser]; when peer credentials are unavailable the socket
         * file and its directory must be private to the current user.
         */
        internal fun isPeerTrusted(
            socketPath: Path,
            peerUser: String?,
            currentUser: String,
            fsSafe: (Path) -> Boolean = { isSocketLocationPrivate(it, currentUser) },
        ): Boolean = if (peerUser != null) peerUser == currentUser else fsSafe(socketPath)

        /**
         * The socket file and its parent directory are real (non-symlink),
         * owned by [user], and the directory is not group/world writable.
         */
        internal fun isSocketLocationPrivate(sock: Path, user: String = currentUser()): Boolean {
            return try {
                val noFollow = LinkOption.NOFOLLOW_LINKS
                val dir = sock.parent ?: return false
                if (Files.getOwner(sock, noFollow).name != user) return false
                val dirAttrs = Files.readAttributes(
                    dir, java.nio.file.attribute.BasicFileAttributes::class.java, noFollow,
                )
                if (!dirAttrs.isDirectory) return false
                if (Files.getOwner(dir, noFollow).name != user) return false
                val perms = Files.getPosixFilePermissions(dir, noFollow)
                perms.none {
                    it == java.nio.file.attribute.PosixFilePermission.GROUP_WRITE ||
                        it == java.nio.file.attribute.PosixFilePermission.OTHERS_WRITE
                }
            } catch (_: IOException) {
                false
            } catch (_: UnsupportedOperationException) {
                false
            }
        }
        private val FALLBACK_SOCK_NAME = Regex("^[0-9a-f]{16}\\.sock$")

        /**
         * Resolves the daemon socket the same way the Go client does: when
         * `<root>/.krit/daemon.sock` is too long to bind, the daemon records
         * its fallback socket in `<root>/.krit/daemon.sock.path`. That pointer
         * is honored only if it passes the same checks as the Go side and the
         * socket exists; otherwise the conventional path is returned.
         */
        fun socketPathFor(projectDir: java.io.File): Path {
            val root = normalizeRoot(projectDir.toPath())
            val conventional = root.resolve(".krit").resolve("daemon.sock")
            if (socketPathBytes(conventional.toString()) <= MAX_SOCKET_PATH) return conventional
            val pointed = readSocketPointer(root) ?: return conventional
            // Defence in depth against a pointer planted by an untrusted checkout:
            // only follow it into a private, user-owned directory.
            return if (Files.exists(pointed) && isSocketLocationPrivate(pointed)) pointed else conventional
        }

        internal fun fallbackSocketName(root: Path): String {
            val sum = java.security.MessageDigest.getInstance("SHA-256")
                .digest(normalizeRoot(root).toString().toByteArray(StandardCharsets.UTF_8))
            return sum.take(8).joinToString("") { "%02x".format(it) } + ".sock"
        }

        private fun normalizeRoot(root: Path): Path {
            val abs = root.toAbsolutePath().normalize()
            return try {
                abs.toRealPath()
            } catch (_: IOException) {
                abs
            }
        }

        private fun readSocketPointer(root: Path): Path? {
            val pointer = root.resolve(".krit").resolve("daemon.sock.path")
            try {
                val noFollow = LinkOption.NOFOLLOW_LINKS
                val attrs = Files.readAttributes(pointer, java.nio.file.attribute.BasicFileAttributes::class.java, noFollow)
                if (!attrs.isRegularFile || attrs.size() > POINTER_MAX_SIZE) return null
                val owner = Files.getOwner(pointer, noFollow).name
                if (owner != System.getProperty("user.name")) return null
                val text = String(Files.readAllBytes(pointer), StandardCharsets.UTF_8).trim()
                if (text.isEmpty() || socketPathBytes(text) > MAX_SOCKET_PATH || text.contains('\u0000')) return null
                val sock = Paths.get(text)
                if (!sock.isAbsolute) return null
                val name = sock.fileName?.toString() ?: return null
                val dir = sock.parent?.fileName?.toString() ?: return null
                if (!FALLBACK_SOCK_NAME.matches(name) || name != fallbackSocketName(root)) return null
                if (dir != "krit" && !dir.startsWith("krit-")) return null
                return sock
            } catch (_: IOException) {
                return null
            } catch (_: java.nio.file.InvalidPathException) {
                return null
            } catch (_: UnsupportedOperationException) {
                return null
            }
        }
    }
}
