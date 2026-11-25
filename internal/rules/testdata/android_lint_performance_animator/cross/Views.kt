package p

import android.content.Context
import android.view.View
import androidx.annotation.Keep

class Gauge(context: Context) : View(context) {
    var level: Float = 0f
    @Keep
    var kept: Float = 0f
    var count: Int = 0
    fun setRadius(r: Float) {}
    fun setWrong(x: String) {}
    @Keep fun setSafe(x: Float) {}
}

@Keep
class KeptGauge(context: Context) : View(context) {
    fun setLevel(l: Float) {}
}
