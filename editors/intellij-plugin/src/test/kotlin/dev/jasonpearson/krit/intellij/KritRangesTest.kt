package dev.jasonpearson.krit.intellij

import kotlin.test.Test
import kotlin.test.assertEquals

class KritRangesTest {
    private fun translate(text: String, byteCol: Int): Int {
        return KritRanges.byteColumnToCharOffset(text, 0, text.length, byteCol)
    }

    @Test
    fun `ascii line passes through unchanged`() {
        val line = "val answer = 42"
        assertEquals(0, translate(line, 0))
        assertEquals(4, translate(line, 4))
        assertEquals(line.length, translate(line, 999))
    }

    @Test
    fun `byte column past emoji maps to correct char offset`() {
        // 😀 is U+1F600, 4 UTF-8 bytes, 2 UTF-16 chars (surrogate pair).
        // Layout: v(1) a(1) l(1)  (1) x(1)  (1) =(1)  (1) "(1) 😀(4) "(1)
        // Byte offsets: 0 1 2 3 4 5 6 7 8 9..12 13
        // Char offsets: 0 1 2 3 4 5 6 7 8 9,10 11
        val line = "val x = \"😀\""
        assertEquals(9, translate(line, 9))    // start of emoji
        assertEquals(11, translate(line, 13))  // closing quote
    }

    @Test
    fun `accented identifier comment maps correctly`() {
        // é = U+00E9, 2 UTF-8 bytes, 1 UTF-16 char.
        // "// café" — bytes:  / / SP c a f é
        //            chars:   0 1 2  3 4 5 6
        //            bytes:   0 1 2  3 4 5 6,7
        val line = "// café x"
        // Byte 8 sits just after é (which occupies bytes 6..7), so the
        // expected char offset is 7 (the space before 'x').
        assertEquals(7, translate(line, 8))
    }

    @Test
    fun `cjk character maps correctly`() {
        // 中 is U+4E2D, 3 UTF-8 bytes, 1 UTF-16 char.
        // "val 中 = 1" — bytes: v a l SP 中(3) SP = SP 1
        //                chars: 0 1 2 3  4    5  6 7  8
        //                bytes: 0 1 2 3  4..6 7  8 9  10
        val line = "val 中 = 1"
        assertEquals(4, translate(line, 4))   // start of 中
        assertEquals(5, translate(line, 7))   // space after 中
        assertEquals(6, translate(line, 8))   // '='
    }

    @Test
    fun `byte column zero or negative returns line start`() {
        val line = "val x = 1"
        assertEquals(0, translate(line, 0))
        assertEquals(0, translate(line, -5))
    }

    @Test
    fun `byte column beyond line returns line end`() {
        val line = "val x = 1"
        assertEquals(line.length, translate(line, 1000))
    }

    @Test
    fun `empty line returns line start`() {
        assertEquals(0, translate("", 0))
        assertEquals(0, translate("", 5))
    }

    @Test
    fun `translation honors line bounds when called mid-document`() {
        // Simulate the second line of a document. byteColumnToCharOffset
        // should never read past lineEndChar, even if byteCol is huge.
        val text = "line one\nval 中 = 1\nline three"
        val lineStart = "line one\n".length
        val lineEnd = lineStart + "val 中 = 1".length
        assertEquals(
            lineStart + 4,
            KritRanges.byteColumnToCharOffset(text, lineStart, lineEnd, 4),
        )
        assertEquals(
            lineEnd,
            KritRanges.byteColumnToCharOffset(text, lineStart, lineEnd, 9999),
        )
    }
}
