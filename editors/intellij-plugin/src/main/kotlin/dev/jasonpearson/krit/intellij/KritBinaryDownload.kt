package dev.jasonpearson.krit.intellij

import com.google.gson.Gson
import com.google.gson.JsonParseException
import com.google.gson.annotations.SerializedName
import java.io.BufferedInputStream
import java.io.File
import java.io.IOException
import java.io.InputStream
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.security.MessageDigest
import java.util.Locale
import java.util.zip.GZIPInputStream
import java.util.zip.ZipInputStream

/** An OS and CPU pair krit publishes release archives for. */
data class KritPlatform(val os: String, val arch: String) {
    val isWindows: Boolean get() = os == "windows"

    val binaryName: String get() = if (isWindows) "krit.exe" else "krit"

    /** The release asset name: `krit_<version>_<os>_<arch>.tar.gz`, `.zip` on Windows. */
    fun assetName(version: KritVersion): String =
        "krit_${version}_${os}_$arch" + if (isWindows) ".zip" else ".tar.gz"

    companion object {
        /** The platform for the JVM's `os.name` / `os.arch`, or null when krit has no build for it. */
        fun of(osName: String, osArch: String): KritPlatform? {
            val name = osName.lowercase(Locale.ROOT)
            val os = when {
                name.startsWith("mac") || name.startsWith("darwin") -> "darwin"
                name.startsWith("linux") -> "linux"
                name.startsWith("windows") -> "windows"
                else -> return null
            }
            val arch = when (osArch.lowercase(Locale.ROOT)) {
                "amd64", "x86_64", "x64" -> "amd64"
                "aarch64", "arm64" -> "arm64"
                else -> return null
            }
            return KritPlatform(os, arch)
        }

        fun current(): KritPlatform? =
            of(System.getProperty("os.name").orEmpty(), System.getProperty("os.arch").orEmpty())
    }
}

/** The HTTP calls the installer makes; production uses the IDE's proxy-aware client. */
interface KritHttp {
    fun getText(url: String): String

    fun download(url: String, target: File)
}

/**
 * Downloads a krit release from GitHub Releases and installs it under
 * [installRoot] (`<root>/<version>/krit`). It picks the newest published
 * release in [KritCompatibility]'s range that has an archive for [platform],
 * checks the archive against the release's `checksums.txt`, and unpacks it.
 * Free of IntelliJ classes so the selection, verification and unpacking are
 * unit-testable.
 */
class KritBinaryInstaller(
    private val http: KritHttp,
    private val installRoot: File,
    private val platform: KritPlatform? = KritPlatform.current(),
    private val releasesUrl: String = RELEASES_URL,
) {
    sealed class Result {
        data class Installed(val binary: File, val version: KritVersion) : Result()

        data class Failed(val message: String) : Result()
    }

    data class Asset(val name: String, val url: String)

    data class Release(val version: KritVersion, val assets: List<Asset>)

    private class ApiAsset(
        val name: String? = null,
        @SerializedName("browser_download_url") val downloadUrl: String? = null,
    )

    private class ApiRelease(
        @SerializedName("tag_name") val tagName: String? = null,
        val draft: Boolean = false,
        val prerelease: Boolean = false,
        val assets: List<ApiAsset>? = null,
    )

    /** Downloads, verifies and installs a release. Network and disk failures become [Result.Failed]. */
    fun install(): Result {
        val target = platform ?: return Result.Failed(
            "krit publishes no prebuilt binary for this OS and CPU. $INSTALL_YOURSELF",
        )
        return try {
            val releases = parseReleases(fetch { http.getText(releasesUrl) })
                ?: return Result.Failed("GitHub returned an unexpected release list. $INSTALL_YOURSELF")
            val release = choose(releases, target) ?: return Result.Failed(noReleaseMessage(releases, target))
            installRelease(release, target)
        } catch (e: NetworkException) {
            Result.Failed(networkMessage(e.cause))
        } catch (e: IOException) {
            Result.Failed("Couldn't install krit under ${installRoot.path} (${reason(e)}). $INSTALL_YOURSELF")
        }
    }

    // Separates network failures (proxy and firewall advice) from disk ones.
    private class NetworkException(override val cause: IOException) : Exception(cause)

    private fun <T> fetch(call: () -> T): T = try {
        call()
    } catch (e: IOException) {
        throw NetworkException(e)
    }

    private fun installRelease(release: Release, target: KritPlatform): Result {
        val assetName = target.assetName(release.version)
        val asset = release.assets.first { it.name == assetName }
        val sums = release.assets.first { it.name == CHECKSUMS }
        val expected = parseChecksums(fetch { http.getText(sums.url) })[assetName]
            ?: return Result.Failed("$CHECKSUMS of krit ${release.version} has no entry for $assetName.")
        installRoot.mkdirs()
        val archive = File.createTempFile("krit-download-", ".part", installRoot)
        val staging = Files.createTempDirectory(installRoot.toPath(), ".krit-${release.version}-").toFile()
        try {
            fetch { http.download(asset.url, archive) }
            val actual = sha256(archive)
            if (!actual.equals(expected, ignoreCase = true)) {
                return Result.Failed(
                    "The downloaded $assetName doesn't match its checksum in $CHECKSUMS, so it wasn't installed. " +
                        "Try again, or install krit yourself.",
                )
            }
            if (target.isWindows) extractZip(archive, staging) else extractTarGz(archive, staging)
            val binary = File(staging, target.binaryName)
            if (!binary.isFile) return Result.Failed("$assetName has no ${target.binaryName}. $INSTALL_YOURSELF")
            if (!target.isWindows) binary.setExecutable(true)
            val dest = File(installRoot, release.version.toString())
            if (dest.exists()) dest.deleteRecursively()
            Files.move(staging.toPath(), dest.toPath(), StandardCopyOption.ATOMIC_MOVE)
            return Result.Installed(File(dest, target.binaryName), release.version)
        } finally {
            archive.delete()
            if (staging.exists()) staging.deleteRecursively()
        }
    }

    private fun noReleaseMessage(releases: List<Release>, target: KritPlatform): String {
        val newest = releases.maxByOrNull { it.version }
        val why = when {
            newest == null -> "GitHub lists no krit releases."
            releases.none { it.version.preRelease == null && KritCompatibility.isCompatible(it.version) } ->
                "No published krit release is in the range this plugin supports " +
                    "(${KritCompatibility.rangeText}); the newest is ${newest.version}."
            else -> "No compatible krit release has a ${target.os}/${target.arch} archive."
        }
        return "$why $INSTALL_YOURSELF"
    }

    companion object {
        const val RELEASES_URL = "https://api.github.com/repos/kaeawc/krit/releases?per_page=50"
        const val CHECKSUMS = "checksums.txt"

        private const val SETTINGS_HINT = "set its path in Settings → Tools → Krit → Krit binary"
        private const val INSTALL_YOURSELF = "Install krit yourself and $SETTINGS_HINT."

        // Archives are a few tens of MB; anything far larger is not a krit release.
        private const val MAX_ENTRY_BYTES = 512L * 1024 * 1024

        private val gson = Gson()

        /** The message for a failed download, pointing at the proxy and binary settings. */
        fun networkMessage(e: IOException): String =
            "Couldn't download krit from GitHub (${reason(e)}). If your network blocks GitHub or needs a proxy, " +
                "set the proxy in Settings → Appearance & Behavior → System Settings → HTTP Proxy, " +
                "or install krit yourself and $SETTINGS_HINT."

        private fun reason(e: IOException): String = e.message?.takeIf { it.isNotBlank() } ?: e.javaClass.simpleName

        /**
         * The published (not draft, not pre-release) releases in a GitHub
         * releases API response; null when it isn't one.
         */
        // Gson leaves nulls in non-null Kotlin types for JSON nulls.
        @Suppress("SENSELESS_COMPARISON", "USELESS_ELVIS")
        fun parseReleases(json: String): List<Release>? {
            val parsed = try {
                gson.fromJson(json, Array<ApiRelease>::class.java)
            } catch (_: JsonParseException) {
                return null
            } catch (_: IllegalStateException) {
                return null
            } ?: return null
            return parsed.filter { it != null && !it.draft && !it.prerelease }.mapNotNull { r ->
                val version = KritVersion.parse(r.tagName) ?: return@mapNotNull null
                val assets = r.assets.orEmpty().filter { it != null }.mapNotNull { a ->
                    val name = a.name ?: return@mapNotNull null
                    val url = a.downloadUrl?.takeIf { it.startsWith("https://") } ?: return@mapNotNull null
                    Asset(name, url)
                }
                Release(version, assets)
            }
        }

        /**
         * The newest compatible release with an archive for [platform] and a
         * checksum file. Release candidates are skipped even when GitHub
         * doesn't mark them as pre-releases.
         */
        fun choose(releases: List<Release>, platform: KritPlatform): Release? =
            releases
                .filter { it.version.preRelease == null && KritCompatibility.isCompatible(it.version) }
                .filter { r ->
                    val names = r.assets.map { it.name }
                    platform.assetName(r.version) in names && CHECKSUMS in names
                }
                .maxByOrNull { it.version }

        /** `checksums.txt` (`<sha256>  <file>` per line) as file name → lower-case hex digest. */
        fun parseChecksums(text: String): Map<String, String> {
            val out = HashMap<String, String>()
            for (line in text.lineSequence()) {
                val parts = line.trim().split(Regex("\\s+"), limit = 2)
                if (parts.size != 2) continue
                val (hash, name) = parts
                if (hash.length != 64 || !hash.all { it.isDigit() || it.lowercaseChar() in 'a'..'f' }) continue
                // sha256sum marks binary-mode entries with a leading '*'.
                out[name.removePrefix("*")] = hash.lowercase(Locale.ROOT)
            }
            return out
        }

        fun sha256(file: File): String {
            val digest = MessageDigest.getInstance("SHA-256")
            file.inputStream().use { input ->
                val buf = ByteArray(64 * 1024)
                while (true) {
                    val n = input.read(buf)
                    if (n < 0) break
                    digest.update(buf, 0, n)
                }
            }
            return digest.digest().joinToString("") { "%02x".format(it) }
        }

        /** Unpacks a zip's regular files into [dest]; entries that would land outside it fail. */
        fun extractZip(archive: File, dest: File) {
            ZipInputStream(BufferedInputStream(archive.inputStream())).use { zip ->
                while (true) {
                    val entry = zip.nextEntry ?: break
                    if (entry.isDirectory) continue
                    copyEntry(zip, entryTarget(dest, entry.name), limit = MAX_ENTRY_BYTES)
                }
            }
        }

        /**
         * Unpacks a gzipped tar's regular files into [dest], keeping the
         * executable bit. Handles the ustar, PAX and GNU long-name headers Go's
         * archive/tar writes; links and other entry types are skipped, and
         * entries that would land outside [dest] fail.
         */
        fun extractTarGz(archive: File, dest: File) {
            GZIPInputStream(BufferedInputStream(archive.inputStream())).use { input ->
                val header = ByteArray(BLOCK)
                var longName: String? = null
                while (true) {
                    if (!readFully(input, header)) break
                    if (header.all { it == 0.toByte() }) break
                    val size = octal(header, 124, 12)
                    if (size < 0 || size > MAX_ENTRY_BYTES) throw IOException("bad tar entry size")
                    val type = header[156].toInt().toChar()
                    when (type) {
                        'x' -> {
                            longName = paxPath(String(readBytes(input, size), StandardCharsets.UTF_8)) ?: longName
                            skipPadding(input, size)
                            continue
                        }
                        'L' -> {
                            longName = String(readBytes(input, size), StandardCharsets.UTF_8).trimEnd('\u0000')
                            skipPadding(input, size)
                            continue
                        }
                    }
                    val name = longName ?: headerName(header)
                    longName = null
                    if (type == '0' || type == '\u0000') {
                        val target = entryTarget(dest, name)
                        copyEntry(input, target, limit = size, exact = true)
                        if (octal(header, 100, 8) and 0b001_001_001 != 0L) target.setExecutable(true)
                    } else {
                        skipFully(input, size)
                    }
                    skipPadding(input, size)
                }
            }
        }

        private const val BLOCK = 512

        private fun headerName(header: ByteArray): String {
            val name = cString(header, 0, 100)
            val ustar = cString(header, 257, 6).startsWith("ustar")
            val prefix = if (ustar) cString(header, 345, 155) else ""
            return if (prefix.isEmpty()) name else "$prefix/$name"
        }

        private fun paxPath(records: String): String? =
            records.lineSequence()
                .mapNotNull { line -> line.substringAfter(' ', "").takeIf { it.startsWith("path=") } }
                .lastOrNull()
                ?.removePrefix("path=")

        private fun cString(bytes: ByteArray, offset: Int, length: Int): String {
            var end = offset
            while (end < offset + length && bytes[end] != 0.toByte()) end++
            return String(bytes, offset, end - offset, StandardCharsets.UTF_8)
        }

        private fun octal(bytes: ByteArray, offset: Int, length: Int): Long {
            val text = cString(bytes, offset, length).trim()
            if (text.isEmpty()) return 0
            return text.toLongOrNull(8) ?: -1
        }

        // The file [name] unpacks to under [dest]; absolute paths and ".." escapes fail.
        private fun entryTarget(dest: File, name: String): File {
            val clean = name.removePrefix("./")
            val root = dest.canonicalFile
            val target = File(root, clean).canonicalFile
            if (clean.isEmpty() || File(clean).isAbsolute || !target.path.startsWith(root.path + File.separator)) {
                throw IOException("archive entry outside the install directory: $name")
            }
            return target
        }

        private fun copyEntry(input: InputStream, target: File, limit: Long, exact: Boolean = false) {
            target.parentFile.mkdirs()
            target.outputStream().use { out ->
                val buf = ByteArray(64 * 1024)
                var remaining = limit
                while (true) {
                    val want = if (exact) minOf(buf.size.toLong(), remaining).toInt() else buf.size
                    if (exact && want == 0) break
                    val n = input.read(buf, 0, want)
                    if (n < 0) {
                        if (exact) throw IOException("truncated archive")
                        break
                    }
                    remaining -= n
                    if (!exact && remaining < 0) throw IOException("archive entry too large")
                    out.write(buf, 0, n)
                }
            }
        }

        private fun readFully(input: InputStream, buf: ByteArray): Boolean {
            var off = 0
            while (off < buf.size) {
                val n = input.read(buf, off, buf.size - off)
                if (n < 0) {
                    if (off == 0) return false
                    throw IOException("truncated archive")
                }
                off += n
            }
            return true
        }

        private fun readBytes(input: InputStream, size: Long): ByteArray {
            val buf = ByteArray(size.toInt())
            if (!readFully(input, buf) && size > 0) throw IOException("truncated archive")
            return buf
        }

        private fun skipFully(input: InputStream, size: Long) {
            var remaining = size
            val buf = ByteArray(8 * 1024)
            while (remaining > 0) {
                val n = input.read(buf, 0, minOf(buf.size.toLong(), remaining).toInt())
                if (n < 0) throw IOException("truncated archive")
                remaining -= n
            }
        }

        private fun skipPadding(input: InputStream, size: Long) {
            val rem = (size % BLOCK).toInt()
            if (rem != 0) skipFully(input, (BLOCK - rem).toLong())
        }

        /**
         * The newest installed krit under [installRoot] (`<version>/krit`),
         * preferring versions in the compatible range; null when none is
         * installed.
         */
        fun installedBinary(installRoot: File, platform: KritPlatform? = KritPlatform.current()): File? {
            val target = platform ?: return null
            val installed = installRoot.listFiles().orEmpty()
                .filter { it.isDirectory && !it.name.startsWith(".") }
                .mapNotNull { dir ->
                    val version = KritVersion.parse(dir.name)?.takeIf { it.toString() == dir.name }
                        ?: return@mapNotNull null
                    val binary = File(dir, target.binaryName)
                    if (binary.isFile && binary.canExecute()) version to binary else null
                }
            return installed
                .sortedWith(
                    compareBy<Pair<KritVersion, File>>({ KritCompatibility.isCompatible(it.first) }, { it.first }),
                )
                .lastOrNull()
                ?.second
        }
    }
}
