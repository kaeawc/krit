package test

annotation class IntDef(vararg val value: Int, val flag: Boolean = false)

@IntDef(View.VISIBLE, View.INVISIBLE, View.GONE)
annotation class Visibility

open class View {
    companion object {
        const val VISIBLE = 0
        const val INVISIBLE = 4
        const val GONE = 8
    }

    fun setVisibility(@Visibility value: Int) {}
}

class Fake {
    fun setVisibility(value: Int) {}
}

const val LOCAL_VISIBLE = 0

fun configure(view: View, fake: Fake) {
    view.setVisibility(View.GONE)
    view.setVisibility(LOCAL_VISIBLE)
    fake.setVisibility(0)
    unresolved.setVisibility(0)
    val text = "view.setVisibility(0)"
    // view.setVisibility(0)
}
