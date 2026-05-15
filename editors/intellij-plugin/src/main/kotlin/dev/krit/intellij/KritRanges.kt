package dev.krit.intellij

import com.intellij.openapi.util.TextRange
import com.intellij.psi.PsiFile

object KritRanges {
    fun rangeFor(file: PsiFile, finding: KritFinding): TextRange {
        val document = file.viewProvider.document ?: return TextRange(0, 0)
        val lineIndex = (finding.line - 1).coerceAtLeast(0)
        if (lineIndex >= document.lineCount) {
            return endOfFileRange(document.charsSequence)
        }

        val lineStart = document.getLineStartOffset(lineIndex)
        val lineEnd = document.getLineEndOffset(lineIndex)
        val columnIndex = (finding.column - 1).coerceAtLeast(0)
        val start = (lineStart + columnIndex).coerceIn(lineStart, lineEnd)
        val end = findTokenEnd(document.charsSequence, start, lineEnd)
        return TextRange(start, end.coerceAtLeast(start))
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
