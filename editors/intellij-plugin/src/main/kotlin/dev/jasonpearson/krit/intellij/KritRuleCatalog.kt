package dev.jasonpearson.krit.intellij

import com.google.gson.Gson
import com.google.gson.JsonParseException
import java.io.File
import java.io.IOException
import java.nio.charset.StandardCharsets
import java.security.MessageDigest
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.TimeUnit

/**
 * The rule list of a krit binary (`krit --list-rules --format json`), cached
 * so the IDE doesn't run krit for it on every project open or profile load.
 *
 * Entries are keyed by the binary's absolute path, size and modification
 * time, so replacing the binary (an upgrade in place, a different path in
 * settings) fetches a fresh list, while reopening a project or restarting the
 * IDE with the same binary reads the cached one. The cache lives in memory
 * and, when [cacheDir] is set, on disk: one small JSON file per key.
 * [refresh] drops a binary's entry and fetches again.
 *
 * Kept free of IntelliJ classes so the keying and invalidation are
 * unit-testable; [fetch] is injectable for the same reason.
 */
class KritRuleCatalog(
    private val cacheDir: File?,
    private val fetch: (File) -> String? = ::runListRules,
) {
    data class Key(val path: String, val size: Long, val modified: Long) {
        fun fileName(): String {
            val digest = MessageDigest.getInstance("SHA-256")
                .digest("$path\u0000$size\u0000$modified".toByteArray(StandardCharsets.UTF_8))
            return "rules-" + digest.take(12).joinToString("") { "%02x".format(it) } + ".json"
        }

        companion object {
            fun of(binary: File): Key {
                val abs = binary.absoluteFile
                return Key(abs.path, abs.length(), abs.lastModified())
            }
        }
    }

    private class Stored(
        val path: String? = null,
        val size: Long = 0,
        val modified: Long = 0,
        val rules: List<KritRuleInfo>? = null,
    )

    private val gson = Gson()
    private val memory = ConcurrentHashMap<Key, List<KritRuleInfo>>()

    // Guards the disk cache and fetches; the memory map is read without it.
    private val lock = Any()

    /**
     * The rules of [binary]: from memory, then disk, then by running it.
     * Null when the binary can't produce a list (missing, too old to print
     * JSON, failed); that outcome isn't cached, so a later call retries.
     */
    fun rulesFor(binary: File): List<KritRuleInfo>? {
        val key = Key.of(binary)
        memory[key]?.let { return it }
        // Cold path: one loader at a time, so concurrent callers (several
        // projects opening) neither run krit twice nor share the per-key
        // temp file in writeDisk.
        synchronized(lock) {
            memory[key]?.let { return it }
            readDisk(key)?.let {
                memory[key] = it
                return it
            }
            return load(binary, key)
        }
    }

    /** The cached rules of [binary], without running it; null when not cached. */
    fun cachedRulesFor(binary: File): List<KritRuleInfo>? {
        val key = Key.of(binary)
        memory[key]?.let { return it }
        synchronized(lock) {
            return memory[key] ?: readDisk(key)?.also { memory[key] = it }
        }
    }

    /** Drops [binary]'s cached list and fetches it again. */
    fun refresh(binary: File): List<KritRuleInfo>? {
        val key = Key.of(binary)
        synchronized(lock) {
            memory.remove(key)
            cacheDir?.let { File(it, key.fileName()).delete() }
            return load(binary, key)
        }
    }

    private fun load(binary: File, key: Key): List<KritRuleInfo>? {
        val rules = fetch(binary)?.let(KritRuleList::parse) ?: return null
        memory[key] = rules
        writeDisk(key, rules)
        return rules
    }

    private fun readDisk(key: Key): List<KritRuleInfo>? {
        val dir = cacheDir ?: return null
        val file = File(dir, key.fileName())
        if (!file.isFile) return null
        return try {
            val stored = gson.fromJson(file.readText(), Stored::class.java) ?: return null
            // The file name is a hash of the key; check the key itself too.
            if (stored.path != key.path || stored.size != key.size || stored.modified != key.modified) return null
            KritRuleList.parse(KritRuleList.toJson(stored.rules ?: return null))
        } catch (_: IOException) {
            null
        } catch (_: JsonParseException) {
            null
        }
    }

    private fun writeDisk(key: Key, rules: List<KritRuleInfo>) {
        val dir = cacheDir ?: return
        try {
            dir.mkdirs()
            val target = File(dir, key.fileName())
            val tmp = File(dir, target.name + ".tmp")
            tmp.writeText(gson.toJson(Stored(key.path, key.size, key.modified, rules)))
            if (!tmp.renameTo(target)) {
                target.delete()
                tmp.renameTo(target)
            }
        } catch (_: IOException) {
            // A cache that can't be written only costs a rerun next time.
        }
    }

    companion object {
        private const val LIST_TIMEOUT_SECONDS = 20L

        /**
         * Runs `krit --list-rules --format json --no-daemon` and returns its
         * stdout, or null when it fails. --no-daemon keeps a running daemon
         * (possibly from another krit build) out of the answer.
         */
        fun runListRules(binary: File): String? {
            val process = try {
                ProcessBuilder(binary.absolutePath, "--list-rules", "--format", "json", "--no-daemon")
                    .redirectError(ProcessBuilder.Redirect.DISCARD)
                    .start()
            } catch (_: IOException) {
                return null
            }
            return try {
                val out = StringBuilder()
                val reader = Thread({
                    runCatching { out.append(process.inputStream.bufferedReader().readText()) }
                }, "krit-list-rules").apply {
                    isDaemon = true
                    start()
                }
                if (!process.waitFor(LIST_TIMEOUT_SECONDS, TimeUnit.SECONDS)) {
                    process.destroyForcibly()
                    return null
                }
                reader.join(TimeUnit.SECONDS.toMillis(LIST_TIMEOUT_SECONDS))
                if (process.exitValue() != 0) null else out.toString()
            } catch (_: InterruptedException) {
                Thread.currentThread().interrupt()
                process.destroyForcibly()
                null
            }
        }
    }
}
