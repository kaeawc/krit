package fixtures.compose

import androidx.constraintlayout.compose.ConstraintSet

val ids = arrayOf("title", "subtitle")

val constraints = ConstraintSet {
    val (title, subtitle, icon) = createRefsFor("title", "subtitle", "icon")
    val (a, b) = createRefsFor(*ids)
}
