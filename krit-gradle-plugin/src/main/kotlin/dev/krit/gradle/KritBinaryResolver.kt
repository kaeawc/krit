package dev.krit.gradle

import org.gradle.api.file.DirectoryProperty
import org.gradle.api.provider.Property
import org.gradle.api.services.BuildService
import org.gradle.api.services.BuildServiceParameters
import java.io.BufferedInputStream
import java.io.File
import java.io.FileOutputStream
import java.net.URL
import java.security.MessageDigest
import java.util.zip.GZIPInputStream

/**
 * Build service that downloads and caches the krit binary from GitHub Releases.
 *
 * Registered as a shared build service with maxParallelUsages=1 to prevent
 * concurrent downloads in multi-module builds.
 */
abstract class KritBinaryResolver : BuildService<KritBinaryResolver.Params> {

    interface Params : BuildServiceParameters {
        val version: Property<String>
        val cacheDir: DirectoryProperty
    }

    /**
     * Resolve the krit binary, downloading it if not already cached.
     * Returns the path to the executable binary.
     */
    fun resolve(): File {
        val version = parameters.version.get()
        val platform = detectPlatform()
        val cacheDir = parameters.cacheDir.get().asFile
        val binaryDir = File(cacheDir, "krit/$version/${platform.os}-${platform.arch}")
        val binary = File(binaryDir, platform.binaryName)

        if (binary.exists() && binary.canExecute()) {
            return binary
        }

        binaryDir.mkdirs()
        val baseUrl = "https://github.com/kaeawc/krit/releases/download/v$version"
        val url = "$baseUrl/${platform.archiveName}"
        val checksumsUrl = "$baseUrl/checksums.txt"
        downloadAndExtract(URL(url), binaryDir, platform.binaryName)
        binary.setExecutable(true)

        // Verify checksum (skip gracefully for dev builds)
        verifyChecksum(binary, checksumsUrl, platform.archiveName)

        return binary
    }

    private fun downloadAndExtract(url: URL, targetDir: File, binaryName: String) {
        val connection = url.openConnection()
        connection.connectTimeout = 30_000
        connection.readTimeout = 60_000

        BufferedInputStream(connection.getInputStream()).use { input ->
            // The archive is a .tar.gz -- decompress gzip, then extract tar
            val gzipInput = GZIPInputStream(input)
            val tarBytes = gzipInput.readBytes()

            // Simple tar extraction: find the binary entry and write it
            // tar format: 512-byte header blocks followed by file data
            var offset = 0
            while (offset < tarBytes.size) {
                // Read filename from header (first 100 bytes, null-terminated)
                val nameEnd = tarBytes.indexOf(0, offset, offset + 100)
                if (nameEnd == offset) break // empty header = end of archive
                val name = String(tarBytes, offset, nameEnd - offset)

                // Read file size from header (octal string at offset+124, 12 bytes)
                val sizeStr = String(tarBytes, offset + 124, 11).trim()
                if (sizeStr.isEmpty()) break
                val fileSize = sizeStr.toLong(8)

                val dataOffset = offset + 512
                if (name.endsWith(binaryName) || name == binaryName) {
                    FileOutputStream(File(targetDir, binaryName)).use { out ->
                        out.write(tarBytes, dataOffset, fileSize.toInt())
                    }
                    return
                }

                // Advance to next header (data is padded to 512-byte boundary)
                val dataBlocks = (fileSize + 511) / 512
                offset = dataOffset + (dataBlocks * 512).toInt()
            }

            throw IllegalStateException("Binary '$binaryName' not found in archive from $url")
        }
    }

    /**
     * Verify the SHA-256 checksum of the downloaded binary against checksums.txt.
     * Throws on mismatch; logs a warning and skips if checksums.txt is unavailable (dev builds).
     */
    private fun verifyChecksum(binaryFile: File, checksumsUrl: String, archiveName: String) {
        val checksumsText = try {
            URL(checksumsUrl).readText()
        } catch (_: Exception) {
            // checksums.txt not available (dev build) -- skip verification
            return
        }

        val expectedLine = checksumsText.lines().find { it.contains(archiveName) }
            ?: return // archive not listed in checksums -- skip

        val expected = expectedLine.split("\\s+".toRegex())[0]

        val digest = MessageDigest.getInstance("SHA-256")
        val actual = binaryFile.inputStream().use { stream ->
            val buffer = ByteArray(8192)
            var read: Int
            while (stream.read(buffer).also { read = it } != -1) {
                digest.update(buffer, 0, read)
            }
            digest.digest().joinToString("") { "%02x".format(it) }
        }

        require(expected == actual) {
            "Checksum mismatch for $archiveName: expected $expected, got $actual"
        }
    }

    private fun ByteArray.indexOf(value: Byte, from: Int, to: Int): Int {
        for (i in from until minOf(to, size)) {
            if (this[i] == value) return i
        }
        return to
    }

    data class Platform(
        val os: String,
        val arch: String,
    ) {
        val binaryName: String
            get() = when (os) {
                "windows" -> "krit.exe"
                else -> "krit"
            }
        val archiveName: String
            get() = "krit-${os}-${arch}.tar.gz"
    }

    companion object {
        fun detectPlatform(): Platform {
            val osName = System.getProperty("os.name").lowercase()
            val archName = System.getProperty("os.arch").lowercase()

            val os = when {
                osName.contains("mac") || osName.contains("darwin") -> "darwin"
                osName.contains("linux") -> "linux"
                osName.contains("windows") -> "windows"
                else -> error("Unsupported OS: $osName")
            }

            val arch = when {
                archName == "aarch64" || archName == "arm64" -> "arm64"
                archName == "amd64" || archName == "x86_64" -> "amd64"
                else -> error("Unsupported architecture: $archName")
            }

            return Platform(os, arch)
        }
    }
}
