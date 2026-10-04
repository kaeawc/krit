// Smoke: Paint property setters, Color statics, and Canvas drawing.
package stubs

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.graphics.Rect
import java.io.InputStream

fun paintRed(canvas: Canvas) {
    val paint = Paint(Paint.ANTI_ALIAS_FLAG)
    paint.color = Color.RED
    paint.style = Paint.Style.STROKE
    paint.strokeWidth = 2f
    paint.isAntiAlias = true
    paint.textSize = 12f
    paint.color = Color.parseColor("#FF0000")
    paint.color = Color.argb(255, 0, 0, 0)
    paint.color = Color.rgb(0, 0, 0)
    canvas.drawColor(Color.TRANSPARENT)
    canvas.drawCircle(0f, 0f, 10f, paint)
    canvas.drawText("label", 0f, 0f, paint)
}

// The bounds-then-sample decode idiom: every decode passes Options.
fun decodeSampled(path: String, reqWidth: Int): Bitmap? {
    val options = BitmapFactory.Options()
    options.inJustDecodeBounds = true
    BitmapFactory.decodeFile(path, options)
    options.inSampleSize = maxOf(1, options.outWidth / reqWidth)
    options.inJustDecodeBounds = false
    options.inPreferredConfig = Bitmap.Config.RGB_565
    return BitmapFactory.decodeFile(path, options)
}

fun decodeMutable(stream: InputStream): Bitmap? {
    val padding = Rect()
    val options = BitmapFactory.Options().apply { inMutable = true }
    return BitmapFactory.decodeStream(stream, padding, options)
}
