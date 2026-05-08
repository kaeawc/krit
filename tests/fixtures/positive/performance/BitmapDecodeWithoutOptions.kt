package fixtures.positive.performance

import android.graphics.BitmapFactory

fun loadBitmap(path: String) {
    val bitmap = BitmapFactory.decodeFile(path)
    println(bitmap)
}
