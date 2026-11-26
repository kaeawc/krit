package fixtures.compose

import androidx.constraintlayout.compose.ConstraintSet

val ids = arrayOf("title", "subtitle")

val constraints = ConstraintSet {
    val (title, subtitle, icon) = createRefsFor("title", "subtitle", "icon")
    val (a, b) = createRefsFor(*ids)
}

// A lookalike createRefsFor on another receiver is not ConstraintSetScope's.
class RefBuilder {
    fun createRefsFor(vararg ids: String): List<Int> = ids.map { 0 }
}

fun build(b: RefBuilder) = with(b) {
    val (x, y) = createRefsFor("a", "b", "c")
    x + y
}
