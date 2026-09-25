// Smoke: Paint property setters, Color statics, and Canvas drawing.
package stubs

import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint

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
