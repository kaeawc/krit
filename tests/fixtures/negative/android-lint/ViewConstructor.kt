package com.example

import android.content.Context
import android.util.AttributeSet
import android.view.View

class CustomView @JvmOverloads constructor(
    context: Context,
    attrs: AttributeSet? = null,
    defStyleAttr: Int = 0
) : View(context, attrs, defStyleAttr) {
}

// Activity subclass should NOT be flagged (not a View)
class MyActivity : Activity() {
    // body mentions ": View(" in a string but is not a View subclass
    val label = "extends: View(foo)"
}

// Fragment subclass should NOT be flagged
class MyFragment : Fragment() {
    fun getViewName() = "View("
}

// Transition subclass should NOT be flagged
class FadeTransition : Transition() {
    override fun captureStartValues(transitionValues: TransitionValues) {}
}

// Service subclass should NOT be flagged
class MyService : Service() {
    override fun onBind(intent: Intent) = null
}

// Plain class with no superclass should NOT be flagged
class PlainHelper {
    fun createView(context: Context): View = View(context)
}
