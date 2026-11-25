package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.saveable.listSaver
import androidx.compose.runtime.saveable.rememberSaveable

class Point(val x: Int, val y: Int)

val PointSaver = listSaver<Point, Int>(save = { listOf(it.x, it.y) }, restore = { Point(it[0], it[1]) })

@Composable
fun Canvas() {
    val origin = rememberSaveable(PointSaver) { Point(0, 0) }
}
