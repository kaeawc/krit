package fixtures.compose

import androidx.compose.ui.Modifier
import androidx.compose.ui.input.pointer.pointerInput

fun Modifier.tapAndDrag(): Modifier = pointerInput(Unit) {
    awaitPointerEventScope {
        while (true) {
            awaitPointerEvent()
        }
    }
}.pointerInput(Unit) {
    awaitPointerEventScope { awaitPointerEvent() }
}

class Recorder {
    fun <T> awaitPointerEventScope(block: () -> T): T = block()

    fun twice() {
        awaitPointerEventScope { 1 }
        awaitPointerEventScope { 2 }
    }
}
