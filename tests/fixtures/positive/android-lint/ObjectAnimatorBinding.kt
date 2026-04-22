import android.animation.ObjectAnimator
import android.view.View

private class Gauge

fun viewTypo(view: View) {
    ObjectAnimator.ofFloat(view, "translatoinX", 1f)
}

fun multilineViewTypo(view: View) {
    ObjectAnimator.ofFloat(
        view,
        "rotatoin",
        1f,
    )
}

fun namedArgumentTypo(view: View) {
    ObjectAnimator.ofFloat(
        target = view,
        propertyName = "translatoinY",
        values = floatArrayOf(1f),
    )
}

fun customTargetMissingSetter(gauge: Gauge) {
    ObjectAnimator.ofFloat(gauge, "progressFraction", 1f)
}
