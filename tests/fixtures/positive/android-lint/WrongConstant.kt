package test

annotation class IntDef(vararg val value: Int, val flag: Boolean = false)

@IntDef(View.VISIBLE, View.INVISIBLE, View.GONE)
annotation class Visibility

@IntDef(View.LAYOUT_DIRECTION_LTR, View.LAYOUT_DIRECTION_RTL)
annotation class LayoutDirection

@IntDef(View.IMPORTANT_FOR_ACCESSIBILITY_AUTO, View.IMPORTANT_FOR_ACCESSIBILITY_YES, View.IMPORTANT_FOR_ACCESSIBILITY_NO)
annotation class ImportantForAccessibility

@IntDef(Gravity.LEFT, Gravity.RIGHT, Gravity.CENTER, flag = true)
annotation class GravityInt

@IntDef(LinearLayout.HORIZONTAL, LinearLayout.VERTICAL)
annotation class Orientation

open class View {
    companion object {
        const val VISIBLE = 0
        const val INVISIBLE = 4
        const val GONE = 8
        const val LAYOUT_DIRECTION_LTR = 0
        const val LAYOUT_DIRECTION_RTL = 1
        const val IMPORTANT_FOR_ACCESSIBILITY_AUTO = 0
        const val IMPORTANT_FOR_ACCESSIBILITY_YES = 1
        const val IMPORTANT_FOR_ACCESSIBILITY_NO = 2
    }

    fun setVisibility(@Visibility value: Int) {}
    fun setLayoutDirection(@LayoutDirection value: Int) {}
    fun setImportantForAccessibility(@ImportantForAccessibility value: Int) {}
}

object Gravity {
    const val LEFT = 3
    const val RIGHT = 5
    const val CENTER = 17
}

class TextView : View() {
    fun setGravity(@GravityInt value: Int) {}
}

class LinearLayout : View() {
    companion object {
        const val HORIZONTAL = 0
        const val VERTICAL = 1
    }

    fun setOrientation(@Orientation value: Int) {}
}

fun configure(view: View, textView: TextView, layout: LinearLayout) {
    view.setVisibility(0)
    view.setLayoutDirection(
        1
    )
    view.setImportantForAccessibility(2)
    textView.setGravity(17)
    layout.setOrientation(1)
}
