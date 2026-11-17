package dev.jasonpearson.krit.intellij

import java.io.ByteArrayOutputStream
import java.io.File
import java.io.IOException
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import java.security.MessageDigest
import java.util.zip.GZIPOutputStream
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream
import kotlin.test.AfterTest
import kotlin.test.BeforeTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.assertNull
import kotlin.test.assertTrue

class KritBinaryInstallerTest {
    private lateinit var root: File
    private val mac = KritPlatform("darwin", "arm64")
    private val windows = KritPlatform("windows", "amd64")

    /** Serves canned responses by URL and records what was fetched. */
    private class FakeHttp : KritHttp {
        val texts = HashMap<String, String>()
        val files = HashMap<String, ByteArray>()
        val fetched = mutableListOf<String>()
        var failure: IOException? = null

        override fun getText(url: String): String {
            fetched += url
            failure?.let { throw it }
            return texts[url] ?: throw IOException("404 $url")
        }

        override fun download(url: String, target: File) {
            fetched += url
            failure?.let { throw it }
            target.writeBytes(files[url] ?: throw IOException("404 $url"))
        }
    }

    private val http = FakeHttp()

    @BeforeTest
    fun setUp() {
        root = Files.createTempDirectory("krit-install").toFile()
    }

    @AfterTest
    fun tearDown() {
        root.deleteRecursively()
    }

    private fun installer(platform: KritPlatform? = mac) =
        KritBinaryInstaller(http, File(root, "bin"), platform, releasesUrl = RELEASES)

    // Publishes a release with [archives] (asset name to bytes) and a checksums.txt for them.
    private fun publish(
        tag: String,
        archives: Map<String, ByteArray>,
        draft: Boolean = false,
        prerelease: Boolean = false,
        sums: Map<String, ByteArray> = archives,
    ): String {
        val base = "https://github.com/kaeawc/krit/releases/download/$tag"
        archives.forEach { (name, bytes) -> http.files["$base/$name"] = bytes }
        http.texts["$base/checksums.txt"] = sums.entries.joinToString("\n") { (name, bytes) -> "${sha256(bytes)}  $name" }
        val assets = (archives.keys + "checksums.txt").joinToString(",") {
            """{"name": "$it", "browser_download_url": "$base/$it"}"""
        }
        return """{"tag_name": "$tag", "draft": $draft, "prerelease": $prerelease, "assets": [$assets]}"""
    }

    private fun releases(vararg releases: String) {
        http.texts[RELEASES] = releases.joinToString(",", "[", "]")
    }

    @Test
    fun `installs the newest compatible release and verifies its checksum`() {
        releases(
            publish("v1.0.0", mapOf("krit_1.0.0_darwin_arm64.tar.gz" to tarGz("krit" to "new"))),
            publish("v0.4.0", mapOf("krit_0.4.0_darwin_arm64.tar.gz" to tarGz("krit" to "four", "LICENSE" to "MIT"))),
            publish("v0.3.0", mapOf("krit_0.3.0_darwin_arm64.tar.gz" to tarGz("krit" to "three"))),
            publish("v0.5.0", mapOf("krit_0.5.0_darwin_arm64.tar.gz" to tarGz("krit" to "pre")), prerelease = true),
            publish("v0.6.0", mapOf("krit_0.6.0_darwin_arm64.tar.gz" to tarGz("krit" to "draft")), draft = true),
            // A release candidate GitHub doesn't flag as a pre-release.
            publish("v0.4.1-rc1", mapOf("krit_0.4.1-rc1_darwin_arm64.tar.gz" to tarGz("krit" to "rc"))),
        )

        val result = installer().install()

        val installed = assertIs<KritBinaryInstaller.Result.Installed>(result)
        assertEquals(KritVersion(0, 4, 0), installed.version)
        assertEquals(File(root, "bin/0.4.0/krit").canonicalFile, installed.binary.canonicalFile)
        assertEquals("four", installed.binary.readText())
        assertTrue(installed.binary.canExecute())
        assertEquals("MIT", File(root, "bin/0.4.0/LICENSE").readText())
        // No staging directory or partial download is left behind.
        assertEquals(listOf("0.4.0"), File(root, "bin").list()!!.toList())
        assertEquals(installed.binary.canonicalFile, KritBinaryInstaller.installedBinary(File(root, "bin"), mac)?.canonicalFile)
    }

    @Test
    fun `a checksum mismatch installs nothing`() {
        releases(
            publish(
                "v0.4.0",
                mapOf("krit_0.4.0_darwin_arm64.tar.gz" to tarGz("krit" to "tampered")),
                sums = mapOf("krit_0.4.0_darwin_arm64.tar.gz" to tarGz("krit" to "original")),
            ),
        )

        val failed = assertIs<KritBinaryInstaller.Result.Failed>(installer().install())

        assertTrue(failed.message.contains("doesn't match its checksum"), failed.message)
        assertEquals(emptyList(), File(root, "bin").list()!!.toList())
    }

    @Test
    fun `a release missing from checksums txt installs nothing`() {
        releases(
            publish(
                "v0.4.0",
                mapOf("krit_0.4.0_darwin_arm64.tar.gz" to tarGz("krit" to "x")),
                sums = mapOf("krit_0.4.0_linux_amd64.tar.gz" to tarGz("krit" to "x")),
            ),
        )

        val failed = assertIs<KritBinaryInstaller.Result.Failed>(installer().install())

        assertTrue(failed.message.contains("no entry for krit_0.4.0_darwin_arm64.tar.gz"), failed.message)
        assertNull(KritBinaryInstaller.installedBinary(File(root, "bin"), mac))
    }

    @Test
    fun `only releases outside the range download nothing and say so`() {
        releases(publish("v0.2.0", mapOf("krit_0.2.0_darwin_arm64.tar.gz" to tarGz("krit" to "old"))))

        val failed = assertIs<KritBinaryInstaller.Result.Failed>(installer().install())

        assertTrue(failed.message.contains(KritCompatibility.rangeText), failed.message)
        assertTrue(failed.message.contains("the newest is 0.2.0"), failed.message)
        assertTrue(failed.message.contains("Settings → Tools → Krit → Krit binary"), failed.message)
        assertEquals(listOf(RELEASES), http.fetched)
    }

    @Test
    fun `no archive for this platform downloads nothing`() {
        releases(publish("v0.4.0", mapOf("krit_0.4.0_linux_amd64.tar.gz" to tarGz("krit" to "x"))))

        val failed = assertIs<KritBinaryInstaller.Result.Failed>(installer().install())

        assertTrue(failed.message.contains("darwin/arm64"), failed.message)
        assertEquals(listOf(RELEASES), http.fetched)
    }

    @Test
    fun `an unsupported platform does not touch the network`() {
        val failed = assertIs<KritBinaryInstaller.Result.Failed>(installer(platform = null).install())

        assertTrue(failed.message.contains("no prebuilt binary"), failed.message)
        assertEquals(emptyList(), http.fetched)
    }

    @Test
    fun `offline or proxy failures point at the proxy and binary settings`() {
        http.failure = IOException("Connection refused")

        val failed = assertIs<KritBinaryInstaller.Result.Failed>(installer().install())

        assertTrue(failed.message.contains("Couldn't download krit from GitHub (Connection refused)"), failed.message)
        assertTrue(failed.message.contains("HTTP Proxy"), failed.message)
        assertTrue(failed.message.contains("Settings → Tools → Krit → Krit binary"), failed.message)
    }

    @Test
    fun `a non-JSON release list fails cleanly`() {
        http.texts[RELEASES] = "<html>captive portal</html>"

        val failed = assertIs<KritBinaryInstaller.Result.Failed>(installer().install())

        assertTrue(failed.message.contains("unexpected release list"), failed.message)
    }

    @Test
    fun `installs the zip on Windows`() {
        releases(publish("v0.4.0", mapOf("krit_0.4.0_windows_amd64.zip" to zip("krit.exe" to "exe", "krit-lsp.exe" to "lsp"))))

        val installed = assertIs<KritBinaryInstaller.Result.Installed>(installer(windows).install())

        assertEquals("krit.exe", installed.binary.name)
        assertEquals("exe", installed.binary.readText())
    }

    @Test
    fun `an archive entry escaping the install directory fails the install`() {
        releases(publish("v0.4.0", mapOf("krit_0.4.0_darwin_arm64.tar.gz" to tarGz("krit" to "x", "../../evil" to "x"))))

        val failed = assertIs<KritBinaryInstaller.Result.Failed>(installer().install())

        assertTrue(failed.message.contains("outside the install directory"), failed.message)
        assertFalse(File(root, "evil").exists())
        assertNull(KritBinaryInstaller.installedBinary(File(root, "bin"), mac))
    }

    @Test
    fun `reads PAX long names`() {
        val longName = "docs/" + "a".repeat(120) + ".md"
        val archive = File(root, "a.tar.gz").apply { writeBytes(tarGz("krit" to "k", longName to "doc", pax = true)) }
        val dest = File(root, "out").apply { mkdirs() }

        KritBinaryInstaller.extractTarGz(archive, dest)

        assertEquals("doc", File(dest, longName).readText())
        assertEquals("k", File(dest, "krit").readText())
    }

    @Test
    fun `parses checksums txt`() {
        val hash = "a".repeat(64)
        val sums = KritBinaryInstaller.parseChecksums("$hash  krit_0.4.0_linux_amd64.tar.gz\n${"B".repeat(64)} *krit.zip\nnot a line\n")

        assertEquals(mapOf("krit_0.4.0_linux_amd64.tar.gz" to hash, "krit.zip" to "b".repeat(64)), sums)
    }

    @Test
    fun `maps JVM OS and CPU names to release platforms`() {
        assertEquals(KritPlatform("darwin", "arm64"), KritPlatform.of("Mac OS X", "aarch64"))
        assertEquals(KritPlatform("linux", "amd64"), KritPlatform.of("Linux", "amd64"))
        assertEquals(KritPlatform("windows", "amd64"), KritPlatform.of("Windows 11", "x86_64"))
        assertNull(KritPlatform.of("FreeBSD", "amd64"))
        assertNull(KritPlatform.of("Linux", "riscv64"))
        assertEquals("krit_0.4.0_windows_amd64.zip", KritPlatform("windows", "amd64").assetName(KritVersion(0, 4, 0)))
        assertEquals("krit_0.4.0_linux_arm64.tar.gz", KritPlatform("linux", "arm64").assetName(KritVersion(0, 4, 0)))
    }

    @Test
    fun `the installed binary prefers the newest compatible version`() {
        val bin = File(root, "bin")
        fun install(version: String) = File(bin, "$version/krit").apply {
            parentFile.mkdirs()
            writeText(version)
            setExecutable(true)
        }
        install("0.2.0")
        install("0.3.0")
        install("0.10.0")
        install("1.0.0")
        File(bin, ".krit-0.11.0-staging").mkdirs()
        File(bin, "not-a-version").mkdirs()

        assertEquals("0.10.0", KritBinaryInstaller.installedBinary(bin, mac)?.readText())
        assertNull(KritBinaryInstaller.installedBinary(File(root, "missing"), mac))
    }

    private fun sha256(bytes: ByteArray): String =
        MessageDigest.getInstance("SHA-256").digest(bytes).joinToString("") { "%02x".format(it) }

    private fun zip(vararg entries: Pair<String, String>): ByteArray {
        val out = ByteArrayOutputStream()
        ZipOutputStream(out).use { zip ->
            for ((name, text) in entries) {
                zip.putNextEntry(ZipEntry(name))
                zip.write(text.toByteArray())
                zip.closeEntry()
            }
        }
        return out.toByteArray()
    }

    // A minimal ustar writer: regular files, mode 0755, with a PAX header for long names when [pax].
    private fun tarGz(vararg entries: Pair<String, String>, pax: Boolean = false): ByteArray {
        val out = ByteArrayOutputStream()
        GZIPOutputStream(out).use { gz ->
            for ((name, text) in entries) {
                val data = text.toByteArray(StandardCharsets.UTF_8)
                if (pax && name.length > 100) {
                    val body = "path=$name\n"
                    var len = body.length + 3
                    while ("$len $body".length != len) len = "$len $body".length
                    val record = "$len $body".toByteArray(StandardCharsets.UTF_8)
                    gz.write(header("PaxHeader", record.size.toLong(), 'x'))
                    gz.write(padded(record))
                    gz.write(header(name.takeLast(100), data.size.toLong(), '0'))
                } else {
                    gz.write(header(name, data.size.toLong(), '0'))
                }
                gz.write(padded(data))
            }
            gz.write(ByteArray(1024))
        }
        return out.toByteArray()
    }

    private fun header(name: String, size: Long, type: Char): ByteArray {
        val h = ByteArray(512)
        fun put(offset: Int, s: String) = s.toByteArray(StandardCharsets.UTF_8).copyInto(h, offset)
        put(0, name.take(100))
        put(100, "0000755\u0000")
        put(108, "0000000\u0000")
        put(116, "0000000\u0000")
        put(124, "%011o".format(size) + "\u0000")
        put(136, "%011o".format(0) + "\u0000")
        h[156] = type.code.toByte()
        put(257, "ustar\u0000")
        put(263, "00")
        put(148, "        ")
        val sum = h.sumOf { it.toInt() and 0xff }
        put(148, "%06o".format(sum) + "\u0000 ")
        return h
    }

    private fun padded(data: ByteArray): ByteArray = data.copyOf(((data.size + 511) / 512) * 512)

    private companion object {
        const val RELEASES = "https://api.github.test/releases"
    }
}
