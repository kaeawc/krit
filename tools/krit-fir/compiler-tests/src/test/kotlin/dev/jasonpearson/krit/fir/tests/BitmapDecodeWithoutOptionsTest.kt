package dev.jasonpearson.krit.fir.tests

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

// BitmapDecodeWithoutOptions on a call split across lines. Go reports on the
// called method's name, so the finding must sit on the method's line, not on
// the receiver's.
class BitmapDecodeWithoutOptionsTest {

    private fun linesOf(source: String): List<Int> =
        KritFirProbe.diagnose(source)
            .filter { it.name == "BitmapDecodeWithoutOptions" }
            .map { it.line }
            .sorted()

    @Test
    fun multilineCallReportsOnMethodLine() {
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
        assertEquals(listOf(8, 15), linesOf(source))
    }
}
