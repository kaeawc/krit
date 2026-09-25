package dev.jasonpearson.krit.gradle

import org.gradle.api.file.DirectoryProperty
import org.gradle.api.provider.Property
import org.gradle.api.services.BuildService
import org.gradle.api.services.BuildServiceParameters
import java.io.BufferedInputStream
import java.io.File
import java.io.InputStream
import java.net.URI
import java.net.URL
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.security.MessageDigest
import java.util.zip.GZIPInputStream
import java.util.zip.ZipInputStream

/**
 * Build service that downloads and caches the krit binary from GitHub Releases.
 *
 * Registered as a shared build service with maxParallelUsages=1 to prevent
 * concurrent downloads in multi-module builds.
 *
 * Release archives follow the naming in `.goreleaser.yml`:
 * `krit_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows, and
 * `krit_<version>_linux_musl_amd64.tar.gz` for musl libc). Every archive is
 * listed in the release's `checksums.txt`, which is verified before extraction.
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
        val version = normalizeVersion(parameters.version.get())
        val platform = detectPlatform()
        val cacheDir = parameters.cacheDir.get().asFile
        val binaryDir = File(cacheDir, "krit/$version/${platform.id}")
        val binary = File(binaryDir, platform.binaryName)

        if (binary.exists() && binary.canExecute()) {
            return binary
        }

        binaryDir.mkdirs()
        val archiveName = platform.archiveName(version)
        val baseUrl = releaseBaseUrl(version)

        val archiveBytes = downloadBytes(URI.create("$baseUrl/$archiveName").toURL())
        val checksums = String(downloadBytes(URI.create("$baseUrl/checksums.txt").toURL()))
        verifyChecksum(archiveBytes, checksums, archiveName)
        extractBinary(archiveBytes, archiveName, binaryDir, platform.binaryName)

        return binary
    }

    private fun downloadBytes(url: URL): ByteArray {
        val connection = url.openConnection()
        connection.connectTimeout = 30_000
        connection.readTimeout = 60_000
        return BufferedInputStream(connection.getInputStream()).use { it.readBytes() }
    }

    data class Platform(
        val os: String,
        val arch: String,
        val musl: Boolean = false,
    ) {
        val binaryName: String
            get() = when (os) {
                "windows" -> "krit.exe"
                else -> "krit"
            }

        /** Cache-directory key; distinct per libc so glibc and musl builds never collide. */
        val id: String
            get() = if (musl) "$os-musl-$arch" else "$os-$arch"

        private val archiveExtension: String
            get() = if (os == "windows") "zip" else "tar.gz"

        /** Release asset name for [version] (no leading `v`), matching `.goreleaser.yml`. */
        fun archiveName(version: String): String {
            val libc = if (musl) "musl_" else ""
            return "krit_${normalizeVersion(version)}_${os}_$libc$arch.$archiveExtension"
        }
    }

    companion object {
        private const val RELEASES_URL = "https://github.com/kaeawc/krit/releases/download"

        /** Accepts `0.2.0` or `v0.2.0`; release assets use the bare version. */
        fun normalizeVersion(version: String): String = version.trim().removePrefix("v")

        fun releaseBaseUrl(version: String): String = "$RELEASES_URL/v${normalizeVersion(version)}"

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

            check(!(os == "windows" && arch == "arm64")) {
                "krit does not publish a windows/arm64 build; set krit { advanced { binary.set(...) } } " +
                    "to a locally built binary"
            }

            // Only linux/amd64 has a musl build; musl on arm64 falls back to
            // the glibc archive, matching install.sh.
            val musl = os == "linux" && arch == "amd64" && isMuslLibc()
            return Platform(os, arch, musl)
        }

        private fun isMuslLibc(): Boolean =
            File("/lib").listFiles()?.any { it.name.startsWith("ld-musl-") } == true

        /**
         * Verify the SHA-256 of [archiveBytes] against the release's
         * `checksums.txt` ([checksumsText]). Fails when the archive is missing
         * from the file or the digest does not match.
         */
        internal fun verifyChecksum(archiveBytes: ByteArray, checksumsText: String, archiveName: String) {
            val expected = checksumsText.lineSequence()
                .map { it.trim().split(Regex("\\s+")) }
                .firstOrNull { it.size == 2 && it[1].removePrefix("*") == archiveName }
                ?.get(0)
                ?.lowercase()
                ?: error("$archiveName is not listed in checksums.txt")

            val actual = MessageDigest.getInstance("SHA-256")
                .digest(archiveBytes)
                .joinToString("") { "%02x".format(it) }

            require(expected == actual) {
                "Checksum mismatch for $archiveName: expected $expected, got $actual"
            }
        }

        /**
         * Extract [binaryName] from a release archive (`.tar.gz` or `.zip`,
         * chosen by [archiveName]) into [targetDir]. The binary is written to a
         * temporary file and moved into place so an interrupted extraction
         * never leaves a truncated binary that looks cached. The result is executable.
         */
        internal fun extractBinary(archiveBytes: ByteArray, archiveName: String, targetDir: File, binaryName: String) {
            val contents = when {
                archiveName.endsWith(".zip") -> readZipEntry(archiveBytes, binaryName)
                archiveName.endsWith(".tar.gz") -> readTarGzEntry(archiveBytes, binaryName)
                else -> error("Unsupported archive format: $archiveName")
            } ?: throw IllegalStateException("Binary '$binaryName' not found in $archiveName")

            targetDir.mkdirs()
            val tmp = File.createTempFile("$binaryName.", ".tmp", targetDir)
            try {
                tmp.writeBytes(contents)
                tmp.setExecutable(true)
                Files.move(
                    tmp.toPath(),
                    File(targetDir, binaryName).toPath(),
                    StandardCopyOption.REPLACE_EXISTING,
                )
            } finally {
                tmp.delete()
            }
        }

        private fun readZipEntry(archiveBytes: ByteArray, binaryName: String): ByteArray? {
            ZipInputStream(archiveBytes.inputStream()).use { zip ->
                while (true) {
                    val entry = zip.nextEntry ?: return null
                    if (!entry.isDirectory && entry.name.substringAfterLast('/') == binaryName) {
                        return zip.readBytes()
                    }
                }
            }
        }

        private fun readTarGzEntry(archiveBytes: ByteArray, binaryName: String): ByteArray? {
            val tar = GZIPInputStream(archiveBytes.inputStream()).use(InputStream::readBytes)

            // tar format: 512-byte header blocks followed by file data padded to 512.
            var offset = 0
            while (offset + 512 <= tar.size) {
                val nameEnd = tar.indexOf(0, offset, offset + 100)
                if (nameEnd == offset) return null // empty header = end of archive
                val name = String(tar, offset, nameEnd - offset)

                val sizeField = String(tar, offset + 124, 12).trim { it <= ' ' }
                if (sizeField.isEmpty()) return null
                val fileSize = sizeField.toLong(8).toInt()

                // Only regular files ('0' or NUL); skips PAX/GNU metadata
                // headers whose names can share the binary's basename.
                val type = tar[offset + 156].toInt().toChar()
                val dataOffset = offset + 512
                if ((type == '0' || type == '\u0000') && name.substringAfterLast('/') == binaryName) {
                    return tar.copyOfRange(dataOffset, dataOffset + fileSize)
                }

                offset = dataOffset + ((fileSize + 511) / 512) * 512
            }
            return null
        }

        private fun ByteArray.indexOf(value: Byte, from: Int, to: Int): Int {
            for (i in from until minOf(to, size)) {
                if (this[i] == value) return i
            }
            return to
        }
    }
}
