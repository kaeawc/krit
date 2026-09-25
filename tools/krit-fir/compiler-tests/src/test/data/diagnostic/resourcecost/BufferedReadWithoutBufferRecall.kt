// RENDER_DIAGNOSTICS_FULL_TEXT
// Unbuffered FileInputStream reads Go misses and FIR reports. Go runs this
// rule without type information, so it only sees a FileInputStream(...) call
// in the receiver text or a local initialized with one; each receiver below
// still is (or wraps) a FileInputStream with no buffer.
package test

import java.io.DataInputStream
import java.io.File
import java.io.FileInputStream
import java.io.InputStream
import java.io.FileInputStream as RawFileStream

typealias FileStream = FileInputStream

class TrackedStream(file: File) : FileInputStream(file)

class Holder(val raw: FileInputStream)

class Recall {
    // Go misses this: a parameter typed FileInputStream.
    fun parameter(input: FileInputStream): Int = <!BufferedReadWithoutBuffer!>input.read()<!>

    // Go misses this: a nullable parameter read through a safe call.
    fun nullableParameter(input: FileInputStream?): Int? = <!BufferedReadWithoutBuffer!>input?.read()<!>

    // Go misses this: a subclass of FileInputStream.
    fun subclass(file: File): Int = <!BufferedReadWithoutBuffer!>TrackedStream(file).read()<!>

    fun subclassParameter(input: TrackedStream): Int = <!BufferedReadWithoutBuffer!>input.read()<!>

    // Go misses this: a property typed FileInputStream.
    fun property(holder: Holder): Int = <!BufferedReadWithoutBuffer!>holder.raw.read()<!>

    // Go misses these: the lambda parameter of use is the FileInputStream.
    fun useIt(path: String): Int = FileInputStream(path).use { <!BufferedReadWithoutBuffer!>it.read()<!> }

    fun useNamed(path: String): Int = FileInputStream(path).use { input -> <!BufferedReadWithoutBuffer!>input.read()<!> }

    // Go misses these: File.inputStream() returns a FileInputStream.
    fun fileInputStream(file: File): Int = <!BufferedReadWithoutBuffer!>file.inputStream().read()<!>

    fun fileInputStreamLocal(file: File): Int {
        val input = file.inputStream()
        return <!BufferedReadWithoutBuffer!>input.read()<!>
    }

    // Go misses this: a smart cast to FileInputStream.
    fun smartCast(input: InputStream): Int = if (input is FileInputStream) <!BufferedReadWithoutBuffer!>input.read()<!> else 0

    // Go misses this: a local initialized with an unbuffered wrapper.
    fun wrapperLocal(path: String): Int {
        val input = DataInputStream(FileInputStream(path))
        return <!BufferedReadWithoutBuffer!>input.read()<!>
    }

    // Go misses this: a buffered() call on a side branch does not buffer the
    // stream that is read.
    fun sideBranchBuffered(path: String): Int = <!BufferedReadWithoutBuffer!>FileInputStream(path).also { it.buffered() }.read()<!>

    // Go misses these: an import alias and a type alias of FileInputStream.
    fun importAlias(path: String): Int = <!BufferedReadWithoutBuffer!>RawFileStream(path).read()<!>

    fun typeAlias(path: String): Int = <!BufferedReadWithoutBuffer!>FileStream(path).read()<!>

    // Go misses this: a type parameter bounded by FileInputStream.
    fun <T : FileInputStream> generic(input: T): Int = <!BufferedReadWithoutBuffer!>input.read()<!>

    // Go misses these: a local subclass and an anonymous subclass.
    fun localSubclass(file: File): Int {
        class LocalStream : FileInputStream(file)
        return <!BufferedReadWithoutBuffer!>LocalStream().read()<!>
    }

    fun anonymousSubclass(file: File): Int {
        val input = object : FileInputStream(file) {}
        return <!BufferedReadWithoutBuffer!>input.read()<!>
    }

    // An object expression's member reads a parameter; Go misses it.
    val reader = object {
        fun read(input: FileInputStream): Int = <!BufferedReadWithoutBuffer!>input.read()<!>
    }
}
