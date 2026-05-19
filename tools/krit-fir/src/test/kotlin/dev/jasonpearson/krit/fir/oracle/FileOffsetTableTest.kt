package dev.jasonpearson.krit.fir.oracle

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

class FileOffsetTableTest {

    @Test
    fun lineColIsOneBasedAndRecoversFromOffset() {
        // 0  1  2  3 4 5
        // a  b  \n c d e
        val table = FileOffsetTable("ab\ncde")

        assertEquals(1 to 1, table.lineColAt(0))   // 'a'
        assertEquals(1 to 2, table.lineColAt(1))   // 'b'
        assertEquals(1 to 3, table.lineColAt(2))   // '\n' is end of line 1
        assertEquals(2 to 1, table.lineColAt(3))   // 'c'
        assertEquals(2 to 3, table.lineColAt(5))   // 'e'
    }

    @Test
    fun byteOffsetTracksUtf8WidthOfPrecedingChars() {
        // "é" is U+00E9, two UTF-8 bytes.
        val table = FileOffsetTable("aé")

        assertEquals(0, table.byteOffsetAt(0))   // before 'a'
        assertEquals(1, table.byteOffsetAt(1))   // before 'é' — one byte in
        assertEquals(3, table.byteOffsetAt(2))   // after 'é' — three bytes total
    }

    @Test
    fun byteOffsetHandlesSupplementaryCodepointSurrogatePairs() {
        // U+1F600 (😀) is encoded as a surrogate pair in Kotlin strings
        // (high + low surrogate) and as four UTF-8 bytes on the wire.
        val emoji = "😀"
        val table = FileOffsetTable("a${emoji}b")

        assertEquals(0, table.byteOffsetAt(0))   // before 'a'
        assertEquals(1, table.byteOffsetAt(1))   // before high surrogate
        assertEquals(5, table.byteOffsetAt(3))   // after the emoji's 4 bytes
        assertEquals(6, table.byteOffsetAt(4))   // after 'b'
    }

    @Test
    fun lineColClampsToFileBounds() {
        val table = FileOffsetTable("abc")

        assertEquals(1 to 1, table.lineColAt(-1))
        assertEquals(1 to 4, table.lineColAt(10))
    }
}
