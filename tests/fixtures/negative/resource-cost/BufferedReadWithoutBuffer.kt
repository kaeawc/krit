package fixtures.negative.resourcecost

import java.io.FileInputStream

class BufferedReadWithoutBuffer {
    fun read(path: String): ByteArray {
        return FileInputStream(path).buffered().use { input ->
            input.readBytes()
        }
    }
}
