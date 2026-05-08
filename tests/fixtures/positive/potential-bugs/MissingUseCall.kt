package fixtures.positive.potentialbugs

import java.io.FileInputStream

class MissingUseCall {
    fun readFile() {
        val stream = FileInputStream("file.txt")
        stream.read()
        stream.close()
    }
}
