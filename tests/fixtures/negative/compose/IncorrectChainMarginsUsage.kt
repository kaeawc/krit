package fixtures.compose

import androidx.compose.ui.unit.dp
import androidx.constraintlayout.compose.ConstraintSet

val constraints = ConstraintSet {
    val (first, second) = createRefsFor("first", "second")
    createHorizontalChain(first.withChainParams(endMargin = 8.dp), second)
    constrain(first) {
        top.linkTo(parent.top)
    }
    constrain(second) {}
}
