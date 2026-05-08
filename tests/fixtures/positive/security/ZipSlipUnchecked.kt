package test

import java.io.File
import java.io.InputStream
import java.util.zip.ZipInputStream

class Extractor {
    fun unzip(stream: InputStream, destDir: File) {
        ZipInputStream(stream).use { zis ->
            var entry = zis.nextEntry
            while (entry != null) {
                val out = File(destDir, entry.name)
                out.parentFile?.mkdirs()
                out.outputStream().use { zis.copyTo(it) }
                entry = zis.nextEntry
            }
        }
    }
}
