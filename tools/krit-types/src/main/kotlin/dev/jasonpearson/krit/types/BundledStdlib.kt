package dev.jasonpearson.krit.types

import java.io.File
import java.io.IOException
import java.nio.file.AtomicMoveNotSupportedException
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.security.MessageDigest
import java.util.concurrent.ConcurrentHashMap
import java.util.zip.ZipFile

internal object BundledStdlib {
    // The file facade that declares listOf/mapOf. Only its presence proves a
    // usable stdlib: a stdlib-named entry can be missing on disk,
    // kotlin-stdlib-jdk7/8 are empty shims since Kotlin 1.8, and build tools
    // (Bazel, AAR extraction) can rename the stdlib jar.
    private const val STDLIB_PROBE = "kotlin/collections/CollectionsKt.class"
    private val probed = ConcurrentHashMap<String, Boolean>()
    private val bundledPath: String? by lazy(LazyThreadSafetyMode.SYNCHRONIZED) {
        extract()
    }

    fun effectiveClasspath(user: List<String>): List<String> {
        if (user.any(::containsStdlib)) {
            return user
        }
        return bundledPath?.let { user + it } ?: user
    }

    internal fun containsStdlib(entry: String): Boolean = probed.getOrPut(entry) {
        val file = File(entry)
        when {
            file.isDirectory -> File(file, STDLIB_PROBE).isFile
            file.isFile -> try {
                ZipFile(file).use { it.getEntry(STDLIB_PROBE) != null }
            } catch (_: IOException) {
                false
            }
            else -> false
        }
    }

    private fun extract(): String? = try {
        val bytes = BundledStdlib::class.java.getResourceAsStream("/krit/kotlin-stdlib.jar")?.use { it.readBytes() }
            ?: throw IOException("resource /krit/kotlin-stdlib.jar was not found")
        val hash = MessageDigest.getInstance("SHA-256").digest(bytes)
            .take(8)
            .joinToString("") { "%02x".format(it) }
        val cacheDir = File(System.getProperty("java.io.tmpdir"), "krit-cache").toPath()
        Files.createDirectories(cacheDir)
        val target = cacheDir.resolve("kotlin-stdlib-${KotlinVersion.CURRENT}-$hash.jar")
        if (Files.isRegularFile(target)) {
            target.toString()
        } else {
            val temp = Files.createTempFile(cacheDir, "kotlin-stdlib-", ".tmp")
            try {
                Files.write(temp, bytes)
                try {
                    Files.move(temp, target, StandardCopyOption.ATOMIC_MOVE, StandardCopyOption.REPLACE_EXISTING)
                } catch (_: AtomicMoveNotSupportedException) {
                    Files.move(temp, target, StandardCopyOption.REPLACE_EXISTING)
                }
            } catch (failure: IOException) {
                // The name is content-addressed, so a copy another process put
                // there first (and may hold open, which blocks the replace on
                // Windows) is the same jar.
                if (!Files.isRegularFile(target)) throw failure
            } finally {
                Files.deleteIfExists(temp)
            }
            target.toString()
        }
    } catch (failure: Throwable) {
        System.err.println("krit-types: failed to extract bundled Kotlin stdlib: ${failure.message ?: failure.javaClass.simpleName}")
        null
    }
}

internal fun effectiveClasspath(user: List<String>): List<String> = BundledStdlib.effectiveClasspath(user)
