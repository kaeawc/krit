// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 21, 24
// Only the buffer the read sees decides its size: a large val in another function does not exempt a parameter,
// a small local shadows a large member, and a lambda parameter is a buffer of unknown size.
package test

import java.io.File
import java.io.FileInputStream

class Reader(private val file: File) {
    private val chunk = ByteArray(16384)

    fun elsewhere() {
        val buffer = ByteArray(8192)
        FileInputStream(file).read(buffer)
    }

    fun param(buffer: ByteArray) = <!BufferedReadWithoutBuffer!>FileInputStream(file).read(buffer)<!>
    fun shadowed() {
        val chunk = ByteArray(1)
        <!BufferedReadWithoutBuffer!>FileInputStream(file).read(chunk)<!>
    }
    fun member() = FileInputStream(file).read(chunk)
    fun lambda() = listOf(ByteArray(1)).forEach { buffer -> <!BufferedReadWithoutBuffer!>FileInputStream(file).read(buffer)<!> }
}
