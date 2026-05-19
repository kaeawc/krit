package dev.jasonpearson.krit.fir.oracle

/**
 * Pre-computed line and UTF-8 byte offset tables for a single source
 * file. Letting callers translate a FIR `KtSourceElement` offset
 * (which is a char index into the original text) into:
 *
 * - 1-based `(line, col)` for the `"line:col"` expression key that
 *   matches krit-types' wire format.
 * - 0-based UTF-8 byte offset for `startByte` / `endByte`, also
 *   matching krit-types' rendering (which uses `Utf8ByteOffsets`).
 *
 * Computed once per file from the file's content; safe to cache for
 * the lifetime of the enclosing [`OracleCollector`] because that
 * collector is rebuilt per compilation.
 */
internal class FileOffsetTable(private val content: String) {

    /** Char offset where each line begins; lineStarts[0] = 0 by definition. */
    private val lineStarts: IntArray = buildLineStarts(content)

    /**
     * UTF-8 byte offset for each char offset in `0..content.length`. The
     * trailing entry holds the file's total UTF-8 byte length so
     * `byteOffsetAt(content.length)` is well-defined.
     */
    private val charToByte: IntArray = buildCharToByte(content)

    /** Map a 0-based char offset to a 1-based (line, column) pair. */
    fun lineColAt(charOffset: Int): Pair<Int, Int> {
        val clamped = charOffset.coerceIn(0, content.length)
        val line = lineIndexFor(clamped)
        val col = clamped - lineStarts[line] + 1
        return (line + 1) to col
    }

    /** Map a 0-based char offset to a 0-based UTF-8 byte offset. */
    fun byteOffsetAt(charOffset: Int): Int {
        val clamped = charOffset.coerceIn(0, content.length)
        return charToByte[clamped]
    }

    private fun lineIndexFor(charOffset: Int): Int {
        var lo = 0
        var hi = lineStarts.size - 1
        while (lo < hi) {
            val mid = (lo + hi + 1) ushr 1
            if (lineStarts[mid] <= charOffset) lo = mid else hi = mid - 1
        }
        return lo
    }

    companion object {
        private fun buildLineStarts(content: String): IntArray {
            val starts = ArrayList<Int>().apply { add(0) }
            for (i in content.indices) {
                if (content[i] == '\n') starts.add(i + 1)
            }
            return starts.toIntArray()
        }

        private fun buildCharToByte(content: String): IntArray {
            val table = IntArray(content.length + 1)
            var bytes = 0
            for (i in content.indices) {
                table[i] = bytes
                val cp = content[i].code
                bytes += when {
                    cp < 0x80 -> 1
                    cp < 0x800 -> 2
                    cp in 0xD800..0xDBFF -> {
                        // High surrogate: counted with its low surrogate as a
                        // single 4-byte UTF-8 sequence. Only the high
                        // surrogate contributes the four bytes; the low
                        // surrogate that follows contributes zero.
                        4
                    }
                    cp in 0xDC00..0xDFFF -> 0
                    else -> 3
                }
            }
            table[content.length] = bytes
            return table
        }
    }
}
