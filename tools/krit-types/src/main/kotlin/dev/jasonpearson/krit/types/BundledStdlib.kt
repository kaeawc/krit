package dev.jasonpearson.krit.types

import java.io.File
import java.io.IOException
import java.nio.file.AtomicMoveNotSupportedException
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.security.MessageDigest

internal object BundledStdlib {
    private val stdlibJarName = Regex("kotlin-stdlib(-[0-9][^/]*)?\\.jar")
    private val jdkStdlibJarName = Regex("kotlin-stdlib-jdk[78](-.*)?\\.jar")
    private val bundledPath: String? by lazy(LazyThreadSafetyMode.SYNCHRONIZED) {
        extract()
    }

    fun effectiveClasspath(user: List<String>): List<String> {
        if (user.any { entry ->
                val name = File(entry).name
                stdlibJarName.matches(name) || jdkStdlibJarName.matches(name)
            }
        ) {
            return user
        }
        return bundledPath?.let { user + it } ?: user
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
