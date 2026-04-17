package fixtures.negative.potentialbugs

import java.io.FileInputStream

class MissingUseCall {
    fun readFile() {
        FileInputStream("file.txt").use { stream ->
            stream.read()
        }
    }
}
