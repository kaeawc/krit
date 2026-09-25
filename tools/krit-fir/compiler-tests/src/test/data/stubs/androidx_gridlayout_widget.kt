// Compiler-test source stubs; never packaged in the production artifact.
package androidx.gridlayout.widget

import android.content.Context
import android.util.AttributeSet
import android.view.ViewGroup

open class GridLayout : ViewGroup {
    constructor(context: Context) : super(context)

    constructor(context: Context, attrs: AttributeSet?) : super(context, attrs)

    constructor(context: Context, attrs: AttributeSet?, defStyle: Int) : super(context, attrs, defStyle)

    open var columnCount: Int
        get() = TODO()
        set(value) = TODO()

    open var rowCount: Int
        get() = TODO()
        set(value) = TODO()

    open var orientation: Int
        get() = TODO()
        set(value) = TODO()

    override fun onLayout(changed: Boolean, l: Int, t: Int, r: Int, b: Int) {
        TODO()
    }

    companion object {
        const val HORIZONTAL: Int = 0
        const val VERTICAL: Int = 1
    }
}
