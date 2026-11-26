package fixtures.compose

import androidx.compose.ui.unit.dp
import androidx.constraintlayout.compose.ConstraintSet

val constraints = ConstraintSet {
    val (first, second) = createRefsFor("first", "second")
    createHorizontalChain(first, second)
    constrain(first) {
        end.linkTo(second.start, 8.dp)
        top.linkTo(parent.top)
    }
    constrain(second) {}
}
