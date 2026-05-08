package test

import java.io.File
import java.io.InputStream
import java.nio.file.Path
import java.util.zip.ZipInputStream

class Extractor {
    fun unzipWithCanonicalCheck(stream: InputStream, destDir: File) {
        ZipInputStream(stream).use { zis ->
            var entry = zis.nextEntry
            while (entry != null) {
                val out = File(destDir, entry.name)
                require(out.canonicalPath.startsWith(destDir.canonicalPath + File.separator))
                out.parentFile?.mkdirs()
                out.outputStream().use { zis.copyTo(it) }
                entry = zis.nextEntry
            }
        }
    }

    fun unzipWithRequireGuard(stream: InputStream, destDir: File) {
        ZipInputStream(stream).use { zis ->
            var entry = zis.nextEntry
            while (entry != null) {
                val out = File(destDir, entry.name)
                check(out.canonicalPath.startsWith(destDir.canonicalPath + File.separator))
                entry = zis.nextEntry
            }
        }
    }

    fun unzipWithNio(stream: InputStream, destDir: Path) {
        ZipInputStream(stream).use { zis ->
            var entry = zis.nextEntry
            while (entry != null) {
                val resolved = destDir.resolve(entry.name).normalize()
                check(resolved.startsWith(destDir.normalize()))
                entry = zis.nextEntry
            }
        }
    }

    fun copyTrustedEntry(destDir: File, name: String) {
        val out = File(destDir, name)
        out.writeBytes(byteArrayOf())
    }
}
