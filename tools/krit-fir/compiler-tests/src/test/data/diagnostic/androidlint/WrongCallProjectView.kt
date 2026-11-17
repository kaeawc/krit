// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23, 24
// A project class named View, not android.view.View. Its onDraw / onLayout
// are not View callbacks, so the message ("should probably call `draw`
// rather than `onDraw`") is not true of these calls and FIR reports neither,
// as Lint, which needs a member of android.view.View or a subclass, does not.
package test

import android.graphics.Canvas

class View {
    fun onDraw(canvas: Canvas) {
    }

    fun onLayout(changed: Boolean) {
    }
}

// Go reports both calls: it proves a View receiver by the simple name `View`,
// and this View is the project class above, not android.view.View.
class Screen {
    fun paint(view: View, canvas: Canvas) {
        view.onDraw(canvas)
        view.onLayout(true)
    }
}
