package fixtures.compose

import androidx.compose.ui.Modifier
import androidx.compose.ui.input.pointer.PointerInputScope
import androidx.compose.ui.input.pointer.pointerInput
import kotlinx.coroutines.coroutineScope

fun Modifier.events(): Modifier = pointerInput(Unit) {
    awaitPointerEventScope { awaitPointerEvent() }
}

suspend fun PointerInputScope.scoped() = coroutineScope {
    awaitPointerEventScope { awaitPointerEvent() }
}

val handler: suspend PointerInputScope.() -> Unit = {
    awaitPointerEventScope { awaitPointerEvent() }
}

suspend fun PointerInputScope.kept() {
    awaitPointerEventScope {
        val event = awaitPointerEvent()
        println(event)
    }
}
