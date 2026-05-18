package com.example

import android.graphics.Canvas

// onDraw inside a non-View class is a lookalike callback — must not fire.
class CustomRenderer {
    fun render(canvas: Canvas) {
        // Bare call to onDraw with a non-View receiver.
        val helper = Helper()
        helper.onDraw(canvas)
    }
}

class Helper {
    fun onDraw(canvas: Canvas) {
        // Body irrelevant.
    }
}

// Override inside a non-View base class should also be skipped.
abstract class CustomMeasurer {
    abstract fun onMeasure(width: Int, height: Int)
}

class MeasurerImpl : CustomMeasurer() {
    override fun onMeasure(width: Int, height: Int) {
        // override modifier — explicitly skipped.
    }
}

// Super call inside a real View subclass is allowed.
class CustomViewSuper : android.view.View {
    constructor() : super(null)
    override fun onDraw(canvas: Canvas) {
        super.onDraw(canvas) // skip: receiver is super.
    }
}
