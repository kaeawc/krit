// RENDER_DIAGNOSTICS_FULL_TEXT
// Unbuffered reads Go and FIR both report, and buffered or unrelated reads
// both leave alone.
package test

import java.io.BufferedInputStream
import java.io.DataInputStream
import java.io.File
import java.io.FileInputStream
import java.io.InputStream
import java.util.zip.GZIPInputStream

class Reads {
    fun direct(path: String): Int = <!BufferedReadWithoutBuffer!>FileInputStream(path).read()<!>

    fun intoArray(path: String): Int = <!BufferedReadWithoutBuffer!>FileInputStream(path).read(ByteArray(512))<!>

    fun intoRange(path: String, bytes: ByteArray): Int = <!BufferedReadWithoutBuffer!>FileInputStream(path).read(bytes, 0, bytes.size)<!>

    fun qualified(path: String): Int = <!BufferedReadWithoutBuffer!>java.io.FileInputStream(path).read()<!>

    fun localVal(path: String): Int {
        val input = FileInputStream(path)
        return <!BufferedReadWithoutBuffer!>input.read()<!>
    }

    fun localVar(path: String): Int {
        var input = FileInputStream(path)
        return <!BufferedReadWithoutBuffer!>input.read()<!>
    }

    // The declared type hides the FileInputStream; the initializer shows it.
    fun declaredAsInputStream(path: String): Int {
        val input: InputStream = FileInputStream(path)
        return <!BufferedReadWithoutBuffer!>input.read()<!>
    }

    // A var that holds a FileInputStream on every assignment.
    fun reassignedToFileStream(first: String, second: String): Int {
        var input: InputStream = FileInputStream(first)
        if (second.isNotEmpty()) input = FileInputStream(second)
        return <!BufferedReadWithoutBuffer!>input.read()<!>
    }

    fun readInLambda(path: String): Int {
        val input = FileInputStream(path)
        return run { <!BufferedReadWithoutBuffer!>input.read()<!> }
    }

    // A wrapper that adds no buffer still reads the file byte by byte.
    fun dataWrapper(path: String): Int = <!BufferedReadWithoutBuffer!>DataInputStream(FileInputStream(path)).read()<!>

    fun gzipWrapper(path: String): Int = <!BufferedReadWithoutBuffer!>GZIPInputStream(FileInputStream(path)).read()<!>

    fun cast(path: String): Int = <!BufferedReadWithoutBuffer!>(FileInputStream(path) as InputStream).read()<!>

    fun fromList(path: String): Int = <!BufferedReadWithoutBuffer!>listOf(FileInputStream(path)).first().read()<!>

    @Suppress("UNNECESSARY_SAFE_CALL")
    fun safeCall(path: String): Int? = <!BufferedReadWithoutBuffer!>FileInputStream(path)?.read()<!>

    @Suppress("UNNECESSARY_NOT_NULL_ASSERTION")
    fun notNull(path: String): Int = <!BufferedReadWithoutBuffer!>FileInputStream(path)!!.read()<!>

    // Reported on the line the chain starts, as Go reports the call expression.
    fun multiline(path: String): Int =
        <!BufferedReadWithoutBuffer!>FileInputStream(path)<!>
            .read()

    @Suppress("UNNECESSARY_SAFE_CALL")
    fun multilineSafeCall(path: String): Int? =
        <!BufferedReadWithoutBuffer!>FileInputStream(path)<!>
            ?.read()

    // A branch that is a FileInputStream.
    fun conditional(path: String, useFile: Boolean): Int =
        <!BufferedReadWithoutBuffer!>(if (useFile) FileInputStream(path) else System.`in`).read()<!>

    fun elvis(path: String, fallback: InputStream?): Int = <!BufferedReadWithoutBuffer!>(fallback ?: FileInputStream(path)).read()<!>

    // Buffered: no finding.
    fun bufferedExtension(path: String): Int = FileInputStream(path).buffered().read()

    fun bufferedConstructor(path: String): Int = BufferedInputStream(FileInputStream(path)).read()

    fun bufferedLocal(path: String): Int {
        val input = FileInputStream(path).buffered()
        return input.read()
    }

    fun bufferedAtRead(path: String): Int {
        val input = FileInputStream(path)
        return input.buffered().read()
    }

    fun bufferedWrapper(path: String): Int = DataInputStream(FileInputStream(path).buffered()).read()

    // Not a read call.
    fun readBytes(path: String): ByteArray = FileInputStream(path).readBytes()

    // No explicit receiver: Go has no receiver to inspect.
    fun implicitReceiver(path: String): Int = with(FileInputStream(path)) { read() }

    // Another stream.
    fun otherStream(input: InputStream): Int = input.read()

    fun fileRead(file: File): String = file.readText()
}

// A FileInputStream subclass reading itself: no explicit stream receiver.
class CountingStream(file: File) : FileInputStream(file) {
    var count = 0

    override fun read(): Int {
        count++
        return super.read()
    }

    fun readTwice(): Int = this.read() + read()
}
