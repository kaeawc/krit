package fixtures.negative.slacklint

import android.graphics.Canvas
import android.graphics.Paint
import androidx.recyclerview.widget.RecyclerView

class HeaderDecoration(private val paint: Paint) : RecyclerView.ItemDecoration() {
    override fun onDraw(c: Canvas, parent: RecyclerView, state: RecyclerView.State) {
        c.drawText("Header", 0f, 0f, paint)
    }
}
