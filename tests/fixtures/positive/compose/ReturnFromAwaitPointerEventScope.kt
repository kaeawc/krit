package fixtures.compose

import androidx.compose.ui.input.pointer.PointerEvent
import androidx.compose.ui.input.pointer.PointerInputScope

suspend fun PointerInputScope.nextEvent(): PointerEvent {
    val event = awaitPointerEventScope { awaitPointerEvent() }
    return event
}
