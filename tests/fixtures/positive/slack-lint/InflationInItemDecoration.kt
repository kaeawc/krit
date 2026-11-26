package fixtures.positive.slacklint

import android.graphics.Canvas
import android.view.LayoutInflater
import androidx.recyclerview.widget.RecyclerView

class HeaderDecoration : RecyclerView.ItemDecoration() {
    override fun onDraw(c: Canvas, parent: RecyclerView, state: RecyclerView.State) {
        val header = LayoutInflater.from(parent.context).inflate(1, parent, false)
        header.draw(c)
    }
}
