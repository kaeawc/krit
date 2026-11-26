package fixtures.compose

import androidx.constraintlayout.compose.ConstraintSet
import androidx.constraintlayout.compose.Dimension

val constraints = ConstraintSet {
    val (title, button) = createRefsFor("title", "button")
    constrain(title) {
        width = Dimension.matchParent
        start.linkTo(button.end)
        top.linkTo(parent.top)
    }
}
