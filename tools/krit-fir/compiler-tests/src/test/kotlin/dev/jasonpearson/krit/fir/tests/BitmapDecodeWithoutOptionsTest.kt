package dev.jasonpearson.krit.fir.tests

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

// BitmapDecodeWithoutOptions on a call split across lines, which a golden
// marker cannot span. Go reports on the first line of the call expression,
// the receiver's line, so the finding must not move to the method's line.
class BitmapDecodeWithoutOptionsTest {

    private fun linesOf(source: String): List<Int> =
        KritFirProbe.diagnose(source)
            .filter { it.name == "BitmapDecodeWithoutOptions" }
            .map { it.line }
            .sorted()

    @Test
    fun multilineCallReportsOnReceiverLine() {
        val source = """
            package multiline

            import android.graphics.Bitmap
            import android.graphics.BitmapFactory

            fun split(path: String): Bitmap? {
                val bitmap = BitmapFactory
                    .decodeFile(path)
                return bitmap
            }

            fun qualified(path: String): Bitmap? =
                android.graphics
                    .BitmapFactory
                    .decodeFile(path)

            fun withOptions(path: String): Bitmap? = BitmapFactory
                .decodeFile(path, BitmapFactory.Options())
        """.trimIndent()
        assertEquals(listOf(7, 13), linesOf(source))
    }
}
