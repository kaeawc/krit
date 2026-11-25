package t

import android.app.Activity
import android.content.Context
import android.content.res.Resources
import android.view.View
import android.widget.TextView
import androidx.compose.runtime.Composable
import androidx.compose.runtime.RememberObserver
import androidx.compose.runtime.annotation.DoNotRetain
import androidx.compose.runtime.retain.RetainObserver
import androidx.compose.runtime.retain.RetainedValuesStore
import androidx.compose.runtime.retain.retain

class Plain
class Observer : RememberObserver {
    override fun onRemembered() {}
    override fun onForgotten() {}
    override fun onAbandoned() {}
}
class Both : RememberObserver, RetainObserver {
    override fun onRemembered() {}
    override fun onForgotten() {}
    override fun onAbandoned() {}
    override fun onRetained() {}
    override fun onEnteredComposition() {}
    override fun onExitedComposition() {}
    override fun onRetired() {}
    override fun onUnused() {}
}

@DoNotRetain("Unretainable for testing")
class Marked
@DoNotRetain
class MarkedNoReason
class MarkedChild : MarkedBase()
@DoNotRetain("base")
open class MarkedBase

fun noop() {}

@Composable
fun Retains(context: Context, activity: Activity, view: View, res: Resources, k: Int) {
    retain { Unit }
    retain<Unit> { }
    retain { noop() }
    retain { 42 }
    retain { Plain() }
    retain { Observer() }
    retain { Both() }
    retain { Marked() }
    retain { MarkedNoReason() }
    retain { MarkedChild() }
    retain { context }
    retain { activity }
    retain { view }
    retain { res }
    retain { TextView(context) }
    retain(context) { 42 }
    retain(k, view) { 42 }
    retain(Marked()) { 42 }
    retain { listOf<Context>() }
}
