// Smoke: widget subclasses, property setters, and the Toast.makeText().show() idiom.
package stubs

import android.content.Context
import android.util.AttributeSet
import android.widget.Button
import android.widget.EditText
import android.widget.FrameLayout
import android.widget.GridLayout
import android.widget.HorizontalScrollView
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.ProgressBar
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast

class SmokeTextView(context: Context, attrs: AttributeSet?) : TextView(context, attrs)

class SmokeProgress(context: Context) : ProgressBar(context)

class SmokeScroll(context: Context) : ScrollView(context)

fun buildLayout(context: Context) {
    val root = LinearLayout(context)
    root.orientation = LinearLayout.VERTICAL
    val label = TextView(context)
    label.text = "hello"
    label.setText(android.R.string.ok)
    label.textSize = 14f
    label.setTextColor(0)
    val input = EditText(context)
    input.hint = "type"
    val button = Button(context)
    button.setOnClickListener { Toast.makeText(context, "clicked", Toast.LENGTH_SHORT).show() }
    val image = ImageView(context)
    image.contentDescription = "logo"
    image.setImageResource(android.R.drawable.ic_menu_add)
    root.addView(label)
    root.addView(input)
    root.addView(button)
    root.addView(image)
    val frame: FrameLayout = SmokeScroll(context)
    frame.addView(root)
    val horizontal: FrameLayout = HorizontalScrollView(context)
    horizontal.addView(GridLayout(context).apply { columnCount = 2 })
    val progress = SmokeProgress(context)
    progress.progress = 50
    progress.isIndeterminate = false
    Toast.makeText(context, android.R.string.ok, Toast.LENGTH_LONG).show()
}
