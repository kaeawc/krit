// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 19, 21, 23, 27, 32, 38, 45, 50, 54, 56, 58, 60, 63, 66, 70, 75, 80, 82, 85, 87, 89, 92, 95, 100, 107, 116, 126, 132
// Unbuffered reads Go and FIR both report, and buffered or unrelated reads
// both leave alone.
package test

import java.io.BufferedInputStream
import java.io.DataInputStream
import java.io.File
import java.io.FileInputStream
import java.io.FilterInputStream
import java.io.InputStream
import java.io.SequenceInputStream
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

    // The stream is a lambda's result.
    fun lambdaResult(path: String): Int = <!BufferedReadWithoutBuffer!>run { DataInputStream(FileInputStream(path)) }.read()<!>

    fun letWrapper(path: String): Int = <!BufferedReadWithoutBuffer!>FileInputStream(path).let { DataInputStream(it) }.read()<!>

    fun mappedList(path: String): Int = <!BufferedReadWithoutBuffer!>listOf(FileInputStream(path)).map { DataInputStream(it) }.first().read()<!>

    // The stream is a try expression's result.
    fun tryResult(path: String): Int = <!BufferedReadWithoutBuffer!>(try { DataInputStream(FileInputStream(path)) } finally { println() }).read()<!>

    // An anonymous object whose superclass wraps the FileInputStream.
    fun anonymousFilter(path: String): Int = <!BufferedReadWithoutBuffer!>object : FilterInputStream(FileInputStream(path)) {}.read()<!>

    // Reads before the var is reassigned still read the FileInputStream.
    fun readBeforeReassigned(path: String): Int {
        var input: InputStream = FileInputStream(path)
        val first = <!BufferedReadWithoutBuffer!>input.read()<!>
        input = System.`in`
        return first + input.available()
    }

    fun readBeforeBuffering(path: String): Int {
        var input: InputStream = FileInputStream(path)
        val header = <!BufferedReadWithoutBuffer!>input.read()<!>
        input = BufferedInputStream(input)
        return header + input.available()
    }

    // The first iteration reads the FileInputStream.
    fun readInLoopBeforeReassigned(path: String): Int {
        var input: InputStream = FileInputStream(path)
        while (true) {
            val value = <!BufferedReadWithoutBuffer!>input.read()<!>
            input = System.`in`
            if (value > 0) return value
        }
    }

    // Buffered only on one branch: the other still reads the file.
    fun bufferedOnOneBranch(path: String, buffer: Boolean): Int {
        var input: InputStream = FileInputStream(path)
        if (buffer) input = BufferedInputStream(input)
        return <!BufferedReadWithoutBuffer!>input.read()<!>
    }

    // A buffered stream held in a local does not buffer the other source.
    fun sequenceWithBufferedLocal(first: String, second: String): Int {
        val buffered = BufferedInputStream(FileInputStream(second))
        return <!BufferedReadWithoutBuffer!>SequenceInputStream(FileInputStream(first), buffered).read()<!>
    }

    // A buffered() / BufferedInputStream call among the sources: Go exempts the
    // whole receiver, and FIR matches it.
    fun sequenceWithBufferedCall(first: String, second: String): Int =
        SequenceInputStream(BufferedInputStream(FileInputStream(first)), FileInputStream(second)).read()

    fun branchBuffered(path: String, buffer: Boolean): Int =
        (if (buffer) BufferedInputStream(FileInputStream(path)) else FileInputStream(path)).read()

    fun listWithBuffered(first: String, second: String): Int =
        listOf(FileInputStream(first), FileInputStream(second).buffered()).last().read()

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

    // The FileInputStream is assigned on another branch than the read.
    fun assignedOnOtherBranch(path: String, useFile: Boolean): Int {
        var input: InputStream = System.`in`
        if (useFile) {
            input = FileInputStream(path)
            return 0
        } else {
            return input.read()
        }
    }

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
