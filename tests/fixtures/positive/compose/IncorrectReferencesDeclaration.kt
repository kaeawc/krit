package fixtures.compose

import androidx.constraintlayout.compose.ConstraintSet

val constraints = ConstraintSet {
    val (title, subtitle) = createRefsFor("title", "subtitle", "icon")
}
