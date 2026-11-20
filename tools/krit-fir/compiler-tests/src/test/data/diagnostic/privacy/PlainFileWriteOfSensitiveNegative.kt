// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 39, 44
package test.plainfilenegative

import java.io.File

fun safe(dir: File, token: String, bytes: ByteArray) {
    File(dir, "cache.json").writeText(token)
    File("pin.txt").writeBytes(bytes)
    File("api_key.txt").writeText(token)
    File("tokenizer.txt").writeText(token)
    File("token2.txt").writeText(token)
    File("shipping.txt").writeText(token)
    File("cache", "credentials.json").writeText(token)
    File("credentials.json").appendText(token)
    File("auth_token.bin").appendBytes(bytes)
}

fun unresolvedPath(path: String, text: String) {
    File(path).writeText(text)
    val credentials = File("credentials.json")
    credentials.writeText(text)
}

class Writer {
    fun writeText(text: String) {}
    fun writeBytes(bytes: ByteArray) {}
}

fun lookalike(writer: Writer, token: String, bytes: ByteArray) {
    writer.writeText(token)
    writer.writeBytes(bytes)
}

fun shadowedConstructor(text: String) {
    class File(path: String) {
        fun writeText(text: String) {}
    }
    File("credentials.json").writeText(text)
}

fun projectExtension(text: String) {
    fun File.writeText(text: String) {}
    File("credentials.json").writeText(text)
}
