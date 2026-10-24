package dev.jasonpearson.krit.intellij

import java.io.File
import java.io.IOException
import java.security.MessageDigest
import java.util.concurrent.ConcurrentHashMap

// SHA-256 of a krit binary, as the daemon reports it in `status`. Hashing a
// ~50 MB binary is not free, so results are cached per path and recomputed
// only when the file's size or modification time changes (an upgrade in
// place).
object KritBinaryHash {
    private data class Entry(val size: Long, val modified: Long, val hash: String)

    private val cache = ConcurrentHashMap<String, Entry>()

    // Lower-case hex digest, or null when the file can't be read.
    fun of(file: File): String? {
        val path = file.absolutePath
        val size = file.length()
        val modified = file.lastModified()
        cache[path]?.let { if (it.size == size && it.modified == modified) return it.hash }
        val hash = try {
            val digest = MessageDigest.getInstance("SHA-256")
            file.inputStream().use { input ->
                val buf = ByteArray(1 shl 16)
                while (true) {
                    val n = input.read(buf)
                    if (n < 0) break
                    digest.update(buf, 0, n)
                }
            }
            digest.digest().joinToString("") { "%02x".format(it) }
        } catch (_: IOException) {
            return null
        }
        cache[path] = Entry(size, modified, hash)
        return hash
    }
}
