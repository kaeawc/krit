// Smoke: the AndroidX GridLayout is a ViewGroup (not a LinearLayout).
package stubs

import android.content.Context
import android.util.AttributeSet
import android.view.ViewGroup
import androidx.gridlayout.widget.GridLayout

class SmokeGrid(context: Context, attrs: AttributeSet?) : GridLayout(context, attrs)

fun grid(context: Context): ViewGroup {
    val grid = GridLayout(context)
    grid.columnCount = 3
    grid.rowCount = 2
    return grid
}
