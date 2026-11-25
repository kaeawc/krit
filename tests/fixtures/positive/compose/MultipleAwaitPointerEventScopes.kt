package fixtures.compose

import androidx.compose.ui.Modifier
import androidx.compose.ui.input.pointer.pointerInput

fun Modifier.tapAndDrag(): Modifier = pointerInput(Unit) {
    awaitPointerEventScope { awaitPointerEvent() }
    awaitPointerEventScope { awaitPointerEvent() }
}
