package fixtures.positive.resourcecost

import java.io.FileInputStream

class BufferedReadWithoutBuffer {
    fun read(path: String): Int {
        val bytes = ByteArray(512)
        return FileInputStream(path).read(bytes)
    }
}
