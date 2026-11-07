// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 24
// Negatives for WrongCall: super calls, calls inside an override of the same
// name, bare calls with no explicit receiver,
// the right draw / measure / layout calls, and onDraw-named members of
// classes that are not Views.
package test

import android.content.Context
import android.graphics.Canvas
import android.view.View

class GaugeView(context: Context) : View(context) {
    override fun onDraw(canvas: Canvas) {
        super.onDraw(canvas)
    }

    override fun onMeasure(widthMeasureSpec: Int, heightMeasureSpec: Int) {
        super.onMeasure(widthMeasureSpec, heightMeasureSpec)
        <!WrongCall!>this.onDraw(Canvas())<!>
    }

    override fun onLayout(changed: Boolean, left: Int, top: Int, right: Int, bottom: Int) {
        post { <!WrongCall!>this.onMeasure(0, 0)<!> }
    }

    fun helperSuper(canvas: Canvas) {
        super.onDraw(canvas)
    }

    fun bare(canvas: Canvas) {
        onDraw(canvas)
        onMeasure(0, 0)
    }

    fun rightCalls(other: GaugeView) {
        other.layout(0, 0, 10, 10)
        other.requestLayout()
        other.invalidate()
    }
}

class Renderer {
    fun onDraw(canvas: Canvas) {
    }

    fun onMeasure(width: Int, height: Int) {
    }
}

class RenderHost {
    private val renderer = Renderer()

    fun render(canvas: Canvas) {
        renderer.onDraw(canvas)
        Renderer().onMeasure(1, 2)
    }
}

abstract class Measurer {
    abstract fun onMeasure(width: Int, height: Int)
}

class MeasurerImpl : Measurer() {
    override fun onMeasure(width: Int, height: Int) {
    }

    fun again(other: MeasurerImpl) {
        other.onMeasure(1, 2)
    }
}
