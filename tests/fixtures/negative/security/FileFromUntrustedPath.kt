package test

import java.io.File

class ZipExtractor {
    fun extractEntry(zipDir: File, entryName: String, data: ByteArray) {
        val out = File(zipDir, entryName)
        require(out.canonicalPath.startsWith(zipDir.canonicalPath + File.separator))
        out.writeBytes(data)
    }
}
