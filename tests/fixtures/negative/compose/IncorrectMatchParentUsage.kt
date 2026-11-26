package fixtures.compose

import androidx.constraintlayout.compose.ConstraintSet
import androidx.constraintlayout.compose.Dimension

val constraints = ConstraintSet {
    val (title, button) = createRefsFor("title", "button")
    constrain(title) {
        width = Dimension.matchParent
        start.linkTo(parent.start)
        end.linkTo(parent.end)
    }
    constrain(button) {
        width = Dimension.percent(1f)
        start.linkTo(title.end)
        top.linkTo(parent.top)
    }
}
