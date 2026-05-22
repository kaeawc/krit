package dev.jasonpearson.krit.intellij

import com.intellij.openapi.util.TextRange
import com.intellij.psi.PsiFile

object KritRanges {
    fun rangeFor(file: PsiFile, finding: KritFinding): TextRange {
        val document = file.viewProvider.document ?: return TextRange(0, 0)
        val text = document.charsSequence
        val lineIndex = (finding.line - 1).coerceAtLeast(0)
        if (lineIndex >= document.lineCount) {
            return endOfFileRange(text)
        }

        val lineStart = document.getLineStartOffset(lineIndex)
        val lineEnd = document.getLineEndOffset(lineIndex)
        val byteCol = (finding.column - 1).coerceAtLeast(0)
        val start = byteColumnToCharOffset(text, lineStart, lineEnd, byteCol)
        val end = findTokenEnd(text, start, lineEnd)
        return TextRange(start, end.coerceAtLeast(start))
    }

    // Krit emits `column` from tree-sitter's start.Column, which is a UTF-8
    // byte offset within the line. IntelliJ documents are UTF-16. This
    // translation walks the line one char at a time, summing the UTF-8
    // byte width of each code point until the byte budget is exhausted.
    internal fun byteColumnToCharOffset(
        text: CharSequence,
        lineStartChar: Int,
        lineEndChar: Int,
        byteCol: Int,
    ): Int {
        if (byteCol <= 0) return lineStartChar
        var bytes = 0
        var i = lineStartChar
        while (i < lineEndChar) {
            if (bytes >= byteCol) return i
            val c = text[i]
            val code = c.code
            val width: Int
            val advance: Int
            when {
                code < 0x80 -> {
                    width = 1
                    advance = 1
                }
                code < 0x800 -> {
                    width = 2
                    advance = 1
                }
                c.isHighSurrogate() && i + 1 < lineEndChar && text[i + 1].isLowSurrogate() -> {
                    width = 4
                    advance = 2
                }
                else -> {
                    width = 3
                    advance = 1
                }
            }
            bytes += width
            i += advance
        }
        return lineEndChar
    }

    private fun endOfFileRange(text: CharSequence): TextRange {
        if (text.isEmpty()) {
            return TextRange(0, 0)
        }
        val end = text.length
        val start = (end - 1).coerceAtLeast(0)
        return TextRange(start, end)
    }

    private fun findTokenEnd(text: CharSequence, start: Int, lineEnd: Int): Int {
        var end = start
        while (end < lineEnd && isTokenChar(text[end])) {
            end++
        }
        if (end == start && end < lineEnd) {
            end++
        }
        return end
    }

    private fun isTokenChar(ch: Char): Boolean {
        return ch.isLetterOrDigit() || ch == '_' || ch == '$'
    }
}
