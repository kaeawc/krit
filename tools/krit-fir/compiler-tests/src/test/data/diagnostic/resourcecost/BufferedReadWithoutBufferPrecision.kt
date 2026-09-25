// RENDER_DIAGNOSTICS_FULL_TEXT
// Reads Go reports and FIR does not: Go reports any call named `read` whose
// receiver text contains a call named FileInputStream (and no call named
// buffered / BufferedInputStream). None of these reads a FileInputStream
// without a buffer.
package test

import java.io.BufferedInputStream
import java.io.ByteArrayInputStream
import java.io.FileInputStream
import java.io.InputStream
import java.io.InputStreamReader
import java.nio.ByteBuffer

fun FileInputStream.read(tag: String): Int = tag.length

class CountingBufferedStream(input: InputStream) : BufferedInputStream(input)

class Precision {
    // Go reports this: the receiver is a Reader, not an InputStream, and
    // InputStreamReader decodes the file through its own byte buffer.
    fun reader(path: String): Int = InputStreamReader(FileInputStream(path)).read()

    // Go reports this: same Reader, built by the stdlib extension.
    fun readerExtension(path: String): Int = FileInputStream(path).reader().read()

    // Go reports this: bufferedReader() is buffered, but it is not spelled
    // buffered / BufferedInputStream.
    fun bufferedReader(path: String): Int = FileInputStream(path).bufferedReader().read()

    // Go reports this: FileChannel.read(ByteBuffer) is a channel read into a
    // caller-sized buffer, not InputStream.read.
    fun channel(path: String): Int = FileInputStream(path).channel.read(ByteBuffer.allocate(8192))

    // Go reports this: the stream reads the file's bytes from memory.
    fun inMemory(path: String): Int = ByteArrayInputStream(FileInputStream(path).readBytes()).read()

    // Go reports this: an extension function named read, not a stream read.
    fun extension(path: String): Int = FileInputStream(path).read("tag")

    // Go reports this: it only checks the var's initializer, but every read
    // goes through the BufferedInputStream the var was reassigned to.
    fun reassignedToBuffered(path: String): Int {
        var input: InputStream = FileInputStream(path)
        input = BufferedInputStream(input)
        return input.read()
    }

    // Go reports this: it only checks the var's initializer, but the read is
    // on System.in, which the var holds from the assignment before it.
    fun reassignedToOtherStream(path: String): Int {
        var input: InputStream = FileInputStream(path)
        input.close()
        input = System.`in`
        return input.read()
    }

    // Go reports these: a BufferedInputStream subclass and an anonymous
    // BufferedInputStream buffer the FileInputStream, but Go only exempts a
    // call spelled buffered / BufferedInputStream.
    fun bufferedSubclass(path: String): Int = CountingBufferedStream(FileInputStream(path)).read()

    fun anonymousBuffered(path: String): Int = object : BufferedInputStream(FileInputStream(path)) {}.read()

    // Go reports this: it matches the local by name anywhere earlier in the
    // function, but the read is on the shadowing System.in.
    fun shadowed(path: String): Int {
        val input = FileInputStream(path)
        input.close()
        return run {
            val input: InputStream = System.`in`
            input.read()
        }
    }
}
