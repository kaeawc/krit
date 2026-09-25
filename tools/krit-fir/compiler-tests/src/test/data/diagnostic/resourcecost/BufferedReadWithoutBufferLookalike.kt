// RENDER_DIAGNOSTICS_FULL_TEXT
// A class of this package named FileInputStream: not a java.io.FileInputStream.
package test

import java.io.InputStream

class FileInputStream(private val data: ByteArray) : InputStream() {
    private var position = 0

    override fun read(): Int = if (position < data.size) data[position++].toInt() and 0xff else -1
}

class Lookalike {
    // Go reports these two because the receiver calls, or the local is
    // initialized by, something named FileInputStream; FIR is correct to drop
    // them because the stream is test.FileInputStream, which reads from memory.
    fun inMemory(data: ByteArray): Int = FileInputStream(data).read()

    fun inMemoryLocal(data: ByteArray): Int {
        val input = FileInputStream(data)
        return input.read()
    }

    // The JDK stream, fully qualified, still is one.
    fun jdk(path: String): Int = <!BufferedReadWithoutBuffer!>java.io.FileInputStream(path).read()<!>
}
