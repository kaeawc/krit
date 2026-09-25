// Smoke: subclass Drawable and override its abstract Java methods.
package stubs

import android.graphics.Canvas
import android.graphics.Color
import android.graphics.ColorFilter
import android.graphics.PixelFormat
import android.graphics.drawable.ColorDrawable
import android.graphics.drawable.Drawable

class SmokeDrawable : Drawable() {
    override fun draw(canvas: Canvas) {}

    override fun setAlpha(alpha: Int) {}

    override fun setColorFilter(colorFilter: ColorFilter?) {}

    @Deprecated("Deprecated in Java")
    override fun getOpacity(): Int = PixelFormat.TRANSLUCENT
}

fun background(): Drawable {
    val drawable = ColorDrawable(Color.BLACK)
    drawable.setBounds(0, 0, 10, 10)
    <!PrintlnInProduction!>println<!>(drawable.intrinsicWidth)
    return drawable
}
