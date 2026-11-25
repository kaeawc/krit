package p.params

import androidx.compose.ui.Modifier
import androidx.compose.ui.input.pointer.PointerEvent
import androidx.compose.ui.input.pointer.PointerInputScope
import androidx.compose.ui.input.pointer.pointerInput

fun Modifier.handler(
    block: suspend PointerInputScope.() -> Unit = { awaitPointerEventScope { awaitPointerEvent() } },
): Modifier = pointerInput(Unit, block)

fun Modifier.custom(block: suspend PointerInputScope.() -> Unit): Modifier = pointerInput(Unit, block)

fun Modifier.usesCustom(): Modifier = custom { awaitPointerEventScope { awaitPointerEvent() } }

suspend fun PointerInputScope.typedLocal() {
    val block: suspend PointerInputScope.() -> PointerEvent = { awaitPointerEventScope { awaitPointerEvent() } }
    block()
}
